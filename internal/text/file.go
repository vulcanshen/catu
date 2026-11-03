package text

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/rivo/uniseg"
	terminu "github.com/vulcanshen/terminu-go"
)

// mocha is the colours text is highlighted in: catppuccin-mocha, the
// foreground only (tdp note.2, file-picker.3).
var mocha = styles.Get("catppuccin-mocha")

// Lexer is the lexer for a file named name, nil when none matches: the
// language is judged by the name (tdp note.2).
func Lexer(name string) chroma.Lexer {
	return lexers.Match(filepath.Base(name))
}

// LexerFor is the lexer a code block's language names ("go", "yaml"); nil
// when there is none.
func LexerFor(lang string) chroma.Lexer {
	if lang == "" {
		return nil
	}
	return lexers.Get(lang)
}

// File is a text's lines as a viewer draws them (tdp note.2, file-picker.3,
// components.5), numbered from 1: in the colours lexer reads them in,
// catppuccin-mocha, the foreground only, Text where it leaves them (and with
// no lexer); a tab as tabW spaces; the \r of a CRLF not drawn; any other
// control character by its name in Red. A text with colour codes of its own
// is drawn in them instead, not lexed, a \r inside a line leaving the line's
// last state, as a terminal does.
func File(content string, lexer chroma.Lexer, tabW int) []Line {
	if content == "" {
		return nil // an empty file has no lines
	}
	raw := strings.Split(content, "\n")
	if len(raw) > 1 && raw[len(raw)-1] == "" {
		raw = raw[:len(raw)-1] // the text's last line break ends a line, it starts none
	}
	for i, l := range raw {
		raw[i] = strings.TrimSuffix(l, "\r")
	}
	out := make([]Line, len(raw))
	if strings.Contains(content, "\x1b[") {
		for i, l := range raw {
			out[i] = Line{Chars: coloured(l, tabW), Num: i + 1}
		}
		return out
	}
	colours := lex(raw, lexer)
	for i, l := range raw {
		b := newBuilder(tabW)
		fg := func(k int) terminu.Color {
			if k < len(colours[i]) && colours[i][k].Set {
				return colours[i][k]
			}
			return terminu.Text
		}
		if isASCII(l) { // a character a byte: no graphemes to find
			b.chars = make([]Char, 0, len(l))
			for k := 0; k < len(l); k++ {
				b.add(ascii[l[k]], Style{Fg: fg(k)})
			}
		} else {
			k := 0
			gr := uniseg.NewGraphemes(l)
			for gr.Next() {
				b.add(gr.Str(), Style{Fg: fg(k)})
				k++
			}
		}
		out[i] = Line{Chars: b.chars, Num: i + 1}
	}
	return out
}

// ascii are the 128 ASCII characters as strings.
var ascii = func() (a [128]string) {
	for i := range a {
		a[i] = string(rune(i))
	}
	return a
}()

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			return false
		}
	}
	return true
}

// Code is a code block's lines, as File draws a file's but not numbered.
func Code(content string, lexer chroma.Lexer, tabW int, ground terminu.Color) [][]Char {
	lines := File(content, lexer, tabW)
	out := make([][]Char, len(lines))
	for i, l := range lines {
		for k := range l.Chars {
			l.Chars[k].St.Bg = ground
		}
		out[i] = l.Chars
	}
	return out
}

// lex is the foreground of each grapheme of each line as lexer reads them;
// an unset colour where it gives none.
func lex(lines []string, lexer chroma.Lexer) [][]terminu.Color {
	out := make([][]terminu.Color, len(lines))
	if lexer == nil {
		return out
	}
	it, err := chroma.Coalesce(lexer).Tokenise(nil, strings.Join(lines, "\n"))
	if err != nil {
		return out
	}
	toml := lexer.Config() != nil && lexer.Config().Name == "TOML"
	row, startOfLine, header := 0, true, false
	for tok := it(); tok != chroma.EOF; tok = it() {
		kind := tok.Type
		if toml {
			kind, startOfLine, header = tomlKind(tok, startOfLine, header)
		}
		c := mocha.Get(kind).Colour
		fg := terminu.Color{}
		if c.IsSet() {
			fg = terminu.Color{RGB: terminu.RGB{c.Red(), c.Green(), c.Blue()}, Set: true}
		}
		for i, part := range strings.Split(tok.Value, "\n") {
			if i > 0 {
				row++
			}
			if row >= len(out) {
				break
			}
			gr := uniseg.NewGraphemes(part)
			for gr.Next() {
				out[row] = append(out[row], fg)
			}
		}
	}
	return out
}

// TODO(tdp #1): tdp note.2 has the viewer coloured as the file-picker's
// preview is, and terminu-go's preview leaves these uncoloured; until tdp
// says who fills a lexer's gap, viewu does it here (the user's choice).
//
// tomlKind is the kind a TOML token is coloured as. chroma's TOML lexer
// calls a table's name and a key both NameOther, which catppuccin-mocha
// leaves uncoloured; they are coloured as the INI lexer's are, a table
// (a line that starts with [) a Keyword, a key a NameAttribute. It is told
// whether the token starts a line and whether the line is a table's; it
// gives them back for the next token.
func tomlKind(tok chroma.Token, startOfLine, header bool) (chroma.TokenType, bool, bool) {
	kind := tok.Type
	switch {
	case strings.Contains(tok.Value, "\n"):
		return kind, true, false
	case startOfLine && strings.TrimSpace(tok.Value) == "":
		return kind, true, false // the indent before the line's first token
	case startOfLine:
		header = tok.Type == chroma.Punctuation && strings.HasPrefix(tok.Value, "[")
	}
	if kind == chroma.NameOther {
		kind = chroma.NameAttribute
		if header {
			kind = chroma.Keyword
		}
	}
	return kind, false, header
}

