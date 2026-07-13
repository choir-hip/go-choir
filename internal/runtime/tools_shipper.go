package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/agentprofile"
)

type publishPackageHumanProofInput struct {
	Summary           string
	HumanSummary      string
	Recommendation    string
	TextureDocID      string
	TextureRevisionID string
	ScreenshotRefs    []string
	VideoRefs         []string
	BenchmarkRefs     []string
	ArtifactRefs      []string
	BehaviorContract  string
}

func RegisterShipperTools(registry *toolregistry.ToolRegistry, rt *Runtime, cwd string) error {
	for _, tool := range []Tool{
		newPublishAppChangePackageTool(rt, cwd),
	} {
		if err := registry.Register(tool); err != nil {
			return err
		}
	}
	return nil
}

func newPublishAppChangePackageTool(rt *Runtime, cwd string) Tool {
	type args struct {
		RepoPath                 string   `json:"repo_path"`
		BaseSHA                  string   `json:"base_sha"`
		AppID                    string   `json:"app_id,omitempty"`
		Visibility               string   `json:"visibility,omitempty"`
		SourceComputerID         string   `json:"source_computer_id,omitempty"`
		SourceCandidateID        string   `json:"source_candidate_id,omitempty"`
		SourceActiveRef          string   `json:"source_active_ref,omitempty"`
		CandidateSourceRef       string   `json:"candidate_source_ref,omitempty"`
		SourceLedgerRepo         string   `json:"source_ledger_repo,omitempty"`
		SourceLedgerBaseRef      string   `json:"source_ledger_base_ref,omitempty"`
		SourceLedgerCandidateRef string   `json:"source_ledger_candidate_ref,omitempty"`
		AppProtocolContract      string   `json:"app_protocol_contract,omitempty"`
		TraceID                  string   `json:"trace_id,omitempty"`
		Summary                  string   `json:"summary,omitempty"`
		HumanSummary             string   `json:"human_summary,omitempty"`
		Recommendation           string   `json:"recommendation,omitempty"`
		TextureDocID             string   `json:"texture_doc_id,omitempty"`
		TextureRevisionID        string   `json:"texture_revision_id,omitempty"`
		ScreenshotRefs           []string `json:"screenshot_refs,omitempty"`
		VideoRefs                []string `json:"video_refs,omitempty"`
		BenchmarkRefs            []string `json:"benchmark_refs,omitempty"`
		ArtifactRefs             []string `json:"artifact_refs,omitempty"`
		BehaviorContract         string   `json:"behavior_contract,omitempty"`
	}
	return Tool{
		Name:        "publish_app_change_package",
		Description: "Publish committed candidate repo changes as an AppChangePackage source delta for recipient rebuild/adoption. Include human_summary plus Texture/screenshot/video/benchmark refs when the change is intended to be owner-reviewable. Build receipts, npm/go build success, unavailable screenshots/videos, and recommendation prose are not human proof; if real screenshots/video or measured behavior benchmarks are missing, publish honestly as evidence_pending. This tool cannot push to GitHub or promote active state.",
		Parameters: jsonSchemaObject(map[string]any{
			"repo_path":                   map[string]any{"type": "string"},
			"base_sha":                    map[string]any{"type": "string"},
			"app_id":                      map[string]any{"type": "string"},
			"visibility":                  map[string]any{"type": "string", "enum": []string{"private", "unlisted", "public"}},
			"source_computer_id":          map[string]any{"type": "string"},
			"source_candidate_id":         map[string]any{"type": "string"},
			"source_active_ref":           map[string]any{"type": "string"},
			"candidate_source_ref":        map[string]any{"type": "string"},
			"source_ledger_repo":          map[string]any{"type": "string"},
			"source_ledger_base_ref":      map[string]any{"type": "string"},
			"source_ledger_candidate_ref": map[string]any{"type": "string"},
			"app_protocol_contract":       map[string]any{"type": "string"},
			"trace_id":                    map[string]any{"type": "string"},
			"summary":                     map[string]any{"type": "string"},
			"human_summary":               map[string]any{"type": "string"},
			"recommendation":              map[string]any{"type": "string"},
			"texture_doc_id":              map[string]any{"type": "string"},
			"texture_revision_id":         map[string]any{"type": "string"},
			"screenshot_refs":             map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"video_refs":                  map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"benchmark_refs":              map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"artifact_refs":               map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"behavior_contract":           map[string]any{"type": "string"},
		}, []string{"repo_path", "base_sha"}, false),
		Func: func(ctx context.Context, raw json.RawMessage) (string, error) {
			profile := toolregistry.ExecutionContextFrom(ctx).Profile
			if profile != agentprofile.Super && profile != agentprofile.CoSuper && profile != agentprofile.VSuper {
				return "", fmt.Errorf("publish_app_change_package is only available to super, co-super, and vsuper agents")
			}
			if profile == agentprofile.CoSuper {
				if rec := toolregistry.ExecutionContextFrom(ctx).RunRecord; rec != nil {
					if slot := normalizeVSuperCoSuperSlot(metadataStringValue(rec.Metadata, runMetadataCoSuperSlot)); slot == "verifier" {
						return "", fmt.Errorf("verifier co-super cannot publish_app_change_package; verifiers may write scratch tests/evidence and must report pass/fail to the implementation worker or vsuper")
					}
				}
			}
			if err := guardForegroundSuperMutation(ctx, "publish_app_change_package"); err != nil {
				return "", err
			}
			var in args
			if err := json.Unmarshal(raw, &in); err != nil {
				return "", fmt.Errorf("decode publish_app_change_package args: %w", err)
			}
			runID := toolregistry.ExecutionContextFrom(ctx).RunID
			if profile == agentprofile.VSuper && rt != nil && runID != "" {
				if reusedPackage, found, err := rt.latestTrajectoryCoSuperAppChangePackage(ctx, toolregistry.ExecutionContextFrom(ctx).RunRecord); err != nil {
					return "", err
				} else if found {
					reusedPackage["requested_by_run_id"] = runID
					reusedPackage["reused_coagent_package"] = true
					return toolResultJSON(reusedPackage)
				}
			}
			baseCWD := effectiveToolCWD(ctx, cwd)
			repoPath, err := resolveToolPath(baseCWD, in.RepoPath)
			if err != nil {
				return "", fmt.Errorf("repo_path: %w", err)
			}
			baseSHA := strings.TrimSpace(in.BaseSHA)
			if baseSHA == "" {
				return "", fmt.Errorf("base_sha is required")
			}
			traceID := runID
			if rec := toolregistry.ExecutionContextFrom(ctx).RunRecord; rec != nil && rec.Metadata != nil {
				if id, _ := rec.Metadata[runMetadataTrajectoryID].(string); strings.TrimSpace(id) != "" {
					traceID = strings.TrimSpace(id)
				}
			}
			if strings.TrimSpace(in.TraceID) != "" {
				traceID = strings.TrimSpace(in.TraceID)
			}
			headSHA, err := gitOutputInDir(ctx, repoPath, "rev-parse", "HEAD")
			if err != nil {
				return "", fmt.Errorf("resolve candidate head: %w", err)
			}
			runtimeDelta, err := gitDiffInDir(ctx, repoPath, baseSHA, "HEAD", ".", ":(exclude)frontend")
			if err != nil {
				return "", fmt.Errorf("runtime source delta: %w", err)
			}
			uiDelta, err := gitDiffInDir(ctx, repoPath, baseSHA, "HEAD", "frontend")
			if err != nil {
				return "", fmt.Errorf("ui source delta: %w", err)
			}
			if strings.TrimSpace(runtimeDelta) == "" && strings.TrimSpace(uiDelta) == "" {
				return "", fmt.Errorf("no source delta found between %s and HEAD", baseSHA)
			}
			ownerID := toolregistry.ExecutionContextFrom(ctx).OwnerID
			if ownerID == "" {
				return "", fmt.Errorf("publish_app_change_package requires owner context")
			}
			if rt == nil {
				return "", fmt.Errorf("publish_app_change_package requires runtime")
			}
			sourceComputerID := firstNonEmpty(
				strings.TrimSpace(in.SourceComputerID),
				toolregistry.ExecutionContextFrom(ctx).DesktopID,
				toolregistry.ExecutionContextFrom(ctx).SandboxID,
				"candidate-computer",
			)
			sourceCandidateID := firstNonEmpty(strings.TrimSpace(in.SourceCandidateID), runID, sanitizeExportPart(headSHA))
			appID := firstNonEmpty(explicitAppChangePackageAppID(ctx), strings.TrimSpace(in.AppID), "computer-change")
			visibility := firstNonEmpty(explicitAppChangePackageVisibility(ctx), strings.TrimSpace(in.Visibility), "unlisted")
			candidateSourceRef := strings.TrimSpace(in.CandidateSourceRef)
			sourceLedgerCandidateRef := strings.TrimSpace(in.SourceLedgerCandidateRef)
			if candidateSourceRef != "" && !isProductCandidateSourceRef(candidateSourceRef) {
				if sourceLedgerCandidateRef == "" {
					sourceLedgerCandidateRef = candidateSourceRef
				}
				candidateSourceRef = ""
			}
			contract := strings.TrimSpace(in.AppProtocolContract)
			if contract == "" {
				contract = "recipient_build_required: " + firstNonEmpty(strings.TrimSpace(in.Summary), "candidate source change requires recipient Go/Svelte rebuild")
			}
			proofInput := publishPackageHumanProofInput{
				Summary:           in.Summary,
				HumanSummary:      in.HumanSummary,
				Recommendation:    in.Recommendation,
				TextureDocID:      in.TextureDocID,
				TextureRevisionID: in.TextureRevisionID,
				ScreenshotRefs:    in.ScreenshotRefs,
				VideoRefs:         in.VideoRefs,
				BenchmarkRefs:     in.BenchmarkRefs,
				ArtifactRefs:      in.ArtifactRefs,
				BehaviorContract:  in.BehaviorContract,
			}
			provenanceRefs := humanProofProvenanceRefs(proofInput)
			verifierContracts := humanProofVerifierContracts(proofInput)
			rec, err := rt.PublishAppChangePackage(ctx, ownerID, publishAppChangePackageInput{
				AppID:                       appID,
				Visibility:                  visibility,
				SourceComputerID:            sourceComputerID,
				SourceCandidateID:           sourceCandidateID,
				SourceActiveRef:             strings.TrimSpace(in.SourceActiveRef),
				CandidateSourceRef:          candidateSourceRef,
				SourceLedgerRepo:            strings.TrimSpace(in.SourceLedgerRepo),
				SourceLedgerBaseRef:         firstNonEmpty(strings.TrimSpace(in.SourceLedgerBaseRef), baseSHA),
				SourceLedgerCandidateRef:    sourceLedgerCandidateRef,
				SourceLedgerCommitSHA:       strings.TrimSpace(headSHA),
				RuntimeSourceDelta:          runtimeDelta,
				UISourceDelta:               uiDelta,
				AppProtocolContract:         contract,
				SourceRuntimeArtifactDigest: "sha256:" + digestParts("source-runtime", ownerID, sourceComputerID, sourceCandidateID, sha256Hex(runtimeDelta), headSHA),
				SourceUIArtifactDigest:      "sha256:" + digestParts("source-ui", ownerID, sourceComputerID, sourceCandidateID, sha256Hex(uiDelta), headSHA),
				VerifierContracts:           verifierContracts,
				ProvenanceRefs:              provenanceRefs,
				TraceID:                     traceID,
			})
			if err != nil {
				return "", err
			}

			return toolResultJSON(map[string]any{
				"status":                         rec.Status,
				"package_id":                     rec.PackageID,
				"app_id":                         rec.AppID,
				"visibility":                     rec.Visibility,
				"run_id":                         runID,
				"trace_id":                       traceID,
				"base_sha":                       baseSHA,
				"candidate_head_sha":             headSHA,
				"source_computer_id":             rec.SourceComputerID,
				"source_candidate_id":            rec.SourceCandidateID,
				"candidate_source_ref":           rec.CandidateSourceRef,
				"package_manifest_sha256":        rec.PackageManifestSHA256,
				"runtime_source_delta_sha256":    rec.RuntimeSourceDeltaSHA256,
				"ui_source_delta_sha256":         rec.UISourceDeltaSHA256,
				"runtime_source_delta_present":   strings.TrimSpace(rec.RuntimeSourceDelta) != "",
				"ui_source_delta_present":        strings.TrimSpace(rec.UISourceDelta) != "",
				"recipient_build_required":       true,
				"source_runtime_artifact_digest": rec.SourceRuntimeArtifactDigest,
				"source_ui_artifact_digest":      rec.SourceUIArtifactDigest,
				"human_proof_state":              humanProofForAppChangePackage(rec).State,
				"github_push":                    false,
			})
		},
	}
}

