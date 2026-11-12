package main

import (
	yaml "gopkg.in/yaml.v2"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const good = `definition_version: 7
readiness: executable
start:
  captured_at: 2026-11-12
finish:
  acceptance:
    - action: read it
      proves: it works
      evidence_class: deployed
value:
  better_means: more
  goodharting_would_be: fewer
boundaries:
  authority: x
stations:
  - {id: S0, name: first, class: yellow, status: pending}
decisions_for_the_owner:
  - {id: D1, question: q, default: d}
owner_rulings:
  - {on: 2026-11-12, decision: d, said: s}
now:
  status: working
  conjecture: {claim: c, test: t, edge: resource, status: proposed}
`

func doc(fm string) string { return "---\n" + fm + "---\n\n# Body\n" }

// mutate replaces old with new in good (old must exist).
func mutate(t *testing.T, old, new string) string {
	t.Helper()
	if !strings.Contains(good, old) {
		t.Fatalf("fixture lacks %q", old)
	}
	return strings.Replace(good, old, new, 1)
}

func TestCases(t *testing.T) {
	obs := "  status: observing\n  observation:\n    readings: r\n    reader: me\n    window_ends: 2026-12-01\n    pass: p\n    tripwire: t\n"
	cases := []struct {
		name, fm, want string // want "" means pass
	}{
		{"v7 good", good, ""},
		{"v4 unparseable", "definition_version: 4\nfoo: [unclosed\n", "does not parse"},
		{"v4 tab-indented", "definition_version: 4\nstart:\n\tx: 1\n", "does not parse"},
		{"v3 unparseable fails", "definition_version: 3\nfoo: [unclosed\n", "does not parse"},
		{"string version unparseable fails", "definition_version: throughline/1\nfoo: [unclosed\n", "does not parse"},
		{"version with comment checked", "definition_version: 7 # note\nreadiness: executable\n", "missing required key"},
		{"version 7.0 checked", "definition_version: 7.0\nreadiness: executable\n", "missing required key"},
		{"quoted version checked", "definition_version: \"7\"\nreadiness: executable\n", "missing required key"},
		{"version 7.5 fails", "definition_version: 7.5\n", "not an integer"},
		{"version 8 checked", "definition_version: 8\nreadiness: executable\n", "missing required key"},
		{"frontmatter not a mapping", "- definition_version: 7\n", "not a mapping"},
		{"bad readiness", mutate(t, "readiness: executable", "readiness: ready"), "readiness"},
		{"stations a string", mutate(t, "stations:\n  - {id: S0, name: first, class: yellow, status: pending}", "stations: none"), "stations must be a list"},
		{"stations a mapping", mutate(t, "stations:\n  - {id: S0, name: first, class: yellow, status: pending}", "stations: {id: S0}"), "stations must be a list"},
		{"decisions a string", mutate(t, "decisions_for_the_owner:\n  - {id: D1, question: q, default: d}", "decisions_for_the_owner: later"), "decisions_for_the_owner must be a list"},
		{"rulings a mapping", mutate(t, "owner_rulings:\n  - {on: 2026-11-12, decision: d, said: s}", "owner_rulings: {on: x}"), "owner_rulings must be a list"},
		{"station item a string", mutate(t, "  - {id: S0, name: first, class: yellow, status: pending}", "  - S0"), "stations[0] must be a mapping"},
		{"finish a string", mutate(t, "finish:\n  acceptance:\n    - action: read it\n      proves: it works\n      evidence_class: deployed", "finish: done"), "finish must be a mapping"},
		{"now a list", mutate(t, "now:\n  status: working\n  conjecture: {claim: c, test: t, edge: resource, status: proposed}", "now: [working]"), "now must be a mapping"},
		{"conjecture a string", mutate(t, "conjecture: {claim: c, test: t, edge: resource, status: proposed}", "conjecture: maybe"), "now.conjecture must be a mapping"},
		{"v5 schema not enforced", "definition_version: 5\nreadiness: x\n", ""},
		{"no version ignored", "title: [unclosed\n", ""},
		{"missing boundaries", strings.Replace(good, "boundaries:\n  authority: x\n", "", 1), `missing required key "boundaries"`},
		{"empty better_means", mutate(t, "better_means: more", `better_means: ""`), "value.better_means"},
		{"empty goodharting", mutate(t, "goodharting_would_be: fewer", "goodharting_would_be:"), "value.goodharting_would_be"},
		{"no acceptance item field", mutate(t, "      proves: it works\n", ""), "acceptance[0].proves"},
		{"empty acceptance", strings.Replace(good, "  acceptance:\n    - action: read it\n      proves: it works\n      evidence_class: deployed\n", "  acceptance: []\n", 1), "finish.acceptance"},
		{"bad status", mutate(t, "status: working\n  conj", "status: busy\n  conj"), "now.status"},
		{"observing without observation", mutate(t, "status: working\n  conj", "status: observing\n  conj"), "requires now.observation"},
		{"observing good", mutate(t, "  status: working\n", obs), ""},
		{"observing bad date", mutate(t, "  status: working\n", strings.Replace(obs, "2026-12-01", "soon", 1)), "not a date"},
		{"observing missing tripwire", mutate(t, "  status: working\n", strings.Replace(obs, "    tripwire: t\n", "", 1)), "tripwire"},
		{"working with observation", mutate(t, "  status: working\n", "  status: working\n  observation: {readings: r}\n"), "must not carry now.observation"},
		{"blocked none", strings.Replace(mutate(t, "readiness: executable", "readiness: drafted"), "status: working\n  conj", "status: blocked_incomplete\n  blocker_or_risk: none\n  conj", 1), "blocked_incomplete requires"},
		{"blocked real", strings.Replace(mutate(t, "readiness: executable", "readiness: drafted"), "status: working\n  conj", "status: blocked_incomplete\n  blocker_or_risk: waiting on x\n  conj", 1), ""},
		{"executable but complete", mutate(t, "status: working\n  conj", "status: complete\n  conj"), "readiness executable requires"},
		{"station bad status", mutate(t, "status: pending", "status: done"), "stations[0].status"},
		{"station bad class", mutate(t, "class: yellow", "class: purple"), "stations[0].class"},
		{"station missing name", mutate(t, "name: first, ", ""), "stations[0].name"},
		{"decision missing default", mutate(t, ", default: d", ""), "decisions_for_the_owner[0].default"},
		{"ruling bad date", mutate(t, "on: 2026-11-12", "on: yesterday"), "owner_rulings[0].on"},
		{"ruling missing said", mutate(t, ", said: s", ""), "owner_rulings[0].said"},
		{"conjecture bad edge", mutate(t, "edge: resource", "edge: vibes"), "now.conjecture.edge"},
		{"conjecture missing status", mutate(t, ", status: proposed", ""), "now.conjecture.status"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "x-2026-11-12.md"), []byte(doc(strings.TrimSuffix(c.fm, "\n")+"\n")), 0o644); err != nil {
				t.Fatal(err)
			}
			fails, n := run([]string{dir})
			if n != 1 {
				t.Fatalf("scanned %d", n)
			}
			joined := strings.Join(fails, "\n")
			if c.want == "" && joined != "" {
				t.Fatalf("expected pass, got:\n%s", joined)
			}
			if c.want != "" && !strings.Contains(joined, c.want) {
				t.Fatalf("want %q in:\n%s", c.want, joined)
			}
			if c.want != "" && !strings.HasPrefix(joined, filepath.Join(dir, "x-2026-11-12.md")+": ") {
				t.Fatalf("line is not path: message: %s", joined)
			}
		})
	}
}

