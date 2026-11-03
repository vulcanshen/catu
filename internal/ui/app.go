// Package ui is viewu's screen: one panel showing a file, the statusbar over
// it, the footer under it.
package ui

import (
	_ "embed"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	terminu "github.com/vulcanshen/terminu-go"

	"github.com/vulcanshen/viewu/internal/text"
	"github.com/vulcanshen/viewu/internal/version"
)

//go:embed icon.svg
var icon []byte // the app's mark, a copy of docs/icon.svg, drawn by its splash (tdp S5, E6)

// intro is the splash's line under the version (tdp S5).
const intro = "A file viewer for the terminal"

// viewer is viewu: what it shows and the panel it shows it in.
type viewer struct {
	app   *terminu.App
	panel *terminu.Panel
	body  *reader
	cfg   config
	cur   *shown
	// warned are the diagrams' errors logged for what is shown: a diagram
	// is drawn again at every width, its error logged once.
	warned map[string]bool
}

// Open makes viewu showing p, a file or a directory; the error is reading p
// (it is not there, it may not be read), before anything is drawn.
func Open(p string) (*terminu.App, error) {
	cfg, cfgPath, cfgErr := readConfig()
	v := &viewer{app: terminu.New("viewu"), body: &reader{}, cfg: cfg}
	s, err := load(p, cfg, v.diagramError)
	if err != nil {
		return nil, err
	}
	a := v.app
	// One panel: 80×24 terminals and a half of a tmux window are enough (tdp L1).
	a.MinWidth, a.MinHeight = 40, 12
	a.InputTabWidth, a.HideSecretsInPreview = cfg.TabWidth, cfg.HideSecrets
	a.Status = v.status
	a.OnMsg = v.onMsg
	v.panel = &terminu.Panel{Body: v.body, Actions: v.actions(), OnTab: v.onTab}
	a.AddScreen(&terminu.Screen{Name: "View", Panels: []*terminu.Panel{v.panel}})
	colour := v.show(s)
	if a.Splash, err = terminu.NewSplash(icon, "viewu", version.Display(), intro); err != nil {
		return nil, err
	}
	a.Start = func() tea.Cmd {
		v.opened(s)
		if cfgErr != nil {
			// Settings that cannot be read: moved aside, or viewu leaves (tdp E3).
			return tea.Batch(colour, a.MoveAside(cfgPath, cfgErr, func() tea.Cmd {
				v.cfg = defaults()
				a.InputTabWidth, a.HideSecretsInPreview = v.cfg.TabWidth, v.cfg.HideSecrets
				if again, err := load(v.cur.path, v.cfg, v.diagramError); err == nil {
					return v.show(again)
				}
				return nil
			}))
		}
		return colour
	}
	return a, nil
}

// opened logs that s was opened, and why its content could not be made out
// when it could not (tdp note.7, F5; the panel says it too, tdp panel.15).
func (v *viewer) opened(s *shown) {
	v.app.Log(terminu.Info, "Opened "+tilde(s.path))
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
	if !ok || c.s != v.cur {
		return nil // what was shown when it started is not any more
	}
	vw := &v.cur.views[c.view]
	vw.lines, vw.colour = c.lines, nil
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
	if vw.layout != nil {
		v.body.setLayout(vw.layout, vw.failHint)
	} else {
		v.body.set(vw.lines, vw.numbered)
	}
	v.body.fact, v.body.hint, v.body.red = vw.fact, vw.hint, vw.red // when there is nothing to show
	v.body.where = tilde(v.cur.path)
	v.panel.OnEsc = nil // no hits to clear
}

// diagramError logs a diagram that cannot be drawn, once for what is shown
// (tdp note.7: the app's own doing).
func (v *viewer) diagramError(err error) {
	if v.warned[err.Error()] {
		return
	}
	v.warned[err.Error()] = true
	v.app.Log(terminu.Warning, "Can't draw a mermaid diagram in "+filepath.Base(v.cur.path)+": "+err.Error())
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
