package ui

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/bzip2"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/dslipak/pdf"
	terminu "github.com/vulcanshen/terminu-go"

	"github.com/vulcanshen/viewu/internal/doc"
	"github.com/vulcanshen/viewu/internal/mermaid"
	"github.com/vulcanshen/viewu/internal/text"
)

// Limits of what is read (tdp P5): a text's first 16 MB, a binary's first
// 1 MB in hex, an image up to 1 MB as a data URI (tdp file-picker.3), a PDF's
// first 20 pages and 128 KB of their text, a tree's first 5000 rows.
const (
	imageBytes = 1 << 20
	pdfPages   = 20
	pdfBytes   = 128 << 10
	headBytes  = 64 << 10 // what the kind is judged by
)

// Variables so a test can make them small.
var (
	textBytes = 16 << 20
	hexBytes  = int64(1 << 20)
	treeRows  = 5000
	colourNow = 64 << 10 // a text up to this size is coloured before it is shown
)

// shown is what the panel shows: a file or a directory, in one view or two
// (the panel's tabs).
type shown struct {
	path  string      // absolute
	info  fs.FileInfo // the target's, a link followed (tdp file-picker.4)
	kind  string      // tdp file-picker.4's TYPE, or markdown, mermaid
	views []view
	// problem is why its content could not be made out, for the App Log
	// (tdp F5); "" when it could.
	problem string
}

// view is one way of showing it: a tab.
type view struct {
	name     string // the tab's name, upper case (tdp components.4)
	lines    []text.Line
	numbered bool
	// layout, when set, lays the lines out at the panel's width.
	layout func(w int) []text.Line
	// colour, when set, is the lines in colour, made in the background:
	// lines are the same in Text until then.
	colour func() []text.Line
	// fact and hint, when lines is empty, say why (tdp screen.8); red marks
	// the fact an error (tdp panel.15).
	fact, hint string
	red        bool
}

// grey is a line of the app's own in Overlay0, not numbered.
func grey(s string) text.Line {
	return text.Line{Chars: text.Chars(s, text.Style{Fg: terminu.Overlay0})}
}

// load reads p for showing. An error is one reading it: it does not exist,
// it may not be read. warn is told when a diagram in it cannot be drawn.
func load(p string, cfg config, warn func(error)) (*shown, error) {
	abs, err := filepath.Abs(p)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return nil, err
	}
	s := &shown{path: abs, info: info}
	if info.IsDir() {
		lines, err := dirTree(abs)
		if err != nil {
			return nil, err
		}
		s.kind = "directory"
		s.views = []view{{name: "DIRECTORY", lines: lines, fact: "Empty directory", hint: "Press [o] to open a file"}}
		return s, nil
	}
	f, err := os.Open(abs)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	head := make([]byte, headBytes)
	n, err := io.ReadFull(f, head)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return nil, err
	}
	head = head[:n]
	s.kind = fileKind(abs, head)
	// One that opens but whose content cannot be made out (a damaged
	// archive or PDF) says so in the middle of the panel, in Red (tdp
	// panel.15); the App Log keeps it (tdp F5).
	unreadable := func(vw view, what string, lines []text.Line, err error) []view {
		vw.lines, vw.hint = lines, "Press [o] to open another file"
		if err != nil {
			s.problem = "Can't read this " + what + ": " + err.Error()
			vw.lines, vw.fact, vw.red = nil, s.problem, true
		}
		return []view{vw}
	}
	switch s.kind {
	case "archive":
		lines, err := archiveTree(abs)
		s.views = unreadable(view{name: "ARCHIVE", fact: "Empty archive"}, "archive", lines, err)
	case "image":
		lines, err := imageLines(abs, info.Size())
		s.views = unreadable(view{name: "IMAGE"}, "image", lines, err)
	case "PDF":
		lines, err := pdfLines(abs, info.Size())
		s.views = unreadable(view{name: "PDF"}, "PDF", lines, err)
	case "binary":
		lines, err := hexLines(abs, info.Size())
		s.views = unreadable(view{name: "BINARY"}, "file", lines, err)
	default: // text, a private key
		content, rest, err := readText(f, head, info.Size())
		if err != nil {
			return nil, err
		}
		s.views = textViews(s, content, cfg.TabWidth, warn)
		if rest > 0 {
			note := grey(fmt.Sprintf("(%s more not shown)", size(rest)))
			for i := range s.views {
				if s.views[i].layout == nil {
					s.views[i].lines = append(s.views[i].lines, note)
				}
				if colour := s.views[i].colour; colour != nil {
					s.views[i].colour = func() []text.Line { return append(colour(), note) }
				}
			}
		}
	}
	return s, nil
}

