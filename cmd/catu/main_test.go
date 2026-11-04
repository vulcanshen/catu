package main

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"
)

// TestMain runs catu itself, as a process of its own, when a test starts
// this binary with CATU_TEST_ARGS set: a signal is sent to a process.
func TestMain(m *testing.M) {
	if args, ok := os.LookupEnv("CATU_TEST_ARGS"); ok {
		os.Exit(run(strings.Fields(args)))
	}
	os.Exit(m.Run())
}

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
		if code, out, _ := capture(t, a); code != 0 || !strings.Contains(out, "catu [path...]") {
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

// TestPipeFileGoes: the file what comes down a pipe is put in is in $TMPDIR
// and goes when catu does (tdp E3-26): on its own, the terminal closed, Ctrl-C or kill while the pipe is still read, the pipe it
// prints into closed (catu | head); ended by a signal, the exit code a shell
// gives a process ended by it.
func TestPipeFileGoes(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("no signals but kill on Windows")
	}
	start := func(stdin *os.File, stdout, stderr io.Writer) (*exec.Cmd, string) {
		t.Helper()
		tmp := t.TempDir()
		c := exec.Command(os.Args[0])
		c.Env = append(os.Environ(), "CATU_TEST_ARGS=-l md", "TMPDIR="+tmp, "CATU__CONFIG="+t.TempDir())
		c.Stdin, c.Stdout, c.Stderr = stdin, stdout, stderr
		if err := c.Start(); err != nil {
			t.Fatal(err)
		}
		return c, tmp
	}
	gone := func(what, tmp string) {
		t.Helper()
		if left, _ := filepath.Glob(filepath.Join(tmp, "catu-*")); len(left) > 0 {
			t.Errorf("%s: %v left", what, left)
		}
	}
	doc := filepath.Join(t.TempDir(), "in.md")
	os.WriteFile(doc, []byte(strings.Repeat("* item\n", 10000)), 0o644)

	in, _ := os.Open(doc)
	c, tmp := start(in, nil, nil)
	if err := c.Wait(); err != nil {
		t.Errorf("on its own: %v", err)
	}
	in.Close()
	gone("on its own", tmp)

	for _, sig := range []syscall.Signal{syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM} {
		r, w, _ := os.Pipe()
		w.WriteString("# Title\n") // and the pipe left open: catu still reads it
		c, tmp := start(r, nil, nil)
		r.Close()
		for i := 0; i < 500; i++ {
			if f, _ := filepath.Glob(filepath.Join(tmp, "catu-*", "stdin.md")); len(f) == 1 {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
		if f, _ := filepath.Glob(filepath.Join(tmp, "catu-*", "stdin.md")); len(f) != 1 {
			t.Fatalf("%v: no stdin.md in $TMPDIR while the pipe is read", sig)
		}
		c.Process.Signal(sig)
		c.Wait()
		w.Close()
		if code := c.ProcessState.ExitCode(); code != 128+int(sig) {
			t.Errorf("%v: exit code %d, want %d", sig, code, 128+int(sig))
		}
		gone(sig.String(), tmp)
	}

	in, _ = os.Open(doc)
	r, w, _ := os.Pipe()
	r.Close() // catu | head, head gone
	errs := new(strings.Builder)
	c, tmp = start(in, w, errs)
	c.Wait()
	in.Close()
	w.Close()
	if code := c.ProcessState.ExitCode(); code != 128+int(syscall.SIGPIPE) || errs.Len() > 0 {
		t.Errorf("printing into a closed pipe: exit code %d, %q", code, errs)
	}
	gone("printing into a closed pipe", tmp)
}

// TestPrintSeveral: two or more paths printed one after another, as cat a b
// does (task 16): what cannot be read is said on stderr and the rest are
// printed, exit code 1; - among them is what comes down the pipe.
func TestPrintSeveral(t *testing.T) {
	t.Setenv("CATU__CONFIG", t.TempDir())
	dir := t.TempDir()
	a, b := filepath.Join(dir, "a.txt"), filepath.Join(dir, "b.txt")
	os.WriteFile(a, []byte("one\n"), 0o644)
	os.WriteFile(b, []byte("two\n"), 0o644)
	if code, out, _ := capture(t, a, b); code != 0 || out != "one\ntwo\n" {
		t.Errorf("two files: %d %q", code, out)
	}
	nope := filepath.Join(dir, "nope")
	if code, out, errs := capture(t, a, nope, b); code != 1 || out != "one\ntwo\n" || errs != "catu: "+nope+": no such file or directory\n" {
		t.Errorf("one not there: %d %q %q", code, out, errs)
	}
	if code, out, _ := captureIn(t, "piped\n", a, "-", b); code != 0 || out != "one\npiped\ntwo\n" {
		t.Errorf("- among them: %d %q", code, out)
	}
	if code, _, errs := captureIn(t, "x", "-", "-"); code != 2 || !strings.Contains(errs, "once only") {
		t.Errorf("- twice: %d %q", code, errs)
	}
}

// TestPrintSettingsUnreadable: settings that cannot be read, with no screen
// to ask on, stop catu as Esc does on the screen (tdp E3, user 2026-11-04):
// nothing printed, the file and why on stderr, exit code 1, no file touched
// and no .bak made; a pipe's text is not copied either.
func TestPrintSettingsUnreadable(t *testing.T) {
	cfg := t.TempDir()
	t.Setenv("CATU__CONFIG", cfg)
	os.WriteFile(filepath.Join(cfg, "config.yaml"), []byte(":\n:"), 0o644)
	a := filepath.Join(t.TempDir(), "a.txt")
	os.WriteFile(a, []byte("text\n"), 0o644)
	check := func(what string, code int, out, errs string) {
		t.Helper()
		if code == 0 || out != "" || !strings.HasPrefix(errs, "catu: "+filepath.Join(cfg, "config.yaml")+" can't be read: ") {
			t.Errorf("%s: %d %q %q", what, code, out, errs)
		}
	}
	code, out, errs := capture(t, a)
	check("into a pipe", code, out, errs)
	code, out, errs = capture(t, "-p", a)
	check("-p", code, out, errs)
	code, out, errs = captureIn(t, "piped\n")
	check("from a pipe", code, out, errs)
	entries, _ := os.ReadDir(cfg)
	if len(entries) != 1 || entries[0].Name() != "config.yaml" {
		t.Errorf("the settings directory has %v, want config.yaml alone", entries)
	}
}

func TestBadArguments(t *testing.T) {
	if code, _, errs := capture(t, "--watch"); code != 2 || !strings.Contains(errs, `unknown flag "--watch"`) {
		t.Errorf("a flag: %d %q", code, errs)
	}
}
