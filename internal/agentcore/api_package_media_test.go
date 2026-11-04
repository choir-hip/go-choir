package agentcore

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/capsule/transaction"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/pkgrelease"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
)

// Release media at the publisher (docs/problems/library-releases-have-no-screenshots-2026-11-04.md,
// design 2 and 3), written before the code. Failure modes pinned:
//
//	R1  the draft names an image that is not one of the applied operation's
//	    bound frozen screens (another operation's, a file by path), or
//	    serves bytes for a digest it did not name;
//	R2  the draft names more than 8 screenshots, a blank screen, one over
//	    2 MiB or over 4096 px a side, or more than one video;
//	R3  a bound screen whose bytes on disk are not its digest is named (the
//	    host would then refuse the whole publish); it is skipped instead;
//	R4  an operation with no screens fails to draft, or drafts a media field
//	    (its release id would move);
//	R5  the media route answers for an operation that is not applied, or
//	    for a write.

func mediaPNG(t *testing.T, w, h int, seed int64, uniform bool) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	r := rand.New(rand.NewSource(seed))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := color.RGBA{uint8(seed), 40, 90, 255}
			if !uniform {
				c = color.RGBA{uint8(x + int(seed)), uint8(y), uint8(r.Intn(4)), 255}
			}
			img.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// noisyPNG is incompressible: w*h*4 bytes and more.
func noisyPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	r := rand.New(rand.NewSource(7))
	_, _ = r.Read(img.Pix)
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func mediaWebM() []byte {
	head := []byte{0x1a, 0x45, 0xdf, 0xa3, 0x9f, 0x42, 0x86, 0x81, 0x01, 0x42, 0xf7, 0x81, 0x01, 0x42, 0xf2, 0x81, 0x04, 0x42, 0xf3, 0x81, 0x08, 0x42, 0x82, 0x84, 'w', 'e', 'b', 'm', 0x42, 0x87, 0x81, 0x04, 0x42, 0x85, 0x81, 0x02}
	return append(head, bytes.Repeat([]byte{3}, 512)...)
}

