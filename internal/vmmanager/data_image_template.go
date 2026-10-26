package vmmanager

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/yusefmosiah/go-choir/internal/buildinfo"
)

// Data image template (docs/problems/new-computer-creation-14s-2026-10-26.md,
// step 1). A new computer's data image is the same for every account: an
// empty ext4 holding files/Source/platform, a clean git checkout of the
// deployed source. Building it (git clone + mkfs.ext4 -d of a 32 GiB file)
// took about 4 s under the global manager lock. With the switch on, the
// image is built once per seed identity into a read-only template beside
// the VM state, outside the lock, and each new computer gets a FICLONE of
// it (a metadata operation on btrfs). Anything that fails falls back to
// today's build, and says why.
//
// Switch (off by default; rollback is unsetting it):
//
//	VMCTL_DATA_IMAGE_TEMPLATE=on       reflink new computers' data images
//	VMCTL_DATA_IMAGE_TEMPLATE_DIR      default <VM_STATE_DIR>/.data-image-templates
//
// Only first creation uses it (VMConfig.NewComputer). Boots of existing
// computers, a recovery's staged successor image, and a lost data.img keep
// today's path.
//
// The template holds no identity. Today's image holds no per-computer
// material either: the gateway token goes to the host persist dir and the
// credential disk, the envelope and grants to the credential disk, and the
// guest creates its signer state, credentials and store on first boot.
// The build refuses a seed tree that holds anything but the clean checkout,
// a per-computer file name anywhere in it, or an image root mkfs did not
// get from the seed. What every clone shares with the template is the ext4
// filesystem UUID and directory hash seed (random per mkfs today).

const (
	dataImageTemplateDirName = ".data-image-templates"
	dataImageTemplatePrefix  = "data-"
	dataImageTemplateSuffix  = ".img"
	dataImageTemplateTmp     = ".building-"
	dataImageTemplateFormat  = "choir-data-image-template/v1"
	// defaultDataImageTemplateWait bounds how long a new computer waits for
	// a template build in flight before it builds in place.
	defaultDataImageTemplateWait = 30 * time.Second
)

// DataImageTemplateConfig is the template switch (VMCTL_DATA_IMAGE_TEMPLATE).
type DataImageTemplateConfig struct {
	Enabled bool
	// Dir holds the templates; it must be on the state directory's
	// filesystem for FICLONE. Empty means <StateDir>/.data-image-templates.
	Dir string
}

// perComputerNames are the names of per-computer material on the host
// and in the guest's /mnt/persistent. None may appear in a template.
var perComputerNames = map[string]bool{
	"gateway-token":           true,
	"computer-event-envelope": true,
	"privacy-key":             true,
	"choir-credentials":       true,
	"choir-signers":           true,
	"credential.img":          true,
}

var errDataImageTemplateWait = errors.New("wait_timeout")

// dataImageTemplatePick is what a boot learned, outside the lock, about the
// template for its new data image. Path is empty when there is none; Reason
// then says why.
type dataImageTemplatePick struct {
	Path   string
	ID     string
	Reason string
}

type dataImageTemplateFlight struct {
	done chan struct{}
	path string
	err  error
}

func (m *Manager) dataImageTemplateDir() string {
	if dir := strings.TrimSpace(m.cfg.DataImageTemplate.Dir); dir != "" {
		return filepath.Clean(dir)
	}
	return filepath.Join(m.cfg.StateDir, dataImageTemplateDirName)
}

func (m *Manager) dataImageMkfsBin() string {
	mkfsBin := strings.TrimSpace(m.cfg.MkfsExt4Path)
	if mkfsBin == "" || mkfsBin == "mkfs.ext4" {
		mkfsBin = findBinary("mkfs.ext4", "/run/current-system/sw/bin/mkfs.ext4")
	}
	return mkfsBin
}

// sourceSeedRepo is the seed repository, or "" when seeding is off (the
// same test sourceSeedStagingRoot applies).
func (m *Manager) sourceSeedRepo() string {
	repo := strings.TrimSpace(m.cfg.SourceSeedRepoPath)
	if repo == "" {
		return ""
	}
	info, err := os.Stat(filepath.Join(repo, ".git"))
	if err != nil || !info.IsDir() {
		return ""
	}
	return repo
}

