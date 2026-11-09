package yaegikernel

import (
	"fmt"
	"reflect"
	"regexp"
	"sort"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
)

// Progressive disclosure of the verb surface
// (docs/evidence/token-efficiency-2026-11-09/followup/progressive-disclosure.md).
// A desk's prompt carries a hot card for the verbs that desk calls in most
// runs (signature, result fields, one literal example) and one line naming
// the cold verbs; choir.Help("Name") returns the rest as an ordinary tool
// result, so the cached prefix never changes mid-run. The block is a pure
// function of (desk, slot, readOnly, hidden): the hot order is the list
// below, the cold names are sorted, so the same bytes render every run.

// deskHotVerbs names, per desk, the verbs shown as full cards, in card
// order. A verb the desk cannot call is skipped. The sets come from the
// verb frequency of the paper's and d@e's traces (at least half the runs, or
// a verb whose Help models already ask for). A desk with no entry (an app
// desk) keeps the plain list of every verb.
func deskHotVerbs(desk string) []string {
	switch desk {
	case deskmanifest.ResearcherName():
		return []string{"StoryAnalysis", "Records", "SearchWireCorpus", "SearchStart", "Searches", "SearchWait", "Read", "ReadCapture", "Captures", "Relate", "ReportPacket"}
	case deskmanifest.AdmitterName():
		return []string{"Records", "Pack", "Verify", "VerifyFindings", "Screens", "Ask", "Cast", "Updates", "ProductAPI", "ShotStart", "ReportPacket", "Context"}
	case deskmanifest.PerformerName():
		return []string{"Exec", "Start", "Poll", "ReadFile", "WriteFile", "ListDir", "Edit", "Freeze", "Complete", "Precommit", "Records", "Context", "InspectBundle"}
	case deskmanifest.DocumentCustodianName():
		return []string{"Write", "ReadCapture", "Done", "Wait", "Cast", "ReadDoc"}
	case deskmanifest.TaskRoleName():
		return []string{"Done", "Records", "Stories", "ReadCapture", "Cast", "Feed", "Decline"}
	}
	return nil
}

// splitHotCold partitions names (sorted) into the desk's hot verbs, in card
// order, and the rest, sorted. A desk with no hot list is all hot.
func splitHotCold(desk string, names []string) (hot, cold []string) {
	have := make(map[string]bool, len(names))
	for _, n := range names {
		have[n] = true
	}
	list := deskHotVerbs(desk)
	if list == nil {
		return append([]string(nil), names...), nil
	}
	isHot := map[string]bool{}
	for _, n := range list {
		if have[n] && !isHot[n] {
			isHot[n] = true
			hot = append(hot, n)
		}
	}
	for _, n := range names {
		if !isHot[n] {
			cold = append(cold, n)
		}
	}
	sort.Strings(cold)
	return hot, cold
}

// exampleFor is the verb's one worked call.
func exampleFor(name string) string {
	if ex := choirFunctionDocs[name].Example; ex != "" {
		return ex
	}
	return choirExamples[name]
}

// resultFieldsFor is the verb's result fields: from the Go type when it is a
// struct, else the JSON shape the verb documents.
func resultFieldsFor(name string) string {
	if f := returnFields(name); f != "" {
		return f
	}
	if f := choirFields[name]; f != "" {
		return "JSON bytes with " + f
	}
	return ""
}

var jsonBytesType = reflect.TypeOf(JSON(nil))

// returnsJSON reports a verb whose first result is JSON bytes.
func returnsJSON(name string) bool {
	m, ok := choirScopeType.MethodByName(name)
	return ok && m.Type.NumOut() > 0 && m.Type.Out(0) == jsonBytesType
}

// callShape names how to bind a verb's results, from its signature, so a
// wrong result count is answered by the signature itself.
func callShape(name string) string {
	if _, ok := choirNamespaces[name]; ok {
		return ""
	}
	m, ok := choirScopeType.MethodByName(name)
	if !ok {
		return ""
	}
	ft := m.Type
	args := make([]string, 0, ft.NumIn()-1)
	for i := 1; i < ft.NumIn(); i++ {
		args = append(args, "...")
	}
	call := "choir." + name + "(" + strings.Join(args, ", ") + ")"
	hasErr := ft.NumOut() > 0 && ft.Out(ft.NumOut()-1).String() == "error"
	switch {
	case ft.NumOut() == 0:
		return call + " returns nothing: call it as a statement."
	case ft.NumOut() == 1 && hasErr:
		return call + " returns only an error: err := " + call + "."
	case ft.NumOut() == 1:
		return call + " returns one value and no error: v := " + call + " (not v, err :=)."
	case hasErr && ft.NumOut() == 2:
		return call + " returns a value and an error: v, err := " + call + "."
	}
	return call + " returns " + fmt.Sprint(ft.NumOut()) + " values."
}

// reqShort stands for reqDoc on a card once the card block has stated it.
const reqShort = `Required last argument req (the map described above).`

