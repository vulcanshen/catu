package ui

import (
	"errors"
	"io/fs"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vulcanshen/terminu-go"
)

// Watching (task 17): the file shown is looked at every watchEvery, and read
// again when its modification time or its size changed, where the view was;
// at the end, the view follows what was added, as tail -f. What came down a
// pipe is not a file to watch.
const watchEvery = time.Second

// watchTick is the time to look at the file shown again.
type watchTick struct{}

// watch looks at the file shown again in watchEvery, on the App's clock.
func (v *viewer) watch() tea.Cmd {
	return v.app.Tick(watchEvery, func(time.Time) tea.Msg { return watchTick{} })
}

// check looks at the file shown: read again when it changed, the Visual
// mode too, which only selects and copies (the user, 2026-11-16). A file
// gone, or that cannot be read now, stays shown as it was, said once in a
// Warning (tdp toast.2, note.7); when it is back, it is read again.
func (v *viewer) check() tea.Cmd {
	s := v.cur
	if s == nil || s.stdin {
		return nil
	}
	info, err := os.Stat(s.path)
	if err != nil {
		return v.lost(s, s.base()+" is no longer there: shown as it was")
	}
	if v.gone != s.path && info.ModTime().Equal(s.info.ModTime()) && info.Size() == s.info.Size() {
		return nil
	}
	again, err := load(s.path, v.cfg, v.diagramError)
	if err != nil {
		var pe *fs.PathError
		if errors.As(err, &pe) {
			err = pe.Err // the path is said already
		}
		return v.lost(s, "Can't read "+s.base()+" now: "+err.Error()+"; shown as it was")
	}
	v.gone = ""
	return v.refresh(again)
}

// lost says once, in a Warning, that s cannot be read now.
func (v *viewer) lost(s *shown, why string) tea.Cmd {
	if v.gone == s.path {
		return nil
	}
	v.gone = s.path
	return v.app.Toast(terminu.Warning, why)
}

// refresh shows again, read again, what is shown: on the tab it was on,
// where the view was, the search found again (tdp note.3), in the Visual
// mode it was in, the cursor and where the selection started kept where
// the text still reaches; logged as the app's own doing (tdp note.7). In
// the Visual mode an Info toast says the text changed (the user,
// 2026-11-16): what is selected may be other text now.
func (v *viewer) refresh(again *shown) tea.Cmd {
	b := v.body
	tab, top, sub, end := v.panel.Tab, b.top, b.sub, b.atEnd()
	q, cur := b.query, b.current
	vis := b.visualState()
	warned := v.warned // a diagram's error is logged once for the file
	cmd := v.display(again)
	v.warned = warned
	if tab > 0 && tab < len(again.views) {
		v.panel.Tab = tab
		v.showView(tab)
	}
	b.keep(top, sub, end, q, cur)
	b.keepVisual(vis)
	if q != "" {
		v.panel.OnEsc = v.clearHits
	}
	if v.files != nil {
		v.files.loaded[again.path] = again
	}
	if vis != nil {
		return tea.Batch(cmd, v.app.Toast(terminu.Info, again.base()+" changed: shown as it is now"))
	}
	v.app.Log(terminu.Info, "Reloaded "+again.name())
	return cmd
}
