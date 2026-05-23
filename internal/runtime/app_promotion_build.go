package runtime

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

type appAdoptionBuildReport struct {
	Required              bool                        `json:"required"`
	Status                string                      `json:"status"`
	WorkspacePath         string                      `json:"workspace_path,omitempty"`
	BuildScratchPath      string                      `json:"build_scratch_path,omitempty"`
	BaseSHA               string                      `json:"base_sha,omitempty"`
	HeadSHA               string                      `json:"head_sha,omitempty"`
	RuntimeArtifactPath   string                      `json:"runtime_artifact_path,omitempty"`
	RuntimeArtifactDigest string                      `json:"runtime_artifact_digest,omitempty"`
	UIArtifactPath        string                      `json:"ui_artifact_path,omitempty"`
	UIArtifactDigest      string                      `json:"ui_artifact_digest,omitempty"`
	Commands              []appPromotionCommandReport `json:"commands,omitempty"`
	Error                 string                      `json:"error,omitempty"`
}

type appPromotionCommandReport struct {
	Name     string `json:"name"`
	Command  string `json:"command"`
	ExitCode int    `json:"exit_code"`
	Output   string `json:"output,omitempty"`
	Duration string `json:"duration,omitempty"`
}

func (rt *Runtime) materializeAppAdoptionCandidate(ctx context.Context, pkg types.AppChangePackageRecord, rec types.AppAdoptionRecord, cutoverRef string) (appAdoptionBuildReport, error) {
	report := appAdoptionBuildReport{Required: true, Status: "pending"}
	if rt == nil {
		return report, fmt.Errorf("recipient build: runtime unavailable")
	}
	sourceRepo := strings.TrimSpace(rt.cfg.PromotionSourceRepo)
	if sourceRepo == "" {
		return report, fmt.Errorf("recipient build: promotion source repo is not configured")
	}
	root := strings.TrimSpace(rt.cfg.PromotionWorkspaceRoot)
	if root == "" {
		return report, fmt.Errorf("recipient build: promotion workspace root is not configured")
	}
	candidateDir, err := safeAppPromotionChildPath(root, rec.AdoptionID)
	if err != nil {
		return report, err
	}
	if err := os.RemoveAll(candidateDir); err != nil {
		return report, fmt.Errorf("recipient build: clear workspace: %w", err)
	}
	if err := os.MkdirAll(candidateDir, 0o755); err != nil {
		return report, fmt.Errorf("recipient build: create workspace: %w", err)
	}
	repoPath := filepath.Join(candidateDir, "repo")
	report.WorkspacePath = repoPath
	buildEnv, buildScratchPath, err := appPromotionBuildEnv(candidateDir)
	if err != nil {
		return report, err
	}
	report.BuildScratchPath = buildScratchPath

	buildCtx := ctx
	cancel := func() {}
	if rt.cfg.AppPromotionBuildTimeout > 0 {
		buildCtx, cancel = context.WithTimeout(ctx, rt.cfg.AppPromotionBuildTimeout)
	}
	defer cancel()

	var cmdReport appPromotionCommandReport
	cmdReport, err = runAppPromotionCommand(buildCtx, "", buildEnv, "clone", "git", "clone", "--no-checkout", sourceRepo, repoPath)
	report.Commands = append(report.Commands, cmdReport)
	if err != nil {
		return report, fmt.Errorf("recipient build: clone source repo: %w", err)
	}
	for _, cfg := range [][]string{
		{"git-config-name", "git", "config", "user.name", "Choir App Adoption Builder"},
		{"git-config-email", "git", "config", "user.email", "app-adoption@choir.local"},
		{"git-fetch", "git", "fetch", "--all", "--prune"},
	} {
		cmdReport, err = runAppPromotionCommand(buildCtx, repoPath, buildEnv, cfg[0], cfg[1], cfg[2:]...)
		report.Commands = append(report.Commands, cmdReport)
		if err != nil {
			return report, fmt.Errorf("recipient build: %s: %w", cfg[0], err)
		}
	}

	baseRef := appPromotionBaseRef(pkg, rec, cutoverRef)
	branch := "app-adoptions/" + safeRefPart(rec.AdoptionID)
	cmdReport, err = runAppPromotionCommand(buildCtx, repoPath, buildEnv, "checkout-base", "git", "switch", "-C", branch, baseRef)
	report.Commands = append(report.Commands, cmdReport)
	if err != nil {
		return report, fmt.Errorf("recipient build: checkout base %s: %w", baseRef, err)
	}
	report.BaseSHA, err = appPromotionGitOutput(buildCtx, repoPath, "rev-parse", "HEAD")
	if err != nil {
		return report, fmt.Errorf("recipient build: resolve base sha: %w", err)
	}

	if err := writeAndApplyAppPromotionPatch(buildCtx, repoPath, candidateDir, buildEnv, pkg.RuntimeSourceDelta, "runtime.patch"); err != nil {
		return report, err
	}
	if err := writeAndApplyAppPromotionPatch(buildCtx, repoPath, candidateDir, buildEnv, pkg.UISourceDelta, "ui.patch"); err != nil {
		return report, err
	}
	cmdReport, err = runAppPromotionCommand(buildCtx, repoPath, buildEnv, "commit-candidate", "git", "commit", "-m", "Apply app change package "+pkg.PackageID)
	report.Commands = append(report.Commands, cmdReport)
	if err != nil {
		return report, fmt.Errorf("recipient build: commit candidate changes: %w", err)
	}
	report.HeadSHA, err = appPromotionGitOutput(buildCtx, repoPath, "rev-parse", "HEAD")
	if err != nil {
		return report, fmt.Errorf("recipient build: resolve candidate sha: %w", err)
	}

	cmdReport, err = runAppPromotionShellCommand(buildCtx, repoPath, buildEnv, "runtime-build", rt.cfg.AppPromotionRuntimeBuildCommand)
	report.Commands = append(report.Commands, cmdReport)
	if err != nil {
		return report, fmt.Errorf("recipient build: runtime build: %w", err)
	}
	runtimePath := filepath.Join(repoPath, rt.cfg.AppPromotionRuntimeArtifactPath)
	report.RuntimeArtifactPath = rt.cfg.AppPromotionRuntimeArtifactPath
	report.RuntimeArtifactDigest, err = hashAppPromotionPath(runtimePath)
	if err != nil {
		return report, fmt.Errorf("recipient build: hash runtime artifact: %w", err)
	}

	cmdReport, err = runAppPromotionShellCommand(buildCtx, repoPath, buildEnv, "ui-build", rt.cfg.AppPromotionUIBuildCommand)
	report.Commands = append(report.Commands, cmdReport)
	if err != nil {
		return report, fmt.Errorf("recipient build: UI build: %w", err)
	}
	uiPath := filepath.Join(repoPath, rt.cfg.AppPromotionUIArtifactPath)
	report.UIArtifactPath = rt.cfg.AppPromotionUIArtifactPath
	report.UIArtifactDigest, err = hashAppPromotionPath(uiPath)
	if err != nil {
		return report, fmt.Errorf("recipient build: hash UI artifact: %w", err)
	}

	report.Status = "passed"
	return report, nil
}

