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

// A pie chart is drawn as bars, not as a circle (user 2026-11-04): a grid
// of cells draws a circle only in dots. A slice a row, the biggest first:
// its name, a bar as long as it is next to the biggest, in the colour pele
// gives it, and its share of the whole.

// pieColours are pele's colours for the slices, in turn.
var pieColours = []terminu.Color{
	terminu.Hex("#4c78a8"), terminu.Hex("#f58518"), terminu.Hex("#54a24b"), terminu.Hex("#e45756"),
	terminu.Hex("#72b7b2"), terminu.Hex("#eeca3b"), terminu.Hex("#b279a2"), terminu.Hex("#9d755d"),
}

var sliceRe = regexp.MustCompile(`^"([^"]*)"\s*:\s*([0-9]*\.?[0-9]+)$`)

// eighths are a cell's left eighths, one to seven.
var eighths = []string{"▏", "▎", "▍", "▌", "▋", "▊", "▉"}

type slice struct {
	name   string
	value  float64
	colour terminu.Color
}

type pie struct {
	title  string
	data   bool // showData: each slice's value after its share
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

// draw is the chart in at most w cells: the title in the middle, a blank
// row, then a row a slice.
func (p pie) draw(w int) [][]text.Char {
	// The names and the title are the file's: a control character in them
	// is drawn by its name, in Red (tdp components.5).
	plain := text.Style{Fg: terminu.Text}
	names := make([][]text.Char, len(p.slices))
	total, most := 0.0, 0.0
	nameW, dataW := 0, 0
	for i, s := range p.slices {
		total += s.value
		most = math.Max(most, s.value)
		names[i] = text.Escape(s.name, plain)
		nameW = max(nameW, width(names[i]))
		dataW = max(dataW, len(value(s.value))+2)
	}
	if !p.data {
		dataW = 0
	}
	nameW = min(nameW, max(w/3, 4))
	const pctW = 4 // "100%"
	gaps := 2 + 2 + 2
	if p.data {
		gaps += 2
	}
	barW := min(max(w-nameW-pctW-dataW-gaps, 4), 48)
	var rows [][]text.Char
	if p.title != "" {
		title := text.Escape(p.title, text.Style{Fg: terminu.Text, Bold: true})
		across := 2 + nameW + 2 + barW + 2 + pctW
		title = fit(title, min(width(title), across))
		rows = append(rows, append(text.Spaces((across-width(title))/2, text.Style{}), title...), nil)
	}
	for i, s := range p.slices {
		row := append(text.Spaces(2, text.Style{}), fit(names[i], nameW)...)
		row = append(row, text.Spaces(2, text.Style{})...)
		row = append(row, text.Chars(bar(s.value, most, barW), text.Style{Fg: s.colour})...)
		share := 0.0
		if total > 0 {
			share = s.value / total * 100
		}
		row = append(row, text.Chars("  "+terminu.PadLeft(strconv.Itoa(int(math.Round(share)))+"%", pctW), text.Style{Fg: terminu.Text})...)
		if p.data {
			row = append(row, text.Chars("  ["+value(s.value)+"]", text.Style{Fg: subtext0})...)
		}
		rows = append(rows, row)
	}
	return rows
}

// bar is v next to most as eighths of w cells, a slice that is not nothing
// an eighth at least, padded to w.
func bar(v, most float64, w int) string {
	n := 0
	if most > 0 {
		n = int(math.Round(v / most * float64(w*8)))
	}
	if v > 0 {
		n = max(n, 1)
	}
	s := strings.Repeat("█", n/8)
	if n%8 > 0 {
		s += eighths[n%8-1]
	}
	return terminu.Pad(s, w)
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
