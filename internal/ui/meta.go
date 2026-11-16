package ui

import (
	"os"

	terminu "github.com/vulcanshen/terminu-go"

	"github.com/vulcanshen/catu/internal/text"
)

// metaView is the META tab, the last of every file: its metadata, the four
// rows tdp file-picker.4 writes a file's in (TYPE, SIZE, MODIFIED, MODE),
// as panel.12 draws one value a row: the label upper case in Blue, the value
// in Text, two cells after the longest label. A value there is none of is
// —: what came down a pipe was never modified or given a mode.
func metaView(s *shown) view {
	info := s.info
	kind := s.kind
	switch kind {
	case "markdown", "mermaid":
		kind = "text" // file-picker.4's kinds
	}
	if l, err := os.Lstat(s.path); err == nil && l.Mode()&os.ModeSymlink != 0 && !s.stdin {
		if to, err := os.Readlink(s.path); err == nil {
			kind = "link → " + to
		}
	}
	sz := size(info.Size())
	if info.IsDir() {
		sz = items(s.path)
	}
	modified, mode := info.ModTime().Format("2006-01-02 15:04"), lsMode(info.Mode())
	if s.stdin {
		modified, mode = "—", "—"
	}
	rows := [][2]string{{"TYPE", kind}, {"SIZE", sz}, {"MODIFIED", modified}, {"MODE", mode}}
	labelW := 0
	for _, r := range rows {
		labelW = max(labelW, len(r[0]))
	}
	label, value := text.Style{Fg: terminu.Blue}, text.Style{Fg: terminu.Text}
	lines := make([]text.Line, len(rows))
	for i, r := range rows {
		chars := text.Chars(r[0], label)
		chars = append(chars, text.Spaces(labelW-len(r[0])+2, value)...)
		lines[i] = text.Line{Chars: append(chars, text.Escape(r[1], value)...)}
	}
	return view{name: "META", lines: lines}
}
