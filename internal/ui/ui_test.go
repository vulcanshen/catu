package ui

import (
	"archive/zip"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	terminu "github.com/vulcanshen/terminu-go"
	"github.com/vulcanshen/terminu-go/tdptest"
)

// files writes files (name → content) in a new directory and returns it.
// HOME and the settings go to a directory of their own, so no test reads the
// user's.
func files(t *testing.T, fs map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("CATU__CONFIG", t.TempDir())
	for name, content := range fs {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// start opens p in catu on a w × h terminal.
func start(t *testing.T, p string, w, h int) *tdptest.Driver {
	t.Helper()
	a, err := Open(p)
	if err != nil {
		t.Fatalf("Open(%s): %v", p, err)
	}
	return tdptest.Start(t, a, w, h)
}

// row is the screen's row holding s, "" when none does.
func row(d *tdptest.Driver, s string) string {
	for _, l := range d.Screen() {
		if strings.Contains(l, s) {
			return l
		}
	}
	return ""
}

func want(t *testing.T, d *tdptest.Driver, texts ...string) {
	t.Helper()
	for _, s := range texts {
		if !d.Has(s) {
			t.Errorf("no %q on the screen:\n%s", s, strings.Join(d.Screen(), "\n"))
		}
	}
}

func wantNot(t *testing.T, d *tdptest.Driver, texts ...string) {
	t.Helper()
	for _, s := range texts {
		if d.Has(s) {
			t.Errorf("%q is on the screen:\n%s", s, strings.Join(d.Screen(), "\n"))
		}
	}
}

func lines(n int, f string) string {
	var b strings.Builder
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&b, f+"\n", i)
	}
	return b.String()
}

// --- task 1: the screen --------------------------------------------------

func TestScreenShowsAFile(t *testing.T) {
	dir := files(t, map[string]string{"main.go": "package main\n\nfunc main() {}\n"})
	tdptest.EachIconWidth(t, func(t *testing.T) {
		for _, sz := range [][2]int{{40, 12}, {80, 24}, {120, 40}} {
			d := start(t, filepath.Join(dir, "main.go"), sz[0], sz[1])
			want(t, d, "[1] TEXT", "main.go", "Size: 29 B", "Modified: ", "q:quit")
			if !strings.HasPrefix(d.Screen()[0], "Size: ") {
				t.Errorf("the statusbar is not the first row: %q", d.Screen()[0])
			}
			if r := row(d, "package main"); !strings.Contains(r, "  1 package main") {
				t.Errorf("line 1 is not numbered: %q", r)
			}
		}
	})
}

func TestStatusbarModeAndDirectorySize(t *testing.T) {
	dir := files(t, map[string]string{"a.txt": "a", "b.txt": "b", ".hidden": "h"})
	d := start(t, dir, 80, 24)
	want(t, d, "Size: 2 items", "Mode: drwx")
	d = start(t, filepath.Join(dir, "a.txt"), 80, 24)
	want(t, d, "Size: 1 B", "Mode: -rw-r--r--")
}

func TestQuitAtOnce(t *testing.T) {
	dir := files(t, map[string]string{"a.txt": "hello\n"})
	d := start(t, filepath.Join(dir, "a.txt"), 80, 24)
	d.Press("q")
	if !d.Quit() {
		t.Error("q did not leave at once")
	}
}

func TestEmptyFile(t *testing.T) {
	dir := files(t, map[string]string{"empty.txt": ""})
	d := start(t, filepath.Join(dir, "empty.txt"), 80, 24)
	want(t, d, "Empty file", "Press [o] to open another file", "empty.txt")
}

// --- task 2: what it opens -------------------------------------------------

func TestOpenLogsWhatItOpened(t *testing.T) {
	dir := files(t, map[string]string{"a.txt": "hello\n"})
	d := start(t, filepath.Join(dir, "a.txt"), 100, 24)
	d.Press("!")
	want(t, d, "INFO Opened", "a.txt")
}

func TestOpenErrors(t *testing.T) {
	dir := files(t, nil)
	if _, err := Open(filepath.Join(dir, "nope")); !os.IsNotExist(err) {
		t.Errorf("a path not there: %v", err)
	}
}

// --- task 3: a text file -------------------------------------------------

