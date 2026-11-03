package mermaid

import (
	"encoding/xml"
	"io"
	"math"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/rivo/uniseg"
	terminu "github.com/vulcanshen/terminu-go"

	"github.com/vulcanshen/viewu/internal/text"
)

// The diagram's colours: what each part of it is (the content's own, tdp
// P4, P5).
var (
	overlay1 = terminu.Hex("#7f849c")
	overlay2 = terminu.Hex("#9399b2")
	subtext0 = terminu.Hex("#a6adc8")
)

// Directions a box-drawing cell joins to.
const (
	up = 1 << iota
	down
	left
	right
)

// boxChars draws the directions a cell joins to; corners are round.
var boxChars = map[int]string{
	left | right: "─", up | down: "│", left: "─", right: "─", up: "│", down: "│",
	right | down: "╭", left | down: "╮", right | up: "╰", left | up: "╯",
	left | right | down: "┬", left | right | up: "┴", up | down | right: "├", up | down | left: "┤",
	up | down | left | right: "┼",
}

// cell is one cell of the grid: a glyph (a word's character, an arrow) over
// the lines drawn with box-drawing characters over the dots of the curves.
type cell struct {
	glyph  string
	glyphW int
	cont   bool // the second cell of a wide glyph
	gst    text.Style
	box    int
	dashed bool
	bst    text.Style
	dots   uint8
	dst    text.Style
}

type grid struct {
	w, h   int
	cells  [][]cell
	arrows []arrowMark // drawn last, over the lines they point along
}

func newGrid(w, h int) *grid {
	g := &grid{w: w, h: h, cells: make([][]cell, h)}
	for i := range g.cells {
		g.cells[i] = make([]cell, w)
	}
	return g
}

func (g *grid) at(c, r int) *cell {
	if c < 0 || r < 0 || c >= g.w || r >= g.h {
		return nil
	}
	return &g.cells[r][c]
}

func col(x float64) int { return int(math.Round(x / cellW)) }
func row(y float64) int { return int(math.Round(y / cellH)) }

// hline joins the cells from x1 to x2 on the row of y.
func (g *grid) hline(x1, x2, y float64, st text.Style, dashed bool) {
	c1, c2, r := col(math.Min(x1, x2)), col(math.Max(x1, x2)), row(y)
	for c := c1; c <= c2; c++ {
		k := g.at(c, r)
		if k == nil {
			continue
		}
		switch {
		case c1 == c2:
			if k.box != 0 {
				continue // no length on the grid: a joint of a line already there
			}
			k.box |= left | right
		case c == c1:
			k.box |= right
		case c == c2:
			k.box |= left
		default:
			k.box |= left | right
		}
		k.bst, k.dashed = st, k.dashed || dashed
	}
}

// vline joins the cells from y1 to y2 on the column of x.
func (g *grid) vline(x, y1, y2 float64, st text.Style, dashed bool) {
	r1, r2, c := row(math.Min(y1, y2)), row(math.Max(y1, y2)), col(x)
	for r := r1; r <= r2; r++ {
		k := g.at(c, r)
		if k == nil {
			continue
		}
		switch {
		case r1 == r2:
			if k.box != 0 {
				continue // no length on the grid: a joint of a line already there
			}
			k.box |= up | down
		case r == r1:
			k.box |= down
		case r == r2:
			k.box |= up
		default:
			k.box |= up | down
		}
		k.bst, k.dashed = st, k.dashed || dashed
	}
}

// brailleBits are the dots of a braille character by place: two columns,
// four rows.
var brailleBits = [2][4]uint8{{0x01, 0x02, 0x04, 0x40}, {0x08, 0x10, 0x20, 0x80}}

// dot puts a dot at x, y.
func (g *grid) dot(x, y float64, st text.Style) {
	fx, fy := x/cellW+0.5, y/cellH+0.5
	c, r := int(math.Floor(fx)), int(math.Floor(fy))
	k := g.at(c, r)
	if k == nil {
		return
	}
	sx := min(max(int((fx-float64(c))*2), 0), 1)
	sy := min(max(int((fy-float64(r))*4), 0), 3)
	k.dots |= brailleBits[sx][sy]
	k.dst = st
}

