package platform

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"regexp"
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

const (
	// artifactGCLogDir holds the append-only deletion log, one JSONL file per
	// UTC day, beside (not inside) the swept sha256/ tree.
	artifactGCLogDir = "artifact-gc-log"
	// artifactGCLogBatch is how many intent records are made durable (fsync)
	// before any of their unlinks run.
	artifactGCLogBatch = 256
	// DefaultArtifactGCMaxBytes caps the bytes one run may delete.
	DefaultArtifactGCMaxBytes int64 = 32 << 30
)

// ArtifactGCConfig configures one sweep over the content-addressed artifact
// namespaces that have a database-derived live set.
type ArtifactGCConfig struct {
	Mode       string        `json:"mode"`
	Grace      time.Duration `json:"grace"`
	MaxDeletes int           `json:"max_deletes"`
	// MaxBytes caps the bytes one run deletes. A candidate that would carry
	// the run past the cap is deferred whole, never partially taken.
	MaxBytes int64 `json:"max_bytes"`
	// ActiveNamespaces lists the namespaces a run may delete from; the rest
	// are swept report-only. Empty means every swept namespace.
	ActiveNamespaces []string `json:"active_namespaces,omitempty"`
}

// ArtifactGCNamespaceReport records one namespace's observed vs deleted set.
type ArtifactGCNamespaceReport struct {
	// Mode is the namespace's effective mode: active, or dry-run when the
	// run is a dry-run or the namespace is report-only.
	Mode        string `json:"mode"`
	Scanned     int    `json:"scanned"`
	Live        int    `json:"live"`
	Unreachable int    `json:"unreachable"`
	InGrace     int    `json:"in_grace"`
	// Unproven entries are kept because their liveness could not be
	// established (unreadable manifest, computer with no recorded root).
	Unproven int `json:"unproven"`
	// Eligible is every unreachable, past-grace entry, uncapped: the report
	// an owner reads before admitting deletion.
	Eligible      int   `json:"eligible"`
	EligibleBytes int64 `json:"eligible_bytes"`
	// Deleted is what this run removed (active) or would remove under the
	// same caps (dry-run).
	Deleted       int   `json:"deleted"`
	BytesDeleted  int64 `json:"bytes_deleted"`
	DeferredByCap int   `json:"deferred_by_cap"`
	// KeptOnRecheck counts candidates a writer refreshed (reused) or removed
	// between classification and the locked unlink.
	KeptOnRecheck int `json:"kept_on_recheck"`
	Errors        int `json:"errors"`
	// Retired counts file_cas root rows retired (active) or that would be
	// retired (dry-run) this run (file-cas-roots only).
	Retired int `json:"retired,omitempty"`
}

// ArtifactGCReport is the deployed-proof receipt for one sweep.
type ArtifactGCReport struct {
	Mode         string                               `json:"mode"`
	Grace        string                               `json:"grace"`
	Deleted      int                                  `json:"deleted"`
	BytesDeleted int64                                `json:"bytes_deleted"`
	Namespaces   map[string]ArtifactGCNamespaceReport `json:"namespaces"`
	Warnings     []string                             `json:"warnings,omitempty"`
	// LogRotation summarizes the GC-log compression of this pass (active).
	LogRotation *ArtifactGCLogRotation `json:"log_rotation,omitempty"`
	StartedAt   time.Time              `json:"started_at"`
	DurationMs  int64                  `json:"duration_ms"`
}

var hexDigestRE = regexp.MustCompile(`^[0-9a-f]{64}$`)

func validHexDigest(s string) bool {
	return hexDigestRE.MatchString(strings.ToLower(strings.TrimSpace(s)))
}

// artifactGCNamespaces are the reachability-GC'd namespaces, in sweep order:
// a few large objects first, so a capped run is not spent inside the chunk
// store before it reaches them. The event tape (computer-event,
// computer-event-payload, pin-receipts) is excluded: its live set is the
// retained append-receipt chain (a chain-retention policy, not a refcount
// sweep).
var artifactGCNamespaces = []string{
	"projection-base",
	"platform-update",
	"file-cas-roots",
	"file-cas-chunks",
	"og",
}