func TestTextFoldsALongLine(t *testing.T) {
	long := strings.Repeat("word ", 30) + "END"
	dir := files(t, map[string]string{"a.txt": long + "\nnext\n"})
	d := start(t, filepath.Join(dir, "a.txt"), 60, 20)
	want(t, d, "END", "  2 next")
	n := 0
	for _, l := range d.Screen() {
		if strings.Contains(l, "word") {
			n++
		}
	}
	if n < 2 {
		t.Errorf("the long line is not folded:\n%s", strings.Join(d.Screen(), "\n"))
	}
	if r := row(d, "END"); strings.Contains(r, "  1 ") {
		t.Errorf("a folded row has a number: %q", r)
	}
}

func TestTextColours(t *testing.T) {
	dir := files(t, map[string]string{"main.go": "package main\n\nfunc main() {}\n"})
	d := start(t, filepath.Join(dir, "main.go"), 80, 24)
	styled := strings.Join(d.Styled(), "\n")
	// catppuccin-mocha colours package (a namespace keyword) Teal, #94e2d5.
	if !strings.Contains(styled, "38;2;148;226;213mpackage") {
		t.Error("the Go source is not in catppuccin-mocha")
	}
}

func TestTextTabsAndControlCharacters(t *testing.T) {
	dir := files(t, map[string]string{"a.txt": "a\tb\x07c\r\nnext\r\n10%\r100%\n"})
	d := start(t, filepath.Join(dir, "a.txt"), 80, 24)
	want(t, d, "a    b\\x07c", "  2 next", "  3 100%")
	wantNot(t, d, `\r`, "10%")
}

func TestTextOwnColourCodes(t *testing.T) {
	dir := files(t, map[string]string{"out.log": "\x1b[31mred\x1b[0m plain \x1b[2Jdone\n"})
	d := start(t, filepath.Join(dir, "out.log"), 80, 24)
	want(t, d, "red plain \\x1b[2Jdone")
	if !strings.Contains(strings.Join(d.Styled(), ""), "38;2;243;139;168mred") {
		t.Error("the file's own red is not drawn")
	}
}

func TestTextTooBig(t *testing.T) {
	old := textBytes
	textBytes = 100 << 10
	defer func() { textBytes = old }()
	dir := files(t, map[string]string{"big.txt": lines(20000, "line %d")})
	d := start(t, filepath.Join(dir, "big.txt"), 80, 24)
	d.Press("G")
	want(t, d, "more not shown)")
}

func TestTricky(t *testing.T) {
	for _, s := range tdptest.Tricky {
		dir := files(t, nil)
		name := "file.txt"
		if s != "" && !strings.ContainsAny(s, "/\x00") && len(s) < 200 {
			name = s
		}
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(s+"\n"+s+"\n"), 0o644); err != nil {
			continue // a name the file system takes no file by
		}
		a, err := Open(p)
		if err != nil {
			t.Fatalf("%q: %v", s, err)
		}
		d := tdptest.Start(t, a, 60, 20)
		d.Note(fmt.Sprintf("Tricky %q", s))
		d.AllowLineBreaks = true // the file's own \n, \r by name
		d.Press("v", "j", "$", "Esc", "/")
		d.Type("a")
		d.Press("Enter", "Esc", "!", "Esc")
	}
}

// --- task 4: the other kinds -----------------------------------------------

func TestDirectoryTree(t *testing.T) {
	dir := files(t, map[string]string{
		"b.txt": "", "a/one.txt": "", "a/deep/two.txt": "", "a/deep/deeper/three.txt": "", ".hidden": "",
	})
	d := start(t, dir, 80, 24)
	want(t, d, "[1] DIRECTORY", " "+glyphFolder+" a", "  ├─ "+glyphFolder+" deep", "  │  ├─ "+glyphFolder+" deeper", "  │  └─ "+glyphFile+" two.txt", "  └─ "+glyphFile+" one.txt", " "+glyphFile+" b.txt")
	wantNot(t, d, "three.txt", ".hidden")
	if strings.Index(strings.Join(d.Screen(), "\n"), "a") > strings.Index(strings.Join(d.Screen(), "\n"), "b.txt") {
		t.Error("the folder is not before the file")
	}
}

func TestEmptyDirectory(t *testing.T) {
	dir := files(t, nil)
	d := start(t, dir, 80, 24)
	want(t, d, "Empty directory", "Press [o] to open a file")
}

func TestArchiveTree(t *testing.T) {
	dir := files(t, nil)
	p := filepath.Join(dir, "a.zip")
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, name := range []string{"src/main.go", "README.md"} {
		w, _ := zw.Create(name)
		w.Write([]byte("x"))
	}
	zw.Close()
	os.WriteFile(p, buf.Bytes(), 0o644)
	d := start(t, p, 80, 24)
	want(t, d, "[1] ARCHIVE", " "+glyphFolder+" src", "  └─ "+glyphFile+" main.go", " "+glyphFile+" README.md")
}

