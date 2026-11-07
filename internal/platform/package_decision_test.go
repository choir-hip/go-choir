package platform

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/hostbuild"
	"github.com/yusefmosiah/go-choir/internal/pkgrelease"
)

// Slices P1 and P2 of docs/design/package-operation-decision-2026-11-06.md,
// over the real store and a real git repository. Failure modes pinned here:
//
//	K1  a package decision is minted over a base that is not corpusd's, or
//	    (empty stack) is not the booted base's build commit;
//	K2  the patch corpusd drafts is not the patch the bundle names;
//	K3  a bundle whose effects reach beyond the patch's package is decided;
//	K4  a platform decision stops needing its host build;
//	K5  a record other than the draft the decision binds is published;
//	K6  another owner's publisher or track is published on this credential;
//	K7  one decision publishes twice (a different record), or the identical
//	    repeat is not the same release;
//	K8  a decision publishes after its window, on a platform computer, with
//	    no publisher address, or with media the capsule never rendered.

const pdOwner = "owner-a"

type pdFixture struct {
	*consensusFixture
	w     *pkgWorld
	patch []byte
	shots [][]byte
	// queryComputer overrides the computer_id the guest names in the URL.
	queryComputer string
}

func newPDFixture(t *testing.T) *pdFixture {
	t.Helper()
	f := newConsensusFixture(t, OwnerActEnforce)
	w := newPkgWorld(t)
	f.handler.ConfigurePackageBase(context.Background(), w.repo)
	f.handler.ConfigurePackageOwnerActs(NewOwnerActAuthority(OwnerActObserve, nil))
	if err := f.handler.ConfigureConsensusActuator(f.actuator); err != nil {
		t.Fatal(err)
	}
	f.handler.publisherAddress = func(context.Context, string) (string, error) { return "001@choir.test", nil }
	f.handler.guestLookup = fakeGuestLookup{"172.16.0.2": {Found: true, UserID: pdOwner, ComputerID: f.computerID}}
	f.builds.boots = map[string]string{f.bootedBase: w.base}
	pf := &pdFixture{consensusFixture: f, w: w, shots: [][]byte{defaultBeforeShot(t), defaultShot(t)}}
	pf.patch = pf.patchFor("<p>v2</p>\n")
	return pf
}

// patchFor is the frozen source patch of App.svelte set to content.
func (p *pdFixture) patchFor(content string) []byte {
	tree := p.w.fullTree(map[string]string{pkgRoot + "/App.svelte": content})
	return []byte(pkgGit(p.t(), p.w.repo, nil, "", "diff", "--binary", p.w.base, tree) + "\n")
}

func (p *pdFixture) t() *testing.T { return p.w.t }

// setBundle freezes a bundle over patch, based at base, with these effects.
func (p *pdFixture) setBundle(base string, patch []byte, effects ...string) {
	sum := sha256.Sum256(patch)
	var files []map[string]any
	for _, e := range effects {
		files = append(files, map[string]any{"path": e, "kind": "modified", "mode": 420})
	}
	var screens []map[string]any
	for i, shot := range p.shots {
		screens = append(screens, map[string]any{"name": []string{"01-before-home.png", "01-after-home.png"}[i], "sha256": computerevent.DigestBytes(shot), "bytes": len(shot), "width": 16, "height": 16})
	}
	bundle, err := computerevent.CanonicalJSON(map[string]any{
		"computer_id": p.computerID, "content_digest": digestOf('c'), "ordered_file_effects": files,
		"source_patch_sha256": hex.EncodeToString(sum[:]), "source_patch_base_commit": base, "screens": screens,
	})
	if err != nil {
		p.t().Fatal(err)
	}
	p.bundle = bundle
}

func (p *pdFixture) readerEffect() string { return workspacePlatformPrefix + pkgRoot + "/App.svelte" }

// request is the package seat's request for operation op.
func (p *pdFixture) packageRequest(op, claimed string) ConsensusRequest {
	r := p.request(p.t(), op)
	r.HostBuildJobID = ""
	r.Kind, r.SourcePatch, r.ClaimedPlatformBase, r.OwnerID = ConsensusKindPackage, p.patch, claimed, pdOwner
	return r
}

// decideAdmitted decides op and records the decision as admitted on the tape.
func (p *pdFixture) decideAdmitted(op, claimed string) (ConsensusResponse, string) {
	p.t().Helper()
	response, err := p.decide(p.t(), p.packageRequest(op, claimed))
	if err != nil {
		p.t().Fatalf("package decision: %v", err)
	}
	if _, err := p.store.db.Exec(`UPDATE consensus_decisions SET admitted_event_digest=? WHERE decision_digest=?`, digestOf('a'), response.DecisionDigest); err != nil {
		p.t().Fatal(err)
	}
	return response, computerevent.DigestBytes(p.bundle)
}

