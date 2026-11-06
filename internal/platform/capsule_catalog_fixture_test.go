package platform

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/pkgrelease"
	"github.com/yusefmosiah/go-choir/internal/release"
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

// fixtureRelease is one release in the seeded store: the package, the
// publisher's slot, the files it changes (relative to the package root; the
// track's earlier files carry over), its operation's questions, and whether
// the publisher withdrew it afterwards.
type fixtureRelease struct {
	pkg       string
	publisher int
	edits     map[string]string
	questions []release.Precommitment
	withdrawn bool
	testCmd   string
	// media: desktop screenshots, phone screenshots, and whether the
	// release carries the fixture's walkthrough video. A release with
	// none shows the gallery's "no screenshots" state.
	desktop, phone int
	video          bool
}

// fixturePackages are the packages the fixture's store holds, as the base
// platform commit carries them: the files each starts with.
var fixturePackages = map[string]map[string]string{
	"files-next": {
		"App.svelte":          "<script>\n  import FileTree from './FileTree.svelte';\n</script>\n<FileTree />\n",
		"FileTree.svelte":     "<ul class=\"tree\"></ul>\n",
		"files-store.js":      "export const files = [];\n",
		"choir-contract.json": `{"schema": "choir.package-contract/v1", "ready": ".tree"}` + "\n",
	},
	"slides-next": {
		"App.svelte":          "<script>\n  import Deck from './Deck.svelte';\n</script>\n<Deck />\n",
		"Deck.svelte":         "<section class=\"deck\"></section>\n",
		"slide-render.js":     "export function render(slide) { return slide.title; }\n",
		"choir-contract.json": `{"schema": "choir.package-contract/v1", "ready": ".deck"}` + "\n",
	},
	"library-next": {
		"App.svelte":          "<script>\n  import Catalog from './Catalog.svelte';\n</script>\n<Catalog />\n",
		"Catalog.svelte":      "<div class=\"catalog\"></div>\n",
		"catalog-client.js":   "export async function packages() { return []; }\n",
		"choir-contract.json": `{"schema": "choir.package-contract/v1", "ready": ".catalog"}` + "\n",
	},
}

func pc(question string, permille int, settled string) release.Precommitment {
	return release.Precommitment{Kind: "tests_pass", Question: question, PYesPermille: permille, Settled: settled}
}

