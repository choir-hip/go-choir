package vmmanager

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// Shared read-only artifacts for jailed VMs (RES-05,
// docs/problems/jailer-copies-store-image-per-vm-2026-10-21.md).
//
// The Linux page cache is per inode. A per-VM reflink or copy of the store
// image gives every jailed VM its own inode, so N jailed VMs can hold N host
// copies of the hot store pages where direct VMMs share one. The source
// cannot be hard-linked into a jail: it lives in /nix/store, another mount
// (EXDEV), and must never be chowned or exposed by name.
//
// So vmctl keeps one copy per source identity under <jail base>/.shared
// (inside the vm-state mount, outside every chroot), root-owned 0444, and
// hard-links that one inode into each jail. The VMM uid owns its jail
// directory and can unlink its name, but cannot write or chmod the inode.
//
// Identity is the source's device, inode, size and mtime: the same facts
// snapshotBoundPaths binds into a snapshot, so a restore whose source is
// unchanged links the copy its memory image was taken against, and a deploy
// that replaces the source gives new launches a new inode while running
// jails keep theirs. A copy no jail links (link count 1) is swept.

const jailSharedDirName = ".shared"

func (m *Manager) jailSharedDir() string {
	return filepath.Join(m.jailChrootBase(), jailSharedDirName)
}

// sharedArtifactName names the shared copy of src by its identity.
func sharedArtifactName(src string, info os.FileInfo) string {
	st, _ := info.Sys().(*syscall.Stat_t)
	var dev, ino uint64
	if st != nil {
		dev, ino = uint64(st.Dev), uint64(st.Ino)
	}
	key := fmt.Sprintf("%s\x00%d\x00%d\x00%d\x00%d", src, dev, ino, info.Size(), info.ModTime().UnixNano())
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:16]) + "-" + sanitizeSharedBase(filepath.Base(src))
}

func sanitizeSharedBase(name string) string {
	var b strings.Builder
	for _, r := range name {
		if r == '-' || r == '_' || r == '.' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
		if b.Len() >= 48 {
			break
		}
	}
	return b.String()
}

// usableSharedCopy reports whether path is vmctl's own read-only copy of a
// source of the given size: a regular file (not a symlink), owned by vmctl,
// mode 0444.
func usableSharedCopy(path string, size int64) bool {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0o444 || info.Size() != size {
		return false
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	return ok && int(st.Uid) == os.Geteuid()
}

// linkSharedReadOnly places a read-only artifact at dst as a hard link of the
// shared copy of src, building the copy (reflink, else byte copy) once per
// source identity. It returns the receipt note: "shared" when the inode is
// shared, else what placeReadOnlyCopy did. A shared copy that cannot be built
// or linked falls back to a private copy (today's behavior), logged.
func (m *Manager) linkSharedReadOnly(src, dst string) (string, error) {
	info, err := os.Stat(src)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("%s is not a regular file", src)
	}
	m.jailSharedMu.Lock()
	defer m.jailSharedMu.Unlock()
	dir := m.jailSharedDir()
	if err := ensurePrivateDir(dir); err != nil {
		return m.privateReadOnlyCopy(src, dst, fmt.Errorf("shared dir: %w", err))
	}
	shared := filepath.Join(dir, sharedArtifactName(src, info))
	built := ""
	if !usableSharedCopy(shared, info.Size()) {
		if _, err := os.Lstat(shared); err == nil {
			log.Printf("vmmanager: jail: shared copy %s is not vmctl's read-only copy; rebuilding it", shared)
			if err := os.Remove(shared); err != nil {
				return m.privateReadOnlyCopy(src, dst, fmt.Errorf("remove unusable shared copy: %w", err))
			}
		}
		tmp := shared + ".tmp"
		_ = os.Remove(tmp)
		method, err := placeReadOnlyCopy(src, tmp)
		if err != nil {
			_ = os.Remove(tmp)
			return m.privateReadOnlyCopy(src, dst, fmt.Errorf("build shared copy: %w", err))
		}
		if err := syncRegularFile(tmp); err != nil {
			_ = os.Remove(tmp)
			return m.privateReadOnlyCopy(src, dst, fmt.Errorf("sync shared copy: %w", err))
		}
		if err := os.Rename(tmp, shared); err != nil {
			_ = os.Remove(tmp)
			return m.privateReadOnlyCopy(src, dst, fmt.Errorf("commit shared copy: %w", err))
		}
		built = "(" + method + ")"
		log.Printf("vmmanager: jail: built shared copy of %s by %s at %s", src, method, shared)
	}
	if err := os.Link(shared, dst); err != nil {
		return m.privateReadOnlyCopy(src, dst, fmt.Errorf("link shared copy: %w", err))
	}
	m.sweepSharedLocked(shared)
	return "shared" + built, nil
}

func (m *Manager) privateReadOnlyCopy(src, dst string, why error) (string, error) {
	log.Printf("vmmanager: jail: %s not shared (%v); placing a private copy", src, why)
	return placeReadOnlyCopy(src, dst)
}

// sweepSharedLocked removes shared copies no jail links, and leftover temp
// files, except keep. Callers hold jailSharedMu, so no launch is between
// building a copy and linking it.
func (m *Manager) sweepSharedLocked(keep string) {
	dir := m.jailSharedDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		path := filepath.Join(dir, e.Name())
		if path == keep {
			continue
		}
		info, err := os.Lstat(path)
		if err != nil {
			continue
		}
		st, ok := info.Sys().(*syscall.Stat_t)
		if info.Mode().IsRegular() && ok && st.Nlink > 1 && !strings.HasSuffix(e.Name(), ".tmp") {
			continue
		}
		if err := os.Remove(path); err != nil {
			log.Printf("vmmanager: jail: sweep shared copy %s: %v", path, err)
		}
	}
}
