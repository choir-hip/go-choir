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
	"errors"
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

// 4: a refused advertisement retracts the published base.
func TestCaptureBaseRefusedAdvertisementRetractsTheBase(t *testing.T) {
	f := newFakeCaptureJob(t)
	f.advertErr = errors.New("projection base is not bound to canonical event at advertised sequence")
	if _, err := runCapture(t, f); err == nil {
		t.Fatal("refused advertisement reported success")
	}
	if len(f.retracted) != 1 || f.retracted[0] != hex64('b') {
		t.Fatalf("retracted = %v, want the published base", f.retracted)
	}
	imageGone(t, f)
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