// line draws a straight line: across or down with box-drawing characters,
// any other way with dots.
func (g *grid) line(a, b point, st text.Style, dashed bool) {
	switch {
	case math.Abs(a.y-b.y) < 0.75:
		g.hline(a.x, b.x, a.y, st, dashed)
	case math.Abs(a.x-b.x) < 0.75:
		g.vline(a.x, a.y, b.y, st, dashed)
	default:
		steps := int(math.Max(math.Abs(b.x-a.x)/(cellW/4), math.Abs(b.y-a.y)/(cellH/8))) + 1
		for i := 0; i <= steps; i++ {
			t := float64(i) / float64(steps)
			g.dot(a.x+(b.x-a.x)*t, a.y+(b.y-a.y)*t, st)
		}
	}
}

// glyph puts a character at cell c, r.
func (g *grid) glyph(c, r int, s string, st text.Style) {
	k := g.at(c, r)
	if k == nil {
		return
	}
	w := max(terminu.Width(s), 1)
	k.glyph, k.glyphW, k.gst, k.cont = s, w, st, false
	k.box, k.dots = 0, 0
	for i := 1; i < w; i++ {
		if n := g.at(c+i, r); n != nil {
			*n = cell{cont: true}
		}
	}
}

// words puts s on row r from column c, clearing pad cells either side of it
// (an edge's label sits on the edge).
func (g *grid) words(c, r int, s string, st text.Style, pad int) {
	for i := 1; i <= pad; i++ {
		if k := g.at(c-i, r); k != nil && k.glyph == "" {
			k.box, k.dots = 0, 0
		}
	}
	gr := uniseg.NewGraphemes(s)
	for gr.Next() {
		ch := gr.Str()
		if name, ok := text.CtrlName(ch); ok {
			red := st
			red.Fg = terminu.Red
			for _, b := range name {
				g.glyph(c, r, string(b), red)
				c++
			}
			continue
		}
		g.glyph(c, r, ch, st)
		c += max(terminu.Width(ch), 1)
	}
	for i := 0; i < pad; i++ {
		if k := g.at(c+i, r); k != nil && k.glyph == "" {
			k.box, k.dots = 0, 0
		}
	}
}

// rows are the grid as rows of characters, the blank rows at the top and
// the bottom and the blank columns on the left left out.
func (g *grid) rows() [][]text.Char {
	first, last := g.h, -1
	leftmost := g.w
	for r := range g.cells {
		for c := range g.cells[r] {
			k := g.cells[r][c]
			if k.glyph != "" || k.box != 0 || k.dots != 0 {
				first, last = min(first, r), max(last, r)
				leftmost = min(leftmost, c)
			}
		}
	}
	var out [][]text.Char
	for r := first; r <= last; r++ {
		var line []text.Char
		for c := leftmost; c < g.w; c++ {
			k := g.cells[r][c]
			switch {
			case k.cont:
			case k.glyph != "":
				line = append(line, text.Char{Draw: k.glyph, W: k.glyphW, St: k.gst, Src: k.glyph})
			case k.box != 0:
				s := boxChars[k.box]
				if k.dashed {
					switch k.box {
					case left | right, left, right:
						s = "╌"
					case up | down, up, down:
						s = "╎"
					}
				}
				line = append(line, text.Char{Draw: s, W: 1, St: k.bst, Src: s})
			case k.dots != 0:
				s := string(rune(0x2800 + int(k.dots)))
				line = append(line, text.Char{Draw: s, W: 1, St: k.dst, Src: s})
			default:
				line = append(line, text.Char{Draw: " ", W: 1, Src: " "})
			}
		}
		for len(line) > 0 && line[len(line)-1].Draw == " " {
			line = line[:len(line)-1]
		}
		out = append(out, line)
	}
	return out
}

