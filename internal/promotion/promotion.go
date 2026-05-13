package promotion

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/shipper"
)

// CandidateWorld records the identity and git geometry of a background VM
// candidate before it is eligible for canonical promotion.
type CandidateWorld struct {
	CandidateID          string `json:"candidate_id"`
	OwnerID              string `json:"owner_id"`
	ForegroundDesktopID  string `json:"foreground_desktop_id,omitempty"`
	ParentRunID          string `json:"parent_loop_id,omitempty"`
	CandidateRunID       string `json:"candidate_loop_id"`
	VMID                 string `json:"vm_id"`
	SnapshotID           string `json:"snapshot_id,omitempty"`
	Purpose              string `json:"purpose"`
	ObjectiveFingerprint string `json:"objective_fingerprint,omitempty"`
	BaseSHA              string `json:"base_sha"`
	WorkerHeadSHA        string `json:"worker_head_sha"`
	PatchsetSHA256       string `json:"patchset_sha256,omitempty"`
	ManifestPath         string `json:"manifest_path"`
	PatchsetPath         string `json:"patchset_path"`
	IntegrationBranch    string `json:"integration_branch"`
	CreatedAt            string `json:"created_at"`
}

// VerifierContract is a record of what must be checked before a candidate can
// be promoted. It is intentionally a contract, not a verifier-agent type.
type VerifierContract struct {
	ContractID              string   `json:"contract_id"`
	Target                  string   `json:"target"`
	Purpose                 string   `json:"purpose"`
	Invariants              []string `json:"invariants,omitempty"`
	RequiredChecks          []string `json:"required_checks,omitempty"`
	CapabilityProfile       string   `json:"capability_profile,omitempty"`
	IndependenceRequirement string   `json:"independence_requirement,omitempty"`
	ResultSchema            string   `json:"result_schema,omitempty"`
	EvidencePaths           []string `json:"evidence_paths,omitempty"`
}

type VerifierResult struct {
	ContractID    string                `json:"contract_id"`
	Status        string                `json:"status"`
	Checks        []shipper.CheckReport `json:"checks,omitempty"`
	EvidencePaths []string              `json:"evidence_paths,omitempty"`
	Error         string                `json:"error,omitempty"`
	VerifiedAt    string                `json:"verified_at"`
}

type RollbackPoint struct {
	DestinationBranch string `json:"destination_branch"`
	BaseSHA           string `json:"base_sha"`
	IntegrationBranch string `json:"integration_branch"`
	RevertCommand     string `json:"revert_command"`
}

type Report struct {
	Status              string             `json:"status"`
	Candidate           CandidateWorld     `json:"candidate"`
	Integration         *shipper.Report    `json:"integration,omitempty"`
	VerifierContracts   []VerifierContract `json:"verifier_contracts"`
	VerifierResults     []VerifierResult   `json:"verifier_results"`
	Rollback            RollbackPoint      `json:"rollback"`
	CanonicalMutated    bool               `json:"canonical_mutated"`
	PromotionApproved   bool               `json:"promotion_approved"`
	PromotionCommitSHA  string             `json:"promotion_commit_sha,omitempty"`
	PromotionDecisionAt string             `json:"promotion_decision_at,omitempty"`
	ReportPath          string             `json:"report_path,omitempty"`
	CreatedAt           string             `json:"created_at"`
}

type PrepareOptions struct {
	RepoPath          string
	ManifestPath      string
	PatchsetPath      string
	IntegrationBranch string
	DestinationBranch string
	ReportPath        string
	CommitMessage     string
	Candidate         CandidateWorld
	Contracts         []VerifierContract
}

