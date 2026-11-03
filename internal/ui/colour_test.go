package ui

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	terminu "github.com/vulcanshen/terminu-go"
	"github.com/vulcanshen/terminu-go/tdptest"
)

// TestBigTextColouredLater: a text too big to colour at once is shown in
// Text with the panel loading, and in its colours once they are made, where
// it was (task 3).
func TestBigTextColouredLater(t *testing.T) {
	old := colourNow
	colourNow = 10
	defer func() { colourNow = old }()
	dir := files(t, map[string]string{"main.go": "package main\n\nfunc main() {}\n" + lines(200, "// line %d")})
	s, err := load(filepath.Join(dir, "main.go"), defaults(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if s.views[0].colour == nil || s.views[0].lines[0].Chars[0].St.Fg != terminu.Text {
		t.Fatal("a big text is not shown in Text first, to be coloured later")
	}
	a, err := Open(filepath.Join(dir, "main.go"))
	if err != nil {
		t.Fatal(err)
	}
	keyword := "38;2;148;226;213mpackage" // catppuccin-mocha's Teal on package
	d := tdptest.Start(t, a, 80, 24)
	want(t, d, "  1 package main")
	d.WaitFor(func() bool { return strings.Contains(strings.Join(d.Styled(), ""), keyword) }, 10*time.Second)
	if !strings.Contains(strings.Join(d.Styled(), ""), keyword) {
		t.Fatal("never coloured")
	}
	if strings.Contains(d.Screen()[1], terminu.LoadingFrame()) {
		t.Error("still loading once coloured")
	}
	want(t, d, "  1 package main", "1-")
}
