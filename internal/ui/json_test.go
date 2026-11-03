package ui

import (
	"path/filepath"
	"strings"
	"testing"

	terminu "github.com/vulcanshen/terminu-go"
	"github.com/vulcanshen/terminu-go/tdptest"
)

// --- task 12: JSON formatted ---------------------------------------------

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