func TestBaselinedFileIsSkipped(t *testing.T) {
	dir := t.TempDir()
	parseBaseline["zz-baselined.md"] = true
	defer delete(parseBaseline, "zz-baselined.md")
	os.WriteFile(filepath.Join(dir, "zz-baselined.md"), []byte(doc("definition_version: 4\nfoo: [unclosed\n")), 0o644)
	os.WriteFile(filepath.Join(dir, "zz-other.md"), []byte(doc("definition_version: 4\nfoo: [unclosed\n")), 0o644)
	fails, _ := run([]string{dir})
	if len(fails) != 1 || !strings.Contains(fails[0], "zz-other.md") {
		t.Fatalf("want only the non-baselined file to fail, got %v", fails)
	}
}

// TestBaselineShrinks fails when a baselined file now parses (or is gone):
// remove its entry from parseBaseline.
func TestBaselineShrinks(t *testing.T) {
	for name := range parseBaseline {
		b, err := os.ReadFile(filepath.Join("..", "..", "..", "docs", "definitions", name))
		if err != nil {
			t.Errorf("%s: baselined file unreadable (%v): remove it from parseBaseline", name, err)
			continue
		}
		fm, _ := frontmatter(string(b))
		var d m
		if yaml.Unmarshal([]byte(fm), &d) == nil {
			t.Errorf("%s: now parses: remove it from parseBaseline", name)
		}
	}
}
