package buildinfo

import "os"

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
// at request time so systemd EnvironmentFile updates are reflected after restart.
func Snapshot(service string) Info {
	return Info{
		Service:        service,
		Version:        Version,
		Commit:         Commit,
		BuiltAt:        BuiltAt,
		DeployedAt:     os.Getenv("CHOIR_DEPLOYED_AT"),
		DeployedCommit: os.Getenv("CHOIR_DEPLOYED_COMMIT"),
	}
}
