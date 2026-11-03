// Package doc lays Markdown out as a document at a width, the way webu draws
// a page (webu docs/reader.md, the elements): no Markdown symbols on the
// screen — a heading a chip of its level's colour, a list item its marker,
// code and quotes on Mantle, a link and an image an icon before their words,
// a table a grid.
package doc

import (
	"path"
	"strconv"
	"strings"

	"github.com/rivo/uniseg"
	terminu "github.com/vulcanshen/terminu-go"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	east "github.com/yuin/goldmark/extension/ast"
	gtext "github.com/yuin/goldmark/text"

	"github.com/vulcanshen/viewu/internal/text"
)

// Glyphs and colours of the elements (webu docs/reader.md; the content's
// own, tdp P4, P5).
var (
	glyphLink  = "\U000F0337" // nf-md-link
	glyphImage = "\U000F02E9" // nf-md-image

	mantle   = terminu.Mantle
	surface0 = terminu.Hex("#313244")
	pink     = terminu.Hex("#f5c2e7") // code in a line
	sapphire = terminu.Hex("#74c7ec") // a link: press it
	// levels are the colours of a level, one each: a heading's by its level,
	// a quote's bar by how deep the quote is. Markdown has six levels.
	levels = []terminu.Color{
		terminu.Hex("#fab387"), // peach
		terminu.Hex("#f5e0dc"), // rosewater
		terminu.Hex("#a6e3a1"), // green
		terminu.Hex("#94e2d5"), // teal
		terminu.Hex("#f2cdcd"), // flamingo
		terminu.Hex("#cba6f7"), // mauve
	}
)

// level is the colour of level n, counting from 1; past the sixth (a quote
// that deep) they come round again.
func level(n int) terminu.Color { return levels[(max(n, 1)-1)%len(levels)] }

// Diagram draws a diagram's source (mermaid) at most w cells wide: its rows,
// or why it could not.
type Diagram func(src string, w int) ([][]text.Char, error)

// Options are what the layout needs besides the Markdown.
type Options struct {
	TabW int
	// Diagram, when set, draws a code block in mermaid as its diagram.
	Diagram Diagram
	// OnDiagramError, when set, is told why a diagram could not be drawn.
	OnDiagramError func(error)
}

var md = goldmark.New(goldmark.WithExtensions(extension.Table, extension.Strikethrough, extension.TaskList, extension.Linkify))

// Render lays src out at w cells: rows of characters, each a fixed line
// exactly w wide.
func Render(src []byte, w int, opt Options) []text.Line {
	r := &renderer{w: max(w, 1), opt: opt, src: src}
	if r.opt.TabW < 1 {
		r.opt.TabW = 4
	}
	doc := md.Parser().Parse(gtext.NewReader(src))
	r.blocks(doc, &ctx{})
	for len(r.rows) > 0 && strings.TrimSpace(r.rows[len(r.rows)-1].Drawn()) == "" {
		r.rows = r.rows[:len(r.rows)-1]
	}
	return r.rows
}

type renderer struct {
	w    int
	opt  Options
	src  []byte
	rows []text.Line
}

// ctx is what a block's rows start with — head on the first, hang on the
// rest — and the ground they are on; a list's blocks have no blank rows
// between them. The depths are how deep in lists and quotes it is.
type ctx struct {
	head, hang             []text.Char
	used                   *bool
	ground                 terminu.Color
	tight                  bool
	udepth, odepth, qdepth int
}

func (c *ctx) next() []text.Char {
	if c.used == nil || *c.used {
		return c.hang
	}
	*c.used = true
	return c.head
}

func (c *ctx) inner() ctx {
	return ctx{hang: c.hang, ground: c.ground, tight: c.tight, udepth: c.udepth, odepth: c.odepth, qdepth: c.qdepth}
}

func width(cs []text.Char) int {
	w := 0
	for _, c := range cs {
		w += c.W
	}
	return w
}

func trimRight(cs []text.Char) []text.Char {
	for len(cs) > 0 && cs[len(cs)-1].Draw == " " {
		cs = cs[:len(cs)-1]
	}
	return cs
}

