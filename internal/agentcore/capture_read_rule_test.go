package agentcore

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

// Enforce flip S1, section 7.2 and FM9, written first. The built-in branch of
// the capture read gate is a read rule bound to capture authority: Texture
// (the owner ruling), the research desk and its child roles, and any desk that
// holds captures.read or a lease ceiling for captures (a management task is
// one) read an owner-private capture without a count. Any other built-in desk,
// engineering included, still reads in S1 and S2 but is counted wrong_reader
// (refused at S3). A package desk stays refused.
//
//	FM9 a built-in desk outside the set reads an owner-private capture: counted.

func TestFM9TheCaptureReadRuleIsBoundToCaptureAuthority(t *testing.T) {
	private := json.RawMessage(`{"capture_id":"cap-private","labels":["owner_private:capture","world_untrusted"]}`)
	counted := func(role string) bool {
		objectgraph.ResetReaderGateCounts()
		if err := gateCaptureRead(context.Background(), role, private); err != nil {
			t.Fatalf("%s: a built-in desk is not refused in S1: %v", role, err)
		}
		for k := range objectgraph.ReaderGateCounts() {
			if strings.Contains(k, "wrong_reader") {
				return true
			}
		}
		return false
	}
	for _, role := range []string{
		deskmanifest.Texture, deskmanifest.ResearcherName(), deskmanifest.ResearchWeb, deskmanifest.ResearchTexture,
		deskmanifest.AdmitterName(), deskmanifest.ManagementTask,
	} {
		if counted(role) {
			t.Errorf("%s holds capture authority and was counted", role)
		}
	}
	for _, role := range []string{deskmanifest.PerformerName(), deskmanifest.Conductor} {
		if !counted(role) {
			t.Errorf("%s holds no capture authority and was not counted (FM9)", role)
		}
	}
	// A world capture is counted for no one.
	world := json.RawMessage(`{"capture_id":"cap-world","labels":["world_untrusted"]}`)
	objectgraph.ResetReaderGateCounts()
	if err := gateCaptureRead(context.Background(), deskmanifest.PerformerName(), world); err != nil || len(objectgraph.ReaderGateCounts()) != 0 {
		t.Errorf("a world capture was counted or refused: %v %v", err, objectgraph.ReaderGateCounts())
	}
}
