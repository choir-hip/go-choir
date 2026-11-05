// Package contract holds the desk protocol's contract helpers and tests: a
// decoder that checks a JSON body against the locked shapes, and the emit,
// golden and skew tests that use it (docs/design/app-agent-desks-2026-11-04.md
// 5.5). The reduce-side tests live in internal/agentcore, which owns the
// reducer.
package contract

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/deskprotocol"
)

// CheckShape decodes raw as the locked type typeName ("types.CommitmentRecord")
// and returns every way it falls outside the lock: a key the locked type does
// not have, a required field that is missing, an enum value the lock does not
// list (for a closed enum). It follows field types into other locked types.
func CheckShape(lock *deskprotocol.Surface, typeName string, raw []byte) []string {
	var v any
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&v); err != nil {
		return []string{"not JSON: " + err.Error()}
	}
	var out []string
	checkValue(lock, typeName, v, typeName, &out)
	return out
}

func checkValue(lock *deskprotocol.Surface, typ string, v any, path string, out *[]string) {
	typ = strings.TrimSpace(typ)
	switch {
	case strings.HasPrefix(typ, "*"):
		if v != nil {
			checkValue(lock, typ[1:], v, path, out)
		}
		return
	case strings.HasPrefix(typ, "[]"):
		if arr, ok := v.([]any); ok {
			for i, e := range arr {
				checkValue(lock, typ[2:], e, fmt.Sprintf("%s[%d]", path, i), out)
			}
		}
		return
	case strings.HasPrefix(typ, "map["):
		close := strings.Index(typ, "]")
		if m, ok := v.(map[string]any); ok && close > 0 {
			for _, k := range sortedKeys(m) {
				checkValue(lock, typ[close+1:], m[k], path+"."+k, out)
			}
		}
		return
	}
	n, ok := lock.Names[typ]
	if !ok {
		return // a standard-library or basic type
	}
	switch n.Kind {
	case "struct":
		m, ok := v.(map[string]any)
		if !ok {
			*out = append(*out, fmt.Sprintf("%s: want an object for %s", path, typ))
			return
		}
		byJSON := map[string]deskprotocol.Field{}
		for _, f := range n.Fields {
			byJSON[f.JSON] = f
		}
		for _, k := range sortedKeys(m) {
			f, ok := byJSON[k]
			if !ok {
				*out = append(*out, fmt.Sprintf("%s: key %q is outside the locked fields of %s", path, k, typ))
				continue
			}
			checkValue(lock, f.Type, m[k], path+"."+k, out)
		}
		for _, f := range n.Fields {
			if !f.Optional && f.JSON != "" {
				if _, present := m[f.JSON]; !present {
					*out = append(*out, fmt.Sprintf("%s: required field %q of %s is missing", path, f.JSON, typ))
				}
			}
		}
	case "enum":
		s, ok := v.(string)
		if !ok || n.Open {
			return
		}
		for _, lit := range n.Values {
			if lit == fmt.Sprintf("%q", s) {
				return
			}
		}
		*out = append(*out, fmt.Sprintf("%s: %q is not a locked value of closed enum %s", path, s, typ))
	}
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// LockedIntentKinds returns the staged-intent kind literals in the lock.
func LockedIntentKinds(lock *deskprotocol.Surface) map[string]bool {
	out := map[string]bool{}
	for _, lit := range lock.Names["yaegikernel.Intent*"].Values {
		out[strings.Trim(lit, `"`)] = true
	}
	return out
}

// LoadCurrentLock loads the lock for deskprotocol.Version from the repository
// the tests run in.
func LoadCurrentLock(root string) (*deskprotocol.Surface, error) {
	locks, err := deskprotocol.LoadLocks(root + "/" + deskprotocol.LockDir)
	if err != nil {
		return nil, err
	}
	for _, l := range locks {
		if l.Version == deskprotocol.Version {
			return l, nil
		}
	}
	return nil, fmt.Errorf("no lock for %s", deskprotocol.Version)
}