func clone(cs []text.Char) []text.Char { return append([]text.Char(nil), cs...) }

// row adds cs as a row on ground: what has no ground of its own takes it,
// and the row is filled to the width with it.
func (r *renderer) row(cs []text.Char, ground terminu.Color) {
	cs = clone(cs)
	for i := range cs {
		if !cs[i].St.Bg.Set {
			cs[i].St.Bg = ground
		}
	}
	if w := width(cs); w < r.w {
		cs = append(cs, text.Spaces(r.w-w, text.Style{Bg: ground})...)
	}
	r.rows = append(r.rows, text.Line{Chars: cs, Fixed: true})
}

// blank is an empty row between blocks, once, with the bars of the quote
// it is in.
func (r *renderer) blank(c *ctx) {
	if c.tight || len(r.rows) == 0 {
		return
	}
	// A row of nothing but a quote's bars is a blank one already.
	if strings.Trim(r.rows[len(r.rows)-1].Drawn(), "▎ ") == "" {
		return
	}
	r.row(trimRight(c.hang), c.ground)
}

// lineBreak marks a hard line break in a run of inline characters.
const lineBreak = "\n"

// flow wraps a run of inline characters into rows: at a space or beside a
// wide character, a word longer than a row cut; the first row after the
// block's head, the rest after its hang. chip, when set, puts each row's
// words on a block of that colour, a space either side: a heading.
func (r *renderer) flow(cs []text.Char, c *ctx, chip terminu.Color) {
	room := r.w
	if chip.Set {
		room -= 2
	}
	for len(cs) > 0 && cs[0].Draw == " " {
		cs = cs[1:]
	}
	cs = trimRight(cs)
	if len(cs) == 0 {
		return
	}
	put := func(cur []text.Char, start int) {
		cur = trimRight(cur)
		if chip.Set {
			on := text.Style{Fg: terminu.Base, Bg: chip, Bold: true}
			words := clone(cur[start:])
			for i := range words {
				words[i].St.Fg, words[i].St.Bg, words[i].St.Bold = terminu.Base, chip, true
			}
			cur = append(append(clone(cur[:start]), text.Spaces(1, on)...), words...)
			cur = append(cur, text.Spaces(1, on)...)
		}
		r.row(cur, c.ground)
	}
	cur := clone(c.next())
	start := len(cur)
	w, brk := width(cur), -1
	emit := func() {
		put(cur, start)
		cur = clone(c.hang)
		start = len(cur)
		w, brk = width(cur), -1
	}
	for _, x := range cs {
		if x.Draw == lineBreak {
			emit()
			continue
		}
		if w+x.W > room && len(cur) > start {
			cut := len(cur)
			if brk > start && brk < len(cur) {
				cut = brk
			}
			rest := clone(cur[cut:])
			cur = cur[:cut]
			emit()
			for len(rest) > 0 && rest[0].Draw == " " {
				rest = rest[1:]
			}
			cur = append(cur, rest...)
			w = width(cur)
		}
		if x.Draw == " " && len(cur) == start {
			continue
		}
		cur = append(cur, x)
		w += x.W
		if x.Draw == " " || x.W > 1 {
			brk = len(cur)
		}
	}
	if len(cur) > start {
		put(cur, start)
	}
}

// blocks draws n's children: a run of inline ones gathered into one flow,
// each block on rows of its own.
func (r *renderer) blocks(n ast.Node, c *ctx) {
	var run []text.Char
	flush := func() {
		if strings.TrimSpace(plainOf(run)) != "" {
			r.blank(c)
			r.flow(run, c, terminu.Color{})
		}
		run = nil
	}
	for k := n.FirstChild(); k != nil; k = k.NextSibling() {
		if k.Type() == ast.TypeBlock {
			flush()
			r.block(k, c)
			continue
		}
		run = r.inline(run, k, text.Style{Fg: terminu.Text})
	}
	flush()
}