func writeAndApplyAppPromotionPatch(ctx context.Context, repoPath, candidateDir string, env []string, patchText, name string) error {
	if strings.TrimSpace(patchText) == "" {
		return nil
	}
	if !looksLikeGitPatch(patchText) {
		return fmt.Errorf("recipient build: %s is not a git patch", name)
	}
	patchPath := filepath.Join(candidateDir, name)
	if err := os.WriteFile(patchPath, []byte(patchText), 0o644); err != nil {
		return fmt.Errorf("recipient build: write %s: %w", name, err)
	}
	if _, err := runAppPromotionCommand(ctx, repoPath, env, "apply-"+name, "git", "apply", "--index", patchPath); err != nil {
		return fmt.Errorf("recipient build: apply %s: %w", name, err)
	}
	return nil
}

func looksLikeGitPatch(value string) bool {
	trimmed := strings.TrimSpace(value)
	return strings.HasPrefix(trimmed, "diff --git ") || strings.Contains(trimmed, "\ndiff --git ")
}

func appPromotionBaseRef(pkg types.AppChangePackageRecord, rec types.AppAdoptionRecord, cutoverRef string) string {
	for _, candidate := range []string{
		stringFromMap(appChangePackageManifest(pkg), "source_ledger_base_ref"),
		rec.TargetActiveSourceRefAtCandidateStart,
		cutoverRef,
		pkg.SourceActiveRef,
		os.Getenv("RUNTIME_WORKER_REPO_BASE_SHA"),
		"origin/main",
		"HEAD",
	} {
		candidate = appPromotionCheckoutRef(candidate)
		if candidate == "" {
			continue
		}
		return candidate
	}
	return "origin/main"
}

