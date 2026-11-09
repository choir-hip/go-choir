package yaegikernel

import (
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
)

// API-shape errors: the common cell failures that are about what a choir
// verb takes or returns (a wrong result count, a wrong argument type, a field
// the result does not have, a slice bound past the end). Each hint names the
// verb's real shape, from the same reflection the prompt card is built from.

// choirAssignRE finds `a, b := choir.V(`, `a = choir.V(` and
// `for _, u := range choir.V(`.
var choirAssignRE = regexp.MustCompile(`((?:\w+\s*,\s*)*\w+)\s*:?=\s*(range\s+)?choir\.(\w+)\(`)

var (
	assignMismatchRE = regexp.MustCompile(`assignment mismatch`)
	argTypeRE        = regexp.MustCompile(`cannot (?:convert|use)\b`)
	errorLineRE      = regexp.MustCompile(`(?:^|\s)(?:[^:\s]*\.go:)?(\d+):\d+:`)
	sliceBoundsRE    = regexp.MustCompile(`slice index out of bounds|slice bounds out of range`)
)

const sliceBoundsHint = "A string or slice is shorter than the bound you sliced it to (s[:3000] on a shorter string panics). Slice with s[:min(len(s), n)], and print only what you need."

// verbResultCount is the number of results of a function verb, -1 when the
// name is not a plain function.
func verbResultCount(name string) int {
	if _, ns := choirNamespaces[name]; ns {
		return -1
	}
	m, ok := choirScopeType.MethodByName(name)
	if !ok {
		return -1
	}
	return m.Type.NumOut()
}

// resultCountHint answers "assignment mismatch": it finds the choir call in
// the cell bound to the wrong number of variables and names its shape.
func resultCountHint(surface map[string]bool, src, msg string) string {
	if !assignMismatchRE.MatchString(msg) {
		return ""
	}
	for _, m := range choirAssignRE.FindAllStringSubmatch(src, -1) {
		verb := m[3]
		if !surface[verb] || m[2] != "" {
			continue
		}
		want := verbResultCount(verb)
		if want < 0 {
			continue
		}
		if got := len(strings.Split(m[1], ",")); got != want {
			return fmt.Sprintf("choir.%s returns %d value(s), the cell bound %d. Signature: %s. %s", verb, want, got, choirSignature(verb), callShape(verb))
		}
	}
	return ""
}

// argumentTypeHint answers a type mismatch on a line that calls a choir
// verb with that verb's signature.
func argumentTypeHint(surface map[string]bool, src, msg string) string {
	if !argTypeRE.MatchString(msg) {
		return ""
	}
	m := errorLineRE.FindStringSubmatch(msg)
	if m == nil {
		return ""
	}
	lineNo, _ := strconv.Atoi(m[1])
	lines := strings.Split(src, "\n")
	if lineNo < 1 || lineNo > len(lines) {
		return ""
	}
	var sigs []string
	seen := map[string]bool{}
	for _, c := range verbCallRE.FindAllStringSubmatch(lines[lineNo-1], -1) {
		if surface[c[1]] && !seen[c[1]] && len(sigs) < 3 {
			seen[c[1]] = true
			sigs = append(sigs, choirSignature(c[1]))
		}
	}
	if len(sigs) == 0 {
		return ""
	}
	return "A value has the wrong type for this call. Signature: " + strings.Join(sigs, "; ") + "."
}

// trackedFieldHint answers "undefined method/field X" when the cell binds the
// value it reads X from to a choir call: it lists that call's result fields,
// instead of guessing the type from the field name alone.
func trackedFieldHint(surface map[string]bool, src, missing string) string {
	for _, m := range choirAssignRE.FindAllStringSubmatch(src, -1) {
		verb := m[3]
		if !surface[verb] {
			continue
		}
		vars := strings.Split(m[1], ",")
		v := strings.TrimSpace(vars[0])
		if m[2] != "" {
			v = strings.TrimSpace(vars[len(vars)-1])
		}
		if v == "_" || !regexp.MustCompile(`\b`+regexp.QuoteMeta(v)+`\.`+regexp.QuoteMeta(missing)+`\b`).MatchString(src) {
			continue
		}
		fields := resultFieldsFor(verb)
		if fields == "" {
			continue
		}
		what := "what choir." + verb + " returns"
		if m[2] != "" {
			what = "an element of choir." + verb + "()"
		}
		hint := fmt.Sprintf("%s has no field %s: %s. %s", v, missing, what, fields)
		folded := ""
		for _, f := range fieldNames(verb) {
			if strings.EqualFold(f, missing) {
				folded = f
			}
		}
		if folded != "" {
			hint += " Go field names are case-sensitive: write ." + folded + "."
		} else if c := closestName(missing, fieldNames(verb)); c != "" {
			hint += " Closest: " + c + "."
		} else if other := resultFieldHint(missing); other != "" {
			hint += " " + other
		}
		return hint
	}
	return ""
}

// fieldNames lists the exported field names of a verb's struct result.
func fieldNames(verb string) []string {
	m, ok := choirScopeType.MethodByName(verb)
	if !ok || m.Type.NumOut() == 0 {
		return nil
	}
	t := m.Type.Out(0)
	for t.Kind() == reflect.Slice || t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return nil
	}
	var out []string
	for i := 0; i < t.NumField(); i++ {
		if f := t.Field(i); f.IsExported() {
			out = append(out, f.Name)
		}
	}
	return out
}