// TestEmptyArchive: an archive with nothing in it says so, as an empty
// directory does (tdp screen.8).
func TestEmptyArchive(t *testing.T) {
	dir := files(t, nil)
	var buf bytes.Buffer
	zip.NewWriter(&buf).Close()
	os.WriteFile(filepath.Join(dir, "a.zip"), buf.Bytes(), 0o644)
	d := start(t, filepath.Join(dir, "a.zip"), 80, 24)
	want(t, d, "[1] ARCHIVE", "Empty archive", "Press [o] to open another file")
}

// TestContentUnreadable: an archive or a PDF that opens but cannot be made
// out says why in the middle of the panel, the fact in Red and a hint in
// Overlay0, and the App Log keeps it (tdp panel.15, F5).
func TestContentUnreadable(t *testing.T) {
	dir := files(t, map[string]string{"a.zip": "PK\x03\x04 not a zip", "a.pdf": "%PDF-1.4\nnot a pdf\n"})
	for _, c := range []struct{ file, fact string }{
		{"a.zip", "Can't read this archive: "},
		{"a.pdf", "Can't read this PDF: "},
	} {
		d := start(t, filepath.Join(dir, c.file), 80, 24)
		want(t, d, c.fact, "Press [o] to open another file")
		wantNot(t, d, "(unreadable)")
		red := strings.TrimSuffix(terminu.Fg(terminu.Red).Render("Can't read this"), "\x1b[0m")
		if !strings.Contains(strings.Join(d.Styled(), ""), red) {
			t.Errorf("%s: the fact is not in Red", c.file)
		}
		d.Press("!")
		want(t, d, "ERR  "+c.fact)
	}
}

func TestImageDataURI(t *testing.T) {
	png := "\x89PNG\r\n\x1a\n" + strings.Repeat("\x00", 200)
	dir := files(t, map[string]string{"a.png": png})
	d := start(t, filepath.Join(dir, "a.png"), 60, 20)
	want(t, d, "[1] IMAGE", "data:image/png · 208 B", "data:image/png;base64,iVBORw0KGgo")
}

func TestPDFText(t *testing.T) {
	dir := files(t, map[string]string{"a.pdf": minimalPDF("Hello PDF")})
	d := start(t, filepath.Join(dir, "a.pdf"), 80, 24)
	want(t, d, "[1] PDF", "1 page", "Hello PDF")
}

// minimalPDF is a one-page PDF with text on it.
func minimalPDF(text string) string {
	stream := "BT /F1 12 Tf 72 712 Td (" + text + ") Tj ET"
	objs := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Contents 4 0 R /Resources << /Font << /F1 5 0 R >> >> >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(stream), stream),
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
	}
	var b strings.Builder
	b.WriteString("%PDF-1.4\n")
	offs := make([]int, len(objs))
	for i, o := range objs {
		offs[i] = b.Len()
		fmt.Fprintf(&b, "%d 0 obj\n%s\nendobj\n", i+1, o)
	}
	xref := b.Len()
	fmt.Fprintf(&b, "xref\n0 %d\n0000000000 65535 f \n", len(objs)+1)
	for _, o := range offs {
		fmt.Fprintf(&b, "%010d 00000 n \n", o)
	}
	fmt.Fprintf(&b, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objs)+1, xref)
	return b.String()
}

func TestBinaryHex(t *testing.T) {
	dir := files(t, map[string]string{"a.bin": "\x00\x01ABCDEFGH"})
	d := start(t, filepath.Join(dir, "a.bin"), 60, 20)
	want(t, d, "[1] BINARY", "00 01 41 42 43 44 45 46  ..ABCDEF", "47 48")
}

func TestBinaryTooBig(t *testing.T) {
	old := hexBytes
	hexBytes = 64
	defer func() { hexBytes = old }()
	dir := files(t, map[string]string{"a.bin": "\x00" + strings.Repeat("x", 200)})
	d := start(t, filepath.Join(dir, "a.bin"), 60, 20)
	want(t, d, "(137 B more not shown)")
}

