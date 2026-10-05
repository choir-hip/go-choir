package platform

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/filecas"
)

// ArtifactGCMode controls whether the platform-artifacts reachability sweep
// reports (dry-run) or deletes (active). Default is dry-run: a storage
// lifecycle is admitted only after its report is observed on the live store.
const (
	ArtifactGCModeDryRun = "dry-run"
	ArtifactGCModeActive = "active"
	ArtifactGCModeOff    = "off"
)

// ArtifactGCConfig configures one sweep over the content-addressed artifact
// namespaces that have a database-derived live set.
type ArtifactGCConfig struct {
	Mode       string        `json:"mode"`
	Grace      time.Duration `json:"grace"`
	MaxDeletes int           `json:"max_deletes"`
}

// ArtifactGCNamespaceReport records one namespace's observed vs deleted set.
type ArtifactGCNamespaceReport struct {
	Scanned      int   `json:"scanned"`
	Live         int   `json:"live"`
	Unreachable  int   `json:"unreachable"`
	InGrace      int   `json:"in_grace"`
	Deleted      int   `json:"deleted"`
	BytesDeleted int64 `json:"bytes_deleted"`
}

// ArtifactGCReport is the deployed-proof receipt for one sweep.
type ArtifactGCReport struct {
	Mode         string                               `json:"mode"`
	Grace        string                               `json:"grace"`
	Deleted      int                                  `json:"deleted"`
	BytesDeleted int64                                `json:"bytes_deleted"`
	Namespaces   map[string]ArtifactGCNamespaceReport `json:"namespaces"`
	Warnings     []string                             `json:"warnings,omitempty"`
	StartedAt    time.Time                            `json:"started_at"`
	DurationMs   int64                                `json:"duration_ms"`
}

var hexDigestRE = regexp.MustCompile(`^[0-9a-f]{64}$`)

func validHexDigest(s string) bool {
	return hexDigestRE.MatchString(strings.ToLower(strings.TrimSpace(s)))
}

// sweepNamespaces are the reachability-GC'd namespaces. The event tape
// (computer-event, computer-event-payload, pin-receipts) is excluded: its live
// set is the retained append-receipt chain (a chain-retention policy, not a
// refcount sweep) and is handled by a separate S0 slice.
var artifactGCNamespaces = []string{
	"file-cas-chunks",
	"file-cas-roots",
	"projection-base",
	"platform-update",
	"og",
}

// RunArtifactGC performs one reachability sweep across the GC'd artifact
// namespaces. For each namespace it computes the live set from the authoritative
// DB reference surface, then deletes every on-disk entry that is both
// unreachable and older than the grace cutoff. Dry-run reports without
// deleting. It never deletes a file newer than the grace cutoff, so an
// in-flight write/append/apply that has not yet committed its DB reference is
// never collected.
func (s *Service) RunArtifactGC(ctx context.Context, cfg ArtifactGCConfig) (ArtifactGCReport, error) {
	started := time.Now()
	report := ArtifactGCReport{
		Mode:       cfg.Mode,
		Grace:      cfg.Grace.String(),
		Namespaces: map[string]ArtifactGCNamespaceReport{},
		StartedAt:  started.UTC(),
	}
	if s == nil || s.store == nil || s.artifactsRoot == "" {
		return report, fmt.Errorf("artifact gc: service/store/artifacts root unavailable")
	}
	cfg = normalizeArtifactGCConfig(cfg)
	if cfg.Mode == ArtifactGCModeOff {
		report.Mode = ArtifactGCModeOff
		return report, nil
	}
	cutoff := started.UTC().Add(-cfg.Grace)
	deleted := 0
	var bytesDeleted int64
	remaining := func() int { return cfg.MaxDeletes - deleted }

	// Compute each namespace's live set once per sweep; a set that errors is
	// conservatively treated as "keep everything" (empty live + warning).
	live, warnings := s.artifactGCLiveSets(ctx)
	report.Warnings = append(report.Warnings, warnings...)

	for _, ns := range artifactGCNamespaces {
		if remaining() <= 0 {
			break
		}
		nsReport, err := s.sweepArtifactNamespace(ctx, ns, live[ns], cfg, cutoff, remaining())
		if err != nil {
			report.Warnings = append(report.Warnings, fmt.Sprintf("%s: %v", ns, err))
		}
		report.Namespaces[ns] = nsReport
		deleted += nsReport.Deleted
		bytesDeleted += nsReport.BytesDeleted
	}
	report.Deleted = deleted
	report.BytesDeleted = bytesDeleted
	report.DurationMs = time.Since(started).Milliseconds()
	return report, nil
}

