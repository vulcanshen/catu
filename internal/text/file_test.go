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
// lexer's section and key are, Mauve and Blue, as the lib fills chroma's
// gap (tdp file-picker.3; task 3).
func TestTOMLKeysAndTables(t *testing.T) {
	src := "title = \"x\"\n[project]\nname = \"viewu\"\n  [[tool.mypy]]\n  strict = true\ninline = { key = 1 }\n"
	lines := File(src, "pyproject.toml", 4)
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

// TestYAMLValuesAsThePreview: a value without quotes is Green, as one with
// them, as the lib fills chroma's gap; a tab stays the viewer's to draw
// (tdp file-picker.3, note.2; task 3).
func TestYAMLValuesAsThePreview(t *testing.T) {
	lines := File("kind: Pod\nimage: \"nginx\"\nargs:\n- serve\nnote:\tx\n", "pod.yaml", 4)
	green := terminu.Hex("#a6e3a1")
	for _, c := range []struct {
		line int
		word string
	}{{0, "Pod"}, {1, `"nginx"`}, {3, "serve"}} {
		if got := colourOf(t, lines, c.line, c.word); got != green {
			t.Errorf("%s on line %d is %v, want Green", c.word, c.line+1, got)
		}
	}
	if got := lines[4].Drawn(); !strings.Contains(got, "note:    x") {
		t.Errorf("the tab is drawn %q, want 4 spaces", got)
	}
}

// TestCarriageReturnKeepsTheLines: a \r inside a line of a coloured file
// leaves the line's last state, and the lines after it stay where they are
// (tdp note.2; task 3).
func TestCarriageReturnKeepsTheLines(t *testing.T) {
	lines := File("10%\r100%\nnext\n", "build.yaml", 4)
	var got []string
	for _, l := range lines {
		got = append(got, l.Drawn())
	}
	if strings.Join(got, "|") != "100%|next" {
		t.Errorf("the lines are %q, want [100%% next]", got)
	}
}
