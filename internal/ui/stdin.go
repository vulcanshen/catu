package ui

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/alecthomas/chroma/v2/lexers"
)

// Standard input is read as a file is (task 15): what comes down a pipe is
// put in a file of its own, named for the format given, and catu reads that
// file, its kind and colours told as a file's are; it is shown as standard
// input.
const stdinName = "standard input"

var (
	stdinPath string // the file standard input is in; "" when it is not read
	stdinMore bool   // there was more of it than textBytes
)

// TODO(tgo pipe probe): with standard input a pipe, tgo up to v1.2.0-beta.4
// skips its startup probe, icons and powerline symbols taken a cell wide
// (CATU__ICON_WIDTH, CATU__POWERLINE_WIDTH set them); fixed in the lib, not
// yet released: nothing to change here but this note once catu moves to it.

// Spool puts standard input, its first textBytes, in a file of its own
// named for lang (md, mermaid, json, a language's name or an extension; ""
// for a plain text) and returns that file's path; done removes it. An error
// is lang not being a format catu knows, or the file not being made.
func Spool(r io.Reader, lang string) (path string, done func(), err error) {
	ext, err := extOf(lang)
	if err != nil {
		return "", nil, err
	}
	dir, err := os.MkdirTemp("", "catu-")
	if err != nil {
		return "", nil, err
	}
	done = func() { os.RemoveAll(dir) }
	path = filepath.Join(dir, "stdin"+ext)
	f, err := os.Create(path)
	if err != nil {
		done()
		return "", nil, err
	}
	n, err := io.Copy(f, io.LimitReader(r, int64(textBytes)+1))
	if err == nil && n > int64(textBytes) {
		err, stdinMore = f.Truncate(int64(textBytes)), true
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		done()
		return "", nil, err
	}
	stdinPath = path
	return path, done, nil
}

// CheckFormat is an error when lang is not a format catu knows.
func CheckFormat(lang string) error {
	_, err := extOf(lang)
	return err
}

// IsDocument reports whether lang draws standard input (a Markdown document,
// a mermaid diagram) rather than giving its text as it is.
func IsDocument(lang string) bool {
	ext, _ := extOf(lang)
	return ext == ".md" || ext == ".mmd"
}

// extOf is the extension a file in format lang has: md and markdown are
// Markdown, mermaid and mmd a diagram, anything else a language chroma
// knows, by its name or its extension.
func extOf(lang string) (string, error) {
	switch l := strings.ToLower(lang); l {
	case "":
		return "", nil
	case "md", "markdown":
		return ".md", nil
	case "mermaid", "mmd":
		return ".mmd", nil
	}
	if lexers.Match("stdin."+lang) != nil {
		return "." + lang, nil
	}
	if l := lexers.Get(lang); l != nil {
		for _, g := range l.Config().Filenames {
			if strings.HasPrefix(g, "*.") && !strings.ContainsAny(g[2:], "*?[") {
				return g[1:], nil
			}
		}
	}
	return "", fmt.Errorf("unknown format %q", lang)
}

// name is how what is shown is called: its path, the home as ~, or standard
// input.
func (s *shown) name() string {
	if s.stdin {
		return stdinName
	}
	return tilde(s.path)
}

// base is its name without its directory.
func (s *shown) base() string {
	if s.stdin {
		return stdinName
	}
	return filepath.Base(s.path)
}
