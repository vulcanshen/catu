package ui

import (
	"path/filepath"
	"strings"
	"testing"

	terminu "github.com/vulcanshen/terminu-go"
	"github.com/vulcanshen/terminu-go/tdptest"
)

// --- task 9: Markdown ------------------------------------------------------

func TestMarkdownDocumentAndSource(t *testing.T) {
	dir := files(t, map[string]string{"a.md": "# Title\n\nSome **bold** words.\n"})
	tdptest.EachIconWidth(t, func(t *testing.T) {
		for _, sz := range [][2]int{{40, 12}, {80, 24}} {
			d := start(t, filepath.Join(dir, "a.md"), sz[0], sz[1])
			want(t, d, "DOCUMENT", "SOURCE", "h|l:tab", " Title ", "Some bold words.")
			wantNot(t, d, "# Title", "**")
			d.Press("l")
			want(t, d, "  1 # Title", "  3 Some **bold** words.")
			d.Press("h")
			want(t, d, "Some bold words.")
		}
	})
}

func TestMarkdownLaidOutAgainOnResize(t *testing.T) {
	dir := files(t, map[string]string{"a.md": strings.Repeat("word ", 30) + "END\n"})
	d := start(t, filepath.Join(dir, "a.md"), 100, 24)
	rowsOf := func() int {
		n := 0
		for _, l := range d.Screen() {
			if strings.Contains(l, "word") {
				n++
			}
		}
		return n
	}
	wide := rowsOf()
	d.Resize(50, 24)
	want(t, d, "END")
	if narrow := rowsOf(); narrow <= wide {
		t.Errorf("not laid out again: %d rows at 100, %d at 50", wide, narrow)
	}
}

func TestMarkdownSearchAndCopy(t *testing.T) {
	dir := files(t, map[string]string{"a.md": "# Title\n\n- one item\n"})
	d := start(t, filepath.Join(dir, "a.md"), 60, 20)
	d.Press("/")
	d.Type("item")
	d.Press("Enter")
	if !strings.Contains(strings.Join(d.Styled(), ""), cursorStyle.SGR()+"item") {
		t.Error("the hit in the document is not the cursor block")
	}
	d.Press("Esc", "v", "j", "j", "v", "$", "y")
	if got := d.Clipboard(); len(got) != 1 || got[0] != "- one item" {
		t.Errorf("copied %q: what is drawn, without the spaces filling the row", got)
	}
}

func TestEmptyMarkdown(t *testing.T) {
	dir := files(t, map[string]string{"a.md": ""})
	d := start(t, filepath.Join(dir, "a.md"), 60, 20)
	want(t, d, "DOCUMENT", "Empty file")
}

// --- task 10: mermaid ------------------------------------------------------

func TestMermaidInMarkdown(t *testing.T) {
	dir := files(t, map[string]string{"a.md": "# Flow\n\n```mermaid\nflowchart LR\n  A[Start] --> B[End]\n```\n"})
	d := start(t, filepath.Join(dir, "a.md"), 80, 24)
	want(t, d, "│ Start", "│ End", "▶")
	wantNot(t, d, "flowchart")
}

func TestMermaidFile(t *testing.T) {
	dir := files(t, map[string]string{"flow.mmd": "flowchart LR\n  A[Start] --> B[End]\n"})
	tdptest.EachIconWidth(t, func(t *testing.T) {
		for _, sz := range [][2]int{{40, 12}, {80, 24}, {120, 40}} {
			d := start(t, filepath.Join(dir, "flow.mmd"), sz[0], sz[1])
			want(t, d, "DIAGRAM", "SOURCE", "Start", "End")
			d.Press("l")
			want(t, d, "  1 flowchart LR")
		}
	})
}

func TestMermaidTooWide(t *testing.T) {
	var b strings.Builder
	b.WriteString("sequenceDiagram\n")
	for _, p := range []string{"Alpha", "Bravo", "Charlie", "Delta", "Echo", "Foxtrot"} {
		b.WriteString("  participant " + p + "\n")
	}
	b.WriteString("  Alpha->>Foxtrot: a long way across\n")
	dir := files(t, map[string]string{"seq.mmd": b.String()})
	d := start(t, filepath.Join(dir, "seq.mmd"), 40, 20)
	want(t, d, "…", "Alpha")
}

// TestMermaidFileCannotBeDrawn: a diagram file pele cannot draw leaves the
// whole DIAGRAM tab with nothing to show: why in Red in the middle, a hint
// under it (tdp panel.15), the location row over them (panel.10); the App
// Log keeps all of pele's message, once; the source is a tab away.
func TestMermaidFileCannotBeDrawn(t *testing.T) {
	dir := files(t, map[string]string{"bad.mmd": "flowchart LR\n  A --> B[[[\n"})
	d := start(t, filepath.Join(dir, "bad.mmd"), 80, 24)
	want(t, d, "Can't draw this diagram: Parse error on line 2:", "Press [l] to see the source, [!] for the whole error", "!:1 warning")
	if !strings.HasSuffix(strings.TrimRight(d.Screen()[2], " ┃"), "bad.mmd") {
		t.Errorf("no location row over the reason: %q", d.Screen()[2])
	}
	wantNot(t, d, "B[[[")
	red := strings.TrimSuffix(terminu.Fg(terminu.Red).Render("Can't draw this diagram"), "\x1b[0m")
	if !strings.Contains(strings.Join(d.Styled(), ""), red) {
		t.Error("the reason is not in Red")
	}
	d.Resize(70, 24)
	d.Resize(80, 24)
	d.Press("l")
	want(t, d, "  2   A --> B[[[")
	wantNot(t, d, "Can't draw this diagram")
	d.Press("h")
	want(t, d, "Can't draw this diagram: Parse error on line 2:")
	d.Press("!")
	want(t, d, "Expecting")
	if n := strings.Count(strings.Join(d.Screen(), "\n"), "WARN Can't draw a mermaid diagram in bad.mmd"); n != 1 {
		t.Errorf("%d warnings, want 1", n)
	}
}

func TestMermaidErrorLoggedOnce(t *testing.T) {
	dir := files(t, map[string]string{"bad.md": "```mermaid\nflowchart LR\n  B[[[\n```\n"})
	d := start(t, filepath.Join(dir, "bad.md"), 80, 24)
	want(t, d, "B[[[", "Parse error on line 2:", "!:1 warning")
	d.Resize(70, 24)
	d.Resize(80, 24)
	d.Press("!")
	if n := strings.Count(strings.Join(d.Screen(), "\n"), "WARN Can't draw a mermaid diagram in bad.md"); n != 1 {
		t.Errorf("%d warnings, want 1:\n%s", n, strings.Join(d.Screen(), "\n"))
	}
}
