// Package mermaid draws mermaid diagrams as characters: pele lays the
// diagram out and gives it as SVG, run in goja, a JavaScript engine in Go,
// so catu stays one static binary; the SVG is then drawn on a grid of
// terminal cells (svg.go).
package mermaid

import (
	_ "embed"
	"errors"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dop251/goja"
	terminu "github.com/vulcanshen/terminu-go"

	"github.com/vulcanshen/catu/internal/text"
)

// pele is Pele 1.13.0's browser bundle (https://github.com/obsidianmd/pele,
// MIT, pele/LICENSE).
//
//go:embed pele/pele.global.js
var pele string

// cellW and cellH are a terminal cell in the SVG's pixels: pele measures
// every character a cell wide (cellW), so its words land on the grid; a
// cell is twice as tall as it is wide.
const (
	cellW   = 8.4
	cellH   = 2 * cellW
	timeout = 3 * time.Second
)

var (
	once    sync.Once
	mu      sync.Mutex // goja runs one call at a time
	vm      *goja.Runtime
	render  goja.Callable
	loadErr error
	cache   = map[string]result{}
)

type result struct {
	rows [][]text.Char
	err  error
}

// Warm loads pele, which takes a moment, so the first diagram does not wait
// for it; any goroutine may call it.
func Warm() { once.Do(load) }

func load() {
	vm = goja.New()
	vm.Set("cellsOf", func(s string) int { return terminu.Width(s) })
	if _, err := vm.RunScript("pele.global.js", pele); err != nil {
		loadErr = err
		return
	}
	fn, err := vm.RunString(`(function (src, maxWidth) {
	var measurer = { width: function (t) { return cellsOf(t) * ` + strconv.FormatFloat(cellW, 'f', -1, 64) + `; } };
	// directionBreakpoint: turn a chart too wide for the panel top-down at any
	// width, not only under pele's 640px.
	return pele.render(src, { measurer: measurer, fontSize: 14, maxWidth: maxWidth, autoDirection: true, directionBreakpoint: 1e9, links: false, images: false }).svg;
})`)
	if err != nil {
		loadErr = err
		return
	}
	render, _ = goja.AssertFunction(fn)
}

// SVG is src laid out by pele as SVG, at most maxWidth pixels wide where
// it can be.
func SVG(src string, maxWidth float64) (string, error) {
	Warm()
	if loadErr != nil {
		return "", loadErr
	}
	mu.Lock()
	defer mu.Unlock()
	stop := time.AfterFunc(timeout, func() { vm.Interrupt("took too long") })
	defer func() {
		stop.Stop()
		vm.ClearInterrupt()
	}()
	v, err := render(goja.Undefined(), vm.ToValue(src), vm.ToValue(maxWidth))
	if err != nil {
		return "", errors.New(message(err))
	}
	return v.String(), nil
}

// message is what went wrong in pele, in its words: a PeleError's message.
func message(err error) string {
	var ex *goja.Exception
	if errors.As(err, &ex) {
		if o, ok := ex.Value().(*goja.Object); ok {
			if m := o.Get("message"); m != nil && !goja.IsUndefined(m) {
				return strings.TrimSpace(m.String())
			}
		}
		return strings.TrimSpace(ex.Value().String())
	}
	var in *goja.InterruptedError
	if errors.As(err, &in) {
		return "The diagram took too long to lay out"
	}
	return err.Error()
}

// Draw draws src, a mermaid diagram, as rows of characters, at most w cells
// wide where pele can lay it out so; a row may be wider when it cannot.
func Draw(src string, w int) ([][]text.Char, error) {
	key := strconv.Itoa(w) + "\x00" + src
	mu.Lock()
	r, ok := cache[key]
	mu.Unlock()
	if ok {
		return r.rows, r.err
	}
	laid := asDecisions(src)
	svg, err := SVG(laid, float64(w)*cellW)
	if err != nil && laid != src {
		// As written: the error in the user's own words, or the diagram
		// with its diamonds when only the hexagons failed.
		svg, err = SVG(src, float64(w)*cellW)
	}
	if p, ok := pieOf(src); ok && err == nil {
		r.rows = p.draw(w) // pele read it; it is drawn as bars (pie.go)
	} else if err == nil {
		r.rows, r.err = drawSVG(svg)
	} else {
		r.err = err
	}
	mu.Lock()
	cache[key] = r
	mu.Unlock()
	return r.rows, r.err
}
