// Package ui is catu's screen: one panel showing a file (and a FILES panel
// beside it with two or more), the statusbar over it, the footer under it;
// and what catu prints instead when it opens no screen.
package ui

import (
	_ "embed"
	"errors"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	terminu "github.com/vulcanshen/terminu-go"

	"github.com/vulcanshen/catu/internal/text"
	"github.com/vulcanshen/catu/internal/version"
)

//go:embed icon.svg
var icon []byte // the app's mark, a copy of docs/icon.svg, drawn by its splash (tdp S5, E6)

// intro is the splash's line under the version (tdp S5).
const intro = "A file viewer for the terminal"

// viewer is catu: what it shows and the panel it shows it in.
type viewer struct {
	app   *terminu.App
	panel *terminu.Panel
	body  *reader
	cfg   config
	cur   *shown
	// warned are the diagrams' errors logged for what is shown: a diagram
	// is drawn again at every width, its error logged once.
	warned map[string]bool
	// files, with two or more paths, lists them in a panel of its own
	// beside the viewer (files.go); nil with one.
	files *fileList
}

// Open makes catu showing p, a file or a directory; the error is reading p
// (it is not there, it may not be read), before anything is drawn.
func Open(p string) (*terminu.App, error) {
	a, err := OpenAll([]string{p})
	var oe *OpenError
	if errors.As(err, &oe) {
		return nil, oe.Err
	}
	return a, err
}

// OpenError is a path OpenAll could not read, as it was given.
type OpenError struct {
	Path string
	Err  error
}

func (e *OpenError) Error() string { return e.Path + ": " + e.Err.Error() }
func (e *OpenError) Unwrap() error { return e.Err }

// OpenAll makes catu showing paths: one in its panel; two or more listed in a
// FILES panel beside it, which shows the one under the list's cursor
// (task 16). The error is the first path it cannot read, before anything is
// drawn.
func OpenAll(paths []string) (*terminu.App, error) {
	cfg, cfgPath, cfgErr := readConfig()
	v := &viewer{app: terminu.New("catu"), body: &reader{}, cfg: cfg}
	var all []*shown
	var given []string
	seen := map[string]bool{}
	for _, p := range paths {
		s, err := load(p, cfg, v.diagramError)
		if err != nil {
			return nil, &OpenError{p, err}
		}
		if !seen[s.path] { // named twice: once
			seen[s.path] = true
			all, given = append(all, s), append(given, p)
		}
	}
	paths = given
	a := v.app
	// One panel: 80×24 terminals and a half of a tmux window are enough (tdp L1).
	a.MinWidth, a.MinHeight = 40, 12
	a.InputTabWidth, a.HideSecretsInPreview = cfg.TabWidth, cfg.HideSecrets
	a.Status = v.status
	a.OnMsg = v.onMsg
	v.panel = &terminu.Panel{Body: v.body, Actions: v.actions(), OnTab: v.onTab}
	screen := &terminu.Screen{Name: "View", Panels: []*terminu.Panel{v.panel}}
	if len(all) > 1 {
		v.listFiles(screen, paths, all)
	}
	a.AddScreen(screen)
	colour := v.show(all[0])
	var err error
	if a.Splash, err = terminu.NewSplash(icon, "catu", version.Display(), intro); err != nil {
		return nil, err
	}
	a.Start = func() tea.Cmd {
		for _, s := range all {
			v.opened(s)
		}
		if cfgErr != nil {
			// Settings that cannot be read: moved aside, or catu leaves (tdp E3).
			return tea.Batch(colour, a.MoveAside(cfgPath, cfgErr, func() tea.Cmd {
				v.cfg = defaults()
				a.InputTabWidth, a.HideSecretsInPreview = v.cfg.TabWidth, v.cfg.HideSecrets
				return v.reload()
			}))
		}
		return colour
	}
	return a, nil
}

// reload reads again what is shown, and every file listed, with the settings
// as they are now.
func (v *viewer) reload() tea.Cmd {
	if v.files != nil {
		v.files.reload()
	}
	again, err := load(v.cur.path, v.cfg, v.diagramError)
	if err != nil {
		return nil
	}
	if v.files != nil {
		v.files.loaded[again.path] = again
	}
	return v.show(again)
}

