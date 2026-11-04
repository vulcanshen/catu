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

// Spool puts standard input, its first textBytes, in a file of its own
// named for lang (md, mermaid, json, a language's name or an extension; ""
// for a plain text) in the system temp dir and returns that file's path.
// made is given what removes it as soon as it is there, before standard
// input is read: catu may end while it is. An error is lang not being a
// format catu knows, or the file not being made.
func Spool(r io.Reader, lang string, made func(done func())) (path string, err error) {
	ext, err := extOf(lang)
	if err != nil {
		return "", err
	}
	dir, err := os.MkdirTemp("", "catu-")
	if err != nil {
		return "", err
	}
	done := func() { os.RemoveAll(dir) }
	made(done)
	path = filepath.Join(dir, "stdin"+ext)
	f, err := os.Create(path)
	if err != nil {
		done()
		return "", err
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
		return "", err
	}
	stdinPath = path
	return path, nil
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