// --- reading the SVG --------------------------------------------------------

type point struct{ x, y float64 }

// matrix is an affine transform: x' = a x + c y + e, y' = b x + d y + f.
type matrix struct{ a, b, c, d, e, f float64 }

var identity = matrix{a: 1, d: 1}

func (m matrix) apply(p point) point {
	return point{m.a*p.x + m.c*p.y + m.e, m.b*p.x + m.d*p.y + m.f}
}

func (m matrix) then(n matrix) matrix { // m applied after n: m × n
	return matrix{
		a: m.a*n.a + m.c*n.b, b: m.b*n.a + m.d*n.b,
		c: m.a*n.c + m.c*n.d, d: m.b*n.c + m.d*n.d,
		e: m.a*n.e + m.c*n.f + m.e, f: m.b*n.e + m.d*n.f + m.f,
	}
}

var transformRe = regexp.MustCompile(`(\w+)\s*\(([^)]*)\)`)

// parseTransform reads a transform attribute: translate, scale, rotate,
// matrix.
func parseTransform(s string) matrix {
	m := identity
	for _, t := range transformRe.FindAllStringSubmatch(s, -1) {
		v := numbers(t[2])
		n := identity
		switch t[1] {
		case "translate":
			if len(v) > 0 {
				n.e = v[0]
			}
			if len(v) > 1 {
				n.f = v[1]
			}
		case "scale":
			if len(v) > 0 {
				n.a, n.d = v[0], v[0]
			}
			if len(v) > 1 {
				n.d = v[1]
			}
		case "rotate":
			if len(v) > 0 {
				r := v[0] * math.Pi / 180
				n = matrix{a: math.Cos(r), b: math.Sin(r), c: -math.Sin(r), d: math.Cos(r)}
				if len(v) > 2 {
					n = matrix{a: 1, d: 1, e: v[1], f: v[2]}.then(n).then(matrix{a: 1, d: 1, e: -v[1], f: -v[2]})
				}
			}
		case "matrix":
			if len(v) == 6 {
				n = matrix{v[0], v[1], v[2], v[3], v[4], v[5]}
			}
		}
		m = m.then(n)
	}
	return m
}

var numberRe = regexp.MustCompile(`[-+]?(?:\d+\.?\d*|\.\d+)(?:[eE][-+]?\d+)?`)

func numbers(s string) []float64 {
	var out []float64
	for _, n := range numberRe.FindAllString(s, -1) {
		v, _ := strconv.ParseFloat(n, 64)
		out = append(out, v)
	}
	return out
}

// frame is what an element takes from the groups it is in.
type frame struct {
	m        matrix
	classes  string
	fill     string
	stroke   string
	fontSize float64
	anchor   string
	dashed   bool
}

// has reports whether the element or a group it is in has class: which part
// of the diagram it belongs to, which gives its colours.
func (f frame) has(class string) bool {
	for _, c := range strings.Fields(f.classes) {
		if c == class {
			return true
		}
	}
	return false
}

func (f frame) lineStyle() text.Style {
	switch {
	case f.has("pele-node"), f.has("pele-actor"), f.has("pele-state"):
		return text.Style{Fg: terminu.Blue}
	case f.has("pele-cluster"), f.has("pele-lifeline"), f.has("pele-lifelines"):
		return text.Style{Fg: terminu.Overlay0}
	}
	if c, ok := hexOf(f.stroke); ok {
		return text.Style{Fg: c}
	}
	return text.Style{Fg: overlay2}
}

func (f frame) wordStyle() text.Style {
	switch {
	case f.has("pele-edge-label"), f.has("pele-edge-labels"):
		return text.Style{Fg: subtext0}
	case f.has("pele-cluster-label"), f.has("pele-cluster-title"):
		return text.Style{Fg: overlay1}
	case f.has("pele-title"):
		return text.Style{Fg: terminu.Text, Bold: true}
	}
	return text.Style{Fg: terminu.Text}
}