// DefaultArtifactGCActiveNamespaces is the deletion allowlist when
// PLATFORM_ARTIFACT_GC_ACTIVE_NAMESPACES is unset. og is excluded: the
// pre-teardown corpus bodies are unreferenced since 2026-10-09 and their
// collection is an explicit owner step (the corpus dump may still name them).
var DefaultArtifactGCActiveNamespaces = []string{"projection-base", "platform-update", "file-cas-roots", "file-cas-chunks"}

// ParseArtifactGCNamespaces reads a comma list. Empty selects the default
// allowlist; "all" selects every swept namespace.
func ParseArtifactGCNamespaces(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return append([]string(nil), DefaultArtifactGCActiveNamespaces...)
	}
	if raw == "all" {
		return nil
	}
	var out []string
	for _, part := range strings.Split(raw, ",") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	if len(out) == 0 {
		// A list of only separators names nothing: delete nothing rather
		// than fall back to everything.
		return []string{"none"}
	}
	return out
}

type gcCandidate struct {
	ns    string
	rel   string // slash path relative to the artifacts root
	path  string
	size  int64
	mtime time.Time
}

// RunArtifactGC performs one reachability sweep across the GC'd artifact
// namespaces. For each namespace it computes the live set from the
// authoritative DB reference surface, classifies every on-disk entry, and
// (active) deletes entries that are both unreachable and older than the grace
// cutoff, within the run's count and byte caps. Every deletion is preceded by
// a durable intent record in the deletion log and re-checked under the CAS
// reuse lock. Dry-run reports the full eligible set without deleting.
func (s *Service) RunArtifactGC(ctx context.Context, cfg ArtifactGCConfig) (ArtifactGCReport, error) {
	report, err := s.runArtifactGC(ctx, cfg)
	if report.Mode == ArtifactGCModeActive && s != nil && s.artifactsRoot != "" && ctx.Err() == nil {
		// F6: compress old daily logs, after artifactGCMu is released. Best
		// effort: a failure is a warning and never fails the pass (the
		// original is kept).
		s.artifactGCLogMu.Lock()
		rot, rotErr := s.rotateArtifactGCLogs(ctx, time.Now())
		s.artifactGCLogMu.Unlock()
		if rot.Compressed > 0 {
			report.LogRotation = &rot
		}
		if rotErr != nil {
			report.Warnings = append(report.Warnings, "artifact-gc-log rotation: "+rotErr.Error())
		}
	}
	return report, err
}