func plainOf(cs []text.Char) string {
	var b strings.Builder
	for _, c := range cs {
		b.WriteString(c.Src)
	}
	return b.String()
}

func (r *renderer) block(n ast.Node, c *ctx) {
	switch n := n.(type) {
	case *ast.Heading:
		r.blank(c)
		var run []text.Char
		for k := n.FirstChild(); k != nil; k = k.NextSibling() {
			run = r.inline(run, k, text.Style{Fg: terminu.Text, Bold: true})
		}
		r.flow(run, c, level(n.Level))
	case *ast.Paragraph, *ast.TextBlock:
		r.blank(c)
		var run []text.Char
		for k := n.FirstChild(); k != nil; k = k.NextSibling() {
			run = r.inline(run, k, text.Style{Fg: terminu.Text})
		}
		r.flow(run, c, terminu.Color{})
	case *ast.List:
		r.list(n, c)
	case *ast.FencedCodeBlock:
		lang := string(n.Language(r.src))
		r.code(r.lines(n), lang, c)
	case *ast.CodeBlock:
		r.code(r.lines(n), "", c)
	case *ast.HTMLBlock:
		r.blank(c)
		for _, l := range strings.Split(strings.TrimRight(r.lines(n), "\n"), "\n") {
			r.flow(text.Escape(l, text.Style{Fg: terminu.Overlay0}), c, terminu.Color{})
		}
	case *ast.Blockquote:
		r.blank(c)
		// The bar is in the colour of the quote's depth, a heading's colours:
		// each bar of a quote in a quote its own.
		depth := c.qdepth + 1
		bar := append(text.Chars("▎", text.Style{Fg: level(depth)}), text.Spaces(1, text.Style{})...)
		used := false
		in := ctx{head: append(clone(c.next()), bar...), hang: append(clone(c.hang), bar...),
			used: &used, ground: mantle, udepth: c.udepth, odepth: c.odepth, qdepth: depth}
		r.blocks(n, &in)
	case *ast.ThematicBreak:
		r.blank(c)
		rule := append(clone(c.next()), text.Chars(strings.Repeat("─", max(1, r.w-width(c.hang))), text.Style{Fg: terminu.Overlay0})...)
		r.row(rule, c.ground)
	case *east.Table:
		r.table(n, c)
	default:
		r.blocks(n, c)
	}
}

// lines is a block's text, its lines as the source has them.
func (r *renderer) lines(n ast.Node) string {
	var b strings.Builder
	ls := n.Lines()
	for i := 0; i < ls.Len(); i++ {
		s := ls.At(i)
		b.Write(s.Value(r.src))
	}
	return b.String()
}

// list draws a list's items after their markers: a Blue - at every depth (a
// task's [x] or [ ]), or the number by depth — 1., a., i.
func (r *renderer) list(n *ast.List, c *ctx) {
	r.blank(c)
	inner := c.inner()
	inner.tight = true
	if n.IsOrdered() {
		inner.odepth++
	} else {
		inner.udepth++
	}
	num := n.Start
	if num == 0 {
		num = 1
	}
	for it := n.FirstChild(); it != nil; it = it.NextSibling() {
		var marker []text.Char
		switch {
		case n.IsOrdered():
			marker = text.Chars(olLabel(num, inner.odepth)+". ", text.Style{Fg: terminu.Text})
			num++
		case taskOf(it) != nil:
			mark := "[ ]"
			if taskOf(it).IsChecked {
				mark = "[x]"
			}
			marker = append(text.Chars(mark, text.Style{Fg: terminu.Blue}), text.Spaces(1, text.Style{})...)
		default:
			marker = append(text.Chars("-", text.Style{Fg: terminu.Blue}), text.Spaces(1, text.Style{})...)
		}
		used := false
		item := inner
		item.head = append(clone(c.next()), marker...)
		item.hang = append(clone(c.hang), text.Spaces(width(marker), text.Style{})...)
		item.used = &used
		r.blocks(it, &item)
		if !used {
			r.row(trimRight(item.head), c.ground)
		}
	}
}

