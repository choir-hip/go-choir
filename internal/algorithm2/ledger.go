// Package algorithm2 is the behavior ledger shown to its owner and the second
// prompt that proposes edits to the algorithm document
// (docs/design/autopaper-next-mission-handoff-2026-11-09.md, 4.H).
//
// The ledger is private to the owner's computer and is evidence for algorithm2
// only. It is derived on read, so it adds no state of its own. Algorithm2 is a
// document the owner edits by hand; there is no algorithm3.
package algorithm2

import (
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

// Kind is what an owner did that the ledger remembers.
type Kind string

const (
	// KindResponse is a comment or instruction the owner gave on a document, or
	// a merge of a published version under an instruction.
	KindResponse Kind = "response"
	// KindFork is a document the owner forked from a published version.
	KindFork Kind = "fork"
	// KindPublication is a document the owner published.
	KindPublication Kind = "publication"
	// KindCitation is a published version the owner took or cited.
	KindCitation Kind = "citation"
)

var kindOrder = []Kind{KindResponse, KindFork, KindPublication, KindCitation}

var kindHeading = map[Kind]string{
	KindResponse: "Responses", KindFork: "Forks", KindPublication: "Publications", KindCitation: "Citations",
}

const (
	// MaxLedgerEntries bounds one outline (and so one prompt).
	MaxLedgerEntries = 200
	fieldMaxRunes    = 120
	listMaxItems     = 8
)

// Entry is one remembered act, with the categories, entities and authors it
// touched.
type Entry struct {
	Kind       Kind
	At         time.Time
	Ref        string
	Title      string
	Categories []string
	Entities   []string
	Authors    []string
}

// clean makes ledger text one line of plain text. Titles and author labels
// come from other people's documents: they are data, so they must not be able
// to start a heading or a list line in the outline.
func clean(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if utf8.RuneCountInString(s) > fieldMaxRunes {
		s = string([]rune(s)[:fieldMaxRunes]) + "..."
	}
	return strings.TrimLeft(s, "#-*>+ ")
}

func cleanList(in []string) string {
	seen := map[string]bool{}
	var out []string
	for _, v := range in {
		v = clean(v)
		if v == "" || seen[strings.ToLower(v)] {
			continue
		}
		seen[strings.ToLower(v)] = true
		out = append(out, v)
		if len(out) == listMaxItems {
			break
		}
	}
	return strings.Join(out, ", ")
}

// RenderOutline is the ledger as text the owner can read: newest first within
// each kind, bounded to MaxLedgerEntries, with category counts.
func RenderOutline(entries []Entry) string {
	sorted := append([]Entry(nil), entries...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].At.After(sorted[j].At) })
	hidden := 0
	if len(sorted) > MaxLedgerEntries {
		hidden = len(sorted) - MaxLedgerEntries
		sorted = sorted[:MaxLedgerEntries]
	}
	var b strings.Builder
	b.WriteString("Behavior ledger (private to this computer; evidence for algorithm2 only)\n")
	fmt.Fprintf(&b, "%d entries shown, newest first.\n", len(sorted))
	if hidden > 0 {
		fmt.Fprintf(&b, "%d older entries are not shown.\n", hidden)
	}
	counts := map[string]int{}
	for _, e := range sorted {
		for _, c := range e.Categories {
			if c = clean(c); c != "" {
				counts[strings.ToLower(c)]++
			}
		}
	}
	if len(counts) > 0 {
		names := make([]string, 0, len(counts))
		for n := range counts {
			names = append(names, n)
		}
		sort.Slice(names, func(i, j int) bool {
			if counts[names[i]] != counts[names[j]] {
				return counts[names[i]] > counts[names[j]]
			}
			return names[i] < names[j]
		})
		b.WriteString("\nCategories by count:")
		for i, n := range names {
			if i == 12 {
				break
			}
			fmt.Fprintf(&b, " %s (%d);", n, counts[n])
		}
		b.WriteString("\n")
	}
	for _, kind := range kindOrder {
		var group []Entry
		for _, e := range sorted {
			if e.Kind == kind {
				group = append(group, e)
			}
		}
		if len(group) == 0 {
			continue
		}
		fmt.Fprintf(&b, "\n%s (%d)\n", kindHeading[kind], len(group))
		for _, e := range group {
			fmt.Fprintf(&b, "  * %s  %s", e.At.UTC().Format("2006-01-02"), quoteTitle(e.Title))
			if v := cleanList(e.Categories); v != "" {
				fmt.Fprintf(&b, "  categories: %s;", v)
			}
			if v := cleanList(e.Entities); v != "" {
				fmt.Fprintf(&b, "  entities: %s;", v)
			}
			if v := cleanList(e.Authors); v != "" {
				fmt.Fprintf(&b, "  authors: %s;", v)
			}
			b.WriteString("\n")
		}
	}
	return b.String()
}

func quoteTitle(t string) string {
	t = clean(t)
	if t == "" {
		return "(untitled)"
	}
	return `"` + strings.ReplaceAll(t, `"`, "'") + `"`
}