var hexRe = regexp.MustCompile(`#[0-9a-fA-F]{6}\b`)

// hexOf is the colour a fill or a stroke names: its own, or the one a
// var() falls back to.
func hexOf(s string) (terminu.Color, bool) {
	if h := hexRe.FindString(s); h != "" {
		return terminu.Hex(h), true
	}
	return terminu.Color{}, false
}

func attr(se xml.StartElement, name string) (string, bool) {
	for _, a := range se.Attr {
		if a.Name.Local == name {
			return a.Value, true
		}
	}
	return "", false
}

func num(se xml.StartElement, name string) float64 {
	v, _ := attr(se, name)
	f, _ := strconv.ParseFloat(strings.TrimSuffix(strings.TrimSpace(v), "px"), 64)
	return f
}

// textRun is a text element's words waiting to be placed.
type textRun struct {
	f       frame
	x, y    float64
	words   strings.Builder
	spans   bool
	skip    bool
	central bool
}

// drawSVG draws pele's SVG on a grid of cells.
func drawSVG(svg string) ([][]text.Char, error) {
	dec := xml.NewDecoder(strings.NewReader(svg))
	var g *grid
	stack := []frame{{m: identity, fontSize: 14, anchor: "start"}}
	var runs []*textRun
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			f := stack[len(stack)-1]
			if tr, ok := attr(t, "transform"); ok {
				f.m = f.m.then(parseTransform(tr))
			}
			if c, ok := attr(t, "class"); ok {
				f.classes += " " + c
			}
			if v, ok := attr(t, "fill"); ok {
				f.fill = v
			}
			if v, ok := attr(t, "stroke"); ok {
				f.stroke = v
			}
			if v, ok := attr(t, "font-size"); ok {
				if n := numbers(v); len(n) > 0 {
					f.fontSize = n[0]
				}
			}
			if v, ok := attr(t, "text-anchor"); ok {
				f.anchor = v
			}
			if _, ok := attr(t, "stroke-dasharray"); ok {
				f.dashed = true
			}
			stack = append(stack, f)
			switch t.Name.Local {
			case "svg":
				w, h := num(t, "width"), num(t, "height")
				if vb, ok := attr(t, "viewBox"); ok && (w == 0 || h == 0) {
					if v := numbers(vb); len(v) == 4 {
						w, h = v[2], v[3]
					}
				}
				g = newGrid(col(w)+3, row(h)+3)
			case "text":
				run := &textRun{f: f, x: num(t, "x"), y: num(t, "y")}
				run.skip = strings.TrimSpace(f.fill) == "none"
				if b, ok := attr(t, "dominant-baseline"); ok && (b == "central" || b == "middle") {
					run.central = true
				}
				runs = append(runs, run)
			case "tspan":
				if len(runs) > 0 {
					top := runs[len(runs)-1]
					top.spans = true
					span := &textRun{f: f, x: top.x, y: top.y, skip: top.skip, central: top.central}
					if _, ok := attr(t, "x"); ok {
						span.x = num(t, "x")
					}
					if _, ok := attr(t, "y"); ok {
						span.y = num(t, "y")
					}
					if v, ok := attr(t, "dy"); ok {
						span.y += length(v, f.fontSize)
					}
					runs = append(runs, span)
				}
			default:
				if g != nil {
					shape(g, t, f)
				}
			}
		case xml.CharData:
			if len(runs) > 0 {
				runs[len(runs)-1].words.Write(t)
			}
		case xml.EndElement:
			if (t.Name.Local == "text" || t.Name.Local == "tspan") && len(runs) > 0 {
				run := runs[len(runs)-1]
				runs = runs[:len(runs)-1]
				if g != nil && !run.skip && !(t.Name.Local == "text" && run.spans) {
					place(g, run)
				}
			}
			stack = stack[:len(stack)-1]
		}
	}
	if g == nil {
		return nil, nil
	}
	for _, a := range g.arrows {
		g.arrow(a)
	}
	return g.rows(), nil
}

