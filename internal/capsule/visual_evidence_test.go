package capsule

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/visualcapture"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// Visual evidence at Freeze (docs/design/visual-evidence-slice-2026-10-15.md
// V1, V8, V11, V17, V19-V21, V31): the staging and its refusals, on a fake
// opener over a temp directory, so none of it needs a capsule.

const evW, evH = 320, 200

// evidenceFixture is a staged-release frontend, a capture directory and the
// manifest that binds them.
type evidenceFixture struct {
	t        *testing.T
	dir      string // the capsule's evidence directory
	release  []FrozenReleaseFile
	manifest visualcapture.Manifest
	names    []string
}

func pngBytes(t *testing.T, w, h int, paint bool) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: 250, G: 250, B: 250, A: 255})
		}
	}
	if paint {
		for y := 20; y < 60; y++ {
			for x := 20; x < 200; x++ {
				img.Set(x, y, color.RGBA{R: 10, G: 20, B: 30, A: 255})
			}
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func sha(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func newEvidenceFixture(t *testing.T) *evidenceFixture {
	t.Helper()
	f := &evidenceFixture{t: t, dir: t.TempDir()}
	f.release = []FrozenReleaseFile{
		{Path: "bin/autoputer", SHA256: strings.Repeat("a", 64), Mode: 0o555},
		{Path: "frontend/index.html", SHA256: strings.Repeat("b", 64), Mode: 0o444},
		{Path: "frontend/assets/app.js", SHA256: strings.Repeat("c", 64), Mode: 0o444},
	}
	f.manifest = visualcapture.Manifest{
		Kind:    visualcapture.ManifestKind,
		Subject: visualcapture.Subject{FrontendSHA256: ReleaseFrontendDigest(f.release), FrontendFiles: 2},
		Browser: visualcapture.Browser{Name: "chromium", Path: "/nix/store/x-chromium/bin/chromium"},
		WaitMS:  5000,
	}
	return f
}

// capture adds one capture of route with the given image bytes.
func (f *evidenceFixture) capture(route string, data []byte) {
	f.t.Helper()
	digest := sha(data)
	name := digest + ".png"
	if err := os.WriteFile(filepath.Join(f.dir, name), data, 0o644); err != nil {
		f.t.Fatal(err)
	}
	f.manifest.Captures = append(f.manifest.Captures, visualcapture.Capture{
		Artifact: visualcapture.ArtifactRef(digest), File: name, MediaType: visualcapture.MediaTypePNG,
		Bytes: int64(len(data)), Route: route, Width: evW, Height: evH, ReducedMotion: true,
		TakenAt: "2026-10-20T00:00:00Z",
	})
	f.names = append(f.names, name)
}

// writeManifest writes visual-evidence.json and returns its digest.
func (f *evidenceFixture) writeManifest() string {
	f.t.Helper()
	raw, err := json.MarshalIndent(f.manifest, "", "  ")
	if err != nil {
		f.t.Fatal(err)
	}
	raw = append(raw, '\n')
	if err := os.WriteFile(filepath.Join(f.dir, visualcapture.ManifestFile), raw, 0o644); err != nil {
		f.t.Fatal(err)
	}
	if !contains(f.names, visualcapture.ManifestFile) {
		f.names = append(f.names, visualcapture.ManifestFile)
	}
	return sha(raw)
}

func contains(list []string, want string) bool {
	for _, v := range list {
		if v == want {
			return true
		}
	}
	return false
}

func (f *evidenceFixture) open(name string) (evidenceInput, error) {
	return os.Open(filepath.Join(f.dir, name))
}

func (f *evidenceFixture) stage(jobs []yaegikernel.JobView) (*VisualEvidence, string, error) {
	f.t.Helper()
	temporary := f.t.TempDir()
	ev, err := stageEvidenceFiles(context.Background(), f.names, f.open, temporary, f.release, jobs)
	return ev, temporary, err
}

func captureJob(state, receipt, tail string) yaegikernel.JobView {
	return yaegikernel.JobView{ID: "job-1", State: state, Argv: []string{"choir-capture", "--route", "/"}, ReceiptRef: receipt, Tail: tail}
}

const receiptA = "capsule-exec:sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
const receiptB = "capsule-exec:sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

func TestStageEvidenceBindsCapturesToTheFrozenFrontendAndTheCaptureJob(t *testing.T) {
	f := newEvidenceFixture(t)
	shot := pngBytes(t, evW, evH, true)
	f.capture("/", shot)
	manifestSHA := f.writeManifest()
	// The job's output is what choir-capture prints, not a hand-made string.
	raw, err := os.ReadFile(filepath.Join(f.dir, visualcapture.ManifestFile))
	if err != nil {
		t.Fatal(err)
	}
	jobs := []yaegikernel.JobView{captureJob("done", receiptA, "building...\n"+string(visualcapture.Summary(f.manifest, raw, nil, 0))+"\n")}

	ev, temporary, err := f.stage(jobs)
	if err != nil {
		t.Fatal(err)
	}
	if ev.ManifestSHA256 != manifestSHA || ev.FrontendSHA256 != ReleaseFrontendDigest(f.release) || ev.CaptureReceipt != receiptA {
		t.Fatalf("evidence not bound: %+v", ev)
	}
	if len(ev.Captures) != 1 || ev.Captures[0].Artifact != "artifact:sha256:"+sha(shot) || ev.Captures[0].Route != "/" || ev.Captures[0].Blank {
		t.Fatalf("captures: %+v", ev.Captures)
	}
	if refs := ev.Refs(); len(refs) != 1 || refs[0] != ev.Captures[0].Artifact {
		t.Fatalf("refs: %v", refs)
	}
	// V20: the evidence is beside release/, never in it.
	staged := filepath.Join(temporary, "evidence", "visual", sha(shot)+".png")
	if got, err := os.ReadFile(staged); err != nil || !bytes.Equal(got, shot) {
		t.Fatalf("staged capture: %v", err)
	}
	if _, err := os.Stat(filepath.Join(temporary, "evidence", "visual", visualcapture.ManifestFile)); err != nil {
		t.Fatalf("staged manifest: %v", err)
	}
	if _, err := os.Stat(filepath.Join(temporary, "frontend")); err == nil {
		t.Fatal("evidence leaked into the release area")
	}
}

func TestStageEvidenceWithNoFilesStagesNothing(t *testing.T) {
	f := newEvidenceFixture(t)
	ev, temporary, err := f.stage(nil)
	if err != nil || ev != nil {
		t.Fatalf("no files: %+v %v", ev, err)
	}
	if entries, _ := os.ReadDir(temporary); len(entries) != 0 {
		t.Fatalf("no files created %d entries", len(entries))
	}
}

// V17: a manifest the capture job did not write gets no receipt, and so
// visual_captured cannot settle on it.
func TestCaptureReceiptRequiresTheJobThatWroteThisManifest(t *testing.T) {
	f := newEvidenceFixture(t)
	f.capture("/", pngBytes(t, evW, evH, true))
	manifestSHA := f.writeManifest()
	good := `{"manifest_sha256":"` + manifestSHA + `"}`
	other := `{"manifest_sha256":"` + strings.Repeat("9", 64) + `"}`
	notCapture := yaegikernel.JobView{ID: "job-x", State: "done", Argv: []string{"npm", "run", "build"}, ReceiptRef: receiptB, Tail: good}

	for name, tc := range map[string]struct {
		jobs []yaegikernel.JobView
		want string
	}{
		"no jobs (hand-written evidence)":               {nil, ""},
		"job still running":                             {[]yaegikernel.JobView{captureJob("running", "", good)}, ""},
		"job failed":                                    {[]yaegikernel.JobView{captureJob("failed", receiptA, good)}, ""},
		"manifest rewritten after the job":              {[]yaegikernel.JobView{captureJob("done", receiptA, other)}, ""},
		"another command printed the digest":            {[]yaegikernel.JobView{notCapture}, ""},
		"done job without a receipt":                    {[]yaegikernel.JobView{captureJob("done", "", good)}, ""},
		"newest matching job wins":                      {[]yaegikernel.JobView{captureJob("done", receiptA, good), captureJob("done", receiptB, good)}, receiptB},
		"a later non-matching job does not hide it":     {[]yaegikernel.JobView{captureJob("done", receiptA, good), captureJob("done", receiptB, other)}, receiptA},
		"a later non-capture job does not hide it":      {[]yaegikernel.JobView{captureJob("done", receiptA, good), notCapture}, receiptA},
		"capture tool named by its store-less basename": {[]yaegikernel.JobView{{State: "done", Argv: []string{"/run/current-system/sw/bin/choir-capture"}, ReceiptRef: receiptA, Tail: good}}, receiptA},
	} {
		t.Run(name, func(t *testing.T) {
			ev, _, err := f.stage(tc.jobs)
			if err != nil {
				t.Fatalf("a manifest without a receipt is still admitted as evidence: %v", err)
			}
			if ev.CaptureReceipt != tc.want {
				t.Fatalf("receipt %q, want %q", ev.CaptureReceipt, tc.want)
			}
		})
	}
}

// Fix 3 (trace review round 4): the failed asset answers the receipt's own
// capture job printed reach the bundle record as advice; another job's, or a
// tail that does not parse, add nothing and never refuse the Freeze.
func TestStageEvidenceCarriesTheReceiptJobsAssetFailures(t *testing.T) {
	f := newEvidenceFixture(t)
	f.capture("/read.html", pngBytes(t, evW, evH, true))
	manifestSHA := f.writeManifest()
	raw, err := os.ReadFile(filepath.Join(f.dir, visualcapture.ManifestFile))
	if err != nil {
		t.Fatal(err)
	}
	missing := []visualcapture.AssetFailure{{Path: "/pdf-text-BRoX2Cg_.js", Status: 404, Count: 2}}
	withFailures := string(visualcapture.Summary(f.manifest, raw, missing, 0))
	other := `{"manifest_sha256":"` + strings.Repeat("9", 64) + `","asset_failures":[{"path":"/other.js","status":404,"count":1}]}`
	for name, tc := range map[string]struct {
		jobs []yaegikernel.JobView
		want []visualcapture.AssetFailure
	}{
		"the receipt job's failures":             {[]yaegikernel.JobView{captureJob("done", receiptA, "noise\n"+withFailures+"\n")}, missing},
		"a clean capture":                        {[]yaegikernel.JobView{captureJob("done", receiptA, string(visualcapture.Summary(f.manifest, raw, nil, 0)))}, nil},
		"another manifest's job":                 {[]yaegikernel.JobView{captureJob("done", receiptA, withFailures), captureJob("done", receiptB, other)}, missing},
		"no receipt job":                         {[]yaegikernel.JobView{captureJob("failed", receiptA, withFailures)}, nil},
		"a tail that does not parse":             {[]yaegikernel.JobView{captureJob("done", receiptA, `{"manifest_sha256":"`+manifestSHA+`","asset_failures":"x"}`)}, nil},
		"a newer clean run of the same manifest": {[]yaegikernel.JobView{captureJob("done", receiptA, withFailures), captureJob("done", receiptB, string(visualcapture.Summary(f.manifest, raw, nil, 0)))}, nil},
	} {
		t.Run(name, func(t *testing.T) {
			ev, _, err := f.stage(tc.jobs)
			if err != nil {
				t.Fatalf("advice refused the Freeze: %v", err)
			}
			if fmt.Sprint(ev.AssetFailures) != fmt.Sprint(tc.want) {
				t.Fatalf("asset failures = %v, want %v", ev.AssetFailures, tc.want)
			}
			if err := ev.Validate(); err != nil {
				t.Fatal(err)
			}
		})
	}
	bad := &VisualEvidence{ManifestSHA256: manifestSHA, FrontendSHA256: manifestSHA, Captures: []VisualEvidenceCapture{{Artifact: "artifact:sha256:" + manifestSHA, Route: "/", MediaType: visualcapture.MediaTypePNG, Bytes: 1}},
		AssetFailures: []visualcapture.AssetFailure{{Path: "/api/x.js", Status: 503, Count: 1}}}
	if err := bad.Validate(); err == nil {
		t.Fatal("a record naming the intended /api/ 503 validates")
	}
}

// V8: a capture of an older build is refused at Freeze.
func TestStageEvidenceRefusesACaptureOfADifferentFrontend(t *testing.T) {
	f := newEvidenceFixture(t)
	f.capture("/", pngBytes(t, evW, evH, true))
	f.writeManifest()
	f.release[1].SHA256 = strings.Repeat("d", 64) // rebuilt after the capture
	_, temporary, err := f.stage(nil)
	if err == nil || !strings.Contains(err.Error(), "different frontend build") {
		t.Fatalf("stale capture admitted: %v", err)
	}
	assertNothingStaged(t, temporary)
}

// V21: a partial directory (no manifest, or a manifest naming a missing
// file) is refused, and a refusal leaves no partial evidence behind.
func TestStageEvidenceRefusesAPartialDirectory(t *testing.T) {
	t.Run("captures without a manifest", func(t *testing.T) {
		f := newEvidenceFixture(t)
		f.capture("/", pngBytes(t, evW, evH, true))
		_, temporary, err := f.stage(nil)
		if err == nil || !strings.Contains(err.Error(), visualcapture.ManifestFile) {
			t.Fatalf("manifest-less evidence admitted: %v", err)
		}
		assertNothingStaged(t, temporary)
	})
	t.Run("manifest names a file that is not there", func(t *testing.T) {
		f := newEvidenceFixture(t)
		f.capture("/", pngBytes(t, evW, evH, true))
		f.writeManifest()
		f.names = []string{visualcapture.ManifestFile}
		_, temporary, err := f.stage(nil)
		if err == nil || !strings.Contains(err.Error(), "not in the evidence directory") {
			t.Fatalf("missing capture admitted: %v", err)
		}
		assertNothingStaged(t, temporary)
	})
	t.Run("a file the manifest does not name", func(t *testing.T) {
		f := newEvidenceFixture(t)
		f.capture("/", pngBytes(t, evW, evH, true))
		f.writeManifest()
		extra := pngBytes(t, evW, evH, false)
		if err := os.WriteFile(filepath.Join(f.dir, sha(extra)+".png"), extra, 0o644); err != nil {
			t.Fatal(err)
		}
		f.names = append(f.names, sha(extra)+".png")
		if _, _, err := f.stage(nil); err == nil || !strings.Contains(err.Error(), "does not name") {
			t.Fatalf("unnamed file admitted: %v", err)
		}
	})
	t.Run("the opener fails", func(t *testing.T) {
		f := newEvidenceFixture(t)
		f.capture("/", pngBytes(t, evW, evH, true))
		f.writeManifest()
		open := func(string) (evidenceInput, error) { return nil, errors.New("symlink refused") }
		if _, err := stageEvidenceFiles(context.Background(), f.names, open, t.TempDir(), f.release, nil); err == nil || !strings.Contains(err.Error(), "unavailable") {
			t.Fatalf("unreadable evidence admitted: %v", err)
		}
	})
}

func TestStageEvidenceRefusesImagesThatAreNotTheViewportShot(t *testing.T) {
	t.Run("bytes changed after the manifest was written", func(t *testing.T) {
		f := newEvidenceFixture(t)
		f.capture("/", pngBytes(t, evW, evH, true))
		f.writeManifest()
		if err := os.WriteFile(filepath.Join(f.dir, f.manifest.Captures[0].File), pngBytes(t, evW, evH, false), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, _, err := f.stage(nil); err == nil || !strings.Contains(err.Error(), "sha256 or size") {
			t.Fatalf("tampered capture admitted: %v", err)
		}
	})
	t.Run("not a PNG (V11)", func(t *testing.T) {
		f := newEvidenceFixture(t)
		f.capture("/", []byte("this is not an image, only text of a plausible size"))
		f.writeManifest()
		if _, _, err := f.stage(nil); err == nil || !strings.Contains(err.Error(), "not a PNG") {
			t.Fatalf("non-PNG admitted: %v", err)
		}
	})
	t.Run("not the viewport the manifest names (V11)", func(t *testing.T) {
		f := newEvidenceFixture(t)
		f.capture("/", pngBytes(t, evW, evH+40, true))
		f.writeManifest()
		if _, _, err := f.stage(nil); err == nil || !strings.Contains(err.Error(), "viewport") {
			t.Fatalf("wrong-size capture admitted: %v", err)
		}
	})
	t.Run("blank is recomputed, not trusted", func(t *testing.T) {
		f := newEvidenceFixture(t)
		f.capture("/", pngBytes(t, evW, evH, false)) // uniform
		f.capture("/b", pngBytes(t, evW, evH, true))
		f.manifest.Captures[0].Blank = false // the manifest lies
		f.manifest.Captures[1].Blank = true  // and lies the other way
		f.writeManifest()
		ev, _, err := f.stage(nil)
		if err != nil {
			t.Fatal(err)
		}
		if !ev.Captures[0].Blank || ev.Captures[1].Blank {
			t.Fatalf("blank flags taken from the manifest: %+v", ev.Captures)
		}
	})
}

// V19 and V31: bounded, and no secret rides in a screenshot or manifest.
func TestStageEvidenceBoundsAndScansWhatItStages(t *testing.T) {
	t.Run("too many files", func(t *testing.T) {
		var changes []FileChange
		for i := 0; i < visualcapture.MaxRoutes+2; i++ {
			changes = append(changes, FileChange{Path: visualcapture.EvidencePrefix + strings.Repeat("a", 63) + string(rune('0'+i)) + ".png", Kind: ChangeAdded})
		}
		if _, err := visualEvidenceNames(changes); err == nil || !strings.Contains(err.Error(), "at most") {
			t.Fatalf("too many files admitted: %v", err)
		}
	})
	t.Run("one capture over the limit", func(t *testing.T) {
		f := newEvidenceFixture(t)
		name := strings.Repeat("e", 64) + ".png"
		if err := os.WriteFile(filepath.Join(f.dir, name), make([]byte, visualcapture.MaxCaptureBytes+1), 0o644); err != nil {
			t.Fatal(err)
		}
		f.names = []string{name}
		if _, _, err := f.stage(nil); err == nil || !strings.Contains(err.Error(), "larger than") {
			t.Fatalf("oversize capture admitted: %v", err)
		}
	})
	t.Run("oversize manifest", func(t *testing.T) {
		f := newEvidenceFixture(t)
		if err := os.WriteFile(filepath.Join(f.dir, visualcapture.ManifestFile), make([]byte, visualcapture.MaxManifestBytes+1), 0o644); err != nil {
			t.Fatal(err)
		}
		f.names = []string{visualcapture.ManifestFile}
		if _, _, err := f.stage(nil); err == nil || !strings.Contains(err.Error(), "larger than") {
			t.Fatalf("oversize manifest admitted: %v", err)
		}
	})
	t.Run("a private key hidden after the image data", func(t *testing.T) {
		f := newEvidenceFixture(t)
		shot := append(pngBytes(t, evW, evH, true), []byte("\n-----BEGIN RSA PRIVATE KEY-----\nMIIEowIBAAKCAQEA\n-----END RSA PRIVATE KEY-----\n")...)
		f.capture("/", shot)
		f.writeManifest()
		_, temporary, err := f.stage(nil)
		if err == nil || !strings.Contains(err.Error(), "secret") {
			t.Fatalf("secret in a capture admitted: %v", err)
		}
		assertNothingStaged(t, temporary)
	})
	t.Run("a secret in the manifest", func(t *testing.T) {
		f := newEvidenceFixture(t)
		f.capture("/", pngBytes(t, evW, evH, true))
		f.manifest.Browser.Path = "/nix/store/x/bin/chromium?token=ghp_abcdefghijklmnopqrstuvwxyz0123456789"
		f.writeManifest()
		if _, _, err := f.stage(nil); err == nil || !strings.Contains(err.Error(), "secret") {
			t.Fatalf("secret in the manifest admitted: %v", err)
		}
	})
}

func TestVisualEvidenceNamesReadsOnlyTheFlatEvidenceDirectory(t *testing.T) {
	changes := []FileChange{
		{Path: "var/lib/artifact/release/frontend/index.html", Kind: ChangeAdded},
		{Path: "var/lib/artifact/evidence", Kind: ChangeAdded, Mode: os.ModeDir},
		{Path: "var/lib/artifact/evidence/visual", Kind: ChangeAdded, Mode: os.ModeDir},
		{Path: "var/lib/artifact/evidence/visual/" + visualcapture.ManifestFile, Kind: ChangeAdded},
		{Path: "/var/lib/artifact/evidence/visual/" + strings.Repeat("f", 64) + ".png", Kind: ChangeModified},
		{Path: "var/lib/artifact/evidence/visual/stale.png", Kind: ChangeDeleted},
		{Path: "workspace/platform/notes.md", Kind: ChangeAdded},
	}
	names, err := visualEvidenceNames(changes)
	if err != nil || len(names) != 2 || names[0] != visualcapture.ManifestFile {
		t.Fatalf("names %v, err %v", names, err)
	}
	if _, err := visualEvidenceNames([]FileChange{{Path: "var/lib/artifact/evidence/visual/sub/x.png", Kind: ChangeAdded}}); err == nil {
		t.Fatal("a nested evidence path was admitted")
	}
	if names, err := visualEvidenceNames(changes[:3]); err != nil || len(names) != 0 {
		t.Fatalf("a capsule that captured nothing: %v %v", names, err)
	}
}

func TestVisualEvidenceValidateRefusesMalformedRecords(t *testing.T) {
	good := func() *VisualEvidence {
		return &VisualEvidence{
			ManifestSHA256: strings.Repeat("a", 64), FrontendSHA256: strings.Repeat("b", 64), CaptureReceipt: receiptA,
			Captures: []VisualEvidenceCapture{{Artifact: "artifact:sha256:" + strings.Repeat("c", 64), Route: "/", MediaType: "image/png", Bytes: 10, Width: 1280, Height: 800}},
		}
	}
	if err := good().Validate(); err != nil {
		t.Fatal(err)
	}
	var none *VisualEvidence
	if none.Validate() != nil || none.Refs() != nil {
		t.Fatal("a nil record must be valid and empty")
	}
	for name, mutate := range map[string]func(*VisualEvidence){
		"no manifest digest":       func(v *VisualEvidence) { v.ManifestSHA256 = "" },
		"upper-case digest":        func(v *VisualEvidence) { v.FrontendSHA256 = strings.Repeat("B", 64) },
		"receipt of another kind":  func(v *VisualEvidence) { v.CaptureReceipt = "artifact:sha256:" + strings.Repeat("a", 64) },
		"no captures":              func(v *VisualEvidence) { v.Captures = nil },
		"capture not content-addr": func(v *VisualEvidence) { v.Captures[0].Artifact = "file:/etc/passwd" },
		"capture not a PNG":        func(v *VisualEvidence) { v.Captures[0].MediaType = "image/svg+xml" },
		"route off this origin":    func(v *VisualEvidence) { v.Captures[0].Route = "https://example.com/" },
	} {
		v := good()
		mutate(v)
		if err := v.Validate(); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func assertNothingStaged(t *testing.T, temporary string) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(temporary, "evidence")); err == nil {
		t.Fatal("a refused evidence directory left files in the bundle area")
	}
}
