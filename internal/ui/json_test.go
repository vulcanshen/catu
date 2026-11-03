package ui

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	terminu "github.com/vulcanshen/terminu-go"
	"github.com/vulcanshen/terminu-go/tdptest"
)

// --- task 12: JSON formatted ---------------------------------------------

// TestJSONNumbersOnlyWiden: the number column is as wide as the widest tab
// seen, and stays so back on the narrower one (tdp note.2).
func TestJSONNumbersOnlyWiden(t *testing.T) {
	var b strings.Builder
	b.WriteString("{")
	for i := range 1000 {
		if i > 0 {
			b.WriteString(",")
		}
		fmt.Fprintf(&b, `"k%d":%d`, i, i)
	}
	b.WriteString("}\n")
	dir := files(t, map[string]string{"a.json": b.String()})
	d := start(t, filepath.Join(dir, "a.json"), 80, 24)
	want(t, d, `┃   1 {"k0":0`) // one line: three places
	d.Press("l")
	want(t, d, "┃    1 {") // 1002 lines: four
	d.Press("h")
	want(t, d, `┃    1 {"k0":0`) // back: still four
}

// TestJSONEmpty: an empty file is empty in both tabs, FORMATTED pointing
// back to the file as it is (tdp screen.8).
func TestJSONEmpty(t *testing.T) {
	dir := files(t, map[string]string{"a.json": ""})
	d := start(t, filepath.Join(dir, "a.json"), 80, 24)
	want(t, d, "Empty file", "Press [o] to open another file")
	d.Press("l")
	want(t, d, "Empty file", "Press [h] to see the file as it is")
}

// TestJSONBigColouredLater: over 64 KB the formatted text is shown in Text
// first and coloured in the background, as the file is (tdp panel.15).
func TestJSONBigColouredLater(t *testing.T) {
	old := colourNow
	colourNow = 10
	defer func() { colourNow = old }()
	dir := files(t, map[string]string{"a.json": `{"alpha":[1,2,3],"beta":"x"}` + "\n"})
	s, err := load(filepath.Join(dir, "a.json"), defaults(), nil)
	if err != nil {
		t.Fatal(err)
	}
	f := s.views[1]
	if f.name != "FORMATTED" || f.colour == nil || f.lines[1].Chars[2].St.Fg != terminu.Text {
		t.Fatal("the formatted text is not in Text first, to be coloured later")
	}
	if got := f.colour()[1].Chars[2].St.Fg; got == terminu.Text {
		t.Error("the formatted text is never coloured")
	}
}

func TestJSONAsItIsThenFormatted(t *testing.T) {
	dir := files(t, map[string]string{"a.json": `{"zeta":1,"alpha":[true,null],"nested":{"k":"v"}}` + "\n"})
	tdptest.EachIconWidth(t, func(t *testing.T) {
		for _, sz := range [][2]int{{40, 12}, {80, 24}} {
			d := start(t, filepath.Join(dir, "a.json"), sz[0], sz[1])
			want(t, d, "TEXT", "FORMATTED", "h|l:tab", `  1 {"zeta":1,"alpha"`)
			d.Press("l")
			want(t, d, "  1 {", `  2   "zeta": 1,`, `  3   "alpha": [`, `  4     true,`)
			if strings.Index(strings.Join(d.Screen(), "\n"), "zeta") > strings.Index(strings.Join(d.Screen(), "\n"), "alpha") {
				t.Error("the keys are not in the file's order")
			}
			d.Press("h")
			want(t, d, `  1 {"zeta":1`)
		}
	})
}

func TestJSONNotValid(t *testing.T) {
	dir := files(t, map[string]string{"a.json": "{\n  \"a\": 1,\n}\n"})
	d := start(t, filepath.Join(dir, "a.json"), 80, 24)
	want(t, d, `  3 }`)
	d.Press("l")
	want(t, d, "Not valid JSON: line 3: invalid character '}'", "Press [h] to see the file as it is")
	red := strings.TrimSuffix(terminu.Fg(terminu.Red).Render("Not valid JSON"), "\x1b[0m")
	if !strings.Contains(strings.Join(d.Styled(), ""), red) {
		t.Error("the reason is not in Red")
	}
}

func TestJSONFormattedSearch(t *testing.T) {
	dir := files(t, map[string]string{"a.json": `{"a":{"needle":1}}`})
	d := start(t, filepath.Join(dir, "a.json"), 60, 20)
	d.Press("l", "/")
	d.Type("needle")
	d.Press("Enter")
	want(t, d, `  3     "needle": 1`)
	wantNot(t, d, "No line has")
}