// seedCheckoutCommit reports whether the seed is checked out at the build
// commit (otherwise the clone's default HEAD).
func seedCheckoutCommit(commit string) bool {
	return commit != "" && commit != "local" && commit != "dirty" && len(commit) >= 7
}

func resolvedPath(bin string) string {
	if p, err := exec.LookPath(bin); err == nil {
		bin = p
	}
	if p, err := filepath.EvalSymlinks(bin); err == nil {
		return p
	}
	return bin
}

func seedGit(repo string, args ...string) (string, error) {
	full := append([]string{"-c", "safe.directory=" + repo, "-C", repo}, args...)
	out, err := exec.Command("git", full...).Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return string(out), nil
}

// dataImageSeedIdentity is the content identity of a new data image: every
// input that decides its bytes, short of mkfs's own randomness. It names
// the template, so a template built for another seed is never found.
func (m *Manager) dataImageSeedIdentity(sizeMB int) (string, error) {
	h := sha256.New()
	fmt.Fprintf(h, "%s\nsize_mb=%d\nmkfs=%s\n", dataImageTemplateFormat, sizeMB, resolvedPath(m.dataImageMkfsBin()))
	repo := m.sourceSeedRepo()
	if repo == "" {
		fmt.Fprintf(h, "seed=none\n")
		return hex.EncodeToString(h.Sum(nil)), nil
	}
	commit := strings.TrimSpace(buildinfo.Commit)
	rev := "HEAD^{commit}"
	if seedCheckoutCommit(commit) {
		rev = commit + "^{commit}"
	}
	checkout, err := seedGit(repo, "rev-parse", "--verify", "--quiet", rev)
	if err != nil {
		return "", fmt.Errorf("resolve seed checkout %s: %w", rev, err)
	}
	headRef, _ := seedGit(repo, "symbolic-ref", "-q", "HEAD")
	refs, err := seedGit(repo, "for-each-ref", "--format=%(objectname) %(refname)")
	if err != nil {
		return "", err
	}
	fmt.Fprintf(h, "git=%s\nrepo=%s\nbuild_commit=%s\ncheckout=%s\nhead_ref=%s\nrefs:\n%s",
		resolvedPath("git"), repo, commit, strings.TrimSpace(checkout), strings.TrimSpace(headRef), refs)
	return hex.EncodeToString(h.Sum(nil)), nil
}

func dataImageTemplatePath(dir, id string) string {
	return filepath.Join(dir, dataImageTemplatePrefix+id+dataImageTemplateSuffix)
}

// validDataImageTemplate checks a template file without opening it: a
// regular read-only file (never a symlink) of the image's size.
func validDataImageTemplate(path string, sizeMB int) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("template %s is not a regular file", path)
	}
	if info.Mode().Perm() != 0o400 {
		return fmt.Errorf("template %s mode %o, want 400", path, info.Mode().Perm())
	}
	if want := int64(sizeMB) * 1024 * 1024; info.Size() != want {
		return fmt.Errorf("template %s size %d, want %d", path, info.Size(), want)
	}
	return nil
}

// dataImageTemplateForBoot runs before bootVM takes the global lock. It
// returns the template a new computer's data image should be cloned from,
// building it first when it is missing; nothing here holds m.mu.
func (m *Manager) dataImageTemplateForBoot(cfg VMConfig, sizeMB int) dataImageTemplatePick {
	if !m.cfg.DataImageTemplate.Enabled {
		return dataImageTemplatePick{Reason: "switch_off"}
	}
	if !cfg.NewComputer {
		return dataImageTemplatePick{Reason: "not_new_computer"}
	}
	if strings.TrimSpace(cfg.StoreDiskPath) == "" && strings.TrimSpace(m.cfg.StoreDiskPath) == "" {
		return dataImageTemplatePick{Reason: "no_store_disk"}
	}
	if cfg.VMID != "" {
		if _, err := os.Lstat(filepath.Join(m.cfg.StateDir, cfg.VMID, "data.img")); err == nil {
			return dataImageTemplatePick{Reason: "image_exists"}
		}
	}
	path, id, err := m.ensureDataImageTemplate(sizeMB)
	switch {
	case err == nil:
		return dataImageTemplatePick{Path: path, ID: id}
	case errors.Is(err, errDataImageTemplateWait):
		return dataImageTemplatePick{ID: id, Reason: "wait_timeout"}
	case id == "":
		return dataImageTemplatePick{Reason: "seed_identity: " + err.Error()}
	default:
		return dataImageTemplatePick{ID: id, Reason: "build_failed: " + err.Error()}
	}
}

