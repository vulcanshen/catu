package doc

import (
	"errors"
	"strings"
	"testing"

	terminu "github.com/vulcanshen/terminu-go"

	"github.com/vulcanshen/viewu/internal/text"
)

func render(src string, w int) []text.Line { return Render([]byte(src), w, Options{TabW: 4}) }

func drawnRows(lines []text.Line) []string {
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = l.Drawn()
	}
	return out
}

// find is the first row holding s and the index of its first character
// there; -1 when none.
func find(lines []text.Line, s string) (int, int) {
	for i, l := range lines {
		d := l.Drawn()
		if k := strings.Index(d, s); k >= 0 {
			// the character index: count the characters before k bytes
			n, b := 0, 0
			for _, c := range l.Chars {
				if b >= k {
					break
				}
				b += len(c.Draw)
				n++
			}
			return i, n
		}
	}
	return -1, -1
}

func TestEveryRowIsTheWidth(t *testing.T) {
	src := "# Title\n\nA paragraph that is long enough to fold onto a second row at forty cells.\n\n- a\n- b\n\n```go\nfunc main() {}\n```\n\n> quote\n\n| a | b |\n|---|---|\n| 1 | 2 |\n\n---\n"
	for _, w := range []int{20, 40, 77} {
		for i, l := range render(src, w) {
			if l.Width() != w {
				t.Errorf("width %d: row %d is %d wide: %q", w, i, l.Width(), l.Drawn())
			}
			if !l.Fixed {
				t.Errorf("row %d is not fixed", i)
			}
		}
	}
}

func TestNoMarkdownSymbols(t *testing.T) {
	rows := strings.Join(drawnRows(render("# Head\n\n**bold** *it* ~~gone~~ `code` [link](https://x.y)\n", 60)), "\n")
	for _, s := range []string{"#", "**", "~~", "`", "](", "[link"} {
		if strings.Contains(rows, s) {
			t.Errorf("%q drawn:\n%s", s, rows)
		}
	}
}

func TestHeadingIsAChipOfItsLevel(t *testing.T) {
	for level, colour := range levels {
		src := strings.Repeat("#", level+1) + " Title\n"
		lines := render(src, 40)
		i, k := find(lines, "Title")
		if i < 0 {
			t.Fatalf("level %d: no title", level+1)
		}
		c := lines[i].Chars[k]
		if c.St.Bg != colour || c.St.Fg != terminu.Base || !c.St.Bold {
			t.Errorf("level %d: %+v", level+1, c.St)
		}
		if before := lines[i].Chars[k-1]; before.Draw != " " || before.St.Bg != colour {
			t.Errorf("level %d: no space on the chip before the words", level+1)
		}
		if after := lines[i].Chars[k+5]; after.Draw != " " || after.St.Bg != colour {
			t.Errorf("level %d: no space on the chip after the words", level+1)
		}
		if end := lines[i].Chars[len(lines[i].Chars)-1]; end.St.Bg == colour {
			t.Errorf("level %d: the chip runs to the row's end", level+1)
		}
	}
	// The sixth level starts the colours over.
	lines := render("###### Six\n", 40)
	i, k := find(lines, "Six")
	if lines[i].Chars[k].St.Bg != levels[0] {
		t.Error("the sixth level is not the first colour again")
	}
}

func TestLists(t *testing.T) {
	src := "- one\n  - inner\n- [x] done\n- [ ] todo\n\n1. a\n   1. b\n      1. c\n         1. d\n"
	rows := drawnRows(render(src, 40))
	for _, want := range []string{"- one", "  - inner", "[x] done", "[ ] todo", "1. a", "   a. b", "      i. c", "         1. d"} {
		found := false
		for _, r := range rows {
			if strings.HasPrefix(r, want) {
				found = true
			}
		}
		if !found {
			t.Errorf("no row starting %q:\n%s", want, strings.Join(rows, "\n"))
		}
	}
	lines := render("- one\n", 40)
	if c := lines[0].Chars[0]; c.Draw != "-" || c.St.Fg != terminu.Blue {
		t.Errorf("the bullet: %+v", c)
	}
}

func TestListItemFolds(t *testing.T) {
	rows := drawnRows(render("- "+strings.Repeat("word ", 12), 30))
	if len(rows) < 2 || !strings.HasPrefix(rows[1], "  word") {
		t.Errorf("the item's second row does not hang under its words:\n%s", strings.Join(rows, "\n"))
	}
}

func TestCodeBlock(t *testing.T) {
	lines := render("```go\nfunc main() {\n\treturn\n}\n```\n", 40)
	if !strings.HasSuffix(strings.TrimRight(lines[0].Drawn(), " "), "go") {
		t.Errorf("the language is not at the top right: %q", lines[0].Drawn())
	}
	for i, l := range lines {
		for _, c := range l.Chars {
			if c.St.Bg != mantle {
				t.Fatalf("row %d is not all on Mantle: %q", i, l.Drawn())
			}
		}
	}
	if i, _ := find(lines, "    return"); i < 0 {
		t.Error("the tab is not 4 spaces")
	}
	if i, k := find(lines, "func"); lines[i].Chars[k].St.Fg == terminu.Text {
		t.Error("the code is not in colour")
	}
}

func TestCodeLineFolds(t *testing.T) {
	long := strings.Repeat("x", 50)
	rows := drawnRows(render("```\n"+long+"\n```\n", 30))
	if len(rows) != 2 || strings.TrimRight(rows[0], " ")+strings.TrimRight(rows[1], " ") != long {
		t.Errorf("a long line is not folded whole:\n%s", strings.Join(rows, "\n"))
	}
}

