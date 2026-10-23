package autoputer

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

// The eager stream_seq seed starts in runReplayPhase right after the
// vocabulary fence, before the runtime starts serving
// (docs/problems/stream-seq-high-water-mark-design-2026-10-23.md, section 4).
// Ways it can fail, written before the code:
//  1. It starts before replay has finished: the head precondition does not
//     hold and the scan could race an applied row.
//  2. It never starts at boot, so the first allocation (the boot passivation)
//     seeds lazily, inside a request path, with no log of its own.
//  3. It starts after the runtime, so migration is a planned outage of event
//     appends instead of a wait.
func TestRunReplayPhaseStartsTheEagerSeedAfterTheVocabularyFence(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "run.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var fn *ast.FuncDecl
	for _, decl := range file.Decls {
		if d, ok := decl.(*ast.FuncDecl); ok && d.Name.Name == "runReplayPhase" {
			fn = d
		}
	}
	if fn == nil {
		t.Fatal("runReplayPhase not found")
	}
	var fenced, seed, started token.Pos
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		switch fun := call.Fun.(type) {
		case *ast.Ident:
			if fun.Name == "bootMark" && len(call.Args) == 1 {
				if lit, ok := call.Args[0].(*ast.BasicLit); ok {
					switch lit.Value {
					case `"vocab_fenced"`:
						fenced = call.Pos()
					case `"runtime_started"`:
						started = call.Pos()
					}
				}
			}
		case *ast.SelectorExpr:
			if fun.Sel.Name == "StartStreamSeqSeed" {
				seed = call.Pos()
			}
		}
		return true
	})
	if fenced == 0 || seed == 0 || started == 0 {
		t.Fatalf("runReplayPhase must mark vocab_fenced, call StartStreamSeqSeed and mark runtime_started (fenced=%d seed=%d started=%d)", fenced, seed, started)
	}
	if !(fenced < seed && seed < started) {
		t.Fatalf("order must be vocab_fenced < StartStreamSeqSeed < runtime_started; got %d, %d, %d", fenced, seed, started)
	}
}
