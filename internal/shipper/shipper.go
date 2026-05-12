package shipper

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

type Manifest struct {
	RunID              string       `json:"run_id"`
	TraceID            string       `json:"trace_id"`
	VMID               string       `json:"vm_id"`
	SnapshotID         string       `json:"snapshot_id,omitempty"`
	BaseSHA            string       `json:"base_sha"`
	ExpectedHeadSHA    string       `json:"expected_head_sha,omitempty"`
	Verification       []TestResult `json:"verification,omitempty"`
	ResidualRisks      []string     `json:"residual_risks,omitempty"`
	Summary            string       `json:"summary,omitempty"`
	GeneratedAt        string       `json:"generated_at,omitempty"`
	VerificationSource string       `json:"verification_source,omitempty"`
}

type TestResult struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Output string `json:"output,omitempty"`
}

type Options struct {
	RepoPath      string
	ManifestPath  string
	PatchsetPath  string
	Branch        string
	Remote        string
	Checks        []string
	ReportPath    string
	CommitMessage string
	Push          bool
}

type ExportOptions struct {
	RepoPath   string
	OutputDir  string
	BaseSHA    string
	RunID      string
	TraceID    string
	VMID       string
	SnapshotID string
	Summary    string
	Checks     []string
}

type CheckReport struct {
	Command  string `json:"command"`
	ExitCode int    `json:"exit_code"`
	Output   string `json:"output"`
}

type Report struct {
	Status       string        `json:"status"`
	Branch       string        `json:"branch"`
	Remote       string        `json:"remote,omitempty"`
	Pushed       bool          `json:"pushed"`
	BaseSHA      string        `json:"base_sha"`
	HeadSHA      string        `json:"head_sha"`
	ManifestPath string        `json:"manifest_path"`
	PatchsetPath string        `json:"patchset_path"`
	Checks       []CheckReport `json:"checks"`
	ImportedAt   string        `json:"imported_at"`
}

type ExportReport struct {
	Status       string        `json:"status"`
	BaseSHA      string        `json:"base_sha"`
	HeadSHA      string        `json:"head_sha"`
	ManifestPath string        `json:"manifest_path"`
	PatchsetPath string        `json:"patchset_path"`
	Checks       []CheckReport `json:"checks"`
	ExportedAt   string        `json:"exported_at"`
}

var safeBranchRE = regexp.MustCompile(`^agent/[A-Za-z0-9][A-Za-z0-9._-]*/[A-Za-z0-9][A-Za-z0-9._-]*$`)

func ExportPatchset(ctx context.Context, opts ExportOptions) (*ExportReport, error) {
	opts.RepoPath = strings.TrimSpace(opts.RepoPath)
	if opts.RepoPath == "" {
		opts.RepoPath = "."
	}
	opts.OutputDir = strings.TrimSpace(opts.OutputDir)
	if opts.OutputDir == "" {
		return nil, errors.New("output dir is required")
	}
	manifest := Manifest{
		RunID:           strings.TrimSpace(opts.RunID),
		TraceID:         strings.TrimSpace(opts.TraceID),
		VMID:            strings.TrimSpace(opts.VMID),
		SnapshotID:      strings.TrimSpace(opts.SnapshotID),
		BaseSHA:         strings.TrimSpace(opts.BaseSHA),
		Summary:         strings.TrimSpace(opts.Summary),
		GeneratedAt:     time.Now().UTC().Format(time.RFC3339),
		ExpectedHeadSHA: "",
	}
	if err := validateManifest(manifest); err != nil {
		return nil, err
	}
	if err := ensureCleanRepo(ctx, opts.RepoPath); err != nil {
		return nil, err
	}
	head, err := gitOutput(ctx, opts.RepoPath, "rev-parse", "HEAD")
	if err != nil {
		return nil, err
	}
	head = strings.TrimSpace(head)
	if head == manifest.BaseSHA {
		return nil, errors.New("worker HEAD equals base_sha; no committed work to export")
	}
	if _, err := gitOutput(ctx, opts.RepoPath, "merge-base", "--is-ancestor", manifest.BaseSHA, head); err != nil {
		return nil, fmt.Errorf("base_sha %s is not an ancestor of HEAD %s: %w", manifest.BaseSHA, head, err)
	}

	checkReports, err := runChecks(ctx, opts.RepoPath, opts.Checks)
	if err != nil {
		return nil, err
	}
	manifest.ExpectedHeadSHA = head
	manifest.Verification = make([]TestResult, 0, len(checkReports))
	for _, check := range checkReports {
		status := "passed"
		if check.ExitCode != 0 {
			status = "failed"
		}
		manifest.Verification = append(manifest.Verification, TestResult{
			Name:   check.Command,
			Status: status,
			Output: check.Output,
		})
	}
	manifest.VerificationSource = "shipper export"

	diff, err := gitOutput(ctx, opts.RepoPath, "diff", "--binary", manifest.BaseSHA+"..HEAD")
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(diff) == "" {
		return nil, errors.New("git diff produced an empty patchset")
	}

	if err := os.MkdirAll(opts.OutputDir, 0o755); err != nil {
		return nil, err
	}
	patchPath := filepath.Join(opts.OutputDir, "changes.patch")
	manifestPath := filepath.Join(opts.OutputDir, "manifest.json")
	reportPath := filepath.Join(opts.OutputDir, "export-report.json")
	if err := os.WriteFile(patchPath, []byte(diff), 0o644); err != nil {
		return nil, fmt.Errorf("write patchset: %w", err)
	}
	manifestData, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(manifestPath, append(manifestData, '\n'), 0o644); err != nil {
		return nil, fmt.Errorf("write manifest: %w", err)
	}

	report := &ExportReport{
		Status:       "exported",
		BaseSHA:      manifest.BaseSHA,
		HeadSHA:      head,
		ManifestPath: manifestPath,
		PatchsetPath: patchPath,
		Checks:       checkReports,
		ExportedAt:   manifest.GeneratedAt,
	}
	if err := writeExportReport(reportPath, report); err != nil {
		return nil, err
	}
	return report, nil
}