// length is an SVG length in pixels: em counts the font size.
func length(s string, font float64) float64 {
	n := numbers(s)
	if len(n) == 0 {
		return 0
	}
	if strings.HasSuffix(strings.TrimSpace(s), "em") {
		return n[0] * font
	}
	return n[0]
}

// place puts a text's words on the grid: its baseline moved up to the
// middle of its letters, its anchor kept.
func place(g *grid, run *textRun) {
	words := strings.Join(strings.FieldsFunc(run.words.String(), func(r rune) bool { return r == '\n' || r == '\r' || r == '\t' }), " ")
	words = strings.TrimFunc(words, unicode.IsSpace)
	if words == "" {
		return
	}
	y := run.y
	if !run.central {
		y -= run.f.fontSize * 0.35
	}
	p := run.f.m.apply(point{run.x, y})
	w := float64(terminu.Width(words)) * cellW
	switch run.f.anchor {
	case "middle":
		p.x -= w / 2
	case "end":
		p.x -= w
	}
	pad := 0
	if run.f.has("pele-edge-label") || run.f.has("pele-edge-labels") {
		pad = 1
	}
	g.words(col(p.x), row(p.y), words, run.f.wordStyle(), pad)
}

// shape draws a shape: its outline, an arrow where it is an edge's marker,
// a dot where it is smaller than a cell.
func shape(g *grid, t xml.StartElement, f frame) {
	st := f.lineStyle()
	fill := strings.TrimSpace(f.fill)
	stroked := f.stroke != "" && strings.TrimSpace(f.stroke) != "none"
	if v, ok := attr(t, "stroke"); ok && strings.TrimSpace(v) == "none" {
		stroked = false
	}
	filled := fill != "" && fill != "none"
	if !stroked && filled {
		if c, ok := hexOf(fill); ok {
			st = text.Style{Fg: c}
		}
	}
	switch t.Name.Local {
	case "rect":
		x, y, w, h := num(t, "x"), num(t, "y"), num(t, "width"), num(t, "height")
		if !stroked && filled && (f.has("pele-edge-label") || strings.Contains(fill, "--_bg")) {
			return // a label's ground: its words clear the line under them
		}
		if !stroked && !filled {
			return
		}
		a, b := f.m.apply(point{x, y}), f.m.apply(point{x + w, y + h})
		if math.Abs(b.x-a.x) < cellW*1.5 && math.Abs(b.y-a.y) < cellH*1.5 {
			c := f.m.apply(point{x + w/2, y + h/2})
			g.glyph(col(c.x), row(c.y), "■", st)
			return
		}
		// The top and the bottom an equal number of rows from the middle
		// row, where the words and the edges at it are: the words centred.
		mid := (a.y + b.y) / 2
		half := float64(max(1, int(math.Round(math.Abs(b.y-a.y)/2/cellH)))) * cellH
		top, bottom := float64(row(mid))*cellH-half, float64(row(mid))*cellH+half
		g.line(point{a.x, top}, point{b.x, top}, st, f.dashed)
		g.line(point{b.x, top}, point{b.x, bottom}, st, f.dashed)
		g.line(point{b.x, bottom}, point{a.x, bottom}, st, f.dashed)
		g.line(point{a.x, bottom}, point{a.x, top}, st, f.dashed)
	case "circle", "ellipse":
		cx, cy := num(t, "cx"), num(t, "cy")
		rx, ry := num(t, "r"), num(t, "r")
		if t.Name.Local == "ellipse" {
			rx, ry = num(t, "rx"), num(t, "ry")
		}
		c := f.m.apply(point{cx, cy})
		if rx < cellW && ry < cellH/2 {
			mark := "○"
			if filled && !stroked {
				mark = "●"
			}
			g.glyph(col(c.x), row(c.y), mark, st)
			return
		}
		n := int(2*math.Pi*math.Max(rx, ry)/2) + 8
		for i := 0; i < n; i++ {
			a := 2 * math.Pi * float64(i) / float64(n)
			p := f.m.apply(point{cx + rx*math.Cos(a), cy + ry*math.Sin(a)})
			g.dot(p.x, p.y, st)
		}
	case "line":
		a := f.m.apply(point{num(t, "x1"), num(t, "y1")})
		b := f.m.apply(point{num(t, "x2"), num(t, "y2")})
		g.line(a, b, st, f.dashed)
	case "polyline", "polygon":
		v, _ := attr(t, "points")
		ns := numbers(v)
		var pts []point
		for i := 0; i+1 < len(ns); i += 2 {
			pts = append(pts, f.m.apply(point{ns[i], ns[i+1]}))
		}
		if t.Name.Local == "polygon" && len(pts) > 2 {
			pts = append(pts, pts[0])
		}
		for i := 1; i < len(pts); i++ {
			g.line(pts[i-1], pts[i], st, f.dashed)
		}
	case "path":
		d, _ := attr(t, "d")
		cls, _ := attr(t, "class")
		if strings.Contains(cls, "pele-marker") {
			if a, ok := arrowOf(flatten(d, f.m)); ok {
				g.arrows = append(g.arrows, a)
			}
			return
		}
		pathLines(g, d, f, st)
	}
}