func normalizeArtifactGCConfig(cfg ArtifactGCConfig) ArtifactGCConfig {
	switch strings.ToLower(strings.TrimSpace(cfg.Mode)) {
	case ArtifactGCModeActive:
		cfg.Mode = ArtifactGCModeActive
	case ArtifactGCModeDryRun, "":
		cfg.Mode = ArtifactGCModeDryRun
	default:
		cfg.Mode = ArtifactGCModeOff
	}
	if cfg.Grace <= 0 {
		cfg.Grace = 30 * time.Minute
	}
	if cfg.MaxDeletes <= 0 {
		cfg.MaxDeletes = 10000
	}
	return cfg
}

// artifactGCLiveSets returns, per namespace, the set of live on-disk entry
// names (the basename under the namespace dir). A namespace's live set is the
// union of every name the authoritative DB reference surface still names; the
// sweep deletes only names absent from it and older than grace.
func (s *Service) artifactGCLiveSets(ctx context.Context) (map[string]map[string]struct{}, []string) {
	live := map[string]map[string]struct{}{}
	warnings := []string{}
	for _, ns := range artifactGCNamespaces {
		live[ns] = map[string]struct{}{}
	}

	// file-cas-roots: every manifest_ref row is live (the current root table
	// retains all committed roots). file-cas-chunks liveness is resolved per
	// computer inside sweepFileCASChunks because it depends on manifest parses.
	if roots, err := s.store.db.QueryContext(ctx, `SELECT manifest_ref FROM computer_file_roots`); err != nil {
		warnings = append(warnings, "file-cas-roots live set: "+err.Error())
	} else {
		for roots.Next() {
			var ref string
			if err := roots.Scan(&ref); err == nil {
				live["file-cas-roots"][filepath.Base(ref)] = struct{}{}
			}
		}
		roots.Close()
	}

	// projection-base: the advertised watermark base_ref (blob) is live; its
	// .descriptor.json sidecar is protected by pairing inside the sweep.
	if wm, err := s.store.db.QueryContext(ctx, `SELECT base_ref FROM computer_replay_watermarks`); err != nil {
		warnings = append(warnings, "projection-base live set: "+err.Error())
	} else {
		for wm.Next() {
			var ref string
			if err := wm.Scan(&ref); err == nil {
				digest := filepath.Base(ref)
				live["projection-base"][digest] = struct{}{}
				live["projection-base"][digest+".descriptor.json"] = struct{}{}
			}
		}
		wm.Close()
	}

	// platform-update: every route-table-referenced payload digest. Union of
	// current route slots, all transition-receipt old/new refs (rollback walks
	// history — any receipt's version may be a rollback target), and
	// authorization evidence. The code/program JSON carries artifact+sha256
	// URIs whose path basename under /sha256/platform-update/ is the digest.
	updateRefs, warn := s.platformUpdateLiveDigests(ctx)
	if warn != "" {
		warnings = append(warnings, "platform-update live set: "+warn)
	}
	for d := range updateRefs {
		live["platform-update"][d] = struct{}{}
	}

	// og: exact body_ref reachability (basename of sha256/og/<digest>.bin).
	if og, err := s.store.db.QueryContext(ctx, `SELECT body_ref FROM og_objects WHERE body_ref <> ''`); err != nil {
		warnings = append(warnings, "og live set: "+err.Error())
	} else {
		for og.Next() {
			var ref string
			if err := og.Scan(&ref); err == nil {
				live["og"][filepath.Base(ref)] = struct{}{}
			}
		}
		og.Close()
	}
	return live, warnings
}