func ImportPatchset(ctx context.Context, opts Options) (*Report, error) {
	opts.RepoPath = strings.TrimSpace(opts.RepoPath)
	if opts.RepoPath == "" {
		opts.RepoPath = "."
	}
	opts.Remote = strings.TrimSpace(opts.Remote)
	if opts.Remote == "" {
		opts.Remote = "origin"
	}

	manifest, err := loadManifest(opts.ManifestPath)
	if err != nil {
		return nil, err
	}
	if err := validateManifest(manifest); err != nil {
		return nil, err
	}
	branch := strings.TrimSpace(opts.Branch)
	if branch == "" {
		branch = "agent/" + sanitizeBranchPart(manifest.RunID) + "/ship"
	}
	if err := validateBranch(branch); err != nil {
		return nil, err
	}
	if err := ensureCleanRepo(ctx, opts.RepoPath); err != nil {
		return nil, err
	}
	head, err := gitOutput(ctx, opts.RepoPath, "rev-parse", "HEAD")
	if err != nil {
		return nil, err
	}
	head = strings.TrimSpace(head)
	if head != manifest.BaseSHA {
		return nil, fmt.Errorf("repo HEAD %s does not match manifest base_sha %s", head, manifest.BaseSHA)
	}

	if _, err := gitOutput(ctx, opts.RepoPath, "switch", "-C", branch, manifest.BaseSHA); err != nil {
		return nil, err
	}
	if err := applyPatchset(ctx, opts.RepoPath, opts.PatchsetPath); err != nil {
		return nil, err
	}
	if err := ensureStagedChanges(ctx, opts.RepoPath); err != nil {
		return nil, err
	}

	message := strings.TrimSpace(opts.CommitMessage)
	if message == "" {
		message = defaultCommitMessage(manifest)
	}
	if _, err := gitOutput(ctx, opts.RepoPath, "commit", "-m", message, "-m", provenanceBody(manifest)); err != nil {
		return nil, err
	}

	checkReports, err := runChecks(ctx, opts.RepoPath, opts.Checks)
	if err != nil {
		return nil, err
	}

	newHead, err := gitOutput(ctx, opts.RepoPath, "rev-parse", "HEAD")
	if err != nil {
		return nil, err
	}
	newHead = strings.TrimSpace(newHead)

	report := &Report{
		Status:       "imported",
		Branch:       branch,
		Remote:       opts.Remote,
		BaseSHA:      manifest.BaseSHA,
		HeadSHA:      newHead,
		ManifestPath: opts.ManifestPath,
		PatchsetPath: opts.PatchsetPath,
		Checks:       checkReports,
		ImportedAt:   time.Now().UTC().Format(time.RFC3339),
	}

	if opts.Push {
		if _, err := gitOutput(ctx, opts.RepoPath, "push", "-u", opts.Remote, branch); err != nil {
			return nil, err
		}
		report.Pushed = true
		report.Status = "pushed"
	}

	if strings.TrimSpace(opts.ReportPath) != "" {
		if err := writeReport(opts.ReportPath, report); err != nil {
			return nil, err
		}
	}
	return report, nil
}

func loadManifest(path string) (Manifest, error) {
	if strings.TrimSpace(path) == "" {
		return Manifest{}, errors.New("manifest path is required")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, fmt.Errorf("read manifest: %w", err)
	}
	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return Manifest{}, fmt.Errorf("decode manifest: %w", err)
	}
	return manifest, nil
}

