package yaegikernel

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// docs/archive/problems/trace-review-desk-protocol-friction-2026-10-10.md F13: a desk
// did not know its own interaction surface. Texture spent ~19 of 20 turns in
// rerun 11 guessing choir names (PendingUpdates, Ledger, Poll, Help …) and
// its prompt taught a bare ReadDoc().
//
// Ways the surface fix can fail:
//  1. A function a desk can call has no doc, or a doc names a function no
//     desk can call (the surface drifts from the exports).
//  2. A doc's argument names or count differ from the Go method (the prompt
//     teaches a call that does not compile).
//  3. choir.Help() lists functions this desk cannot call, or omits ones it can.
//  4. Help on a function omits the fields of what it returns.
//  5. An undefined choir name, a bare choir call, or a wrong argument count
//     fails without naming what exists.

func allSurfaceScopes() []*ChoirScope {
	scopes := []*ChoirScope{{desk: "engineering", slot: "verifier"}, {desk: "research", readOnly: true}}
	for desk := range deskModuleSets {
		scopes = append(scopes, &ChoirScope{desk: desk})
	}
	return scopes
}

func sourceMethodParams(t *testing.T, receiver string) map[string][]string {
	t.Helper()
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	out := map[string][]string{}
	fset := token.NewFileSet()
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		parsed, err := parser.ParseFile(fset, file, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, decl := range parsed.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv == nil || len(fn.Recv.List) != 1 {
				continue
			}
			star, ok := fn.Recv.List[0].Type.(*ast.StarExpr)
			if !ok {
				continue
			}
			if ident, ok := star.X.(*ast.Ident); !ok || ident.Name != receiver {
				continue
			}
			var names []string
			for _, field := range fn.Type.Params.List {
				for _, name := range field.Names {
					names = append(names, name.Name)
				}
			}
			out[fn.Name.Name] = names
		}
	}
	return out
}

func TestSurfaceDocsMatchExportsAndSource(t *testing.T) {
	exported := map[string]bool{}
	for _, scope := range allSurfaceScopes() {
		for name := range scope.ChoirExports()["choir/choir"] {
			exported[name] = true
		}
	}
	params := sourceMethodParams(t, "ChoirScope")
	for name := range exported {
		doc, ok := choirFunctionDocs[name]
		if !ok {
			t.Errorf("choir.%s is callable but has no surface doc", name)
			continue
		}
		if strings.TrimSpace(doc.Summary) == "" {
			t.Errorf("choir.%s doc has no summary", name)
		}
		if ns, ok := choirNamespaces[name]; ok {
			// A namespace documents each exported method's parameters.
			nsParams := sourceMethodParams(t, ns.Elem().Name())
			for i := 0; i < ns.NumMethod(); i++ {
				m := ns.Method(i).Name
				if _, documented := doc.Methods[m]; !documented {
					t.Errorf("choir.%s.%s is callable but has no surface doc", name, m)
				}
				if got, want := strings.Join(doc.Methods[m], ","), strings.Join(nsParams[m], ","); got != want {
					t.Errorf("choir.%s.%s doc args = %q, source params = %q", name, m, got, want)
				}
			}
			for m := range doc.Methods {
				if _, ok := ns.MethodByName(m); !ok {
					t.Errorf("surface doc names choir.%s.%s, which is not callable", name, m)
				}
			}
			continue
		}
		if got, want := strings.Join(doc.Args, ","), strings.Join(params[name], ","); got != want {
			t.Errorf("choir.%s doc args = %q, source params = %q", name, got, want)
		}
	}
	for name := range choirFunctionDocs {
		if !exported[name] {
			t.Errorf("surface doc names choir.%s, which no desk can call", name)
		}
	}
}

func TestHelpListsExactlyTheDeskSurface(t *testing.T) {
	texture := &ChoirScope{desk: "texture"}
	var want []string
	for name := range texture.ChoirExports()["choir/choir"] {
		want = append(want, name)
	}
	sort.Strings(want)
	help := texture.Help()
	for _, name := range want {
		if !strings.Contains(help, "choir."+name+"(") {
			t.Errorf("Help() omits callable choir.%s:\n%s", name, help)
		}
	}
	for _, absent := range []string{"choir.Exec(", "choir.ProductAPI(", "choir.WriteFile(", "choir.Verify("} {
		if strings.Contains(help, absent) {
			t.Errorf("Help() lists %s, which texture cannot call", absent)
		}
	}
	if !strings.Contains(help, `choir.Help("`) {
		t.Errorf("Help() does not say how to get one function's detail:\n%s", help)
	}
}