// textViews are a text file's views: its text, numbered, in the colours its
// name says (tdp note.2); a Markdown file is first drawn as a document, as
// webu draws a page.
func textViews(s *shown, content string, tabW int, warn func(error)) []view {
	name := "TEXT"
	if s.kind == "private key" {
		name = "PRIVATE KEY"
	}
	source := view{name: name, numbered: true, fact: "Empty file", hint: "Press [o] to open another file"}
	source.lines, source.colour = textLines(content, s.path, tabW)
	opt := doc.Options{TabW: tabW, Diagram: mermaid.Draw, OnDiagramError: warn}
	switch {
	case s.kind == "text" && isJSON(s.path):
		// The file as it is first; formatted, a tab away (user 2026-11-03).
		formatted := view{name: "FORMATTED", numbered: true, fact: "Empty file", hint: "Press [h] to see the file as it is"}
		var b bytes.Buffer
		switch err := json.Indent(&b, []byte(content), "", "  "); {
		case strings.TrimSpace(content) == "":
		case err != nil:
			formatted.fact, formatted.red = "Not valid JSON: "+jsonError(content, err), true
		default:
			formatted.lines, formatted.colour = textLines(b.String()+"\n", s.path, tabW)
		}
		return []view{source, formatted}
	case s.kind == "text" && isMarkdown(s.path):
		s.kind = "markdown"
		source.name = "SOURCE"
		if strings.Contains(content, "```mermaid") || strings.Contains(content, "~~~mermaid") {
			go mermaid.Warm()
		}
		document := view{
			name: "DOCUMENT", fact: "Empty file", hint: "Press [o] to open another file",
			layout: func(w int) []text.Line { return doc.Render([]byte(content), w, opt) },
		}
		return []view{document, source}
	case s.kind == "text" && isMermaid(s.path):
		// A diagram is drawn as a mermaid block in a document is, its
		// error the same (tdp P5).
		s.kind = "mermaid"
		source.name = "SOURCE"
		go mermaid.Warm()
		block := []byte("```mermaid\n" + strings.TrimRight(content, "\n") + "\n```\n")
		diagram := view{
			name: "DIAGRAM", fact: "Empty file", hint: "Press [o] to open another file",
			layout: func(w int) []text.Line {
				if strings.TrimSpace(content) == "" {
					return nil
				}
				return doc.Render(block, w, opt)
			},
		}
		return []view{diagram, source}
	}
	return []view{source}
}

// textLines are a text's lines in the colours of lang's syntax, a file's
// path or a language's name, which the lib tells. Colouring takes about a
// second a megabyte: a big text is given in Text, colour the lines in
// colour, to be made in the background (tdp panel.15: the panel loading
// meanwhile).
func textLines(content, lang string, tabW int) (lines []text.Line, colour func() []text.Line) {
	if lang != "" && len(content) > colourNow {
		return text.File(content, "", tabW), func() []text.Line { return text.File(content, lang, tabW) }
	}
	return text.File(content, lang, tabW), nil
}

// isJSON reports a JSON file by its name.
func isJSON(p string) bool { return strings.EqualFold(filepath.Ext(p), ".json") }

// jsonError is why content is not JSON, with the line it is on.
func jsonError(content string, err error) string {
	var syntax *json.SyntaxError
	if errors.As(err, &syntax) {
		line := strings.Count(content[:min(int(syntax.Offset), len(content))], "\n") + 1
		return "line " + strconv.Itoa(line) + ": " + err.Error()
	}
	return err.Error()
}

