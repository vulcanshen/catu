package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vulcanshen/terminu-go/tdptest"
)

// change writes content to p as an editor saving it does, its modification
// time a step later each time.
func change(t *testing.T, p, content string) {
	t.Helper()
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	changed++
	at := time.Now().Add(time.Duration(changed) * time.Second)
	if err := os.Chtimes(p, at, at); err != nil {
		t.Fatal(err)
	}
}

var changed int

// look lets watchEvery go by: catu looks at the file shown.
func look(d *tdptest.Driver) { d.Advance(watchEvery) }

// TestWatchReadsAgain: the file shown, changed on the disk, is read again
// within watchEvery, without a key pressed, and the App Log says so (task 17,
// tdp note.7); unchanged, it is not.
func TestWatchReadsAgain(t *testing.T) {
	dir := files(t, map[string]string{"a.txt": "first words\n"})
	p := filepath.Join(dir, "a.txt")
	d := start(t, p, 60, 20)
	want(t, d, "first words")
	look(d)
	d.Press("!")
	wantNot(t, d, "Reloaded")
	d.Press("Esc")
	change(t, p, "second words, longer\n")
	look(d)
	want(t, d, "second words, longer")
	wantNot(t, d, "first words")
	d.Press("l") // its META is the new one's
	want(t, d, "SIZE      21 B")
	d.Press("h")
	d.Press("!")
	want(t, d, "INFO Reloaded") // then the path, folded
}

// TestWatchKeepsThePlace: read again, the view stays on the line it was on;
// at the end, it follows what was added, as tail -f (task 17).
func TestWatchKeepsThePlace(t *testing.T) {
	dir := files(t, map[string]string{"log.txt": lines(100, "line %d")})
	p := filepath.Join(dir, "log.txt")
	d := start(t, p, 60, 20)
	d.Press("d", "d")
	want(t, d, " line 17 ")
	change(t, p, lines(100, "line %d")+"line 101\n")
	look(d)
	want(t, d, " line 17 ", "of 101")
	wantNot(t, d, "line 101")
	d.Press("G")
	want(t, d, "line 101")
	change(t, p, lines(100, "line %d")+"line 101\nline 102\n")
	look(d)
	want(t, d, "line 102", "of 102")
}

// TestWatchFindsAgain: a search's hits are found again in what was read,
// the view not moved to them, the status on the border counting them (task
// 17, tdp note.3).
func TestWatchFindsAgain(t *testing.T) {
	dir := files(t, map[string]string{"a.txt": "needle one\nhay\nneedle two\n"})
	p := filepath.Join(dir, "a.txt")
	d := start(t, p, 60, 20)
	d.Press("/")
	d.Type("needle")
	d.Press("Enter", "n")
	want(t, d, "/needle 2 of 2")
	change(t, p, "needle one\nhay\nneedle two\nneedle three\n")
	look(d)
	want(t, d, "needle three", "/needle 2 of 3")
	change(t, p, "hay only\n")
	look(d)
	wantNot(t, d, "/needle")
}

// TestWatchInVisual: the Visual mode only selects and copies, so the file
// is read again in it too (the user, 2026-11-16): still in the mode, the
// cursor and where the selection started where they were, and what is
// copied is the text as it is now; where the text no longer reaches, they
// go to its last character. No toast: the screen says it (the user) (task
// 17).
func TestWatchInVisual(t *testing.T) {
	dir := files(t, map[string]string{"a.txt": "old text\nsecond line\n"})
	p := filepath.Join(dir, "a.txt")
	d := start(t, p, 60, 20)
	d.Press("v", "v", "e") // select "old"
	change(t, p, "new text\nsecond line\n")
	look(d)
	want(t, d, "new text", "select", "y:copy")
	wantNot(t, d, "INFO")
	d.Press("y")
	if got := d.Clipboard(); len(got) != 1 || got[0] != "new" {
		t.Errorf("copied %q, want the text as it is now", got)
	}
	d.Press("v", "j", "$", "v") // the cursor at the end of the second line
	change(t, p, "a\n")
	look(d)
	want(t, d, "select")
	d.Press("y")
	if got := d.Clipboard(); len(got) != 2 || got[1] != "a" {
		t.Errorf("copied %q, want the last character there is", got)
	}
}

// TestWatchGone: a file gone stays shown as it was, said once in a Warning;
// back, it is read again (task 17).
func TestWatchGone(t *testing.T) {
	dir := files(t, map[string]string{"a.txt": "still here\n"})
	p := filepath.Join(dir, "a.txt")
	d := start(t, p, 60, 20)
	os.Remove(p)
	look(d)
	want(t, d, "still here", "a.txt is no longer there: shown as it was")
	d.Advance(10 * time.Second) // the toast goes, and no other comes
	look(d)
	wantNot(t, d, "no longer there")
	change(t, p, "back again\n")
	look(d)
	want(t, d, "back again")
	d.Press("!")
	if n := strings.Count(strings.Join(d.Screen(), "\n"), "no longer there"); n != 1 {
		t.Errorf("the Warning is in the App Log %d times, want once", n)
	}
}

// TestWatchKeepsTheTab: a Markdown file read again stays on the tab it was
// on (task 17).
func TestWatchKeepsTheTab(t *testing.T) {
	dir := files(t, map[string]string{"a.md": "# Title\n\nold\n"})
	p := filepath.Join(dir, "a.md")
	d := start(t, p, 60, 20)
	d.Press("l")
	want(t, d, "# Title")
	change(t, p, "# Title\n\nnew line\n")
	look(d)
	want(t, d, "# Title", "new line") // the source, as it is now
	d.Press("h")
	want(t, d, "new line")
	wantNot(t, d, "# Title")
}

// TestWatchNotThePipe: what came down a pipe is not a file to watch: its
// copy is not read again (task 17).
func TestWatchNotThePipe(t *testing.T) {
	t.Setenv("CATU__CONFIG", t.TempDir())
	var done func()
	p, err := Spool(strings.NewReader("from the pipe\n"), "", func(d func()) { done = d })
	if err != nil {
		t.Fatal(err)
	}
	defer done()
	defer func() { stdinPath = "" }()
	d := start(t, p, 60, 20)
	change(t, p, "changed under it\n")
	look(d)
	want(t, d, "from the pipe")
}

// TestWatchSeveralFiles: with a FILES list, the file shown is the one
// watched; one changed while another was shown is read again once it is
// shown (task 17, task 16).
func TestWatchSeveralFiles(t *testing.T) {
	dir := files(t, map[string]string{"a.txt": "first\n", "b.txt": "second\n"})
	a, b := filepath.Join(dir, "a.txt"), filepath.Join(dir, "b.txt")
	d := startAll(t, []string{a, b}, 80, 20)
	change(t, b, "second, changed\n")
	look(d)
	want(t, d, "  1 first")
	d.Press("j")
	look(d)
	want(t, d, "second, changed")
	d.Press("k", "j") // shown again from the list: as it is now
	want(t, d, "second, changed")
}
