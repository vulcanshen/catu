// catu is a file viewer for the terminal: a text in colour, Markdown drawn
// as a document, mermaid as diagrams, any other file as a preview.
package main

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"github.com/charmbracelet/x/term"

	"github.com/vulcanshen/catu/internal/ui"
	"github.com/vulcanshen/catu/internal/version"
)

const usage = `catu: a file viewer for the terminal

Usage:
  catu [path...]        look at a file or a directory (the current directory when left out);
                        two or more are listed beside the one shown
  cmd | catu [-l fmt]   look at what comes down a pipe (also catu -)
  catu -p [path...]     print them instead, in colour, and quit (--print)
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
	pipes := 0
	for _, p := range paths {
		if p == "-" {
			pipes++
		}
	}
	switch {
	case pipes > 1:
		fmt.Fprintf(os.Stderr, "catu: - (what comes down a pipe) once only\n\n%s", usage)
		return 2
	case lang != "" && pipes == 0:
		fmt.Fprintf(os.Stderr, "catu: --lang is for what comes down a pipe\n\n%s", usage)
		return 2
	case lang != "":
		if err := ui.CheckFormat(lang); err != nil {
			fmt.Fprintf(os.Stderr, "catu: %s\n", err)
			return 2
		}
	}
	toTerminal := term.IsTerminal(os.Stdout.Fd())
	// The pipe's file is only for this run: it goes when catu does (tdp E3),
	// on its own or otherwise: the terminal closed (SIGHUP), Ctrl-C or kill
	// while the pipe is read or printed, the pipe it prints into closed
	// (catu | head). kill -9 cannot be caught; SIGQUIT and a crash are left
	// to Go, to dump what it was doing. On the screen they are terminu-go's
	// (SIGHUP) and Bubble Tea's (SIGINT, SIGTERM): Run returns on them, and
	// the file goes as catu ends on its own.
	var mu sync.Mutex
	var cleanups []func()
	cleanup := func() {
		mu.Lock()
		defer mu.Unlock()
		for _, done := range cleanups {
			done()
		}
		cleanups = nil
	}
	defer cleanup()
	quit := make(chan os.Signal, 1)
	if pipes > 0 {
		signal.Notify(quit, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGPIPE)
		defer signal.Stop(quit)
		go func() {
			s := <-quit
			cleanup()
			os.Exit(128 + int(s.(syscall.Signal)))
		}()
	}
	failed := false
	fail := func(name string, err error) {
		if errors.Is(err, syscall.EPIPE) {
			// What catu prints into is closed (catu | head): it ends there,
			// saying nothing, as cat does.
			cleanup()
			os.Exit(128 + int(syscall.SIGPIPE))
		}
		fmt.Fprintf(os.Stderr, "catu: %s: %s\n", name, reason(err))
		failed = true
	}
	// spool puts what comes down the pipe in a file catu reads as any other;
	// "" when it could not.
	spool := func() string {
		spooled, err := ui.Spool(os.Stdin, lang, func(done func()) {
			mu.Lock()
			cleanups = append(cleanups, done)
			mu.Unlock()
		})
		if err != nil {
			fail("standard input", err)
			return ""
		}
		return spooled
	}
	if printing || !toTerminal {
		// Printed (tasks 13, 14), one after another as cat does (task 16): in
		// colour only on a terminal. Settings that cannot be read stop it
		// before anything is printed, as Esc does on the screen (tdp E3).
		if err := ui.CheckSettings(); err != nil {
			fmt.Fprintln(os.Stderr, "catu:", err)
			return 1
		}
		for _, p := range paths {
			name := p
			if p == "-" {
				name = "standard input"
				if !toTerminal && !ui.IsDocument(lang) {
					// As cat does: every byte, as it comes, however much.
					if _, err := io.Copy(os.Stdout, os.Stdin); err != nil {
						fail(name, err)
					}
					continue
				}
				if p = spool(); p == "" {
					continue
				}
			}
			ui.Print([]string{p}, os.Stdout, toTerminal, width(toTerminal), func(_ string, err error) { fail(name, err) })
		}
		if failed {
			return 1
		}
		return 0
	}
	spooled := ""
	for i, p := range paths {
		if p == "-" {
			if spooled = spool(); spooled == "" {
				return 1
			}
			paths[i] = spooled
		}
	}
	app, err := ui.OpenAll(paths)
	if err != nil {
		var oe *ui.OpenError
		name := ""
		if errors.As(err, &oe) {
			name = oe.Path
			if name == spooled {
				name = "standard input"
			}
		}
		fail(name, err)
		return 1
	}
	signal.Stop(quit) // the screen's: Run returns on them, and the file goes after
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
