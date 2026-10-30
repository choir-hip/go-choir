package buildinfo

import (
	"encoding/json"
	"os"
	"strings"
	"time"
)

// These values are filled by release builds through Go ldflags. Local builds
// keep deterministic fallback values so tests and dev servers still work.
var (
	Version = "dev"
	Commit  = "local"
	BuiltAt = "unknown"
)

// Info is the public build/deploy identity shape exposed by health endpoints.
// Keep it small and stable: staging acceptance reads this on every deploy.
type Info struct {
	Service        string `json:"service"`
	Version        string `json:"version"`
	Commit         string `json:"commit"`
	BuiltAt        string `json:"built_at"`
	DeployedAt     string `json:"deployed_at,omitempty"`
	DeployedCommit string `json:"deployed_commit,omitempty"`
}

// Snapshot returns the current service build identity and the separately
// observed deployment metadata. Commit is immutable process identity filled by
// the linker; a deploy marker must never rewrite what binary is actually
// running.
func Snapshot(service string) Info {
	deployedAt, deployedCommit := deployMetadata(service)
	return Info{
		Service:        service,
		Version:        Version,
		Commit:         Commit,
		BuiltAt:        BuiltAt,
		DeployedAt:     deployedAt,
		DeployedCommit: deployedCommit,
	}
}

// Platform is the platform-level deployment answer: the commit of the last
// successful staging deploy, whatever services that deploy moved. It is
// deliberately not part of Info. Info.DeployedCommit stays per-service
// ("this receipt's deploy moved this binary"), which the execution-identity
// join relies on, and Info is inside guest-signed evidence that a proxy
// decodes strictly, so a new Info field would break guest/host version skew
// (docs/archive/problems/partial-deploy-receipt-blanks-deployed-commit-2026-10-14.md).
type Platform struct {
	DeployedCommit string `json:"deployed_commit,omitempty"`
	DeployedAt     string `json:"deployed_at,omitempty"`
	// Source names where the answer came from: the receipt's platform
	// field, or (old receipts) the reporting service's own artifact.
	Source string `json:"source,omitempty"`
}

const (
	PlatformSourceReceipt         = "receipt.deployed_commit"
	PlatformSourceServiceArtifact = "receipt.artifacts.service"
)

// PlatformSnapshot reports the platform deployed commit from the activation
// receipt's deployed_commit field. Receipts written before that field
// existed fall back to the per-service rule for service, so an old partial
// receipt without that service still reports nothing rather than guessing.
func PlatformSnapshot(service string) Platform {
	receipt, activatedAt, ok := readDeployReceipt()
	if !ok {
		return Platform{}
	}
	if strings.TrimSpace(receipt.DeployedCommit) != "" {
		return Platform{
			DeployedCommit: strings.TrimSpace(receipt.TargetCommit),
			DeployedAt:     activatedAt,
			Source:         PlatformSourceReceipt,
		}
	}
	deployedAt, deployedCommit := serviceDeployMetadata(receipt, activatedAt, service)
	if deployedCommit == "" {
		return Platform{}
	}
	return Platform{DeployedCommit: deployedCommit, DeployedAt: deployedAt, Source: PlatformSourceServiceArtifact}
}

type deployReceipt struct {
	SchemaVersion int    `json:"schema_version"`
	TargetCommit  string `json:"target_commit"`
	// DeployedCommit is the platform commit (equal to TargetCommit) that
	// merged receipts record; absent on receipts written before the merge.
	DeployedCommit string                           `json:"deployed_commit"`
	ActivatedAt    string                           `json:"activated_at"`
	Artifacts      map[string]deployReceiptArtifact `json:"artifacts"`
}

type deployReceiptArtifact struct {
	Commit string `json:"commit"`
	Status string `json:"status"`
}

// readDeployReceipt returns a structurally valid activation receipt and its
// activation time. A receipt whose platform field conflicts with its target
// is not trusted for any answer.
func readDeployReceipt() (deployReceipt, string, bool) {
	path := strings.TrimSpace(os.Getenv("CHOIR_DEPLOY_RECEIPT_PATH"))
	if path == "" {
		path = "/var/lib/go-choir/deploy-receipt.json"
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return deployReceipt{}, "", false
	}
	var receipt deployReceipt
	if err := json.Unmarshal(data, &receipt); err != nil {
		return deployReceipt{}, "", false
	}
	activatedAt := strings.TrimSpace(receipt.ActivatedAt)
	targetCommit := strings.TrimSpace(receipt.TargetCommit)
	if receipt.SchemaVersion != 1 || !isFullCommit(targetCommit) || len(receipt.Artifacts) == 0 {
		return deployReceipt{}, "", false
	}
	if platform := strings.TrimSpace(receipt.DeployedCommit); platform != "" && platform != targetCommit {
		return deployReceipt{}, "", false
	}
	if _, err := time.Parse(time.RFC3339, activatedAt); err != nil {
		return deployReceipt{}, "", false
	}
	return receipt, activatedAt, true
}

func deployMetadata(service string) (string, string) {
	receipt, activatedAt, ok := readDeployReceipt()
	if !ok {
		return "", ""
	}
	return serviceDeployMetadata(receipt, activatedAt, service)
}

// serviceDeployMetadata is the per-service rule: a service reports the
// receipt's target only when this receipt's deploy moved it (its artifact
// commit is the target and it is active or installed). An artifact carried
// forward from an earlier deploy keeps its own older commit and so reports
// nothing here.
func serviceDeployMetadata(receipt deployReceipt, activatedAt, service string) (string, string) {
	targetCommit := strings.TrimSpace(receipt.TargetCommit)
	artifact, ok := receipt.Artifacts[service]
	if !ok {
		return "", ""
	}
	if strings.TrimSpace(artifact.Commit) != targetCommit {
		return "", ""
	}
	switch strings.TrimSpace(artifact.Status) {
	case "active", "installed":
		return activatedAt, targetCommit
	}
	return "", ""
}

func isFullCommit(commit string) bool {
	if len(commit) != 40 {
		return false
	}
	for _, r := range commit {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return false
		}
	}
	return true
}