// isMermaid reports a mermaid file by its name.
func isMermaid(p string) bool {
	switch strings.ToLower(filepath.Ext(p)) {
	case ".mmd", ".mermaid":
		return true
	}
	return false
}

// isMarkdown reports a Markdown file by its name.
func isMarkdown(p string) bool {
	switch strings.ToLower(filepath.Ext(p)) {
	case ".md", ".markdown", ".mdown", ".mkd", ".mkdn":
		return true
	}
	return false
}

// readText is the text of f, its first textBytes; rest is how much more
// there is.
func readText(f *os.File, head []byte, sz int64) (string, int64, error) {
	if int64(len(head)) >= sz {
		return string(head), 0, nil
	}
	b := make([]byte, textBytes)
	copy(b, head)
	n, err := io.ReadFull(f, b[len(head):])
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return "", 0, err
	}
	b = b[:len(head)+n]
	return string(trimPartialRune(b)), max(sz-int64(len(b)), 0), nil
}

// fileKind is what a file is, for its view and its label (tdp
// file-picker.3, file-picker.4).
func fileKind(p string, head []byte) string {
	name := strings.ToLower(filepath.Base(p))
	switch {
	case isArchive(name):
		return "archive"
	case slices.Contains([]string{".png", ".jpg", ".jpeg", ".gif", ".bmp", ".webp", ".tiff", ".tif", ".ico", ".avif", ".heic"}, path.Ext(name)):
		return "image"
	case path.Ext(name) == ".pdf":
		return "PDF"
	case isText(head):
		if isPrivateKey(head) {
			return "private key"
		}
		return "text"
	}
	return "binary"
}

func isArchive(name string) bool {
	for _, ext := range []string{".zip", ".tar", ".tar.gz", ".tgz", ".tar.bz2", ".tbz2"} {
		if strings.HasSuffix(name, ext) {
			return true
		}
	}
	return false
}

func isText(b []byte) bool {
	return !bytes.Contains(b, []byte{0}) && utf8.Valid(trimPartialRune(b))
}

// trimPartialRune is b without a character cut off at its end.
func trimPartialRune(b []byte) []byte {
	for i := 0; i < utf8.UTFMax && i < len(b); i++ {
		if r, _ := utf8.DecodeLastRune(b[:len(b)-i]); r != utf8.RuneError {
			return b[:len(b)-i]
		}
	}
	return b
}

// isPrivateKey reports a first line that is a private key's header (tdp
// file-picker.5).
func isPrivateKey(head []byte) bool {
	first := firstLine(head)
	return strings.HasPrefix(first, "-----BEGIN") && strings.Contains(first, "PRIVATE KEY") || strings.HasPrefix(first, "PuTTY-User-Key-File-")
}

func firstLine(b []byte) string {
	first, _, _ := strings.Cut(string(b), "\n")
	return strings.TrimRight(first, "\r")
}

// size is a size as tdp file-picker.4 writes it: 411 B, 4.2 KB, 3.4 MB.
func size(n int64) string {
	if n < 1024 {
		return fmt.Sprintf("%d B", n)
	}
	v := float64(n)
	for _, unit := range []string{"KB", "MB", "GB", "TB"} {
		v /= 1024
		if v < 1024 || unit == "TB" {
			return fmt.Sprintf("%.1f %s", v, unit)
		}
	}
	return ""
}

// --- a tree: a directory's, an archive's (tdp file-picker.3) ---------------

const (
	glyphFolder = "\U000F024B" // nf-md-folder
	glyphFile   = "\U000F0214" // nf-md-file
)

type node struct {
	name string
	dir  bool
	kids []*node
}

// sortNodes puts directories first, then files, each by name in either case
// (tdp file-picker.2).
func sortNodes(ns []*node) {
	sort.SliceStable(ns, func(i, j int) bool {
		if ns[i].dir != ns[j].dir {
			return ns[i].dir
		}
		return strings.ToLower(ns[i].name) < strings.ToLower(ns[j].name)
	})
}

