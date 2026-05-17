package runtime

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/shipper"
)

const maxInlinePatchsetBytes = 2 * 1024 * 1024

func RegisterShipperTools(registry *ToolRegistry, rt *Runtime, cwd string) error {
	for _, tool := range []Tool{
		newExportPatchsetTool(rt, cwd),
	} {
		if err := registry.Register(tool); err != nil {
			return err
		}
	}
	return nil
}

func newExportPatchsetTool(rt *Runtime, cwd string) Tool {
	type args struct {
		RepoPath   string   `json:"repo_path"`
		OutputDir  string   `json:"output_dir,omitempty"`
		BaseSHA    string   `json:"base_sha"`
		SnapshotID string   `json:"snapshot_id,omitempty"`
		Summary    string   `json:"summary,omitempty"`
		Checks     []string `json:"checks,omitempty"`
	}
	return Tool{
		Name:        "export_patchset",
		Description: "Export committed worker repo changes as a patchset plus manifest. This tool cannot push to GitHub.",
		Parameters: jsonSchemaObject(map[string]any{
			"repo_path":   map[string]any{"type": "string"},
			"output_dir":  map[string]any{"type": "string"},
			"base_sha":    map[string]any{"type": "string"},
			"snapshot_id": map[string]any{"type": "string"},
			"summary":     map[string]any{"type": "string"},
			"checks": map[string]any{
				"type":  "array",
				"items": map[string]any{"type": "string"},
			},
		}, []string{"repo_path", "base_sha"}, false),
		Func: func(ctx context.Context, raw json.RawMessage) (string, error) {
			profile := stringFromToolContext(ctx, toolCtxProfile)
			if profile != AgentProfileSuper && profile != AgentProfileCoSuper && profile != AgentProfileVSuper {
				return "", fmt.Errorf("export_patchset is only available to super, co-super, and vsuper agents")
			}
			if err := guardForegroundSuperMutation(ctx, "export_patchset"); err != nil {
				return "", err
			}
			var in args
			if err := json.Unmarshal(raw, &in); err != nil {
				return "", fmt.Errorf("decode export_patchset args: %w", err)
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

			runID := stringFromToolContext(ctx, toolCtxRunID)
			traceID := runID
			if rec := ctxRunRecord(ctx); rec != nil && rec.Metadata != nil {
				if id, _ := rec.Metadata[runMetadataTrajectoryID].(string); strings.TrimSpace(id) != "" {
					traceID = strings.TrimSpace(id)
				}
			}
			vmID := stringFromToolContext(ctx, toolCtxSandboxID)
			if vmID == "" {
				vmID = "unknown-sandbox"
			}
			if profile == AgentProfileVSuper && rt != nil && rt.store != nil {
				if childExport, found, err := rt.latestChildExportPatchset(ctx, runID); err != nil {
					return "", err
				} else if found {
					childExport["reused_child_export"] = true
					childExport["parent_loop_id"] = runID
					childExport["trace_id"] = traceID
					if vmID != "" {
						childExport["vm_id"] = vmID
					}
					return toolResultJSON(childExport)
				}
			}

			outputDir := strings.TrimSpace(in.OutputDir)
			if outputDir == "" {
				outputDir = filepath.Join(".choir", "exports", sanitizeExportPart(runID))
			}
			outputPath, err := resolveToolPath(baseCWD, outputDir)
			if err != nil {
				return "", fmt.Errorf("output_dir: %w", err)
			}

			report, err := shipper.ExportPatchset(ctx, shipper.ExportOptions{
				RepoPath:   repoPath,
				OutputDir:  outputPath,
				BaseSHA:    baseSHA,
				RunID:      runID,
				TraceID:    traceID,
				VMID:       vmID,
				SnapshotID: strings.TrimSpace(in.SnapshotID),
				Summary:    strings.TrimSpace(in.Summary),
				Checks:     in.Checks,
			})
			if err != nil {
				return "", err
			}

			result := map[string]any{
				"status":          report.Status,
				"run_id":          runID,
				"trace_id":        traceID,
				"vm_id":           vmID,
				"snapshot_id":     strings.TrimSpace(in.SnapshotID),
				"base_sha":        report.BaseSHA,
				"worker_head":     report.HeadSHA,
				"worker_head_sha": report.HeadSHA,
				"manifest_path":   report.ManifestPath,
				"patchset_path":   report.PatchsetPath,
				"checks":          report.Checks,
				"exported_at":     report.ExportedAt,
				"github_push":     false,
			}
			if manifestContent, ok, truncated, err := readInlineArtifact(report.ManifestPath); err != nil {
				return "", err
			} else if ok {
				result["manifest_json"] = manifestContent
			} else if truncated {
				result["manifest_inline_truncated"] = true
			}
			if patchsetContent, ok, truncated, err := readInlineArtifact(report.PatchsetPath); err != nil {
				return "", err
			} else if ok {
				sum := sha256.Sum256([]byte(patchsetContent))
				result["patchset_content"] = patchsetContent
				result["patchset_sha256"] = hex.EncodeToString(sum[:])
			} else if truncated {
				result["patchset_inline_truncated"] = true
			}

			return toolResultJSON(result)
		},
	}
}

func readInlineArtifact(path string) (string, bool, bool, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", false, false, nil
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", false, false, fmt.Errorf("stat export artifact: %w", err)
	}
	if info.IsDir() {
		return "", false, false, nil
	}
	if info.Size() > maxInlinePatchsetBytes {
		return "", false, true, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false, false, fmt.Errorf("read export artifact: %w", err)
	}
	return string(data), true, false, nil
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