// arrowMark is an edge's marker waiting to be drawn: the cell of its tip
// and the way it points, a cell at a time.
type arrowMark struct {
	c, r, dc, dr int
	mark         string
}

// arrowOf is a marker's triangle as an arrow.
func arrowOf(pts []point) (arrowMark, bool) {
	if len(pts) < 3 {
		return arrowMark{}, false
	}
	pts = pts[:3]
	var tip, base point
	far := -1.0
	for i := range pts {
		a, b := pts[(i+1)%3], pts[(i+2)%3]
		mid := point{(a.x + b.x) / 2, (a.y + b.y) / 2}
		if d := math.Hypot(pts[i].x-mid.x, pts[i].y-mid.y); d > far {
			far, tip, base = d, pts[i], mid
		}
	}
	dx, dy := tip.x-base.x, tip.y-base.y
	a := arrowMark{c: col(tip.x), r: row(tip.y)}
	switch {
	case math.Abs(dx)/cellW >= math.Abs(dy)/cellH && dx >= 0:
		a.mark, a.dc = "▶", 1
	case math.Abs(dx)/cellW >= math.Abs(dy)/cellH:
		a.mark, a.dc = "◀", -1
	case dy >= 0:
		a.mark, a.dr = "▼", 1
	default:
		a.mark, a.dr = "▲", -1
	}
	return a, true
}

// arrow draws an arrow once every line is drawn: on the cell before the
// border it points at, the line run on to it, or on the cell before its
// tip when there is no border there.
func (g *grid) arrow(a arrowMark) {
	border := func(k *cell) bool {
		if k == nil || k.glyph != "" {
			return false
		}
		if a.dr != 0 {
			return k.box&(left|right) != 0
		}
		return k.box&(up|down) != 0
	}
	along := func(k *cell) bool {
		if k == nil || k.glyph != "" {
			return false
		}
		if a.dr != 0 {
			return k.box&(up|down) != 0
		}
		return k.box&(left|right) != 0
	}
	c, r := a.c-a.dc, a.r-a.dr // the cell before the tip
	for k := 0; k <= 2; k++ {
		if bc, br := a.c-a.dc+k*a.dc, a.r-a.dr+k*a.dr; border(g.at(bc, br)) {
			g.glyph(bc-a.dc, br-a.dr, a.mark, text.Style{Fg: overlay2})
			return
		}
	}
	// No border to stop at (a diamond, a circle): on the line's last cell.
	for k := 0; k < 2 && along(g.at(c+a.dc, r+a.dr)); k++ {
		c, r = c+a.dc, r+a.dr
	}
	g.glyph(c, r, a.mark, text.Style{Fg: overlay2})
}

