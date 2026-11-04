package ui

import (
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	terminu "github.com/vulcanshen/terminu-go"
)

// fileList is the FILES panel catu has with two or more paths (task 16): the
// paths in the order given, the viewer beside it showing the one under the
// cursor as it moves (tdp panel.13); Enter on one sends the focus there.
type fileList struct {
	v      *viewer
	list   *terminu.List
	panel  *terminu.Panel
	loaded map[string]*shown // by absolute path, a row's ID
}

// listFiles puts the files in a FILES panel [1] beside the viewer, now [2].
func (v *viewer) listFiles(sc *terminu.Screen, paths []string, all []*shown) {
	f := &fileList{v: v, loaded: map[string]*shown{}}
	f.list = &terminu.List{Columns: []terminu.Column{{Shrink: true, Path: true}}}
	var rows []terminu.Row
	for i, s := range all {
		if _, ok := f.loaded[s.path]; ok {
			continue // named twice: listed once
		}
		f.loaded[s.path] = s
		rows = append(rows, listRow(s, paths[i]))
	}
	f.list.SetRows(rows)
	f.list.OnMove = func(r terminu.Row) tea.Cmd {
		if s := f.loaded[r.ID]; s != nil && s != v.cur {
			return v.show(s)
		}
		return nil
	}
	f.panel = &terminu.Panel{
		Label: "FILES", Body: f.list, Details: v.panel,
		Actions: []terminu.Action{{
			Key: "Enter", Label: "View", Desc: "the file, in [2]", Scope: terminu.ItemScope, Hint: "view",
			ToDetails: func() bool { return true }, Do: func() tea.Cmd { return nil },
		}},
	}
	sc.Panels = []*terminu.Panel{f.panel, v.panel}
	// Side by side from 60 columns (tdp L1); narrower, the one with the focus
	// (tdp screen.5).
	sc.NarrowBelow = 60
	sc.Layout = func(w, h int) []terminu.Rect {
		lw := f.width(w)
		return []terminu.Rect{{X: 0, Y: 0, W: lw, H: h}, {X: lw, Y: 0, W: w - lw, H: h}}
	}
	v.files = f
}

// listRow is s as the list names it: its path as it was given, the home as ~,
// or standard input.
func listRow(s *shown, given string) terminu.Row {
	name := given
	switch {
	case s.stdin:
		name = stdinName
	case filepath.IsAbs(given):
		name = tilde(given)
	}
	return terminu.Row{ID: s.path, Cells: []string{name}}
}

// width is the list's: its longest name and the frame round it, at most a
// third of w.
func (f *fileList) width(w int) int {
	longest := 0
	for _, r := range f.list.Rows() {
		longest = max(longest, terminu.Width(r.Cells[0]))
	}
	return min(longest+4, w/3)
}

// add lists s, the file o chose, last unless it is listed already, and puts
// the cursor on it, which shows it.
func (f *fileList) add(s *shown) tea.Cmd {
	if _, ok := f.loaded[s.path]; !ok {
		f.list.SetRows(append(f.list.Rows(), listRow(s, s.path)))
	}
	f.loaded[s.path] = s
	f.list.Select(s.path)
	return f.v.show(s)
}

// reload reads every file listed again, with the settings as they are now.
func (f *fileList) reload() {
	for p := range f.loaded {
		if s, err := load(p, f.v.cfg, f.v.diagramError); err == nil {
			f.loaded[p] = s
		}
	}
}
