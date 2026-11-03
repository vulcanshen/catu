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
}

// Open makes viewu showing p, a file or a directory; the error is reading p
// (it is not there, it may not be read), before anything is drawn.
func Open(p string) (*terminu.App, error) {
	cfg, cfgPath, cfgErr := readConfig()
	s, err := load(p, cfg)
	if err != nil {
		return nil, err
	}
	v := &viewer{app: terminu.New("viewu"), body: &reader{}, cfg: cfg}
	a := v.app
	// One panel: 80×24 terminals and a half of a tmux window are enough (tdp L1).
	a.MinWidth, a.MinHeight = 40, 12
	a.InputTabWidth, a.HideSecretsInPreview = cfg.TabWidth, cfg.HideSecrets
	a.Status = v.status
	v.panel = &terminu.Panel{Body: v.body, Actions: v.actions(), OnTab: v.onTab}
	a.AddScreen(&terminu.Screen{Name: "View", Panels: []*terminu.Panel{v.panel}})
	v.show(s)
	if a.Splash, err = terminu.NewSplash(icon, "viewu", version.Display(), intro); err != nil {
		return nil, err
	}
	a.Start = func() tea.Cmd {
		a.Log(terminu.Info, "Opened "+tilde(s.path))
		if cfgErr != nil {
			// Settings that cannot be read: moved aside, or viewu leaves (tdp E3).
			return a.MoveAside(cfgPath, cfgErr, func() tea.Cmd {
				v.cfg = defaults()
				a.InputTabWidth, a.HideSecretsInPreview = v.cfg.TabWidth, v.cfg.HideSecrets
				if again, err := load(v.cur.path, v.cfg); err == nil {
					v.show(again)
				}
				return nil
			})
		}
		return nil
	}
	return a, nil
}

// show puts s in the panel: its views' names on the label and the tabs, the
// first view in the body.
func (v *viewer) show(s *shown) {
	v.cur = s
	v.panel.LeaveMode()
	v.panel.Label = s.views[0].name
	v.panel.Tabs = nil
	for _, vw := range s.views[1:] {
		v.panel.Tabs = append(v.panel.Tabs, vw.name)
	}
	v.panel.Tab = 0
	v.showView(0)
}

// showView puts view i of what is shown in the body.
func (v *viewer) showView(i int) {
	vw := v.cur.views[i]
	if vw.layout != nil {
		v.body.setLayout(vw.layout)
	} else {
		v.body.set(vw.lines, vw.numbered)
		if len(vw.lines) == 0 {
			v.body.fact, v.body.hint = vw.fact, vw.hint
		}
	}
	v.body.where = tilde(v.cur.path)
	v.panel.OnEsc = nil // no hits to clear
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
