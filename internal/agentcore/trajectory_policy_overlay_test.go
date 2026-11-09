package agentcore

// A model policy overlay follows its trajectory (L4 benchmark, 2026-11-09:
// the overlay chosen for a prompt reached only the Texture desk; research
// casts stayed on the default pool). Failure modes, written first:
//
//  1. the overlay names research, and research does not use it
//  2. the overlay names only texture, and research leaves its default
//  3. no overlay on the trajectory changes anything
//  4. a run's own overlay is replaced by the trajectory's
//  5. a restart loses the overlay (the ref must be durable on the tape)
//  6. an overlay that no longer resolves refuses or breaks the run instead of
//     falling back to the default with the error visible

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/modelpolicy"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func seedOverlayTrajectory(t *testing.T, s *store.Store, owner, docID, overlayID string) string {
	t.Helper()
	agentID := currentTextureAgentID(docID)
	now := time.Now().UTC()
	refs := map[string]string{"artifact": "texture://documents/" + docID}
	if overlayID != "" {
		refs[types.TrajectoryRefModelPolicyOverlay] = overlayID
	}
	req := types.StartLifecycleRequest{
		OwnerID: owner, ComputerID: "autoputer-test", CommandID: "overlay-start:" + docID,
		TrajectoryID: "overlay-trajectory:" + docID, Kind: types.TrajectoryKindTask,
		SettlementRule:  types.SettlementRule{Version: types.LifecycleReducerVersion, RequireNoOpenWorkItems: true, RequiredSubjectRefs: []string{"artifact"}},
		SubjectRefs:     refs,
		InitialWork:     types.WorkItemRecord{WorkItemID: "overlay-work:" + docID, Objective: "x", AssignedAgentID: agentID},
		InitialDocument: types.Document{DocID: docID, Title: "Overlay"},
		InitialRevision: types.Revision{RevisionID: "overlay-rev:" + docID, AuthorKind: types.AuthorUser, AuthorLabel: owner, Content: "x"},
		Agent:           types.AgentRecord{AgentID: agentID, OwnerID: owner, ComputerID: "autoputer-test", Profile: "texture", Role: "texture", ChannelID: docID, CreatedAt: now, UpdatedAt: now},
	}
	req.StartRequestDigest, _ = store.ComputeStartLifecycleRequestDigest(req)
	if _, err := s.StartLifecycle(context.Background(), req); err != nil {
		t.Fatalf("start trajectory: %v", err)
	}
	return req.TrajectoryID
}

func overlayPolicyManager(t *testing.T) *modelpolicy.Manager {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "System", "model-policy.toml")
	odir := filepath.Join(dir, "System", "model-policy-overlays")
	if err := os.MkdirAll(odir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("[defaults]\nprovider = \"chatgpt\"\nmodel = \"gpt-6-luna\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(odir, name+".toml"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("both", "[overlay]\nexpires_at = \"2099-01-01T00:00:00Z\"\n\n[roles.texture]\nprovider = \"synthetic\"\nmodel = \"hf:deepseek-ai/DeepSeek-V4.1-Flash-1017\"\n\n[roles.research]\nprovider = \"synthetic\"\nmodel = \"hf:deepseek-ai/DeepSeek-V4.1-Flash-1017\"\n")
	write("texonly", "[overlay]\nexpires_at = \"2099-01-01T00:00:00Z\"\n\n[roles.texture]\nprovider = \"synthetic\"\nmodel = \"hf:deepseek-ai/DeepSeek-V4.1-Flash-1017\"\n")
	write("gone", "[overlay]\nexpires_at = \"2000-01-01T00:00:00Z\"\n\n[roles.research]\nprovider = \"synthetic\"\nmodel = \"hf:deepseek-ai/DeepSeek-V4.1-Flash-1017\"\n")
	return modelpolicy.NewManager(modelpolicy.ManagerConfig{PolicyPath: path})
}

func resolveForCast(rt *Runtime, owner, trajectoryID, role string, own map[string]any) map[string]any {
	md := map[string]any{runMetadataTrajectoryID: trajectoryID, runMetadataAgentProfile: role, runMetadataAgentRole: role}
	for k, v := range own {
		md[k] = v
	}
	md, _ = rt.inheritTrajectoryPolicyOverlay(context.Background(), owner, md)
	return rt.modelPolicy.EnrichMetadata(context.Background(), owner, role, md)
}

func TestTrajectoryPolicyOverlayFollowsCasts(t *testing.T) {
	rt, s := testRuntime(t)
	rt.modelPolicy = overlayPolicyManager(t)
	const owner = "owner-overlay"
	model := func(md map[string]any) string { return metadataStringValue(md, modelpolicy.MetadataModel) }

	base := model(rt.modelPolicy.EnrichMetadata(context.Background(), owner, "research", map[string]any{}))
	if base == "hf:deepseek-ai/DeepSeek-V4.1-Flash-1017" {
		t.Fatalf("baseline must differ from the overlay model")
	}
	both := seedOverlayTrajectory(t, s, owner, "doc-both", "both")
	if got := model(resolveForCast(rt, owner, both, "research", nil)); got != "hf:deepseek-ai/DeepSeek-V4.1-Flash-1017" {
		t.Fatalf("1: research model = %q", got)
	}
	texonly := seedOverlayTrajectory(t, s, owner, "doc-texonly", "texonly")
	if got := model(resolveForCast(rt, owner, texonly, "research", nil)); got != base {
		t.Fatalf("2: research model = %q", got)
	}
	if got := model(resolveForCast(rt, owner, texonly, "texture", nil)); got != "hf:deepseek-ai/DeepSeek-V4.1-Flash-1017" {
		t.Fatalf("2: texture model = %q", got)
	}
	none := seedOverlayTrajectory(t, s, owner, "doc-none", "")
	md := resolveForCast(rt, owner, none, "research", nil)
	if model(md) != base || metadataStringValue(md, modelpolicy.MetadataPolicyOverlayID) != "" {
		t.Fatalf("3: no-overlay metadata = %+v", md)
	}
	if got := model(resolveForCast(rt, owner, both, "research", map[string]any{modelpolicy.MetadataPolicyOverlayID: "texonly"})); got != base {
		t.Fatalf("4: own overlay replaced, research model = %q", got)
	}
	// 5: a restarted runtime over the same store resolves the same overlay.
	peer := testPeerRuntime(t, rt, s)
	peer.modelPolicy = rt.modelPolicy
	if got := model(resolveForCast(peer, owner, both, "research", nil)); got != "hf:deepseek-ai/DeepSeek-V4.1-Flash-1017" {
		t.Fatalf("5: after restart research model = %q", got)
	}
	// 6: an expired overlay falls back to the default, error visible.
	gone := seedOverlayTrajectory(t, s, owner, "doc-gone", "gone")
	md = resolveForCast(rt, owner, gone, "research", nil)
	if model(md) != base || metadataStringValue(md, modelpolicy.MetadataPolicyError) == "" {
		t.Fatalf("6: expired overlay metadata = %+v", md)
	}
}