func validateManifest(manifest Manifest) error {
	if strings.TrimSpace(manifest.RunID) == "" {
		return errors.New("manifest run_id is required")
	}
	if strings.TrimSpace(manifest.TraceID) == "" {
		return errors.New("manifest trace_id is required")
	}
	if strings.TrimSpace(manifest.VMID) == "" {
		return errors.New("manifest vm_id is required")
	}
	if strings.TrimSpace(manifest.BaseSHA) == "" {
		return errors.New("manifest base_sha is required")
	}
	return nil
}

func validateBranch(branch string) error {
	if !safeBranchRE.MatchString(branch) || strings.Contains(branch, "..") || strings.HasSuffix(branch, ".lock") {
		return fmt.Errorf("branch %q must match agent/<run-id>/<slug> with safe characters", branch)
	}
	return nil
}

func sanitizeBranchPart(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			b.WriteRune(r)
			continue
		}
		if b.Len() > 0 {
			b.WriteByte('-')
		}
	}
	out := strings.Trim(b.String(), "-.")
	if out == "" {
		return "run"
	}
	return out
}

func ensureCleanRepo(ctx context.Context, repo string) error {
	status, err := gitOutput(ctx, repo, "status", "--porcelain")
	if err != nil {
		return err
	}
	if strings.TrimSpace(status) != "" {
		return errors.New("shipper repo must be clean before import")
	}
	return nil
}

func applyPatchset(ctx context.Context, repo, patchsetPath string) error {
	patches, err := patchFiles(patchsetPath)
	if err != nil {
		return err
	}
	for _, patch := range patches {
		if _, err := gitOutput(ctx, repo, "apply", "--index", patch); err != nil {
			return fmt.Errorf("apply patch %s: %w", patch, err)
		}
	}
	return nil
}

func patchFiles(path string) ([]string, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("patchset path is required")
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("stat patchset: %w", err)
	}
	if !info.IsDir() {
		return []string{path}, nil
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("read patchset dir: %w", err)
	}
	var patches []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(name, ".patch") || strings.HasSuffix(name, ".diff") {
			patches = append(patches, filepath.Join(path, name))
		}
	}
	sort.Strings(patches)
	if len(patches) == 0 {
		return nil, fmt.Errorf("patchset directory %s contains no .patch or .diff files", path)
	}
	return patches, nil
}

func ensureStagedChanges(ctx context.Context, repo string) error {
	_, err := gitOutput(ctx, repo, "diff", "--cached", "--quiet")
	if err == nil {
		return errors.New("patchset produced no staged changes")
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
		return nil
	}
	return err
}

func runChecks(ctx context.Context, repo string, checks []string) ([]CheckReport, error) {
	var reports []CheckReport
	for _, check := range checks {
		check = strings.TrimSpace(check)
		if check == "" {
			continue
		}
		cmd := exec.CommandContext(ctx, "/bin/sh", "-c", check)
		cmd.Dir = repo
		var out bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &out
		err := cmd.Run()
		report := CheckReport{Command: check, ExitCode: 0, Output: out.String()}
		if err != nil {
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				report.ExitCode = exitErr.ExitCode()
			} else {
				report.ExitCode = -1
			}
			reports = append(reports, report)
			return reports, fmt.Errorf("check failed: %s", check)
		}
		reports = append(reports, report)
	}
	return reports, nil
}

func defaultCommitMessage(manifest Manifest) string {
	summary := strings.TrimSpace(manifest.Summary)
	if summary == "" {
		summary = "Import verified agent patchset"
	}
	return summary
}

func provenanceBody(manifest Manifest) string {
	lines := []string{
		"Choir-Run-ID: " + manifest.RunID,
		"Choir-Trace-ID: " + manifest.TraceID,
		"Choir-VM-ID: " + manifest.VMID,
		"Choir-Base-SHA: " + manifest.BaseSHA,
	}
	if strings.TrimSpace(manifest.SnapshotID) != "" {
		lines = append(lines, "Choir-Snapshot-ID: "+manifest.SnapshotID)
	}
	if strings.TrimSpace(manifest.ExpectedHeadSHA) != "" {
		lines = append(lines, "Choir-Worker-Head-SHA: "+manifest.ExpectedHeadSHA)
	}
	if len(manifest.ResidualRisks) > 0 {
		lines = append(lines, "", "Residual risks:")
		for _, risk := range manifest.ResidualRisks {
			if strings.TrimSpace(risk) != "" {
				lines = append(lines, "- "+strings.TrimSpace(risk))
			}
		}
	}
	return strings.Join(lines, "\n")
}

func writeReport(path string, report *Report) error {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func writeExportReport(path string, report *ExportReport) error {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func gitOutput(ctx context.Context, repo string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = repo
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	if err != nil {
		return out.String(), fmt.Errorf("git %s: %s: %w", strings.Join(args, " "), strings.TrimSpace(out.String()), err)
	}
	return out.String(), nil
}
