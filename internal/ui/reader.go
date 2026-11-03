package ui

import (
	"strconv"
	"strings"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
	terminu "github.com/vulcanshen/terminu-go"

	"github.com/vulcanshen/viewu/internal/text"
)

// reader is the panel's body: what is shown, a line at a time, folded at the
// panel's width and scrolled a row at a time as less does; a search's hits;
// the Visual mode's cursor and selection (tdp note.2, note.3, note.4,
// panel.14). The first row is the position row, the path (tdp panel.10).
type reader struct {
	where string // the position row; "" for none
	lines []text.Line
	// numbered draws the lines' numbers in a column of their own.
	numbered bool
	// layout, when set, lays the lines out at a width (a document, a
	// diagram): it is asked again whenever the width changes.
	layout func(w int) []text.Line
	laidW  int
	// fact and hint say why there is nothing to show, in the middle (tdp
	// screen.8).
	fact, hint string

	textW, numW int
	top, sub    int // the line the first row in sight belongs to, and that row among its rows
	visible     int // the rows of text in sight
	scroll      string
	folds       [][]int // each line's rows, where they start, at textW; nil until asked

	hits    []hit
	current int

	visual, selecting bool
	line, col         int // the cursor
	fromL, fromC      int // where the selection started
}

// hit is a stretch of a line a search found: its characters from to to, to
// not in.
type hit struct{ line, from, to int }

// set shows lines; numbered draws their numbers. It goes back to the top,
// with no hits and no cursor.
func (r *reader) set(lines []text.Line, numbered bool) {
	r.lines, r.numbered, r.layout, r.laidW = lines, numbered, nil, 0
	r.reset()
}

// setLayout shows what layout lays out at the panel's width.
func (r *reader) setLayout(layout func(w int) []text.Line) {
	r.lines, r.numbered, r.layout, r.laidW = nil, false, layout, -1
	r.reset()
}

func (r *reader) reset() {
	r.top, r.sub, r.folds = 0, 0, nil
	r.hits, r.current = nil, 0
	r.visual, r.selecting = false, false
	r.fact, r.hint = "", ""
	r.numW = 0
}

// Empty is false: the position row stays when there is nothing to show (tdp
// panel.10), so the reader draws its own empty state.
func (r *reader) Empty() bool { return false }

// HasItem is false: a text has no items (tdp K3).
func (r *reader) HasItem() bool { return false }

func (r *reader) Scroll() string { return r.scroll }

// MoveKeys are the scrolling keys; none when everything is in sight (tdp M6).
func (r *reader) MoveKeys() []terminu.KeyRow {
	if r.fits() {
		return nil
	}
	return []terminu.KeyRow{
		{Key: "j|k", Desc: "scroll down, up"},
		{Key: "u|d", Desc: "half a page up, down"},
		{Key: "gg|G", Desc: "to the top, the bottom"},
	}
}

// fits reports that every row is in sight.
func (r *reader) fits() bool {
	if r.visible == 0 {
		return false
	}
	rows := 0
	for i := range r.lines {
		if rows += len(r.wrap(i)); rows > r.visible {
			return false
		}
	}
	return true
}

// Key scrolls (tdp note.3, K12).
func (r *reader) Key(key string, _ tea.KeyMsg) (bool, tea.Cmd) {
	half := max(r.visible/2, 1)
	switch key {
	case "j", "↓":
		r.scrollRows(1)
	case "k", "↑":
		r.scrollRows(-1)
	case "d", "PgDn":
		r.scrollRows(half)
	case "u", "PgUp":
		r.scrollRows(-half)
	case "gg":
		r.top, r.sub = 0, 0
	case "G":
		r.top, r.sub = len(r.lines), 0
		r.clampTop()
	default:
		return false, nil
	}
	return true, nil
}

