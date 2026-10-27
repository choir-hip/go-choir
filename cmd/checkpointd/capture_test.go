package main

// The operator capture job, `checkpointd capture-base` (S2,
// docs/problems/paper-never-checkpointed-2026-10-27.md). Ways it could fail,
// written before the code:
//
//  1. the capture image outlives the job (a failure path returns early): a
//     128 GiB sparse clone diverging from the live image for days;
//  2. the image is deleted while still mounted, or left mounted;
//  3. a refused publication is advertised anyway;
//  4. a refused advertisement leaves a published, unadvertised base the
//     seeding scan of the next rebuild adopts;
//  5. the report hides the pause, the base or the head it bound.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/projectionbase"
)

type fakeCaptureJob struct {
	t         *testing.T
	image     string
	events    []string
	mountErr  error
	publish   func(projectionbase.CaptureConfig) (*projectionbase.Result, error)
	advertErr error
	retracted []string
}

func (f *fakeCaptureJob) deps() captureDeps {
	return captureDeps{
		capture: func(_ context.Context, computerID string) (captureGrant, error) {
			f.events = append(f.events, "capture "+computerID)
			if err := os.WriteFile(f.image, []byte("clone"), 0o600); err != nil {
				f.t.Fatal(err)
			}
			return captureGrant{VMID: "vm-1", UserID: "owner-1", ComputerID: computerID, CapturePath: f.image, PausedMS: 1200, ClonedMS: 1100}, nil
		},
		mount: func(_ context.Context, image, dir string) (func() error, error) {
			f.events = append(f.events, "mount "+filepath.Base(image))
			if f.mountErr != nil {
				return nil, f.mountErr
			}
			return func() error {
				if _, err := os.Stat(f.image); err != nil {
					f.t.Fatalf("image removed before unmount: %v", err)
				}
				f.events = append(f.events, "unmount")
				return nil
			}, nil
		},
		publish: func(_ context.Context, cfg projectionbase.CaptureConfig) (*projectionbase.Result, error) {
			f.events = append(f.events, "publish "+filepath.Base(cfg.StoreMarker))
			return f.publish(cfg)
		},
		advertise: func(_ context.Context, computerID string, seq uint64, ref string) error {
			f.events = append(f.events, "advertise")
			return f.advertErr
		},
		retract: func(root, ref string) error {
			f.retracted = append(f.retracted, ref)
			return nil
		},
		tapeHeadAt: func(context.Context, string, string, uint64) (string, error) { return "", nil },
	}
}

func captured(seq uint64) func(projectionbase.CaptureConfig) (*projectionbase.Result, error) {
	return func(cfg projectionbase.CaptureConfig) (*projectionbase.Result, error) {
		return &projectionbase.Result{Descriptor: projectionbase.Descriptor{
			ComputerID: cfg.ComputerID, Sequence: seq, CanonicalHead: hex64('a'), BlobSHA256: hex64('b'),
			BlobSizeBytes: 42, Origin: projectionbase.OriginLiveCaptured,
		}}, nil
	}
}

func newFakeCaptureJob(t *testing.T) *fakeCaptureJob {
	return &fakeCaptureJob{t: t, image: filepath.Join(t.TempDir(), "data.img.capture"), publish: captured(4394600)}
}

func runCapture(t *testing.T, f *fakeCaptureJob) (captureReport, error) {
	t.Helper()
	return runCaptureBase(context.Background(), f.deps(), captureOptions{
		ComputerID: "computer-paper", ArtifactsRoot: t.TempDir(), MountRoot: t.TempDir(), Timeout: time.Hour,
	})
}

func imageGone(t *testing.T, f *fakeCaptureJob) {
	t.Helper()
	if _, err := os.Stat(f.image); !os.IsNotExist(err) {
		t.Fatalf("capture image left behind: %v", err)
	}
}

// 1, 2, 5: the happy path, in order, and a report that names everything.
func TestCaptureBasePublishesAdvertisesAndRemovesTheImage(t *testing.T) {
	f := newFakeCaptureJob(t)
	report, err := runCapture(t, f)
	if err != nil {
		t.Fatalf("runCaptureBase: %v", err)
	}
	want := "capture computer-paper,mount data.img.capture,publish state,advertise,unmount"
	if got := strings.Join(f.events, ","); got != want {
		t.Fatalf("events = %s\nwant     %s", got, want)
	}
	imageGone(t, f)
	if report.VMID != "vm-1" || report.PausedMS != 1200 || report.Sequence != 4394600 || report.BaseRef != hex64('b') || report.Origin != projectionbase.OriginLiveCaptured {
		t.Fatalf("report = %+v", report)
	}
}

