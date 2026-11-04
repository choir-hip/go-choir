package agentcore

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// The capsule install fixture (docs/problems/capsule-library-catalog-404-2026-11-03.md).
// Installing a package release (POST /api/computers/<id>/self-development/adoptions)
// and reading the installed list are the owner's acts on the real computer,
// and a capsule has no route to them. scripts/choir-shot.mjs answers them in a
// capsule from scripts/fixtures/capsule-install.json, labelled, so a Library
// shot can show an install outcome. This test writes the fixture from the
// real install handler (CHOIR_WRITE_CAPSULE_INSTALL_FIXTURE=1) and otherwise
// holds it to the handler's answer shapes, so the fixture cannot drift.

const capsuleInstallFixturePath = "../../scripts/fixtures/capsule-install.json"

type capsuleInstallAnswer struct {
	Status int             `json:"status"`
	Body   json.RawMessage `json:"body"`
}

type capsuleInstallFixture struct {
	Fixture string `json:"fixture"`
	Note    string `json:"note"`
	// RefusalPackage: the earliest release of this package, in the catalog
	// fixture, answers Refusal; every other release is installed.
	RefusalPackage string               `json:"refusal_package"`
	Created        capsuleInstallAnswer `json:"created"`
	Repeat         capsuleInstallAnswer `json:"repeat"`
	List           capsuleInstallAnswer `json:"list"`
	Refusal        capsuleInstallAnswer `json:"refusal"`
}

func generateCapsuleInstallFixture(t *testing.T) capsuleInstallFixture {
	t.Helper()
	const computerID = "computer-b"
	rt, _, _ := releaseTestRuntime(t, computerID)
	repo, base := releaseSourceRepo(t)
	rt.releaseSourceDir = repo
	_, id, raw, treePatch := publishedPackage(t, repo, base, "track-a", map[string]string{
		"choir-package.json": installManifest("Demo"), "App.svelte": "<p>demo</p>\n",
	})
	submitter := &fakePromptSubmitter{}
	rt.SetPromptSubmitter(submitter.submit)
	h := NewAPIHandler(rt)
	adoptions := "/api/computers/" + computerID + "/self-development/adoptions"
	call := func(method string, body []byte) capsuleInstallAnswer {
		w := releaseRouterCall(t, h, method, adoptions, "owner-b", computerID, body)
		return capsuleInstallAnswer{Status: w.Code, Body: json.RawMessage(w.Body.Bytes())}
	}
	body := installBody(t, id, raw, treePatch, "")

	// A name taken: this computer holds its own package of that name.
	taken := t.TempDir()
	releaseGit(t, taken, "clone", "-q", repo, ".")
	dir := filepath.Join(taken, "frontend", "src", "packages", installName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "App.svelte"), []byte("<p>mine</p>\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	releaseGit(t, taken, "add", "-A")
	releaseGit(t, taken, "commit", "-q", "-m", "mine")
	rt.releaseSourceDir = taken
	refusal := call(http.MethodPost, body)
	rt.releaseSourceDir = repo
	if refusal.Status != http.StatusConflict {
		t.Fatalf("refusal = %d %s", refusal.Status, refusal.Body)
	}
	created := call(http.MethodPost, body)
	repeat := call(http.MethodPost, body)
	list := call(http.MethodGet, nil)
	if created.Status != http.StatusCreated || repeat.Status != http.StatusOK || list.Status != http.StatusOK {
		t.Fatalf("created %d, repeat %d, list %d", created.Status, repeat.Status, list.Status)
	}

	// Normalize what the test's temp dirs and clock put in: the staging dir
	// and the request time, and the refusal's detail (git's words, a temp path).
	norm := func(a capsuleInstallAnswer, f func(map[string]any)) capsuleInstallAnswer {
		var v map[string]any
		if err := json.Unmarshal(a.Body, &v); err != nil {
			t.Fatal(err)
		}
		f(v)
		b, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		a.Body = b
		return a
	}
	view := func(v map[string]any) {
		v["staged_dir"] = "/var/lib/choir/updater/adoptions/" + id
		v["requested_at"] = "2026-11-03T03:00:00Z"
	}
	created = norm(created, view)
	repeat = norm(repeat, view)
	list = norm(list, func(v map[string]any) {
		for _, e := range v["adoptions"].([]any) {
			view(e.(map[string]any))
		}
	})
	refusal = norm(refusal, func(v map[string]any) {
		v["error"] = "this computer holds its own package of that name; it is never overwritten"
	})
	return capsuleInstallFixture{
		Fixture: "capsule-install",
		Note: "Not the live computer. Generated from the real install handler by internal/agentcore/capsule_install_fixture_test.go " +
			"(CHOIR_WRITE_CAPSULE_INSTALL_FIXTURE=1 go test ./internal/agentcore -run TestCapsuleInstallFixture); choir-shot serves it in a capsule, labelled. " +
			"Release-specific fields (release_id, adoption_id, publisher and patch fields) are filled from the catalog fixture's release. " +
			"Modelled simply: the owner and times are fixed, a new package (same_base), and a release is installed or refused (package_name_taken) by its package, not by a tree.",
		RefusalPackage: "slides-next",
		Created:        created, Repeat: repeat, List: list, Refusal: refusal,
	}
}

func installShape(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := map[string]any{}
		for k, e := range x {
			out[k] = installShape(e)
		}
		return out
	case []any:
		kinds := []string{}
		for _, e := range x {
			b, _ := json.Marshal(installShape(e))
			kinds = append(kinds, string(b))
		}
		sort.Strings(kinds)
		if len(kinds) > 0 {
			kinds = kinds[:1]
		}
		return kinds
	case string:
		return "string"
	case float64:
		return "number"
	case bool:
		return "bool"
	}
	return "null"
}

