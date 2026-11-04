package ui

import (
	"errors"
	"io"
	"os"
	"strings"

	"github.com/vulcanshen/catu/internal/text"
)

// Print writes what catu shows first of each of paths to out, one after
// another as cat does, without a screen (tasks 13, 14; tdp E1): a text file
// byte for byte when out is not coloured, else the first tab's lines, a
// document or a diagram laid out w cells wide, coloured as on the screen when
// coloured. A path it cannot is told to fail, and the rest are printed.
func Print(paths []string, out io.Writer, coloured bool, w int, fail func(path string, err error)) {
	cfg, _, _ := readConfig() // settings it cannot read: the defaults, with no screen to ask on
	for _, p := range paths {
		if err := printOne(p, out, cfg, coloured, w); err != nil {
			fail(p, err)
		}
	}
}

func printOne(p string, out io.Writer, cfg config, coloured bool, w int) error {
	s, err := load(p, cfg, func(error) {}) // a broken block in a document is drawn in it
	if err != nil {
		return err
	}
	if !coloured && (s.kind == "text" || s.kind == "private key") {
		// As cat does: every byte as it is, all of it, however big.
		f, err := os.Open(s.path)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = io.Copy(out, f)
		return err
	}
	vw := s.views[0]
	if vw.fail != nil {
		return errors.New(vw.fail.Fact)
	}
	lines := vw.lines
	switch {
	case vw.layout != nil:
		if lines, err = vw.layout(w); err != nil {
			return err
		}
	case vw.colour != nil && coloured:
		lines = vw.colour()
	}
	for _, l := range lines {
		row := strings.TrimRight(l.Drawn(), " ")
		if coloured {
			// The spaces a row is padded with, on no colour of their own, are
			// not printed.
			chars := l.Chars
			for len(chars) > 0 && chars[len(chars)-1].Draw == " " && chars[len(chars)-1].St.Bg == (text.Style{}).Bg {
				chars = chars[:len(chars)-1]
			}
			row = text.Render(chars)
		}
		if _, err := io.WriteString(out, row+"\n"); err != nil {
			return err
		}
	}
	return nil
}