// ensureDataImageTemplate returns the template for the current seed,
// building it when missing. Concurrent callers share one build; a caller
// waits at most templateWait, after which the build continues for the
// next creation.
func (m *Manager) ensureDataImageTemplate(sizeMB int) (string, string, error) {
	id, err := m.dataImageSeedIdentity(sizeMB)
	if err != nil {
		return "", "", err
	}
	final := dataImageTemplatePath(m.dataImageTemplateDir(), id)
	if validDataImageTemplate(final, sizeMB) == nil {
		return final, id, nil
	}
	key := fmt.Sprintf("%s/%d", id, sizeMB)
	m.templateMu.Lock()
	if m.templateFlights == nil {
		m.templateFlights = map[string]*dataImageTemplateFlight{}
	}
	flight := m.templateFlights[key]
	if flight == nil {
		flight = &dataImageTemplateFlight{done: make(chan struct{})}
		m.templateFlights[key] = flight
		go func() {
			path, err := m.buildDataImageTemplate(id, sizeMB)
			if err != nil {
				log.Printf("vmmanager: data image template %s not built: %v", shortTemplateID(id), err)
			}
			m.templateMu.Lock()
			flight.path, flight.err = path, err
			delete(m.templateFlights, key)
			m.templateMu.Unlock()
			close(flight.done)
		}()
	}
	m.templateMu.Unlock()
	wait := m.templateWait
	if wait <= 0 {
		wait = defaultDataImageTemplateWait
	}
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-flight.done:
		return flight.path, id, flight.err
	case <-timer.C:
		return "", id, errDataImageTemplateWait
	}
}

// buildDataImageTemplate builds the template for id: today's seed and
// mkfs into a temporary file, checked, then renamed into place. A crash
// leaves only a temporary file, which the next build sweeps.
func (m *Manager) buildDataImageTemplate(id string, sizeMB int) (path string, err error) {
	m.templateBuildMu.Lock()
	defer m.templateBuildMu.Unlock()
	dir := m.dataImageTemplateDir()
	final := dataImageTemplatePath(dir, id)
	if validDataImageTemplate(final, sizeMB) == nil {
		return final, nil // a build that finished while this one queued
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("template dir: %w", err)
	}
	sweepDataImageTemplateTemporaries(dir)
	if _, statErr := os.Lstat(final); statErr == nil {
		// A final name that failed validation (torn, re-moded, replaced).
		if err := os.Remove(final); err != nil {
			return "", fmt.Errorf("remove invalid template: %w", err)
		}
	}
	started := time.Now()
	staging, cleanup, err := m.sourceSeedStagingRoot()
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		return "", fmt.Errorf("seed: %w", err)
	}
	if hook := m.templateStagingHook; hook != nil {
		if err := hook(staging); err != nil {
			return "", fmt.Errorf("staging hook: %w", err)
		}
	}
	seeded := strings.TrimSpace(staging) != ""
	if seeded {
		if err := verifyTemplateSeedStaging(staging); err != nil {
			return "", fmt.Errorf("unsafe: %w", err)
		}
	}
	tmp := filepath.Join(dir, dataImageTemplateTmp+id+"-"+randomSuffix()+dataImageTemplateSuffix)
	mkfsBin := m.dataImageMkfsBin()
	if err := createSparseDataImageSeeded(tmp, sizeMB, mkfsBin, staging); err != nil {
		return "", err // createSparseDataImageSeeded removes tmp on failure
	}
	keep := false
	defer func() {
		if !keep {
			_ = os.Remove(tmp)
		}
	}()
	if err := verifyTemplateImageRoot(tmp, mkfsBin, seeded); err != nil {
		return "", fmt.Errorf("unsafe: %w", err)
	}
	// The seed must not have moved under the build: the content would not
	// be what the name says.
	after, err := m.dataImageSeedIdentity(sizeMB)
	if err != nil {
		return "", fmt.Errorf("seed identity after build: %w", err)
	}
	if after != id {
		return "", fmt.Errorf("seed moved during the build (%s -> %s)", shortTemplateID(id), shortTemplateID(after))
	}
	if err := os.Chmod(tmp, 0o400); err != nil {
		return "", err
	}
	if err := os.Rename(tmp, final); err != nil {
		return "", err
	}
	keep = true
	if err := syncDirectory(dir); err != nil {
		return "", fmt.Errorf("sync template dir: %w", err)
	}
	removed := pruneDataImageTemplates(dir, final)
	log.Printf("vmmanager: data image template %s built in %dms (seeded=%t, pruned %d older)", shortTemplateID(id), time.Since(started).Milliseconds(), seeded, removed)
	return final, nil
}

