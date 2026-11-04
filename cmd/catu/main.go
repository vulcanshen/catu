// catu is a file viewer for the terminal: a text in colour, Markdown drawn
// as a document, mermaid as diagrams, any other file as a preview.
package main

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strconv"
	"strings"

	"github.com/charmbracelet/x/term"

	"github.com/vulcanshen/catu/internal/ui"
	"github.com/vulcanshen/catu/internal/version"
)

const usage = `catu: a file viewer for the terminal

Usage:
  catu [path]           look at a file or a directory (the current directory when left out)
  cmd | catu [-l fmt]   look at what comes down a pipe (also catu -)
  catu -p [path]        print it instead, in colour, and quit (--print)
  catu version          print the version (-v, --version)
  catu help             print this help (-h, --help)

-l, --lang fmt: the format of what comes down the pipe: md, mermaid, json,
or a language by its name or its extension (go, python, py); without it, a
plain text.

When the output is not a terminal (catu a.md | less, catu a.md > out.txt),
catu prints instead of opening its screen, with no colours: a text file
byte for byte, as cat does; a Markdown file or a mermaid diagram drawn.
The width it is drawn at is $COLUMNS, else 80.

A file named version or help: catu ./version
`

func main() {
	os.Exit(run(os.Args[1:]))
}

// run is catu with args; it returns the exit code.
func run(args []string) int {
	if len(args) == 1 {
		switch a := args[0]; a {
		case "version", "-v", "--version":
			fmt.Println(version.Display())
			return 0
		case "help", "-h", "--help":
			fmt.Print(usage)
			return 0
		}
	}
	var paths []string
	printing, lang := false, ""
	for i := 0; i < len(args); i++ {
		switch a := args[i]; {
		case a == "-p" || a == "--print":
			printing = true
		case a == "-l" || a == "--lang":
			if i+1 == len(args) {
				fmt.Fprintf(os.Stderr, "catu: %s needs a format\n\n%s", a, usage)
				return 2
			}
			i++
			lang = args[i]
		case strings.HasPrefix(a, "--lang="):
			lang = strings.TrimPrefix(a, "--lang=")
		case strings.HasPrefix(a, "-") && a != "-":
			fmt.Fprintf(os.Stderr, "catu: unknown flag %q\n\n%s", a, usage)
			return 2
		default:
			paths = append(paths, a)
		}
	}
	if len(paths) == 0 {
		// Nothing named: what comes down a pipe, else the directory catu is in.
		paths = []string{"."}
		if !term.IsTerminal(os.Stdin.Fd()) {
			paths = []string{"-"}
		}
	}
	if len(paths) > 1 {
		fmt.Fprint(os.Stderr, "catu: one path at a time\n\n"+usage)
		return 2
	}
	if lang != "" {
		if paths[0] != "-" {
			fmt.Fprintf(os.Stderr, "catu: --lang is for what comes down a pipe\n\n%s", usage)
			return 2
		}
		if err := ui.CheckFormat(lang); err != nil {
			fmt.Fprintf(os.Stderr, "catu: %s\n", err)
			return 2
		}
	}
	toTerminal := term.IsTerminal(os.Stdout.Fd())
	path, shown := paths[0], paths[0]
	if path == "-" {
		shown = "standard input"
		if !toTerminal && !ui.IsDocument(lang) {
			// As cat does: every byte, as it comes, however much.
			if _, err := io.Copy(os.Stdout, os.Stdin); err != nil {
				fmt.Fprintf(os.Stderr, "catu: %s: %s\n", shown, reason(err))
				return 1
			}
			return 0
		}
		spooled, done, err := ui.Spool(os.Stdin, lang)
		if err != nil {
			fmt.Fprintf(os.Stderr, "catu: %s: %s\n", shown, reason(err))
			return 1
		}
		defer done()
		path = spooled
	}
	if printing || !toTerminal {
		// Printed (tasks 13, 14): in colour only on a terminal.
		failed := false
		ui.Print([]string{path}, os.Stdout, toTerminal, width(toTerminal), func(_ string, err error) {
			fmt.Fprintf(os.Stderr, "catu: %s: %s\n", shown, reason(err))
			failed = true
		})
		if failed {
			return 1
		}
		return 0
	}
	app, err := ui.Open(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "catu: %s: %s\n", shown, reason(err))
		return 1
	}
	if err := app.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "catu:", err)
		return 1
	}
	return 0
}

// width is what a document or a diagram is printed at: the terminal's width
// on one, else $COLUMNS, else 80.
func width(toTerminal bool) int {
	if toTerminal {
		if w, _, err := term.GetSize(os.Stdout.Fd()); err == nil && w > 0 {
			return w
		}
	}
	if w, err := strconv.Atoi(os.Getenv("COLUMNS")); err == nil && w > 0 {
		return w
	}
	return 80
}

// reason is why a path could not be opened, as cat says it.
func reason(err error) string {
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return "no such file or directory"
	case errors.Is(err, fs.ErrPermission):
		return "permission denied"
	}
	var pe *fs.PathError
	if errors.As(err, &pe) {
		return pe.Err.Error()
	}
	return err.Error()
}
