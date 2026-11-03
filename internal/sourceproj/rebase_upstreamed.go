package sourceproj

import (
	"bytes"
	"context"
	"fmt"
	"path"
	"strconv"
	"strings"
)

// UpstreamedHunk is a hunk of the applied change P that a rebased candidate
// lacks because the new base B' already carries it (problem 14,
// docs/problems/selfdev-arm-rounds-fail-at-freeze-and-capsules-lack-real-content-2026-11-03.md).
// Hunk is P's zero-context hunk header on its own base, so the record says
// which lines were dropped.
type UpstreamedHunk struct {
	Path string `json:"path"`
	Hunk string `json:"hunk"`
}

// A hunk of P on one path is upstreamed when B' carries its content. The
// test is on tokens, not lines, because main writes the same change in its
// own shape: claude's round 3 added `export { fetchWithRenewal,
// AuthRequiredError } from './auth.js';` after the media exports, main's
// e5915e0ca added `export { AuthRequiredError, fetchWithRenewal } from
// './auth.js';` at the end and the media routes inside the existing export
// list. A hunk h of P (git diff -U0 from B to B+P) is upstreamed when:
//
//  1. h only adds lines (a removal B' never made would be lost otherwise);
//  2. h adds at least one code line (not blank, not a comment line in a file
//     whose comment syntax is known), so a comment-only hunk has no evidence;
//  3. one of h's word tokens is not anywhere in B's text of the path: h
//     brings something new, so common lines (`if err != nil {`, `return
//     err`) that B' also added elsewhere do not count;
//  4. every token of h's code lines, string literals aside (words, numbers,
//     operators and brackets), is among the tokens of the code lines B' added
//     to the path (git diff -U0 from B to B'): main made a change of the same
//     words, not merely mentions one name of it;
//  5. every string literal of h's code lines is in B''s text of the path;
//  6. on each code line, its values (numbers, true, false, nil and the like,
//     and literals B lacked) stand on one line B' added together with every
//     word of that line, so a changed value is not "carried" by the name on
//     one line and the value on another.
//
// The evidence (the lines B' added) never includes comment text, in any
// common comment syntax (evidenceLines, splitEvidence): a commented-out
// `# timeout: 30` carries nothing. Comment lines of P's hunk need no
// evidence: a hunk upstreamed for its code drops its comments with it
// (named residual; the drop is recorded).
//
// Then the candidate is checked against P without the upstreamed hunks, by
// the same rules as any other path, and the path passes only if that holds.
// Whitespace and order inside a line do not count; the place in the file
// does not either. Named residuals: a hunk whose meaning is its place or its
// operator order, with all its tokens in lines main added elsewhere, can be
// dropped (every such drop is recorded on the verdict, the lineage receipt
// and the Freeze result); a hunk that brings no word B lacked (re-exporting a
// name B already imports) is never upstreamed, so its rebase is refused, the
// safe direction.

// upstreamedReduction returns P's post-image of one path (theirs) with the
// upstreamed hunks taken out, and those hunks, or nil when none is.
func upstreamedReduction(ctx context.Context, repo, filePath string, base, ours, theirs blobEntry, baseText, oursText, theirsText []byte) ([]byte, []UpstreamedHunk, error) {
	if base.sha == theirs.sha || ours.sha == base.sha {
		return nil, nil, nil // P changes nothing here, or B' changed nothing
	}
	pDiff, err := blobDiffU0(ctx, repo, base.sha, theirs.sha)
	if err != nil {
		return nil, nil, err
	}
	bDiff, err := blobDiffU0(ctx, repo, base.sha, ours.sha)
	if err != nil {
		return nil, nil, err
	}
	comments := commentPrefixes(filePath)
	added := map[string]bool{}
	var addedLines []map[string]bool
	for _, h := range bDiff {
		for _, line := range evidenceLines(h.added, comments) {
			code, literals := splitEvidence(line, comments)
			set := map[string]bool{}
			for _, tok := range append(code, literals...) {
				added[tok] = true
				set[tok] = true
			}
			addedLines = append(addedLines, set)
		}
	}
	if len(added) == 0 {
		return nil, nil, nil
	}
	baseWords := map[string]bool{}
	for _, tok := range wordTokens(string(baseText)) {
		baseWords[tok] = true
	}
	oursStr := string(oursText)
	theirsLines := bytes.SplitAfter(theirsText, []byte("\n"))
	if n := len(theirsLines); n > 0 && len(theirsLines[n-1]) == 0 {
		theirsLines = theirsLines[:n-1]
	}
	drop := map[int]bool{}
	var hunks []UpstreamedHunk
	for _, h := range pDiff {
		if h.removed != 0 || len(h.added) == 0 || h.newStart < 1 || h.newStart-1+len(h.added) > len(theirsLines) {
			continue
		}
		if hunkUpstreamed(h.added, comments, baseWords, added, addedLines, oursStr, string(baseText)) {
			for i := range h.added {
				drop[h.newStart-1+i] = true
			}
			hunks = append(hunks, UpstreamedHunk{Path: filePath, Hunk: h.header})
		}
	}
	if len(hunks) == 0 {
		return nil, nil, nil
	}
	var reduced []byte
	for i, line := range theirsLines {
		if !drop[i] {
			reduced = append(reduced, line...)
		}
	}
	return reduced, hunks, nil
}