// taskOf is the check box a list item starts with, nil for none.
func taskOf(item ast.Node) *east.TaskCheckBox {
	first := item.FirstChild()
	if first == nil {
		return nil
	}
	if box, ok := first.FirstChild().(*east.TaskCheckBox); ok {
		return box
	}
	return nil
}

// olLabel is a numbered list's number at its depth: 1, a, i, then 1 again.
func olLabel(n, depth int) string {
	switch (max(1, depth) - 1) % 3 {
	case 1:
		return alpha(n)
	case 2:
		return roman(n)
	}
	return strconv.Itoa(n)
}

func alpha(n int) string {
	if n < 1 {
		return strconv.Itoa(n)
	}
	var s []byte
	for n > 0 {
		n--
		s = append([]byte{byte('a' + n%26)}, s...)
		n /= 26
	}
	return string(s)
}

func roman(n int) string {
	if n < 1 || n > 3999 {
		return strconv.Itoa(n)
	}
	vals := []int{1000, 900, 500, 400, 100, 90, 50, 40, 10, 9, 5, 4, 1}
	syms := []string{"m", "cm", "d", "cd", "c", "xc", "l", "xl", "x", "ix", "v", "iv", "i"}
	var b strings.Builder
	for i, v := range vals {
		for n >= v {
			b.WriteString(syms[i])
			n -= v
		}
	}
	return b.String()
}

// code draws a code block on Mantle in the colours of its language, the
// language named at its top right in Overlay0; a line too long folds. A
// mermaid block is its diagram when it can be drawn.
func (r *renderer) code(src, lang string, c *ctx) {
	r.blank(c)
	src = strings.TrimRight(src, "\n")
	if strings.EqualFold(lang, "mermaid") && r.opt.Diagram != nil {
		if r.diagram(src, c) {
			return
		}
	}
	pre := c.next()
	if lang != "" {
		pad := max(0, r.w-width(pre)-terminu.Width(lang))
		head := append(clone(pre), text.Spaces(pad, text.Style{})...)
		r.row(append(head, text.Escape(lang, text.Style{Fg: terminu.Overlay0})...), mantle)
		pre = c.hang
	}
	room := max(r.w-width(c.hang), 1)
	for _, line := range text.Code(src, lang, r.opt.TabW, mantle) {
		for _, part := range fold(line, room) {
			r.row(append(clone(pre), part...), mantle)
			pre = c.hang
		}
	}
	if src == "" {
		r.row(clone(pre), mantle)
	}
}

// diagram draws a mermaid block as its diagram; one that cannot be drawn is
// its source, as any code block, a row in Red under it saying why. It
// reports whether it drew anything.
func (r *renderer) diagram(src string, c *ctx) bool {
	room := max(r.w-width(c.hang), 1)
	rows, err := r.opt.Diagram(src, room)
	if err != nil {
		if r.opt.OnDiagramError != nil {
			r.opt.OnDiagramError(err)
		}
		pre := c.next()
		pad := max(0, r.w-width(pre)-len("mermaid"))
		r.row(append(append(clone(pre), text.Spaces(pad, text.Style{})...), text.Chars("mermaid", text.Style{Fg: terminu.Overlay0})...), mantle)
		for _, line := range text.Code(src, "", r.opt.TabW, mantle) {
			for _, part := range fold(line, room) {
				r.row(append(clone(c.hang), part...), mantle)
			}
		}
		// pele's message a line a row: its snippet's caret stays under the
		// place it points at (tdp components.5).
		for _, line := range strings.Split(strings.TrimRight(err.Error(), "\n"), "\n") {
			for _, part := range fold(text.Escape(line, text.Style{Fg: terminu.Red}), room) {
				r.row(append(clone(c.hang), part...), c.ground)
			}
		}
		return true
	}
	for _, row := range rows {
		r.row(append(clone(c.next()), row...), c.ground)
	}
	return true
}