// Draw draws the position row, then the rows in sight: each line's first row
// with its number when numbered, the rows it folds onto with the column
// blank (tdp note.2).
func (r *reader) Draw(w, h int, focused bool) []string {
	out := make([]string, 0, h)
	if r.where != "" && h > 0 {
		where := terminu.Fg(terminu.Lavender).Render(terminu.TruncateLeft(terminu.Escape(r.where), max(w-2, 0)))
		out = append(out, " "+terminu.Pad(where, max(w-2, 0))+" ")
	}
	r.visible = max(h-len(out), 0)
	r.measure(w)
	if len(r.lines) == 0 {
		r.scroll = ""
		return append(out, r.emptyRows(w, r.visible)...)
	}
	if r.visual {
		r.line = min(max(r.line, 0), len(r.lines)-1)
		r.follow(r.line)
	}
	r.clampTop()
	last, whole := r.top-1, true
	for i := r.top; i < len(r.lines) && len(out) < h; i++ {
		starts := r.wrap(i)
		first := 0
		if i == r.top {
			first = r.sub
		}
		for k := first; k < len(starts); k++ {
			if len(out) == h {
				whole = false
				break
			}
			to := len(r.lines[i].Chars)
			if k+1 < len(starts) {
				to = starts[k+1]
			}
			out = append(out, " "+r.number(i, k)+r.row(i, starts[k], to, k+1 == len(starts))+" ")
		}
		last = i
	}
	for len(out) < h {
		out = append(out, strings.Repeat(" ", w))
	}
	r.scroll = ""
	if r.top > 0 || r.sub > 0 || last < len(r.lines)-1 || !whole {
		if r.visual {
			r.scroll = strconv.Itoa(r.line+1) + " of " + strconv.Itoa(len(r.lines))
		} else {
			r.scroll = strconv.Itoa(r.top+1) + "-" + strconv.Itoa(last+1) + " of " + strconv.Itoa(len(r.lines))
		}
	}
	return out
}

// measure works out the text's width in w cells: a cell of padding either
// side, the numbers and the space after them; content laid out at a width
// is laid out again when it changed.
func (r *reader) measure(w int) {
	inner := max(w-2, 1)
	if r.layout != nil {
		if r.laidW != inner {
			top := r.top
			r.lines, r.laidW, r.folds = r.layout(inner), inner, nil
			r.top, r.sub = min(top, max(len(r.lines)-1, 0)), 0
			r.hits = nil
			if r.visual {
				r.visual, r.selecting = false, false
			}
		}
		r.textW, r.numW = inner, 0
		return
	}
	textW := inner
	if r.numbered {
		r.numW = max(r.numW, len(strconv.Itoa(len(r.lines))), 3)
		textW = max(inner-1-r.numW, 1)
	}
	if textW != r.textW {
		r.textW, r.folds = textW, nil
	}
}

// number is the number column of row k of line i: the number on its first
// row, Overlay0, the cursor's line in Text (tdp note.2, note.4).
func (r *reader) number(i, k int) string {
	if !r.numbered {
		return ""
	}
	if k > 0 || r.lines[i].Num == 0 {
		return strings.Repeat(" ", r.numW+1)
	}
	c := terminu.Overlay0
	if r.visual && i == r.line {
		c = terminu.Text
	}
	return terminu.Fg(c).Render(terminu.PadLeft(strconv.Itoa(r.lines[i].Num), r.numW)) + " "
}

// Styles of what is drawn over the text (tdp color): the cursor block of a
// panel, Subtext1; the selection, the mode's Yellow; a control character's
// name on a block, Base on Red.
var (
	cursorStyle = text.Style{Fg: terminu.Base, Bg: terminu.Subtext1, Bold: true}
	selectStyle = text.Style{Fg: terminu.Base, Bg: terminu.Yellow}
	nameOnBlock = text.Style{Fg: terminu.Base, Bg: terminu.Red}
)

