package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	terminu "github.com/vulcanshen/terminu-go"
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

// synced is the sync status on the screen of a read in the last look: on
// the App's clock, at a tick in the last watchEvery, so its second is now's
// or the one before (a key's animations move the clock too).
func synced(t *testing.T, d *tdptest.Driver) string {
	t.Helper()
	for _, at := range []time.Time{d.Now(), d.Now().Add(-watchEvery)} {
		if s := syncGlyph + " " + at.Format("15:04:05"); d.Has(s) {
			return s
		}
	}
	t.Errorf("no sync status of a read in the last %v:\n%s", watchEvery, strings.Join(d.Screen(), "\n"))
	return "\x00" // on no screen
}

// bottom is the viewer panel's bottom border, as it reads and as it is drawn.
func bottom(t *testing.T, d *tdptest.Driver) (string, string) {
	t.Helper()
	s := d.Screen()
	for i := len(s) - 1; i >= 0; i-- {
		if strings.Contains(s[i], "┛") {
			return s[i], d.Styled()[i]
		}
	}
	t.Fatal("no bottom border")
	return "", ""
}

// TestWatchSyncStatus: the viewer panel's frame says at the bottom right
// that the file shown is watched, and when it was last read (tdp panel.1,
// the user, task 17): the same while the file is, the new read's once read
// again; Red while the file is gone, still the last read's.
func TestWatchSyncStatus(t *testing.T) {
	dir := files(t, map[string]string{"a.txt": "first\n"})
	p := filepath.Join(dir, "a.txt")
	d := start(t, p, 60, 20)
	first, _ := bottom(t, d)
	if !strings.Contains(first, "━ "+syncGlyph+" ") || !strings.HasSuffix(first, " ━┛") {
		t.Fatalf("no sync status at the bottom right: %q", first)
	}
	look(d)
	if now, _ := bottom(t, d); now != first {
		t.Errorf("the file unchanged, the status is %q, want %q", now, first)
	}
	change(t, p, "second\n")
	look(d)
	at := synced(t, d)
	if b, _ := bottom(t, d); !strings.Contains(b, "━ "+at+" ━") {
		t.Errorf("read again, the status is not %q: %q", at, b)
	}
	red := strings.TrimSuffix(terminu.Fg(terminu.Red).Render(at), "\x1b[0m")
	os.Remove(p)
	look(d)
	if b, styled := bottom(t, d); !strings.Contains(b, at) || !strings.Contains(styled, red) {
		t.Errorf("gone, the status is not %q in Red: %q", at, styled)
	}
	change(t, p, "back\n")
	look(d)
	back := synced(t, d)
	if back == at {
		t.Errorf("back, the status is still %q", at)
	}
	if _, styled := bottom(t, d); strings.Contains(styled, strings.TrimSuffix(terminu.Fg(terminu.Red).Render("x"), "x\x1b[0m")) {
		t.Errorf("back, the status is not in the frame's colour: %q", styled)
	}
}

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
// at the end, it follows what was added, as tail -f; shorter than the
// view, the view is at its new end (tdp note.9, task 17).
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
	change(t, p, lines(10, "line %d")) // shorter than the view: at its new end (tdp note.9)
	look(d)
	want(t, d, "  1 line 1 ", "line 10")
}

// TestWatchFindsAgain: read again, a search is found again in the new text
// (tdp note.9): the current hit the first at or after where it was, the
// view not moved, the status counting them; none left, a search that found
// nothing, an Info toast saying so, the menu open over it not changed (task
// 17).
func TestWatchFindsAgain(t *testing.T) {
	dir := files(t, map[string]string{"a.txt": "needle one\nhay\nneedle two\n"})
	p := filepath.Join(dir, "a.txt")
	d := start(t, p, 80, 20) // room for the sync status too
	d.Press("/")
	d.Type("needle")
	d.Press("Enter", "n")
	want(t, d, "/needle 2 of 2")
	change(t, p, "needle zero\nneedle one\nhay\nneedle two\nneedle three\n")
	look(d)
	// "needle two" is current still, now the third; the sync status left of it (tdp panel.1)
	want(t, d, "needle three", synced(t, d)+" ━ /needle 3 of 4")
	d.Press("Space")
	want(t, d, "Previous")
	change(t, p, "hay only\n")
	look(d)
	want(t, d, "Previous", "INFO", "No line has needle")
	wantNot(t, d, "/needle")
}

