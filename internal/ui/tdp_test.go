package ui

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	terminu "github.com/vulcanshen/terminu-go"
	"github.com/vulcanshen/terminu-go/tdptest"
)

// launchHome is the sandbox the last launch's Seed made: New opens a file
// in it, as catu <file> does.
var launchHome string

// TestFirstLaunch launches catu as for the first time on many terminal
// sizes and icon widths, and walks what every family app has: the Space
// menu, the global operation popup, the App Log and the key reference; then
// a terminal under the least size and back (tdptest.Check).
func TestFirstLaunch(t *testing.T) {
	tdptest.Check(t, tdptest.App{
		Name: "catu",
		Seed: func(home string) {
			launchHome = home
			os.WriteFile(filepath.Join(home, "notes.md"), []byte("# Notes\n\nSome *text*.\n\n```go\nfunc main() {}\n```\n"), 0o644)
		},
		New: func() *terminu.App {
			a, err := Open(filepath.Join(launchHome, "notes.md"))
			if err != nil {
				panic(err)
			}
			return a
		},
	})
}

// TestOwnPopupsEverySize opens each popup catu opens itself on each icon
// width, then makes the terminal narrower than it, lower, both (at the least
// size or over, so the popup is drawn over the screen), under the least size
// and down to 1×1, and back: every frame is as wide as the
// terminal (tdptest checks each, tdp L4) and nothing panics (tdp E5). The
// Space menu, the global operation popup, the App Log and the key reference
// are TestFirstLaunch's (tdptest.Check), a popup's own frame the lib's (tdp
// E8).
func TestOwnPopupsEverySize(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads everything: no file the picker cannot open")
	}
	dir := files(t, map[string]string{"a.txt": "first line\n", "b.txt": "second\n"})
	os.Chmod(filepath.Join(dir, "b.txt"), 0)
	cfg := filepath.Join(os.Getenv("CATU__CONFIG"), "config.yaml")
	popups := []struct {
		name       string
		badConfig  bool
		keys       []string
		text, want string
	}{
		{name: "search", keys: []string{"/"}, want: "SEARCH"},
		{name: "toast", keys: []string{"/"}, text: "zzz", want: "No line has zzz"},
		{name: "file-picker", keys: []string{"o"}, want: "OPEN · "},
		{name: "error popup", keys: []string{"o", "j", "Enter"}, want: "permission denied"},
		{name: "splash", keys: []string{"V"}, want: "Press any key to close"},
		{name: "move aside", badConfig: true, want: "MOVE ASIDE"},
	}
	tdptest.EachIconWidth(t, func(t *testing.T) {
		for _, p := range popups {
			os.Remove(cfg)
			if p.badConfig {
				os.WriteFile(cfg, []byte("input_tab_width: [\n"), 0o644)
			}
			d := start(t, filepath.Join(dir, "a.txt"), 80, 24)
			d.Press(p.keys...)
			if p.text != "" {
				d.Type(p.text)
				d.Press("Enter")
			}
			if !d.Has(p.want) {
				t.Errorf("%s: no %q", p.name, p.want)
			}
			// Opened at 80×24, a popup keeps its size (tdp F7): at 50×24 the
			// search input is wider than the screen, at 80×12 the
			// file-picker higher, at 50×12 both, all at the least size or
			// over; then under it.
			for _, sz := range [][2]int{{50, 24}, {80, 12}, {50, 12}, {40, 12}, {39, 12}, {80, 5}, {1, 1}, {80, 24}} {
				d.Resize(sz[0], sz[1])
			}
			if !d.Has(p.want) {
				t.Errorf("%s: gone after the resizes", p.name)
			}
		}
	})
}

// TestSplashIsTheIcon keeps the splash's copy of the mark the same as
// docs/icon.svg, its master copy (tdp E6, S5).
func TestSplashIsTheIcon(t *testing.T) {
	master, err := os.ReadFile(filepath.Join("..", "..", "docs", "icon.svg"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(master, icon) {
		t.Error("internal/ui/icon.svg is not docs/icon.svg: copy it again")
	}
}

// TestSplash opens the splash with V; any key closes it, doing nothing else
// (tdp S1, S3).
func TestSplash(t *testing.T) {
	dir := files(t, map[string]string{"a.txt": "hello\n"})
	d := start(t, filepath.Join(dir, "a.txt"), 80, 40)
	d.Press("V")
	d.Settle()
	want(t, d, "catu", intro, "Press any key to close")
	d.Press("q")
	if d.Quit() {
		t.Error("q on the splash left")
	}
	want(t, d, "  1 hello")
}
