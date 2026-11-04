// catu is a file viewer for the terminal: a text in colour, Markdown drawn
// as a document, mermaid as diagrams, any other file as a preview.
package main

import (
	"errors"
	"fmt"
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
  catu [path]       look at a file or a directory (the current directory when left out)
  catu -p [path]    print it instead, in colour, and quit (--print)
  catu version      print the version (-v, --version)
  catu help         print this help (-h, --help)

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
	printing := false
	for _, a := range args {
		switch {
		case a == "-p" || a == "--print":
			printing = true
		case strings.HasPrefix(a, "-") && a != "-":
			fmt.Fprintf(os.Stderr, "catu: unknown flag %q\n\n%s", a, usage)
			return 2
		default:
			paths = append(paths, a)
		}
	}
	switch len(paths) {
	case 0:
		paths = []string{"."}
	case 1:
	default:
		fmt.Fprint(os.Stderr, "catu: one path at a time\n\n"+usage)
		return 2
	}
	if toTerminal := term.IsTerminal(os.Stdout.Fd()); printing || !toTerminal {
		// Printed (tasks 13, 14): in colour only on a terminal.
		failed := false
		ui.Print(paths, os.Stdout, toTerminal, width(toTerminal), func(p string, err error) {
			fmt.Fprintf(os.Stderr, "catu: %s: %s\n", p, reason(err))
			failed = true
		})
		if failed {
			return 1
		}
		return 0
	}
	path := paths[0]
	app, err := ui.Open(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "catu: %s: %s\n", path, reason(err))
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