// fixtureReleases are listed in publish order; each track is a chain.
var fixtureReleases = []fixtureRelease{
	{pkg: "files-next", publisher: 0, edits: map[string]string{
		"FileTree.svelte": "<script>\n  export let nodes = [];\n</script>\n<ul class=\"tree\">\n  {#each nodes as n}<li>{n.name}</li>{/each}\n</ul>\n",
	}, questions: []release.Precommitment{pc("Does the file tree list every file in the home folder?", 880, "yes")}, desktop: 1},
	{pkg: "files-next", publisher: 1, edits: map[string]string{
		"files-store.js": "export const files = [];\nexport function sortByName(list) { return [...list].sort((a, b) => a.name.localeCompare(b.name)); }\n",
	}, questions: []release.Precommitment{pc("Do files sort by name, folders first?", 780, "yes")}, desktop: 1},
	{pkg: "files-next", publisher: 0, edits: map[string]string{
		"RenameDialog.svelte": "<dialog><input aria-label=\"New name\" /></dialog>\n",
		"App.svelte":          "<script>\n  import FileTree from './FileTree.svelte';\n  import RenameDialog from './RenameDialog.svelte';\n</script>\n<FileTree />\n<RenameDialog />\n",
	}, questions: []release.Precommitment{pc("Does renaming a file keep it selected?", 640, "yes"), pc("Does a name clash refuse without losing the typed name?", 700, "")}, testCmd: "npm run check", desktop: 2, phone: 1},
	{pkg: "files-next", publisher: 2, edits: map[string]string{
		"FileTree.svelte": "<ul class=\"tree\" role=\"tree\"></ul>\n",
	}, questions: []release.Precommitment{pc("Is the tree reachable with the keyboard alone?", 560, "no")}},
	{pkg: "files-next", publisher: 1, edits: map[string]string{
		"files-store.js":  "export const files = [];\nexport function sortByName(list) { return [...list].sort((a, b) => a.name.localeCompare(b.name)); }\nexport function sortByModified(list) { return [...list].sort((a, b) => b.modified - a.modified); }\n",
		"FileTree.svelte": "<ul class=\"tree\" data-sort=\"modified\"></ul>\n",
	}, questions: []release.Precommitment{pc("Does the sort order survive a reload?", 720, "")}, desktop: 1},
	{pkg: "files-next", publisher: 0, edits: map[string]string{
		"FileTree.svelte": "<script>\n  export let nodes = [];\n  export let open = new Set();\n</script>\n<ul class=\"tree\">\n  {#each nodes as n}<li class:open={open.has(n.path)}>{n.name}</li>{/each}\n</ul>\n",
		"files-store.js":  "export const files = [];\nexport const expanded = new Set();\n",
	}, questions: []release.Precommitment{pc("Do expanded folders stay expanded after a rename?", 610, "")}, desktop: 2, phone: 1},

	{pkg: "slides-next", publisher: 3, edits: map[string]string{
		"Deck.svelte": "<script>\n  export let slides = [];\n</script>\n<section class=\"deck\">\n  {#each slides as s}<article>{s.title}</article>{/each}\n</section>\n",
	}, questions: []release.Precommitment{pc("Does a deck with forty slides open in under a second?", 700, "yes")}, desktop: 1},
	{pkg: "slides-next", publisher: 4, edits: map[string]string{
		"slide-render.js": "export function render(slide) { return `${slide.title}\\n${slide.notes || ''}`; }\n",
	}, questions: []release.Precommitment{pc("Are speaker notes kept when a slide is reordered?", 820, "yes")}},
	{pkg: "slides-next", publisher: 3, edits: map[string]string{
		"Present.svelte": "<div class=\"present\" tabindex=\"0\"></div>\n",
		"App.svelte":     "<script>\n  import Deck from './Deck.svelte';\n  import Present from './Present.svelte';\n</script>\n<Deck />\n<Present />\n",
	}, questions: []release.Precommitment{pc("Does presenting mode leave with the Escape key?", 900, "yes"), pc("Do the arrow keys move one slide at a time?", 850, "yes")}, desktop: 3, phone: 1},
	{pkg: "slides-next", publisher: 4, edits: map[string]string{
		"Deck.svelte": "<section class=\"deck\" data-layout=\"grid\"></section>\n",
	}, questions: []release.Precommitment{pc("Does the grid layout fit a phone-width window?", 480, "no")}},
	{pkg: "slides-next", publisher: 4, edits: map[string]string{
		"slide-render.js": "export function render(slide) { return `${slide.title}\\n${slide.notes || ''}\\n${slide.footer || ''}`; }\n",
	}, questions: []release.Precommitment{pc("Does the footer print on every slide?", 590, "")}, withdrawn: true},

	{pkg: "library-next", publisher: 0, edits: map[string]string{
		"Catalog.svelte": "<script>\n  export let packages = [];\n</script>\n<div class=\"catalog\">\n  {#each packages as p}<button>{p.package}</button>{/each}\n</div>\n",
	}, questions: []release.Precommitment{pc("Does the catalog list every public package once?", 860, "yes")}, desktop: 1, phone: 1},
	{pkg: "library-next", publisher: 5, edits: map[string]string{
		"catalog-client.js": "export async function packages() {\n  const res = await fetch('/api/catalog/packages');\n  return res.ok ? (await res.json()).packages : [];\n}\n",
	}, questions: []release.Precommitment{pc("Does an unreachable catalog show an empty list instead of an error?", 670, "yes")}},
	{pkg: "library-next", publisher: 0, edits: map[string]string{
		"Catalog.svelte":     "<script>\n  export let packages = [];\n</script>\n<div class=\"catalog\">\n  {#each packages as p}<button>{p.package} by {p.track.slice(0, 8)}</button>{/each}\n</div>\n",
		"ReleasePane.svelte": "<aside class=\"release\"></aside>\n",
	}, questions: []release.Precommitment{pc("Does choosing a release show its changed paths?", 740, "")}, desktop: 2, phone: 1, video: true},
}

// fixtureIDs are random-looking UUIDs from a fixed seed, so the fixture holds
// no real owner, computer or record id and regenerates to the same bytes.
type fixtureIDs struct{ r *rand.Rand }

