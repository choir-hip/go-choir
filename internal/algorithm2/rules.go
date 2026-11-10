package algorithm2

import (
	"strings"
)

// Op is what a change does to a section of the algorithm document.
type Op string

const (
	OpAdd    Op = "addition"
	OpRemove Op = "removal"
	OpChange Op = "change"
	OpAny    Op = ""
)

// SectionChange is one section's change, computed from the texts alone.
type SectionChange struct {
	Section string `json:"section"`
	Added   int    `json:"added"`
	Removed int    `json:"removed"`
	Op      Op     `json:"op"`
}

const maxDiffLines = 2000

type line struct {
	text    string
	section string
}

// sectioned splits text into its non-blank lines, each tagged with the
// markdown heading it falls under.
func sectioned(text string) []line {
	var out []line
	section := ""
	for _, raw := range strings.Split(text, "\n") {
		t := strings.TrimSpace(raw)
		if t == "" {
			continue
		}
		if strings.HasPrefix(t, "#") {
			section = strings.TrimSpace(strings.TrimLeft(t, "#"))
		}
		out = append(out, line{text: t, section: section})
	}
	return out
}

// DiffSections compares two texts line by line and reports, per section, how
// many lines were added and removed. A section with only additions is an
// addition, only removals a removal, both a change. A text longer than the
// diff bound reports one whole-document change, which no rule auto-accepts.
func DiffSections(current, proposed string) []SectionChange {
	a, b := sectioned(current), sectioned(proposed)
	if len(a) > maxDiffLines || len(b) > maxDiffLines {
		return []SectionChange{{Section: "(whole document)", Added: len(b), Removed: len(a), Op: OpChange}}
	}
	n, m := len(a), len(b)
	lcs := make([][]int32, n+1)
	for i := range lcs {
		lcs[i] = make([]int32, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if a[i].text == b[j].text {
				lcs[i][j] = lcs[i+1][j+1] + 1
			} else {
				lcs[i][j] = max(lcs[i+1][j], lcs[i][j+1])
			}
		}
	}
	type tally struct{ added, removed int }
	by := map[string]*tally{}
	var order []string
	touch := func(s string) *tally {
		if by[s] == nil {
			by[s] = &tally{}
			order = append(order, s)
		}
		return by[s]
	}
	i, j := 0, 0
	for i < n && j < m {
		switch {
		case a[i].text == b[j].text:
			i++
			j++
		case lcs[i+1][j] >= lcs[i][j+1]:
			touch(a[i].section).removed++
			i++
		default:
			touch(b[j].section).added++
			j++
		}
	}
	for ; i < n; i++ {
		touch(a[i].section).removed++
	}
	for ; j < m; j++ {
		touch(b[j].section).added++
	}
	var out []SectionChange
	for _, s := range order {
		t := by[s]
		c := SectionChange{Section: s, Added: t.added, Removed: t.removed}
		switch {
		case t.added > 0 && t.removed == 0:
			c.Op = OpAdd
		case t.removed > 0 && t.added == 0:
			c.Op = OpRemove
		default:
			c.Op = OpChange
		}
		out = append(out, c)
	}
	return out
}

// Mode is what the owner wrote about a kind of change.
type Mode string

const (
	ModeAuto Mode = "auto"
	ModeAsk  Mode = "ask"
)

// Rule is one owner-written rule found in the algorithm2 document. An empty
// Area matches every section; OpAny matches every op.
type Rule struct {
	Mode Mode
	Area string
	Op   Op
}

// Decision is whether a proposal is applied at once or waits for the owner.
type Decision string

const (
	DecisionAuto Decision = "auto"
	DecisionAsk  Decision = "ask"
)

var (
	autoPrefixes = []string{"auto-accept", "auto accept", "autoaccept", "accept automatically"}
	askPrefixes  = []string{"ask before", "ask about", "ask on", "ask for", "ask first on", "never auto-accept", "do not auto-accept"}
	stopWords    = map[string]bool{"of": true, "to": true, "for": true, "any": true, "the": true, "all": true, "and": true, "or": true, "a": true, "an": true, "my": true, "in": true, "on": true}
)

// ParseRules reads the owner's rules from the algorithm2 text. A rule is a
// clause that starts with "auto-accept" or "ask before" (clauses split at
// commas, semicolons, periods and line ends). Anything else is prose and is
// ignored. Words after the prefix name the ops (additions, removals, changes)
// and the area (a section name); no area means every section.
func ParseRules(text string) []Rule {
	var rules []Rule
	clauses := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return r == ',' || r == ';' || r == '.' || r == '\n'
	})
	for _, clause := range clauses {
		clause = strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(clause), "-*>0123456789) "))
		mode, rest := Mode(""), ""
		for _, p := range autoPrefixes {
			if strings.HasPrefix(clause, p) {
				mode, rest = ModeAuto, strings.TrimPrefix(clause, p)
			}
		}
		for _, p := range askPrefixes {
			if strings.HasPrefix(clause, p) {
				mode, rest = ModeAsk, strings.TrimPrefix(clause, p)
			}
		}
		if mode == "" {
			continue
		}
		var ops []Op
		var areas []string
		for _, w := range strings.Fields(rest) {
			w = strings.Trim(w, `"'()`)
			switch {
			case strings.HasPrefix(w, "addition") || w == "add" || w == "adds":
				ops = append(ops, OpAdd)
			case strings.HasPrefix(w, "removal") || w == "remove" || w == "removes" || strings.HasPrefix(w, "deletion") || w == "delete":
				ops = append(ops, OpRemove)
			case strings.HasPrefix(w, "change") || strings.HasPrefix(w, "edit") || strings.HasPrefix(w, "rewrit") || strings.HasPrefix(w, "modif"):
				ops = append(ops, OpChange)
			case stopWords[w] || w == "":
			default:
				areas = append(areas, w)
			}
		}
		area := strings.Join(areas, " ")
		if len(ops) == 0 {
			ops = []Op{OpAny}
		}
		for _, op := range ops {
			rules = append(rules, Rule{Mode: mode, Area: area, Op: op})
		}
	}
	return rules
}

func stem(s string) string { return strings.TrimSuffix(strings.ToLower(strings.TrimSpace(s)), "s") }

func areaMatches(ruleArea, section string) bool {
	if ruleArea == "" {
		return true
	}
	rs, ss := stem(ruleArea), stem(section)
	return ss != "" && strings.Contains(ss, rs)
}

// touches is the ask-side match: a rule that asks before removals also asks
// for a change that removes a line.
func touches(op Op, c SectionChange) bool {
	switch op {
	case OpAdd:
		return c.Added > 0
	case OpRemove:
		return c.Removed > 0
	case OpChange:
		return c.Added > 0 && c.Removed > 0
	}
	return true
}

// Decide is the one place a proposal is judged for auto-acceptance. It reads
// the owner's rules and the computed section changes, never the model's words.
// Every changed section must be covered by an auto rule and by no ask rule.
func Decide(rules []Rule, changes []SectionChange) Decision {
	if len(changes) == 0 {
		return DecisionAsk
	}
	for _, c := range changes {
		covered := false
		for _, r := range rules {
			if r.Area != "" && !areaMatches(r.Area, c.Section) {
				continue
			}
			if r.Mode == ModeAsk && touches(r.Op, c) {
				return DecisionAsk
			}
			if r.Mode == ModeAuto && (r.Op == OpAny || r.Op == c.Op) {
				covered = true
			}
		}
		if !covered {
			return DecisionAsk
		}
	}
	return DecisionAuto
}
