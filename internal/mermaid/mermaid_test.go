package mermaid

import (
	"strings"
	"sync"
	"testing"

	terminu "github.com/vulcanshen/terminu-go"

	"github.com/vulcanshen/catu/internal/text"
)

func drawRows(t *testing.T, src string, w int) []string {
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

// TestWarmLoadsPele: Warm, called from the background as a file with a
// diagram opens, loads pele once, however many call it at a time, ready for
// the first diagram.
func TestWarmLoadsPele(t *testing.T) {
	var wg sync.WaitGroup
	for range 4 {
		wg.Add(1)
		go func() { defer wg.Done(); Warm() }()
	}
	wg.Wait()
	if loadErr != nil || render == nil {
		t.Fatalf("pele is not loaded: %v", loadErr)
	}
}

func TestFlowchartLeftToRight(t *testing.T) {
	rs := drawRows(t, "flowchart LR\n  A[One] --> B[Two]\n", 80)
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
	rs := drawRows(t, "flowchart LR\n  A[Alpha] --> B[Bravo] --> C[Charlie]\n", 24)
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
	rs := drawRows(t, "flowchart LR\n  A --> |yes| B\n", 80)
	if !has(rs, "─ yes ─") {
		t.Errorf("the label does not sit on its edge:\n%s", strings.Join(rs, "\n"))
	}
}

func TestSequence(t *testing.T) {
	rs := drawRows(t, "sequenceDiagram\n  Alice->>Bob: Hello\n  Bob-->>Alice: Hi\n", 80)
	for _, s := range []string{"Alice", "Bob", "Hello", "Hi", "▶", "◀", "╌"} {
		if !has(rs, s) {
			t.Errorf("no %q:\n%s", s, strings.Join(rs, "\n"))
		}
	}
}

func braille(rows []string) bool {
	for _, r := range rows {
		for _, c := range r {
			if c >= 0x2800 && c <= 0x28ff {
				return true
			}
		}
	}
	return false
}

// TestDecisionAsABox: a flowchart's decision is a box as tall as any, a ◆
// in the middle of either side (user 2026-11-04); an arrow at its side stops
// before the ◆, and its lines leave from its border.
func TestDecisionAsABox(t *testing.T) {
	rs := drawRows(t, "flowchart LR\n  S[Start] --> A{Is it?}\n  A -- yes --> B\n  A -- no --> C\n", 80)
	if braille(rs) {
		t.Errorf("not a box:\n%s", strings.Join(rs, "\n"))
	}
	for _, s := range []string{"▶◆ Is it?", "╭─────────╮", "╰─────────╯"} {
		if !has(rs, s) {
			t.Errorf("no %q:\n%s", s, strings.Join(rs, "\n"))
		}
	}
	for i, r := range rs {
		if strings.Contains(r, "Is it?") && strings.Count(r, "◆") != 2 {
			t.Errorf("row %d: no ◆ either side: %q", i, r)
		}
	}
	rs = drawRows(t, "flowchart TD\n  A[判斷開始] --> B{是否存在？}\n  B -->|是| C[讀檔]\n", 80)
	if !has(rs, "◆ 是否存在？  ◆") || !has(rs, "╭─────────────╮") {
		t.Errorf("wide words:\n%s", strings.Join(rs, "\n"))
	}
	rs = drawRows(t, "flowchart LR\n  A{{Hexagon}} --> B\n", 80)
	if has(rs, "◆") {
		t.Errorf("a hexagon is not a decision:\n%s", strings.Join(rs, "\n"))
	}
}

// TestAsDecisions: only a flowchart's decisions are written as boxes, not a
// hexagon, words in quotes, a decision with a class of its own, nor any
// other diagram's braces.
func TestAsDecisions(t *testing.T) {
	for _, c := range []struct{ src, want string }{
		{"flowchart TD\n  A --> B{Is it?}", "flowchart TD\n  A --> B[\"Is it?\"]:::" + decisionClass},
		{"graph LR\n  B{\"a [b]\"} -- x --> 判斷{是否？}", "graph LR\n  B[\"a [b]\"]:::" + decisionClass + " -- x --> 判斷[\"是否？\"]:::" + decisionClass},
		{"---\ntitle: T\n---\n%% c\nflowchart TD\n  A{x}", "---\ntitle: T\n---\n%% c\nflowchart TD\n  A[\"x\"]:::" + decisionClass},
		{"flowchart TD\n  A{{hex}} --> B[\"set {k}\"]", "flowchart TD\n  A{{hex}} --> B[\"set {k}\"]"},
		{"flowchart TD\n  A{x}:::mine\n  classDef mine fill:#f00", "flowchart TD\n  A{x}:::mine\n  classDef mine fill:#f00"},
		{"classDiagram\n  class A{\n    +int x\n  }", "classDiagram\n  class A{\n    +int x\n  }"},
		{"stateDiagram-v2\n  state S{\n  }", "stateDiagram-v2\n  state S{\n  }"},
	} {
		if got := asDecisions(c.src); got != c.want {
			t.Errorf("%q:\n got %q\nwant %q", c.src, got, c.want)
		}
	}
}

// TestDecisionErrorAsWritten: a diagram that cannot be drawn says so in the
// words the user wrote, its diamonds as they are.
func TestDecisionErrorAsWritten(t *testing.T) {
	_, err := Draw("flowchart LR\n  A{Is it?} --> B[[[\n", 80)
	if err == nil || strings.Contains(err.Error(), decisionClass) {
		t.Errorf("err %v", err)
	}
}

// TestNoDots: nothing is drawn in dots (user 2026-11-04): a circle is a box
// with a ( and a ) on its sides, a state diagram's start ● and its end ◉,
// a curve an elbow of box-drawing lines (a box's lines to the boxes below
// it share a row, as a tree's do), a loop to its own box a corner out and
// back, the arrow at its border.
func TestNoDots(t *testing.T) {
	for _, c := range []struct {
		src  string
		want []string
	}{
		{"flowchart LR\n  A((Round)) --> B\n", []string{"(Round )────▶│"}},
		{"stateDiagram-v2\n  [*] --> Idle\n  Idle --> [*]\n", []string{"●", "◉"}},
		{"flowchart TD\n  K[kind] --> A[one]\n  K --> B[two]\n  K --> C[three]\n  K --> D[four]\n", []string{"╰┬─┬─┬─┬╯", "╭────╯ ╰─────╮"}},
		{"flowchart TD\n  A[Loop] --> A\n", []string{"├──╮", "│◀─╯"}},
	} {
		rs := drawRows(t, c.src, 80)
		if braille(rs) {
			t.Errorf("dots:\n%s", strings.Join(rs, "\n"))
		}
		for _, s := range c.want {
			if !has(rs, s) {
				t.Errorf("no %q:\n%s", s, strings.Join(rs, "\n"))
			}
		}
	}
}

// TestPieAsBars: a pie chart is a bar a slice, the biggest first, as long as
// it is next to the biggest, in the colour pele gives that slice, its share
// after it; with showData, its value too.
func TestPieAsBars(t *testing.T) {
	src := "pie showData title Files\n  \"text\" : 12\n  \"Markdown\" : 3\n  \"binary\" : 4\n"
	rs, err := Draw(src, 60)
	if err != nil {
		t.Fatal(err)
	}
	drawn := make([]string, len(rs))
	for i, r := range rs {
		drawn[i] = strings.TrimRight(text.Line{Chars: r}.Drawn(), " ")
	}
	if braille(drawn) || strings.TrimSpace(drawn[0]) != "Files" || drawn[1] != "" {
		t.Fatalf("not a title over bars:\n%s", strings.Join(drawn, "\n"))
	}
	for i, want := range []struct {
		name, share, value string
		colour             terminu.Color
	}{
		{"text", "63%", "[12]", pieColours[0]},
		{"binary", "21%", "[4]", pieColours[2]},
		{"Markdown", "16%", "[3]", pieColours[1]},
	} {
		row := drawn[2+i]
		if !strings.HasPrefix(row, "  "+want.name) || !strings.Contains(row, want.share+"  "+want.value) {
			t.Errorf("row %d: %q", i, row)
		}
		for _, c := range rs[2+i] {
			if c.Draw == "█" && c.St.Fg != want.colour {
				t.Errorf("%s: its bar in %v, want %v", want.name, c.St.Fg, want.colour)
				break
			}
		}
	}
	if a, b := strings.Count(drawn[2], "█"), strings.Count(drawn[3], "█"); a <= b || b == 0 {
		t.Errorf("bars %d, %d: the biggest first, as long as it is next to the biggest", a, b)
	}
	if _, err := Draw("pie\n  \"a\" 1\n", 60); err == nil {
		t.Error("pele reads a pie first: no error for a bad one")
	}
	// A name is the file's: a control character in it by its name, in Red;
	// one too long for a third of the width cut with a ….
	rs, err = Draw("pie\n  \"tab\there\" : 2\n  \""+strings.Repeat("long ", 10)+"\" : 1\n", 60)
	if err != nil {
		t.Fatal(err)
	}
	if row := (text.Line{Chars: rs[0]}).Drawn(); !strings.Contains(row, `tab\there`) {
		t.Errorf("the tab is not by its name: %q", row)
	}
	for _, c := range rs[0] {
		if c.Src == "\t" && c.St.Fg != terminu.Red {
			t.Errorf("the tab's name in %v, want Red", c.St.Fg)
		}
	}
	if row := (text.Line{Chars: rs[1]}).Drawn(); !strings.Contains(row, "long…") || strings.Count(row, "long") > 4 {
		t.Errorf("a long name not cut: %q", row)
	}
}

func TestWideWords(t *testing.T) {
	rs := drawRows(t, "flowchart LR\n  A[中文] --> B[日本語]\n", 80)
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
	g := newGrid(6, 4, newRows(nil))
	g.hline(0, 5*cellW, 0, text.Style{}, false)
	g.vline(0, 0, 3*cellH, text.Style{}, false)
	g.vline(5*cellW, 0, 3*cellH, text.Style{}, false)
	g.hline(0, 5*cellW, 3*cellH, text.Style{}, false)
	g.vline(2*cellW, 0, 3*cellH, text.Style{}, true)
	want := []string{"╭─┬──╮", "│ ╎  │", "│ ╎  │", "╰─┴──╯"}
	for i, r := range g.lines() {
		if got := (text.Line{Chars: r}).Drawn(); got != want[i] {
			t.Errorf("row %d %q, want %q", i, got, want[i])
		}
	}
}