// TestHideSecrets: hide_secrets_in_preview is the popup previews' (tdp
// file-picker.5): the file-picker's preview hides a private key, the panel
// shows the file the user opened.
func TestHideSecrets(t *testing.T) {
	dir := files(t, map[string]string{
		"id_ed25519": "-----BEGIN OPENSSH PRIVATE KEY-----\nsecret\n",
		"token":      "secret\n",
	})
	os.Chmod(filepath.Join(dir, "token"), 0o600)
	os.WriteFile(filepath.Join(os.Getenv("CATU__CONFIG"), "config.yaml"), []byte("hide_secrets_in_preview: true\n"), 0o644)
	d := start(t, filepath.Join(dir, "id_ed25519"), 100, 30)
	want(t, d, "[1] PRIVATE KEY", "  2 secret")
	wantNot(t, d, "(sensitive content not shown)")
	d.Press("o") // the cursor on the key: its preview
	want(t, d, "(sensitive content not shown)")
	d = start(t, filepath.Join(dir, "token"), 100, 30)
	want(t, d, "  1 secret")
}

// --- task 5: moving, the Visual mode ---------------------------------------

func TestScrollKeys(t *testing.T) {
	dir := files(t, map[string]string{"a.txt": lines(100, "line %d")})
	d := start(t, filepath.Join(dir, "a.txt"), 60, 20)
	want(t, d, "1-15 of 100")
	d.Press("j")
	want(t, d, "2-16 of 100")
	d.Press("d")
	want(t, d, "9-23 of 100")
	d.Press("u")
	want(t, d, "2-16 of 100")
	d.Press("G")
	want(t, d, "86-100 of 100", "line 100")
	d.Press("k")
	want(t, d, "85-99 of 100")
	d.Press("g", "g")
	want(t, d, "1-15 of 100")
	d.Press("Ctrl-D")
	want(t, d, "1-15 of 100") // not an alias (tdp K12)
}

func TestVisualSelectsAndCopies(t *testing.T) {
	dir := files(t, map[string]string{"a.txt": "one two three\nfour five\n"})
	d := start(t, filepath.Join(dir, "a.txt"), 60, 20)
	d.Press("v")
	want(t, d, "Visual", "v:select", "Esc:cancel")
	d.Press("w", "v")
	want(t, d, "Visual>select", "y:copy", "Esc:unselect")
	d.Press("j", "0", "e")
	d.Press("y")
	if got := d.Clipboard(); len(got) != 1 || got[0] != "two three\nfour" {
		t.Errorf("copied %q", got)
	}
	want(t, d, "Copied")
	d.Settle()
	wantNot(t, d, "┫Visual")
}

// TestVisualSelectionPlain: in the selected range the syntax colours and
// their bold, italic and underline do not show, only Base on Yellow (tdp
// note.2).
func TestVisualSelectionPlain(t *testing.T) {
	dir := files(t, map[string]string{"a.go": "// a comment\n"})
	d := start(t, filepath.Join(dir, "a.go"), 60, 20)
	styled := func() string {
		for i, l := range d.Screen() {
			if strings.Contains(l, "a comment") {
				return d.Styled()[i]
			}
		}
		t.Fatal("no comment on the screen")
		return ""
	}
	const italic = "\x1b[3;"
	if !strings.Contains(styled(), italic) {
		t.Fatal("the comment is not italic before it is selected")
	}
	d.Press("v", "v", "$")
	if got := styled(); strings.Contains(got, italic) || !strings.Contains(got, selectStyle.SGR()+"/") {
		t.Errorf("the selection shows the syntax's look: %q", got)
	}
}

func TestVisualEscOneLayerAtATime(t *testing.T) {
	dir := files(t, map[string]string{"a.txt": "one two\n"})
	d := start(t, filepath.Join(dir, "a.txt"), 60, 20)
	d.Press("v", "v", "Esc")
	d.Settle()
	want(t, d, "┫Visual┣")
	wantNot(t, d, "Visual>select")
	d.Press("Esc")
	d.Settle()
	wantNot(t, d, "┫Visual")
}

func TestVisualMoves(t *testing.T) {
	dir := files(t, map[string]string{"a.txt": "alpha beta.gamma\nzeta\n"})
	d := start(t, filepath.Join(dir, "a.txt"), 60, 20)
	d.Press("v", "$", "v", "0", "y") // the whole line, from its end back to its start
	d.Press("v", "w", "w", "v", "e", "y")
	d.Press("v", "G", "v", "g", "g", "y")
	d.Press("v", "l", "l", "v", "h", "y")
	want := []string{"alpha beta.gamma", ".gamma", "alpha beta.gamma\nz", "lp"}
	got := d.Clipboard()
	if len(got) != len(want) {
		t.Fatalf("copied %q", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("copy %d: %q, want %q", i, got[i], want[i])
		}
	}
}

