package yaegikernel

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/traefik/yaegi/interp"
)

func TestRefuseBannedPackages(t *testing.T) {
	evaluator := newSessionEvaluator(NewDefaultSafeAllowlist(), nil)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	bannedTests := []struct {
		name string
		src  string
	}{
		{"unsafe", `import "unsafe"; func main() {}`},
		{"reflect", `import "reflect"; func main() {}`},
		{"syscall", `import "syscall"; func main() {}`},
		{"os", `import "os"; func main() { os.Exit(1) }`},
		{"os/exec", `import "os/exec"; func main() { exec.Command("ls") }`},
		{"net", `import "net"; func main() {}`},
		{"net/http", `import "net/http"; func main() {}`},
		{"runtime", `import "runtime"; func main() {}`},
		{"plugin", `import "plugin"; func main() {}`},
	}

	for _, tt := range bannedTests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := evaluator.Eval(ctx, tt.src)
			if err == nil {
				t.Fatalf("expected evaluation to refuse banned package %q, got nil error", tt.name)
			}
			if !strings.Contains(err.Error(), "refused") && !strings.Contains(err.Error(), "forbidden") {
				t.Fatalf("expected refusal error for %q, got %v", tt.name, err)
			}
		})
	}
}

func TestRefuseUnlistedPackage(t *testing.T) {
	evaluator := newSessionEvaluator(NewDefaultSafeAllowlist(), nil)
	ctx := context.Background()

	src := `import "github.com/traefik/yaegi/interp"; func main() {}`
	_, err := evaluator.Eval(ctx, src)
	if err == nil {
		t.Fatalf("expected evaluation to refuse unlisted external package, got nil error")
	}
	if !strings.Contains(err.Error(), "refused") {
		t.Fatalf("expected refusal error, got %v", err)
	}
}

func TestAllowSafeStdlib(t *testing.T) {
	evaluator := newSessionEvaluator(NewDefaultSafeAllowlist(), nil)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	src := `
package main

import (
	"fmt"
	"strings"
	"math/big"
	"encoding/json"
)

func main() {
	upper := strings.ToUpper("hello yaegi")
	n := big.NewInt(42)
	data, _ := json.Marshal(map[string]string{"msg": upper})
	fmt.Printf("%s n=%s json=%s", upper, n.String(), string(data))
}
`
	res, err := evaluator.Eval(ctx, src)
	if err != nil {
		t.Fatalf("safe evaluation failed: %v, stderr=%s", err, res.Stderr)
	}
	if !strings.Contains(res.Stdout, "HELLO YAEGI") || !strings.Contains(res.Stdout, "n=42") {
		t.Fatalf("unexpected stdout: %q", res.Stdout)
	}
}

// TestCheckImportsFailClosedOnParseError asserts that an import block that
// cannot be statically parsed is refused rather than silently proceeding to the
// interpreter (the allowlist-bypass risk).
func TestCheckImportsFailClosedOnParseError(t *testing.T) {
	// A snippet whose import block cannot be parsed (no package clause, unbalanced
	// braces) must be refused by CheckImports, not pass through with nil.
	src := `import "os/exec"; func main() {`
	if err := checkImports(NewDefaultSafeAllowlist(), src); err == nil {
		t.Fatalf("expected CheckImports to fail closed on unparseable import block, got nil")
	}
}

// TestEmptyAllowlistDefaultsToSafe asserts that an empty/nil package list fails
// CLOSED to the default safe stdlib set, so an omitted list never yields an
// empty allowlist that rejects every safe package (nor accepts any package).
func TestEmptyAllowlistDefaultsToSafe(t *testing.T) {
	// An empty allowlist must still admit safe packages.
	e := newSessionEvaluator(NewAllowlist(), nil)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	res, err := e.Eval(ctx, `package main; import "fmt"; func main(){ fmt.Println("ok") }`)
	if err != nil {
		t.Fatalf("empty allowlist should default to safe set but refused: %v (stderr=%s)", err, res.Stderr)
	}
	if !strings.Contains(res.Stdout, "ok") {
		t.Fatalf("expected stdout output, got %q", res.Stdout)
	}

	// But an empty allowlist must still refuse a banned package.
	_, err = e.Eval(ctx, `package main; import "os/exec"; func main(){}`)
	if err == nil {
		t.Fatalf("empty allowlist default must still refuse banned os/exec")
	}
}

// TestCannotExpandPackagesPastServerAllowlist asserts that adding an
// authority-bearing package to the request does not broaden the effective
// allowlist: the worker's package set is server-owned, so a model-supplied
// dangerous package is still refused.
func TestCannotExpandPackagesPastServerAllowlist(t *testing.T) {
	// A request allowlisting only "fmt" must refuse "go/parser" even if the
	// model tried to widen it, because the effective set is server-owned.
	e := newSessionEvaluator(NewAllowlist("fmt"), nil)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := e.Eval(ctx, `package main; import "go/parser"; func main(){}`)
	if err == nil {
		t.Fatalf("expected go/parser to be refused when the allowlist is fmt-only")
	}
}

// TestEvalOutputOverflowFailsClosed asserts that a model-authored program which
// prints well beyond maxEvalOutputBytes is cut off and returns an overflow
// error, rather than consuming unbounded memory.
func TestEvalOutputOverflowFailsClosed(t *testing.T) {
	e := newSessionEvaluator(NewDefaultSafeAllowlist(), nil)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	src := `package main
import "fmt"
func main() {
  for i := 0; i < 1000000; i++ { fmt.Println("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa") }
}
`
	res, err := e.Eval(ctx, src)
	if err == nil {
		t.Fatalf("expected overflow error for runaway output, got nil")
	}
	if !strings.Contains(err.Error(), "output exceeded limit") {
		t.Fatalf("expected output-exceeded error, got: %v (stdout len=%d)", err, len(res.Stdout))
	}
	if len(res.Stdout) > maxEvalOutputBytes {
		t.Fatalf("stdout exceeded cap: got %d bytes, cap %d", len(res.Stdout), maxEvalOutputBytes)
	}
}

// sessionEvaluator runs each source in a fresh live Session (the path desk
// cells take), so the containment and refusal contracts are pinned on the
// interpreter the product uses, not on a one-shot prototype.
type sessionEvaluator struct {
	allowlist *Allowlist
	extra     interp.Exports
}

func newSessionEvaluator(allowlist *Allowlist, extra interp.Exports) *sessionEvaluator {
	return &sessionEvaluator{allowlist: allowlist, extra: extra}
}

func (e *sessionEvaluator) Eval(ctx context.Context, src string) (EvalResult, error) {
	sess, err := NewSession(e.allowlist, e.extra)
	if err != nil {
		return EvalResult{}, err
	}
	defer sess.Close()
	return sess.Eval(ctx, src)
}
