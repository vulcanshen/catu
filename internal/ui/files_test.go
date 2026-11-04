package ui

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	terminu "github.com/vulcanshen/terminu-go"
	"github.com/vulcanshen/terminu-go/tdptest"
)

// startAll opens paths in catu on a w × h terminal.
func startAll(t *testing.T, paths []string, w, h int) *tdptest.Driver {
	t.Helper()
	t.Setenv("CATU__CONFIG", t.TempDir())
	a, err := OpenAll(paths)
	if err != nil {
		t.Fatalf("OpenAll: %v", err)
	}
	return tdptest.Start(t, a, w, h)
}

// TestSeveralFiles: two or more paths are listed in a FILES panel [1], the
// viewer [2] beside it showing the one under the list's cursor as it moves,
// the focus staying on the list; Enter sends it to the viewer (tdp panel.13,
// task 16). Each is logged as opened; one named twice is listed once.
func TestSeveralFiles(t *testing.T) {
	dir := files(t, map[string]string{"a.txt": "first\n", "b.md": "# Second\n"})
	a, b := filepath.Join(dir, "a.txt"), filepath.Join(dir, "b.md")
	tdptest.EachIconWidth(t, func(t *testing.T) {
		d := startAll(t, []string{a, b, a}, 80, 20)
		want(t, d, "FILES", "  1 first", "Tab|1–2:panels", "Enter:view")
		if n := strings.Count(strings.Join(d.Screen(), "\n"), "a.txt"); n != 2 { // the list, the location row
			t.Errorf("a.txt %d times:\n%s", n, strings.Join(d.Screen(), "\n"))
		}
		d.Press("j")
		want(t, d, " Second ", "DOCUMENT", "b.md")
		wantNot(t, d, "  1 first")
		d.Press("/") // the list has no search: the focus is still on it
		wantNot(t, d, "Find in")
		d.Press("Enter", "/")
		want(t, d, "Find in b.md")
		d.Press("Esc", "!")
		if n := strings.Count(strings.Join(d.Screen(), "\n"), "INFO Opened"); n != 2 {
			t.Errorf("%d Opened in the App Log, want 2", n)
		}
	})
}

// TestSeveralFilesNarrow: under 60 columns only the panel with the focus is
// drawn, the whole width (tdp screen.5); Tab goes to the other.
func TestSeveralFilesNarrow(t *testing.T) {
	dir := files(t, map[string]string{"a.txt": "first\n", "b.txt": "second\n"})
	d := startAll(t, []string{filepath.Join(dir, "a.txt"), filepath.Join(dir, "b.txt")}, 50, 16)
	want(t, d, "FILES", "b.txt")
	wantNot(t, d, "  1 first")
	d.Press("Tab")
	want(t, d, "  1 first")
	wantNot(t, d, "FILES")
	d.Resize(80, 16)
	want(t, d, "FILES", "  1 first")
}

// TestSeveralFilesOpen: o adds the file chosen to the list, last, and puts
// the cursor on it; one listed already is gone to, not listed again.
func TestSeveralFilesOpen(t *testing.T) {
	dir := files(t, map[string]string{"a.txt": "first\n", "b.txt": "second\n", "c.txt": "third\n"})
	d := startAll(t, []string{filepath.Join(dir, "a.txt"), filepath.Join(dir, "b.txt")}, 100, 24)
	d.Press("Tab", "o")
	want(t, d, "OPEN · ")
	d.Press("j", "j", "Enter")
	want(t, d, "  1 third", "c.txt")
	if n := strings.Count(strings.Join(d.Screen(), "\n"), "c.txt"); n != 2 {
		t.Errorf("c.txt %d times, want the list and the location row:\n%s", n, strings.Join(d.Screen(), "\n"))
	}
	d.Press("o", "k", "k", "Enter")
	want(t, d, "  1 first")
	if n := strings.Count(strings.Join(d.Screen(), "\n"), "a.txt"); n != 2 {
		t.Errorf("a.txt listed again:\n%s", strings.Join(d.Screen(), "\n"))
	}
}

// TestSeveralFilesUnreadable: one path that cannot be read stops catu
// before anything is drawn, the path as it was given in the error.
func TestSeveralFilesUnreadable(t *testing.T) {
	dir := files(t, map[string]string{"a.txt": "first\n"})
	nope := filepath.Join(dir, "nope")
	_, err := OpenAll([]string{filepath.Join(dir, "a.txt"), nope})
	var oe *OpenError
	if !errors.As(err, &oe) || oe.Path != nope || !os.IsNotExist(oe.Err) {
		t.Errorf("err %v", err)
	}
}

// TestSeveralFilesFirstLaunch walks what every family app has, on many
// sizes, with the two panels (tdptest.Check).
func TestSeveralFilesFirstLaunch(t *testing.T) {
	var home string
	tdptest.Check(t, tdptest.App{
		Name: "catu",
		Seed: func(h string) {
			home = h
			os.WriteFile(filepath.Join(h, "a.md"), []byte("# A\n"), 0o644)
			os.WriteFile(filepath.Join(h, "b.go"), []byte("package b\n"), 0o644)
		},
		New: func() *terminu.App {
			a, err := OpenAll([]string{filepath.Join(home, "a.md"), filepath.Join(home, "b.go")})
			if err != nil {
				panic(err)
			}
			return a
		},
	})
}