func hunkUpstreamed(lines []string, comments []string, baseWords, bootedAdded map[string]bool, bootedAddedLines []map[string]bool, booted, base string) bool {
	codeCount, brings := 0, false
	for _, line := range codeLines(lines, comments) {
		code, literals := splitCode(line, comments)
		if len(code) == 0 && len(literals) == 0 {
			continue // blank
		}
		codeCount++
		var words, values []string
		for _, tok := range code {
			if !bootedAdded[tok] {
				return false
			}
			if isValue(tok) {
				values = append(values, tok)
			} else if isWord(tok) {
				words = append(words, tok)
				if !baseWords[tok] {
					brings = true
				}
			}
		}
		for _, lit := range literals {
			if !strings.Contains(booted, lit) {
				return false
			}
			if !strings.Contains(base, lit) {
				values = append(values, lit) // a literal B lacked is a value
			}
			for _, tok := range wordTokens(lit) {
				if !baseWords[tok] {
					brings = true
				}
			}
		}
		// A value (a number, true, false, nil, or a literal B lacked) must
		// stand on one line B' added beside every word of this line:
		// `telemetry: true` is not carried by `telemetry: false` plus an
		// `audited: true` elsewhere (review r1, DeepSeek F1), whether or not
		// B mentioned telemetry already (review r2, Devin F1).
		if len(values) > 0 && !oneLineHolds(bootedAddedLines, append(words, values...)) {
			return false
		}
	}
	return codeCount > 0 && brings
}

func oneLineHolds(lines []map[string]bool, toks []string) bool {
	for _, set := range lines {
		all := true
		for _, tok := range toks {
			if !set[tok] {
				all = false
				break
			}
		}
		if all {
			return true
		}
	}
	return false
}

// isValue reports whether a code token is a literal value: a number or a
// keyword literal.
func isValue(tok string) bool {
	if tok == "" {
		return false
	}
	if '0' <= tok[0] && tok[0] <= '9' {
		return true
	}
	switch tok {
	case "true", "false", "nil", "null", "undefined", "None", "True", "False", "NaN", "Infinity":
		return true
	}
	return false
}

// codeLines drops the comment lines of a run of added lines: a line opening
// with a line-comment marker, and the lines of a block comment opened and
// not yet closed within the run. A line that starts with `*` is code unless
// it is inside such a block (`*dst = *src` is code; review r1, DeepSeek F3).
// A run that starts inside an existing block comment counts its lines as
// code: the stricter test.
func codeLines(lines []string, comments []string) []string {
	if len(comments) == 0 {
		return lines
	}
	var out []string
	inBlock := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if inBlock {
			if strings.Contains(trimmed, "*/") {
				inBlock = false
			}
			continue
		}
		if strings.HasPrefix(trimmed, "/*") {
			inBlock = !strings.Contains(trimmed[2:], "*/")
			continue
		}
		if strings.HasPrefix(trimmed, "//") {
			continue
		}
		out = append(out, line)
	}
	return out
}

type u0Hunk struct {
	header            string
	newStart, removed int
	added             []string
}

// blobDiffU0 is git's zero-context diff of two blobs, as hunks.
func blobDiffU0(ctx context.Context, repo, a, b string) ([]u0Hunk, error) {
	args := append([]string{"diff", "-U0", "--diff-algorithm=myers"}, diffOpts...)
	raw, err := gitRaw(ctx, repo, nil, nil, append(args, a, b)...)
	if err != nil {
		return nil, fmt.Errorf("sourceproj: diff %s %s: %w", a, b, err)
	}
	var out []u0Hunk
	var cur *u0Hunk
	inHunk := false
	for _, line := range strings.Split(string(raw), "\n") {
		switch {
		case strings.HasPrefix(line, "@@ "):
			h, ok := parseU0Header(line)
			if !ok {
				return nil, fmt.Errorf("sourceproj: unexpected hunk header %q", line)
			}
			out = append(out, h)
			cur, inHunk = &out[len(out)-1], true
		case strings.HasPrefix(line, "Binary files "):
			return nil, nil
		case !inHunk:
		case strings.HasPrefix(line, "+"):
			cur.added = append(cur.added, line[1:])
		}
	}
	return out, nil
}

// parseU0Header reads "@@ -a[,b] +c[,d] @@".
func parseU0Header(line string) (u0Hunk, bool) {
	f := strings.Fields(line)
	if len(f) < 4 || f[0] != "@@" || f[3] != "@@" || !strings.HasPrefix(f[1], "-") || !strings.HasPrefix(f[2], "+") {
		return u0Hunk{}, false
	}
	_, removed, ok1 := rangeOf(f[1][1:])
	start, _, ok2 := rangeOf(f[2][1:])
	if !ok1 || !ok2 {
		return u0Hunk{}, false
	}
	newStart := start
	return u0Hunk{header: strings.Join(f[:4], " "), newStart: newStart, removed: removed}, true
}

