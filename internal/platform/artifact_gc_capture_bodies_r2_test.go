package platform

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/sourcefetch"
	"github.com/yusefmosiah/go-choir/internal/sourcegraph"
	"github.com/yusefmosiah/go-choir/internal/vmctl"
)

// Round 2 of the capture-bodies lane, after a two-seat REPAIR. Failure list
// (continues artifact_gc_capture_bodies_test.go's 1-11; 11 is reversed:
// observation is not activation):
// 12. a live set that cannot be computed completely marks bodies eligible, or
//     stops every other namespace;
// 13. a nested blob id is not counted live;
// 14. the report lacks the numbers the admission reads;
// 15. PutBody's reuse refresh does not hold the reuse lock, so the sweep's
//     unlink can interleave with it;
// 16. what the real writer and readers use does not survive an active pass;
// 17. an environment without the variable becomes active for the namespace.

// A live set that cannot be computed completely marks nothing eligible and
// stops only this namespace: the other namespaces still sweep.
func TestCaptureBodiesGCUnparseableRowRefusesThisNamespaceOnly(t *testing.T) {
	f := newCaptureGCFixture(t)
	h, _ := f.body("live", 48*time.Hour)
	f.row("cap", captureMeta(h, h, h), false)
	f.row("cap-bad", `{not json`, false)
	_, p := f.body("dead", 48*time.Hour)
	rep, err := f.svc.RunArtifactGC(context.Background(), ArtifactGCConfig{Mode: "active", Grace: time.Hour})
	if err != nil {
		t.Fatalf("one unparseable capture row aborted the whole pass: %v", err)
	}
	ns := rep.Namespaces["capture-bodies"]
	if !exists(p) || ns.Eligible != 0 || ns.Deleted != 0 || ns.Mode != ArtifactGCModeDryRun || !ns.LiveSetIncomplete {
		t.Fatalf("capture-bodies with an unprovable live set: %+v", ns)
	}
	found := false
	for _, w := range rep.Warnings {
		if strings.Contains(w, "capture-bodies live set") {
			found = true
		}
	}
	if !found {
		t.Fatalf("no warning names the incomplete live set: %v", rep.Warnings)
	}
	if _, ok := rep.Namespaces["file-cas-chunks"]; !ok {
		t.Fatal("the other namespaces were not swept")
	}
}

// A blob id nested in the metadata (an object or an array) is as live as a
// flat one: the walk does not depend on today's flat schema.
func TestCaptureBodiesGCNestedBlobIDsAreLive(t *testing.T) {
	f := newCaptureGCFixture(t)
	h1, p1 := f.body("nested in object", 48*time.Hour)
	h2, p2 := f.body("nested in array", 48*time.Hour)
	f.row("cap", `{"blobs":{"x":"sha256:`+h1+`"},"more":[{"y":"sha256:`+h2+`"}]}`, false)
	_, dead := f.body("dead", 48*time.Hour)
	f.run(ArtifactGCConfig{Mode: "active"})
	if !exists(p1) || !exists(p2) {
		t.Fatal("a nested blob id was not counted live")
	}
	if exists(dead) {
		t.Fatal("unreferenced body kept")
	}
}