// mediaOperation records an applied operation whose frozen bundle carries
// the patch of files and the given screens and video, staged as Freeze
// stages them (evidence/screens/).
func (w *draftWorld) mediaOperation(id string, files map[string]string, screens map[string][]byte, video []byte) []byte {
	w.t.Helper()
	t := w.t
	patch, _ := releasePatch(t, w.repo, files)
	bundle := transaction.CapsuleEffectBundle{
		BundleVersion: 1, ComputerID: draftComputer, BaseEventHead: strings.Repeat("a", 64),
		TrajectoryRef: "trajectory-" + id, CapsuleIdentity: "capsule-" + id,
		SourcePatchSHA256: computerevent.DigestBytes(patch), SourcePatchBaseCommit: w.base,
		VerifierReceipts: []string{strings.Repeat("f", 64)},
	}
	names := make([]string, 0, len(screens))
	for name := range screens {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		raw := screens[name]
		cfg, err := png.DecodeConfig(bytes.NewReader(raw))
		if err != nil {
			t.Fatal(err)
		}
		_, _, blank, err := capsule.CheckScreenPNG(raw)
		if err != nil {
			t.Fatal(err)
		}
		bundle.Screens = append(bundle.Screens, capsule.ScreenFile{Name: name, SHA256: computerevent.DigestBytes(raw), Bytes: int64(len(raw)), Width: cfg.Width, Height: cfg.Height, Blank: blank})
	}
	if video != nil {
		bundle.Video = &capsule.VideoFile{Name: "walk.webm", SHA256: computerevent.DigestBytes(video), Bytes: int64(len(video))}
	}
	var err error
	if bundle.ContentDigest, err = bundle.ComputeContentDigest(); err != nil {
		t.Fatal(err)
	}
	raw, err := computerevent.CanonicalJSON(bundle)
	if err != nil {
		t.Fatal(err)
	}
	digest := computerevent.DigestBytes(raw)
	dir := filepath.Join(w.updaterRoot, "incoming", digest)
	if err := os.MkdirAll(filepath.Join(dir, filepath.FromSlash(capsule.BundleScreensDir)), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "bundle.json"), raw, 0o400); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "source.patch"), patch, 0o444); err != nil {
		t.Fatal(err)
	}
	for name, data := range screens {
		if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(capsule.BundleScreensDir), name), data, 0o444); err != nil {
			t.Fatal(err)
		}
	}
	if video != nil {
		if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(capsule.BundleScreensDir), "walk.webm"), video, 0o444); err != nil {
			t.Fatal(err)
		}
	}
	w.at = w.at.Add(time.Second)
	if _, err := w.products.DB().ExecContext(context.Background(), `INSERT INTO self_development_operations
		(operation_id,computer_id,idempotency_key,request_commitment,trajectory_id,capsule_id,base_head,prompt_artifact_ref,bundle_digest,release_digest,code_ref,verifier_refs_json,decision_event,desired_head,effective_head,checkpoint_ref,route_certificate,state,created_at,updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		id, draftComputer, "key-"+id, strings.Repeat("d", 64), "trajectory-"+id, "capsule-"+id, strings.Repeat("a", 64),
		"artifact:sha256:"+strings.Repeat("b", 64), digest, strings.Repeat("3", 64), "code:sha256:"+strings.Repeat("9", 64), `["verifier:`+id+`"]`,
		strings.Repeat("e", 64), strings.Repeat("c", 64), strings.Repeat("c", 64), "checkpoint:"+id, "route-cert:"+id, selfdev.StateApplied, w.at, w.at); err != nil {
		t.Fatal(err)
	}
	return patch
}

// pairedScreens is the before/after pair every update of an existing package
// carries (capsule.ScreenRole).
func pairedScreens(t *testing.T) map[string][]byte {
	return map[string][]byte{
		"01-before-view.png": mediaPNG(t, 64, 40, 71, false),
		"01-after-view.png":  mediaPNG(t, 64, 40, 72, false),
	}
}

func (w *draftWorld) paired(id string, files map[string]string) []byte {
	w.t.Helper()
	return w.mediaOperation(id, files, pairedScreens(w.t), nil)
}

func (w *draftWorld) media(op, digest string) (int, string, []byte) {
	rec := releaseRouterCall(w.t, w.h, http.MethodGet, "/api/computers/"+draftComputer+"/packages/operations/"+op+"/media/"+digest, "owner-a", draftComputer, nil)
	return rec.Code, rec.Header().Get("Content-Type"), rec.Body.Bytes()
}

func TestPackageDraftNamesTheBoundScreensWithinBounds(t *testing.T) {
	w := newDraftWorld(t)
	screens := map[string][]byte{
		"blank.png":       mediaPNG(t, 64, 40, 1, true),
		"huge.png":        noisyPNG(t, 900, 700),
		"tall.png":        mediaPNG(t, 40, pkgrelease.MaxScreenshotSide+1, 2, false),
		"swapped.png":     mediaPNG(t, 64, 40, 3, false),
		"z-phone.png":     mediaPNG(t, 39, 84, 4, false),
		"a-home.png":      mediaPNG(t, 128, 80, 5, false),
		"after-home.png":  mediaPNG(t, 64, 40, 73, false),
		"before-home.png": mediaPNG(t, 64, 40, 74, false),
	}
	for i := 0; i < 8; i++ {
		screens[fmt.Sprintf("m-%d.png", i)] = mediaPNG(t, 110, 75, int64(10+i), false)
	}
	if len(screens["huge.png"]) <= pkgrelease.MaxScreenshotBytes {
		t.Fatalf("huge.png is %d bytes; the test needs it over the cap", len(screens["huge.png"]))
	}
	video := mediaWebM()
	w.mediaOperation("op-shots", map[string]string{draftRoot + "/App.svelte": "<p>v2</p>\n"}, screens, video)
	// R3: a bound screen whose bytes changed on disk after Freeze.
	swapped := filepath.Join(w.updaterRoot, "incoming")
	matches, _ := filepath.Glob(filepath.Join(swapped, "*", "evidence", "screens", "swapped.png"))
	if len(matches) != 1 {
		t.Fatalf("swapped.png staged %d times", len(matches))
	}
	_ = os.Chmod(matches[0], 0o644)
	if err := os.WriteFile(matches[0], mediaPNG(t, 64, 40, 99, false), 0o644); err != nil {
		t.Fatal(err)
	}

	record, id, _ := decodeDraft(t, w.draft("op-shots", nil))
	var names []string
	shots, videos := 0, 0
	for _, m := range record.Media {
		names = append(names, m.Name)
		switch m.Kind {
		case pkgrelease.MediaKindScreenshot:
			shots++
		case pkgrelease.MediaKindVideo:
			videos++
		}
	}
	// R2: first 8 eligible by name; blank, huge, tall and swapped skipped.
	want := "a-home.png,after-home.png,before-home.png,m-0.png,m-1.png,m-2.png,m-3.png,m-4.png,walk.webm"
	if strings.Join(names, ",") != want || shots != 8 || videos != 1 {
		t.Fatalf("media %v, want %s", names, want)
	}
	if record.Media[0].Width != 128 || record.Media[0].Height != 80 || record.Media[0].SHA256 != computerevent.DigestBytes(screens["a-home.png"]) {
		t.Fatalf("a-home.png %+v", record.Media[0])
	}
	// R1: every named item's bytes come back from the media route, verified.
	for _, m := range record.Media {
		status, ctype, raw := w.media("op-shots", m.SHA256)
		if status != http.StatusOK || ctype != m.MediaType || pkgrelease.VerifyMedia(m, raw) != nil {
			t.Fatalf("media %s = %d %s", m.Name, status, ctype)
		}
	}
	// R1: a bound but unnamed screen, a skipped one and an unknown digest are 404.
	for _, digest := range []string{computerevent.DigestBytes(screens["blank.png"]), computerevent.DigestBytes(screens["m-5.png"]), computerevent.DigestBytes(screens["huge.png"]), strings.Repeat("0", 64), "not-a-digest"} {
		if status, _, _ := w.media("op-shots", digest); status != http.StatusNotFound {
			t.Fatalf("media %s = %d, want 404", digest, status)
		}
	}
	// Deterministic.
	if _, again, _ := decodeDraft(t, w.draft("op-shots", nil)); again != id {
		t.Fatalf("second draft %s, first %s", again, id)
	}
	// R5.
	if rec := releaseRouterCall(t, w.h, http.MethodPost, "/api/computers/"+draftComputer+"/packages/operations/op-shots/media/"+record.Media[0].SHA256, "owner-a", draftComputer, nil); rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("media write = %d, want 405", rec.Code)
	}
	w.operation("op-awaiting", selfdev.StateAwaitingApproval, map[string]string{draftRoot + "/App.svelte": "<p>v3</p>\n"})
	if status, _, _ := w.media("op-awaiting", record.Media[0].SHA256); status != http.StatusConflict {
		t.Fatalf("media of an unapplied operation = %d, want 409", status)
	}
}

// Owner, 2026-11-04: a release that updates a package shows the same view
// before and after. A bundle with no screens, or after shots only, is refused;
// a new package (empty parent) needs only after shots, so it is not drafted
// here (its fixture is the registry's).
func TestPackageDraftOfAnUpdateNeedsABeforeAfterPair(t *testing.T) {
	w := newDraftWorld(t)
	w.operation("op-plain", selfdev.StateApplied, map[string]string{draftRoot + "/App.svelte": "<p>v2</p>\n"})
	wantDraftReason(t, w.draft("op-plain", nil), http.StatusUnprocessableEntity, capsule.BeforeAfterRequiredReason)
	w = newDraftWorld(t)
	w.mediaOperation("op-after", map[string]string{draftRoot + "/App.svelte": "<p>v2</p>\n"}, map[string][]byte{"01-after-view.png": mediaPNG(t, 64, 40, 75, false)}, nil)
	wantDraftReason(t, w.draft("op-after", nil), http.StatusUnprocessableEntity, capsule.BeforeAfterRequiredReason)
	w = newDraftWorld(t)
	w.paired("op-pair", map[string]string{draftRoot + "/App.svelte": "<p>v2</p>\n"})
	record, _, _ := decodeDraft(t, w.draft("op-pair", nil))
	if len(record.Media) != 2 {
		t.Fatalf("a paired draft names %d media, want 2", len(record.Media))
	}
}

// Review r1 F1: two bound screens with the same bytes name the digest once,
// and the draft still drafts; F7: the release's video bound is Freeze's.
func TestPackageDraftNamesARepeatedScreenOnce(t *testing.T) {
	if pkgrelease.MaxVideoBytes != capsule.MaxVideoBytes {
		t.Fatalf("release video cap %d, Freeze cap %d", pkgrelease.MaxVideoBytes, capsule.MaxVideoBytes)
	}
	w := newDraftWorld(t)
	same := mediaPNG(t, 64, 40, 5, false)
	w.mediaOperation("op-twice", map[string]string{draftRoot + "/App.svelte": "<p>v2</p>\n"}, map[string][]byte{"a.png": same, "b.png": same, "c.png": mediaPNG(t, 64, 40, 6, false), "before-x.png": mediaPNG(t, 64, 40, 76, false), "after-x.png": mediaPNG(t, 64, 40, 77, false)}, nil)
	record, _, _ := decodeDraft(t, w.draft("op-twice", nil))
	var names []string
	for _, m := range record.Media {
		names = append(names, m.Name)
	}
	if strings.Join(names, ",") != "a.png,after-x.png,before-x.png,c.png" {
		t.Fatalf("media %v, want a.png,after-x.png,before-x.png,c.png", names)
	}
}