func (s *Service) runArtifactGC(ctx context.Context, cfg ArtifactGCConfig) (ArtifactGCReport, error) {
	started := time.Now()
	cfg = normalizeArtifactGCConfig(cfg)
	report := ArtifactGCReport{
		Mode:       cfg.Mode,
		Grace:      cfg.Grace.String(),
		Namespaces: map[string]ArtifactGCNamespaceReport{},
		StartedAt:  started.UTC(),
	}
	if s == nil || s.store == nil || s.artifactsRoot == "" {
		return report, fmt.Errorf("artifact gc: service/store/artifacts root unavailable")
	}
	// Serialize explicit pins with live-set capture and deletion. A pin of an
	// old blob must not commit after the sweep has classified it unreachable.
	// This is a dedicated lock, never writeMu (staging 2026-10-08: a sweep
	// holding writeMu timed out credential issuance and failed VM boots).
	s.artifactGCMu.Lock()
	defer s.artifactGCMu.Unlock()
	if cfg.Mode == ArtifactGCModeOff {
		return report, nil
	}
	cutoff := started.UTC().Add(-cfg.Grace)

	allowed := map[string]bool{}
	for _, ns := range cfg.ActiveNamespaces {
		allowed[ns] = true
	}

	// A live set that fails to load aborts the whole sweep — sweeping with a
	// missing set deletes live artifacts (s0-gc-og-wrong-store-deletion,
	// 2026-10-05).
	live, warnings := s.artifactGCLiveSets(ctx)
	report.Warnings = append(report.Warnings, warnings...)
	if live == nil {
		report.DurationMs = time.Since(started).Milliseconds()
		return report, fmt.Errorf("artifact gc: live set unavailable: %s", strings.Join(warnings, "; "))
	}

	// F4: retire old file-cas root rows only after the live sets have loaded
	// (a pass that cannot prove them mutates nothing). The file-cas-roots live
	// set is then re-read, so manifests of retired rows are unreachable in this
	// same pass. Dry-run only counts.
	rootsRetired := 0
	if len(cfg.ActiveNamespaces) == 0 || allowed["file-cas-roots"] {
		n, err := s.retireFileCASRoots(ctx, started, cfg.Grace, cfg.MaxDeletes, cfg.Mode == ArtifactGCModeActive)
		rootsRetired = n
		if err != nil {
			report.Warnings = append(report.Warnings, "file-cas-roots retire: "+err.Error())
		}
		if n > 0 && cfg.Mode == ArtifactGCModeActive {
			refreshed := map[string]struct{}{}
			if err := scanStrings(ctx, s.store.db, `SELECT manifest_ref FROM computer_file_roots`, func(ref string) {
				refreshed[filepath.Base(ref)] = struct{}{}
			}); err != nil {
				report.Namespaces["file-cas-roots"] = ArtifactGCNamespaceReport{Mode: ArtifactGCModeDryRun, Retired: n}
				report.DurationMs = time.Since(started).Milliseconds()
				return report, fmt.Errorf("artifact gc: file-cas-roots live set unavailable after retiring %d rows: %w", n, err)
			}
			live["file-cas-roots"] = refreshed
		}
	}

	var selected []gcCandidate
	deletedCount := 0
	var deletedBytes int64
	for _, ns := range artifactGCNamespaces {
		if err := ctx.Err(); err != nil {
			report.DurationMs = time.Since(started).Milliseconds()
			return report, err
		}
		rep := ArtifactGCNamespaceReport{}
		candidates, nsWarnings, err := s.classifyArtifactNamespace(ctx, ns, live[ns], cutoff, &rep)
		report.Warnings = append(report.Warnings, nsWarnings...)
		participates := len(cfg.ActiveNamespaces) == 0 || allowed[ns]
		if err != nil {
			report.Warnings = append(report.Warnings, fmt.Sprintf("%s: %v (namespace not swept)", ns, err))
			participates = false
		}
		// An empty live set beside old unreachable files is the wrong-store
		// signature (2026-10-05). Report it; never sweep on it. file-cas-chunks
		// liveness is per computer and guarded there.
		if ns != "file-cas-chunks" && len(live[ns]) == 0 && rep.Eligible > 0 {
			report.Warnings = append(report.Warnings, fmt.Sprintf("%s: live set is empty but %d old entries exist (namespace not swept)", ns, rep.Eligible))
			participates = false
		}
		if ns == "file-cas-roots" {
			rep.Retired = rootsRetired
		}
		rep.Mode = ArtifactGCModeDryRun
		if participates && cfg.Mode == ArtifactGCModeActive {
			rep.Mode = ArtifactGCModeActive
		}
		if participates {
			for _, c := range candidates {
				if deletedCount >= cfg.MaxDeletes || deletedBytes+c.size > cfg.MaxBytes {
					rep.DeferredByCap++
					continue
				}
				deletedCount++
				deletedBytes += c.size
				selected = append(selected, c)
				if cfg.Mode != ArtifactGCModeActive {
					rep.Deleted++
					rep.BytesDeleted += c.size
				}
			}
		}
		report.Namespaces[ns] = rep
	}

	var runErr error
	if cfg.Mode == ArtifactGCModeActive {
		runErr = s.deleteArtifactCandidates(ctx, started, selected, cutoff, &report)
	}
	for _, rep := range report.Namespaces {
		report.Deleted += rep.Deleted
		report.BytesDeleted += rep.BytesDeleted
	}
	report.DurationMs = time.Since(started).Milliseconds()
	return report, runErr
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
	if cfg.MaxBytes <= 0 {
		cfg.MaxBytes = DefaultArtifactGCMaxBytes
	}
	return cfg
}

// errArtifactGCActiveNotConfigured refuses an on-demand active run on a
// service whose configured policy is not active.
var errArtifactGCActiveNotConfigured = errors.New("artifact gc: active collection is not enabled (PLATFORM_ARTIFACT_GC_MODE)")

// SetArtifactGCPolicy records the configured GC policy. The on-demand
// endpoint may narrow it but never exceed it.
func (s *Service) SetArtifactGCPolicy(cfg ArtifactGCConfig) {
	if s == nil {
		return
	}
	cfg = normalizeArtifactGCConfig(cfg)
	s.artifactGCPolicyMu.Lock()
	s.artifactGCPolicy = &cfg
	s.artifactGCPolicyMu.Unlock()
}