// 1, 2, 3: a refused publication advertises nothing, unmounts, and removes
// the image.
func TestCaptureBaseRefusedPublicationAdvertisesNothing(t *testing.T) {
	f := newFakeCaptureJob(t)
	f.publish = func(projectionbase.CaptureConfig) (*projectionbase.Result, error) {
		return nil, errors.New("head is not on the tape")
	}
	if _, err := runCapture(t, f); err == nil {
		t.Fatal("refused publication reported success")
	}
	for _, e := range f.events {
		if e == "advertise" {
			t.Fatalf("advertised after a refusal: %v", f.events)
		}
	}
	if f.events[len(f.events)-1] != "unmount" {
		t.Fatalf("not unmounted: %v", f.events)
	}
	imageGone(t, f)
}

// 4: a definitively refused advertisement retracts the published base.
func TestCaptureBaseRefusedAdvertisementRetractsTheBase(t *testing.T) {
	f := newFakeCaptureJob(t)
	f.advertErr = fmt.Errorf("%w: readback shows no watermark", errAdvertiseRefused)
	if _, err := runCapture(t, f); err == nil {
		t.Fatal("refused advertisement reported success")
	}
	if len(f.retracted) != 1 || f.retracted[0] != hex64('b') {
		t.Fatalf("retracted = %v, want the published base", f.retracted)
	}
	imageGone(t, f)
}

// Review 364 (DeepSeek, HIGH): an advertised base never loses its bytes. Any
// advertisement error that is not a definitive refusal keeps the base.
func TestCaptureBaseKeepsTheBaseUnlessTheRefusalIsDefinitive(t *testing.T) {
	for _, advertErr := range []error{
		fmt.Errorf("%w: readback failed", errAdvertiseUnknown),
		errors.New("corpusd advertises a higher watermark"),
		errors.New("status 503"),
	} {
		f := newFakeCaptureJob(t)
		f.advertErr = advertErr
		if _, err := runCapture(t, f); err == nil {
			t.Fatalf("%v: reported success", advertErr)
		}
		if len(f.retracted) != 0 {
			t.Fatalf("%v: retracted %v", advertErr, f.retracted)
		}
	}
}

// advertiseServer answers the watermark POST with post, and a GET readback
// with get (nil: 500). It counts readbacks.
func advertiseServer(t *testing.T, post func(w http.ResponseWriter), get func(w http.ResponseWriter)) (*httptest.Server, *int) {
	t.Helper()
	gets := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/computers/files/watermark" {
			http.NotFound(w, r)
			return
		}
		if r.Method == http.MethodPost {
			post(w)
			return
		}
		gets++
		if get == nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		get(w)
	}))
	t.Cleanup(srv.Close)
	return srv, &gets
}

func answerWatermark(seq uint64, ref string) func(http.ResponseWriter) {
	return func(w http.ResponseWriter) {
		_ = json.NewEncoder(w).Encode(map[string]any{"watermark_sequence": seq, "base_ref": ref})
	}
}

func answerStatus(code int, body string) func(http.ResponseWriter) {
	return func(w http.ResponseWriter) {
		w.WriteHeader(code)
		_, _ = w.Write([]byte(body))
	}
}

// Review 364: on any answer but a clean one naming this base, the
// watermark is read back, and only a definitive absent or lower watermark
// refuses. A failed readback is unknown.
func TestAdvertiseBaseReadsBackEveryUnclearAnswer(t *testing.T) {
	ours, other := hex64('b'), hex64('c')
	const seq = 4394600
	for _, tc := range []struct {
		name      string
		post, get func(http.ResponseWriter)
		ok        bool
		refused   bool
		unknown   bool
		readbacks int
	}{
		{"200 naming the base", answerWatermark(seq, ours), nil, true, false, false, 0},
		{"503, readback names it", answerStatus(503, "busy"), answerWatermark(seq, ours), true, false, false, 1},
		{"200 garbage, readback names it", answerStatus(200, "{not json"), answerWatermark(seq, ours), true, false, false, 1},
		{"409, readback names it", answerStatus(409, "refused"), answerWatermark(seq, ours), true, false, false, 1},
		{"200 higher, readback higher", answerWatermark(seq+5, other), answerWatermark(seq+5, other), false, false, false, 1},
		{"503, readback fails", answerStatus(503, "busy"), nil, false, false, true, 1},
		{"409, readback absent", answerStatus(409, "refused"), answerStatus(404, "404 page not found"), false, true, false, 1},
		{"409, readback lower", answerStatus(409, "refused"), answerWatermark(seq-10, other), false, true, false, 1},
		{"409, readback same sequence, another base", answerStatus(409, "refused"), answerWatermark(seq, other), false, true, false, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv, gets := advertiseServer(t, tc.post, tc.get)
			err := advertiseBase(context.Background(), srv.Client(), srv.URL, "computer-paper", seq, ours)
			if tc.ok != (err == nil) {
				t.Fatalf("err = %v, want ok=%v", err, tc.ok)
			}
			if got := errors.Is(err, errAdvertiseRefused); got != tc.refused {
				t.Fatalf("err = %v: refused = %v, want %v", err, got, tc.refused)
			}
			if got := errors.Is(err, errAdvertiseUnknown); got != tc.unknown {
				t.Fatalf("err = %v: unknown = %v, want %v", err, got, tc.unknown)
			}
			if *gets != tc.readbacks {
				t.Fatalf("readbacks = %d, want %d", *gets, tc.readbacks)
			}
		})
	}
}