// treeLines draws nodes three levels deep: the first level with no
// branches, the levels under it joined by ├─ └─ │ in Overlay0; a folder
// Blue, a file Text; past treeRows a last row …. Empty: no lines.
func treeLines(top []*node) []text.Line {
	var out []text.Line
	branch := text.Style{Fg: terminu.Overlay0}
	var walk func(ns []*node, prefix string, depth int)
	walk = func(ns []*node, prefix string, depth int) {
		for i, n := range ns {
			if len(out) >= treeRows {
				return
			}
			var chars []text.Char
			if depth > 1 {
				lead := "├─ "
				if i == len(ns)-1 {
					lead = "└─ "
				}
				chars = append(chars, text.Chars(prefix+lead, branch)...)
			}
			icon, colour := glyphFile, terminu.Text
			if n.dir {
				icon, colour = glyphFolder, terminu.Blue
			}
			chars = append(chars, text.Chars(icon+" ", text.Style{Fg: colour})...)
			chars = append(chars, text.Escape(n.name, text.Style{Fg: colour})...)
			out = append(out, text.Line{Chars: chars})
			if n.dir && depth < 3 {
				next := "  " // the second level under the first, two cells in (tdp file-picker.3)
				if depth > 1 {
					next = prefix + "│  "
					if i == len(ns)-1 {
						next = prefix + "   "
					}
				}
				walk(n.kids, next, depth+1)
			}
		}
	}
	walk(top, "", 1)
	if len(out) >= treeRows {
		out = append(out, grey("…"))
	}
	return out
}

// dirTree is a directory's tree, three levels, hidden entries left out.
func dirTree(dir string) ([]text.Line, error) {
	top, err := readDir(dir, 1)
	if err != nil {
		return nil, err
	}
	return treeLines(top), nil
}

func readDir(dir string, depth int) ([]*node, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var out []*node
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".") {
			continue
		}
		n := &node{name: e.Name(), dir: e.IsDir()}
		if !n.dir && e.Type()&fs.ModeSymlink != 0 {
			if st, err := os.Stat(filepath.Join(dir, e.Name())); err == nil && st.IsDir() {
				n.dir = true
			}
		}
		if n.dir && depth < 3 && e.Type()&fs.ModeSymlink == 0 {
			n.kids, _ = readDir(filepath.Join(dir, e.Name()), depth+1)
		}
		out = append(out, n)
	}
	sortNodes(out)
	return out, nil
}

// archiveTree is what an archive holds, as a directory's tree.
func archiveTree(p string) ([]text.Line, error) {
	names, err := archiveNames(p)
	if err != nil {
		return nil, err
	}
	root := &node{dir: true}
	for _, name := range names {
		dir := strings.HasSuffix(name, "/")
		parts := strings.Split(strings.Trim(name, "/"), "/")
		at := root
		for k, part := range parts {
			if part == "" || part == "." {
				continue
			}
			var next *node
			for _, kid := range at.kids {
				if kid.name == part {
					next = kid
				}
			}
			if next == nil {
				next = &node{name: part, dir: dir || k < len(parts)-1}
				at.kids = append(at.kids, next)
			}
			at = next
		}
	}
	var sortAll func(n *node)
	sortAll = func(n *node) {
		sortNodes(n.kids)
		for _, k := range n.kids {
			sortAll(k)
		}
	}
	sortAll(root)
	return treeLines(root.kids), nil
}

func archiveNames(p string) ([]string, error) {
	lower := strings.ToLower(p)
	if strings.HasSuffix(lower, ".zip") {
		r, err := zip.OpenReader(p)
		if err != nil {
			return nil, err
		}
		defer r.Close()
		var out []string
		for _, f := range r.File {
			out = append(out, f.Name)
		}
		return out, nil
	}
	f, err := os.Open(p)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var rd io.Reader = f
	switch {
	case strings.HasSuffix(lower, ".gz"), strings.HasSuffix(lower, ".tgz"):
		gz, err := gzip.NewReader(f)
		if err != nil {
			return nil, err
		}
		defer gz.Close()
		rd = gz
	case strings.HasSuffix(lower, ".bz2"), strings.HasSuffix(lower, ".tbz2"):
		rd = bzip2.NewReader(f)
	}
	tr := tar.NewReader(rd)
	var out []string
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return out, nil
		}
		if err != nil {
			if len(out) > 0 {
				return out, nil // what could be read of a damaged one
			}
			return nil, err
		}
		name := h.Name
		if h.Typeflag == tar.TypeDir && !strings.HasSuffix(name, "/") {
			name += "/"
		}
		out = append(out, name)
	}
}