// clampArtifactGCRequest bounds an on-demand request by the configured
// policy: no active run unless the policy is active, no grace below the
// policy's window, no caps above its caps, and the policy's namespaces.
func (s *Service) clampArtifactGCRequest(req ArtifactGCConfig) (ArtifactGCConfig, error) {
	policy := normalizeArtifactGCConfig(ArtifactGCConfig{})
	s.artifactGCPolicyMu.Lock()
	if s.artifactGCPolicy != nil {
		policy = *s.artifactGCPolicy
	}
	s.artifactGCPolicyMu.Unlock()
	out := normalizeArtifactGCConfig(ArtifactGCConfig{Mode: req.Mode})
	if out.Mode == ArtifactGCModeOff {
		out.Mode = ArtifactGCModeDryRun
	}
	if out.Mode == ArtifactGCModeActive && policy.Mode != ArtifactGCModeActive {
		return out, errArtifactGCActiveNotConfigured
	}
	out.Grace = policy.Grace
	if req.Grace > policy.Grace {
		out.Grace = req.Grace
	}
	out.MaxDeletes = policy.MaxDeletes
	if req.MaxDeletes > 0 && req.MaxDeletes < policy.MaxDeletes {
		out.MaxDeletes = req.MaxDeletes
	}
	out.MaxBytes = policy.MaxBytes
	if req.MaxBytes > 0 && req.MaxBytes < policy.MaxBytes {
		out.MaxBytes = req.MaxBytes
	}
	out.ActiveNamespaces = policy.ActiveNamespaces
	return out, nil
}

// artifactGCLiveSets returns, per namespace, the set of live on-disk entry
// names (the basename under the namespace dir). A namespace's live set is the
// union of every name the authoritative DB reference surface still names; the
// sweep deletes only names absent from it and older than grace. Any query
// error returns nil: a live set we could not load is not an empty live set.
func (s *Service) artifactGCLiveSets(ctx context.Context) (map[string]map[string]struct{}, []string) {
	live := map[string]map[string]struct{}{}
	warnings := []string{}
	for _, ns := range artifactGCNamespaces {
		live[ns] = map[string]struct{}{}
	}

	// file-cas-roots: every manifest_ref row is live (the current root table
	// retains all committed roots). file-cas-chunks liveness is resolved per
	// computer during classification because it depends on manifest parses.
	if err := scanStrings(ctx, s.store.db, `SELECT manifest_ref FROM computer_file_roots`, func(ref string) {
		live["file-cas-roots"][filepath.Base(ref)] = struct{}{}
	}); err != nil {
		return nil, append(warnings, "file-cas-roots live set: "+err.Error())
	}

	// projection-base: the advertised watermark base_ref, explicit pins
	// (restore points), each computer's latest two bases, and in-flight job
	// seeds. A live blob keeps its .descriptor.json sidecar.
	addBase := func(ref string) {
		digest := filepath.Base(ref)
		live["projection-base"][digest] = struct{}{}
		live["projection-base"][digest+".descriptor.json"] = struct{}{}
	}
	if err := scanStrings(ctx, s.store.db, `SELECT base_ref FROM computer_replay_watermarks`, addBase); err != nil {
		return nil, append(warnings, "projection-base live set: "+err.Error())
	}
	pins, err := s.store.projectionBasePins(ctx)
	if err != nil {
		return nil, append(warnings, "projection-base pins: "+err.Error())
	}
	for ref := range pins {
		addBase(ref)
	}

	// platform-update: every route-table-referenced payload digest (current
	// slots, every transition receipt's old/new refs — the rollback refs —
	// and authorization evidence).
	updateRefs, err := s.platformUpdateLiveDigests(ctx)
	if err != nil {
		return nil, append(warnings, "platform-update live set: "+err.Error())
	}
	for d := range updateRefs {
		live["platform-update"][d] = struct{}{}
	}

	// og: exact body_ref reachability (basename of sha256/og/<digest>.bin).
	// body_ref rows live in Store B (corpus) — reading Store A here made every
	// externalized corpus body unreachable and the sweep mass-deleted live
	// bodies on 2026-10-05 (s0-gc-og-wrong-store-deletion).
	if err := scanStrings(ctx, s.store.corpus(), `SELECT body_ref FROM og_objects WHERE body_ref <> ''`, func(ref string) {
		live["og"][filepath.Base(ref)] = struct{}{}
	}); err != nil {
		return nil, append(warnings, "og live set: "+err.Error())
	}
	return live, warnings
}