func TestHelpOnFunctionShowsSignatureAndReturnFields(t *testing.T) {
	texture := &ChoirScope{desk: "texture"}
	detail := texture.Help("ReadDoc")
	for _, want := range []string{"choir.ReadDoc()", "revision_id", "content", "doc_id"} {
		if !strings.Contains(detail, want) {
			t.Errorf("Help(\"ReadDoc\") missing %q:\n%s", want, detail)
		}
	}
	updates := texture.Help("Updates")
	for _, want := range []string{"update_id", "from_agent_id", "packet"} {
		if !strings.Contains(updates, want) {
			t.Errorf("Help(\"Updates\") missing return field %q:\n%s", want, updates)
		}
	}
	unknown := texture.Help("PendingUpdates")
	if !strings.Contains(unknown, "no choir.PendingUpdates") || !strings.Contains(unknown, "choir.Updates(") {
		t.Errorf("Help on an unknown name should say so and suggest the closest:\n%s", unknown)
	}
}

func TestDeskSurfacePromptBlock(t *testing.T) {
	block := DeskSurface("texture", "", false)
	for _, want := range []string{"choir.Write(edits string, why string, pSupported ...float64)", "choir.Done(why string)", "choir.Wait(why string)", "choir.Reject(record string, why string)", "choir.ReadDoc()", "choir.Help()", "choir.Updates()"} {
		if !strings.Contains(block, want) {
			t.Errorf("texture surface block missing %q:\n%s", want, block)
		}
	}
	for _, retired := range []string{"choir.Exec(", "choir.ApplyTexture(", "choir.Report(", "choir.ReportPacket(", "choir.Resolve(", "choir.Precommit("} {
		if strings.Contains(block, retired) {
			t.Errorf("texture surface block lists %s:\n%s", retired, block)
		}
	}
	if verifier := DeskSurface("engineering", "verifier", false); !strings.Contains(verifier, "choir.InspectBundle()") {
		t.Errorf("verifier surface omits its slot functions:\n%s", verifier)
	}
	if management := DeskSurface("management", "", false); !strings.Contains(management, "choir.Verify(") {
		t.Errorf("management surface omits Verify (C1e step 4):\n%s", management)
	}
}

func TestCellErrorHintNamesWhatExists(t *testing.T) {
	names := []string{"Write", "Help", "Inbox", "ReadDoc", "Report", "Updates"}
	cases := []struct {
		src, msg string
		want     []string
	}{
		{"choir.PendingUpdates()", "1:7: undefined selector choir.PendingUpdates", []string{"choir has no PendingUpdates", "choir.Updates", "choir.Help()"}},
		{"choir.Ledger()", `1:1: package choir "choir" has no symbol Ledger`, []string{"choir has no Ledger", "choir.Help()"}},
		{"d := ReadDoc()", "1:6: constant definition loop", []string{"choir.ReadDoc()"}},
		{"x := Inbox()", "1:6: undefined: Inbox", []string{"choir.Inbox()"}},
		{`choir.Report("management", "claim")`, "1:1: not enough arguments in call to choir.Report", []string{"choir.Report(toDesk string, claim string, evidenceRefs []string, resolverID string)"}},
	}
	surface := map[string]bool{}
	for _, name := range names {
		surface[name] = true
	}
	for _, tc := range cases {
		hint := cellErrorHint(surface, tc.src, tc.msg)
		for _, want := range tc.want {
			if !strings.Contains(hint, want) {
				t.Errorf("hint for %q / %q missing %q: %q", tc.src, tc.msg, want, hint)
			}
		}
	}
	// Cluster A: the top-level forms yaegi cannot compile get the loop form.
	for _, msg := range []string{"1:8: constant definition loop", "yaegi compile panic: nil type", "yaegi compile panic: runtime error: invalid memory address or nil pointer dereference"} {
		if hint := cellErrorHint(surface, "rec := recs[0]", msg); !strings.Contains(hint, "for _, r := range") || !strings.Contains(hint, "func() {") {
			t.Errorf("hint for top-level %q lacks the loop form: %q", msg, hint)
		}
	}
	// Text inside string literals and comments is not code: a capture id
	// is not a reference to choir.web_capture.
	if hint := cellErrorHint(surface, "r := choir.Inbox() // choir.Ledger\nid := \"obj:choir.web_capture:abc\"\n_ = r.ID", "3:7: undefined method: ID"); strings.Contains(hint, "web_capture") || strings.Contains(hint, "Ledger") {
		t.Errorf("hint read a string literal or comment as code: %q", hint)
	}
	// Cluster C: Read's JSON bytes used as a map or a struct.
	for _, tc := range [][2]string{
		{"r, _ := choir.Read(u)\nfmt.Println(r[\"capture_id\"])", `6:25: cannot convert "capture_id" to int`},
		{"r, _ := choir.Read(u)\nm := r.(map[string]any)", "6:13: invalid type assertion: non-interface type yaegikernel.JSON on left"},
		{"r, _ := choir.ReadCapture(id, 0, 0)\nfmt.Println(r.Blocks)", "5:22: undefined method: Blocks"},
	} {
		if hint := cellErrorHint(map[string]bool{"Read": true, "ReadCapture": true, "Report": true}, tc[0], tc[1]); !strings.Contains(hint, "json.Unmarshal(r, &c)") || !strings.Contains(hint, "capture_id") {
			t.Errorf("Read misuse %q got no decode hint: %q", tc[1], hint)
		}
	}
	// T3 (docs/problems/tool-call-failures-dual-model-2026-10-21.md): a packet
	// written as JSON inside a Go map literal does not parse; the hint gives
	// the typed forms and the JSON string the verbs also take.
	packets := map[string]bool{"ReportPacket": true, "Cast": true, "Report": true}
	for _, tc := range [][2]string{
		{`p := map[string]any{"kind":"evidence_update","claims":[] ,"sources":[]}; choir.ReportPacket("texture", p, "")`, "_.go:1:1309: expected type, found ','"},
		{`choir.Cast("texture:new", "Harbor", map[string]any{"hypotheses":[]any{{"statement":"x"}}})`, "_.go:1:60: expected ';', found ':' (and 3 more errors)"},
	} {
		hint := cellErrorHint(packets, tc[0], tc[1])
		for _, want := range []string{"[]any{}", "map[string]any{", "JSON string", "`{"} {
			if !strings.Contains(hint, want) {
				t.Errorf("packet literal %q got hint %q, missing %q", tc[1], hint, want)
			}
		}
	}
	if hint := cellErrorHint(surface, "x := 1 +", "1:9: expected operand"); hint != "" {
		t.Errorf("an unrelated compile error got a choir hint: %q", hint)
	}
}