func answerShape(t *testing.T, a capsuleInstallAnswer) any {
	t.Helper()
	var v any
	if err := json.Unmarshal(a.Body, &v); err != nil {
		t.Fatalf("answer body is not JSON: %v", err)
	}
	return map[string]any{"status": a.Status, "body": installShape(v)}
}

func TestCapsuleInstallFixtureMatchesAdoptionHandler(t *testing.T) {
	fresh := generateCapsuleInstallFixture(t)
	if os.Getenv("CHOIR_WRITE_CAPSULE_INSTALL_FIXTURE") == "1" {
		out, err := json.MarshalIndent(fresh, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Dir(capsuleInstallFixturePath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(capsuleInstallFixturePath, append(out, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("wrote %s", capsuleInstallFixturePath)
	}
	raw, err := os.ReadFile(capsuleInstallFixturePath)
	if err != nil {
		t.Fatalf("read the fixture (write it with CHOIR_WRITE_CAPSULE_INSTALL_FIXTURE=1): %v", err)
	}
	var held capsuleInstallFixture
	if err := json.Unmarshal(raw, &held); err != nil {
		t.Fatalf("the fixture is not JSON: %v", err)
	}
	if held.Fixture != "capsule-install" || !strings.HasPrefix(held.Note, "Not the live computer") || held.RefusalPackage == "" {
		t.Fatalf("the fixture must name itself a fixture: %q %q", held.Fixture, held.Note)
	}
	for _, c := range []struct {
		name        string
		held, fresh capsuleInstallAnswer
	}{
		{"created", held.Created, fresh.Created}, {"repeat", held.Repeat, fresh.Repeat},
		{"list", held.List, fresh.List}, {"refusal", held.Refusal, fresh.Refusal},
	} {
		if got, want := answerShape(t, c.held), answerShape(t, c.fresh); !reflect.DeepEqual(got, want) {
			gj, _ := json.Marshal(got)
			wj, _ := json.Marshal(want)
			t.Fatalf("the fixture's %s no longer has the adoption handler's shape; regenerate with CHOIR_WRITE_CAPSULE_INSTALL_FIXTURE=1\nfixture: %s\nhandler: %s", c.name, gj, wj)
		}
	}
	var refusal struct {
		Error, Reason string
	}
	_ = json.Unmarshal(held.Refusal.Body, &refusal)
	if refusal.Reason != "package_name_taken" {
		t.Fatalf("refusal = %s", held.Refusal.Body)
	}
}