// Review 364 (DeepSeek, MEDIUM): a run starts by holding the capture lock
// (a second run refuses), then removes what a killed run left: a stale
// mount under the mount root and a capture image no vmctl pause holds.
func TestCaptureRunSweepsWhatAKilledRunLeft(t *testing.T) {
	mountRoot, stateDir := t.TempDir(), t.TempDir()
	stray := filepath.Join(mountRoot, "vm-old")
	if err := os.MkdirAll(filepath.Join(stray, "state.texture"), 0o700); err != nil {
		t.Fatal(err)
	}
	for _, vm := range []string{"vm-a", "vm-paused"} {
		if err := os.MkdirAll(filepath.Join(stateDir, vm), 0o700); err != nil {
			t.Fatal(err)
		}
		for _, name := range []string{"data.img", "data.img.capture"} {
			if err := os.WriteFile(filepath.Join(stateDir, vm, name), []byte("x"), 0o600); err != nil {
				t.Fatal(err)
			}
		}
	}
	// vmctl holds this one inside a pause; vmctl's own recovery removes it.
	if err := os.WriteFile(filepath.Join(stateDir, "vm-paused", "capture-pause.json"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	var unmounted []string
	sweep := captureSweep{
		isMount: func(dir string) bool { return dir == stray },
		unmount: func(dir string) error {
			unmounted = append(unmounted, dir)
			// What an unmount leaves: the bare mount point.
			return os.RemoveAll(filepath.Join(dir, "state.texture"))
		},
	}
	release, err := sweep.begin(mountRoot, stateDir)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if _, err := sweep.begin(mountRoot, stateDir); err == nil {
		t.Fatal("a second run started while the first holds the lock")
	}
	if len(unmounted) != 1 || unmounted[0] != stray {
		t.Fatalf("unmounted = %v, want the stray mount", unmounted)
	}
	if _, err := os.Lstat(stray); !os.IsNotExist(err) {
		t.Fatalf("stray mount point left: %v", err)
	}
	if _, err := os.Stat(filepath.Join(stateDir, "vm-a", "data.img.capture")); !os.IsNotExist(err) {
		t.Fatalf("stale capture left: %v", err)
	}
	for _, keep := range []string{filepath.Join(stateDir, "vm-a", "data.img"), filepath.Join(stateDir, "vm-paused", "data.img.capture")} {
		if _, err := os.Stat(keep); err != nil {
			t.Fatalf("%s removed: %v", keep, err)
		}
	}
	release()
	again, err := sweep.begin(mountRoot, stateDir)
	if err != nil {
		t.Fatalf("begin after release: %v", err)
	}
	again()
}

// A stray directory that is not a mount, and holds files, is not deleted:
// only an empty mount point is removed.
func TestCaptureRunSweepLeavesAnUnmountedDirectoryWithFiles(t *testing.T) {
	mountRoot, stateDir := t.TempDir(), t.TempDir()
	odd := filepath.Join(mountRoot, "vm-odd")
	if err := os.MkdirAll(odd, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(odd, "f"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	sweep := captureSweep{isMount: func(string) bool { return false }, unmount: func(string) error { t.Fatal("unmounted a non-mount"); return nil }}
	release, err := sweep.begin(mountRoot, stateDir)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	if _, err := os.Stat(filepath.Join(odd, "f")); err != nil {
		t.Fatalf("files under an unmounted directory removed: %v", err)
	}
}

// Review 364 (DeepSeek, HIGH): the guest's image is mounted nosuid, nodev,
// noexec.
func TestLoopMountIsNosuidNodevNoexec(t *testing.T) {
	args := strings.Join(loopMountArgs("/s/data.img.capture", "/m/vm"), " ")
	if args != "-o loop,nosuid,nodev,noexec /s/data.img.capture /m/vm" {
		t.Fatalf("mount args = %q", args)
	}
}

// 1: a mount failure still removes the image.
func TestCaptureBaseMountFailureRemovesTheImage(t *testing.T) {
	f := newFakeCaptureJob(t)
	f.mountErr = errors.New("mount: wrong fs type")
	if _, err := runCapture(t, f); err == nil {
		t.Fatal("mount failure reported success")
	}
	imageGone(t, f)
}

// The retraction removes exactly the base's blob and descriptor sidecar.
func TestRetractPublishedBaseRemovesBlobAndSidecar(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "sha256", projectionbase.Namespace)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	ref, other := hex64('b'), hex64('c')
	for _, name := range []string{ref, projectionbase.DescriptorSidecarName(ref), other, projectionbase.DescriptorSidecarName(other)} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := retractPublishedBase(root, ref); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(dir)
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	if strings.Join(names, ",") != other+","+projectionbase.DescriptorSidecarName(other) {
		t.Fatalf("left %v", names)
	}
}