// PrepareIntegrationCandidate imports a worker patchset into an isolated
// integration branch and runs verifier contracts there. It never mutates the
// destination/canonical branch.
func PrepareIntegrationCandidate(ctx context.Context, opts PrepareOptions) (*Report, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	opts.RepoPath = strings.TrimSpace(opts.RepoPath)
	if opts.RepoPath == "" {
		opts.RepoPath = "."
	}
	if strings.TrimSpace(opts.DestinationBranch) == "" {
		opts.DestinationBranch = "main"
	}
	if strings.TrimSpace(opts.IntegrationBranch) == "" {
		opts.IntegrationBranch = opts.Candidate.IntegrationBranch
	}
	if strings.TrimSpace(opts.IntegrationBranch) == "" {
		opts.IntegrationBranch = "agent/" + safePart(opts.Candidate.CandidateRunID) + "/candidate"
	}
	if strings.TrimSpace(opts.ManifestPath) == "" {
		opts.ManifestPath = opts.Candidate.ManifestPath
	}
	if strings.TrimSpace(opts.PatchsetPath) == "" {
		opts.PatchsetPath = opts.Candidate.PatchsetPath
	}

	if err := validateCandidate(opts.Candidate); err != nil {
		return nil, err
	}
	if len(opts.Contracts) == 0 {
		return nil, errors.New("at least one verifier contract is required")
	}

	candidate := opts.Candidate
	candidate.ManifestPath = opts.ManifestPath
	candidate.PatchsetPath = opts.PatchsetPath
	candidate.IntegrationBranch = opts.IntegrationBranch
	if strings.TrimSpace(candidate.CreatedAt) == "" {
		candidate.CreatedAt = now
	}

	report := &Report{
		Status:            "integration_pending",
		Candidate:         candidate,
		VerifierContracts: opts.Contracts,
		Rollback: RollbackPoint{
			DestinationBranch: opts.DestinationBranch,
			BaseSHA:           candidate.BaseSHA,
			IntegrationBranch: opts.IntegrationBranch,
			RevertCommand:     fmt.Sprintf("git switch %s && git reset --hard %s", opts.DestinationBranch, candidate.BaseSHA),
		},
		CreatedAt:  now,
		ReportPath: opts.ReportPath,
	}

	integration, err := shipper.ImportPatchset(ctx, shipper.Options{
		RepoPath:      opts.RepoPath,
		ManifestPath:  opts.ManifestPath,
		PatchsetPath:  opts.PatchsetPath,
		Branch:        opts.IntegrationBranch,
		CommitMessage: opts.CommitMessage,
		Checks:        nil,
		Push:          false,
		ReportPath:    "",
	})
	if err != nil {
		report.Status = "integration_failed"
		_ = writeReport(opts.ReportPath, report)
		return report, err
	}
	report.Integration = integration
	report.Status = "integrated"

	results, verifyErr := runVerifierContracts(ctx, opts.RepoPath, opts.Contracts)
	report.VerifierResults = results
	if verifyErr != nil {
		report.Status = "verification_failed"
		_ = writeReport(opts.ReportPath, report)
		return report, verifyErr
	}
	report.Status = "verified"
	if err := writeReport(opts.ReportPath, report); err != nil {
		return report, err
	}
	return report, nil
}