// The report-only pass carries the numbers the admission decision needs:
// live, in grace, eligible count and bytes, how many distinct ids the rows
// name (and how many of them are not on disk), and a sample of eligible ids
// with their ages.
func TestCaptureBodiesReportCarriesAdmissionNumbers(t *testing.T) {
	f := newCaptureGCFixture(t)
	h, _ := f.body("live", 48*time.Hour)
	missing := captureBodyHash("named but never written")
	f.row("cap", captureMeta(h, h, missing), false)
	old1, _ := f.body("dead one", 9*24*time.Hour)
	f.body("dead two", 3*24*time.Hour)
	f.body("fresh", 0)
	rep := f.run(ArtifactGCConfig{Mode: "active", ActiveNamespaces: []string{"og"}}).Namespaces["capture-bodies"]
	if rep.Mode != ArtifactGCModeDryRun || rep.Live != 1 || rep.InGrace != 1 || rep.Eligible != 2 || rep.EligibleBytes != int64(len("dead one")+len("dead two")) {
		t.Fatalf("counts: %+v", rep)
	}
	if rep.NamedByRows != 2 || rep.NamedMissingOnDisk != 1 {
		t.Fatalf("named %d missing %d: %+v", rep.NamedByRows, rep.NamedMissingOnDisk, rep)
	}
	if len(rep.EligibleSample) != 2 || rep.EligibleSample[0].ID != old1 || rep.EligibleSample[0].AgeHours < 9*24-1 {
		t.Fatalf("sample (oldest first): %+v", rep.EligibleSample)
	}
}

// Observation is not activation: an environment without the variable must
// not become active for this namespace (the owner's word and a read report
// come first; docs/problems/capture-bodies-sweep-admission-2026-11-14.md).
func TestCaptureBodiesAreReportOnlyUnlessNamedActive(t *testing.T) {
	for _, ns := range DefaultArtifactGCActiveNamespaces {
		if ns == "capture-bodies" {
			t.Fatal("capture-bodies is in the default active namespaces")
		}
	}
	f := newCaptureGCFixture(t)
	h, _ := f.body("live", 48*time.Hour)
	f.row("cap", captureMeta(h, h, h), false)
	_, dead := f.body("dead", 48*time.Hour)
	rep := f.run(ArtifactGCConfig{Mode: "active", ActiveNamespaces: ParseArtifactGCNamespaces("")}).Namespaces["capture-bodies"]
	if !exists(dead) || rep.Eligible != 1 || rep.Mode != ArtifactGCModeDryRun {
		t.Fatalf("default config deleted or hid capture bodies: %+v", rep)
	}
}

// The writer's reuse refresh holds the CAS reuse lock shared, as every other
// artifact writer does, so a collector unlinking under the exclusive lock
// cannot interleave with the check-and-touch.
func TestCaptureBodyPutWaitsForTheCollectorsUnlinkWindow(t *testing.T) {
	f := newCaptureGCFixture(t)
	_, p := f.body("reused", 48*time.Hour)
	store := f.svc.captureBodies
	f.store.casReuseMu.Lock()
	done := make(chan error, 1)
	go func() { _, err := store.PutBody([]byte("reused")); done <- err }()
	select {
	case <-done:
		f.store.casReuseMu.Unlock()
		t.Fatal("PutBody did not wait for the collector's exclusive reuse lock")
	case <-time.After(150 * time.Millisecond):
	}
	f.store.casReuseMu.Unlock()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(p); err != nil || time.Since(info.ModTime()) > time.Minute {
		t.Fatalf("reused body not refreshed: %v %v", info, err)
	}
}

// The race the sweep's recheck exists for: a body classified dead is reused
// by a capture before the unlink; the refreshed body survives.
func TestCaptureBodyReusedDuringSweepSurvives(t *testing.T) {
	f := newCaptureGCFixture(t)
	hash, p := f.body("reused during sweep", 48*time.Hour)
	f.svc.artifactGCBeforeUnlink = func(rel string) {
		if strings.HasSuffix(rel, hash) {
			if _, err := f.svc.captureBodies.PutBody([]byte("reused during sweep")); err != nil {
				t.Errorf("reuse: %v", err)
			}
		}
	}
	h, _ := f.body("live", 48*time.Hour)
	f.row("cap", captureMeta(h, h, h), false)
	rep := f.run(ArtifactGCConfig{Mode: "active"})
	if !exists(p) {
		t.Fatal("a body reused during the sweep was deleted")
	}
	if got := rep.Namespaces["capture-bodies"]; got.KeptOnRecheck != 1 || got.Deleted != 0 {
		t.Fatalf("recheck: %+v", got)
	}
}

