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

// lost says once, in an Error, that s cannot be read now; the Visual mode
// is left, that Error the only toast (tdp note.9).
func (v *viewer) lost(s *shown, why string) tea.Cmd {
	if v.gone == s.path {
		return nil
	}
	v.gone = s.path
	v.syncStatus()
	if v.body.visual {
		v.panel.LeaveMode()
		v.body.leave()
	}
	return v.app.Toast(terminu.Error, why)
}

// refresh shows again, read again, what is shown, as if it were opened
// again (tdp note.9): on the tab it was on, where the view was, or at the
// end when it was there, but not in the Visual mode; the number column only
// widening; a search found again; the Visual mode kept where its positions
// still are. Logged as the app's own doing (tdp note.7).
func (v *viewer) refresh(again *shown) tea.Cmd {
	b := v.body
	tab, top, sub := v.panel.Tab, b.top, b.sub
	q := b.query
	at, _ := b.currentAt()
	vis := b.visualState()
	end := vis == nil && b.atEnd()
	warned, numW := v.warned, b.seenNumW // a diagram's error is logged once for the file
	cmd := v.display(again)
	v.warned, b.seenNumW = warned, numW
	if tab > 0 && tab < len(again.views) {
		v.panel.Tab = tab
		v.showView(tab)
	}
	b.layOut()
	b.top, b.sub = top, sub // the next Draw keeps it in the text: at its end when shorter
	if end {
		b.top, b.sub = len(b.lines), 0
	}
	if v.files != nil {
		v.files.loaded[again.path] = again
	}
	v.app.Log(terminu.Info, "Reloaded "+again.name())
	cmds := []tea.Cmd{cmd}
	if q != "" {
		cmds = append(cmds, v.findAgain(q, at))
	}
	if vis != nil {
		cmds = append(cmds, v.visualAgain(*vis))
	}
	return tea.Batch(cmds...)
}

// findAgain finds q again in what was read (tdp note.9): the first hit at
// or after where the current one was is current, the view not moved; none
// left, it is a search that found nothing, said as / says it.
func (v *viewer) findAgain(q string, at pos) tea.Cmd {
	b := v.body
	b.collect(q, 0)
	if len(b.hits) == 0 {
		v.panel.OnEsc = nil
		return v.app.Toast(terminu.Info, "No line has "+q)
	}
	b.currentFrom(at)
	v.panel.OnEsc = v.clearHits
	return nil
}

// visualAgain puts the Visual mode back on what was read (tdp note.9): its
// positions, not its text. Both still there, nothing changes. Where the
// selection started gone, the selection goes, back to Visual as one Esc;
// the cursor's place gone, the mode goes, as Esc to the end; either said
// in a Warning.
func (v *viewer) visualAgain(s visualState) tea.Cmd {
	b := v.body
	if !b.there(s.line, s.col) {
		v.panel.LeaveMode()
		return v.app.Toast(terminu.Warning, v.cur.base()+" changed under the cursor: Visual left")
	}
	if s.selecting && !b.there(s.fromL, s.fromC) {
		s.selecting = false
		b.restoreVisual(s)
		v.panel.EnterMode(v.inVisual)
		return v.app.Toast(terminu.Warning, v.cur.base()+" changed: where the selection started is gone")
	}
	b.restoreVisual(s)
	return nil
}
