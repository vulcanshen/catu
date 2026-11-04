package ui

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	terminu "github.com/vulcanshen/terminu-go"
)

// TestPrintInColour: printed on a terminal (task 14), a text is in its
// syntax colours, the same as on the screen, with no line numbers; a
// document is drawn in its colours; each path it cannot is told, and the
// rest are printed.
func TestPrintInColour(t *testing.T) {
	dir := files(t, map[string]string{"a.go": "package a\n\nfunc x() {}\n", "a.md": "# Title\n\nwords\n"})
	var out bytes.Buffer
	var failed []string
	Print([]string{filepath.Join(dir, "a.go"), filepath.Join(dir, "nope"), filepath.Join(dir, "a.md")}, &out, true, 60,
		func(p string, err error) { failed = append(failed, filepath.Base(p)) })
	got := out.String()
	keyword := strings.TrimSuffix(terminu.Fg(terminu.Hex("#94e2d5")).Render("package"), "\x1b[0m") // Teal: a namespace keyword
	if !strings.Contains(got, keyword) {
		t.Errorf("not in its colours: %q", got)
	}
	if strings.Contains(got, "  1 ") || strings.Contains(got, "# Title") || !strings.Contains(got, "Title") || !strings.Contains(got, "words") {
		t.Errorf("line numbers, or the document not drawn: %q", got)
	}
	if len(failed) != 1 || failed[0] != "nope" {
		t.Errorf("failed %q", failed)
	}
}

// TestStandardInput: what came down a pipe is shown as a file of its format
// is, named standard input: the location row, the App Log; the statusbar has
// its size only; o opens a file in the directory catu was started in
// (task 15).
func TestStandardInput(t *testing.T) {
	t.Setenv("CATU__CONFIG", t.TempDir())
	p, done, err := Spool(strings.NewReader("# Title\n\nwords\n"), "markdown")
	if err != nil {
		t.Fatal(err)
	}
	defer done()
	d := start(t, p, 60, 20)
	want(t, d, "standard input", "DOCUMENT", "SOURCE", " Title ", "Size: 15 B")
	wantNot(t, d, "Modified:", "Mode:", "stdin.md")
	d.Press("!")
	want(t, d, "INFO Opened standard input")
	d.Press("Esc", "o")
	wd, _ := os.Getwd()
	want(t, d, filepath.Base(wd))
	if _, _, err := Spool(strings.NewReader("x"), "nosuch"); err == nil {
		t.Error("an unknown format")
	}
}