// pathLines draws a path's lines: a cubic curve that leaves and arrives
// level (or upright) as an elbow of box-drawing lines, any other curve as
// dots.
func pathLines(g *grid, d string, f frame, st text.Style) {
	var cur, start, ctrl point
	var last byte
	cmds := pathCommands(d)
	for _, cmd := range cmds {
		rel := unicode.IsLower(rune(cmd.op))
		v := cmd.args
		abs := func(p point) point {
			if rel {
				return point{cur.x + p.x, cur.y + p.y}
			}
			return p
		}
		seg := func(a, b point) { g.line(f.m.apply(a), f.m.apply(b), st, f.dashed) }
		switch unicode.ToUpper(rune(cmd.op)) {
		case 'M':
			for i := 0; i+1 < len(v); i += 2 {
				p := abs(point{v[i], v[i+1]})
				if i > 0 {
					seg(cur, p)
				} else {
					start = p
				}
				cur = p
			}
		case 'L':
			for i := 0; i+1 < len(v); i += 2 {
				p := abs(point{v[i], v[i+1]})
				seg(cur, p)
				cur = p
			}
		case 'H':
			for _, x := range v {
				p := point{x, cur.y}
				if rel {
					p.x = cur.x + x
				}
				seg(cur, p)
				cur = p
			}
		case 'V':
			for _, y := range v {
				p := point{cur.x, y}
				if rel {
					p.y = cur.y + y
				}
				seg(cur, p)
				cur = p
			}
		case 'C', 'S':
			n := 6
			if unicode.ToUpper(rune(cmd.op)) == 'S' {
				n = 4
			}
			for i := 0; i+n-1 < len(v); i += n {
				var c1, c2, p point
				if n == 6 {
					c1, c2, p = abs(point{v[i], v[i+1]}), abs(point{v[i+2], v[i+3]}), abs(point{v[i+4], v[i+5]})
				} else {
					c1 = cur
					if last == 'C' || last == 'S' {
						c1 = point{2*cur.x - ctrl.x, 2*cur.y - ctrl.y}
					}
					c2, p = abs(point{v[i], v[i+1]}), abs(point{v[i+2], v[i+3]})
				}
				cubic(g, cur, c1, c2, p, f, st)
				cur, ctrl, last = p, c2, 'C'
			}
			continue
		case 'Q', 'T':
			for i := 0; i+3 < len(v); i += 4 {
				c, p := abs(point{v[i], v[i+1]}), abs(point{v[i+2], v[i+3]})
				prev := cur
				for k := 1; k <= 12; k++ {
					t := float64(k) / 12
					q := point{(1-t)*(1-t)*prev.x + 2*(1-t)*t*c.x + t*t*p.x, (1-t)*(1-t)*prev.y + 2*(1-t)*t*c.y + t*t*p.y}
					seg(cur, q)
					cur = q
				}
			}
		case 'A':
			for i := 0; i+6 < len(v); i += 7 {
				p := abs(point{v[i+5], v[i+6]})
				for _, q := range arcPoints(cur, p, v[i], v[i+1], v[i+2], v[i+3] != 0, v[i+4] != 0) {
					seg(cur, q)
					cur = q
				}
				cur = p
			}
		case 'Z':
			seg(cur, start)
			cur = start
		}
		last = cmd.op
	}
}

