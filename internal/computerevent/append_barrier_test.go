package computerevent

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"sort"
	"strings"
	"testing"
)

// The append barrier (docs/reports/telemetry-batching-design-2026-10-20.md
// 4.3, item 4) is what makes telemetry loss a tail: any acknowledged write
// first seals and waits for the pending telemetry window, so the tape never
// shows a semantic event ahead of an earlier telemetry record. It has to sit on
// every entry point that appends; a missed one is a hole in that guarantee.

// appendEntryPoints are the exported appender methods that append an event.
// The scan fails if another exported method takes a.mu without being listed
// here or in nonAppendMethods, and if a listed one does not call
// awaitBarrier before it locks.
var appendEntryPoints = []string{"Append", "AppendNew", "AppendNewPayload", "AppendNewPrivatePayload", "AppendNewPayloadSet"}

// nonAppendMethods take a.mu but never append an event: they hold it,
// swap a dependency, or reconstruct from the canonical tape.
var nonAppendMethods = map[string]string{
	"HoldAppends":               "holds the mutex; nothing inside may append or read through the sealer (design 6.2 item 5)",
	"SetReplayMode":             "flips a flag",
	"RebindProjection":          "swaps the projection",
	"ReplaySnapshot":            "reads replay progress",
	"SetReplayCheckpointPolicy": "sets replay checkpoint policy",
	"SetPayloadResolver":        "installs the payload reader",
	"RecoverPrepared":           "settles prepared events at boot, before the sealer has a queue",
	"ReconstructThrough":        "replay from the canonical tape",
	"ReconstructInto":           "replay from the canonical tape",
	"ReconstructThroughTarget":  "replay from the canonical tape",
}

func TestEveryAppenderEntryPointRunsTheBarrier(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "appender.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	isEntry := map[string]bool{}
	for _, name := range appendEntryPoints {
		isEntry[name] = true
	}
	seen := map[string]bool{}
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv == nil || fn.Body == nil || !fn.Name.IsExported() {
			continue
		}
		star, ok := fn.Recv.List[0].Type.(*ast.StarExpr)
		if !ok {
			continue
		}
		if id, ok := star.X.(*ast.Ident); !ok || id.Name != "ComputerEventAppender" {
			continue
		}
		name := fn.Name.Name
		lockPos, barrierPos := token.NoPos, token.NoPos
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			switch exprString(sel) {
			case "a.mu.Lock":
				if lockPos == token.NoPos || call.Pos() < lockPos {
					lockPos = call.Pos()
				}
			case "a.awaitBarrier":
				if barrierPos == token.NoPos || call.Pos() < barrierPos {
					barrierPos = call.Pos()
				}
			}
			return true
		})
		switch {
		case isEntry[name]:
			seen[name] = true
			if lockPos == token.NoPos {
				t.Errorf("%s is listed as an append entry point but never locks a.mu", name)
			}
			if barrierPos == token.NoPos || barrierPos > lockPos {
				t.Errorf("%s must call a.awaitBarrier(ctx) before it locks a.mu", name)
			}
		case lockPos != token.NoPos:
			if _, ok := nonAppendMethods[name]; !ok && strings.HasPrefix(name, "Append") {
				t.Errorf("%s looks like an append entry point and locks a.mu: add it to appendEntryPoints and call a.awaitBarrier(ctx) first", name)
			} else if !ok {
				t.Errorf("%s locks a.mu and is in neither appendEntryPoints nor nonAppendMethods: decide whether it appends", name)
			}
		}
	}
	var missing []string
	for _, name := range appendEntryPoints {
		if !seen[name] {
			missing = append(missing, name)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Fatalf("append entry points not found in appender.go: %v", missing)
	}
}

func exprString(sel *ast.SelectorExpr) string {
	switch x := sel.X.(type) {
	case *ast.Ident:
		return x.Name + "." + sel.Sel.Name
	case *ast.SelectorExpr:
		return exprString(x) + "." + sel.Sel.Name
	}
	return sel.Sel.Name
}