// scanStrings runs a one-column query and reports any query, scan or
// iteration error: a partial result must never pass for a complete one.
func scanStrings(ctx context.Context, db *sql.DB, query string, add func(string), args ...any) error {
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var value sql.NullString
		if err := rows.Scan(&value); err != nil {
			return err
		}
		if value.Valid {
			add(value.String)
		}
	}
	return rows.Err()
}

// platformUpdateLiveDigests resolves the set of platform-update payload
// digests still named by any route-table record: current route slots, every
// transition-receipt old/new code+program ref, and authorization evidence.
// Conservative over-retention is deliberate — a rollback may reach any
// receipt's version. Any query error fails the whole set.
func (s *Service) platformUpdateLiveDigests(ctx context.Context) (map[string]struct{}, error) {
	live := map[string]struct{}{}
	addClosure := func(closureJSON string) {
		var closure struct {
			Artifacts []struct {
				URI    string `json:"uri"`
				SHA256 string `json:"sha256"`
			} `json:"artifacts"`
		}
		_ = json.Unmarshal([]byte(closureJSON), &closure)
		for _, a := range closure.Artifacts {
			live[platformUpdateDigestFromURI(a.URI)] = struct{}{}
			if validHexDigest(a.SHA256) {
				live[a.SHA256] = struct{}{}
			}
		}
	}
	addProgram := func(programJSON string) {
		var program struct {
			Entries []struct {
				ArtifactURI   string `json:"artifact_uri"`
				ContentSHA256 string `json:"content_sha256"`
			} `json:"entries"`
		}
		_ = json.Unmarshal([]byte(programJSON), &program)
		for _, e := range program.Entries {
			live[platformUpdateDigestFromURI(e.ArtifactURI)] = struct{}{}
			if validHexDigest(e.ContentSHA256) {
				live[e.ContentSHA256] = struct{}{}
			}
		}
	}
	refRows, err := s.store.db.QueryContext(ctx, `
		SELECT DISTINCT current_code_ref, current_artifact_program_ref FROM computer_version_route_slots
		UNION SELECT DISTINCT new_code_ref, new_artifact_program_ref FROM computer_version_route_transition_receipts
		UNION SELECT DISTINCT old_code_ref, old_artifact_program_ref FROM computer_version_route_transition_receipts
		UNION SELECT DISTINCT code_ref, artifact_program_ref FROM computer_version_route_authorization_evidence`)
	if err != nil {
		return nil, fmt.Errorf("list route refs: %w", err)
	}
	type refs struct{ code, program string }
	var list []refs
	for refRows.Next() {
		var code, program sql.NullString
		if err := refRows.Scan(&code, &program); err != nil {
			refRows.Close()
			return nil, fmt.Errorf("scan route refs: %w", err)
		}
		list = append(list, refs{code.String, program.String})
	}
	if err := refRows.Err(); err != nil {
		refRows.Close()
		return nil, fmt.Errorf("iterate route refs: %w", err)
	}
	refRows.Close()
	for _, r := range list {
		if err := scanStrings(ctx, s.store.db, `SELECT closure_json FROM computer_version_code_closures WHERE code_ref = ?`, addClosure, r.code); err != nil {
			return nil, fmt.Errorf("code closure %s: %w", r.code, err)
		}
		if err := scanStrings(ctx, s.store.db, `SELECT program_json FROM computer_version_artifact_programs WHERE artifact_program_ref = ?`, addProgram, r.program); err != nil {
			return nil, fmt.Errorf("artifact program %s: %w", r.program, err)
		}
	}
	return live, nil
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

// classifyArtifactNamespace walks one namespace and returns its deletion
// candidates (unreachable and past grace), filling rep's counts.
func (s *Service) classifyArtifactNamespace(ctx context.Context, ns string, live map[string]struct{}, cutoff time.Time, rep *ArtifactGCNamespaceReport) ([]gcCandidate, []string, error) {
	if ns == "file-cas-chunks" {
		return s.classifyFileCASChunks(ctx, cutoff, rep)
	}
	dir, err := s.artifactPath(filepath.Join("sha256", ns))
	if err != nil {
		return nil, nil, err
	}
	var out []gcCandidate
	// file-cas-roots is nested one level deep by computerID
	// (<ns>/<computerID>/<root>.json); the other namespaces are flat.
	if ns == "file-cas-roots" {
		computers, err := os.ReadDir(dir)
		if os.IsNotExist(err) {
			return nil, nil, nil
		}
		if err != nil {
			return nil, nil, err
		}
		for _, c := range computers {
			if c == nil || !c.IsDir() {
				continue
			}
			if err := s.classifyFlatDir(ns, filepath.Join(dir, c.Name()), live, cutoff, rep, &out); err != nil {
				return nil, nil, err
			}
		}
		return out, nil, nil
	}
	if err := s.classifyFlatDir(ns, dir, live, cutoff, rep, &out); err != nil {
		return nil, nil, err
	}
	return out, nil, nil
}

func (s *Service) classifyFlatDir(ns, dir string, live map[string]struct{}, cutoff time.Time, rep *ArtifactGCNamespaceReport, out *[]gcCandidate) error {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, entry := range entries {
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
		s.addCandidate(ns, filepath.Join(dir, name), info, rep, out)
	}
	return nil
}

func (s *Service) addCandidate(ns, path string, info fs.FileInfo, rep *ArtifactGCNamespaceReport, out *[]gcCandidate) {
	rel, err := filepath.Rel(s.artifactsRoot, path)
	if err != nil {
		return
	}
	rep.Eligible++
	rep.EligibleBytes += info.Size()
	*out = append(*out, gcCandidate{ns: ns, rel: filepath.ToSlash(rel), path: path, size: info.Size(), mtime: info.ModTime()})
}

// classifyFileCASChunks classifies every file-cas-chunks/<computerID> dir.
// A chunk is live when the computer's latest root or any root recorded inside
// the grace window names it. A computer whose liveness cannot be proven — no
// recorded root, or any manifest that cannot be read or verified — keeps all
// of its chunks (Unproven) and is reported.
func (s *Service) classifyFileCASChunks(ctx context.Context, cutoff time.Time, rep *ArtifactGCNamespaceReport) ([]gcCandidate, []string, error) {
	dir, err := s.artifactPath(filepath.Join("sha256", "file-cas-chunks"))
	if err != nil {
		return nil, nil, err
	}
	computers, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	var out []gcCandidate
	var warnings []string
	rootless := 0
	for _, c := range computers {
		if err := ctx.Err(); err != nil {
			return nil, warnings, err
		}
		if c == nil || !c.IsDir() || !safeFileCASComponent(c.Name()) {
			continue
		}
		computerID := c.Name()
		reachable, reason, err := s.fileCASReachableChunks(ctx, computerID, cutoff)
		if err != nil {
			return nil, warnings, fmt.Errorf("roots for %s: %w", computerID, err)
		}
		if reason == "no recorded root" {
			rootless++
		} else if reason != "" {
			warnings = append(warnings, fmt.Sprintf("file-cas-chunks: %s: %s; its chunks are kept", computerID, reason))
		}
		walkErr := filepath.WalkDir(filepath.Join(dir, computerID), func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() || !entry.Type().IsRegular() || !validFileCASDigest(entry.Name()) {
				return nil
			}
			rep.Scanned++
			if reachable == nil {
				rep.Unproven++
				return nil
			}
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
			s.addCandidate("file-cas-chunks", path, info, rep, &out)
			return nil
		})
		if walkErr != nil && !os.IsNotExist(walkErr) {
			return nil, warnings, walkErr
		}
	}
	if rootless > 0 {
		warnings = append(warnings, fmt.Sprintf("file-cas-chunks: %d computers have chunks but no recorded root; their chunks are kept", rootless))
	}
	return out, warnings, nil
}

