package ui

import (
	"bytes"
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
