package yaegikernel

import (
	"fmt"
	"strings"
)

// Edit changes one place in a file: old must occur exactly once in the file
// at path, and is replaced by new. It returns a short diff of the change.
// The model emits the changed lines, not the file: a round spent 55 to 82%
// of its time emitting output tokens, and whole-file rewrites were the large
// calls (docs/evidence/selfdev-round-cycle-time-2026-11-03.md). A backtick in
// old or new may be spelled @@BT@@ as in WriteFileBT. It composes ReadFile and
// WriteFile, so the role table, the read-only denial and the jail apply
// unchanged; a refused edit writes nothing.
func (s *ChoirScope) Edit(path, old, new string) (string, error) {
	return s.EditMulti(path, [][]string{{old, new}})
}

// EditMulti applies several {old, new} replacements to one file in order, each
// of which must match exactly once in the file as the earlier ones left it. It
// is all or nothing: the first pair that fails is named ("edit 2: ...") and
// the file is not written.
func (s *ChoirScope) EditMulti(path string, edits [][]string) (string, error) {
	if err := s.mutateDenied("WriteFile"); err != nil {
		return "", err
	}
	if len(edits) == 0 {
		return "", fmt.Errorf("choir.EditMulti: no edits")
	}
	src, err := s.ReadFile(path)
	if err != nil {
		return "", err
	}
	cur := src
	var diff strings.Builder
	for i, e := range edits {
		if len(e) != 2 {
			return "", fmt.Errorf("choir.EditMulti: edit %d: want {old, new}, got %d strings", i+1, len(e))
		}
		old := strings.ReplaceAll(e[0], backtickPlaceholder, "`")
		repl := strings.ReplaceAll(e[1], backtickPlaceholder, "`")
		if old == "" {
			return "", fmt.Errorf("choir.Edit: edit %d: old is empty", i+1)
		}
		if old == repl {
			return "", fmt.Errorf("choir.Edit: edit %d: old and new are the same", i+1)
		}
		switch n := strings.Count(cur, old); {
		case n == 0:
			return "", fmt.Errorf("choir.Edit: edit %d: old not found in %s (it must match the file exactly, whitespace included; ReadFile the region again)", i+1, path)
		case n > 1:
			return "", fmt.Errorf("choir.Edit: edit %d: old occurs %d times in %s; add surrounding lines so it occurs once", i+1, n, path)
		}
		cur = strings.Replace(cur, old, repl, 1)
		diff.WriteString(shortDiff(old, repl))
	}
	if _, err := s.WriteFile(path, cur); err != nil {
		return "", err
	}
	return diff.String(), nil
}

// shortDiff renders a replaced span as -/+ lines, each side cut to a few lines
// so a large edit's receipt stays small.
func shortDiff(old, repl string) string {
	const maxLines = 6
	side := func(prefix, text string) string {
		lines := strings.Split(strings.TrimSuffix(text, "\n"), "\n")
		var b strings.Builder
		for i, l := range lines {
			if i == maxLines {
				fmt.Fprintf(&b, "%s... (%d more lines)\n", prefix, len(lines)-maxLines)
				break
			}
			b.WriteString(prefix + l + "\n")
		}
		return b.String()
	}
	return side("-", old) + side("+", repl)
}