// cubic draws a cubic curve: as an elbow when it leaves and arrives level or
// upright, else as dots.
func cubic(g *grid, p0, c1, c2, p3 point, f frame, st text.Style) {
	seg := func(a, b point) { g.line(f.m.apply(a), f.m.apply(b), st, f.dashed) }
	const near = 0.5
	switch {
	case math.Abs(c1.y-p0.y) < near && math.Abs(c2.y-p3.y) < near:
		mx := (c1.x + c2.x) / 2
		seg(p0, point{mx, p0.y})
		seg(point{mx, p0.y}, point{mx, p3.y})
		seg(point{mx, p3.y}, p3)
	case math.Abs(c1.x-p0.x) < near && math.Abs(c2.x-p3.x) < near:
		my := (c1.y + c2.y) / 2
		seg(p0, point{p0.x, my})
		seg(point{p0.x, my}, point{p3.x, my})
		seg(point{p3.x, my}, p3)
	default:
		prev := p0
		for k := 1; k <= 16; k++ {
			t := float64(k) / 16
			u := 1 - t
			q := point{
				u*u*u*p0.x + 3*u*u*t*c1.x + 3*u*t*t*c2.x + t*t*t*p3.x,
				u*u*u*p0.y + 3*u*u*t*c1.y + 3*u*t*t*c2.y + t*t*t*p3.y,
			}
			seg(prev, q)
			prev = q
		}
	}
}

// arcPoints are points along an SVG elliptical arc from a to b (SVG's
// endpoint to centre conversion).
func arcPoints(a, b point, rx, ry, rot float64, large, sweep bool) []point {
	if rx == 0 || ry == 0 {
		return []point{b}
	}
	rx, ry = math.Abs(rx), math.Abs(ry)
	phi := rot * math.Pi / 180
	cos, sin := math.Cos(phi), math.Sin(phi)
	dx, dy := (a.x-b.x)/2, (a.y-b.y)/2
	x1 := cos*dx + sin*dy
	y1 := -sin*dx + cos*dy
	if l := x1*x1/(rx*rx) + y1*y1/(ry*ry); l > 1 {
		rx, ry = rx*math.Sqrt(l), ry*math.Sqrt(l)
	}
	num := rx*rx*ry*ry - rx*rx*y1*y1 - ry*ry*x1*x1
	den := rx*rx*y1*y1 + ry*ry*x1*x1
	k := math.Sqrt(math.Max(num/den, 0))
	if large == sweep {
		k = -k
	}
	cx1, cy1 := k*rx*y1/ry, -k*ry*x1/rx
	cx := cos*cx1 - sin*cy1 + (a.x+b.x)/2
	cy := sin*cx1 + cos*cy1 + (a.y+b.y)/2
	angle := func(ux, uy, vx, vy float64) float64 {
		return math.Atan2(ux*vy-uy*vx, ux*vx+uy*vy)
	}
	t1 := angle(1, 0, (x1-cx1)/rx, (y1-cy1)/ry)
	dt := angle((x1-cx1)/rx, (y1-cy1)/ry, (-x1-cx1)/rx, (-y1-cy1)/ry)
	if !sweep && dt > 0 {
		dt -= 2 * math.Pi
	} else if sweep && dt < 0 {
		dt += 2 * math.Pi
	}
	n := int(math.Abs(dt)*math.Max(rx, ry)/4) + 4
	out := make([]point, 0, n)
	for i := 1; i <= n; i++ {
		t := t1 + dt*float64(i)/float64(n)
		x, y := rx*math.Cos(t), ry*math.Sin(t)
		out = append(out, point{cos*x - sin*y + cx, sin*x + cos*y + cy})
	}
	return out
}

type pathCmd struct {
	op   byte
	args []float64
}

var cmdRe = regexp.MustCompile(`[MmLlHhVvCcSsQqTtAaZz][^MmLlHhVvCcSsQqTtAaZz]*`)

func pathCommands(d string) []pathCmd {
	var out []pathCmd
	for _, s := range cmdRe.FindAllString(d, -1) {
		out = append(out, pathCmd{op: s[0], args: numbers(s[1:])})
	}
	return out
}

// flatten is a path's points, transformed: what a marker's triangle needs.
func flatten(d string, m matrix) []point {
	var out []point
	for _, c := range pathCommands(d) {
		for i := 0; i+1 < len(c.args); i += 2 {
			out = append(out, m.apply(point{c.args[i], c.args[i+1]}))
		}
	}
	return out
}