// verifyTemplateSeedStaging admits only the clean public checkout:
// files/Source/platform and nothing beside it, no untracked or ignored
// file in it, and no per-computer file name anywhere in its tree.
func verifyTemplateSeedStaging(staging string) error {
	for _, step := range [][2]string{{"", "files"}, {"files", "Source"}, {"files/Source", "platform"}} {
		entries, err := os.ReadDir(filepath.Join(staging, step[0]))
		if err != nil {
			return err
		}
		if len(entries) != 1 || entries[0].Name() != step[1] || !entries[0].IsDir() {
			names := make([]string, 0, len(entries))
			for _, e := range entries {
				names = append(names, e.Name())
			}
			return fmt.Errorf("seed root /%s holds %v, want only %s", step[0], names, step[1])
		}
	}
	platform := filepath.Join(staging, "files", "Source", "platform")
	status, err := seedGit(platform, "status", "--porcelain=v1", "--ignored", "--untracked-files=all")
	if err != nil {
		return fmt.Errorf("seed status: %w", err)
	}
	if s := strings.TrimSpace(status); s != "" {
		return fmt.Errorf("seed checkout is not clean: %s", firstLine(s))
	}
	return filepath.WalkDir(staging, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && d.Name() == ".git" {
			return filepath.SkipDir
		}
		if perComputerNames[d.Name()] {
			rel, _ := filepath.Rel(staging, path)
			return fmt.Errorf("per-computer name %s in the seed", rel)
		}
		return nil
	})
}

// verifyTemplateImageRoot reads the built image's root directory (debugfs,
// beside mkfs.ext4 in e2fsprogs): only lost+found and, when seeded, files.
func verifyTemplateImageRoot(image, mkfsBin string, seeded bool) error {
	debugfs := filepath.Join(filepath.Dir(mkfsBin), "debugfs")
	if _, err := os.Stat(debugfs); err != nil {
		debugfs = findBinary("debugfs", "/run/current-system/sw/bin/debugfs")
	}
	out, err := exec.Command(debugfs, "-R", "ls -p /", image).Output()
	if err != nil {
		return fmt.Errorf("read template root: %w", err)
	}
	allowed := map[string]bool{".": true, "..": true, "lost+found": true}
	if seeded {
		allowed["files"] = true
	}
	seen := map[string]bool{}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// /inode/mode/uid/gid/name/[size/]
		parts := strings.Split(line, "/")
		if len(parts) < 7 || parts[0] != "" {
			return fmt.Errorf("unreadable template root entry %q", line)
		}
		name := parts[5]
		if !allowed[name] {
			return fmt.Errorf("template root holds %q", name)
		}
		seen[name] = true
	}
	if !seen["lost+found"] || (seeded && !seen["files"]) {
		return fmt.Errorf("template root lacks the seed (saw %v)", sortedKeys(seen))
	}
	return nil
}

