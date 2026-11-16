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

// watchTick is the time to look at the file shown again: at, on the App's
// clock.
type watchTick struct{ at time.Time }

// watch looks at the file shown again in watchEvery, on the App's clock.
func (v *viewer) watch() tea.Cmd {
	return v.app.Tick(watchEvery, func(at time.Time) tea.Msg { return watchTick{at} })
}

// syncGlyph is nf-md-sync: the file shown is watched.
const syncGlyph = "\U000F04E6"

// syncStatus is the viewer panel's status on its frame (tdp panel.1): the
// file shown is watched, and when it was last read, at the bottom right;
// Peach while it cannot be read, still the time of the last read; none for
// what came down a pipe, which is not watched.
func (v *viewer) syncStatus() {
	s := v.cur
	if s == nil || s.stdin {
		v.panel.FrameStatus = terminu.FrameStatus{}
		return
	}
	kind := terminu.StatusFine
	if v.gone == s.path {
		kind = terminu.StatusWarning
	}
	v.panel.FrameStatus = terminu.FrameStatus{Glyph: syncGlyph, Text: s.read.Format("15:04:05"),
		Corner: terminu.BottomRight, Kind: kind}
}

// check looks at the file shown, at the time at: read again when it
// changed, the Visual mode too, which only selects and copies (the user,
// 2026-11-16). A file gone, or that cannot be read now, stays shown as it
// was, said once in a Warning (tdp toast.2, note.7); when it is back, it is
// read again.
func (v *viewer) check(at time.Time) tea.Cmd {
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
	again.read = at
	return v.refresh(again)
}

// lost says once, in a Warning, that s cannot be read now.
func (v *viewer) lost(s *shown, why string) tea.Cmd {
	if v.gone == s.path {
		return nil
	}
	v.gone = s.path
	v.syncStatus()
	return v.app.Toast(terminu.Warning, why)
}

// refresh shows again, read again, what is shown: on the tab it was on,
// where the view was, the search found again (tdp note.3), in the Visual
// mode it was in, the cursor kept where the text still reaches; logged as
// the app's own doing (tdp note.7). A selection is let go, back to the
// Visual mode's own layer (the user, 2026-11-16): the text under it may be
// other text now.
func (v *viewer) refresh(again *shown) tea.Cmd {
	b := v.body
	tab, top, sub, end := v.panel.Tab, b.top, b.sub, b.atEnd()
	q, cur := b.query, b.current
	vis := b.visualState()
	if vis != nil && vis.selecting {
		// A layer less, as Esc leaves it: what the user has open over the
		// panel stays (tdp K3), its rows doing what the keys do now.
		vis.selecting = false
		v.panel.EnterMode(v.inVisual)
	}
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
	v.app.Log(terminu.Info, "Reloaded "+again.name())
	return cmd
}
