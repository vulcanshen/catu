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
// in it, as viewu <file> does.
var launchHome string

// TestFirstLaunch launches viewu as for the first time on many terminal
// sizes and icon widths, and walks what every family app has: the Space
// menu, the global operation popup, the App Log and the key reference; then
// a terminal under the least size and back (tdptest.Check).
func TestFirstLaunch(t *testing.T) {
	tdptest.Check(t, tdptest.App{
		Name: "viewu",
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
	want(t, d, "viewu", intro, "Press any key to close")
	d.Press("q")
	if d.Quit() {
		t.Error("q on the splash left")
	}
	want(t, d, "  1 hello")
}