// TestWatchInVisual: the Visual mode only selects and copies, so the file
// is read again in it too, its positions kept, not its text: the cursor and
// where the selection started both still there, nothing changes and y
// copies the text there now, with no toast (tdp note.9, task 17).
func TestWatchInVisual(t *testing.T) {
	dir := files(t, map[string]string{"a.txt": "old text\nsecond line\n"})
	p := filepath.Join(dir, "a.txt")
	d := start(t, p, 60, 20)
	d.Press("v", "v", "e") // select "old"
	change(t, p, "new text\nsecond line\n")
	look(d)
	want(t, d, "new text", "Visual>select", "y:copy")
	wantNot(t, d, "INFO", "WARNING")
	d.Press("y")
	if got := d.Clipboard(); len(got) != 1 || got[0] != "new" {
		t.Errorf("copied %q, want the text there now", got)
	}
}

// TestWatchVisualStartGone: read again, where the selection started is gone
// and the cursor's place is not: the selection goes, back to Visual as one
// Esc, the cursor where it was, said in a Warning (tdp note.9, task 17).
func TestWatchVisualStartGone(t *testing.T) {
	dir := files(t, map[string]string{"a.txt": "a\nb\nc\nd\n"})
	p := filepath.Join(dir, "a.txt")
	d := start(t, p, 60, 20)
	d.Press("v", "j", "j", "j", "v", "k", "k") // from d up to b
	change(t, p, "x\ny\n")
	look(d)
	want(t, d, "┫Visual┣", "WARNING", "a.txt changed: where the selection started is gone")
	wantNot(t, d, "Visual>select")
	d.Press("v", "y") // from the cursor, still on the second line
	if got := d.Clipboard(); len(got) != 1 || got[0] != "y" {
		t.Errorf("copied %q, want the cursor where it was", got)
	}
}

// TestWatchVisualCursorGone: read again, where the cursor was is gone: the
// mode goes, as Esc to the end, said in a Warning. A file that cannot be
// read leaves it too, its Error the only toast (tdp note.9, task 17).
func TestWatchVisualCursorGone(t *testing.T) {
	dir := files(t, map[string]string{"a.txt": "old text\nsecond line\n"})
	p := filepath.Join(dir, "a.txt")
	d := start(t, p, 60, 20)
	d.Press("v", "j", "$") // the cursor on the e of "line"
	change(t, p, "old text\nsecond\n")
	look(d)
	want(t, d, "WARNING", "a.txt changed under the cursor: Visual left", "q:quit")
	wantNot(t, d, "┫Visual")
	d.Press("Esc") // the toast
	d.Press("v", "v")
	os.Remove(p)
	look(d)
	want(t, d, "ERROR", "a.txt is no longer there", "q:quit")
	wantNot(t, d, "┫Visual", "WARNING")
}