// renderCard renders one hot verb: signature and summary, result fields, example.
func renderCard(name string, shortReq bool) string {
	doc := choirFunctionDocs[name]
	summary := doc.Summary
	if shortReq {
		summary = strings.ReplaceAll(summary, reqDoc, reqShort)
	}
	var b strings.Builder
	b.WriteString("- " + choirSignature(name))
	if summary != "" {
		b.WriteString(" — " + summary)
	}
	b.WriteString("\n")
	if f := resultFieldsFor(name); f != "" {
		b.WriteString("  Result: " + strings.ReplaceAll(f, "\n", "\n  ") + "\n")
	}
	if ex := exampleFor(name); ex != "" {
		b.WriteString("  Example: " + strings.ReplaceAll(ex, "\n", "\n    ") + "\n")
	}
	return b.String()
}

// renderCards renders the hot cards, stating the shared req map once when a
// card needs it.
func renderCards(hot []string) string {
	needsReq := false
	for _, n := range hot {
		if strings.Contains(choirFunctionDocs[n].Summary, reqDoc) {
			needsReq = true
		}
	}
	var b strings.Builder
	if needsReq {
		b.WriteString("req, for the verbs that take it: " + reqDoc + "\n")
	}
	for _, n := range hot {
		b.WriteString(renderCard(n, needsReq))
	}
	return b.String()
}

// DeskSurface is the generated prompt block naming a desk's surface: hot
// cards, then the cold verbs by name. Hidden names are left out of the block
// (a setting that is off).
func DeskSurface(desk, slot string, readOnly bool, hidden ...string) string {
	names := SurfaceNames(desk, slot, readOnly, hidden...)
	if deskHotVerbs(desk) == nil {
		// A desk with no measured hot set (an app desk: a handful of verbs)
		// keeps the plain list; cards would only add bytes.
		return "Your REPL interaction surface. The package choir is predeclared; always call its functions with the choir. prefix. These are the only choir functions this desk can call:\n" + renderSurface(names)
	}
	hot, cold := splitHotCold(desk, names)
	var b strings.Builder
	b.WriteString("Your REPL interaction surface. The package choir is predeclared; always call its functions with the choir. prefix. These are the only choir functions this desk can call. The verbs this desk uses most come first, each with its signature, result fields and an example.\n\n")
	b.WriteString(renderCards(hot))
	if len(cold) > 0 {
		b.WriteString("\nAlso available, by name: " + strings.Join(cold, ", ") + `. Call choir.Help("Name") for one function's signature, result fields, an example and the usual mistake (several at once: choir.Help("Cast", "Feed")).` + "\n")
	}
	b.WriteString(`Call choir.Help() for the list of names and signatures at any time.`)
	return b.String()
}

// SurfaceNames lists the verbs a desk's prompt names (sorted), with the
// hidden ones left out.
func SurfaceNames(desk, slot string, readOnly bool, hidden ...string) []string {
	scope := &ChoirScope{desk: desk, slot: slot, readOnly: readOnly}
	skip := map[string]bool{}
	for _, name := range hidden {
		skip[name] = true
	}
	var names []string
	for _, name := range scope.surfaceNames() {
		if !skip[name] {
			names = append(names, name)
		}
	}
	return names
}

// renderIndex is Help() with no argument: one signature per line, no prose,
// the hot verbs marked.
func (s *ChoirScope) renderIndex(names []string) string {
	hot, _ := splitHotCold(s.desk, names)
	isHot := map[string]bool{}
	for _, n := range hot {
		isHot[n] = true
	}
	var b strings.Builder
	b.WriteString("Your choir functions (always call them with the choir. prefix")
	if len(hot) < len(names) {
		b.WriteString("; * marks the ones on your prompt's cards")
	}
	b.WriteString("):\n")
	for _, n := range names {
		mark := "  "
		if isHot[n] && len(hot) < len(names) {
			mark = "* "
		}
		b.WriteString(mark + choirSignature(n) + "\n")
	}
	b.WriteString(`choir.Help("Name") gives one function's signature, result fields, an example and the usual mistake; choir.Help("Cast", "Feed") gives several; choir.Help("*") gives every function in full.`)
	return b.String()
}

// helpDetail is Help("Name") for one callable verb.
func helpDetail(name string) string {
	doc := choirFunctionDocs[name]
	var b strings.Builder
	b.WriteString(choirSignature(name) + "\n" + doc.Summary + "\n")
	if f := resultFieldsFor(name); f != "" {
		b.WriteString("Result: " + f + "\n")
	}
	if ex := exampleFor(name); ex != "" {
		b.WriteString("Example: " + ex + "\n")
	}
	var mistakes []string
	if shape := callShape(name); shape != "" {
		mistakes = append(mistakes, "a wrong result count is \"assignment mismatch\": "+shape)
	}
	if returnsJSON(name) {
		mistakes = append(mistakes, "the result is JSON bytes, not a map or struct: fmt.Println(string(r)), or json.Unmarshal(r, &v)")
	}
	if p := choirPitfalls[name]; p != "" {
		mistakes = append(mistakes, p)
	}
	if len(mistakes) > 0 {
		b.WriteString("Common error and fix: " + strings.Join(mistakes, " Also: ") + "\n")
	}
	return b.String()
}

func cleanHelpName(raw string) string {
	wanted := strings.TrimPrefix(strings.TrimSpace(raw), "choir.")
	return strings.TrimSuffix(wanted, "()")
}

var verbCallRE = regexp.MustCompile(`\bchoir\.([A-Za-z_]\w*)\(`)