// opened logs that s was opened, and why its content could not be made out
// when it could not (tdp note.7, F5; the panel says it too, tdp panel.15).
func (v *viewer) opened(s *shown) {
	v.app.Log(terminu.Info, "Opened "+s.name())
	if s.problem != "" {
		v.app.Log(terminu.Error, s.problem)
	}
}

// show puts s in the panel: its views' names on the label and the tabs, the
// first view in the body. It returns the colouring of the views to be
// coloured in the background; the panel is loading until they are.
func (v *viewer) show(s *shown) tea.Cmd {
	v.cur, v.warned = s, map[string]bool{}
	v.panel.LeaveMode()
	v.panel.Label = s.views[0].name
	v.panel.Tabs = nil
	for _, vw := range s.views[1:] {
		v.panel.Tabs = append(v.panel.Tabs, vw.name)
	}
	v.panel.Tab = 0
	v.body.seenNumW = 0 // a new file: its number column starts again
	v.showView(0)
	var cmds []tea.Cmd
	for i, vw := range s.views {
		if colour := vw.colour; colour != nil {
			cmds = append(cmds, func() tea.Msg { return coloured{s: s, view: i, lines: colour()} })
		}
	}
	v.panel.Loading = len(cmds) > 0
	return tea.Batch(cmds...)
}

// coloured is a view's lines in colour, made in the background.
type coloured struct {
	s     *shown
	view  int
	lines []text.Line
}

// onMsg takes a view's colours when they are made: in their place, where
// they are, nothing moving (tdp K3: the background moves nothing).
func (v *viewer) onMsg(msg tea.Msg) tea.Cmd {
	c, ok := msg.(coloured)
	if !ok {
		return nil
	}
	vw := &c.s.views[c.view]
	vw.lines, vw.colour = c.lines, nil
	if c.s != v.cur {
		return nil // kept for when it is shown again (a file listed, task 16)
	}
	if v.panel.Tab == c.view && vw.layout == nil {
		v.body.recolour(c.lines)
	}
	v.panel.Loading = false
	for _, vw := range v.cur.views {
		v.panel.Loading = v.panel.Loading || vw.colour != nil
	}
	return nil
}

// showView puts view i of what is shown in the body.
func (v *viewer) showView(i int) {
	vw := v.cur.views[i]
	// Why the tab could not be made, drawn by tgo in the middle when there
	// is nothing to show (tdp panel.15); a layout's own is the body's
	// Failure, known once it is laid out at the panel's width.
	v.panel.Fail = vw.fail
	if vw.layout != nil {
		v.body.setLayout(vw.layout, vw.failHint)
	} else {
		v.body.set(vw.lines, vw.numbered)
	}
	v.body.fact, v.body.hint = vw.fact, vw.hint // when there is nothing to show
	// The location row (tdp panel.10).
	v.panel.Location = v.cur.name()
	v.panel.OnEsc = nil // no hits to clear
}

// diagramError logs a diagram that cannot be drawn, once for what is shown
// (tdp note.7: the app's own doing).
func (v *viewer) diagramError(err error) {
	if v.warned[err.Error()] {
		return
	}
	v.warned[err.Error()] = true
	v.app.Log(terminu.Warning, "Can't draw a mermaid diagram in "+v.cur.base()+": "+err.Error())
}

// onTab shows the tab h|l went to (tdp panel.4, K12).
func (v *viewer) onTab(tab int) tea.Cmd {
	v.showView(tab)
	return nil
}

// clearHits is Esc on the panel: the hits go, one level up (tdp panel.14,
// K4).
func (v *viewer) clearHits() tea.Cmd {
	v.body.hits = nil
	v.panel.OnEsc = nil
	return nil
}

// tilde is p with the home directory written ~.
func tilde(p string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return p
	}
	if p == home {
		return "~"
	}
	if rest, ok := strings.CutPrefix(p, home+string(filepath.Separator)); ok {
		return "~" + string(filepath.Separator) + rest
	}
	return p
}
