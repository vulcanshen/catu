// viewu is a file viewer for the terminal: a text in colour, Markdown drawn
// as a document, mermaid as diagrams, any other file as a preview.
package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"

	"github.com/vulcanshen/viewu/internal/ui"
	"github.com/vulcanshen/viewu/internal/version"
)

const usage = `viewu: a file viewer for the terminal

Usage:
  viewu [path]       look at a file or a directory (the current directory when left out)
  viewu version      print the version (-v, --version)
  viewu help         print this help (-h, --help)

A file named version or help: viewu ./version
`

func main() {
	os.Exit(run(os.Args[1:]))
}

// run is viewu with args; it returns the exit code.
func run(args []string) int {
	path := "."
	switch {
	case len(args) > 1:
		fmt.Fprint(os.Stderr, "viewu: one path at a time\n\n"+usage)
		return 2
	case len(args) == 1:
		switch a := args[0]; {
		case a == "version" || a == "-v" || a == "--version":
			fmt.Println(version.Display())
			return 0
		case a == "help" || a == "-h" || a == "--help":
			fmt.Print(usage)
			return 0
		case strings.HasPrefix(a, "-") && a != "-":
			fmt.Fprintf(os.Stderr, "viewu: unknown flag %q\n\n%s", a, usage)
			return 2
		default:
			path = a
		}
	}
	app, err := ui.Open(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "viewu: %s: %s\n", path, reason(err))
		return 1
	}
	if err := app.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "viewu:", err)
		return 1
	}
	return 0
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
