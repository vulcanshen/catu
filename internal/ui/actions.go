package ui

import (
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	terminu "github.com/vulcanshen/terminu-go"
)

// Glyphs of viewu's own popups (tdp popup: the app's choice).
const (
	glyphSearch = ""          // nf-fa-search
	glyphOpen   = "\U000F0DCF" // nf-md-file_find
)

// actions are the panel's: all of them on the panel as a whole, the text
// has no items (tdp P3, M2, M3).
func (v *viewer) actions() []terminu.Action {
	b := v.body
	nothing := func() bool { return len(b.lines) == 0 }
	noHits := func() bool { return len(b.hits) == 0 }
	return []terminu.Action{
		{Key: "o", Label: "Open", Desc: "another file", Scope: terminu.PanelScope, Hint: "open", Do: v.open},
		{Key: "/", Label: "Search", Desc: "find text in it", Scope: terminu.PanelScope, Hint: "search", Hidden: nothing, Do: v.search},
		{Key: "n", Label: "Next", Desc: "the next place found", Scope: terminu.PanelScope, Hidden: noHits,
			Do: func() tea.Cmd { b.step(1); return nil }},
		{Key: "N", Label: "Previous", Desc: "the place found before", Scope: terminu.PanelScope, Hidden: noHits,
			Do: func() tea.Cmd { b.step(-1); return nil }},
		{Key: "v", Label: "Visual", Desc: "select text to copy", Scope: terminu.PanelScope, Hint: "visual", Hidden: nothing, Do: v.visual},
	}
}

// open is o: a file-picker in the directory of what is shown, the cursor on
// it; the file chosen is shown instead (tdp file-picker). One that cannot be
// read keeps the picker open with an error popup over it (tdp toast.1); the
// file shown, the Space menu it was opened from goes too (tdp T1).
func (v *viewer) open() tea.Cmd {
	root, value := filepath.Dir(v.cur.path), v.cur.path
	if v.cur.info.IsDir() {
		root, value = v.cur.path, ""
	}
	var next *shown
	return v.app.PickFile(terminu.FilePicker{
		Title: "Open", Glyph: glyphOpen, Root: root, Value: value,
		Commit: func(p string) error {
			s, err := load(p, v.cfg, v.diagramError)
			if err != nil {
				return err
			}
			next = s
			return nil
		},
		Done: func(string) tea.Cmd {
			if next == nil {
				return nil
			}
			v.show(next)
			v.app.Log(terminu.Info, "Opened "+tilde(next.path))
			return v.app.CloseAll()
		},
	})
}

// search is /: an input popup asks for the text; every place it is is a hit
// (tdp panel.14, note.3).
func (v *viewer) search() tea.Cmd {
	return v.app.Text(terminu.TextInput{
		Title: "Search", Glyph: glyphSearch, Prompt: "Find in " + filepath.Base(v.cur.path), Verb: "find", Search: true,
		Done: func(q string) tea.Cmd {
			if v.body.find(q) == 0 {
				v.panel.OnEsc = nil
				return v.app.Toast(terminu.Info, "No line has "+q)
			}
			// Esc clears them: it does something here only while there are
			// hits (tdp note.5).
			v.panel.OnEsc = v.clearHits
			return nil
		},
	})
}

// moves are the keys that move the Visual mode's cursor (tdp K12, note.4).
func (v *viewer) moves() []terminu.ModeKey {
	move := func(key string) tea.Cmd { v.body.move(key); return nil }
	return []terminu.ModeKey{
		{Key: "h|j|k|l", Keys: []string{"h", "j", "k", "l", "←", "↓", "↑", "→"}, Desc: "left, down, up, right", NoFooter: true, Do: move},
		{Key: "w|b|e", Keys: []string{"w", "b", "e"}, Desc: "next word, back a word, end of the word", NoFooter: true, Do: move},
		{Key: "0|$", Keys: []string{"0", "$"}, Desc: "start, end of the line", NoFooter: true, Do: move},
		{Key: "u|d", Keys: []string{"u", "d", "PgUp", "PgDn"}, Desc: "half a page up, down", NoFooter: true, Do: move},
		{Key: "gg|G", Keys: []string{"gg", "G"}, Desc: "to the top, the bottom", NoFooter: true, Do: move},
	}
}