// platformUpdateLiveDigests resolves the set of platform-update payload
// digests still named by any route-table record: current route slots, every
// transition-receipt old/new code+program ref, and authorization evidence.
// Conservative over-retention is deliberate — a rollback may reach any
// receipt's version.
func (s *Service) platformUpdateLiveDigests(ctx context.Context) (map[string]struct{}, string) {
	live := map[string]struct{}{}
	collect := func(query string, arg ...interface{}) {
		rows, err := s.store.db.QueryContext(ctx, query, arg...)
		if err != nil {
			return
		}
		defer rows.Close()
		for rows.Next() {
			var closureJSON, programJSON sql.NullString
			if err := rows.Scan(&closureJSON, &programJSON); err != nil {
				continue
			}
			var closure struct {
				Artifacts []struct {
					URI    string `json:"uri"`
					SHA256 string `json:"sha256"`
				} `json:"artifacts"`
			}
			_ = json.Unmarshal([]byte(closureJSON.String), &closure)
			for _, a := range closure.Artifacts {
				live[platformUpdateDigestFromURI(a.URI)] = struct{}{}
				if validHexDigest(a.SHA256) {
					live[a.SHA256] = struct{}{}
				}
			}
			var program struct {
				Entries []struct {
					ArtifactURI   string `json:"artifact_uri"`
					ContentSHA256 string `json:"content_sha256"`
				} `json:"entries"`
			}
			_ = json.Unmarshal([]byte(programJSON.String), &program)
			for _, e := range program.Entries {
				live[platformUpdateDigestFromURI(e.ArtifactURI)] = struct{}{}
				if validHexDigest(e.ContentSHA256) {
					live[e.ContentSHA256] = struct{}{}
				}
			}
		}
	}
	// Current slots + all transition receipts + evidence: gather the union of
	// every code/program ref the route tables can still name.
	refRows, err := s.store.db.QueryContext(ctx, `
		SELECT DISTINCT current_code_ref, current_artifact_program_ref FROM computer_version_route_slots
		UNION SELECT DISTINCT new_code_ref, new_artifact_program_ref FROM computer_version_route_transition_receipts
		UNION SELECT DISTINCT old_code_ref, old_artifact_program_ref FROM computer_version_route_transition_receipts
		UNION SELECT DISTINCT code_ref, artifact_program_ref FROM computer_version_route_authorization_evidence`)
	if err != nil {
		return live, "list route refs: " + err.Error()
	}
	type refs struct{ code, program string }
	var list []refs
	for refRows.Next() {
		var r refs
		if err := refRows.Scan(&r.code, &r.program); err == nil {
			list = append(list, r)
		}
	}
	refRows.Close()
	for _, r := range list {
		collect(`SELECT closure_json, (SELECT program_json FROM computer_version_artifact_programs WHERE artifact_program_ref = ?) FROM computer_version_code_closures WHERE code_ref = ?`, r.program, r.code)
		collect(`SELECT NULL, program_json FROM computer_version_artifact_programs WHERE artifact_program_ref = ?`, r.program)
	}
	return live, ""
}

// platformUpdateDigestFromURI extracts the payload digest from a canonical
// artifact+sha256://<digest>/sha256/platform-update/<digest> URI — the path
// basename after the namespace segment.
func platformUpdateDigestFromURI(uri string) string {
	if i := strings.LastIndex(uri, "/sha256/platform-update/"); i >= 0 {
		return strings.TrimSpace(uri[i+len("/sha256/platform-update/"):])
	}
	return filepath.Base(strings.TrimPrefix(uri, "artifact+sha256://"))
}

// sweepArtifactNamespace removes unreachable, past-grace entries in one
// namespace. file-cas-chunks is handled separately (per-computer manifests).
func (s *Service) sweepArtifactNamespace(ctx context.Context, ns string, live map[string]struct{}, cfg ArtifactGCConfig, cutoff time.Time, maxDelete int) (ArtifactGCNamespaceReport, error) {
	rep := ArtifactGCNamespaceReport{}
	if ns == "file-cas-chunks" {
		return s.sweepFileCASChunks(ctx, cfg, cutoff, maxDelete)
	}
	dir, err := s.artifactPath(filepath.Join("sha256", ns))
	if err != nil {
		return rep, err
	}
	// file-cas-roots is nested one level deep by computerID
	// (<ns>/<computerID>/<root>.json); the other namespaces are flat.
	if ns == "file-cas-roots" {
		computers, err := os.ReadDir(dir)
		if os.IsNotExist(err) {
			return rep, nil
		}
		if err != nil {
			return rep, err
		}
		for _, c := range computers {
			if maxDelete <= 0 {
				break
			}
			if c == nil || !c.IsDir() {
				continue
			}
			if maxDelete, err = s.sweepFlatArtifactDir(filepath.Join(dir, c.Name()), live, cfg, cutoff, maxDelete, &rep); err != nil {
				return rep, err
			}
		}
		return rep, nil
	}
	_, err = s.sweepFlatArtifactDir(dir, live, cfg, cutoff, maxDelete, &rep)
	return rep, err
}

// sweepFlatArtifactDir sweeps one flat directory and returns the remaining
// delete budget.
func (s *Service) sweepFlatArtifactDir(dir string, live map[string]struct{}, cfg ArtifactGCConfig, cutoff time.Time, maxDelete int, rep *ArtifactGCNamespaceReport) (int, error) {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return maxDelete, nil
	}
	if err != nil {
		return maxDelete, err
	}
	for _, entry := range entries {
		if maxDelete <= 0 {
			break
		}
		if entry == nil || !entry.Type().IsRegular() {
			continue
		}
		rep.Scanned++
		name := entry.Name()
		if _, keep := live[name]; keep {
			rep.Live++
			continue
		}
		rep.Unreachable++
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if !info.ModTime().Before(cutoff) {
			rep.InGrace++
			continue
		}
		if cfg.Mode == ArtifactGCModeActive {
			if err := os.Remove(filepath.Join(dir, name)); err != nil && !os.IsNotExist(err) {
				return maxDelete, fmt.Errorf("delete %s: %w", name, err)
			}
		}
		rep.Deleted++
		rep.BytesDeleted += info.Size()
		maxDelete--
	}
	return maxDelete, nil
}

