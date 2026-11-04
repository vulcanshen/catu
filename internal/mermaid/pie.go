package mermaid

import (
	"cmp"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"

	terminu "github.com/vulcanshen/terminu-go"

	"github.com/vulcanshen/catu/internal/text"
)

// A pie chart is drawn as one bar, not as a circle (user 2026-11-04): a
// grid of cells draws a circle only in dots. The bar is the whole, each
// slice a stretch of it in the colour pele gives that slice, the biggest
// first; under it each colour with its share, then each colour with what it
// stands for.

// pieColours are pele's colours for the slices, in turn.
var pieColours = []terminu.Color{
	terminu.Hex("#4c78a8"), terminu.Hex("#f58518"), terminu.Hex("#54a24b"), terminu.Hex("#e45756"),
	terminu.Hex("#72b7b2"), terminu.Hex("#eeca3b"), terminu.Hex("#b279a2"), terminu.Hex("#9d755d"),
}

var sliceRe = regexp.MustCompile(`^"([^"]*)"\s*:\s*([0-9]*\.?[0-9]+)$`)

type slice struct {
	name   string
	value  float64
	colour terminu.Color
}

type pie struct {
	title  string
	data   bool // showData: each slice's value after what it stands for
	slices []slice
}

// pieOf is src as a pie chart; ok is false when it is not one.
func pieOf(src string) (p pie, ok bool) {
	lines := strings.Split(src, "\n")
	i := firstLine(lines)
	if i == len(lines) {
		return p, false
	}
	head := strings.Fields(strings.TrimSpace(lines[i]))
	if len(head) == 0 || head[0] != "pie" {
		return p, false
	}
	for k := 1; k < len(head); k++ {
		switch head[k] {
		case "showData":
			p.data = true
		case "title":
			p.title = strings.Join(head[k+1:], " ")
			k = len(head)
		}
	}
	block := false // an accDescr { … } over several lines
	for _, l := range lines[i+1:] {
		s := strings.TrimSpace(l)
		switch {
		case block:
			block = !strings.HasSuffix(s, "}")
		case strings.HasPrefix(s, "accDescr") && strings.HasSuffix(s, "{"):
			block = true
		case strings.HasPrefix(s, "title "):
			p.title = strings.TrimSpace(s[len("title "):])
		case sliceRe.MatchString(s):
			m := sliceRe.FindStringSubmatch(s)
			v, _ := strconv.ParseFloat(m[2], 64)
			p.slices = append(p.slices, slice{m[1], v, pieColours[len(p.slices)%len(pieColours)]})
		}
	}
	slices.SortStableFunc(p.slices, func(a, b slice) int { return cmp.Compare(b.value, a.value) })
	return p, len(p.slices) > 0
}

// draw is the chart in at most w cells: the title in the middle and a blank
// row, the bar, the shares, then a row a slice with what it stands for.
func (p pie) draw(w int) [][]text.Char {
	total := 0.0
	for _, s := range p.slices {
		total += s.value
	}
	const indent = 2
	pad := func() []text.Char { return text.Spaces(indent, text.Style{}) }
	barW := min(max(w-2*indent, 4), 60)
	var rows [][]text.Char
	if p.title != "" {
		title := text.Escape(p.title, text.Style{Fg: terminu.Text, Bold: true})
		across := indent + barW
		title = fit(title, min(width(title), across))
		rows = append(rows, append(text.Spaces((across-width(title))/2, text.Style{}), title...), nil)
	}
	bar := pad()
	for i, n := range stretches(p.slices, total, barW) {
		bar = append(bar, text.Chars(strings.Repeat("█", n), text.Style{Fg: p.slices[i].colour})...)
	}
	rows = append(rows, bar)
	// Each colour with its share, two cells apart (tdp components.2), on as
	// many rows as they take.
	shares := pad()
	for i, s := range p.slices {
		share := 0.0
		if total > 0 {
			share = s.value / total * 100
		}
		item := append(text.Chars("■", text.Style{Fg: s.colour}),
			text.Chars(" "+strconv.Itoa(int(math.Round(share)))+"%", text.Style{Fg: terminu.Text})...)
		switch {
		case i > 0 && width(shares)+2+width(item) > w:
			rows, shares = append(rows, shares), pad()
		case i > 0:
			shares = append(shares, text.Spaces(2, text.Style{})...)
		}
		shares = append(shares, item...)
	}
	rows = append(rows, shares)
	// Each colour with what it stands for: the file's own words, a control
	// character in them by its name in Red (tdp components.5).
	for _, s := range p.slices {
		row := append(pad(), text.Chars("■ ", text.Style{Fg: s.colour})...)
		var data []text.Char
		if p.data {
			data = text.Chars("  ["+value(s.value)+"]", text.Style{Fg: subtext0})
		}
		name := text.Escape(s.name, text.Style{Fg: terminu.Text})
		name = fit(name, min(width(name), max(w-width(row)-width(data), 1)))
		rows = append(rows, append(append(row, name...), data...))
	}
	return rows
}

// stretches are the cells of w each slice takes as its share of total: a
// slice that is not nothing a cell at least, the cells rounding left over to
// the ones it shortened most; cells given to the smallest come off the
// biggest.
func stretches(sl []slice, total float64, w int) []int {
	n := make([]int, len(sl))
	if total <= 0 {
		return n
	}
	left := make([]float64, len(sl))
	used := 0
	for i, s := range sl {
		exact := s.value / total * float64(w)
		n[i], left[i] = int(exact), exact-math.Floor(exact)
		if s.value > 0 && n[i] == 0 {
			n[i], left[i] = 1, -1
		}
		used += n[i]
	}
	for ; used < w; used++ {
		most := 0
		for i := range left {
			if left[i] > left[most] {
				most = i
			}
		}
		n[most]++
		left[most] = -1
	}
	for ; used > w && n[0] > 1; used-- {
		n[0]--
	}
	return n
}

func width(chars []text.Char) int {
	w := 0
	for _, c := range chars {
		w += c.W
	}
	return w
}

// fit is chars in exactly w cells: cut, a … last, when they are wider,
// spaces after them when they are narrower.
func fit(chars []text.Char, w int) []text.Char {
	if used := width(chars); used <= w {
		return append(chars, text.Spaces(w-used, text.Style{})...)
	}
	var out []text.Char
	used := 0
	for _, c := range chars {
		if used+c.W > w-1 {
			break
		}
		out, used = append(out, c), used+c.W
	}
	out = append(out, text.Char{Draw: "…", W: 1, St: chars[0].St, Src: "…"})
	return append(out, text.Spaces(w-used-1, text.Style{})...)
}

// value is a slice's value as it was written, give or take its zeros.
func value(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }
