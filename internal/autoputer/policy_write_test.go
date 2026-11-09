package autoputer

// The validated, revisioned write of System/model-policy.toml
// (docs/design/paper-control-cli-2026-11-09.md, step 1). Ways it could fail:
//   - invalid TOML is saved anyway, and the loader silently falls back;
//   - a role the desk registry does not know is accepted;
//   - an unknown pool, an unknown key, an unknown effort or a model the
//     catalog does not list is accepted;
//   - a refused write still changes the working file or leaves a revision;
//   - the previous file is not kept as a revision, or a second write in the
//     same instant clobbers the first revision;
//   - the revision bound is not enforced and the directory grows forever;
//   - validation or revision-keeping leaks onto other paths (an overlay, an
//     ordinary file, a file with the same base name elsewhere);
//   - a revision is reachable only by a path that escapes the revisions
//     directory.

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

const goodPolicy = "# owner-edited\n[defaults]\nmodel = \"pool:default\"\nreasoning = \"low\"\n\n[roles.texture]\nmodel = \"pool:default\"\nreasoning = \"medium\"\n"

func putFile(t *testing.T, fh *FilesHandler, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := authBodyRequest(http.MethodPut, "/api/files/"+path, strings.NewReader(body))
	w := httptest.NewRecorder()
	fh.HandleFileByPath(w, req)
	return w
}

func seedPolicy(t *testing.T, root, body string) string {
	t.Helper()
	p := filepath.Join(root, "System", "model-policy.toml")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func revisionNames(t *testing.T, root string) []string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(root, "System", "model-policy.revisions"))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	return names
}

func TestPolicyWriteRefusesInvalidAndKeepsWorkingFile(t *testing.T) {
	cases := map[string]struct{ body, want string }{
		"not key value":   {"[defaults]\nthis is not toml\n", "line 2"},
		"unknown role":    {"[defaults]\nmodel = \"pool:default\"\n[roles.nonsense]\nmodel = \"pool:default\"\n", "nonsense"},
		"unknown pool":    {"[defaults]\nmodel = \"pool:nope\"\n", "nope"},
		"unknown key":     {"[defaults]\nmodel = \"pool:default\"\nmodle = \"x\"\n", "line 3"},
		"unknown effort":  {"[defaults]\nmodel = \"pool:default\"\nreasoning = \"ludicrous\"\n", "ludicrous"},
		"unknown model":   {"[defaults]\nprovider = \"chatgpt\"\nmodel = \"gpt-nonexistent-9\"\n", "gpt-nonexistent-9"},
		"unknown section": {"[bogus]\nmodel = \"x\"\n", "bogus"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			fh, root := setupFileTest(t)
			p := seedPolicy(t, root, goodPolicy)
			w := putFile(t, fh, "System/model-policy.toml", tc.body)
			if w.Code < 400 || w.Code >= 500 {
				t.Fatalf("status = %d, want 4xx; body %s", w.Code, w.Body.String())
			}
			if !strings.Contains(w.Body.String(), tc.want) {
				t.Fatalf("message %q does not name %q", w.Body.String(), tc.want)
			}
			if got, _ := os.ReadFile(p); string(got) != goodPolicy {
				t.Fatalf("a refused write changed the working file: %q", got)
			}
			if names := revisionNames(t, root); len(names) != 0 {
				t.Fatalf("a refused write left revisions: %v", names)
			}
		})
	}
}

func TestPolicyWriteKeepsPreviousAsRevision(t *testing.T) {
	fh, root := setupFileTest(t)
	p := seedPolicy(t, root, goodPolicy)
	next := strings.Replace(goodPolicy, "medium", "high", 1)
	w := putFile(t, fh, "System/model-policy.toml", next)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", w.Code, w.Body.String())
	}
	if got, _ := os.ReadFile(p); string(got) != next {
		t.Fatalf("working file = %q", got)
	}
	names := revisionNames(t, root)
	if len(names) != 1 || !strings.Contains(w.Body.String(), names[0]) {
		t.Fatalf("revisions %v not named in response %s", names, w.Body.String())
	}
	kept, _ := os.ReadFile(filepath.Join(root, "System", "model-policy.revisions", names[0]))
	if string(kept) != goodPolicy {
		t.Fatalf("revision = %q, want the replaced file", kept)
	}

	// A second write keeps the first result without clobbering the first revision.
	third := strings.Replace(goodPolicy, "medium", "low", 1)
	if w := putFile(t, fh, "System/model-policy.toml", third); w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	names = revisionNames(t, root)
	if len(names) != 2 {
		t.Fatalf("revisions = %v, want 2", names)
	}
	var bodies []string
	for _, n := range names {
		b, _ := os.ReadFile(filepath.Join(root, "System", "model-policy.revisions", n))
		bodies = append(bodies, string(b))
	}
	if bodies[0] != goodPolicy || bodies[1] != next {
		t.Fatalf("revision contents = %q", bodies)
	}

	// An identical write changes nothing and keeps nothing.
	if w := putFile(t, fh, "System/model-policy.toml", third); w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	if got := revisionNames(t, root); len(got) != 2 {
		t.Fatalf("an identical write kept a revision: %v", got)
	}
}

func TestPolicyWriteCreatesWithoutRevisionWhenAbsent(t *testing.T) {
	fh, root := setupFileTest(t)
	if err := os.MkdirAll(filepath.Join(root, "System"), 0o755); err != nil {
		t.Fatal(err)
	}
	if w := putFile(t, fh, "System/model-policy.toml", goodPolicy); w.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", w.Code, w.Body.String())
	}
	if names := revisionNames(t, root); len(names) != 0 {
		t.Fatalf("revisions = %v", names)
	}
}

func TestPolicyRevisionsAreBounded(t *testing.T) {
	fh, root := setupFileTest(t)
	seedPolicy(t, root, goodPolicy)
	for i := 0; i < policyRevisionsKept+7; i++ {
		body := goodPolicy + "# edit " + strings.Repeat("x", i+1) + "\n"
		if w := putFile(t, fh, "System/model-policy.toml", body); w.Code != http.StatusOK {
			t.Fatalf("write %d: %d %s", i, w.Code, w.Body.String())
		}
	}
	names := revisionNames(t, root)
	if len(names) != policyRevisionsKept {
		t.Fatalf("revisions = %d, want %d", len(names), policyRevisionsKept)
	}
	// The oldest went: the seed file is no longer among them.
	for _, n := range names {
		b, _ := os.ReadFile(filepath.Join(root, "System", "model-policy.revisions", n))
		if string(b) == goodPolicy {
			t.Fatalf("the oldest revision survived the bound")
		}
	}
}

func TestPolicyValidationAppliesOnlyToThePolicyFile(t *testing.T) {
	fh, root := setupFileTest(t)
	seedPolicy(t, root, goodPolicy)
	if err := os.MkdirAll(filepath.Join(root, "System", "model-policy-overlays"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "Other", "System"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		"notes.txt",
		"System/model-policy-overlays/x.toml",
		"Other/System/model-policy.toml",
		"System/model-policy.toml.bak",
	} {
		if w := putFile(t, fh, path, "this is not a policy\n"); w.Code != http.StatusOK {
			t.Fatalf("%s: status = %d: %s", path, w.Code, w.Body.String())
		}
	}
	if names := revisionNames(t, root); len(names) != 0 {
		t.Fatalf("other paths kept revisions: %v", names)
	}
}

func TestPolicyRevisionIsReadableAndTraversalIsRefused(t *testing.T) {
	fh, root := setupFileTest(t)
	seedPolicy(t, root, goodPolicy)
	putFile(t, fh, "System/model-policy.toml", strings.Replace(goodPolicy, "medium", "high", 1))
	names := revisionNames(t, root)
	req := authRequest(http.MethodGet, "/api/files/System/model-policy.revisions/"+names[0])
	w := httptest.NewRecorder()
	fh.HandleFileByPath(w, req)
	if w.Code != http.StatusOK || w.Body.String() != goodPolicy {
		t.Fatalf("revision read = %d %q", w.Code, w.Body.String())
	}
	req = authRequest(http.MethodGet, "/api/files/System/model-policy.revisions/../../../etc/passwd")
	w = httptest.NewRecorder()
	fh.HandleFileByPath(w, req)
	if w.Code != http.StatusForbidden && w.Code != http.StatusNotFound {
		t.Fatalf("traversal status = %d", w.Code)
	}
}