func TestSessionCellErrorCarriesSurfaceHint(t *testing.T) {
	scope := &ChoirScope{desk: "texture"}
	sess, err := NewSession(NewAllowlist("choir"), scope.ChoirExports())
	if err != nil {
		t.Fatal(err)
	}
	defer sess.Close()
	out, _ := serveCell(sess, SessionFrame{ID: "c1", Source: "u := choir.PendingUpdates()\n_ = u"}, nil, nil)
	if !strings.Contains(out.Error, "choir has no PendingUpdates") || !strings.Contains(out.Error, "choir.Updates") {
		t.Fatalf("cell error lacks the surface hint: %q", out.Error)
	}
	help, _ := serveCell(sess, SessionFrame{ID: "c2", Source: "println(choir.Help())"}, nil, nil)
	if help.Error != "" || !strings.Contains(help.Stdout+help.Stderr, "choir.Write(") {
		t.Fatalf("choir.Help() in the REPL = %+v", help)
	}
}

// Texture probed its write verb with turns built to fail, to read the valid
// values from the rejection (docs/archive/problems/texture-probes-canonical-write-to-learn-schema-2026-10-11.md).
// Write's help gives the whole grammar, so the only way to learn it is not
// a staged write.
func TestWriteHelpGivesTheWholeGrammar(t *testing.T) {
	doc := choirFunctionDocs["Write"].Summary
	for _, want := range []string{"@replace", "@insert", "@delete", "@move", "@section", "@all", "{{cite", "{{ref", "line"} {
		if !strings.Contains(doc, want) {
			t.Errorf("Write help does not mention %q", want)
		}
	}
	// {{embed}} printed a source's text on the published page; Write
	// refuses it, so its help must not teach it (owner, 2026-10-16).
	if strings.Contains(doc, "{{embed") {
		t.Error("Write help still teaches {{embed}}")
	}
}

// Async search (Cut 2 unit 2a). Ways the surface can fail: a desk still
// exports the blocking choir.WebSearch; research or a research child lacks
// the async verbs; choir.Searches renders as a call instead of its methods;
// the surface does not say an arrival never wakes the desk.
func TestSearchSurfaceIsAsyncOnResearchOnly(t *testing.T) {
	for desk := range deskModuleSets {
		exports := (&ChoirScope{desk: desk, searches: newSearchStore()}).ChoirExports()["choir/choir"]
		if _, ok := exports["WebSearch"]; ok {
			t.Errorf("%s still exports choir.WebSearch", desk)
		}
		_, has := exports["SearchStart"]
		if want := desk == "research" || desk == SessionRoleResearchChild; has != want {
			t.Errorf("%s exports SearchStart = %v, want %v", desk, has, want)
		}
	}
	block := DeskSurface("research", "", true)
	for _, want := range []string{
		"choir.SearchStart(query string) (SearchStub, error)",
		"choir.Searches.Get(id string) (Search, error), choir.Searches.List() []Search",
		"choir.SearchWait(id string, ms int) (SearchCount, error)",
		"never wakes you",
	} {
		if !strings.Contains(block, want) {
			t.Errorf("research surface missing %q:\n%s", want, block)
		}
	}
	detail := (&ChoirScope{desk: "research", searches: newSearchStore()}).Help("Searches")
	for _, want := range []string{"arrivals", "answered", "results", "done"} {
		if !strings.Contains(detail, `json "`+want+`"`) {
			t.Errorf("Help(\"Searches\") missing field %q:\n%s", want, detail)
		}
	}
}
