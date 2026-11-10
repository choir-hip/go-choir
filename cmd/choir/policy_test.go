package main

// `choir policy` against the real files handler behind an httptest server
// (docs/design/paper-control-cli-2026-11-09.md, step 1). Ways it could fail:
//   - set writes a policy that lacks the "# owner-edited" marker, which the
//     platform then overwrites at its next policy version, with no word;
//   - the marker check cannot be overridden on purpose (--allow-unmarked);
//   - set reports success when the server refused the file, or when the
//     saved bytes differ from what was sent;
//   - set does not name the revision kept, or the resolve for each role;
//   - invalid TOML or an unknown role is saved anyway (server side, through
//     the real handler);
//   - rollback restores something other than the latest revision, ignores
//     --to, or writes around the validation;
//   - --to carries a path out of the revisions directory;
//   - revisions on a computer that never replaced its policy errors instead
//     of listing nothing;
//   - the policy commands are CLI-only by accident, or MCP set offers --local.

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/autoputer"
	"github.com/yusefmosiah/go-choir/internal/modelpolicy"
)

const cliPolicy = "# owner-edited\n[defaults]\nmodel = \"pool:default\"\nreasoning = \"low\"\n\n[roles.texture]\nmodel = \"pool:default\"\nreasoning = \"medium\"\n"

// policyServer serves the real files handler over a temp root, plus a canned
// resolve route that echoes the role it was asked about.
func policyServer(t *testing.T) (root string, requests *[]string) {
	t.Helper()
	root = t.TempDir()
	fh := autoputer.NewFilesHandler(root)
	var mu sync.Mutex
	var seen []string
	mux := http.NewServeMux()
	mux.HandleFunc("/api/files/", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seen = append(seen, r.Method+" "+r.URL.Path)
		mu.Unlock()
		// What the proxy does: assert the owner, from the host side of the
		// guest boundary (an empty RemoteAddr is that side in a test).
		r.Header.Set("X-Authenticated-User", "owner@example.com")
		r.RemoteAddr = ""
		fh.HandleFileByPath(w, r)
	})
	mux.HandleFunc("/api/model-policy/resolve", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// The real policy manager over the same files, so the pools a test
		// sees are the ones the file defines.
		// A copy of the file: the manager rewrites an unmarked file it loads,
		// and the server under test must keep what the test wrote.
		role := r.URL.Query().Get("role")
		copyPath := filepath.Join(t.TempDir(), "System", "model-policy.toml")
		_ = os.MkdirAll(filepath.Dir(copyPath), 0o755)
		if raw, err := os.ReadFile(filepath.Join(root, "System", "model-policy.toml")); err == nil {
			_ = os.WriteFile(copyPath, raw, 0o644)
		}
		manager := modelpolicy.NewManager(modelpolicy.ManagerConfig{PolicyPath: copyPath})
		md := manager.EnrichMetadata(r.Context(), "owner", role, map[string]any{"trajectory_id": "t"})
		_ = json.NewEncoder(w).Encode(map[string]any{
			"role": role, "model": md[modelpolicy.MetadataModel], "policy_error": md[modelpolicy.MetadataPolicyError],
			"pools": manager.PoolViews(r.Context(), "owner"),
		})
	})
	mux.HandleFunc("/api/model-policy/verify", func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(append([]byte(`{"status":"verified","echo":`), append(raw, '}')...))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	t.Setenv(hostEnvVar, srv.URL)
	t.Setenv(apiKeyEnvVar, "choir_sk_test")
	return root, &seen
}