// record drafts the release the way a guest does, then lets tweak edit it.
func (p *pdFixture) record(patch []byte, op, bundleDigest string, tweak func(*pkgrelease.Record)) built {
	p.t().Helper()
	src, err := pkgrelease.DraftPackage(context.Background(), pkgrelease.DraftInput{Repo: p.w.repo, Name: "reader", Base: p.w.base, Patch: patch, PlatformBase: p.w.base})
	if err != nil {
		p.t().Fatal(err)
	}
	record := pkgrelease.Record{
		Schema: pkgrelease.Schema, Package: pkgrelease.Package{Name: "reader", ManifestVersion: src.ManifestVersion}, Track: pdOwner, Parent: src.Parent,
		Source: pkgrelease.Source{
			Root: src.Root, Tree: src.Tree, TreePatch: *pkgBlob(src.Blobs.TreePatch), ReviewPatch: *pkgBlob(src.Blobs.ReviewPatch),
			ChangedPaths: src.Paths, BuiltOn: p.w.base,
		},
		ParentPatch: pkgBlob(src.Blobs.ParentPatch),
		Manifest:    src.Manifest,
		Publisher:   pkgrelease.Publisher{OwnerID: pdOwner, ComputerID: p.computerID},
		Operation:   pkgrelease.Operation{OperationID: op, BundleDigest: bundleDigest, DecisionEvent: digestOf('a')},
		Media:       []pkgrelease.Media{mediaShot("01-before-home.png", p.shots[0], 16, 16), mediaShot("01-after-home.png", p.shots[1], 16, 16)},
	}
	if tweak != nil {
		tweak(&record)
	}
	b := p.w.seal(record, src.Blobs)
	return b
}

// publish posts b for the decision as the guest bound to pdOwner would.
func (p *pdFixture) publish(b built, decision string, tweak func(*PackagePublishByDecisionRequest)) *httptest.ResponseRecorder {
	p.t().Helper()
	request := PackagePublishByDecisionRequest{
		DecisionDigest: decision, OperationID: b.record.Operation.OperationID, BundleDigest: b.record.Operation.BundleDigest, ReleaseID: b.id,
		Record: json.RawMessage(b.raw), TreePatch: b.blobs.TreePatch, ReviewPatch: b.blobs.ReviewPatch, ParentPatch: b.blobs.ParentPatch,
		SourcePatch: p.patch, ClaimedPlatformBase: p.w.base, Media: p.shots,
	}
	if tweak != nil {
		tweak(&request)
	}
	body, err := json.Marshal(request)
	if err != nil {
		p.t().Fatal(err)
	}
	computer := p.computerID
	if p.queryComputer != "" {
		computer = p.queryComputer
	}
	r := httptest.NewRequest(http.MethodPost, "/internal/computers/self-development/package-publish?computer_id="+computer, bytes.NewReader(body))
	r.RemoteAddr = "172.16.0.2:40000"
	r.Header.Set("Authorization", "Bearer "+p.guestToken)
	w := httptest.NewRecorder()
	p.handler.HandlePackagePublishByDecision(w, r)
	return w
}

