package main

// `choir paper` against a fake operator route in front of the real files
// handler (docs/design/paper-control-cli-2026-11-09.md, slices O2 and O3).
// Ways it could fail:
//   - the policy verbs go to the owner's own routes instead of the operator
//     route for the platform computer;
//   - the owner-targeting header (X-Choir-Computer) is sent on the operator
//     path, or a computer id that is not a computer id becomes a path;
//   - with two platform computers the CLI picks one silently;
//   - the receipt the server returns is not shown;
//   - a paper verb appears as an MCP tool.

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/autoputer"
	"github.com/yusefmosiah/go-choir/internal/server"
)

const (
	paperID1 = "computer-1355b83bced1613eb048ee28d66688a8"
	paperID2 = "computer-0000b83bced1613eb048ee28d66688a8"
	paperOp  = "1111aaaa-2222-3333-4444-55555555beef"
)

type paperFake struct {
	root     string
	mu       sync.Mutex
	paths    []string
	computer []string // X-Choir-Computer seen per request
	ids      []string // computers the status route lists
	receipts int
}

func newPaperFake(t *testing.T, ids ...string) *paperFake {
	t.Helper()
	f := &paperFake{root: t.TempDir(), ids: ids}
	fh := autoputer.NewFilesHandler(f.root)
	fh.SetPolicyActRecorder(func(_ *http.Request, act *autoputer.PolicyAct) error {
		f.mu.Lock()
		f.receipts++
		act.ReceiptID = "evt-receipt-1"
		f.mu.Unlock()
		return nil
	})
	mux := http.NewServeMux()
	mux.HandleFunc(paperComputersRoute, func(w http.ResponseWriter, r *http.Request) {
		f.note(r)
		var list []map[string]string
		for _, id := range f.ids {
			list = append(list, map[string]string{"computer_id": id})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"computers": list})
	})
	prefix := paperComputersRoute + "/" + paperID1 + "/guest"
	mux.HandleFunc(prefix+"/api/files/", func(w http.ResponseWriter, r *http.Request) {
		f.note(r)
		r.URL.Path = strings.TrimPrefix(r.URL.Path, prefix)
		r.Header.Set("X-Authenticated-User", "paper-owner")
		r.Header.Set(server.PlatformOperatorHeader, paperOp)
		r.RemoteAddr = "192.0.2.1:1"
		fh.HandleFileByPath(w, r)
	})
	mux.HandleFunc(prefix+"/api/model-policy/resolve", func(w http.ResponseWriter, r *http.Request) {
		f.note(r)
		_ = json.NewEncoder(w).Encode(map[string]string{"role": r.URL.Query().Get("role"), "model": "pool:default"})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	t.Setenv(hostEnvVar, srv.URL)
	t.Setenv(apiKeyEnvVar, "choir_sk_test")
	return f
}

func (f *paperFake) note(r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.paths = append(f.paths, r.Method+" "+r.URL.Path)
	f.computer = append(f.computer, r.Header.Get("X-Choir-Computer"))
}

func runPaper(args ...string) (int, string, string) {
	var stdout, stderr bytes.Buffer
	code := run(append([]string{"paper"}, args...), &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func TestPaperPolicySetGoesThroughTheOperatorRouteAndShowsTheReceipt(t *testing.T) {
	f := newPaperFake(t, paperID1)
	seedServerPolicy(t, f.root, "# owner-edited\n[defaults]\nmodel = \"pool:default\"\n")
	code, out, errOut := runPaper("policy", "set", "--local", writeLocal(t, cliPolicy))
	if code != 0 {
		t.Fatalf("paper policy set failed: %d %s", code, errOut)
	}
	var res policyWriteResult
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if res.Status != "saved" || res.Revision == "" || res.Receipt != "evt-receipt-1" || f.receipts != 1 {
		t.Fatalf("result = %+v, receipts %d", res, f.receipts)
	}
	if got, _ := os.ReadFile(filepath.Join(f.root, "System", "model-policy.toml")); string(got) != cliPolicy {
		t.Fatalf("saved = %q", got)
	}
	for _, p := range f.paths {
		if !strings.HasPrefix(p, "GET "+paperComputersRoute) && !strings.HasPrefix(p, "PUT "+paperComputersRoute) {
			t.Fatalf("a request left the operator route: %q in %v", p, f.paths)
		}
	}
	for _, c := range f.computer {
		if c != "" {
			t.Fatalf("X-Choir-Computer was sent on the operator path: %v", f.computer)
		}
	}
}

func TestPaperReadsAndRollbackUseTheOperatorRoute(t *testing.T) {
	f := newPaperFake(t, paperID1)
	seedServerPolicy(t, f.root, cliPolicy)
	if code, _, errOut := runPaper("policy", "set", "--local", writeLocal(t, strings.Replace(cliPolicy, "low", "high", 1))); code != 0 {
		t.Fatalf("set: %d %s", code, errOut)
	}
	code, out, errOut := runPaper("policy", "revisions")
	if code != 0 || !strings.Contains(out, ".toml") {
		t.Fatalf("revisions: %d %s %s", code, out, errOut)
	}
	if code, out, errOut := runPaper("policy", "get"); code != 0 || !strings.Contains(out, "owner-edited") {
		t.Fatalf("get: %d %s %s", code, out, errOut)
	}
	if code, _, errOut := runPaper("policy", "rollback"); code != 0 {
		t.Fatalf("rollback: %d %s", code, errOut)
	}
	if got, _ := os.ReadFile(filepath.Join(f.root, "System", "model-policy.toml")); string(got) != cliPolicy {
		t.Fatalf("rollback restored %q", got)
	}
	if f.receipts != 2 {
		t.Fatalf("set and rollback each need a receipt, got %d", f.receipts)
	}
}

func TestPaperNeedsOneUnambiguousPlatformComputer(t *testing.T) {
	f := newPaperFake(t, paperID1, paperID2)
	code, _, errOut := runPaper("policy", "revisions")
	if code == 0 || !strings.Contains(errOut, "--computer") {
		t.Fatalf("two computers: %d %q", code, errOut)
	}
	if code, _, errOut := runPaper("policy", "revisions", "--computer", "../../etc/passwd"); code == 0 || !strings.Contains(errOut, "computer-") {
		t.Fatalf("bad id accepted: %d %q", code, errOut)
	}
	if code, _, errOut := runPaper("policy", "revisions", "--computer", paperID1); code != 0 {
		t.Fatalf("named computer: %d %q", code, errOut)
	}
	for _, p := range f.paths {
		if strings.Contains(p, "..") {
			t.Fatalf("a bad id became a path: %q", p)
		}
	}
}

func TestPaperStatusIsAReadOfTheOperatorRoute(t *testing.T) {
	f := newPaperFake(t, paperID1)
	code, out, errOut := runPaper("status")
	if code != 0 || !strings.Contains(out, paperID1) {
		t.Fatalf("status: %d %s %s", code, out, errOut)
	}
	if len(f.paths) != 1 || f.paths[0] != "GET "+paperComputersRoute {
		t.Fatalf("status requests = %v", f.paths)
	}
}

func TestPaperVerbsAreNotMCPTools(t *testing.T) {
	for _, cmd := range mcpCommands() {
		if cmd.path[0] == "paper" {
			t.Fatalf("%s is exposed as an MCP tool", cmd.name())
		}
	}
}
