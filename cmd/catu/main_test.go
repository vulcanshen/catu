package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// capture runs catu with args, returning its exit code, stdout and stderr.
func capture(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	outR, outW, _ := os.Pipe()
	errR, errW, _ := os.Pipe()
	stdout, stderr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = outW, errW
	code := run(args)
	os.Stdout, os.Stderr = stdout, stderr
	outW.Close()
	errW.Close()
	out, _ := io.ReadAll(outR)
	errs, _ := io.ReadAll(errR)
	return code, string(out), string(errs)
}

func TestVersionAndHelp(t *testing.T) {
	for _, a := range []string{"version", "-v", "--version"} {
		if code, out, _ := capture(t, a); code != 0 || out != "dev\n" {
			t.Errorf("catu %s: %d %q", a, code, out)
		}
	}
	for _, a := range []string{"help", "-h", "--help"} {
		if code, out, _ := capture(t, a); code != 0 || !strings.Contains(out, "catu [path]") {
			t.Errorf("catu %s: %d %q", a, code, out)
		}
	}
}

func TestPathNotThere(t *testing.T) {
	p := filepath.Join(t.TempDir(), "nope.txt")
	code, _, errs := capture(t, p)
	if code != 1 || errs != "catu: "+p+": no such file or directory\n" {
		t.Errorf("%d %q", code, errs)
	}
}

func TestPathNotReadable(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads everything")
	}
	p := filepath.Join(t.TempDir(), "secret.txt")
	os.WriteFile(p, []byte("x"), 0)
	code, _, errs := capture(t, p)
	if code != 1 || errs != "catu: "+p+": permission denied\n" {
		t.Errorf("%d %q", code, errs)
	}
}

// TestPrintWhenNotATerminal: with its output not a terminal, catu prints
// instead of opening its screen (task 13): a text file byte for byte, as cat
// does; a Markdown file and a mermaid diagram drawn, $COLUMNS wide; a
// directory as a tree; no colours, -p or not; what it cannot read is said on
// stderr, exit code 1.
func TestPrintWhenNotATerminal(t *testing.T) {
	t.Setenv("CATU__CONFIG", t.TempDir())
	t.Setenv("COLUMNS", "60")
	dir := t.TempDir()
	write := func(name, content string) string {
		p := filepath.Join(dir, name)
		os.WriteFile(p, []byte(content), 0o644)
		return p
	}
	raw := "package a\n\tfunc  x() {}\r\nno newline at the end"
	for _, args := range [][]string{{write("a.go", raw)}, {"-p", filepath.Join(dir, "a.go")}} {
		if code, out, errs := capture(t, args...); code != 0 || out != raw || errs != "" {
			t.Errorf("catu %v: %d %q %q", args, code, out, errs)
		}
	}
	code, out, _ := capture(t, write("a.md", "# Title\n\nSome **bold** words.\n"))
	if code != 0 || !strings.Contains(out, "Title") || strings.Contains(out, "#") || strings.Contains(out, "**") || strings.Contains(out, "\x1b") {
		t.Errorf("a document: %d %q", code, out)
	}
	code, out, _ = capture(t, write("flow.mmd", "flowchart TD\n  A[Start] --> B[End]\n"))
	if code != 0 || !strings.Contains(out, "│ Start") || !strings.Contains(out, "▼") || strings.Contains(out, "flowchart") {
		t.Errorf("a diagram: %d\n%s", code, out)
	}
	for _, row := range strings.Split(out, "\n") {
		if w := len([]rune(row)); w > 60 {
			t.Errorf("wider than $COLUMNS: %q", row)
		}
	}
	os.Mkdir(filepath.Join(dir, "sub"), 0o755)
	write("sub/b.txt", "b")
	if code, out, _ := capture(t, filepath.Join(dir, "sub")); code != 0 || !strings.Contains(out, "b.txt") {
		t.Errorf("a directory: %d %q", code, out)
	}
	bad := write("bad.zip", "PK\x03\x04 not a zip")
	if code, out, errs := capture(t, bad); code != 1 || out != "" || !strings.HasPrefix(errs, "catu: "+bad+": Can't read this archive: ") {
		t.Errorf("a broken archive: %d %q %q", code, out, errs)
	}
	if code, _, errs := capture(t, write("bad.mmd", "flowchart LR\n  B[[[\n")); code != 1 || !strings.Contains(errs, "Can't draw this diagram: Parse error") {
		t.Errorf("a diagram that cannot be drawn: %d %q", code, errs)
	}
}

// captureIn runs catu with args and input coming down a pipe as its
// standard input.
func captureIn(t *testing.T, input string, args ...string) (int, string, string) {
	t.Helper()
	p := filepath.Join(t.TempDir(), "in")
	os.WriteFile(p, []byte(input), 0o644)
	f, _ := os.Open(p)
	defer f.Close()
	stdin := os.Stdin
	os.Stdin = f
	defer func() { os.Stdin = stdin }()
	return capture(t, args...)
}

// TestReadFromAPipe: with nothing named and input coming down a pipe, or
// with -, catu reads it (task 15); printed as it is when it is a text, as
// cat does, drawn when -l names a document's format; -l names a format
// catu knows, and is for the pipe only.
func TestReadFromAPipe(t *testing.T) {
	t.Setenv("CATU__CONFIG", t.TempDir())
	t.Setenv("COLUMNS", "60")
	raw := "a\tb\r\nno newline"
	for _, args := range [][]string{nil, {"-"}, {"-l", "go"}} {
		if code, out, errs := captureIn(t, raw, args...); code != 0 || out != raw || errs != "" {
			t.Errorf("catu %v: %d %q %q", args, code, out, errs)
		}
	}
	code, out, _ := captureIn(t, "# Title\n\nSome **bold** words.\n", "-l", "md")
	if code != 0 || !strings.Contains(out, "Title") || strings.Contains(out, "#") || strings.Contains(out, "**") {
		t.Errorf("a document: %d %q", code, out)
	}
	code, out, _ = captureIn(t, "flowchart LR\n  A[Start] --> B[End]\n", "--lang=mermaid")
	if code != 0 || !strings.Contains(out, "│ Start") || strings.Contains(out, "flowchart") {
		t.Errorf("a diagram: %d %q", code, out)
	}
	if code, _, errs := captureIn(t, "flowchart LR\n  B[[[\n", "-l", "mmd"); code != 1 || !strings.HasPrefix(errs, "catu: standard input: Can't draw this diagram") {
		t.Errorf("a diagram that cannot be drawn: %d %q", code, errs)
	}
	if code, _, errs := captureIn(t, "x", "-l", "nosuch"); code != 2 || errs != "catu: unknown format \"nosuch\"\n" {
		t.Errorf("an unknown format: %d %q", code, errs)
	}
	if code, _, errs := captureIn(t, "x", "-l", "md", "a.md"); code != 2 || !strings.Contains(errs, "--lang is for what comes down a pipe") {
		t.Errorf("-l with a file: %d %q", code, errs)
	}
	if code, _, errs := captureIn(t, "x", "-l"); code != 2 || !strings.Contains(errs, "-l needs a format") {
		t.Errorf("-l with no format: %d %q", code, errs)
	}
}

func TestBadArguments(t *testing.T) {
	if code, _, errs := capture(t, "a", "b"); code != 2 || !strings.Contains(errs, "one path at a time") {
		t.Errorf("two paths: %d %q", code, errs)
	}
	if code, _, errs := capture(t, "--watch"); code != 2 || !strings.Contains(errs, `unknown flag "--watch"`) {
		t.Errorf("a flag: %d %q", code, errs)
	}
}
