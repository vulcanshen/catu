package mermaid

import (
	"strings"
	"testing"

	terminu "github.com/vulcanshen/terminu-go"

	"github.com/vulcanshen/viewu/internal/text"
)

func rows(t *testing.T, src string, w int) []string {
	t.Helper()
	rs, err := Draw(src, w)
	if err != nil {
		t.Fatalf("Draw: %v", err)
	}
	out := make([]string, len(rs))
	for i, r := range rs {
		out[i] = text.Line{Chars: r}.Drawn()
	}
	return out
}

func has(rows []string, s string) bool { return strings.Contains(strings.Join(rows, "\n"), s) }

func TestFlowchartLeftToRight(t *testing.T) {
	rs := rows(t, "flowchart LR\n  A[One] --> B[Two]\n", 80)
	for _, s := range []string{"│ One", "│ Two", "▶", "╭", "╯"} {
		if !has(rs, s) {
			t.Errorf("no %q:\n%s", s, strings.Join(rs, "\n"))
		}
	}
	// The edge joins the boxes on the row of their words.
	for _, r := range rs {
		if strings.Contains(r, "One") && !strings.Contains(r, "├") {
			t.Errorf("the edge does not leave One's box on its row: %q", r)
		}
	}
}

func TestNarrowTurnsTopDown(t *testing.T) {
	rs := rows(t, "flowchart LR\n  A[Alpha] --> B[Bravo] --> C[Charlie]\n", 24)
	if !has(rs, "▼") || has(rs, "▶") {
		t.Errorf("not laid out top down at 24 cells:\n%s", strings.Join(rs, "\n"))
	}
	for _, r := range rs {
		if w := terminu.Width(r); w > 24 {
			t.Errorf("a row %d wide: %q", w, r)
		}
	}
}

func TestEdgeLabelOnTheLine(t *testing.T) {
	rs := rows(t, "flowchart LR\n  A --> |yes| B\n", 80)
	if !has(rs, "─ yes ─") {
		t.Errorf("the label does not sit on its edge:\n%s", strings.Join(rs, "\n"))
	}
}

func TestSequence(t *testing.T) {
	rs := rows(t, "sequenceDiagram\n  Alice->>Bob: Hello\n  Bob-->>Alice: Hi\n", 80)
	for _, s := range []string{"Alice", "Bob", "Hello", "Hi", "▶", "◀", "╌"} {
		if !has(rs, s) {
			t.Errorf("no %q:\n%s", s, strings.Join(rs, "\n"))
		}
	}
}

func TestCurvesAsDots(t *testing.T) {
	rs := rows(t, "flowchart LR\n  A{Is it?} --> B\n", 80)
	braille := false
	for _, r := range rs {
		for _, c := range r {
			if c >= 0x2800 && c <= 0x28ff {
				braille = true
			}
		}
	}
	if !braille {
		t.Errorf("the diamond is not drawn in dots:\n%s", strings.Join(rs, "\n"))
	}
}

func TestWideWords(t *testing.T) {
	rs := rows(t, "flowchart LR\n  A[中文] --> B[日本語]\n", 80)
	if !has(rs, "中文") || !has(rs, "日本語") {
		t.Errorf("%s", strings.Join(rs, "\n"))
	}
}

func TestColours(t *testing.T) {
	rs, err := Draw("flowchart LR\n  A[One] --> B[Two]\n", 80)
	if err != nil {
		t.Fatal(err)
	}
	var box, word, edge bool
	for _, r := range rs {
		for _, c := range r {
			switch {
			case c.Draw == "╭" && c.St.Fg == terminu.Blue:
				box = true
			case c.Draw == "O" && c.St.Fg == terminu.Text:
				word = true
			case c.Draw == "▶" && c.St.Fg == overlay2:
				edge = true
			}
		}
	}
	if !box || !word || !edge {
		t.Errorf("box %v, words %v, arrow %v", box, word, edge)
	}
}

func TestSyntaxError(t *testing.T) {
	_, err := Draw("flowchart LR\n  B[[[\n", 80)
	if err == nil || !strings.Contains(err.Error(), "Parse error") {
		t.Errorf("err %v", err)
	}
	_, err = Draw("not a diagram at all", 80)
	if err == nil {
		t.Error("no error for no diagram")
	}
}

func TestBoxJoins(t *testing.T) {
	g := newGrid(6, 4)
	g.hline(0, 5*cellW, 0, text.Style{}, false)
	g.vline(0, 0, 3*cellH, text.Style{}, false)
	g.vline(5*cellW, 0, 3*cellH, text.Style{}, false)
	g.hline(0, 5*cellW, 3*cellH, text.Style{}, false)
	g.vline(2*cellW, 0, 3*cellH, text.Style{}, true)
	want := []string{"╭─┬──╮", "│ ╎  │", "│ ╎  │", "╰─┴──╯"}
	for i, r := range g.rows() {
		if got := (text.Line{Chars: r}).Drawn(); got != want[i] {
			t.Errorf("row %d %q, want %q", i, got, want[i])
		}
	}
}