// End to end with the real writer and readers: a capture written by
// sourcegraph.WriteCaptures through the handler into the SQL store, its
// bodies aged past the grace, an active pass, then the capture still reads
// and its raw body is still served. An orphan goes.
func TestCaptureBodiesSweepKeepsWhatTheRealWriterAndReadersUse(t *testing.T) {
	f := newCaptureGCFixture(t)
	ogStore := NewObjectGraphStore(f.store)
	svc := objectgraph.NewService(objectgraph.Config{Durable: ogStore})
	defer svc.Close()
	h := NewObjectGraphHandler(svc, ogStore)
	guest := &vmctl.GuestOwnership{Found: true, UserID: "paper-owner", ComputerID: "computer-paper"}
	if err := h.ConfigureGuestBinding(fakeGuestLookup{"10.200.1.2": guest}); err != nil {
		t.Fatal(err)
	}
	h.ConfigureCaptureBodies(CaptureBodiesRoot(f.root))
	page, err := os.ReadFile("../sourcefetch/testdata/wiki-table.html")
	if err != nil {
		t.Fatal(err)
	}
	doc := sourcefetch.ExtractBlocks(page)
	item := sourcegraph.ReadCaptureItem("https://en.example.org/wiki/Cities", page, "text/html", doc, time.Now().UTC())
	raw, _ := json.Marshal(CaptureWriteRequest{OwnerID: "paper-owner", ComputerID: "computer-paper", Items: []sourcegraph.CaptureItem{item}})
	req := httptest.NewRequest(http.MethodPost, "/internal/platform/captures", bytes.NewReader(raw))
	req.RemoteAddr = "10.200.1.2:40000"
	rec := httptest.NewRecorder()
	h.HandleCaptures(rec, req)
	var wrote CaptureWriteResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &wrote); rec.Code != http.StatusOK || err != nil || wrote.New != 1 {
		t.Fatalf("write = %d %s", rec.Code, rec.Body)
	}
	_, orphan := f.body("orphan", 0)
	old := time.Now().Add(-10 * 24 * time.Hour)
	bodies := 0
	_ = filepath.WalkDir(CaptureBodiesRoot(f.root), func(path string, d os.DirEntry, err error) error {
		if err == nil && d.Type().IsRegular() {
			bodies++
			_ = os.Chtimes(path, old, old)
		}
		return nil
	})
	if bodies != 4 {
		t.Fatalf("%d bodies on disk, want the writer's three and the orphan", bodies)
	}
	rep := f.run(ArtifactGCConfig{Mode: "active", ActiveNamespaces: []string{"capture-bodies"}})
	if got := rep.Namespaces["capture-bodies"]; got.Live != 3 || got.Deleted != 1 {
		t.Fatalf("sweep of real captures: %+v", got)
	}
	if exists(orphan) {
		t.Fatal("orphan kept")
	}
	id := wrote.Captured[0].CaptureID
	get := httptest.NewRequest(http.MethodGet, "/internal/platform/captures/"+id+"?max_chars=100000", nil)
	get.RemoteAddr = "10.200.1.2:40000"
	out := httptest.NewRecorder()
	h.HandleCaptureRead(out, get)
	if out.Code != http.StatusOK {
		t.Fatalf("capture unreadable after the sweep: %d %s", out.Code, out.Body)
	}
	var read CaptureReadResponse
	_ = json.Unmarshal(out.Body.Bytes(), &read)
	body := httptest.NewRequest(http.MethodGet, "/internal/platform/captures/bodies/"+strings.TrimPrefix(read.ContentBlobID, "sha256:"), nil)
	body.RemoteAddr = "10.200.1.2:40000"
	rawOut := httptest.NewRecorder()
	h.HandleCaptureBody(rawOut, body)
	if rawOut.Code != http.StatusOK || !bytes.Equal(rawOut.Body.Bytes(), page) {
		t.Fatalf("raw body after the sweep: %d", rawOut.Code)
	}
}