// fileCASReachableChunks returns the chunk digests named by the computer's
// latest root and every root recorded after cutoff. A nil set with a reason
// means liveness could not be proven and nothing may be collected.
func (s *Service) fileCASReachableChunks(ctx context.Context, computerID string, cutoff time.Time) (map[string]struct{}, string, error) {
	roots, err := s.fileCASRootsForGC(ctx, computerID, cutoff)
	if err != nil {
		return nil, "", err
	}
	if len(roots) == 0 {
		return nil, "no recorded root", nil
	}
	reachable := make(map[string]struct{})
	for _, root := range roots {
		manifestData, err := s.readBlob(root.ManifestRef)
		if err != nil {
			return nil, fmt.Sprintf("root %s manifest unreadable (%v)", root.Root, err), nil
		}
		manifest, err := filecas.ParseManifest(manifestData)
		if err != nil {
			return nil, fmt.Sprintf("root %s manifest invalid (%v)", root.Root, err), nil
		}
		if manifest.ComputerID != computerID || manifest.Root != root.Root {
			return nil, fmt.Sprintf("root %s manifest belongs elsewhere", root.Root), nil
		}
		for _, entry := range manifest.Files {
			for _, digest := range entry.Chunks {
				reachable[digest] = struct{}{}
			}
		}
	}
	return reachable, "", nil
}

