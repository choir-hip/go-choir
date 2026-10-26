package platform

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	DefaultPort          = "8086"
	DefaultDoltDSN       = "root@tcp(127.0.0.1:13306)/platform?parseTime=true&multiStatements=true&clientFoundRows=true"
	DefaultArtifactsRoot = "/var/lib/go-choir/platform-artifacts"
	// DefaultFrontImportancePath is where the host front-importance timer
	// writes its scores (cmd/front-importance).
	DefaultFrontImportancePath = "/var/lib/go-choir/front-importance/scores.json"
)

type Config struct {
	Port                  string
	DoltDSN               string
	CorpusDoltDSN         string
	ArtifactsRoot         string
	SigningKeyPath        string
	KeyEscrowKeyPath      string
	KeyEscrowOperatorsRaw string
	VmctlURL              string
	ArtifactGCMode        string
	ArtifactGCGrace       string
	ArtifactGCInterval    string
	ArtifactGCMaxDeletes  string
	ArtifactGCMaxBytes    string
	// ArtifactGCNamespaces is PLATFORM_ARTIFACT_GC_ACTIVE_NAMESPACES: the
	// comma list an active run may delete from (ParseArtifactGCNamespaces).
	ArtifactGCNamespaces string
	// TapeSweepMode is PLATFORM_TAPE_SWEEP_MODE: dry-run (default), active
	// (apply admitted) or off. TapeSweepArchiveDir is
	// PLATFORM_TAPE_SWEEP_ARCHIVE_DIR (default <artifacts>/tape-sweep-archive).
	TapeSweepMode       string
	TapeSweepArchiveDir string
	// TapeSegments is PLATFORM_TAPE_SEGMENTS: off (default) or write. Write
	// puts projection batch events, payloads and their pin receipts in the
	// computer's tape segment instead of one file each (tape_segments_write.go).
	TapeSegments string
	// AutopaperOwnersRaw is CORPUSD_AUTOPAPER_OWNERS: comma-separated owner
	// ids whose auto stories the front list shows; unset shows every
	// owner's (ParseAutopaperOwners).
	AutopaperOwnersRaw string
	// PublicBaseURL is CORPUSD_PUBLIC_BASE_URL: the host a published
	// piece's read_url names (default https://choir.news).
	PublicBaseURL string
	// OpinionListingRaw is CORPUSD_OPINION_LISTING: "on" publishes, lists and
	// searches opinion pieces; anything else (the default) hides them all.
	// OpinionDailyLimitRaw is CORPUSD_OPINION_DAILY_LIMIT: new opinion pieces
	// per user per 24 hours (default 5).
	OpinionListingRaw    string
	OpinionDailyLimitRaw string
	// FrontImportancePath is CORPUSD_FRONT_IMPORTANCE_PATH: the scores file
	// the host front-importance timer writes (cmd/front-importance). The
	// front list attaches its scores read-only; a missing or bad file
	// leaves the list unchanged.
	FrontImportancePath string
	// ReleaseReadsRaw is CHOIR_RELEASE_READS (Library S1): owner (the
	// default, also when unset) serves a release only to the owner who
	// published it; an explicit open serves any release to any reader
	// (ParseReleaseReads).
	ReleaseReadsRaw string
}

func LoadConfig() (*Config, error) {
	cfg := &Config{
		Port:                  envOr("CORPUSD_PORT", DefaultPort),
		DoltDSN:               envOr("CORPUSD_DOLT_DSN", DefaultDoltDSN),
		CorpusDoltDSN:         os.Getenv("CORPUSD_CORPUS_DOLT_DSN"),
		ArtifactsRoot:         envOr("CORPUSD_ARTIFACTS_ROOT", DefaultArtifactsRoot),
		SigningKeyPath:        envOr("PLATFORM_SIGNING_KEY_PATH", filepath.Join(envOr("CORPUSD_ARTIFACTS_ROOT", DefaultArtifactsRoot), "signing-key")),
		KeyEscrowKeyPath:      envOr("CHOIR_KEY_ESCROW_KEY_PATH", filepath.Join(envOr("CORPUSD_ARTIFACTS_ROOT", DefaultArtifactsRoot), "key-escrow-key")),
		KeyEscrowOperatorsRaw: os.Getenv("CHOIR_KEY_ESCROW_OPERATORS"),
		VmctlURL:              os.Getenv("CORPUSD_VMCTL_URL"),
		ArtifactGCMode:        os.Getenv("PLATFORM_ARTIFACT_GC_MODE"),
		ArtifactGCGrace:       os.Getenv("PLATFORM_ARTIFACT_GC_GRACE"),
		ArtifactGCInterval:    os.Getenv("PLATFORM_ARTIFACT_GC_INTERVAL"),
		ArtifactGCMaxDeletes:  os.Getenv("PLATFORM_ARTIFACT_GC_MAX_DELETES"),
		ArtifactGCMaxBytes:    os.Getenv("PLATFORM_ARTIFACT_GC_MAX_BYTES"),
		ArtifactGCNamespaces:  os.Getenv("PLATFORM_ARTIFACT_GC_ACTIVE_NAMESPACES"),
		TapeSweepMode:         os.Getenv("PLATFORM_TAPE_SWEEP_MODE"),
		TapeSweepArchiveDir:   os.Getenv("PLATFORM_TAPE_SWEEP_ARCHIVE_DIR"),
		TapeSegments:          os.Getenv("PLATFORM_TAPE_SEGMENTS"),
		AutopaperOwnersRaw:    os.Getenv("CORPUSD_AUTOPAPER_OWNERS"),
		PublicBaseURL:         os.Getenv("CORPUSD_PUBLIC_BASE_URL"),
		OpinionListingRaw:     os.Getenv("CORPUSD_OPINION_LISTING"),
		OpinionDailyLimitRaw:  os.Getenv("CORPUSD_OPINION_DAILY_LIMIT"),
		FrontImportancePath:   envOr("CORPUSD_FRONT_IMPORTANCE_PATH", DefaultFrontImportancePath),
		ReleaseReadsRaw:       os.Getenv(EnvReleaseReads),
	}
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *Config) validate() error {
	if c.Port == "" {
		return fmt.Errorf("platform config: CORPUSD_PORT must not be empty")
	}
	if c.DoltDSN == "" {
		return fmt.Errorf("platform config: CORPUSD_DOLT_DSN must not be empty")
	}
	if c.ArtifactsRoot == "" {
		return fmt.Errorf("platform config: CORPUSD_ARTIFACTS_ROOT must not be empty")
	}
	return nil
}

func (c *Config) EnsureDirs() error {
	// Platform artifacts are content-addressed below this root.
	if err := os.MkdirAll(filepath.Join(c.ArtifactsRoot, "sha256"), 0o750); err != nil {
		return fmt.Errorf("platform config: create artifacts root: %w", err)
	}
	return nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