func TestVisualSpaceMenu(t *testing.T) {
	dir := files(t, map[string]string{"a.txt": "one two\n"})
	d := start(t, filepath.Join(dir, "a.txt"), 60, 20)
	d.Press("v", "Space")
	want(t, d, "[v] Select")
	d.Press("Esc", "v", "Space")
	want(t, d, "Cop[y]")
}

// --- task 6: search --------------------------------------------------------

func TestSearchFindsAndSteps(t *testing.T) {
	dir := files(t, map[string]string{"a.txt": lines(60, "line %d") + "needle one\n" + lines(30, "more %d") + "Needle two\n"})
	d := start(t, filepath.Join(dir, "a.txt"), 60, 20)
	d.Press("/")
	want(t, d, "SEARCH", "Find in a.txt")
	d.Type("needle")
	d.Press("Enter")
	want(t, d, "needle one")
	d.Press("n")
	want(t, d, "Needle two")
	d.Press("n")
	want(t, d, "needle one") // round to the first
	d.Press("N")
	want(t, d, "Needle two")
	d.Press("Space")
	want(t, d, "[n]ext", "[N] Previous")
	d.Press("Esc", "Esc") // the menu, then the hits
	d.Press("Space")
	wantNot(t, d, "[n]ext")
}

func TestSearchNoHit(t *testing.T) {
	dir := files(t, map[string]string{"a.txt": "hello\n"})
	d := start(t, filepath.Join(dir, "a.txt"), 60, 20)
	d.Press("/")
	d.Type("zzz")
	d.Press("Enter")
	want(t, d, "No line has zzz")
}

func TestSearchHitsDrawn(t *testing.T) {
	dir := files(t, map[string]string{"a.txt": "ab ab\n"})
	d := start(t, filepath.Join(dir, "a.txt"), 60, 20)
	d.Press("/")
	d.Type("ab")
	d.Press("Enter")
	styled := strings.Join(d.Styled(), "\n")
	if !strings.Contains(styled, cursorStyle.SGR()) {
		t.Error("the current hit is not the cursor block")
	}
	if !strings.Contains(styled, "\x1b[4;") {
		t.Error("the other hit is not underlined")
	}
}

// TestSearchFromTheSpaceMenu: the search's input popup is a gate. Esc on it
// goes back to the menu; Enter searches, and the input popup and the menu
// close together, found or not (tdp T1, menu.3, note.3).
func TestSearchFromTheSpaceMenu(t *testing.T) {
	dir := files(t, map[string]string{"a.txt": "needle one\nhay\n"})
	d := start(t, filepath.Join(dir, "a.txt"), 60, 20)
	d.Press("Space", "/")
	want(t, d, "SEARCH")
	d.Press("Esc")
	want(t, d, "PANEL OPERATION") // cancelled: back to the menu (tdp F4)
	wantNot(t, d, "SEARCH")
	d.Press("/")
	d.Type("needle")
	d.Press("Enter")
	wantNot(t, d, "PANEL OPERATION", "SEARCH") // searched: the menu went too (tdp T1)
	if !strings.Contains(strings.Join(d.Styled(), "\n"), cursorStyle.SGR()) {
		t.Error("the hit is not in sight")
	}
	d.Press("Space", "/")
	d.Type("zzz")
	d.Press("Enter")
	want(t, d, "No line has zzz")
	wantNot(t, d, "PANEL OPERATION", "SEARCH") // nothing found: the menu goes all the same
}

// --- task 7: opening another file ------------------------------------------

func TestOpenAnotherFile(t *testing.T) {
	dir := files(t, map[string]string{"a.txt": "first\n", "b.txt": "second\n"})
	d := start(t, filepath.Join(dir, "a.txt"), 100, 30)
	d.Press("o")
	want(t, d, "OPEN · ")
	d.Press("j", "Enter")
	want(t, d, "  1 second", "b.txt")
	d.Press("!")
	if n := strings.Count(strings.Join(d.Screen(), "\n"), "INFO Opened"); n != 2 {
		t.Errorf("%d Opened in the App Log, want 2:\n%s", n, strings.Join(d.Screen(), "\n"))
	}
}