func rangeOf(s string) (start, count int, ok bool) {
	a, b, has := strings.Cut(s, ",")
	start, err := strconv.Atoi(a)
	if err != nil {
		return 0, 0, false
	}
	count = 1
	if has {
		if count, err = strconv.Atoi(b); err != nil {
			return 0, 0, false
		}
	}
	return start, count, true
}

// commentPrefixes are the line-comment openers of a file kind whose syntax is
// known; elsewhere every line counts as code (the stricter test).
func commentPrefixes(filePath string) []string {
	switch strings.ToLower(path.Ext(filePath)) {
	case ".go", ".ts", ".tsx", ".js", ".mjs", ".cjs", ".jsx", ".svelte", ".c", ".h", ".rs", ".java", ".swift", ".kt":
		return []string{"//", "/*"}
	}
	return nil
}

// evidenceCommentPrefixes open a comment line in some common syntax. Only the
// evidence side drops these: dropping evidence can only refuse.
var evidenceCommentPrefixes = []string{"//", "/*", "*", "#", "--", ";", "<!--", "%"}

// evidenceLines are the lines B' added that may carry evidence: codeLines,
// then without any line that opens like a comment in a common syntax.
func evidenceLines(lines []string, comments []string) []string {
	var out []string
	for _, line := range codeLines(lines, comments) {
		trimmed := strings.TrimSpace(line)
		comment := false
		for _, prefix := range evidenceCommentPrefixes {
			if strings.HasPrefix(trimmed, prefix) {
				comment = true
				break
			}
		}
		if !comment {
			out = append(out, line)
		}
	}
	return out
}

// splitEvidence is splitCode for a line B' added, cut at the first comment
// opener outside a string literal in any common syntax (`//`, `/*`, `#`, ` --`),
// so an inline comment's words are not evidence (review r2, Devin F2).
func splitEvidence(line string, comments []string) (code, literals []string) {
	return splitCodeCut(line, comments, true)
}

// splitCode tokenizes one line: its code tokens (words, numbers, runs of
// operator characters, and each bracket or separator alone) outside string
// literals, and the literals with their quotes, up to a trailing line
// comment in a file whose comment syntax is known. Comment lines are dropped
// before (codeLines).
func splitCode(line string, comments []string) (code, literals []string) {
	return splitCodeCut(line, comments, false)
}

func splitCodeCut(line string, comments []string, evidence bool) (code, literals []string) {
	trimmed := strings.TrimSpace(line)
	lineComments := len(comments) > 0
	i := 0
	for i < len(trimmed) {
		c := trimmed[i]
		switch {
		case lineComments && strings.HasPrefix(trimmed[i:], "//"):
			// A trailing comment is not code on either side (review r1,
			// Devin F1).
			return code, literals
		case evidence && (strings.HasPrefix(trimmed[i:], "//") || strings.HasPrefix(trimmed[i:], "/*") || c == '#' ||
			(strings.HasPrefix(trimmed[i:], "--") && (i == 0 || trimmed[i-1] == ' '))):
			return code, literals
		case c == '"' || c == '\'' || c == '`':
			j := i + 1
			for j < len(trimmed) && trimmed[j] != c {
				if trimmed[j] == '\\' {
					j++
				}
				j++
			}
			if j >= len(trimmed) {
				j = len(trimmed) - 1
			}
			literals = append(literals, trimmed[i:j+1])
			i = j + 1
		case isWordByte(c):
			j := i
			for j < len(trimmed) && isWordByte(trimmed[j]) {
				j++
			}
			code = append(code, trimmed[i:j])
			i = j
		case c == ' ' || c == '\t' || c == '\r':
			i++
		case strings.IndexByte("(){}[];,", c) >= 0:
			code = append(code, trimmed[i:i+1])
			i++
		default:
			j := i
			for j < len(trimmed) && !isWordByte(trimmed[j]) && strings.IndexByte(" \t\r\"'`(){}[];,", trimmed[j]) < 0 {
				j++
			}
			code = append(code, trimmed[i:j])
			i = j
		}
	}
	return code, literals
}

func wordTokens(text string) []string {
	var out []string
	i := 0
	for i < len(text) {
		if !isWordByte(text[i]) {
			i++
			continue
		}
		j := i
		for j < len(text) && isWordByte(text[j]) {
			j++
		}
		out = append(out, text[i:j])
		i = j
	}
	return out
}

func isWord(tok string) bool { return tok != "" && isWordByte(tok[0]) }

func isWordByte(c byte) bool {
	return c == '_' || c == '$' || c >= 0x80 || ('0' <= c && c <= '9') || ('a' <= c && c <= 'z') || ('A' <= c && c <= 'Z')
}