func runPolicy(args ...string) (int, string, string) {
	var stdout, stderr bytes.Buffer
	code := run(append([]string{"policy"}, args...), &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func writeLocal(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "policy.toml")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func seedServerPolicy(t *testing.T, root, body string) {
	t.Helper()
	p := filepath.Join(root, "System", "model-policy.toml")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestPolicySetWritesReadsBackAndNamesRevision(t *testing.T) {
	root, _ := policyServer(t)
	seedServerPolicy(t, root, "# owner-edited\n[defaults]\nmodel = \"pool:default\"\n")
	code, out, errOut := runPolicy("set", "--local", writeLocal(t, cliPolicy))
	if code != 0 {
		t.Fatalf("set failed: %d %s", code, errOut)
	}
	var res policyWriteResult
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatalf("output is not the result: %v\n%s", err, out)
	}
	if res.Status != "saved" || res.Revision == "" || len(res.Roles) != len(policyRoles) {
		t.Fatalf("result = %+v", res)
	}
	got, _ := os.ReadFile(filepath.Join(root, "System", "model-policy.toml"))
	if string(got) != cliPolicy {
		t.Fatalf("saved = %q", got)
	}
	kept, _ := os.ReadFile(filepath.Join(root, "System", "model-policy.revisions", res.Revision))
	if string(kept) != "# owner-edited\n[defaults]\nmodel = \"pool:default\"\n" {
		t.Fatalf("revision = %q", kept)
	}
}

func TestPolicySetRefusesUnmarkedLocallyUnlessAllowed(t *testing.T) {
	root, seen := policyServer(t)
	seedServerPolicy(t, root, cliPolicy)
	unmarked := strings.TrimPrefix(cliPolicy, "# owner-edited\n")
	code, _, errOut := runPolicy("set", "--local", writeLocal(t, unmarked))
	if code == 0 || !strings.Contains(errOut, "# owner-edited") {
		t.Fatalf("code %d, stderr %q", code, errOut)
	}
	for _, r := range *seen {
		if strings.HasPrefix(r, "PUT ") {
			t.Fatalf("the refused set still reached the server: %v", *seen)
		}
	}
	if code, _, errOut := runPolicy("set", "--allow-unmarked", "--local", writeLocal(t, unmarked)); code != 0 {
		t.Fatalf("--allow-unmarked failed: %d %s", code, errOut)
	}
	got, _ := os.ReadFile(filepath.Join(root, "System", "model-policy.toml"))
	if string(got) != unmarked {
		t.Fatalf("saved = %q", got)
	}
}

func TestPolicySetSurfacesServerRefusal(t *testing.T) {
	root, _ := policyServer(t)
	seedServerPolicy(t, root, cliPolicy)
	for name, body := range map[string]string{
		"invalid toml": "# owner-edited\n[defaults]\nnot toml at all\n",
		"unknown role": "# owner-edited\n[defaults]\nmodel = \"pool:default\"\n[roles.nonsense]\nmodel = \"pool:default\"\n",
	} {
		code, out, errOut := runPolicy("set", "--local", writeLocal(t, body))
		if code == 0 || out != "" || !strings.Contains(errOut, "not saved") {
			t.Fatalf("%s: code %d stdout %q stderr %q", name, code, out, errOut)
		}
		got, _ := os.ReadFile(filepath.Join(root, "System", "model-policy.toml"))
		if string(got) != cliPolicy {
			t.Fatalf("%s: working file changed: %q", name, got)
		}
	}
}

func TestPolicyRevisionsAndRollback(t *testing.T) {
	root, _ := policyServer(t)
	if code, out, _ := runPolicy("revisions"); code != 0 || !strings.Contains(out, `"revisions": []`) && !strings.Contains(out, `"revisions":[]`) {
		t.Fatalf("empty revisions: %d %q", code, out)
	}
	if code, _, errOut := runPolicy("rollback"); code == 0 || !strings.Contains(errOut, "no kept revision") {
		t.Fatalf("rollback with nothing kept: %d %q", code, errOut)
	}
	v1, v2, v3 := cliPolicy, strings.Replace(cliPolicy, "medium", "high", 1), strings.Replace(cliPolicy, "medium", "low", 1)
	seedServerPolicy(t, root, v1)
	for _, v := range []string{v2, v3} {
		if code, _, errOut := runPolicy("set", "--local", writeLocal(t, v)); code != 0 {
			t.Fatalf("set: %s", errOut)
		}
	}
	var listed struct{ Revisions []string }
	_, out, _ := runPolicy("revisions")
	if err := json.Unmarshal([]byte(out), &listed); err != nil || len(listed.Revisions) != 2 {
		t.Fatalf("revisions = %q (%v)", out, err)
	}
	// Default rollback restores the latest revision: v2, the file v3 replaced.
	if code, _, errOut := runPolicy("rollback"); code != 0 {
		t.Fatalf("rollback: %s", errOut)
	}
	if got, _ := os.ReadFile(filepath.Join(root, "System", "model-policy.toml")); string(got) != v2 {
		t.Fatalf("after rollback = %q, want v2", got)
	}
	// --to names the oldest: v1.
	if code, _, errOut := runPolicy("rollback", "--to", listed.Revisions[0]); code != 0 {
		t.Fatalf("rollback --to: %s", errOut)
	}
	if got, _ := os.ReadFile(filepath.Join(root, "System", "model-policy.toml")); string(got) != v1 {
		t.Fatalf("after rollback --to = %q, want v1", got)
	}
}

func TestPolicyRollbackGoesThroughValidationAndRefusesTraversal(t *testing.T) {
	root, seen := policyServer(t)
	seedServerPolicy(t, root, cliPolicy)
	// A revision that is no longer valid (written around the handler, as an
	// older release might have kept it) stays refused by the validated write.
	dir := filepath.Join(root, "System", "model-policy.revisions")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	bad := "20260101T000000.000000Z.toml"
	if err := os.WriteFile(filepath.Join(dir, bad), []byte("[roles.nonsense]\nmodel = \"x\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, _, errOut := runPolicy("rollback", "--to", bad)
	if code == 0 || !strings.Contains(errOut, "not saved") {
		t.Fatalf("rollback of an invalid revision: %d %q", code, errOut)
	}
	if got, _ := os.ReadFile(filepath.Join(root, "System", "model-policy.toml")); string(got) != cliPolicy {
		t.Fatalf("working file changed: %q", got)
	}
	before := len(*seen)
	for _, to := range []string{"../model-policy.toml", "../../etc/passwd", "a/b.toml", "x.toml", "..%2f..%2fx.toml"} {
		if code, _, errOut := runPolicy("rollback", "--to", to); code == 0 || !strings.Contains(errOut, "revision name") {
			t.Fatalf("--to %q: %d %q", to, code, errOut)
		}
	}
	if len(*seen) != before {
		t.Fatalf("a refused --to reached the server: %v", (*seen)[before:])
	}
}

func TestPolicyGetShowsFileAndRoles(t *testing.T) {
	root, _ := policyServer(t)
	seedServerPolicy(t, root, cliPolicy)
	code, out, errOut := runPolicy("get")
	if code != 0 {
		t.Fatalf("get: %s", errOut)
	}
	var res policyGetResult
	if err := json.Unmarshal([]byte(out), &res); err != nil || res.File != cliPolicy {
		t.Fatalf("get does not return the file: %v %q", err, out)
	}
	for i, role := range policyRoles {
		var got struct{ Role string }
		if err := json.Unmarshal(res.Roles[i], &got); err != nil || got.Role != role {
			t.Fatalf("get lacks the resolve for %s: %s", role, res.Roles[i])
		}
	}
}

func TestPolicyVerifyPostsRole(t *testing.T) {
	policyServer(t)
	code, out, errOut := runPolicy("verify", "--role", "research")
	if code != 0 {
		t.Fatalf("verify: %s", errOut)
	}
	if !strings.Contains(out, `"role": "research"`) && !strings.Contains(out, `"role":"research"`) {
		t.Fatalf("verify did not send the role: %q", out)
	}
}

func TestPolicyCommandsAreMCPToolsAndSetTakesContent(t *testing.T) {
	want := map[string]bool{"policy lead": false, "policy pin": false, "policy get": false, "policy set": false, "policy revisions": false, "policy rollback": false, "policy verify": false}
	for _, cmd := range commands() {
		if _, ok := want[cmd.name()]; ok {
			want[cmd.name()] = true
			if cmd.cliOnly != "" {
				t.Fatalf("%s is CLI-only", cmd.name())
			}
		}
		if cmd.name() == "policy set" {
			props := cmd.inputSchema()["properties"].(map[string]any)
			if _, ok := props["local"]; ok {
				t.Fatal("MCP policy_set offers --local")
			}
			if _, ok := props["content"]; !ok {
				t.Fatal("MCP policy_set lacks content")
			}
		}
	}
	for name, found := range want {
		if !found {
			t.Fatalf("%s is not registered", name)
		}
	}
}
