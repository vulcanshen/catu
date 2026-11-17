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
// Red while it cannot be read (the user: an error), still the time of the
// last read; none for what came down a pipe, which is not watched.
func (v *viewer) syncStatus() {
	s := v.cur
	if s == nil || s.stdin {
		v.panel.FrameStatus = terminu.FrameStatus{}
		return
	}
	kind := terminu.StatusFine
	if v.gone == s.path {
		kind = terminu.StatusError
	}
	v.panel.FrameStatus = terminu.FrameStatus{Glyph: syncGlyph, Text: s.read.Format("15:04:05"),
		Corner: terminu.BottomRight, Kind: kind}
}

// check looks at the file shown, at the time at: read again when it
// changed, the Visual mode too, which only selects and copies (the user,
// 2026-11-16). A file gone, or that cannot be read now, stays shown as it
// was, said once in an Error (the user, tdp toast.2: a failure is an
// Error; note.7); when it is back, it is read again.
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

// lost says once, in an Error, that s cannot be read now.
func (v *viewer) lost(s *shown, why string) tea.Cmd {
	if v.gone == s.path {
		return nil
	}
	v.gone = s.path
	v.syncStatus()
	return v.app.Toast(terminu.Error, why)
}

// refresh shows again, read again, what is shown: on the tab it was on,
// where the view was, the search found again (tdp note.3); logged as the
// app's own doing (tdp note.7). In the Visual mode it stays, the cursor and
// where the selection started at their line and column, the text there
// being what it is now, and the view does not follow what is added at the
// end (the user, 2026-11-16).
func (v *viewer) refresh(again *shown) tea.Cmd {
	b := v.body
	tab, top, sub := v.panel.Tab, b.top, b.sub
	q, cur := b.query, b.current
	vis := b.visualState()
	end := vis == nil && b.atEnd()
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