func TestOpenFromTheSpaceMenu(t *testing.T) {
	dir := files(t, map[string]string{"a.txt": "first\n", "b.txt": "second\n"})
	d := start(t, filepath.Join(dir, "a.txt"), 100, 30)
	d.Press("Space")
	want(t, d, "[o]pen")
	d.Press("o")
	d.Press("Esc")
	want(t, d, "PANEL OPERATION") // cancelled: back to the menu (tdp F4)
	d.Press("o", "j", "Enter")
	want(t, d, "  1 second")
	wantNot(t, d, "PANEL OPERATION") // chosen: the menu went too (tdp T1)
}

func TestOpenUnreadable(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads everything")
	}
	dir := files(t, map[string]string{"a.txt": "first\n", "b.txt": "second\n"})
	os.Chmod(filepath.Join(dir, "b.txt"), 0)
	d := start(t, filepath.Join(dir, "a.txt"), 100, 30)
	d.Press("o", "j", "Enter")
	want(t, d, "permission denied", "OPEN · ")
	d.Press("Enter", "Esc")
	want(t, d, "  1 first")
	d.Press("!") // the error is kept (tdp F5)
	want(t, d, "ERR  Open failed")
}

// TestOpenTheSameFile: Enter on the file shown changes nothing: the
// file-picker closes at once, back where it was opened from (tdp input.1,
// K3).
func TestOpenTheSameFile(t *testing.T) {
	dir := files(t, map[string]string{"a.txt": "first\n", "b.txt": "second\n"})
	d := start(t, filepath.Join(dir, "a.txt"), 100, 30)
	d.Press("o", "Enter")
	wantNot(t, d, "OPEN · ")
	want(t, d, "  1 first")
	d.Press("Space", "o", "Enter")
	wantNot(t, d, "OPEN · ")
	want(t, d, "PANEL OPERATION")
	d.Press("Esc", "!")
	if got := strings.Count(strings.Join(d.Screen(), "\n"), "Opened"); got != 1 {
		t.Errorf("opened %d times, want once, at the start", got)
	}
}

func TestOpenFromADirectory(t *testing.T) {
	dir := files(t, map[string]string{"a.txt": "first\n"})
	d := start(t, dir, 100, 30)
	d.Press("o", "j", "Enter") // from .. to the file
	want(t, d, "  1 first")
}

// --- task 8: settings ------------------------------------------------------

func TestSettingsTabWidth(t *testing.T) {
	dir := files(t, map[string]string{"a.txt": "a\tb\n"})
	os.WriteFile(filepath.Join(os.Getenv("CATU__CONFIG"), "config.yaml"), []byte("input_tab_width: 2\n"), 0o644)
	d := start(t, filepath.Join(dir, "a.txt"), 60, 20)
	want(t, d, "  1 a  b")
}

func TestSettingsUnreadable(t *testing.T) {
	dir := files(t, map[string]string{"a.txt": "a\tb\n"})
	cfg := filepath.Join(os.Getenv("CATU__CONFIG"), "config.yaml")
	os.WriteFile(cfg, []byte("input_tab_width: [\n"), 0o644)
	d := start(t, filepath.Join(dir, "a.txt"), 80, 24)
	want(t, d, "MOVE ASIDE", "config.yaml.bak")
	d.Press("Enter")
	if _, err := os.Stat(cfg + ".bak"); err != nil {
		t.Errorf("not moved aside: %v", err)
	}
	want(t, d, "  1 a    b")
}

func TestSettingsUnreadableQuit(t *testing.T) {
	dir := files(t, map[string]string{"a.txt": "a\n"})
	os.WriteFile(filepath.Join(os.Getenv("CATU__CONFIG"), "config.yaml"), []byte(":\n:"), 0o644)
	d := start(t, filepath.Join(dir, "a.txt"), 80, 24)
	d.Press("Esc")
	if !d.Quit() {
		t.Error("Esc did not leave")
	}
}

// TestSettingsDirOrder: the settings directory is CATU__CONFIG, else
// $XDG_CONFIG_HOME/catu, else ~/.config/catu (tdp E2, E3).
func TestSettingsDirOrder(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CATU__CONFIG", "/tmp/own")
	t.Setenv("XDG_CONFIG_HOME", "/tmp/xdg")
	if got := configDir(); got != "/tmp/own" {
		t.Errorf("with CATU__CONFIG: %s", got)
	}
	t.Setenv("CATU__CONFIG", "")
	if got := configDir(); got != filepath.Join("/tmp/xdg", "catu") {
		t.Errorf("with XDG_CONFIG_HOME: %s", got)
	}
	t.Setenv("XDG_CONFIG_HOME", "")
	if got := configDir(); got != filepath.Join(home, ".config", "catu") {
		t.Errorf("with neither: %s", got)
	}
}