func newBarrierTestAppender(t *testing.T) (*ComputerEventAppender, *dryRunProjection) {
	t.Helper()
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer := SigningKey{SignerRef: SignerRef{SignerDomain: "platform-control", KeyID: "platform-1"}, PrivateKey: privateKey}
	projection := &dryRunProjection{memoryProjection: &memoryProjection{}}
	appender, err := NewComputerEventAppender(testComputerID, memoryPinner{signer: signer}, projection, &memoryCAS{signer: signer}, EventHeadReceiptVerifier{Keys: staticKeyResolver{key: publicKey}})
	if err != nil {
		t.Fatal(err)
	}
	return appender, projection
}

// Each entry point calls the hook exactly once, before it takes a.mu (the
// hook may flush through the appender; if it ran under a.mu it would
// deadlock), and an exempt context skips it.
func TestAppendBarrierRunsOncePerEntryPointBeforeTheLock(t *testing.T) {
	appender, _ := newBarrierTestAppender(t)
	calls, lockedInHook := 0, 0
	appender.SetAppendBarrier(func(context.Context) {
		calls++
		if !appender.mu.TryLock() {
			lockedInHook++
			return
		}
		appender.mu.Unlock()
	})
	ctx := context.Background()
	event := testEvent(t, nil, EventProjectionBatchRecorded)
	calls0 := func(name string, call func()) {
		t.Helper()
		before := calls
		call() // the call may fail on its arguments; the hook runs first regardless
		if calls != before+1 {
			t.Errorf("%s: barrier ran %d times, want 1", name, calls-before)
		}
	}
	calls0("Append", func() { _, _ = appender.Append(ctx, event, TransitionInput{}, nil) })
	calls0("AppendNew", func() { _, _ = appender.AppendNew(ctx, event, TransitionInput{}, nil) })
	calls0("AppendNewPayload", func() {
		_, _, _ = appender.AppendNewPayload(ctx, event, TransitionInput{}, testBatchPayloadFor(t, event.EventID), ProjectionBatchMediaType, "owner")
	})
	calls0("AppendNewPrivatePayload", func() {
		_, _, _ = appender.AppendNewPrivatePayload(ctx, event, TransitionInput{}, []byte("x"), "text/plain", nil)
	})
	calls0("AppendNewPayloadSet", func() { _, _, _ = appender.AppendNewPayloadSet(ctx, event, TransitionInput{}, nil, nil) })
	if lockedInHook != 0 {
		t.Fatalf("the barrier ran with a.mu held %d times", lockedInHook)
	}

	before := calls
	_, _, _ = appender.AppendNewPayload(WithBarrierExemption(ctx), event, TransitionInput{}, testBatchPayloadFor(t, event.EventID), ProjectionBatchMediaType, "owner")
	if calls != before {
		t.Fatalf("an exempt context ran the barrier")
	}
	appender.SetAppendBarrier(nil)
	_, _ = appender.AppendNew(ctx, event, TransitionInput{}, nil)
}

// A dry-run refusal and a decode refusal both wrap ErrBatchRefused, so the
// sealer can tell "nothing was pinned, split and retry" from an unknown
// outcome. The message is unchanged.
func TestAppendNewPayloadWrapsBatchRefusal(t *testing.T) {
	appender, projection := newBarrierTestAppender(t)
	ctx := context.Background()
	genesis := testEvent(t, nil, EventGenesisImported)
	genesis.ResultingEffectiveCommitment = testDigestA
	if _, err := appender.AppendNew(ctx, genesis, TransitionInput{TargetStateCommitment: testDigestA}, nil); err != nil {
		t.Fatal(err)
	}
	projection.dryRunErr = errors.New("vocabulary refused")
	event := testEvent(t, nil, EventProjectionBatchRecorded)
	_, _, err := appender.AppendNewPayload(ctx, event, TransitionInput{}, testBatchPayloadFor(t, event.EventID), ProjectionBatchMediaType, "owner")
	if !errors.Is(err, ErrBatchRefused) {
		t.Fatalf("dry-run refusal = %v, want ErrBatchRefused", err)
	}
	if !strings.Contains(err.Error(), "projection dry-run refused batch: vocabulary refused") {
		t.Fatalf("refusal message changed: %v", err)
	}
	projection.dryRunErr = nil
	_, _, err = appender.AppendNewPayload(ctx, event, TransitionInput{}, []byte("{not json"), ProjectionBatchMediaType, "owner")
	if !errors.Is(err, ErrBatchRefused) {
		t.Fatalf("decode refusal = %v, want ErrBatchRefused", err)
	}
}
