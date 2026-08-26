package yaegikernel

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestRefuseBannedPackages(t *testing.T) {
	evaluator := NewEvaluator(NewDefaultSafeAllowlist(), nil)
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
	evaluator := NewEvaluator(NewDefaultSafeAllowlist(), nil)
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
	evaluator := NewEvaluator(NewDefaultSafeAllowlist(), nil)
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
