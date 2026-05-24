package buildinfo

import (
	"os"
	"strings"
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

// Snapshot returns the current service build identity. Deploy metadata is read
// at request time so frontend-only deploys can update /health identity without
// restarting otherwise-unaffected host services.
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

func deployMetadata() (string, string) {
	deployedAt := os.Getenv("CHOIR_DEPLOYED_AT")
	deployedCommit := os.Getenv("CHOIR_DEPLOYED_COMMIT")

	path := strings.TrimSpace(os.Getenv("CHOIR_DEPLOY_ENV_PATH"))
	if path == "" {
		path = "/var/lib/go-choir/deploy.env"
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return deployedAt, deployedCommit
	}
	for _, line := range strings.Split(string(data), "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok {
			continue
		}
		switch key {
		case "CHOIR_DEPLOYED_AT":
			deployedAt = value
		case "CHOIR_DEPLOYED_COMMIT":
			deployedCommit = value
		}
	}
	return deployedAt, deployedCommit
}