// row is the characters from to to of line i, textW cells: the hits
// underlined, the current one and the cursor blocks, the selection Yellow; a
// fixed line too wide is cut, a … in Overlay0 at its end.
func (r *reader) row(i, from, to int, lastRow bool) string {
	chars := r.lines[i].Chars
	sel0, sel1 := -1, -1
	if r.selecting {
		s, e := r.selection()
		if i >= s.line && i <= e.line {
			sel0, sel1 = 0, len(chars)
			if i == s.line {
				sel0 = s.col
			}
			if i == e.line {
				sel1 = min(e.col+1, len(chars))
			}
		}
	}
	mark := make(map[int]int) // 1 a hit, 2 the current one
	for n, h := range r.hits {
		if h.line != i {
			continue
		}
		for k := max(h.from, from); k < min(h.to, to); k++ {
			mark[k] = 1
			if n == r.current {
				mark[k] = 2
			}
		}
	}
	out := make([]text.Char, 0, to-from+1)
	used := 0
	for k := from; k < to; k++ {
		c := chars[k]
		block := func(st text.Style) {
			if c.Src != c.Draw && strings.HasPrefix(c.Draw, `\`) {
				st = nameOnBlock
			}
			c.St = st
		}
		switch {
		case r.visual && i == r.line && k == r.col:
			block(cursorStyle)
		case k >= sel0 && k < sel1:
			block(selectStyle)
		case mark[k] == 2:
			block(cursorStyle)
		case mark[k] == 1:
			c.St.Underline = true
		}
		if used+c.W > r.textW {
			break
		}
		out = append(out, c)
		used += c.W
	}
	if r.lines[i].Fixed && r.lines[i].Width() > r.textW && r.textW > 0 {
		for used+1 > r.textW && len(out) > 0 {
			used -= out[len(out)-1].W
			out = out[:len(out)-1]
		}
		out = append(out, text.Char{Draw: "…", W: 1, St: text.Style{Fg: terminu.Overlay0}})
		used++
	}
	if lastRow && used < r.textW {
		switch {
		case r.visual && i == r.line && r.col >= len(chars):
			out = append(out, text.Char{Draw: " ", W: 1, St: cursorStyle}) // on an empty line the cursor still shows
			used++
		case len(chars) == 0 && sel0 >= 0:
			out = append(out, text.Char{Draw: " ", W: 1, St: selectStyle})
			used++
		}
	}
	return text.Render(out) + strings.Repeat(" ", max(r.textW-used, 0))
}

// emptyRows is the middle of an empty panel: the fact, then the hint, each
// folded and centred, a blank row between them, in Overlay0 (tdp screen.8).
func (r *reader) emptyRows(w, h int) []string {
	inner := max(w-2, 1)
	var rows []string
	for _, l := range terminu.Wrap(r.fact, inner) {
		rows = append(rows, terminu.Fg(terminu.Overlay0).Render(l))
	}
	if r.hint != "" {
		rows = append(rows, "")
		for _, l := range terminu.Wrap(r.hint, inner) {
			rows = append(rows, terminu.Fg(terminu.Overlay0).Render(l))
		}
	}
	out := make([]string, h)
	top := max((h-len(rows))/2, 0)
	for i := range out {
		line := ""
		if k := i - top; k >= 0 && k < len(rows) {
			line = rows[k]
		}
		out[i] = " " + terminu.Center(line, inner) + " "
	}
	return out
}

// wrap is where line i's rows start, as indices into its characters: a line
// too long for a row folds at a space, a word longer than a row cut by cells
// (tdp components.3); a fixed line is one row.
func (r *reader) wrap(i int) []int {
	if len(r.folds) != len(r.lines) {
		r.folds = make([][]int, len(r.lines))
	}
	if r.folds[i] != nil {
		return r.folds[i]
	}
	l := r.lines[i]
	if l.Fixed || len(l.Chars) == 0 {
		r.folds[i] = []int{0}
		return r.folds[i]
	}
	cells, space := make([]int, len(l.Chars)), make([]bool, len(l.Chars))
	for k, c := range l.Chars {
		cells[k], space[k] = c.W, c.Src == " " || c.Src == "\t"
	}
	r.folds[i] = foldAt(cells, space, max(r.textW, 1))
	return r.folds[i]
}

// foldAt is where the rows of a line start when it folds at w cells keeping
// every character, as terminu-go's viewer folds (tdp components.3): after a
// space or on either side of a wide character; inside a word only when it is
// longer than a row, a wide character never cut in half. A space that does
// not fit starts the next row.
func foldAt(cells []int, space []bool, w int) []int {
	wide := func(i int) bool { return cells[i] > 1 && !space[i] }
	starts := []int{0}
	start, used, brk := 0, 0, 0
	for i, cw := range cells {
		if i > start && (space[i-1] && !space[i] || wide(i) || wide(i-1)) {
			brk = i
		}
		if used+cw > w && i > start {
			next := i
			if !space[i] && brk > start {
				next = brk
			}
			used = 0
			for _, c := range cells[next:i] {
				used += c
			}
			starts, start = append(starts, next), next
		}
		used += cw
	}
	return starts
}

// maxTop is the last row that can be at the top: from there the rows to the
// end fill the view.
func (r *reader) maxTop() (line, sub int) {
	rows := 0
	for t := len(r.lines) - 1; t >= 0; t-- {
		n := len(r.wrap(t))
		if rows+n >= r.visible {
			return t, rows + n - r.visible
		}
		rows += n
	}
	return 0, 0
}

func (r *reader) clampTop() {
	if r.top < 0 || len(r.lines) == 0 {
		r.top, r.sub = 0, 0
		return
	}
	if t, s := r.maxTop(); r.top > t || r.top == t && r.sub > s {
		r.top, r.sub = t, s
	}
	r.sub = min(max(r.sub, 0), len(r.wrap(r.top))-1)
}

// scrollRows scrolls n rows, down when n is positive (tdp note.3).
func (r *reader) scrollRows(n int) {
	r.clampTop()
	for ; n > 0; n-- {
		if r.sub+1 < len(r.wrap(r.top)) {
			r.sub++
		} else if r.top+1 < len(r.lines) {
			r.top, r.sub = r.top+1, 0
		}
	}
	for ; n < 0; n++ {
		if r.sub > 0 {
			r.sub--
		} else if r.top > 0 {
			r.top--
			r.sub = len(r.wrap(r.top)) - 1
		}
	}
	r.clampTop()
}

// follow scrolls as little as it takes to show line whole.
func (r *reader) follow(line int) {
	if line < r.top || line == r.top && r.sub > 0 {
		r.top, r.sub = line, 0
		return
	}
	t, rows := line, 0
	for t >= 0 {
		if rows += len(r.wrap(t)); rows > r.visible {
			break
		}
		t--
	}
	if top := max(r.top, min(t+1, line)); top != r.top {
		r.top, r.sub = top, 0
	}
}

// --- search (tdp panel.14, note.3) -------------------------------------

// find makes every place q is a hit, in either case; it reports how many.
// The first is scrolled into sight.
func (r *reader) find(q string) int {
	r.hits, r.current = nil, 0
	want := []rune(strings.ToLower(q))
	if len(want) == 0 {
		return 0
	}
	for i, l := range r.lines {
		low := make([]string, len(l.Chars))
		for k, c := range l.Chars {
			low[k] = strings.ToLower(c.Src)
		}
		for k := 0; k < len(low); k++ {
			if end, ok := matchAt(low, k, want); ok {
				r.hits = append(r.hits, hit{i, k, end})
				k = end - 1
			}
		}
	}
	if len(r.hits) > 0 {
		r.follow(r.hits[0].line)
	}
	return len(r.hits)
}

// matchAt reports whether want starts at character k, and the character
// after it.
func matchAt(chars []string, k int, want []rune) (int, bool) {
	rest := want
	for ; k < len(chars) && len(rest) > 0; k++ {
		c := []rune(chars[k])
		if len(c) == 0 || len(c) > len(rest) || string(rest[:len(c)]) != string(c) {
			return 0, false
		}
		rest = rest[len(c):]
	}
	return k, len(rest) == 0
}

// step goes by hits, wrapping at the ends, scrolling only when the hit is
// out of sight.
func (r *reader) step(by int) {
	if n := len(r.hits); n > 0 {
		r.current = ((r.current+by)%n + n) % n
		r.follow(r.hits[r.current].line)
	}
}

// --- the Visual mode (tdp note.4) ---------------------------------------

type pos struct{ line, col int }

// startVisual brings the cursor out at the start of the first line in
// sight; nothing is selected yet.
func (r *reader) startVisual() {
	r.visual, r.selecting = true, false
	r.line, r.col = min(max(r.top, 0), max(len(r.lines)-1, 0)), 0
}

// startSelect starts the selection where the cursor is.
func (r *reader) startSelect() {
	r.selecting = true
	r.fromL, r.fromC = r.line, r.col
}

func (r *reader) leave()    { r.visual, r.selecting = false, false }
func (r *reader) unselect() { r.selecting = false }

func (r *reader) selection() (pos, pos) {
	s, e := pos{r.fromL, r.fromC}, pos{r.line, r.col}
	if e.line < s.line || e.line == s.line && e.col < s.col {
		s, e = e, s
	}
	return s, e
}

// selected is the text from where the selection started to the cursor, the
// character under the cursor in, as a copy gives it; the spaces a laid-out
// row is filled with are left out.
func (r *reader) selected() string {
	s, e := r.selection()
	var out []string
	for i := s.line; i <= e.line && i < len(r.lines); i++ {
		chars := r.lines[i].Chars
		from, to := 0, len(chars)
		if i == s.line {
			from = min(s.col, len(chars))
		}
		if i == e.line {
			to = min(e.col+1, len(chars))
		}
		var b strings.Builder
		for _, c := range chars[from:max(to, from)] {
			b.WriteString(c.Src)
		}
		line := b.String()
		if r.lines[i].Fixed {
			line = strings.TrimRight(line, " ")
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

func charClass(c string) int {
	if c == "" {
		return 0
	}
	ch := []rune(c)[0]
	switch {
	case unicode.IsSpace(ch):
		return 0
	case unicode.IsLetter(ch) || unicode.IsDigit(ch) || ch == '_':
		return 1
	}
	return 2
}

func (r *reader) classAt(p pos) int {
	if p.line >= len(r.lines) || p.col >= len(r.lines[p.line].Chars) {
		return 0
	}
	return charClass(r.lines[p.line].Chars[p.col].Src)
}

func (r *reader) next(p pos) (pos, bool) {
	if p.col+1 < len(r.lines[p.line].Chars) {
		return pos{p.line, p.col + 1}, true
	}
	if p.line+1 < len(r.lines) {
		return pos{p.line + 1, 0}, true
	}
	return p, false
}

func (r *reader) prev(p pos) (pos, bool) {
	if p.col > 0 {
		return pos{p.line, p.col - 1}, true
	}
	if p.line > 0 {
		return pos{p.line - 1, max(len(r.lines[p.line-1].Chars)-1, 0)}, true
	}
	return p, false
}

// move moves the cursor through the text as vim does, as terminu-go's viewer
// moves its own (tdp K12): h|j|k|l, w|b|e, 0|$, u|d, gg|G; it does not wrap
// at the ends.
func (r *reader) move(key string) {
	if len(r.lines) == 0 {
		return
	}
	half := max(r.visible/2, 1)
	lineLen := func() int { return len(r.lines[r.line].Chars) }
	p := pos{r.line, r.col}
	switch key {
	case "h", "←":
		r.col = max(r.col-1, 0)
	case "l", "→":
		r.col = min(r.col+1, max(lineLen()-1, 0))
	case "j", "↓":
		r.line = min(r.line+1, len(r.lines)-1)
	case "k", "↑":
		r.line = max(r.line-1, 0)
	case "d", "PgDn":
		r.line = min(r.line+half, len(r.lines)-1)
	case "u", "PgUp":
		r.line = max(r.line-half, 0)
	case "gg":
		r.line, r.col = 0, 0
	case "G":
		r.line = len(r.lines) - 1
	case "0":
		r.col = 0
	case "$":
		r.col = max(lineLen()-1, 0)
	case "w":
		c, ok := r.classAt(p), true
		for ok && c != 0 && r.classAt(p) == c {
			p, ok = r.next(p)
		}
		for ok && r.classAt(p) == 0 {
			p, ok = r.next(p)
		}
		r.line, r.col = p.line, p.col
	case "b":
		p, ok := r.prev(p)
		for ok && r.classAt(p) == 0 {
			p, ok = r.prev(p)
		}
		for c := r.classAt(p); ok; {
			q, more := r.prev(p)
			if !more || r.classAt(q) != c || q.line != p.line {
				break
			}
			p = q
		}
		r.line, r.col = p.line, p.col
	case "e":
		p, ok := r.next(p)
		for ok && r.classAt(p) == 0 {
			p, ok = r.next(p)
		}
		for c := r.classAt(p); ok; {
			q, more := r.next(p)
			if !more || r.classAt(q) != c || q.line != p.line {
				break
			}
			p = q
		}
		r.line, r.col = p.line, p.col
	}
	r.col = min(r.col, max(lineLen()-1, 0))
	r.follow(r.line)
}
