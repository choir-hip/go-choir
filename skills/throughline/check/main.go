// Command goalfilecheck checks throughline goal files (docs/definitions/*.md).
// Check A: frontmatter parses as YAML, for every file with a definition_version
// (any value), except the files in parseBaseline.
// Check B: the version 7 schema (skills/throughline/references/mission-schema.md),
// for integer definition_version >= 7.
// Usage: goalfilecheck [DIR ...]   (default docs/definitions)
// Prints "path: message" per failure and exits 1, or a one-line summary.
package main

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	yaml "gopkg.in/yaml.v2"
)

type m = map[interface{}]interface{}

var versionRe = regexp.MustCompile(`(?m)^\W*definition_version\s*:`)
var dateRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}`)

// parseBaseline lists goal files whose frontmatter did not parse when check A
// was added (2026-11-12). Entries may only be removed, never added: a new
// unparseable file must be fixed. TestBaselineShrinks fails once an entry parses.
var parseBaseline = map[string]bool{
	"choir-launch-fp10-net-05-2026-10-21.md":              true,
	"choir-launch-fp10-res-08-2026-10-21.md":              true,
	"choir-launch-fp4-selfdev-architecture-2026-10-21.md": true,
	"choir-launch-fp6-network-2026-10-21.md":              true,
	"choir-selfdev-apps-feature-complete-2026-11-03.md":   true,
	"choir-store-header-body-split-2026-11-02.md":         true,
	"ci-test-cooptimization-2026-10-24.md":                true,
}

var classRe = regexp.MustCompile(`green|yellow|orange|red|black`)

// frontmatter returns the text between the first two --- lines.
func frontmatter(src string) (string, bool) {
	lines := strings.Split(src, "\n")
	if len(lines) == 0 || strings.TrimRight(lines[0], " \r") != "---" {
		return "", false
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimRight(lines[i], " \r") == "---" {
			return strings.Join(lines[1:i], "\n"), true
		}
	}
	return "", false
}

func checkFile(path string, src string) []string {
	fm, ok := frontmatter(src)
	if !ok {
		return nil
	}
	vm := versionRe.FindStringSubmatch(fm)
	if vm == nil {
		return nil // not a goal file
	}
	var raw interface{}
	if err := yaml.Unmarshal([]byte(fm), &raw); err != nil {
		if parseBaseline[filepath.Base(path)] {
			return nil
		}
		return []string{path + ": frontmatter does not parse as YAML: " + strings.ReplaceAll(err.Error(), "\n", " ")}
	}
	var out []string
	fail := func(f string, a ...interface{}) { out = append(out, path+": "+fmt.Sprintf(f, a...)) }
	doc, isMap := raw.(m)
	if !isMap {
		fail("frontmatter is not a mapping")
		return out
	}
	switch versionKind(doc["definition_version"]) {
	case verOld, verOther:
		return nil
	case verBad:
		fail("definition_version %q is a number at or above 7 but not an integer", str(doc["definition_version"]))
		return out
	}
	checkV7(doc, fail)
	return out
}

const (
	verOther = iota // not numeric (throughline/1): check A only
	verOld          // numeric below 7
	verNew          // integer >= 7
	verBad          // numeric >= 7 but not an integer
)

// versionKind classifies the parsed definition_version: an integer, an
// integral float or a quoted integer is a number; anything else numeric at or
// above 7 is verBad.
func versionKind(v interface{}) int {
	var f float64
	switch x := v.(type) {
	case int:
		f = float64(x)
	case int64:
		f = float64(x)
	case uint64:
		f = float64(x)
	case float64:
		f = x
	case string:
		g, err := strconv.ParseFloat(strings.TrimSpace(x), 64)
		if err != nil {
			return verOther
		}
		f = g
	default:
		return verOther
	}
	switch {
	case f < 7:
		return verOld
	case f != math.Trunc(f):
		return verBad
	}
	return verNew
}

func get(x interface{}, k string) interface{} {
	if mm, ok := x.(m); ok {
		if v, ok := mm[k]; ok {
			return v
		}
		if k == "on" { // YAML 1.1 reads a bare on as true
			return mm[true]
		}
	}
	return nil
}

func str(x interface{}) string {
	switch v := x.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(v)
	case time.Time:
		return v.Format("2006-01-02")
	default:
		return strings.TrimSpace(fmt.Sprint(v))
	}
}

func nonEmpty(x interface{}) bool {
	switch v := x.(type) {
	case nil:
		return false
	case []interface{}:
		return len(v) > 0
	case m:
		return len(v) > 0
	}
	return str(x) != ""
}

func isDate(x interface{}) bool {
	if _, ok := x.(time.Time); ok {
		return true
	}
	return dateRe.MatchString(str(x))
}

func list(x interface{}) []interface{} {
	l, _ := x.([]interface{})
	return l
}

func checkV7(doc m, fail func(string, ...interface{})) {
	for _, k := range []string{"readiness", "start", "finish", "value", "boundaries", "now"} {
		if !nonEmpty(doc[k]) {
			fail("missing required key %q", k)
		}
	}
	// A present block must have the right type; later checks then read it.
	for _, k := range []string{"start", "finish", "value", "boundaries", "now"} {
		if v := doc[k]; nonEmpty(v) {
			if _, ok := v.(m); !ok {
				fail("%s must be a mapping", k)
				doc[k] = nil
			}
		}
	}
	rd := str(doc["readiness"])
	switch rd {
	case "", "intent", "drafted", "reviewed", "executable":
	default:
		fail("readiness %q is not one of intent, drafted, reviewed, executable", rd)
	}
	for _, k := range []string{"better_means", "goodharting_would_be"} {
		if !nonEmpty(get(doc["value"], k)) && doc["value"] != nil {
			fail("value.%s is missing or empty", k)
		}
	}
	accRaw := get(doc["finish"], "acceptance")
	acc, accOK := accRaw.([]interface{})
	if doc["finish"] != nil && (!accOK || len(acc) == 0) {
		fail("finish.acceptance must be a non-empty list")
	}
	for i, a := range acc {
		if _, ok := a.(m); !ok {
			fail("finish.acceptance[%d] must be a mapping", i)
			continue
		}
		for _, k := range []string{"action", "proves", "evidence_class"} {
			if !nonEmpty(get(a, k)) {
				fail("finish.acceptance[%d].%s is missing or empty", i, k)
			}
		}
	}

	now := doc["now"]
	status := str(get(now, "status"))
	valid := map[string]bool{"working": true, "observing": true, "complete": true,
		"checkpoint_incomplete": true, "blocked_incomplete": true, "superseded": true}
	if now != nil && !valid[status] {
		fail("now.status %q is not one of working, observing, complete, checkpoint_incomplete, blocked_incomplete, superseded", status)
	}
	obs := get(now, "observation")
	if status == "observing" {
		if _, ok := obs.(m); !ok || !nonEmpty(obs) {
			fail("now.status observing requires now.observation as a mapping")
		} else {
			for _, k := range []string{"readings", "reader", "window_ends", "pass", "tripwire"} {
				if !nonEmpty(get(obs, k)) {
					fail("now.observation.%s is missing or empty", k)
				}
			}
			if w := get(obs, "window_ends"); nonEmpty(w) && !isDate(w) {
				fail("now.observation.window_ends %q is not a date", str(w))
			}
		}
	} else if obs != nil {
		fail("now.status %q must not carry now.observation", status)
	}
	if status == "blocked_incomplete" {
		b := str(get(now, "blocker_or_risk"))
		if b == "" || strings.EqualFold(b, "none") {
			fail("now.status blocked_incomplete requires a real now.blocker_or_risk (not empty or none)")
		}
	}
	if rd == "executable" && now != nil && status != "working" && status != "observing" {
		fail("readiness executable requires now.status working or observing, got %q", status)
	}

	// items checks a present key: it must be a list of mappings.
	items := func(key string, fn func(i int, it m)) {
		v, ok := doc[key]
		if !ok {
			return
		}
		l, isList := v.([]interface{})
		if !isList {
			fail("%s must be a list", key)
			return
		}
		for i, e := range l {
			it, ok := e.(m)
			if !ok {
				fail("%s[%d] must be a mapping", key, i)
				continue
			}
			fn(i, it)
		}
	}
	need := func(key string, i int, it m, ks ...string) {
		for _, k := range ks {
			if !nonEmpty(it[k]) {
				fail("%s[%d].%s is missing or empty", key, i, k)
			}
		}
	}
	okStatus := map[string]bool{"pending": true, "working": true, "landed": true, "proved": true, "superseded": true}
	items("stations", func(i int, s m) {
		need("stations", i, s, "id", "name", "class", "status")
		if c := str(s["class"]); c != "" && !classRe.MatchString(c) {
			fail("stations[%d].class %q names no mutation class", i, c)
		}
		if v := str(s["status"]); v != "" && !okStatus[v] {
			fail("stations[%d].status %q is not pending|working|landed|proved|superseded", i, v)
		}
	})
	items("decisions_for_the_owner", func(i int, s m) {
		need("decisions_for_the_owner", i, s, "id", "question", "default")
	})
	items("owner_rulings", func(i int, s m) {
		need("owner_rulings", i, s, "decision", "said")
		o := get(s, "on")
		if !nonEmpty(o) {
			fail("owner_rulings[%d].on is missing or empty", i)
		} else if !isDate(o) {
			fail("owner_rulings[%d].on %q is not a date", i, str(o))
		}
	})
	if c := get(now, "conjecture"); c != nil {
		if _, ok := c.(m); !ok {
			fail("now.conjecture must be a mapping")
		} else {
			for _, k := range []string{"claim", "test", "edge", "status"} {
				if !nonEmpty(get(c, k)) {
					fail("now.conjecture.%s is missing or empty", k)
				}
			}
			switch e := str(get(c, "edge")); e {
			case "", "independence", "resource", "missing_oracle", "frame_lock":
			default:
				fail("now.conjecture.edge %q is not independence|resource|missing_oracle|frame_lock", e)
			}
		}
	}
}

func run(dirs []string) ([]string, int) {
	var fails []string
	n := 0
	for _, d := range dirs {
		files, _ := filepath.Glob(filepath.Join(d, "*.md"))
		sort.Strings(files)
		for _, f := range files {
			b, err := os.ReadFile(f)
			if err != nil {
				fails = append(fails, f+": "+err.Error())
				continue
			}
			n++
			fails = append(fails, checkFile(f, string(b))...)
		}
	}
	return fails, n
}

func main() {
	dirs := os.Args[1:]
	if len(dirs) == 0 {
		dirs = []string{"docs/definitions"}
	}
	fails, n := run(dirs)
	if len(fails) > 0 {
		fmt.Println(strings.Join(fails, "\n"))
		os.Exit(1)
	}
	fmt.Printf("check-goal-files: %d files scanned, all goal files pass\n", n)
}