// TestWatchVisualMenuOpen: with the Visual mode's Space menu or ? open, a
// read that takes the selection away says so in a note over them, not a
// toast: a Warning's in Peach, an error popup when the file cannot be read;
// Enter or Esc closes it and them together, back to the panel (tdp note-99,
// note-100, task 17).
func TestWatchVisualMenuOpen(t *testing.T) {
	dir := files(t, map[string]string{"a.txt": "a\nb\nc\nd\n"})
	p := filepath.Join(dir, "a.txt")
	d := start(t, p, 70, 24)
	d.Press("v", "j", "j", "j", "v", "k", "k", "Space") // from d up to b, the menu open
	change(t, p, "x\ny\n")
	look(d)
	want(t, d, "READ AGAIN", "a.txt changed: where the selection started is gone", "Enter|Esc:close", "ITEM OPERATION")
	wantNot(t, d, "WARNING")
	d.Press("Enter")
	wantNot(t, d, "READ AGAIN", "ITEM OPERATION")
	want(t, d, "┫Visual┣") // back to Visual, as one Esc
	d.Press("?")
	change(t, p, "x\n") // the cursor's line gone
	look(d)
	want(t, d, "READ AGAIN", "a.txt changed under the cursor: Visual left", "VISUAL KEYS")
	d.Press("Esc")
	wantNot(t, d, "READ AGAIN", "VISUAL KEYS", "┫Visual")
	d.Press("v", "v", "Space")
	os.Remove(p)
	look(d)
	want(t, d, "CANNOT READ", "a.txt is no longer there: shown as it was", "The selection is gone", "ITEM OPERATION")
	wantNot(t, d, "ERROR")
	d.Press("Esc")
	wantNot(t, d, "CANNOT READ", "ITEM OPERATION", "┫Visual")
	change(t, p, "back\n") // and gone again, the note closed with Enter
	look(d)
	d.Press("v", "v", "?")
	os.Remove(p)
	look(d)
	want(t, d, "CANNOT READ", "VISUAL>SELECT KEYS")
	d.Press("Enter")
	wantNot(t, d, "CANNOT READ", "VISUAL>SELECT KEYS", "┫Visual")
	d.Press("!") // the notes are in the App Log as their toasts would be (tdp F5, note.7)
	want(t, d, "WARN", "Read again: a.txt changed", "ERR", "Cannot read: a.txt is no longer")
}

// TestWatchNumbersOnlyWiden: read again, the number column only widens while
// the file is open (tdp note.9, task 17).
func TestWatchNumbersOnlyWiden(t *testing.T) {
	dir := files(t, map[string]string{"a.txt": lines(50, "line %d")})
	p := filepath.Join(dir, "a.txt")
	d := start(t, p, 60, 20)
	want(t, d, "┃   1 line 1 ")
	change(t, p, lines(1000, "line %d"))
	look(d)
	want(t, d, "┃    1 line 1 ")
	change(t, p, lines(50, "line %d"))
	look(d)
	want(t, d, "┃    1 line 1 ")
}

// TestWatchVisualStaysPut: at the end, the Visual mode does not follow what
// is added: the cursor stays in sight (the user, 2026-11-16, task 17).
func TestWatchVisualStaysPut(t *testing.T) {
	dir := files(t, map[string]string{"log.txt": lines(100, "line %d")})
	p := filepath.Join(dir, "log.txt")
	d := start(t, p, 60, 20)
	d.Press("G", "v", "G") // the cursor on the last line
	change(t, p, lines(130, "line %d"))
	look(d)
	want(t, d, " line 85 ", " line 100 ", "Visual")
	wantNot(t, d, "line 101")
}

// TestWatchGone: a file gone stays shown as it was, said once in an Error
// (the user); back, it is read again (task 17).
func TestWatchGone(t *testing.T) {
	dir := files(t, map[string]string{"a.txt": "still here\n"})
	p := filepath.Join(dir, "a.txt")
	d := start(t, p, 60, 20)
	os.Remove(p)
	look(d)
	want(t, d, "still here", "ERROR", "a.txt is no longer there: shown as it was")
	d.Advance(10 * time.Second) // the toast goes, and no other comes
	look(d)
	wantNot(t, d, "no longer there")
	change(t, p, "back again\n")
	look(d)
	want(t, d, "back again")
	d.Press("!")
	if n := strings.Count(strings.Join(d.Screen(), "\n"), "no longer there"); n != 1 {
		t.Errorf("the Error is in the App Log %d times, want once", n)
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
	wantNot(t, d, syncGlyph) // and no sync status
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
