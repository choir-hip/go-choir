package agentcore

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/pkgrelease"
	"github.com/yusefmosiah/go-choir/internal/platform"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
)

// P1 guest side (docs/design/package-operation-decision-2026-11-06.md 2.1,
// 2.2): the guest classifies by the frozen patch with the effects in
// agreement, claims a platform base, and accepts a package decision only when
// its draft content digest is the one this computer derived itself.

func packageBundleWithEffects(t *testing.T, effects ...string) []byte {
	t.Helper()
	var files []map[string]string
	for _, e := range effects {
		files = append(files, map[string]string{"path": e})
	}
	raw, err := json.Marshal(map[string]any{"ordered_file_effects": files})
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestStandingClassifiesByPatchAndChecksTheDecisionsDraftDigest(t *testing.T) {
	w := newDraftWorld(t)
	w.bareOperation("op-pkg", selfdev.StateAwaitingApproval, map[string]string{draftRoot + "/App.svelte": "<p>v2</p>\n"})
	ctx := context.Background()
	operation, err := w.rt.selfdevOperations.Get(ctx, draftComputer, "op-pkg")
	if err != nil {
		t.Fatal(err)
	}
	effect := "workspace/platform/" + draftRoot + "/App.svelte"
	c, err := w.rt.standingClassify(ctx, operation, packageBundleWithEffects(t, effect))
	if err != nil || c.kind != platform.ConsensusKindPackage || c.name != "reader" || c.claimedBase != w.base || len(c.patch) == 0 {
		t.Fatalf("classification = %+v, %v", c, err)
	}
	// An effect beyond the patch's package is a mixed bundle.
	if _, err := w.rt.standingClassify(ctx, operation, packageBundleWithEffects(t, effect, "workspace/platform/frontend/src/app.css")); err == nil || !strings.Contains(err.Error(), "package_path_refused") {
		t.Fatalf("mixed effects: %v", err)
	}

	// The decision the poster would mint: its draft digest is what the draft
	// of the same patch is, on the parent it names.
	src, err := pkgrelease.DraftPackage(ctx, pkgrelease.DraftInput{Repo: w.repo, Name: "reader", Base: c.patchBase, Patch: c.patch, PlatformBase: c.claimedBase})
	if err != nil {
		t.Fatal(err)
	}
	digest, err := src.Facts("reader").Digest()
	if err != nil {
		t.Fatal(err)
	}
	decision := func(mutate func(block map[string]any)) *computerevent.Receipt {
		block := map[string]any{
			"name": "reader", "draft_digest": digest, "source_patch_sha256": c.sourcePatchSHA256, "claimed_platform_base": c.claimedBase,
			"parent": map[string]any{"kind": src.Parent.Kind, "commit": src.Parent.Commit, "tree": src.Parent.Tree},
		}
		if mutate != nil {
			mutate(block)
		}
		return &computerevent.Receipt{KindFields: map[string]any{"kind": platform.ConsensusKindPackage, "package": block}}
	}
	if err := w.rt.verifyPackageDecisionDraft(ctx, c, decision(nil)); err != nil {
		t.Fatalf("the decision for this draft: %v", err)
	}
	for name, mutate := range map[string]func(map[string]any){
		"another draft digest": func(b map[string]any) { b["draft_digest"] = strings.Repeat("f", 64) },
		"another patch":        func(b map[string]any) { b["source_patch_sha256"] = strings.Repeat("e", 64) },
		"another base":         func(b map[string]any) { b["claimed_platform_base"] = strings.Repeat("d", 40) },
		"another package":      func(b map[string]any) { b["name"] = "other" },
	} {
		if err := w.rt.verifyPackageDecisionDraft(ctx, c, decision(mutate)); err == nil {
			t.Fatalf("%s was accepted", name)
		}
	}
	// A platform decision is no answer to a package request, and the reverse.
	platformDecision := &computerevent.Receipt{KindFields: map[string]any{"host_build": map[string]any{"job_id": strings.Repeat("a", 64)}}}
	if err := w.rt.verifyPackageDecisionDraft(ctx, c, platformDecision); err == nil {
		t.Fatal("a platform decision answered a package request")
	}
	if consensusDecisionKind(platformDecision) != platform.ConsensusKindPlatform || consensusDecisionKind(decision(nil)) != platform.ConsensusKindPackage {
		t.Fatal("decision kinds misread")
	}
}