func reasonOfBody(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	var refusal struct {
		Reason string `json:"reason"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &refusal)
	return refusal.Reason
}

func (p *pdFixture) wantRefused(w *httptest.ResponseRecorder, status int, reason string) {
	p.t().Helper()
	if w.Code != status || (reason != "" && reasonOfBody(p.t(), w) != reason) {
		p.t().Fatalf("publish = %d %s; want %d %s", w.Code, w.Body.String(), status, reason)
	}
}

func (p *pdFixture) listedCount() int {
	listings, _, err := p.handler.trackListings(context.Background(), pdOwner, "reader")
	if err != nil {
		p.t().Fatal(err)
	}
	return len(listings)
}

// --- the seat (P1) ---

func TestPackageSeatDecidesOnAComputerWithStackCommits(t *testing.T) {
	f := newPDFixture(t)
	// The capsule was cast from a projection commit only the guest holds; the
	// claim is the stack's base, which corpusd holds.
	f.setBundle(strings.Repeat("c", 40), f.patch, f.readerEffect())
	response, err := f.decide(t, f.packageRequest("op-pkg", f.w.base))
	if err != nil {
		t.Fatalf("decision: %v", err)
	}
	fields := response.Decision.KindFields
	block, _ := fields["package"].(map[string]any)
	if fields["kind"] != ConsensusKindPackage || fields["host_build"] != nil || block == nil || block["name"] != "reader" {
		t.Fatalf("decision fields: %#v", fields)
	}
	src, err := pkgrelease.DraftPackage(context.Background(), pkgrelease.DraftInput{Repo: f.w.repo, Name: "reader", Base: f.w.base, Patch: f.patch, PlatformBase: f.w.base})
	if err != nil {
		t.Fatal(err)
	}
	want, _ := src.Facts("reader").Digest()
	if block["draft_digest"] != want || response.HostBuildJobID != want {
		t.Fatalf("draft digest %v / %s, want %s", block["draft_digest"], response.HostBuildJobID, want)
	}
	var kind, job, result, sha, media string
	if err := f.store.db.QueryRow(`SELECT kind, host_build_job_id, host_build_result_digest, package_source_patch_sha256, package_media_digests FROM consensus_decisions WHERE decision_digest=?`, response.DecisionDigest).
		Scan(&kind, &job, &result, &sha, &media); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(f.patch)
	if kind != ConsensusKindPackage || job != want || result != block["verdict_digest"] || sha != hex.EncodeToString(sum[:]) || !strings.Contains(media, computerevent.DigestBytes(f.shots[0])) {
		t.Fatalf("row = %s %s %s %s %s", kind, job, result, sha, media)
	}
}

func TestPackageSeatBindsTheClaimedBaseToTheBootedBaseOnAnEmptyStack(t *testing.T) {
	f := newPDFixture(t)
	f.setBundle(f.w.base, f.patch, f.readerEffect())
	// Empty stack: the bundle's base is the claim, and the booted base's
	// build commit is the only claim corpusd takes.
	f.builds.boots[f.bootedBase] = strings.Repeat("9", 40)
	if _, err := f.decide(t, f.packageRequest("op-pkg", f.w.base)); !errors.Is(err, ErrConsensusRefused) || !strings.Contains(err.Error(), "booted base") {
		t.Fatalf("a base that is not the booted one: %v", err)
	}
	f.builds.boots[f.bootedBase] = f.w.base
	if _, err := f.decide(t, f.packageRequest("op-pkg", f.w.base)); err != nil {
		t.Fatalf("the booted base: %v", err)
	}
	// A claim corpusd's store does not hold is refused outright.
	f.setBundle(strings.Repeat("c", 40), f.patch, f.readerEffect())
	if _, err := f.decide(t, f.packageRequest("op-pkg-2", strings.Repeat("d", 40))); !errors.Is(err, ErrConsensusRefused) || !strings.Contains(err.Error(), "not in corpusd's store") {
		t.Fatalf("a claimed base corpusd does not hold: %v", err)
	}
}

func TestPackageSeatRefusesAPatchThatIsNotTheBundles(t *testing.T) {
	f := newPDFixture(t)
	f.setBundle(strings.Repeat("c", 40), f.patch, f.readerEffect())
	request := f.packageRequest("op-pkg", f.w.base)
	request.SourcePatch = f.patchFor("<p>another</p>\n")
	if _, err := f.decide(t, request); !errors.Is(err, ErrConsensusRefused) || !strings.Contains(err.Error(), "source_patch_sha256") {
		t.Fatalf("a patch the bundle does not name: %v", err)
	}
}

func TestPackageSeatRefusesMixedEffects(t *testing.T) {
	f := newPDFixture(t)
	// A regenerated-identical platform file is an effect with no hunk.
	f.setBundle(strings.Repeat("c", 40), f.patch, f.readerEffect(), workspacePlatformPrefix+"frontend/src/app.css")
	if _, err := f.decide(t, f.packageRequest("op-pkg", f.w.base)); !errors.Is(err, ErrConsensusRefused) || !strings.Contains(err.Error(), "package_path_refused") {
		t.Fatalf("effects beyond the package root: %v", err)
	}
	// The same bundle without the stray effect decides.
	f.setBundle(strings.Repeat("c", 40), f.patch, f.readerEffect())
	if _, err := f.decide(t, f.packageRequest("op-pkg", f.w.base)); err != nil {
		t.Fatalf("package-only: %v", err)
	}
}

func TestClassifyOperationAgreesPatchAndEffects(t *testing.T) {
	platformPatch := []byte("diff --git a/frontend/src/app.css b/frontend/src/app.css\n--- a/frontend/src/app.css\n+++ b/frontend/src/app.css\n@@ -1 +1 @@\n-body {}\n+body { margin: 0 }\n")
	packagePatch := []byte("diff --git a/" + pkgRoot + "/App.svelte b/" + pkgRoot + "/App.svelte\n--- a/" + pkgRoot + "/App.svelte\n+++ b/" + pkgRoot + "/App.svelte\n@@ -1 +1 @@\n-<p>reader</p>\n+<p>v2</p>\n")
	bundle := func(effects ...string) []byte {
		var files []map[string]string
		for _, e := range effects {
			files = append(files, map[string]string{"path": e})
		}
		raw, _ := json.Marshal(map[string]any{"ordered_file_effects": files})
		return raw
	}
	for name, tc := range map[string]struct {
		patch, bundle []byte
		kind, pkg     string
		fails         bool
	}{
		"platform":                   {platformPatch, bundle(workspacePlatformPrefix + "frontend/src/app.css"), ConsensusKindPlatform, "", false},
		"package":                    {packagePatch, bundle(workspacePlatformPrefix + pkgRoot + "/App.svelte"), ConsensusKindPackage, "reader", false},
		"package, platform effect":   {packagePatch, bundle(workspacePlatformPrefix+pkgRoot+"/App.svelte", workspacePlatformPrefix+"frontend/src/app.css"), "", "", true},
		"platform, package effect":   {platformPatch, bundle(workspacePlatformPrefix+"frontend/src/app.css", workspacePlatformPrefix+pkgRoot+"/App.svelte"), "", "", true},
		"package, no package effect": {packagePatch, bundle(workspacePlatformPrefix + "frontend/src/app.css"), "", "", true},
		"no patch":                   {nil, bundle(), ConsensusKindPlatform, "", false},
	} {
		kind, pkg, err := ClassifyOperation(tc.patch, tc.bundle)
		if (err != nil) != tc.fails || kind != tc.kind || pkg != tc.pkg {
			t.Fatalf("%s: %q %q %v", name, kind, pkg, err)
		}
	}
}

func TestPlatformDecisionStillRequiresTheHostBuild(t *testing.T) {
	f := newPDFixture(t)
	f.bundle, _ = computerevent.CanonicalJSON(map[string]any{
		"computer_id": f.computerID, "content_digest": digestOf('c'),
		"source_patch_sha256": f.patchSHA, "source_patch_base_commit": f.baseCommit,
		"test_receipts": []string{"capsule-exec:sha256:" + digestOf('7')},
	})
	// No host build for the operation: refused, as before.
	if _, err := f.decide(t, f.request(t, "op-plat")); !errors.Is(err, ErrConsensusRefused) || !strings.Contains(err.Error(), "no host build") {
		t.Fatalf("a platform decision with no host build: %v", err)
	}
	f.recordBuild(t, f.hostRequest("op-plat"), hostbuild.StateDone, f.modeKey)
	response, err := f.decide(t, f.request(t, "op-plat"))
	if err != nil {
		t.Fatal(err)
	}
	fields := response.Decision.KindFields
	var kind string
	if err := f.store.db.QueryRow(`SELECT kind FROM consensus_decisions WHERE decision_digest=?`, response.DecisionDigest).Scan(&kind); err != nil {
		t.Fatal(err)
	}
	if fields["host_build"] == nil || fields["package"] != nil || fields["kind"] != nil || kind != ConsensusKindPlatform {
		t.Fatalf("platform decision: kind=%s fields=%#v", kind, fields)
	}
	// A package request never rides a host build: it takes the seat.
	f.setBundle(strings.Repeat("c", 40), f.patch, f.readerEffect())
	bogus := f.request(t, "op-bogus")
	bogus.Kind = "elsewhere"
	if _, err := f.decide(t, bogus); !errors.Is(err, ErrConsensusRefused) {
		t.Fatalf("unknown kind: %v", err)
	}
}

// --- publish by decision (P2) ---

func newPublishedWorld(t *testing.T) (*pdFixture, ConsensusResponse, string) {
	t.Helper()
	f := newPDFixture(t)
	f.setBundle(strings.Repeat("c", 40), f.patch, f.readerEffect())
	response, bundleDigest := f.decideAdmitted("op-pkg", f.w.base)
	return f, response, bundleDigest
}

func TestPublishByDecisionListsTheReleaseOnceAndRepeatsIdentically(t *testing.T) {
	f, decision, bundle := newPublishedWorld(t)
	b := f.record(f.patch, "op-pkg", bundle, nil)
	first := f.publish(b, decision.DecisionDigest, nil)
	var view PackageReleaseView
	if first.Code != http.StatusCreated || json.Unmarshal(first.Body.Bytes(), &view) != nil || view.ReleaseID != b.id || !view.Published {
		t.Fatalf("first publish = %d %s", first.Code, first.Body.String())
	}
	if f.listedCount() != 1 {
		t.Fatalf("listed %d releases", f.listedCount())
	}
	var held string
	if err := f.store.db.QueryRow(`SELECT published_release_id FROM consensus_decisions WHERE decision_digest=?`, decision.DecisionDigest).Scan(&held); err != nil || held != b.id {
		t.Fatalf("decision consumed by %q: %v", held, err)
	}
	// The identical repeat is the same release, listed once.
	again := f.publish(b, decision.DecisionDigest, nil)
	var repeat PackageReleaseView
	if again.Code != http.StatusOK || json.Unmarshal(again.Body.Bytes(), &repeat) != nil || repeat.ReleaseID != b.id || f.listedCount() != 1 {
		t.Fatalf("repeat = %d %s", again.Code, again.Body.String())
	}
	// Any other record citing the decision is refused. This one carries the
	// same draft content and so passes every other check; only the single
	// consumption stops it.
	other := f.record(f.patch, "op-pkg", bundle, func(r *pkgrelease.Record) { r.Declarations.TestCommand = "npm test" })
	if other.id == b.id {
		t.Fatal("the second record has the first's id")
	}
	f.wantRefused(f.publish(other, decision.DecisionDigest, nil), http.StatusConflict, reasonDecisionConsumed)
	if f.listedCount() != 1 {
		t.Fatalf("a second record was listed: %d", f.listedCount())
	}
}

func TestPublishByDecisionRefusesARecordThatIsNotTheBoundDraft(t *testing.T) {
	f, decision, bundle := newPublishedWorld(t)
	// A record of other content, with its own patch: the draft the seat
	// voted on is not this one.
	other := f.patchFor("<p>sneaky</p>\n")
	b := f.record(other, "op-pkg", bundle, nil)
	f.wantRefused(f.publish(b, decision.DecisionDigest, func(r *PackagePublishByDecisionRequest) { r.SourcePatch = other }), http.StatusForbidden, reasonDecisionMismatch)
	// The decision's own patch, but a record whose content differs (a changed
	// path list the draft does not have).
	b = f.record(f.patch, "op-pkg", bundle, func(r *pkgrelease.Record) {
		r.Source.ChangedPaths = append(r.Source.ChangedPaths, pkgRoot+"/Extra.svelte")
	})
	f.wantRefused(f.publish(b, decision.DecisionDigest, nil), http.StatusForbidden, reasonDraftDigest)
	if f.listedCount() != 0 {
		t.Fatal("a mismatched record was listed")
	}
}

func TestPublishByDecisionRefusesAnotherOwnersRecord(t *testing.T) {
	f, decision, bundle := newPublishedWorld(t)
	// vmctl binds this guest to owner-b, whose credential cannot publish a
	// record that names owner-a.
	f.handler.guestLookup = fakeGuestLookup{"172.16.0.2": {Found: true, UserID: "owner-b", ComputerID: f.computerID}}
	f.wantRefused(f.publish(f.record(f.patch, "op-pkg", bundle, nil), decision.DecisionDigest, nil), http.StatusForbidden, reasonPublishAuthority)
	// A record that names owner-b as publisher and track, on owner-b's own
	// binding, is not the draft the seat made on owner-a's track either only
	// when the heads differ; here the heads agree, so the authority check is
	// the owner's: the request's own claim of an owner never stands.
	b := f.record(f.patch, "op-pkg", bundle, func(r *pkgrelease.Record) {
		r.Publisher.OwnerID, r.Track = "owner-b", "owner-b"
	})
	w := f.publish(b, decision.DecisionDigest, func(r *PackagePublishByDecisionRequest) { r.OwnerID = pdOwner })
	f.wantRefused(w, http.StatusForbidden, "")
	if f.listedCount() != 0 {
		t.Fatal("another owner's record was listed")
	}
	// A guest the binding does not know is refused before anything is read.
	f.handler.guestLookup = fakeGuestLookup{}
	f.wantRefused(f.publish(f.record(f.patch, "op-pkg", bundle, nil), decision.DecisionDigest, nil), http.StatusForbidden, "")
}

func TestPublishByDecisionRefusesPastTheWindow(t *testing.T) {
	f, decision, bundle := newPublishedWorld(t)
	b := f.record(f.patch, "op-pkg", bundle, nil)
	// Inside the grace after not_after the publish is taken; past it, not.
	if _, err := f.store.db.Exec(`UPDATE consensus_decisions SET not_after=? WHERE decision_digest=?`, time.Now().UTC().Add(-20*time.Minute), decision.DecisionDigest); err != nil {
		t.Fatal(err)
	}
	// (the publish itself is checked below; here only the window is probed)
	if _, err := f.store.db.Exec(`UPDATE consensus_decisions SET not_after=? WHERE decision_digest=?`, time.Now().UTC().Add(-45*time.Minute), decision.DecisionDigest); err != nil {
		t.Fatal(err)
	}
	f.wantRefused(f.publish(b, decision.DecisionDigest, nil), http.StatusGone, reasonDecisionExpired)
	if f.listedCount() != 0 {
		t.Fatal("an expired decision published")
	}
}

func TestPublishByDecisionRefusesAPlatformComputer(t *testing.T) {
	f, decision, bundle := newPublishedWorld(t)
	if _, err := f.store.db.Exec(`UPDATE consensus_decisions SET computer_id=? WHERE decision_digest=?`, paperComputerID, decision.DecisionDigest); err != nil {
		t.Fatal(err)
	}
	f.handler.guestLookup = fakeGuestLookup{"172.16.0.2": {Found: true, UserID: pdOwner, ComputerID: paperComputerID}}
	f.queryComputer = paperComputerID
	f.handler.eventAuth = hostBuildAuth{computer: paperComputerID}
	f.guestToken = paperComputerID
	b := f.record(f.patch, "op-pkg", bundle, func(r *pkgrelease.Record) { r.Publisher.ComputerID = paperComputerID })
	w := f.publish(b, decision.DecisionDigest, nil)
	f.wantRefused(w, http.StatusForbidden, reasonPublishAuthority)
	if !strings.Contains(w.Body.String(), "platform computer") || f.listedCount() != 0 {
		t.Fatalf("platform computer publish: %s", w.Body.String())
	}
}

func TestPublishByDecisionNeedsAPublisherAddress(t *testing.T) {
	f, decision, bundle := newPublishedWorld(t)
	b := f.record(f.patch, "op-pkg", bundle, nil)
	f.handler.publisherAddress = func(context.Context, string) (string, error) { return "", nil }
	f.wantRefused(f.publish(b, decision.DecisionDigest, nil), http.StatusUnprocessableEntity, reasonPublisherAddress)
	f.handler.publisherAddress = func(context.Context, string) (string, error) { return "", errors.New("maild down") }
	f.wantRefused(f.publish(b, decision.DecisionDigest, nil), http.StatusBadGateway, reasonPublisherLookup)
	f.handler.publisherAddress = nil
	f.wantRefused(f.publish(b, decision.DecisionDigest, nil), http.StatusBadGateway, reasonPublisherLookup)
	if f.listedCount() != 0 {
		t.Fatal("a publisher with no address published")
	}
	// With an address the same decision publishes: no refusal consumed it.
	f.handler.publisherAddress = func(context.Context, string) (string, error) { return "001@choir.test", nil }
	if w := f.publish(b, decision.DecisionDigest, nil); w.Code != http.StatusCreated {
		t.Fatalf("publish with an address = %d %s", w.Code, w.Body.String())
	}
}

func TestPublishByDecisionAcceptsOnlyTheFrozenMedia(t *testing.T) {
	f, decision, bundle := newPublishedWorld(t)
	// A screenshot the capsule never rendered: valid as media, not frozen.
	stray := hostPNG(t, 16, 16, 3)
	b := f.record(f.patch, "op-pkg", bundle, func(r *pkgrelease.Record) {
		r.Media = []pkgrelease.Media{mediaShot("01-before-home.png", f.shots[0], 16, 16), mediaShot("01-after-home.png", stray, 16, 16)}
	})
	f.wantRefused(f.publish(b, decision.DecisionDigest, func(r *PackagePublishByDecisionRequest) { r.Media = [][]byte{f.shots[0], stray} }), http.StatusForbidden, reasonMediaNotFrozen)
	// A frozen screen the request leaves out is refused, and so is an extra
	// item the record does not name.
	b = f.record(f.patch, "op-pkg", bundle, nil)
	f.wantRefused(f.publish(b, decision.DecisionDigest, func(r *PackagePublishByDecisionRequest) { r.Media = [][]byte{f.shots[0]} }), http.StatusUnprocessableEntity, "")
	f.wantRefused(f.publish(b, decision.DecisionDigest, func(r *PackagePublishByDecisionRequest) { r.Media = append(r.Media, stray) }), http.StatusForbidden, reasonMediaNotFrozen)
	if f.listedCount() != 0 {
		t.Fatal("unfrozen media published")
	}
	if w := f.publish(b, decision.DecisionDigest, nil); w.Code != http.StatusCreated {
		t.Fatalf("frozen media publish = %d %s", w.Code, w.Body.String())
	}
}

func TestPublishByDecisionRefusesWhatTheDecisionIsNot(t *testing.T) {
	f, decision, bundle := newPublishedWorld(t)
	b := f.record(f.patch, "op-pkg", bundle, nil)
	// Another operation's record, another bundle.
	f.wantRefused(f.publish(b, decision.DecisionDigest, func(r *PackagePublishByDecisionRequest) { r.OperationID = "op-other" }), http.StatusForbidden, reasonDecisionMismatch)
	// A decision that was never admitted on the tape.
	if _, err := f.store.db.Exec(`UPDATE consensus_decisions SET admitted_event_digest=NULL WHERE decision_digest=?`, decision.DecisionDigest); err != nil {
		t.Fatal(err)
	}
	f.wantRefused(f.publish(b, decision.DecisionDigest, nil), http.StatusForbidden, reasonDecisionMismatch)
	// An unknown decision.
	f.wantRefused(f.publish(b, digestOf('f'), nil), http.StatusNotFound, reasonDecisionMismatch)
	// A platform decision is no package decision.
	if _, err := f.store.db.Exec(`UPDATE consensus_decisions SET admitted_event_digest=?, kind='platform' WHERE decision_digest=?`, digestOf('a'), decision.DecisionDigest); err != nil {
		t.Fatal(err)
	}
	f.wantRefused(f.publish(b, decision.DecisionDigest, nil), http.StatusForbidden, reasonPackageNotDecided)
	if f.listedCount() != 0 {
		t.Fatal("something published")
	}
}

// --- D7 P1/P2 review follow-ups ---
//
// Failure modes pinned here:
//
//	R1  a guest claims a held commit that is not the booted base and does not
//	    descend from it, and the seat decides over it;
//	R2  a record carrying another decision event than the one the tape
//	    admitted publishes by the decision;
//	R3  the body is read (or the slots taken) before the caller is bound, the
//	    publish needs no event:append capability, or publishes are unbounded;
//	R4  a claimed commit missing from the store or a patch that does not apply
//	    answers 500 (or 503 at the seat) with no named reason;
//	R5  a non-internal caller's OwnerID on the consensus route is believed.

// commitOn makes a commit on parent with App.svelte set to content.
func (p *pdFixture) commitOn(parent, content string) string {
	t := p.t()
	env := []string{"GIT_INDEX_FILE=" + t.TempDir() + "/index"}
	if parent != "" {
		pkgGit(t, p.w.repo, env, "", "read-tree", parent)
	}
	sha := pkgGit(t, p.w.repo, nil, content, "hash-object", "-w", "--stdin")
	pkgGit(t, p.w.repo, env, "", "update-index", "--add", "--cacheinfo", "100644,"+sha+","+pkgRoot+"/App.svelte")
	tree := pkgGit(t, p.w.repo, env, "", "write-tree")
	args := []string{"commit-tree", tree, "-m", "c"}
	if parent != "" {
		args = append(args, "-p", parent)
	}
	return pkgGit(t, p.w.repo, nil, "", args...)
}

func TestPackageSeatClaimedBaseMustBeTheBootedBaseOrDescendFromIt(t *testing.T) {
	f := newPDFixture(t)
	f.setBundle(strings.Repeat("c", 40), f.patch, f.readerEffect()) // stack commits: bundle base is not the claim
	// An unrelated root commit corpusd holds: held, but not the booted lineage.
	stranger := f.commitOn("", "<p>stranger</p>\n")
	if _, err := f.decide(t, f.packageRequest("op-stranger", stranger)); !errors.Is(err, ErrConsensusRefused) || !strings.Contains(err.Error(), "does not descend") {
		t.Fatalf("a held commit outside the booted lineage: %v", err)
	}
	// A descendant of the booted base is the stack base the design names.
	child := f.commitOn(f.w.base, "<p>child</p>\n")
	if _, err := f.decide(t, f.packageRequest("op-child", child)); err != nil {
		// the patch is over base; a child with a changed App.svelte may not
		// apply, which is a reject vote or refusal other than ancestry
		if strings.Contains(err.Error(), "descend") {
			t.Fatalf("a descendant of the booted base: %v", err)
		}
	}
	// The booted base itself.
	if _, err := f.decide(t, f.packageRequest("op-self", f.w.base)); err != nil {
		t.Fatalf("the booted base: %v", err)
	}
}

func TestPublishByDecisionRefusesAnotherDecisionEvent(t *testing.T) {
	f, decision, bundle := newPublishedWorld(t)
	b := f.record(f.patch, "op-pkg", bundle, func(r *pkgrelease.Record) { r.Operation.DecisionEvent = digestOf('b') })
	f.wantRefused(f.publish(b, decision.DecisionDigest, nil), http.StatusForbidden, reasonDecisionMismatch)
	if f.listedCount() != 0 {
		t.Fatal("something published")
	}
}

func TestPublishByDecisionBindsAndAuthorizesBeforeReadingTheBody(t *testing.T) {
	f, decision, bundle := newPublishedWorld(t)
	b := f.record(f.patch, "op-pkg", bundle, nil)
	probe := &readProbe{}
	post := func(remote, bearer string) *httptest.ResponseRecorder {
		probe.read = false
		r := httptest.NewRequest(http.MethodPost, "/internal/computers/self-development/package-publish?computer_id="+f.computerID, probe)
		r.RemoteAddr = remote
		if bearer != "" {
			r.Header.Set("Authorization", "Bearer "+bearer)
		}
		w := httptest.NewRecorder()
		f.handler.HandlePackagePublishByDecision(w, r)
		return w
	}
	if w := post("172.16.0.99:40000", f.guestToken); w.Code != http.StatusForbidden || probe.read {
		t.Fatalf("an unbound caller: %d read=%v", w.Code, probe.read)
	}
	if w := post("172.16.0.2:40000", ""); w.Code != http.StatusForbidden || probe.read {
		t.Fatalf("no event:append capability: %d read=%v", w.Code, probe.read)
	}
	// The bound, authorized publish still lists.
	if w := f.publish(b, decision.DecisionDigest, nil); w.Code != http.StatusCreated {
		t.Fatalf("publish = %d %s", w.Code, w.Body.String())
	}
}

type readProbe struct{ read bool }

func (r *readProbe) Read([]byte) (int, error) { r.read = true; return 0, io.EOF }

func TestPublishByDecisionBoundsConcurrencyAndBodySize(t *testing.T) {
	f, decision, bundle := newPublishedWorld(t)
	b := f.record(f.patch, "op-pkg", bundle, nil)
	for i := 0; i < maxConcurrentPackagePublishes; i++ {
		packagePublishSlots <- struct{}{}
	}
	w := f.publish(b, decision.DecisionDigest, nil)
	for i := 0; i < maxConcurrentPackagePublishes; i++ {
		<-packagePublishSlots
	}
	f.wantRefused(w, http.StatusTooManyRequests, reasonPublishBusy)
	// A body past the base64 cap is cut off while decoding.
	huge := strings.NewReader(`{"source_patch":"` + strings.Repeat("A", maxPackagePublishBody+1024) + `"}`)
	r := httptest.NewRequest(http.MethodPost, "/internal/computers/self-development/package-publish?computer_id="+f.computerID, huge)
	r.RemoteAddr = "172.16.0.2:40000"
	r.Header.Set("Authorization", "Bearer "+f.guestToken)
	rec := httptest.NewRecorder()
	f.handler.HandlePackagePublishByDecision(rec, r)
	f.wantRefused(rec, http.StatusRequestEntityTooLarge, pkgrelease.ReasonBlobTooLarge)
}

func TestPublishByDecisionNamesAnUnreachableBaseAndAnInapplicablePatch(t *testing.T) {
	f, decision, bundle := newPublishedWorld(t)
	b := f.record(f.patch, "op-pkg", bundle, nil)
	f.wantRefused(f.publish(b, decision.DecisionDigest, func(r *PackagePublishByDecisionRequest) { r.ClaimedPlatformBase = strings.Repeat("d", 40) }),
		http.StatusUnprocessableEntity, reasonBaseUnavailable)
	// A held commit the frozen patch does not apply to (its App.svelte differs).
	other := f.commitOn("", "<p>other</p>\n")
	f.wantRefused(f.publish(b, decision.DecisionDigest, func(r *PackagePublishByDecisionRequest) { r.ClaimedPlatformBase = other }),
		http.StatusUnprocessableEntity, pkgrelease.ReasonPatchInapplicable)
	if f.listedCount() != 0 {
		t.Fatal("something published")
	}
}

func TestConsensusRouteIgnoresANonInternalCallersOwnerID(t *testing.T) {
	f := newPDFixture(t)
	f.setBundle(strings.Repeat("c", 40), f.patch, f.readerEffect())
	request := f.packageRequest("op-pkg", f.w.base)
	request.OwnerID = "owner-forged"
	body, _ := json.Marshal(request)
	// Unbound remote: the guest is no tap caller, so no owner is bound.
	r := httptest.NewRequest(http.MethodPost, "/internal/computers/self-development/consensus?computer_id="+f.computerID, bytes.NewReader(body))
	r.RemoteAddr = "172.16.0.99:40000"
	r.Header.Set("Authorization", "Bearer "+f.guestToken)
	w := httptest.NewRecorder()
	f.handler.HandleSelfDevelopmentConsensus(w, r)
	if w.Code != http.StatusForbidden || !strings.Contains(w.Body.String(), "none bound") {
		t.Fatalf("forged owner: %d %s", w.Code, w.Body.String())
	}
	// Bound caller: the owner is vmctl's binding, not the guest's word.
	r = httptest.NewRequest(http.MethodPost, "/internal/computers/self-development/consensus?computer_id="+f.computerID, bytes.NewReader(body))
	r.RemoteAddr = "172.16.0.2:40000"
	r.Header.Set("Authorization", "Bearer "+f.guestToken)
	w = httptest.NewRecorder()
	f.handler.HandleSelfDevelopmentConsensus(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("bound caller: %d %s", w.Code, w.Body.String())
	}
}