func (g *fixtureIDs) uuid() string {
	var b [16]byte
	for i := range b {
		b[i] = byte(g.r.Intn(256))
	}
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// fixtureManifest is the reader's manifest renamed to the package.
func fixtureManifest(name string) string {
	title := strings.ToUpper(name[:1]) + name[1:]
	m := strings.ReplaceAll(pkgManifest, `"name": "reader"`, `"name": "`+name+`"`)
	return strings.ReplaceAll(m, `"title": "Reader"`, `"title": "`+title+`"`)
}

func generateCapsuleCatalogFixture(t *testing.T) (capsuleCatalogFixture, map[string][]byte) {
	t.Helper()
	// Git commit dates are part of the base commit id, which a release
	// records; fix them.
	t.Setenv("GIT_AUTHOR_DATE", "2026-11-01T12:00:00Z")
	t.Setenv("GIT_COMMITTER_DATE", "2026-11-01T12:00:00Z")
	ctx := context.Background()
	w := newPkgWorld(t)
	ids := &fixtureIDs{r: rand.New(rand.NewSource(20261103))}

	// The base platform commit carries the three packages.
	base := map[string]string{}
	for name, files := range fixturePackages {
		base[pkgrelease.Root(name)+"/choir-package.json"] = fixtureManifest(name)
		for rel, content := range files {
			base[pkgrelease.Root(name)+"/"+rel] = content
		}
	}
	baseTree := w.fullTree(base)
	pkgGit(t, w.repo, nil, "", "update-ref", "refs/heads/fixture-base", pkgGit(t, w.repo, nil, "", "commit-tree", baseTree, "-p", w.base, "-m", "packages"))
	w.base = pkgGit(t, w.repo, nil, "", "rev-parse", "refs/heads/fixture-base")
	w.h.ConfigurePackageBase(ctx, w.repo)

	// Publisher slots: slot 4 is the reader (it sees its own withdrawn
	// release; others' are private to them), the rest random-looking ids.
	publishers := make([]string, 6)
	for i := range publishers {
		publishers[i] = ids.uuid()
	}
	publishers[4] = capsuleCatalogReader
	computers := make([]string, len(publishers))
	for i := range computers {
		computers[i] = ids.uuid()
	}
	computers[4] = computerOf(capsuleCatalogReader)

	type trackKey struct{ pkg, owner string }
	heads := map[trackKey]*built{}
	files := map[trackKey]map[string]string{}
	var all []built
	var order []trackKey
	seen := map[trackKey]bool{}
	var withdrawn []built
	media := map[string][]byte{}
	for seq, fr := range fixtureReleases {
		owner, computer := publishers[fr.publisher], computers[fr.publisher]
		key := trackKey{fr.pkg, owner}
		if !seen[key] {
			seen[key] = true
			order = append(order, key)
			files[key] = map[string]string{}
		}
		root := pkgrelease.Root(fr.pkg)
		for rel, content := range fr.edits {
			files[key][root+"/"+rel] = content
		}
		full := map[string]string{}
		for p, c := range files[key] {
			full[p] = c
		}
		tree := w.subtree2(w.fullTree(full), root)
		parent := heads[key]
		parentTree := w.subtree2(w.base, root)
		if parent != nil {
			parentTree = parent.record.Source.Tree
		}
		blobs, err := pkgrelease.BuildBlobs(ctx, w.repo, root, parentTree, tree)
		if err != nil {
			t.Fatal(err)
		}
		manifest, version, err := pkgrelease.CheckManifest(fr.pkg, []byte(fixtureManifest(fr.pkg)), pkgrelease.TreeSource(ctx, w.repo, w.fullTree(full), root))
		if err != nil {
			t.Fatal(err)
		}
		changed := []string{}
		for rel := range fr.edits {
			changed = append(changed, root+"/"+rel)
		}
		sort.Strings(changed)
		precommits := []release.Precommitment{}
		for _, p := range fr.questions {
			p.RecordID = "pc-" + ids.uuid()
			if p.Settled != "" {
				p.ResolveRecordID = p.RecordID + ":resolve:rule"
			}
			precommits = append(precommits, p)
		}
		record := pkgrelease.Record{
			Schema: pkgrelease.Schema, Package: pkgrelease.Package{Name: fr.pkg, ManifestVersion: version}, Track: owner,
			Parent: pkgrelease.Parent{Kind: pkgrelease.ParentBase, Commit: w.base, Tree: parentTree},
			Source: pkgrelease.Source{
				Root: root, Tree: tree, TreePatch: *pkgBlob(blobs.TreePatch), ReviewPatch: *pkgBlob(blobs.ReviewPatch),
				ChangedPaths: changed, BuiltOn: w.base,
			},
			Manifest:  manifest,
			Publisher: pkgrelease.Publisher{OwnerID: owner, ComputerID: computer},
			Operation: pkgrelease.Operation{
				OperationID: "op-" + ids.uuid(), BundleDigest: computerevent.DigestBytes([]byte(ids.uuid())),
				DecisionEvent: computerevent.DigestBytes([]byte(ids.uuid())),
			},
			Precommitments: precommits,
			Declarations:   pkgrelease.Declarations{TestCommand: fr.testCmd},
			Media:          w.fixtureReleaseMedia(fr, seq, media),
		}
		if parent != nil {
			record.Parent = pkgrelease.Parent{Kind: pkgrelease.ParentRelease, ReleaseID: parent.id, Tree: parentTree}
			blobs.ParentPatch = nil
		} else {
			record.ParentPatch = pkgBlob(blobs.ParentPatch)
		}
		b := w.seal(record, blobs)
		w.published(b)
		heads[key] = &b
		all = append(all, b)
		if fr.withdrawn {
			withdrawn = append(withdrawn, b)
		}
	}
	for _, b := range withdrawn {
		if rec := w.state(b, b.record.Publisher.OwnerID, pkgrelease.StateWithdrawn); rec.Code != http.StatusOK {
			t.Fatalf("withdraw = %d %s", rec.Code, rec.Body.String())
		}
	}

	// Publish times are the handler's clock; the fixture's are fixed, one
	// release every 41 minutes from the evening the packages were published.
	clock := map[string]string{}
	start := time.Date(2026, 11, 2, 18, 0, 0, 0, time.UTC)
	for i, b := range all {
		clock[b.id] = start.Add(time.Duration(i) * 41 * time.Minute).Format("2006-01-02T15:04:05.000000Z")
	}
	var retime func(v any) any
	retime = func(v any) any {
		switch x := v.(type) {
		case map[string]any:
			if id, ok := x["release_id"].(string); ok {
				if at, ok := clock[id]; ok {
					if _, has := x["published_at"]; has {
						x["published_at"] = at
					}
				}
			}
			for k, e := range x {
				x[k] = retime(e)
			}
		case []any:
			for i, e := range x {
				x[i] = retime(e)
			}
		}
		return v
	}
	get := func(path string) json.RawMessage {
		t.Helper()
		rec := w.read(packagesRoute + path)
		if rec.Code != 200 {
			t.Fatalf("GET %s = %d %s", path, rec.Code, rec.Body.String())
		}
		var v any
		if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
			t.Fatal(err)
		}
		out, err := json.Marshal(retime(v))
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	owner := "owner_id=" + capsuleCatalogReader
	f := capsuleCatalogFixture{
		Fixture: "capsule-catalog",
		Note: "Not the live catalog. Generated from corpusd's package handler by internal/platform/capsule_catalog_fixture_test.go " +
			"(CHOIR_WRITE_CAPSULE_CATALOG_FIXTURE=1 go test ./internal/platform -run TestCapsuleCatalogFixture); choir-shot serves it in a capsule, labelled.",
		Packages: get("/catalog?" + owner),
	}
	for _, key := range order {
		f.History = append(f.History, get("/catalog/"+key.pkg+"/releases?"+owner+"&track="+key.owner))
	}
	for i := len(all) - 1; i >= 0; i-- {
		f.Releases = append(f.Releases, get("/releases/"+all[i].id+"?"+owner))
	}
	return f, media
}

// subtree2 is the subtree of root in a full tree.
func (w *pkgWorld) subtree2(full, root string) string {
	w.t.Helper()
	tree, present, err := pkgrelease.Subtree(context.Background(), w.repo, full, root)
	if err != nil || !present {
		w.t.Fatalf("subtree %s: %v %v", root, present, err)
	}
	return tree
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

func mustMarshal(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
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
	fresh, media := generateCapsuleCatalogFixture(t)
	if os.Getenv("CHOIR_WRITE_CAPSULE_CATALOG_FIXTURE") == "1" {
		writeFixtureMedia(t, media)
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
	// It is realistic: the three packages, each from two or more publisher
	// tracks, and one withdrawn release in a history.
	byPackage := map[string]int{}
	for _, p := range catalog.Packages {
		byPackage[p.Package]++
	}
	for _, name := range []string{"files-next", "slides-next", "library-next"} {
		if byPackage[name] < 2 {
			t.Fatalf("package %s has %d tracks in the fixture, want 2 or more", name, byPackage[name])
		}
	}
	if !strings.Contains(string(mustMarshal(t, held.History)), `"state":"withdrawn"`) {
		t.Fatal("the fixture holds no withdrawn release")
	}
	for _, p := range catalog.Packages {
		if !ids[p.HeadReleaseID] || !tracks[p.Package+"@"+p.Track] {
			t.Fatalf("catalog entry %+v has no release or history in the fixture", p)
		}
	}
	checkFixtureMedia(t, held)
}