// fold cuts a line into parts of at most w cells, at a space when there is
// one, a wide character never cut in half.
func fold(cs []text.Char, w int) [][]text.Char {
	if len(cs) == 0 {
		return [][]text.Char{nil}
	}
	var out [][]text.Char
	for len(cs) > 0 {
		used, cut, brk := 0, len(cs), -1
		for i, x := range cs {
			if used+x.W > w && i > 0 {
				cut = i
				if brk > 0 {
					cut = brk
				}
				break
			}
			used += x.W
			if x.Src == " " {
				brk = i + 1
			}
		}
		out = append(out, cs[:cut])
		cs = cs[cut:]
	}
	return out
}

// table draws a table as a grid in Overlay0, its header row bold with a
// rule under it; cells too wide for the width are cut, a … at their end.
func (r *renderer) table(n *east.Table, c *ctx) {
	r.blank(c)
	var grid [][][]text.Char
	var head []bool
	cols := 0
	for row := n.FirstChild(); row != nil; row = row.NextSibling() {
		_, header := row.(*east.TableHeader)
		var cells [][]text.Char
		for cell := row.FirstChild(); cell != nil; cell = cell.NextSibling() {
			st := text.Style{Fg: terminu.Text, Bold: header}
			var run []text.Char
			for k := cell.FirstChild(); k != nil; k = k.NextSibling() {
				run = r.inline(run, k, st)
			}
			for i := range run {
				if run[i].Draw == lineBreak {
					run[i] = text.Char{Draw: " ", W: 1, St: st, Src: " "}
				}
			}
			for len(run) > 0 && run[0].Draw == " " {
				run = run[1:]
			}
			cells = append(cells, trimRight(run))
		}
		grid = append(grid, cells)
		head = append(head, header)
		cols = max(cols, len(cells))
	}
	if cols == 0 {
		return
	}
	widths := make([]int, cols)
	for _, row := range grid {
		for j, cl := range row {
			widths[j] = max(widths[j], width(cl))
		}
	}
	avail := r.w - width(c.hang) - (3*cols + 1)
	for sum(widths) > avail {
		widest := 0
		for j := range widths {
			if widths[j] > widths[widest] {
				widest = j
			}
		}
		if widths[widest] <= 3 {
			break
		}
		widths[widest]--
	}
	line := text.Style{Fg: terminu.Overlay0}
	rule := func(l, mid, rr string) []text.Char {
		var b strings.Builder
		b.WriteString(l)
		for j, w := range widths {
			if j > 0 {
				b.WriteString(mid)
			}
			b.WriteString(strings.Repeat("─", w+2))
		}
		b.WriteString(rr)
		return append(clone(c.next()), text.Chars(b.String(), line)...)
	}
	r.row(cut(rule("┌", "┬", "┐"), r.w), c.ground)
	for i, row := range grid {
		rc := append(clone(c.next()), text.Chars("│", line)...)
		for j := 0; j < cols; j++ {
			var cl []text.Char
			if j < len(row) {
				cl = row[j]
			}
			if width(cl) > widths[j] {
				cl = append(cut(cl, widths[j]-1), text.Chars("…", line)...)
			}
			rc = append(rc, text.Spaces(1, text.Style{})...)
			rc = append(rc, cl...)
			rc = append(rc, text.Spaces(widths[j]-width(cl)+1, text.Style{})...)
			rc = append(rc, text.Chars("│", line)...)
		}
		r.row(cut(rc, r.w), c.ground)
		if head[i] && i+1 < len(grid) {
			r.row(cut(rule("├", "┼", "┤"), r.w), c.ground)
		}
	}
	r.row(cut(rule("└", "┴", "┘"), r.w), c.ground)
}

func sum(xs []int) int {
	s := 0
	for _, x := range xs {
		s += x
	}
	return s
}

// cut is cs up to w cells wide.
func cut(cs []text.Char, w int) []text.Char {
	used := 0
	for i, c := range cs {
		if used+c.W > w {
			return cs[:i]
		}
		used += c.W
	}
	return cs
}

