package agentcore

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/imageblob"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/visualcapture"
)

// Freeze's visual evidence on the product path
// (docs/design/visual-evidence-slice-2026-10-15.md part 3, V17, V22, V23).
// The capsule stages the captures beside the release; this puts each one in
// the computer's asset store, tells the cell what resolved, and hands the
// runtime's observation to the visual_captured rule through a key the cell
// never sees.

// imageStore is this computer's content-addressed asset store.
func (rt *Runtime) imageStore() *imageblob.Store {
	return imageblob.New(imageblob.DirForStorePath(rt.cfg.StorePath))
}

// visualCaptureGranted says the host granted this computer visual capture
// (the credential-disk grant and Chromium in the image): the desks' overlays
// teach capture and review only then.
func (rt *Runtime) visualCaptureGranted() bool {
	return rt != nil && rt.capsuleExecutor != nil && rt.capsuleExecutor.VisualCaptureStatus().Granted
}

// freezeVisualKey carries the Freeze's visual observation from
// freezeCapsuleEffectBundle to settleFreezePrecommits, which removes it
// before the result reaches the cell (the model cannot supply it, V23).
const freezeVisualKey = "_rule_visual"

// freezeVisualObservation is what a fresh Freeze saw of the evidence.
type freezeVisualObservation struct {
	Receipt  string
	Captures []types.VisualCaptureObservation
}

// visualAssetStore is the asset store's write side (imageblob.Store).
type visualAssetStore interface {
	Put(data []byte) (ref, mediaType string, err error)
}

// recordFreezeVisual stores the staged captures and fills the Freeze result.
// stagedRoot is the staged bundle directory; visual is the capsule's record of
// the evidence (nil when it captured none). A store failure does not fail the
// Freeze, because the bundle is the record: the result names it and each
// capture's resolvable flag says what the store holds (V22).
func recordFreezeVisual(images visualAssetStore, stagedRoot string, visual *capsule.VisualEvidence, out map[string]any) {
	observation := freezeVisualObservation{}
	if visual != nil {
		observation.Receipt = visual.CaptureReceipt
		var rows []map[string]any
		var failures []string
		var shots []visualcapture.RouteShot
		for _, capture := range visual.Captures {
			shots = append(shots, visualcapture.RouteShot{Route: capture.Route, Artifact: capture.Artifact})
			resolvable := false
			if err := storeStagedCapture(images, stagedRoot, capture); err != nil {
				failures = append(failures, fmt.Sprintf("%s: %v", capture.Artifact, err))
			} else {
				resolvable = true
			}
			rows = append(rows, map[string]any{
				"artifact": capture.Artifact, "route": capture.Route, "width": capture.Width, "height": capture.Height,
				"bytes": capture.Bytes, "blank": capture.Blank, "resolvable": resolvable,
			})
			observation.Captures = append(observation.Captures, types.VisualCaptureObservation{Route: capture.Route, Artifact: capture.Artifact, Blank: capture.Blank})
		}
		out["visual_captures"] = rows
		out["visual_capture_receipt"] = visual.CaptureReceipt
		if len(failures) > 0 {
			out["visual_store_errors"] = failures
		}
		if warnings := visualcapture.SharedShotWarnings(shots); len(warnings) > 0 {
			out["visual_warnings"] = warnings
		}
	}
	out[freezeVisualKey] = observation
}

// storeStagedCapture puts one staged capture in the asset store, checking the
// store's ref is the cited one so swapped bytes are never filed under the
// cited digest.
func storeStagedCapture(images visualAssetStore, stagedRoot string, capture capsule.VisualEvidenceCapture) error {
	digest, ok := strings.CutPrefix(capture.Artifact, "artifact:sha256:")
	if !ok || images == nil {
		return fmt.Errorf("no asset store or a malformed capture ref")
	}
	data, err := os.ReadFile(filepath.Join(stagedRoot, filepath.FromSlash(visualcapture.BundleDir), digest+".png"))
	if err != nil {
		return fmt.Errorf("staged capture unreadable: %w", err)
	}
	ref, _, err := images.Put(data)
	if err != nil {
		return fmt.Errorf("asset store write: %w", err)
	}
	if ref != capture.Artifact {
		return fmt.Errorf("staged bytes are not the cited capture (stored as %s)", ref)
	}
	return nil
}

// applyFreezeVisual moves the Freeze's visual observation from the result
// into the rule observation and removes the key. A freeze that carries none
// (a replay) leaves VisualKnown false, so visual_captured stays open.
func applyFreezeVisual(obs *types.EngineeringFreezeObservation, out map[string]any) {
	observed, ok := out[freezeVisualKey].(freezeVisualObservation)
	delete(out, freezeVisualKey)
	if !ok {
		return
	}
	obs.VisualKnown = true
	obs.VisualReceipt = observed.Receipt
	obs.VisualCaptures = observed.Captures
}