func explicitAppChangePackageAppID(ctx context.Context) string {
	prompt := appChangePackageRunPrompt(ctx)
	for _, marker := range []string{`app_id "`, `app_id: "`, "`app_id`: `", "app_id `"} {
		if value := quotedValueAfter(prompt, marker); value != "" {
			return value
		}
	}
	return ""
}

func explicitAppChangePackageVisibility(ctx context.Context) string {
	prompt := strings.ToLower(appChangePackageRunPrompt(ctx))
	for _, visibility := range []string{"unlisted", "public", "private"} {
		if strings.Contains(prompt, `visibility "`+visibility+`"`) ||
			strings.Contains(prompt, `visibility: `+visibility) ||
			strings.Contains(prompt, `visibility `+visibility) ||
			strings.Contains(prompt, "`visibility`: `"+visibility+"`") {
			return visibility
		}
	}
	return ""
}

func appChangePackageRunPrompt(ctx context.Context) string {
	if rec := toolregistry.ExecutionContextFrom(ctx).RunRecord; rec != nil {
		return rec.Prompt
	}
	return ""
}

func quotedValueAfter(text, marker string) string {
	idx := strings.Index(text, marker)
	if idx < 0 {
		return ""
	}
	rest := text[idx+len(marker):]
	terminators := []string{`"`, "`", "\n", " ", ","}
	end := len(rest)
	for _, term := range terminators {
		if term == "" {
			continue
		}
		if pos := strings.Index(rest, term); pos >= 0 && pos < end {
			end = pos
		}
	}
	return strings.TrimSpace(rest[:end])
}