func TestQuote(t *testing.T) {
	lines := render("> said\n>\n> > again\n", 30)
	if c := lines[0].Chars[0]; c.Draw != "▎" || c.St.Fg != terminu.Lavender || c.St.Bg != mantle {
		t.Errorf("the bar: %+v", c)
	}
	rows := drawnRows(lines)
	if !strings.HasPrefix(rows[len(rows)-1], "▎ ▎ again") {
		t.Errorf("the inner quote:\n%s", strings.Join(rows, "\n"))
	}
	for i, r := range rows {
		if strings.Trim(r, "▎ ") == "" && i+1 < len(rows) && strings.Trim(rows[i+1], "▎ ") == "" {
			t.Errorf("two blank rows in a row:\n%s", strings.Join(rows, "\n"))
		}
	}
}

func TestTable(t *testing.T) {
	rows := drawnRows(render("| Name | Size |\n|---|---|\n| a | 1 |\n| bb | 22 |\n", 40))
	want := []string{"┌──────┬──────┐", "│ Name │ Size │", "├──────┼──────┤", "│ a    │ 1    │", "│ bb   │ 22   │", "└──────┴──────┘"}
	for i, w := range want {
		if i >= len(rows) || strings.TrimRight(rows[i], " ") != w {
			t.Fatalf("row %d:\n%s", i, strings.Join(rows, "\n"))
		}
	}
	lines := render("| Name |\n|---|\n| a |\n", 40)
	if _, k := find(lines, "Name"); !lines[1].Chars[k].St.Bold {
		t.Error("the header is not bold")
	}
}

func TestTableTooWide(t *testing.T) {
	rows := drawnRows(render("| "+strings.Repeat("long ", 10)+"|\n|---|\n| x |\n", 20))
	for _, r := range rows {
		if len([]rune(strings.TrimRight(r, " "))) > 20 {
			t.Errorf("a row past the width: %q", r)
		}
	}
	if !strings.Contains(strings.Join(rows, ""), "…") {
		t.Error("the cut cell has no …")
	}
}

func TestLinkAndImage(t *testing.T) {
	lines := render("[the site](https://x.y) and ![a cat](cat.png) and ![](dog.png)\n", 80)
	row := lines[0].Drawn()
	for _, s := range []string{glyphLink + " the site", glyphImage + " a cat", glyphImage + " dog.png"} {
		if !strings.Contains(row, s) {
			t.Errorf("no %q in %q", s, row)
		}
	}
	_, k := find(lines, "the site")
	if c := lines[0].Chars[k]; c.St.Fg != sapphire || !c.St.Underline {
		t.Errorf("the link's words: %+v", c.St)
	}
}

func TestInlineStyles(t *testing.T) {
	lines := render("**b** *i* ~~s~~ `c`\n", 40)
	l := lines[0]
	for _, c := range []struct {
		s  string
		ok func(text.Style) bool
	}{
		{"b", func(s text.Style) bool { return s.Bold }},
		{"i", func(s text.Style) bool { return s.Italic }},
		{"s", func(s text.Style) bool { return s.Strike }},
		{"c", func(s text.Style) bool { return s.Bg == surface0 && s.Fg == pink }},
	} {
		found := false
		for _, ch := range l.Chars {
			if ch.Draw == c.s && c.ok(ch.St) {
				found = true
			}
		}
		if !found {
			t.Errorf("%q is not styled", c.s)
		}
	}
}

func TestRule(t *testing.T) {
	lines := render("a\n\n---\n\nb\n", 10)
	if lines[2].Drawn() != strings.Repeat("─", 10) {
		t.Errorf("the rule: %q", lines[2].Drawn())
	}
}

func TestDiagram(t *testing.T) {
	opt := Options{Diagram: func(src string, w int) ([][]text.Char, error) {
		return [][]text.Char{text.Chars("[diagram of "+strings.TrimSpace(src)+"]", text.Style{})}, nil
	}}
	rows := drawnRows(Render([]byte("```mermaid\ngraph\n```\n"), 40, opt))
	if len(rows) != 1 || !strings.HasPrefix(rows[0], "[diagram of graph]") {
		t.Errorf("%q", rows)
	}
}

func TestDiagramError(t *testing.T) {
	var told []error
	opt := Options{
		Diagram:        func(string, int) ([][]text.Char, error) { return nil, errors.New("Parse error on line 1:\nbad\n^") },
		OnDiagramError: func(err error) { told = append(told, err) },
	}
	lines := Render([]byte("```mermaid\nbad\n```\n"), 40, opt)
	rows := drawnRows(lines)
	want := []string{"mermaid", "bad", "Parse error on line 1:", "bad", "^"}
	if len(rows) != len(want) {
		t.Fatalf("%q", rows)
	}
	for i, w := range want {
		if strings.TrimSpace(rows[i]) != w {
			t.Errorf("row %d %q, want %q", i, rows[i], w)
		}
	}
	if c := lines[2].Chars[0]; c.St.Fg != terminu.Red {
		t.Errorf("the error is not Red: %+v", c.St)
	}
	if len(told) != 1 {
		t.Errorf("told %d times", len(told))
	}
}

func TestControlCharacters(t *testing.T) {
	lines := render("a\x07b `c\x1bd`\n", 40)
	if !strings.Contains(lines[0].Drawn(), `a\x07b`) || !strings.Contains(lines[0].Drawn(), `c\x1bd`) {
		t.Errorf("%q", lines[0].Drawn())
	}
}