// prepareNewDataImage creates a new computer's data image under the global
// lock: a clone of the picked template, or today's build when there is
// none or the clone fails. It returns what it did, for the boot receipt.
func (m *Manager) prepareNewDataImage(dataImg string, sizeMB int, pick dataImageTemplatePick) (string, error) {
	reason := pick.Reason
	if pick.Path != "" {
		started := time.Now()
		err := m.cloneDataImageTemplate(pick, dataImg, sizeMB)
		if err == nil {
			return fmt.Sprintf("template id=%s clone_ms=%d", shortTemplateID(pick.ID), time.Since(started).Milliseconds()), nil
		}
		reason = err.Error()
	}
	log.Printf("vmmanager: new data image %s built in place: %s", dataImg, reason)
	if err := m.createDataImage(dataImg, sizeMB); err != nil {
		return "", err
	}
	return "built reason=" + reason, nil
}

func (m *Manager) cloneDataImageTemplate(pick dataImageTemplatePick, dataImg string, sizeMB int) (err error) {
	if filepath.Base(pick.Path) != dataImageTemplatePrefix+pick.ID+dataImageTemplateSuffix {
		return fmt.Errorf("template_mismatch: %s is not template %s", pick.Path, shortTemplateID(pick.ID))
	}
	if err := validDataImageTemplate(pick.Path, sizeMB); err != nil {
		return fmt.Errorf("template_gone: %v", err)
	}
	src, err := os.OpenFile(pick.Path, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return fmt.Errorf("template_gone: %v", err)
	}
	defer src.Close()
	srcInfo, err := src.Stat()
	if err != nil || !srcInfo.Mode().IsRegular() || srcInfo.Size() != int64(sizeMB)*1024*1024 {
		return fmt.Errorf("template_gone: opened template is not the checked file")
	}
	dst, err := os.OpenFile(dataImg, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("create data image: %v", err)
	}
	defer func() {
		closeErr := dst.Close()
		if err == nil && closeErr != nil {
			err = fmt.Errorf("close data image: %v", closeErr)
		}
		if err != nil {
			_ = os.Remove(dataImg)
		}
	}()
	clone := m.templateCloneFn
	if clone == nil {
		clone = cloneFile
	}
	if err := clone(dst, src); err != nil {
		return fmt.Errorf("reflink: %v", err)
	}
	dstInfo, err := dst.Stat()
	if err != nil || dstInfo.Size() != srcInfo.Size() {
		return fmt.Errorf("reflink: clone size does not match the template")
	}
	if err := dst.Sync(); err != nil {
		return fmt.Errorf("sync data image: %v", err)
	}
	return syncDirectory(filepath.Dir(dataImg))
}

// warmDataImageTemplate builds the current template at start, so the first
// creation after a deploy finds it ready. It holds no owner work.
func (m *Manager) warmDataImageTemplate() {
	if _, id, err := m.ensureDataImageTemplate(dataImageSizeMB); err != nil {
		log.Printf("vmmanager: data image template %s not warmed: %v", shortTemplateID(id), err)
		return
	}
}

func sweepDataImageTemplateTemporaries(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), dataImageTemplateTmp) {
			if err := os.Remove(filepath.Join(dir, e.Name())); err == nil {
				log.Printf("vmmanager: removed interrupted data image template build %s", e.Name())
			}
		}
	}
}

// pruneDataImageTemplates removes every template but keep. Clones share
// extents, so removal frees only what no computer's image still uses.
func pruneDataImageTemplates(dir, keep string) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	removed := 0
	for _, e := range entries {
		name := e.Name()
		path := filepath.Join(dir, name)
		if path == keep || !strings.HasPrefix(name, dataImageTemplatePrefix) || !strings.HasSuffix(name, dataImageTemplateSuffix) {
			continue
		}
		if err := os.Remove(path); err == nil {
			removed++
		}
	}
	return removed
}

func shortTemplateID(id string) string {
	if len(id) > 12 {
		return id[:12]
	}
	return id
}

func randomSuffix() string {
	var b [6]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b[:])
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
