package sandbox

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const sourceLineageSchemaVersion = 1

// SourceWorkspaceOptions carries the computer/session identity projected into
// the local source workspace that zot inspects from Super Console.
type SourceWorkspaceOptions struct {
	ComputerID string
	Kind       string
	OwnerID    string
	DesktopID  string
	SessionID  string
}

// SourceWorkspaceProjection is the filesystem-local view of the computer's
// source/build lineage. It intentionally duplicates product-level lineage refs
// with local mount paths so repair tools can work without first querying APIs.
type SourceWorkspaceProjection struct {
	SchemaVersion           int       `json:"schema_version"`
	ComputerID              string    `json:"computer_id"`
	ComputerKind            string    `json:"computer_kind"`
	OwnerID                 string    `json:"owner_id,omitempty"`
	DesktopID               string    `json:"desktop_id"`
	SuperConsoleSessionID   string    `json:"super_console_session_id,omitempty"`
	PlatformBaseCommit      string    `json:"platform_base_commit"`
	PlatformSourceRepo      string    `json:"platform_source_repo"`
	PlatformSourceMount     string    `json:"platform_source_mount"`
	UserSourceRef           string    `json:"user_source_ref"`
	UserSourceMount         string    `json:"user_source_mount"`
	CandidateSourceRef      string    `json:"candidate_source_ref,omitempty"`
	CandidateSourceMount    string    `json:"candidate_source_mount"`
	BuildMount              string    `json:"build_mount"`
	PromotionWorkspaceRoot  string    `json:"promotion_workspace_root,omitempty"`
	SourceLedgerRepo        string    `json:"source_ledger_repo,omitempty"`
	CurrentRuntimeBuildRef  string    `json:"current_runtime_build_ref,omitempty"`
	CurrentFrontendBuildRef string    `json:"current_frontend_build_ref,omitempty"`
	DirtyStateSummary       string    `json:"dirty_state_summary"`
	RollbackRef             string    `json:"rollback_ref,omitempty"`
	LastVerifiedAt          time.Time `json:"last_verified_at"`
	LineagePath             string    `json:"lineage_path"`
}

// BootstrapSourceWorkspace ensures the stable source/build roots exist under a
// computer's persistent files root and writes the local lineage projection.
func BootstrapSourceWorkspace(filesRoot string, opts SourceWorkspaceOptions) (SourceWorkspaceProjection, error) {
	root := strings.TrimSpace(filesRoot)
	if root == "" {
		return SourceWorkspaceProjection{}, fmt.Errorf("source workspace: files root is required")
	}
	root = filepath.Clean(root)
	sourceRoot := filepath.Join(root, "Source")
	projection := sourceWorkspaceProjection(root, sourceRoot, opts)
	for _, dir := range []string{
		filepath.Join(sourceRoot, "platform"),
		filepath.Join(sourceRoot, "user"),
		filepath.Join(sourceRoot, "candidate"),
		filepath.Join(root, "Build"),
		filepath.Join(root, ".choir"),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return SourceWorkspaceProjection{}, fmt.Errorf("source workspace: create %s: %w", dir, err)
		}
	}
	if err := writeSourceLineageProjection(projection); err != nil {
		return SourceWorkspaceProjection{}, err
	}
	return projection, nil
}

func sourceWorkspaceProjection(root, sourceRoot string, opts SourceWorkspaceOptions) SourceWorkspaceProjection {
	computerID := firstNonEmptySourceWorkspace(opts.ComputerID, os.Getenv("SANDBOX_ID"), "sandbox-dev")
	kind := firstNonEmptySourceWorkspace(opts.Kind, os.Getenv("CHOIR_COMPUTER_KIND"), inferComputerKind(computerID))
	desktopID := firstNonEmptySourceWorkspace(opts.DesktopID, os.Getenv("CHOIR_DESKTOP_ID"), "primary")
	baseCommit := firstNonEmptySourceWorkspace(
		os.Getenv("CHOIR_DEPLOYED_COMMIT"),
		os.Getenv("CHOIR_BUILD_SHA"),
		os.Getenv("RUNTIME_WORKER_REPO_BASE_SHA"),
		"unknown",
	)
	userRef := activeSourceRefForComputer(computerID, kind)
	candidateRef := ""
	if kind == "candidate" || kind == "worker" {
		candidateID := firstNonEmptySourceWorkspace(opts.SessionID, os.Getenv("SANDBOX_ID"), computerID)
		candidateRef = "refs/computers/" + safeSourceRefPart(computerID) + "/candidates/" + safeSourceRefPart(candidateID)
	}
	return SourceWorkspaceProjection{
		SchemaVersion:           sourceLineageSchemaVersion,
		ComputerID:              computerID,
		ComputerKind:            kind,
		OwnerID:                 strings.TrimSpace(opts.OwnerID),
		DesktopID:               desktopID,
		SuperConsoleSessionID:   strings.TrimSpace(opts.SessionID),
		PlatformBaseCommit:      baseCommit,
		PlatformSourceRepo:      firstNonEmptySourceWorkspace(os.Getenv("RUNTIME_PROMOTION_SOURCE_REPO"), os.Getenv("RUNTIME_WORKER_REPO_REMOTE")),
		PlatformSourceMount:     filepath.Join(sourceRoot, "platform"),
		UserSourceRef:           userRef,
		UserSourceMount:         filepath.Join(sourceRoot, "user"),
		CandidateSourceRef:      candidateRef,
		CandidateSourceMount:    filepath.Join(sourceRoot, "candidate"),
		BuildMount:              filepath.Join(root, "Build"),
		PromotionWorkspaceRoot:  os.Getenv("RUNTIME_PROMOTION_WORKSPACE_ROOT"),
		SourceLedgerRepo:        os.Getenv("RUNTIME_SOURCE_LEDGER_REPO"),
		CurrentRuntimeBuildRef:  baseCommit,
		CurrentFrontendBuildRef: baseCommit,
		DirtyStateSummary:       "not_inspected",
		LastVerifiedAt:          time.Now().UTC(),
		LineagePath:             filepath.Join(root, ".choir", "source-lineage.json"),
	}
}

func writeSourceLineageProjection(projection SourceWorkspaceProjection) error {
	raw, err := json.MarshalIndent(projection, "", "  ")
	if err != nil {
		return fmt.Errorf("source workspace: marshal lineage: %w", err)
	}
	raw = append(raw, '\n')
	tmp := projection.LineagePath + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return fmt.Errorf("source workspace: write lineage: %w", err)
	}
	if err := os.Rename(tmp, projection.LineagePath); err != nil {
		return fmt.Errorf("source workspace: replace lineage: %w", err)
	}
	return nil
}

func inferComputerKind(computerID string) string {
	id := strings.ToLower(strings.TrimSpace(computerID))
	switch {
	case strings.Contains(id, "worker"):
		return "worker"
	case strings.Contains(id, "candidate"):
		return "candidate"
	default:
		return "active"
	}
}

func activeSourceRefForComputer(computerID, kind string) string {
	if strings.TrimSpace(kind) == "platform" {
		return "refs/platform/main"
	}
	return "refs/computers/" + safeSourceRefPart(computerID) + "/active"
}

func safeSourceRefPart(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "unknown"
	}
	var b strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == '_' || r == '.':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	out := strings.Trim(b.String(), "-._")
	if out == "" {
		return "unknown"
	}
	return out
}

func firstNonEmptySourceWorkspace(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