func humanProofProvenanceRefs(in publishPackageHumanProofInput) json.RawMessage {
	payload := map[string]any{
		"summary":             strings.TrimSpace(firstNonEmpty(in.HumanSummary, in.Summary)),
		"recommendation":      strings.TrimSpace(in.Recommendation),
		"texture_doc_id":      strings.TrimSpace(in.TextureDocID),
		"texture_revision_id": strings.TrimSpace(in.TextureRevisionID),
		"screenshot_refs":     compactStringRefs(in.ScreenshotRefs),
		"video_refs":          compactStringRefs(in.VideoRefs),
		"benchmark_refs":      compactStringRefs(in.BenchmarkRefs),
		"artifact_refs":       compactStringRefs(in.ArtifactRefs),
		"behavior_contract":   strings.TrimSpace(in.BehaviorContract),
		"generated_by_tool":   "publish_app_change_package",
		"human_proof_version": "v1",
	}
	data, _ := json.Marshal(payload)
	return data
}

func humanProofVerifierContracts(in publishPackageHumanProofInput) json.RawMessage {
	contracts := []map[string]any{
		{
			"name":     "recipient-build-required",
			"state":    "required",
			"evidence": "AppChangePackage must be rebuilt inside the recipient computer before install.",
		},
	}
	if strings.TrimSpace(in.BehaviorContract) != "" {
		contracts = append(contracts, map[string]any{
			"name":     "human-behavior-proof",
			"state":    "required",
			"evidence": strings.TrimSpace(in.BehaviorContract),
		})
	}
	data, _ := json.Marshal(contracts)
	return data
}

func isProductCandidateSourceRef(ref string) bool {
	return strings.Contains(strings.TrimSpace(ref), "/candidates/")
}

func gitDiffInDir(ctx context.Context, dir, base, head string, paths ...string) (string, error) {
	args := []string{"diff", "--binary", strings.TrimSpace(base) + ".." + strings.TrimSpace(head), "--"}
	args = append(args, paths...)
	return gitOutputInDir(ctx, dir, args...)
}

func sanitizeExportPart(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "run"
	}
	var b strings.Builder
	for _, r := range raw {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.' || r == '_' || r == '-' {
			b.WriteRune(r)
			continue
		}
		if b.Len() == 0 || b.String()[b.Len()-1] == '-' {
			continue
		}
		b.WriteByte('-')
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "run"
	}
	return out
}
