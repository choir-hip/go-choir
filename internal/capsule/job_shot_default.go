//go:build linux

package capsule

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

// A choir-shot run with its build cached needs about 1.5 GiB; the first run
// in a capsule builds with vite and needs about 3 GiB. Engineers on three
// arms never passed memory_mib, so every shot job claimed the full build
// peak and parked 2 to 14 minutes for host memory (live trace 2026-11-03).
// StartJob therefore defaults the claim when the job is a choir-shot run
// and the build it would reuse is already on disk.
const (
	shotDefaultMemoryMiB = 1536
	shotDefaultNote      = "memory_mib 1536 (defaulted for a cached choir-shot)"
	memoryMiBHint        = "if this job needs less, restart it with memory_mib (choir-shot with a cached build: 1536)"
)

var (
	// The command must START with plain `node <script>`: no env prefix, no
	// wrapper (env -C, find -execdir), no node flag (--import, -r and --eval
	// run other code), and the script token ends at a space or quote.
	shotNodeRe  = regexp.MustCompile(`^\s*node\s+['"]?([A-Za-z0-9_./-]*choir-shot\.mjs)(?:['"]|\s|$)`)
	shotHeavyRe = regexp.MustCompile(`\b(npm|npx|yarn|pnpm|vite|make)\b|\bgo\s+(build|test|run|install)\b`)
	shotSkipDir = map[string]bool{"node_modules": true, "dist": true, "tests": true, ".vite": true}
)

// shotScript returns the absolute in-capsule path of scripts/choir-shot.mjs
// when argv runs it as `node ... choir-shot.mjs`, directly or as the one
// command of `sh -c`. A chained command (cd, git, a build) is not a shot run and
// keeps the full claim.
//
// Detection reads argv, not a marker choir-shot would export: the job's
// memory is claimed before the process exists, so a marker written by the
// run itself cannot reach the start decision, and the cached-build check
// needs the script's own location anyway (its frontend is two levels up).
func shotScript(argv []string, cwd string) string {
	if len(argv) == 0 {
		return ""
	}
	var cmd string
	switch path.Base(argv[0]) {
	case "node":
		if path.Base(argv[0]) != argv[0] {
			return "" // only plain node: the pattern below starts at it
		}
		cmd = strings.Join(argv, " ")
	case "sh", "bash", "dash":
		if len(argv) != 3 || (argv[1] != "-c" && argv[1] != "-lc") {
			return ""
		}
		cmd = argv[2]
	default:
		return ""
	}
	// One simple command only: anything chained (cd, git pull, patch, a
	// build) can change which tree runs or what is cached before the shot.
	if strings.ContainsAny(cmd, ";&\n`") || strings.Contains(cmd, "$(") || strings.Contains(cmd, "||") {
		return ""
	}
	if shotHeavyRe.MatchString(cmd) || strings.Contains(cmd, "CHOIR_PLATFORM_ROOT") {
		return ""
	}
	m := shotNodeRe.FindStringSubmatch(cmd)
	if m == nil {
		return ""
	}
	script := m[1]
	if !path.IsAbs(script) {
		// A relative script means the job's cwd, unless the command moved.
		if cwd == "" || regexp.MustCompile(`(^|[\s;&|(])(cd|pushd)\s`).MatchString(cmd) {
			return ""
		}
		script = path.Join(cwd, script)
	}
	return path.Clean(script)
}

// shotSourceHash ports choir-shot.mjs sourceHash: sha256 over the frontend
// tree's file names (as the capsule sees them, namePrefix) and contents,
// sorted, skipping node_modules, dist, tests and .vite.
func shotSourceHash(fsDir, namePrefix string) (string, error) {
	h := sha256.New()
	var walk func(fsPath, name string) error
	walk = func(fsPath, name string) error {
		entries, err := os.ReadDir(fsPath)
		if err != nil {
			return err
		}
		for _, e := range entries { // ReadDir sorts by file name
			if shotSkipDir[e.Name()] {
				continue
			}
			fp, np := filepath.Join(fsPath, e.Name()), path.Join(name, e.Name())
			if e.IsDir() {
				if err := walk(fp, np); err != nil {
					return err
				}
				continue
			}
			b, err := os.ReadFile(fp)
			if err != nil {
				return err
			}
			h.Write([]byte(np))
			h.Write(b)
		}
		return nil
	}
	if err := walk(fsDir, namePrefix); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil))[:16], nil
}

// shotBuildCached is choir-shot's own "build cached" test: the done marker
// under /tmp/choir-shot/<source hash> in the capsule's /tmp.
func shotBuildCached(caps *Capsule, script string) bool {
	if caps == nil || caps.MergedDir == "" {
		return false
	}
	frontend := path.Join(path.Dir(path.Dir(script)), "frontend")
	sum, err := shotSourceHash(filepath.Join(caps.MergedDir, filepath.FromSlash(frontend)), frontend)
	if err != nil {
		return false
	}
	_, err = os.Stat(filepath.Join(caps.MergedDir, "tmp", "choir-shot", sum, "done"))
	return err == nil
}

// defaultShotMemory is the memory_mib a job with none declared gets, and the
// note that says so; 0 and "" when the job keeps the full claim.
func defaultShotMemory(caps *Capsule, argv []string, cwd string, declared int) (int, string) {
	if declared > 0 {
		return declared, ""
	}
	script := shotScript(argv, cwd)
	if script == "" || !shotBuildCached(caps, script) {
		return 0, ""
	}
	return shotDefaultMemoryMiB, shotDefaultNote
}

// jobWaitReason is the host-memory wait text for a job: the reason, the
// defaulted-claim note, and, while the job waits on the undeclared full claim,
// the hint that a smaller job can say so.
// credit is the job's own capsule's unrealised reservation
// (Executor.capsuleReservationCredit).
func jobWaitReason(job *capsuleJob, facts HostMemoryFacts, fresh bool, credit int64) string {
	reason := HostMemoryWaitReasonCredited(facts, fresh, JobHostNeedBytes(job.spec.MemoryMiB), credit)
	if reason == "" {
		return ""
	}
	if job.memoryNote != "" {
		reason += "; " + job.memoryNote
	}
	if job.spec.MemoryMiB <= 0 {
		reason += "; " + memoryMiBHint
	}
	return reason
}