// inline adds n to a run: text in st with its marks, code in a line on
// Surface0, a link or an image as its icon and its words.
func (r *renderer) inline(run []text.Char, n ast.Node, st text.Style) []text.Char {
	switch n := n.(type) {
	case *ast.Text:
		run = append(run, words(string(n.Segment.Value(r.src)), st)...)
		switch {
		case n.HardLineBreak():
			run = append(run, text.Char{Draw: lineBreak, Src: "\n"})
		case n.SoftLineBreak():
			run = append(run, text.Spaces(1, st)...)
		}
		return run
	case *ast.String:
		return append(run, words(string(n.Value), st)...)
	case *ast.CodeSpan:
		code := st
		code.Fg, code.Bg = pink, surface0
		var b strings.Builder
		for k := n.FirstChild(); k != nil; k = k.NextSibling() {
			if t, ok := k.(*ast.Text); ok {
				b.Write(t.Segment.Value(r.src))
			}
		}
		return append(run, text.Escape(b.String(), code)...)
	case *ast.Emphasis:
		in := st
		if n.Level >= 2 {
			in.Bold = true
		} else {
			in.Italic = true
		}
		return r.kids(run, n, in)
	case *east.Strikethrough:
		in := st
		in.Strike = true
		return r.kids(run, n, in)
	case *ast.Link:
		return r.link(run, n, st, string(n.Destination))
	case *ast.AutoLink:
		link := st
		link.Fg, link.Underline = sapphire, true
		run = append(run, text.Chars(glyphLink+" ", text.Style{Fg: sapphire, Bg: st.Bg})...)
		return append(run, text.Escape(string(n.Label(r.src)), link)...)
	case *ast.Image:
		alt := strings.TrimSpace(plainText(n, r.src))
		if alt == "" {
			alt = path.Base(string(n.Destination))
		}
		media := st
		media.Fg, media.Bold = terminu.Overlay0, false
		return append(run, words(glyphImage+" "+alt, media)...)
	case *east.TaskCheckBox:
		return run // its mark is the list item's marker
	case *ast.RawHTML:
		raw := st
		raw.Fg = terminu.Overlay0
		for i := 0; i < n.Segments.Len(); i++ {
			s := n.Segments.At(i)
			run = append(run, words(string(s.Value(r.src)), raw)...)
		}
		return run
	}
	return r.kids(run, n, st)
}

func (r *renderer) kids(run []text.Char, n ast.Node, st text.Style) []text.Char {
	for k := n.FirstChild(); k != nil; k = k.NextSibling() {
		run = r.inline(run, k, st)
	}
	return run
}

// link is a link's icon in Sapphire, then its words in Sapphire,
// underlined; its address when it has no words.
func (r *renderer) link(run []text.Char, n ast.Node, st text.Style, dest string) []text.Char {
	link := st
	link.Fg, link.Underline = sapphire, true
	run = append(run, text.Chars(glyphLink+" ", text.Style{Fg: sapphire, Bg: st.Bg, Bold: st.Bold})...)
	before := len(run)
	run = r.kids(run, n, link)
	if strings.TrimSpace(plainOf(run[before:])) == "" {
		run = append(run[:before], text.Escape(dest, link)...)
	}
	return run
}

// plainText is the words under n.
func plainText(n ast.Node, src []byte) string {
	var b strings.Builder
	ast.Walk(n, func(k ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch t := k.(type) {
		case *ast.Text:
			b.Write(t.Segment.Value(src))
		case *ast.String:
			b.Write(t.Value)
		}
		return ast.WalkContinue, nil
	})
	return b.String()
}

// words is s from outside in st: each control character by its name in Red,
// a tab a space (the text is laid out again).
func words(s string, st text.Style) []text.Char {
	var out []text.Char
	gr := uniseg.NewGraphemes(s)
	for gr.Next() {
		g := gr.Str()
		if g == "\t" || g == "\n" || g == "\r" {
			out = append(out, text.Char{Draw: " ", W: 1, St: st, Src: " "})
			continue
		}
		out = append(out, text.Escape(g, st)...)
	}
	return out
}
