package text

import (
	"strings"
	"testing"

	terminu "github.com/vulcanshen/terminu-go"
)

// colourOf is the colour of the first character of word on line i.
func colourOf(t *testing.T, lines []Line, i int, word string) terminu.Color {
	t.Helper()
	chars := lines[i].Chars
	for k := range chars {
		if strings.HasPrefix((Line{Chars: chars[k:]}).Drawn(), word) {
			return chars[k].St.Fg
		}
	}
	t.Fatalf("no %q on line %d: %q", word, i+1, lines[i].Drawn())
	return terminu.Color{}
}

// TestTOMLKeysAndTables: a table's name and a key are coloured as the INI
// lexer's section and key are, Mauve and Blue (task 3).
func TestTOMLKeysAndTables(t *testing.T) {
	src := "title = \"x\"\n[project]\nname = \"viewu\"\n  [[tool.mypy]]\n  strict = true\ninline = { key = 1 }\n"
	lines := File(src, Lexer("pyproject.toml"), 4)
	keyword, attribute := terminu.Hex("#cba6f7"), terminu.Hex("#89b4fa")
	for _, c := range []struct {
		line int
		word string
		want terminu.Color
	}{
		{0, "title", attribute},
		{1, "project", keyword},
		{2, "name", attribute},
		{3, "tool", keyword},
		{3, "mypy", keyword},
		{4, "strict", attribute},
		{5, "inline", attribute},
		{5, "key", attribute},
	} {
		if got := colourOf(t, lines, c.line, c.word); got != c.want {
			t.Errorf("%s on line %d is %v, want %v", c.word, c.line+1, got, c.want)
		}
	}
	if got := colourOf(t, lines, 2, `"viewu"`); got == terminu.Text {
		t.Error("the string lost its colour")
	}
}
