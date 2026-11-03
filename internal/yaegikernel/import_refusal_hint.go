package yaegikernel

import (
	"fmt"
	"regexp"
	"strings"
)

// Live trace 2026-11-03: engineering cells that wrote `import "os"` were
// refused import_preflight with the reason "direct OS manipulation must route
// through capsule broker" and no word on what to call. importRefusalHint names
// the choir replacement for each refused import it can map, and hands back the
// cell with the import removed when nothing else in the cell uses the package
// (a mechanical rewrite). A cell that uses the package gets a precise hint
// instead; it is never rewritten, since the calls need a human-sized change.

var importRefusedRE = regexp.MustCompile(`import "([^"]+)" refused`)

// importReplacements maps a refused import to what to use instead.
var importReplacements = map[string]string{
	"os":        "for files use choir.ReadFile(path) (string, error), choir.WriteFile(path, content) (int, error) and choir.ListDir(path) ([]string, error); for commands use choir.Exec(name, []string{args...}); os.Exit has no equivalent, end the cell instead",
	"os/exec":   "run commands with choir.Exec(name, []string{args...}), or choir.Start for one that outlasts the cell",
	"io/ioutil": "use choir.ReadFile(path) and choir.WriteFile(path, content)",
	"net/http":  "fetch a page with choir.Read(url); there is no raw HTTP client",
	"net":       "fetch a page with choir.Read(url); there is no raw network access",
}

// importRefusalHint returns the hint for an import refusal message, or "".
func importRefusalHint(src, msg string) string {
	m := importRefusedRE.FindStringSubmatch(msg)
	if m == nil {
		return ""
	}
	path := m[1]
	use, ok := importReplacements[path]
	if !ok {
		return ""
	}
	hint := fmt.Sprintf("import %q is not available in a cell: %s.", path, use)
	if corrected, ok := removeUnusedImport(src, path); ok {
		hint += "\nNothing else in the cell uses it. Send this cell instead:\n" + corrected
	}
	return hint
}

// removeUnusedImport removes the import of path from src when no code in src
// refers to the package by its name. It reports false when the import is used
// or is not found as a plain (unaliased) import.
func removeUnusedImport(src, path string) (string, bool) {
	name := path[strings.LastIndex(path, "/")+1:]
	quoted := regexp.QuoteMeta(`"` + path + `"`)
	single := regexp.MustCompile(`(?m)^[ \t]*import[ \t]+` + quoted + `[ \t]*;?[ \t]*`)
	inBlock := regexp.MustCompile(`(?m)^[ \t]*` + quoted + `[ \t]*\n`)
	var out string
	switch {
	case single.MatchString(src):
		out = single.ReplaceAllString(src, "")
	case inBlock.MatchString(src):
		out = inBlock.ReplaceAllString(src, "")
	default:
		return "", false
	}
	if regexp.MustCompile(`\b` + regexp.QuoteMeta(name) + `\.`).MatchString(codeOnly(out)) {
		return "", false
	}
	out = strings.TrimLeft(out, "\n")
	if strings.TrimSpace(out) == "" || len(out) > maxTopLevelDefineCorrection {
		return "", false
	}
	return out, true
}