// ApplyVerifiedPromotion moves the destination branch to the already verified
// integration branch. It requires explicit approval and blocks foreground
// divergence by requiring the destination branch to still equal the base SHA.
func ApplyVerifiedPromotion(ctx context.Context, repoPath string, report *Report, approved bool) (*Report, error) {
	if report == nil {
		return nil, errors.New("promotion report is required")
	}
	if !approved {
		return report, errors.New("promotion requires explicit approval")
	}
	if report.Status != "verified" {
		return report, fmt.Errorf("candidate status %q is not verified", report.Status)
	}
	if report.Integration == nil {
		return report, errors.New("integration report is required")
	}
	for _, result := range report.VerifierResults {
		if result.Status != "passed" {
			return report, fmt.Errorf("verifier contract %s has status %s", result.ContractID, result.Status)
		}
	}
	repoPath = strings.TrimSpace(repoPath)
	if repoPath == "" {
		repoPath = "."
	}
	if err := ensureCleanRepo(ctx, repoPath); err != nil {
		return report, err
	}
	if _, err := gitOutput(ctx, repoPath, "switch", report.Rollback.DestinationBranch); err != nil {
		return report, err
	}
	head, err := gitOutput(ctx, repoPath, "rev-parse", "HEAD")
	if err != nil {
		return report, err
	}
	head = strings.TrimSpace(head)
	if head != report.Rollback.BaseSHA {
		return report, fmt.Errorf("destination branch diverged: head %s does not match rollback base %s", head, report.Rollback.BaseSHA)
	}
	if _, err := gitOutput(ctx, repoPath, "merge", "--ff-only", report.Rollback.IntegrationBranch); err != nil {
		return report, err
	}
	promotedHead, err := gitOutput(ctx, repoPath, "rev-parse", "HEAD")
	if err != nil {
		return report, err
	}
	report.Status = "promoted"
	report.CanonicalMutated = true
	report.PromotionApproved = true
	report.PromotionCommitSHA = strings.TrimSpace(promotedHead)
	report.PromotionDecisionAt = time.Now().UTC().Format(time.RFC3339)
	if err := writeReport(report.ReportPath, report); err != nil {
		return report, err
	}
	return report, nil
}

func validateCandidate(candidate CandidateWorld) error {
	required := map[string]string{
		"candidate_id":      candidate.CandidateID,
		"owner_id":          candidate.OwnerID,
		"candidate_loop_id": candidate.CandidateRunID,
		"vm_id":             candidate.VMID,
		"base_sha":          candidate.BaseSHA,
		"worker_head_sha":   candidate.WorkerHeadSHA,
		"manifest_path":     candidate.ManifestPath,
		"patchset_path":     candidate.PatchsetPath,
	}
	for name, value := range required {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("candidate %s is required", name)
		}
	}
	return nil
}

func runVerifierContracts(ctx context.Context, repo string, contracts []VerifierContract) ([]VerifierResult, error) {
	results := make([]VerifierResult, 0, len(contracts))
	var firstErr error
	for _, contract := range contracts {
		result := VerifierResult{
			ContractID:    strings.TrimSpace(contract.ContractID),
			Status:        "passed",
			EvidencePaths: append([]string(nil), contract.EvidencePaths...),
			VerifiedAt:    time.Now().UTC().Format(time.RFC3339),
		}
		if result.ContractID == "" {
			result.ContractID = strings.TrimSpace(contract.Target)
		}
		if result.ContractID == "" {
			result.ContractID = "contract"
		}
		for _, check := range contract.RequiredChecks {
			report := runCheck(ctx, repo, check)
			result.Checks = append(result.Checks, report)
			if report.ExitCode != 0 && firstErr == nil {
				result.Status = "failed"
				result.Error = fmt.Sprintf("check failed: %s", check)
				firstErr = errors.New(result.Error)
			}
		}
		results = append(results, result)
	}
	return results, firstErr
}

func runCheck(ctx context.Context, repo, check string) shipper.CheckReport {
	check = strings.TrimSpace(check)
	if check == "" {
		return shipper.CheckReport{Command: check, ExitCode: 0}
	}
	cmd := exec.CommandContext(ctx, "/bin/sh", "-c", check)
	cmd.Dir = repo
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	report := shipper.CheckReport{Command: check, ExitCode: 0, Output: out.String()}
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			report.ExitCode = exitErr.ExitCode()
		} else {
			report.ExitCode = -1
		}
	}
	return report
}

func writeReport(path string, report *Report) error {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func ensureCleanRepo(ctx context.Context, repo string) error {
	status, err := gitOutput(ctx, repo, "status", "--porcelain")
	if err != nil {
		return err
	}
	if strings.TrimSpace(status) != "" {
		return errors.New("promotion repo must be clean before canonical mutation")
	}
	return nil
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

func safePart(value string) string {
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
