package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// capture runs viewu with args, returning its exit code, stdout and stderr.
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
			t.Errorf("viewu %s: %d %q", a, code, out)
		}
	}
	for _, a := range []string{"help", "-h", "--help"} {
		if code, out, _ := capture(t, a); code != 0 || !strings.Contains(out, "viewu [path]") {
			t.Errorf("viewu %s: %d %q", a, code, out)
		}
	}
}

func TestPathNotThere(t *testing.T) {
	p := filepath.Join(t.TempDir(), "nope.txt")
	code, _, errs := capture(t, p)
	if code != 1 || errs != "viewu: "+p+": no such file or directory\n" {
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
	if code != 1 || errs != "viewu: "+p+": permission denied\n" {
		t.Errorf("%d %q", code, errs)
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