// --- an image, a PDF, a binary (tdp file-picker.3) -------------------------

func imageMIME(p string) string {
	ext := strings.TrimPrefix(strings.ToLower(path.Ext(p)), ".")
	switch ext {
	case "jpg":
		ext = "jpeg"
	case "tif":
		ext = "tiff"
	case "ico":
		ext = "x-icon"
	}
	return "image/" + ext
}

// imageLines is the type and the size in grey, a blank line, then the image
// as a data URI, one line that folds at the panel's width; over 1 MB, a line
// saying so.
func imageLines(p string, sz int64) ([]text.Line, error) {
	if sz > imageBytes {
		return []text.Line{grey("(" + size(sz) + " — too large to inline as a data URI)")}, nil
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	mime := imageMIME(p)
	uri := "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(b)
	return []text.Line{
		grey("data:" + mime + " · " + size(sz)),
		{},
		{Chars: text.Chars(uri, text.Style{Fg: terminu.Text})},
	}, nil
}

// pdfLines is the page count in grey, a blank line, then the text of the
// first 20 pages, 128 KB of it at most; (no extractable text) when there is
// none.
func pdfLines(p string, sz int64) (out []text.Line, err error) {
	defer func() {
		if recover() != nil { // the reader panics on some damaged files
			out, err = nil, errors.New("it is damaged")
		}
	}()
	f, err := os.Open(p)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	r, err := pdf.NewReader(f, sz)
	if err != nil {
		return nil, err
	}
	pages := r.NumPage()
	unit := "pages"
	if pages == 1 {
		unit = "page"
	}
	out = []text.Line{grey(fmt.Sprintf("%d %s", pages, unit)), {}}
	read, found := 0, false
	for i := 1; i <= min(pages, pdfPages) && read < pdfBytes; i++ {
		t, err := r.Page(i).GetPlainText(nil)
		if err != nil {
			continue
		}
		t = t[:min(len(t), pdfBytes-read)]
		read += len(t)
		for _, l := range strings.Split(t, "\n") {
			if strings.TrimSpace(l) != "" {
				out = append(out, text.Line{Chars: text.Escape(l, text.Style{Fg: terminu.Text})})
				found = true
			}
		}
	}
	if !found {
		out = append(out, grey("(no extractable text)"))
	}
	return out, nil
}

// hexLines is eight bytes a row in hex, two spaces, then their ASCII in
// Overlay0, 33 cells; the first 1 MB, and a line saying how much more there
// is.
func hexLines(p string, sz int64) ([]text.Line, error) {
	f, err := os.Open(p)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	b := make([]byte, min(sz, hexBytes))
	n, _ := io.ReadFull(f, b)
	b = b[:n]
	out := make([]text.Line, 0, len(b)/8+2)
	for off := 0; off < len(b); off += 8 {
		row := b[off:min(off+8, len(b))]
		var hexes, ascii strings.Builder
		for i := range 8 {
			if i < len(row) {
				fmt.Fprintf(&hexes, "%02x ", row[i])
				c := row[i]
				if c < 0x20 || c > 0x7e {
					c = '.'
				}
				ascii.WriteByte(c)
			} else {
				hexes.WriteString("   ")
			}
		}
		chars := text.Chars(hexes.String()+" ", text.Style{Fg: terminu.Text})
		chars = append(chars, text.Chars(ascii.String(), text.Style{Fg: terminu.Overlay0})...)
		out = append(out, text.Line{Chars: chars})
	}
	if rest := sz - int64(len(b)); rest > 0 {
		out = append(out, grey(fmt.Sprintf("(%s more not shown)", size(rest))))
	}
	return out, nil
}