type artifactGCLogRecord struct {
	At        string `json:"at"`
	Run       string `json:"run"`
	Op        string `json:"op"` // intent | deleted | kept_refreshed | gone | error | aborted
	Namespace string `json:"ns,omitempty"`
	Path      string `json:"path,omitempty"`
	Bytes     int64  `json:"bytes,omitempty"`
	ModTime   string `json:"mtime,omitempty"`
	Error     string `json:"error,omitempty"`
	Reason    string `json:"reason,omitempty"`
}

// deleteArtifactCandidates removes the selected candidates. For each batch it
// first makes an intent record per candidate durable, then re-checks and
// unlinks each one under the CAS reuse lock and records the outcome. A crash
// at any point leaves every unlinked path named by an intent record; the next
// run re-classifies from scratch, so a partial run is simply resumed.
func (s *Service) deleteArtifactCandidates(ctx context.Context, started time.Time, candidates []gcCandidate, cutoff time.Time, report *ArtifactGCReport) error {
	if len(candidates) == 0 {
		return nil
	}
	logDir := filepath.Join(s.artifactsRoot, artifactGCLogDir)
	if err := os.MkdirAll(logDir, 0o700); err != nil {
		return fmt.Errorf("artifact gc: deletion log unavailable, nothing deleted: %w", err)
	}
	logFile, err := os.OpenFile(filepath.Join(logDir, started.UTC().Format("2006-01-02")+".jsonl"), os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o600)
	if err != nil {
		return fmt.Errorf("artifact gc: deletion log unavailable, nothing deleted: %w", err)
	}
	defer logFile.Close()
	writer := bufio.NewWriter(logFile)
	runID := started.UTC().Format("20060102T150405.000000000Z")
	record := func(rec artifactGCLogRecord) {
		rec.At = time.Now().UTC().Format(time.RFC3339Nano)
		rec.Run = runID
		line, _ := json.Marshal(rec)
		_, _ = writer.Write(append(line, '\n'))
	}
	flush := func() error {
		if err := writer.Flush(); err != nil {
			return err
		}
		return logFile.Sync()
	}
	for start := 0; start < len(candidates); start += artifactGCLogBatch {
		end := start + artifactGCLogBatch
		if end > len(candidates) {
			end = len(candidates)
		}
		batch := candidates[start:end]
		for _, c := range batch {
			record(artifactGCLogRecord{Op: "intent", Namespace: c.ns, Path: c.rel, Bytes: c.size, ModTime: c.mtime.UTC().Format(time.RFC3339Nano)})
		}
		if err := flush(); err != nil {
			return fmt.Errorf("artifact gc: deletion log write failed, batch not deleted: %w", err)
		}
		for _, c := range batch {
			if hook := s.artifactGCBeforeUnlink; hook != nil {
				hook(c.rel)
			}
			if err := ctx.Err(); err != nil {
				record(artifactGCLogRecord{Op: "aborted", Error: err.Error()})
				_ = flush()
				return err
			}
			op, size, unlinkErr := s.unlinkIfStillUnreachable(c, cutoff)
			rec := artifactGCLogRecord{Op: op, Namespace: c.ns, Path: c.rel, Bytes: size}
			nsRep := report.Namespaces[c.ns]
			switch op {
			case "deleted":
				nsRep.Deleted++
				nsRep.BytesDeleted += size
			case "error":
				nsRep.Errors++
				rec.Error = unlinkErr.Error()
			default:
				nsRep.KeptOnRecheck++
			}
			report.Namespaces[c.ns] = nsRep
			record(rec)
		}
		if err := flush(); err != nil {
			return fmt.Errorf("artifact gc: deletion log write failed: %w", err)
		}
	}
	return nil
}

