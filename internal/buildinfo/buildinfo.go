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
	deployedAt, deployedCommit := deployMetadata()
	return Info{
		Service:        service,
		Version:        Version,
		Commit:         Commit,
		BuiltAt:        BuiltAt,
		DeployedAt:     deployedAt,
		DeployedCommit: deployedCommit,
	}
}

type deployReceipt struct {
	SchemaVersion int                              `json:"schema_version"`
	TargetCommit  string                           `json:"target_commit"`
	ActivatedAt   string                           `json:"activated_at"`
	Artifacts     map[string]deployReceiptArtifact `json:"artifacts"`
}

type deployReceiptArtifact struct {
	Commit string `json:"commit"`
	Status string `json:"status"`
}

func deployMetadata() (string, string) {
	path := strings.TrimSpace(os.Getenv("CHOIR_DEPLOY_RECEIPT_PATH"))
	if path == "" {
		path = "/var/lib/go-choir/deploy-receipt.json"
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", ""
	}
	var receipt deployReceipt
	if err := json.Unmarshal(data, &receipt); err != nil {
		return "", ""
	}
	activatedAt := strings.TrimSpace(receipt.ActivatedAt)
	targetCommit := strings.TrimSpace(receipt.TargetCommit)
	if receipt.SchemaVersion != 1 || !isFullCommit(targetCommit) || len(receipt.Artifacts) == 0 {
		return "", ""
	}
	if _, err := time.Parse(time.RFC3339, activatedAt); err != nil {
		return "", ""
	}
	verified := false
	for _, artifact := range receipt.Artifacts {
		if strings.TrimSpace(artifact.Commit) != targetCommit {
			continue
		}
		switch strings.TrimSpace(artifact.Status) {
		case "active", "installed":
			verified = true
		}
	}
	if !verified {
		return "", ""
	}
	return activatedAt, targetCommit
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