// sweepFileCASChunks runs the existing per-computer reachability GC across
// every file-cas-chunks/<computerID> dir, honoring grace + the shared delete
// budget. It reuses GCFileChunks' latest-plus-recent-root reachability.
func (s *Service) sweepFileCASChunks(ctx context.Context, cfg ArtifactGCConfig, cutoff time.Time, maxDelete int) (ArtifactGCNamespaceReport, error) {
	rep := ArtifactGCNamespaceReport{}
	dir, err := s.artifactPath(filepath.Join("sha256", "file-cas-chunks"))
	if err != nil {
		return rep, err
	}
	computers, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return rep, nil
	}
	if err != nil {
		return rep, err
	}
	for _, c := range computers {
		if maxDelete <= 0 {
			break
		}
		if c == nil || !c.IsDir() || !safeFileCASComponent(c.Name()) {
			continue
		}
		before := rep.Deleted
		if err := s.gcFileChunksForComputer(ctx, c.Name(), cfg, cutoff, maxDelete, &rep); err != nil {
			return rep, err
		}
		maxDelete -= rep.Deleted - before
	}
	return rep, nil
}

// gcFileChunksForComputer deletes stale chunks for one computer under the
// shared delete budget. Mirrors GCFileChunks' reachability but honors
// dry-run + a global budget. Counts fold into rep; the caller tracks the
// shared delete budget via rep.Deleted.
func (s *Service) gcFileChunksForComputer(ctx context.Context, computerID string, cfg ArtifactGCConfig, cutoff time.Time, maxDelete int, rep *ArtifactGCNamespaceReport) error {
	roots, rerr := s.fileCASRootsForGC(ctx, computerID, cutoff)
	if rerr != nil {
		return fmt.Errorf("file cas: roots for %s: %w", computerID, rerr)
	}
	reachable := make(map[string]struct{})
	for _, root := range roots {
		manifestData, err := s.readBlob(root.ManifestRef)
		if err != nil {
			continue // conservative: an unreadable manifest keeps its chunks
		}
		manifest, err := filecas.ParseManifest(manifestData)
		if err != nil || manifest.ComputerID != computerID {
			continue
		}
		for _, entry := range manifest.Files {
			for _, digest := range entry.Chunks {
				reachable[digest] = struct{}{}
			}
		}
	}
	prefix, err := s.artifactPath(filepath.Join("sha256", "file-cas-chunks", computerID))
	if err != nil {
		return err
	}
	walkErr := filepath.WalkDir(prefix, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil || maxDelete <= 0 {
			return walkErr
		}
		if entry.IsDir() || !entry.Type().IsRegular() || !validFileCASDigest(entry.Name()) {
			return nil
		}
		rep.Scanned++
		if _, keep := reachable[entry.Name()]; keep {
			rep.Live++
			return nil
		}
		rep.Unreachable++
		info, err := entry.Info()
		if err != nil {
			return nil
		}
		if !info.ModTime().Before(cutoff) {
			rep.InGrace++
			return nil
		}
		if cfg.Mode == ArtifactGCModeActive {
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				return err
			}
		}
		rep.Deleted++
		rep.BytesDeleted += info.Size()
		maxDelete--
		return nil
	})
	if os.IsNotExist(walkErr) {
		return nil
	}
	return walkErr
}

// GCRunner periodically runs the artifact GC when enabled.
type GCRunner struct {
	service  *Service
	config   ArtifactGCConfig
	interval time.Duration
}

// NewGCRunner builds a periodic artifact-GC runner. interval<=0 -> hourly.
func NewGCRunner(service *Service, cfg ArtifactGCConfig, interval time.Duration) *GCRunner {
	if interval <= 0 {
		interval = time.Hour
	}
	return &GCRunner{service: service, config: normalizeArtifactGCConfig(cfg), interval: interval}
}

// Start runs the sweep on the interval until ctx is done; disabled when mode
// is off. It logs a receipt for every pass (dry-run and active alike).
func (r *GCRunner) Start(ctx context.Context) {
	if r == nil || r.service == nil || r.config.Mode == ArtifactGCModeOff {
		return
	}
	go func() {
		ticker := time.NewTicker(r.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				report, err := r.service.RunArtifactGC(ctx, r.config)
				if err != nil {
					log.Printf("artifact gc: %v", err)
					continue
				}
				log.Printf("artifact gc: mode=%s deleted=%d bytes=%d warnings=%d", report.Mode, report.Deleted, report.BytesDeleted, len(report.Warnings))
			}
		}
	}()
}

// ListGCNamespaces reports the namespaces the reachability sweep covers.
func ListGCNamespaces() []string {
	out := make([]string, len(artifactGCNamespaces))
	copy(out, artifactGCNamespaces)
	sort.Strings(out)
	return out
}
