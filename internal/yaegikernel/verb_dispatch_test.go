package yaegikernel

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
)

// The manifest derives each desk's frame table from the actions its verbs
// dispatch. This test reads the s.call(ActionX, ...) sites of the kernel's own
// source and requires the manifest's verb-to-action table to match them, so a
// verb that starts dispatching a new host action cannot be missed by the role
// check (failure mode F8: a real turn refused) or hide from it.
func TestVerbDispatchMatchesKernel(t *testing.T) {
	consts := map[string]BrokerAction{}
	for _, a := range allBrokerActions {
		consts["Action"+goName(a)] = a
	}
	found := map[string]BrokerAction{}
	fset := token.NewFileSet()
	for _, file := range []string{"choir.go", "search_store.go", "system1.go", "relate_stories.go", "jobs.go"} {
		f, err := parser.ParseFile(fset, file, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv == nil || fn.Body == nil || !fn.Name.IsExported() {
				continue
			}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok || sel.Sel.Name != "call" || len(call.Args) == 0 {
					return true
				}
				if id, ok := call.Args[0].(*ast.Ident); ok {
					if a, known := consts[id.Name]; known {
						if prev, dup := found[fn.Name.Name]; dup && prev != a {
							t.Errorf("verb %s dispatches both %s and %s", fn.Name.Name, prev, a)
						}
						found[fn.Name.Name] = a
					}
				}
				return true
			})
		}
	}
	// Every verb that dispatches an action is in the manifest with that action.
	for verb, action := range found {
		got, _, ok := deskmanifest.VerbAction(verb)
		if !ok {
			t.Errorf("verb %s dispatches %s but the manifest's verb table omits it", verb, action)
			continue
		}
		if got != string(action) {
			t.Errorf("verb %s dispatches %s, manifest says %s", verb, action, got)
		}
	}
	// Every verb in the manifest's table dispatches what it says.
	for _, verb := range []string{"ReadFile", "ListDir", "WriteFile", "Exec", "SearchStart", "Read", "PollSource", "SourceAPI",
		"ReadCapture", "SearchWireCorpus", "SaveEvidence", "ReadEvidence", "ListEvidence", "RunMemoryEntry",
		"ProductAPI", "Stories", "Research", "System1", "RelateStories"} {
		action, _, ok := deskmanifest.VerbAction(verb)
		if !ok {
			t.Errorf("manifest lost verb %s", verb)
			continue
		}
		if found[verb] == "" || string(found[verb]) != action {
			t.Errorf("manifest says verb %s dispatches %s; the kernel's source says %q", verb, action, found[verb])
		}
	}
}

// goName turns an action's wire name into the suffix of its Go constant
// (read_capture -> ReadCapture, get_run_memory_entry -> GetRunMemoryEntry).
func goName(a BrokerAction) string {
	out := []byte{}
	up := true
	for _, c := range []byte(a) {
		if c == '_' {
			up = true
			continue
		}
		if up && c >= 'a' && c <= 'z' {
			c -= 'a' - 'A'
		}
		up = false
		out = append(out, c)
	}
	res := string(out)
	// system1 -> System1 already; source_api -> SourceAPI needs the acronym.
	switch res {
	case "SourceApi":
		return "SourceAPI"
	case "ProductApi":
		return "ProductAPI"
	}
	return res
}