// unlinkIfStillUnreachable re-checks a candidate under the CAS reuse lock and
// removes it only if it is still a regular file older than the cutoff. A
// writer that reused (refreshed) it since classification wins.
func (s *Service) unlinkIfStillUnreachable(c gcCandidate, cutoff time.Time) (string, int64, error) {
	s.store.casReuseMu.Lock()
	defer s.store.casReuseMu.Unlock()
	info, err := os.Lstat(c.path)
	if os.IsNotExist(err) {
		return "gone", 0, nil
	}
	if err != nil {
		return "error", 0, err
	}
	if !info.Mode().IsRegular() || !info.ModTime().Before(cutoff) {
		return "kept_refreshed", info.Size(), nil
	}
	if err := os.Remove(c.path); err != nil {
		if os.IsNotExist(err) {
			return "gone", 0, nil
		}
		return "error", 0, err
	}
	return "deleted", info.Size(), nil
}

// fileCASRootsForGC returns the computer's latest root plus every root
// recorded after cutoff.
func (s *Service) fileCASRootsForGC(ctx context.Context, computerID string, cutoff time.Time) ([]FileRootRecord, error) {
	latest, err := s.store.LatestFileRoots(ctx, computerID, 1)
	if err != nil {
		return nil, err
	}
	rows, err := s.store.db.QueryContext(ctx, `SELECT computer_id,root,manifest_ref,head_sequence,created_at FROM computer_file_roots WHERE computer_id=? AND created_at>? ORDER BY created_at DESC, root DESC`, computerID, cutoff)
	if err != nil {
		return nil, fmt.Errorf("file cas: list recent roots: %w", err)
	}
	defer rows.Close()
	byRoot := make(map[string]FileRootRecord, len(latest))
	for _, root := range latest {
		byRoot[root.Root] = root
	}
	for rows.Next() {
		var root FileRootRecord
		if err := rows.Scan(&root.ComputerID, &root.Root, &root.ManifestRef, &root.HeadSequence, &root.CreatedAt); err != nil {
			return nil, err
		}
		byRoot[root.Root] = root
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make([]FileRootRecord, 0, len(byRoot))
	for _, root := range byRoot {
		out = append(out, root)
	}
	return out, nil
}

// GCRunner periodically runs the artifact GC when enabled.
type GCRunner struct {
	service      *Service
	config       ArtifactGCConfig
	interval     time.Duration
	initialDelay time.Duration
}

// NewGCRunner builds a periodic artifact-GC runner. interval<=0 -> hourly.
// The first pass runs after min(interval, 10m): corpusd restarts on every
// deploy, and a first pass a full interval out ran 9 times in 26 h on a
// deploy-heavy day (2026-10-14).
func NewGCRunner(service *Service, cfg ArtifactGCConfig, interval time.Duration) *GCRunner {
	if interval <= 0 {
		interval = time.Hour
	}
	delay := 10 * time.Minute
	if interval < delay {
		delay = interval
	}
	return &GCRunner{service: service, config: normalizeArtifactGCConfig(cfg), interval: interval, initialDelay: delay}
}

// Start runs the sweep on the interval until ctx is done; disabled when mode
// is off. It logs the full report (per namespace) for every pass.
func (r *GCRunner) Start(ctx context.Context) {
	if r == nil || r.service == nil || r.config.Mode == ArtifactGCModeOff {
		return
	}
	go func() {
		timer := time.NewTimer(r.initialDelay)
		defer timer.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-timer.C:
				report, err := r.service.RunArtifactGC(ctx, r.config)
				summary, _ := json.Marshal(report)
				if err != nil {
					log.Printf("artifact gc: error: %v report=%s", err, summary)
				} else {
					log.Printf("artifact gc: mode=%s deleted=%d bytes=%d warnings=%d report=%s", report.Mode, report.Deleted, report.BytesDeleted, len(report.Warnings), summary)
				}
				timer.Reset(r.interval)
			}
		}
	}()
}
