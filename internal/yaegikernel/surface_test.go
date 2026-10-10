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

// docs/problems/trace-review-desk-protocol-friction-2026-10-10.md F13: a desk
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

func sourceMethodParams(t *testing.T) map[string][]string {
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
			if ident, ok := star.X.(*ast.Ident); !ok || ident.Name != "ChoirScope" {
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
	params := sourceMethodParams(t)
	for name := range exported {
		doc, ok := choirFunctionDocs[name]
		if !ok {
			t.Errorf("choir.%s is callable but has no surface doc", name)
			continue
		}
		if strings.TrimSpace(doc.Summary) == "" {
			t.Errorf("choir.%s doc has no summary", name)
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
	for _, absent := range []string{"choir.Exec(", "choir.Cast(", "choir.WriteFile(", "choir.Verify("} {
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
	for _, want := range []string{"choir.ApplyTexture(edit any)", "choir.ReadDoc()", "choir.Help()", "choir.Updates()"} {
		if !strings.Contains(block, want) {
			t.Errorf("texture surface block missing %q:\n%s", want, block)
		}
	}
	if strings.Contains(block, "choir.Exec(") {
		t.Errorf("texture surface block lists Exec:\n%s", block)
	}
	if verifier := DeskSurface("engineering", "verifier", false); !strings.Contains(verifier, "choir.Verify(") || !strings.Contains(verifier, "choir.InspectBundle()") {
		t.Errorf("verifier surface omits its slot functions:\n%s", verifier)
	}
}

func TestCellErrorHintNamesWhatExists(t *testing.T) {
	names := []string{"ApplyTexture", "Help", "Inbox", "ReadDoc", "Report", "Updates"}
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
	if help.Error != "" || !strings.Contains(help.Stdout+help.Stderr, "choir.ApplyTexture(") {
		t.Fatalf("choir.Help() in the REPL = %+v", help)
	}
}