// builder makes a line's characters as a terminal leaves them: a \r goes
// back to the line's start, what follows written over what was there.
type builder struct {
	tabW  int
	chars []Char
	at    int
}

func newBuilder(tabW int) *builder { return &builder{tabW: max(tabW, 1)} }

func (b *builder) add(g string, st Style) {
	if g == "\r" {
		b.at = 0
		return
	}
	w := 1
	if len(g) > 1 || g[0] < 0x20 || g[0] >= 0x7f {
		w = terminu.Width(g) // not a printable ASCII character: measure it
	}
	c := Char{Draw: g, W: w, St: st, Src: g}
	switch name, ctrl := CtrlName(g); {
	case g == "\t":
		c.Draw, c.W = strings.Repeat(" ", b.tabW), b.tabW
	case ctrl:
		red := st
		red.Fg = terminu.Red
		c.Draw, c.W, c.St = name, len(name), red
	}
	if b.at < len(b.chars) {
		b.chars[b.at] = c
	} else {
		b.chars = append(b.chars, c)
	}
	b.at++
}

// sgr matches an SGR sequence at the start of a string.
var sgr = regexp.MustCompile(`^\x1b\[[0-9;:]*m`)

// coloured is a line with colour codes of its own, drawn in them (tdp
// components.5): any other escape sequence is characters, its ESC by name
// in Red. What it leaves uncoloured is Text.
func coloured(line string, tabW int) []Char {
	b := newBuilder(tabW)
	st := Style{Fg: terminu.Text}
	for len(line) > 0 {
		if seq := sgr.FindString(line); seq != "" {
			st = applySGR(st, seq)
			line = line[len(seq):]
			continue
		}
		g, rest, _, _ := uniseg.FirstGraphemeClusterInString(line, -1)
		b.add(g, st)
		line = rest
	}
	return b.chars
}

// applySGR is st after the SGR sequence seq.
func applySGR(st Style, seq string) Style {
	body := strings.TrimSuffix(strings.TrimPrefix(seq, "\x1b["), "m")
	params := strings.FieldsFunc(body, func(r rune) bool { return r == ';' || r == ':' })
	if len(params) == 0 {
		params = []string{"0"}
	}
	n := func(i int) int {
		if i >= len(params) {
			return 0
		}
		v, _ := strconv.Atoi(params[i])
		return v
	}
	for i := 0; i < len(params); i++ {
		switch p := n(i); {
		case p == 0:
			st = Style{Fg: terminu.Text}
		case p == 1:
			st.Bold = true
		case p == 22:
			st.Bold = false
		case p == 3:
			st.Italic = true
		case p == 23:
			st.Italic = false
		case p == 4:
			st.Underline = true
		case p == 24:
			st.Underline = false
		case p == 9:
			st.Strike = true
		case p == 29:
			st.Strike = false
		case p >= 30 && p <= 37:
			st.Fg = ansi16[p-30]
		case p >= 90 && p <= 97:
			st.Fg = ansi16[p-90+8]
		case p >= 40 && p <= 47:
			st.Bg = ansi16[p-40]
		case p >= 100 && p <= 107:
			st.Bg = ansi16[p-100+8]
		case p == 39:
			st.Fg = terminu.Text
		case p == 49:
			st.Bg = terminu.Color{}
		case p == 38 || p == 48:
			var c terminu.Color
			switch n(i + 1) {
			case 5:
				c = ansi256(n(i + 2))
				i += 2
			case 2:
				c = terminu.Color{RGB: terminu.RGB{uint8(n(i + 2)), uint8(n(i + 3)), uint8(n(i + 4))}, Set: true}
				i += 4
			default:
				i++
				continue
			}
			if p == 38 {
				st.Fg = c
			} else {
				st.Bg = c
			}
		}
	}
	return st
}

// ansi16 are the sixteen terminal colours, in catppuccin-mocha's.
var ansi16 = [16]terminu.Color{
	terminu.Hex("#45475a"), terminu.Hex("#f38ba8"), terminu.Hex("#a6e3a1"), terminu.Hex("#f9e2af"),
	terminu.Hex("#89b4fa"), terminu.Hex("#f5c2e7"), terminu.Hex("#94e2d5"), terminu.Hex("#bac2de"),
	terminu.Hex("#585b70"), terminu.Hex("#f38ba8"), terminu.Hex("#a6e3a1"), terminu.Hex("#f9e2af"),
	terminu.Hex("#89b4fa"), terminu.Hex("#f5c2e7"), terminu.Hex("#94e2d5"), terminu.Hex("#a6adc8"),
}

// ansi256 is colour n of the 256-colour palette.
func ansi256(n int) terminu.Color {
	switch {
	case n < 16:
		return ansi16[max(n, 0)]
	case n < 232:
		n -= 16
		level := func(v int) uint8 {
			if v == 0 {
				return 0
			}
			return uint8(55 + 40*v)
		}
		return terminu.Color{RGB: terminu.RGB{level(n / 36), level(n / 6 % 6), level(n % 6)}, Set: true}
	case n < 256:
		v := uint8(8 + 10*(n-232))
		return terminu.Color{RGB: terminu.RGB{v, v, v}, Set: true}
	}
	return terminu.Text
}
