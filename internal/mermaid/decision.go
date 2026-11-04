package mermaid

import (
	"regexp"
	"strings"
	"unicode"
)

// decisionClass marks a flowchart's decisions ({…}) once they are laid out
// as boxes ([…]). pele draws a decision as a diamond around its words, two or
// three times as tall as a box, whose slanting sides a grid of cells can only
// draw as steps, and whose lines leave from along them; laid out as a box, a
// decision is as tall as one, its lines leave from its border, and it is
// drawn as a box with a ◆ on either side (svg.go, user 2026-11-04).
const decisionClass = "catuDecision"

var (
	chartRe = regexp.MustCompile(`^(flowchart|graph)\b`)
	shapeRe = regexp.MustCompile(`\{[^{}]*\}`)
)

// asDecisions is src with each decision of a flowchart written as a box
// marked decisionClass; any other diagram is as it is. A decision that has
// a class of its own (A{…}:::c) stays a diamond: a node takes one :::.
func asDecisions(src string) string {
	lines := strings.Split(src, "\n")
	i := firstLine(lines)
	if i == len(lines) || !chartRe.MatchString(strings.TrimSpace(lines[i])) {
		return src
	}
	for i++; i < len(lines); i++ {
		if skipped(strings.TrimSpace(lines[i])) {
			continue
		}
		lines[i] = asBoxes(lines[i])
	}
	return strings.Join(lines, "\n")
}

// firstLine is where the diagram's first line is: after front matter,
// comments and blank lines.
func firstLine(lines []string) int {
	i := 0
	for i < len(lines) && strings.TrimSpace(lines[i]) == "" {
		i++
	}
	if i < len(lines) && strings.TrimSpace(lines[i]) == "---" {
		for i++; i < len(lines) && strings.TrimSpace(lines[i]) != "---"; i++ {
		}
		i++
	}
	for i < len(lines) {
		s := strings.TrimSpace(lines[i])
		if s != "" && !strings.HasPrefix(s, "%%") {
			break
		}
		i++
	}
	return i
}

// skipped are the lines with no nodes in them: comments and styling.
func skipped(s string) bool {
	for _, p := range []string{"%%", "classDef ", "class ", "style ", "linkStyle ", "click "} {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}

// asBoxes writes the decisions on a line, a node's id then {words}, as boxes
// marked decisionClass, the words quoted so that a [ or ] in them stays
// words.
func asBoxes(line string) string {
	var b strings.Builder
	last := 0
	for _, m := range shapeRe.FindAllStringIndex(line, -1) {
		start, end := m[0], m[1]
		if start == 0 || !idChar(rune(line[start-1])) || strings.HasPrefix(line[end:], ":::") || strings.HasPrefix(line[end:], "}") ||
			strings.Count(line[:start], `"`)%2 == 1 { // in a quoted label
			continue
		}
		words := line[start+1 : end-1]
		if !strings.Contains(words, `"`) {
			words = `"` + words + `"`
		}
		b.WriteString(line[last:start])
		b.WriteString("[" + words + "]:::" + decisionClass)
		last = end
	}
	b.WriteString(line[last:])
	return b.String()
}

// idChar is a character a node's id ends with (the byte before { is its
// last; a multi-byte letter's last byte counts as one).
func idChar(r rune) bool {
	return r == '_' || r >= 0x80 || unicode.IsLetter(r) || unicode.IsDigit(r)
}
