package platform

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// The capsule catalog fixture (docs/problems/capsule-library-catalog-404-2026-11-03.md).
// A capsule has no route to the catalog: /api/catalog/* is the proxy's,
// relayed unchanged from these corpusd routes. scripts/choir-shot.mjs answers
// it in a capsule from scripts/fixtures/capsule-catalog.json, which this test
// writes from the real handler (CHOIR_WRITE_CAPSULE_CATALOG_FIXTURE=1) and
// otherwise holds to corpusd's answer shape, so the fixture cannot drift.

const capsuleCatalogFixturePath = "../../scripts/fixtures/capsule-catalog.json"

// capsuleCatalogReader is the owner choir-shot signs the shots in as
// (AUTOPUTER_USER in scripts/choir-shot.mjs).
const capsuleCatalogReader = "owner"

type capsuleCatalogFixture struct {
	Fixture  string            `json:"fixture"`
	Note     string            `json:"note"`
	Packages json.RawMessage   `json:"packages"`
	History  []json.RawMessage `json:"history"`
	Releases []json.RawMessage `json:"releases"`
}

func generateCapsuleCatalogFixture(t *testing.T) capsuleCatalogFixture {
	t.Helper()
	w := newPkgWorld(t)
	v1 := w.build("<p>reader v1</p>\n", capsuleCatalogReader, nil)
	w.published(v1)
	v2 := w.build("<p>reader v2</p>\n", capsuleCatalogReader, &v1)
	w.published(v2)
	other := w.build("<p>reader by b</p>\n", "owner-b", nil)
	w.published(other)

	get := func(path string) json.RawMessage {
		t.Helper()
		rec := w.read(packagesRoute + path)
		if rec.Code != 200 {
			t.Fatalf("GET %s = %d %s", path, rec.Code, rec.Body.String())
		}
		return json.RawMessage(strings.TrimSpace(rec.Body.String()))
	}
	owner := "owner_id=" + capsuleCatalogReader
	f := capsuleCatalogFixture{
		Fixture: "capsule-catalog",
		Note: "Not the live catalog. Generated from corpusd's package handler by internal/platform/capsule_catalog_fixture_test.go " +
			"(CHOIR_WRITE_CAPSULE_CATALOG_FIXTURE=1 go test ./internal/platform -run TestCapsuleCatalogFixture); choir-shot serves it in a capsule, labelled.",
		Packages: get("/catalog?" + owner),
		History: []json.RawMessage{
			get("/catalog/reader/releases?" + owner),
			get("/catalog/reader/releases?" + owner + "&track=owner-b"),
		},
	}
	for _, b := range []built{v2, v1, other} {
		f.Releases = append(f.Releases, get("/releases/"+b.id+"?"+owner))
	}
	return f
}

// jsonShape is a value's structure: object keys with their shapes, the union
// of an array's element shapes, and a scalar's type.
func jsonShape(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := map[string]any{}
		for k, e := range x {
			out[k] = jsonShape(e)
		}
		return out
	case []any:
		union := map[string]any{}
		kinds := map[string]bool{}
		for _, e := range x {
			s := jsonShape(e)
			if m, ok := s.(map[string]any); ok {
				for k, ks := range m {
					union[k] = ks
				}
				kinds["object"] = true
				continue
			}
			kinds[s.(string)] = true
		}
		names := make([]string, 0, len(kinds))
		for k := range kinds {
			names = append(names, k)
		}
		sort.Strings(names)
		return map[string]any{"[]": strings.Join(names, "|"), "of": union}
	case string:
		return "string"
	case float64:
		return "number"
	case bool:
		return "bool"
	case nil:
		return "null"
	}
	return "unknown"
}

func shapeOf(t *testing.T, raw any) any {
	t.Helper()
	b, err := json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatal(err)
	}
	return jsonShape(v)
}

func TestCapsuleCatalogFixtureMatchesCorpusd(t *testing.T) {
	fresh := generateCapsuleCatalogFixture(t)
	if os.Getenv("CHOIR_WRITE_CAPSULE_CATALOG_FIXTURE") == "1" {
		out, err := json.MarshalIndent(fresh, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Dir(capsuleCatalogFixturePath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(capsuleCatalogFixturePath, append(out, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("wrote %s", capsuleCatalogFixturePath)
	}
	raw, err := os.ReadFile(capsuleCatalogFixturePath)
	if err != nil {
		t.Fatalf("read the fixture (write it with CHOIR_WRITE_CAPSULE_CATALOG_FIXTURE=1): %v", err)
	}
	var held capsuleCatalogFixture
	if err := json.Unmarshal(raw, &held); err != nil {
		t.Fatalf("the fixture is not JSON: %v", err)
	}
	if held.Fixture != "capsule-catalog" || !strings.HasPrefix(held.Note, "Not the live catalog") {
		t.Fatalf("the fixture must name itself a fixture: fixture=%q note=%q", held.Fixture, held.Note)
	}
	for _, c := range []struct {
		name        string
		held, fresh any
	}{
		{"packages", held.Packages, fresh.Packages},
		{"history", held.History, fresh.History},
		{"releases", held.Releases, fresh.Releases},
	} {
		if got, want := shapeOf(t, c.held), shapeOf(t, c.fresh); !reflect.DeepEqual(got, want) {
			gj, _ := json.Marshal(got)
			wj, _ := json.Marshal(want)
			t.Fatalf("the fixture's %s no longer has corpusd's shape; regenerate with CHOIR_WRITE_CAPSULE_CATALOG_FIXTURE=1\nfixture: %s\ncorpusd: %s", c.name, gj, wj)
		}
	}

	// The fixture is self-consistent: it is signed in as choir-shot's owner,
	// and every head the catalog names is a release it holds.
	var catalog struct {
		Reader   string `json:"reader"`
		Packages []struct {
			Package       string `json:"package"`
			Track         string `json:"track"`
			HeadReleaseID string `json:"head_release_id"`
		} `json:"packages"`
	}
	if err := json.Unmarshal(held.Packages, &catalog); err != nil || catalog.Reader != capsuleCatalogReader || len(catalog.Packages) == 0 {
		t.Fatalf("catalog = %s", held.Packages)
	}
	ids := map[string]bool{}
	for _, r := range held.Releases {
		var view struct {
			ReleaseID string `json:"release_id"`
		}
		_ = json.Unmarshal(r, &view)
		ids[view.ReleaseID] = true
	}
	tracks := map[string]bool{}
	for _, h := range held.History {
		var view struct {
			Package string `json:"package"`
			Track   string `json:"track"`
		}
		_ = json.Unmarshal(h, &view)
		tracks[view.Package+"@"+view.Track] = true
	}
	for _, p := range catalog.Packages {
		if !ids[p.HeadReleaseID] || !tracks[p.Package+"@"+p.Track] {
			t.Fatalf("catalog entry %+v has no release or history in the fixture", p)
		}
	}
}