// visual is v: the Visual mode (tdp note.4, K11). The cursor comes out at
// the start of the first line in sight; v again starts selecting there
// (Visual>select), y copies what is selected and leaves.
func (v *viewer) visual() tea.Cmd {
	b := v.body
	b.startVisual()
	sel := &terminu.Mode{Name: "select", Exit: "unselect",
		Keys: append(v.moves(), terminu.ModeKey{Key: "y", Desc: "copy", Item: true, Label: "Copy", MenuDesc: "what is selected",
			Do: func(string) tea.Cmd {
				text := b.selected()
				b.leave()
				v.panel.LeaveMode()
				terminu.Clipboard(text)
				return v.app.Toast(terminu.Success, "Copied")
			}}),
		OnExit: func() tea.Cmd { b.unselect(); return nil }}
	v.panel.EnterMode(&terminu.Mode{Name: "Visual", Exit: "cancel",
		Keys: append(v.moves(), terminu.ModeKey{Key: "v", Desc: "select", Label: "Select", MenuDesc: "from here",
			Do: func(string) tea.Cmd {
				b.startSelect()
				v.panel.EnterLayer(sel)
				return nil
			}}),
		OnExit: func() tea.Cmd { b.leave(); return nil }})
	return nil
}

// status is the statusbar (tdp screen.4): the size, when it was modified
// and its mode, as tdp file-picker.4 writes them; each value but the last as
// wide as it can be (tdp L2).
func (v *viewer) status(w int) []terminu.Span {
	info := v.cur.info
	sz := size(info.Size())
	if info.IsDir() {
		sz = items(v.cur.path)
	}
	quiet := terminu.Fg(terminu.Overlay0)
	return []terminu.Span{
		{Text: "Size: " + terminu.Pad(sz, 11) + "  ", Style: quiet},
		{Text: "Modified: " + info.ModTime().Format("2006-01-02 15:04") + "  ", Style: quiet},
		{Text: "Mode: " + lsMode(info.Mode()), Style: quiet},
	}
}

// items is how many entries a directory has, hidden ones left out, as tdp
// file-picker.4 writes it: 12 items, 1 item; — when it cannot be read.
func items(dir string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "—"
	}
	n := 0
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), ".") {
			n++
		}
	}
	if n == 1 {
		return "1 item"
	}
	return strconv.Itoa(n) + " items"
}

// lsMode is a mode as ls -l writes it (tdp file-picker.4).
func lsMode(m fs.FileMode) string {
	kind := byte('-')
	switch {
	case m&fs.ModeDir != 0:
		kind = 'd'
	case m&fs.ModeSymlink != 0:
		kind = 'l'
	case m&fs.ModeCharDevice != 0:
		kind = 'c'
	case m&fs.ModeDevice != 0:
		kind = 'b'
	case m&fs.ModeNamedPipe != 0:
		kind = 'p'
	case m&fs.ModeSocket != 0:
		kind = 's'
	}
	b := []byte{kind}
	for i, who := range []fs.FileMode{0o700, 0o070, 0o007} {
		p := m.Perm() & who
		shift := uint(6 - 3*i)
		r, w, x := p>>shift&4 != 0, p>>shift&2 != 0, p>>shift&1 != 0
		b = append(b, "-r"[btoi(r)], "-w"[btoi(w)])
		special := i == 0 && m&fs.ModeSetuid != 0 || i == 1 && m&fs.ModeSetgid != 0 || i == 2 && m&fs.ModeSticky != 0
		switch {
		case special && i < 2:
			b = append(b, "Ss"[btoi(x)])
		case special:
			b = append(b, "Tt"[btoi(x)])
		default:
			b = append(b, "-x"[btoi(x)])
		}
	}
	return string(b)
}

func btoi(b bool) int {
	if b {
		return 1
	}
	return 0
}