func appPromotionCheckoutRef(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if strings.HasPrefix(value, "refs/computers/") || strings.HasPrefix(value, "refs/platform-computers/") {
		return ""
	}
	if strings.HasPrefix(value, "refs/heads/") {
		return strings.TrimSpace(strings.TrimPrefix(value, "refs/heads/"))
	}
	if strings.HasPrefix(value, "git:") {
		if at := strings.LastIndex(value, "@"); at >= 0 && at+1 < len(value) {
			return strings.TrimSpace(value[at+1:])
		}
		return strings.TrimSpace(strings.TrimPrefix(value, "git:"))
	}
	if strings.HasPrefix(value, "base:") || strings.HasPrefix(value, "candidate:") {
		if _, suffix, ok := strings.Cut(value, ":"); ok {
			return strings.TrimSpace(suffix)
		}
	}
	return value
}

func appPromotionBuildEnv(candidateDir string) ([]string, string, error) {
	candidateDir = filepath.Clean(candidateDir)
	if strings.TrimSpace(candidateDir) == "" || candidateDir == "." {
		return nil, "", fmt.Errorf("recipient build: candidate workspace is required")
	}
	scratchRoot := filepath.Join(candidateDir, ".choir-promotion-scratch")
	cacheRoot := filepath.Join(filepath.Dir(candidateDir), ".choir-promotion-cache")
	dirs := map[string]string{
		"TMPDIR":           filepath.Join(scratchRoot, "tmp"),
		"GOTMPDIR":         filepath.Join(scratchRoot, "go-tmp"),
		"GOCACHE":          filepath.Join(scratchRoot, "go-build-cache"),
		"GOMODCACHE":       filepath.Join(cacheRoot, "go-mod-cache"),
		"NPM_CONFIG_CACHE": filepath.Join(cacheRoot, "npm-cache"),
		"XDG_CACHE_HOME":   filepath.Join(scratchRoot, "xdg-cache"),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, "", fmt.Errorf("recipient build: prepare scratch dir %q: %w", dir, err)
		}
	}
	env := os.Environ()
	for key, value := range dirs {
		env = setEnvValue(env, key, value)
	}
	return env, scratchRoot, nil
}

func runAppPromotionShellCommand(ctx context.Context, dir string, env []string, name, command string) (appPromotionCommandReport, error) {
	return runAppPromotionCommand(ctx, dir, env, name, "/bin/sh", "-c", command)
}

func runAppPromotionCommand(ctx context.Context, dir string, env []string, name, command string, args ...string) (appPromotionCommandReport, error) {
	started := time.Now()
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Dir = dir
	if len(env) > 0 {
		cmd.Env = env
	}
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	report := appPromotionCommandReport{
		Name:     name,
		Command:  strings.TrimSpace(command + " " + strings.Join(args, " ")),
		ExitCode: 0,
		Output:   truncateAppPromotionOutput(out.String()),
		Duration: time.Since(started).String(),
	}
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			report.ExitCode = exitErr.ExitCode()
		} else {
			report.ExitCode = -1
		}
		return report, err
	}
	return report, nil
}

func appPromotionGitOutput(ctx context.Context, repo string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = repo
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %s: %w", strings.Join(args, " "), strings.TrimSpace(out.String()), err)
	}
	return strings.TrimSpace(out.String()), nil
}

func hashAppPromotionPath(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	h := sha256.New()
	if !info.IsDir() {
		if err := hashAppPromotionFile(h, path, filepath.Base(path)); err != nil {
			return "", err
		}
		return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
	}
	var files []string
	if err := filepath.WalkDir(path, func(current string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		files = append(files, current)
		return nil
	}); err != nil {
		return "", err
	}
	sort.Strings(files)
	for _, file := range files {
		rel, err := filepath.Rel(path, file)
		if err != nil {
			return "", err
		}
		if err := hashAppPromotionFile(h, file, filepath.ToSlash(rel)); err != nil {
			return "", err
		}
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
}

func hashAppPromotionFile(h io.Writer, path, label string) error {
	if _, err := io.WriteString(h, "file\x00"+label+"\x00"); err != nil {
		return err
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()
	_, err = io.Copy(h, file)
	return err
}

func truncateAppPromotionOutput(value string) string {
	const limit = 12000
	if len(value) <= limit {
		return value
	}
	const edge = (limit - len("\n...[truncated middle]...\n")) / 2
	return value[:edge] + "\n...[truncated middle]...\n" + value[len(value)-edge:]
}

func safeAppPromotionChildPath(root, child string) (string, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return "", fmt.Errorf("recipient build: workspace root is required")
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(absRoot, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(absRoot, safeRefPart(child))
	rel, err := filepath.Rel(absRoot, path)
	if err != nil {
		return "", err
	}
	if rel == "." || strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
		return "", fmt.Errorf("recipient build: unsafe workspace child")
	}
	return path, nil
}
