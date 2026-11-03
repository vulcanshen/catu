// Package text is what viewu shows, a character at a time: how each is drawn,
// how many cells it takes, its style, and what a copy of it gives.
package text

import (
	"strconv"
	"strings"

	"github.com/rivo/uniseg"
	terminu "github.com/vulcanshen/terminu-go"
)

// Style is how a character is drawn. The zero Style draws it as it is.
type Style struct {
	Fg, Bg                          terminu.Color
	Bold, Italic, Underline, Strike bool
}

// SGR is the escape sequence that starts drawing in s; "" for the zero Style.
func (s Style) SGR() string {
	var p []string
	if s.Bold {
		p = append(p, "1")
	}
	if s.Italic {
		p = append(p, "3")
	}
	if s.Underline {
		p = append(p, "4")
	}
	if s.Strike {
		p = append(p, "9")
	}
	if s.Fg.Set {
		p = append(p, rgb("38", s.Fg))
	}
	if s.Bg.Set {
		p = append(p, rgb("48", s.Bg))
	}
	if len(p) == 0 {
		return ""
	}
	return "\x1b[" + strings.Join(p, ";") + "m"
}

func rgb(lead string, c terminu.Color) string {
	return lead + ";2;" + strconv.Itoa(int(c.R())) + ";" + strconv.Itoa(int(c.G())) + ";" + strconv.Itoa(int(c.B()))
}

// Char is one character as it is shown.
type Char struct {
	// Draw is what is drawn: the character, a tab's spaces, a control
	// character's name.
	Draw string
	// W is the cells it takes.
	W  int
	St Style
	// Src is what a copy gives: the character as the file has it. A
	// decoration (a list's marker, a table's lines) has its drawing.
	Src string
}

// Line is one line of what is shown.
type Line struct {
	Chars []Char
	// Num is the line number drawn on its first row; 0 for none.
	Num int
	// Fixed marks a line laid out already at the panel's width (a
	// document's row, a diagram's): it never folds.
	Fixed bool
}

// Width is the cells the line's characters take.
func (l Line) Width() int {
	w := 0
	for _, c := range l.Chars {
		w += c.W
	}
	return w
}

// Plain is the text of a line as a copy gives it.
func (l Line) Plain() string {
	var b strings.Builder
	for _, c := range l.Chars {
		b.WriteString(c.Src)
	}
	return b.String()
}

// Drawn is the text of a line as it is drawn.
func (l Line) Drawn() string {
	var b strings.Builder
	for _, c := range l.Chars {
		b.WriteString(c.Draw)
	}
	return b.String()
}

// Chars makes s, a text of the app's own or one already made safe, into
// characters in one style: a character each grapheme.
func Chars(s string, st Style) []Char {
	var out []Char
	gr := uniseg.NewGraphemes(s)
	for gr.Next() {
		g := gr.Str()
		out = append(out, Char{Draw: g, W: terminu.Width(g), St: st, Src: g})
	}
	return out
}

// Spaces is n spaces in st, a decoration a copy gives as spaces.
func Spaces(n int, st Style) []Char {
	out := make([]Char, max(n, 0))
	for i := range out {
		out[i] = Char{Draw: " ", W: 1, St: st, Src: " "}
	}
	return out
}

// Render draws chars, each run of one style in its sequence, a reset at the
// end.
func Render(chars []Char) string {
	var b strings.Builder
	cur, open := Style{}, false
	for _, c := range chars {
		if !open || c.St != cur {
			if open {
				b.WriteString("\x1b[0m")
			}
			b.WriteString(c.St.SGR())
			cur, open = c.St, true
		}
		b.WriteString(c.Draw)
	}
	if open && cur != (Style{}) {
		b.WriteString("\x1b[0m")
	}
	return b.String()
}

// CtrlName is how a control character (C0, DEL, C1) is written: \r, \n and
// \t by name, the rest as \xNN, as terminu-go writes them (tdp
// components.5); ok is false for anything else.
func CtrlName(g string) (string, bool) {
	switch g {
	case "\r":
		return `\r`, true
	case "\n":
		return `\n`, true
	case "\t":
		return `\t`, true
	}
	r := []rune(g)
	if len(r) != 1 || !isControl(r[0]) {
		return "", false
	}
	return `\x` + hex2(int(r[0])), true
}

func isControl(r rune) bool { return r < 0x20 || r >= 0x7f && r <= 0x9f }

func hex2(n int) string {
	const digits = "0123456789abcdef"
	return string([]byte{digits[n>>4&0xf], digits[n&0xf]})
}

// Escape is s, a name from outside (a file's), in st, each control character
// by its name in Red (tdp components.5: names and short texts).
func Escape(s string, st Style) []Char {
	var out []Char
	gr := uniseg.NewGraphemes(s)
	for gr.Next() {
		g := gr.Str()
		if name, ok := CtrlName(g); ok {
			red := st
			red.Fg = terminu.Red
			out = append(out, Char{Draw: name, W: len(name), St: red, Src: g})
			continue
		}
		out = append(out, Char{Draw: g, W: terminu.Width(g), St: st, Src: g})
	}
	return out
}
