package capsule

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	"golang.org/x/sys/unix"
)

// overlayOpaqueXattr marks a directory in the overlay upper as opaque, so the
// lower layers' entries under it are hidden. It is the userxattr form that
// MountOverlayFS mounts with (the trusted.* form needs real root).
const overlayOpaqueXattr = "user.overlay.opaque"

// prepareCapsuleReleaseDir gives the capsule an empty release directory of its
// own: <upperDir>/var/lib/artifact/release, owned by uid:gid (the capsule's
// mapped host id, so it is root inside), mode 0755, marked opaque. The
// capsule overlay's lowest layer is the guest's own root, so on a computer
// that has applied a release the guest's release tree would otherwise show
// through owned by an unmapped uid, and a build's `cp` over it fails with
// EPERM (docs/problems/engineering-completion-path-cluster-2026-10-20.md,
// Failure 21). Call it before MountOverlayFS.
//
// Only the leaf is capsule-owned and opaque; missing parents are created
// root-owned 0755, so the capsule gains no write access to the rest of
// /var/lib/artifact. It is idempotent and never removes content. Every
// component is opened O_NOFOLLOW relative to its parent's descriptor, so a
// symlink (or a non-directory) at any component is refused rather than
// followed out of the upper layer.
func prepareCapsuleReleaseDir(upperDir string, uid, gid int) error {
	if err := os.MkdirAll(upperDir, 0o755); err != nil {
		return fmt.Errorf("capsule release dir: upper %s: %w", upperDir, err)
	}
	dirFD, err := unix.Open(upperDir, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return fmt.Errorf("capsule release dir: open upper %s: %w", upperDir, err)
	}
	parts := strings.Split(strings.TrimSuffix(ReleaseTreePrefix, "/"), "/")
	for i, name := range parts {
		next, err := openOrCreateDirAt(dirFD, name)
		_ = unix.Close(dirFD)
		if err != nil {
			return fmt.Errorf("capsule release dir: %s: %w", strings.Join(parts[:i+1], "/"), err)
		}
		dirFD = next
	}
	defer unix.Close(dirFD)
	if err := unix.Fchown(dirFD, uid, gid); err != nil {
		return fmt.Errorf("capsule release dir: chown: %w", err)
	}
	if err := unix.Fchmod(dirFD, 0o755); err != nil {
		return fmt.Errorf("capsule release dir: chmod: %w", err)
	}
	if err := unix.Fsetxattr(dirFD, overlayOpaqueXattr, []byte("y"), 0); err != nil {
		// A state volume without user xattrs cannot carry the marker. The
		// capsule then sees the lower release tree as before this fix, so
		// spawn keeps working; every other failure stays closed.
		if errors.Is(err, unix.ENOTSUP) || errors.Is(err, unix.EOPNOTSUPP) {
			log.Printf("capsule release dir: opaque marker unsupported (%v); the lower release tree may show through", err)
			return nil
		}
		return fmt.Errorf("capsule release dir: mark opaque: %w", err)
	}
	return nil
}

// openOrCreateDirAt opens the plain directory name under parentFD, creating
// it (root-owned, 0755) when absent. A symlink or non-directory fails the
// O_NOFOLLOW|O_DIRECTORY open and is returned as an error.
func openOrCreateDirAt(parentFD int, name string) (int, error) {
	const flags = unix.O_RDONLY | unix.O_DIRECTORY | unix.O_NOFOLLOW | unix.O_CLOEXEC
	fd, err := unix.Openat(parentFD, name, flags, 0)
	if errors.Is(err, unix.ENOENT) {
		if err := unix.Mkdirat(parentFD, name, 0o755); err != nil && !errors.Is(err, unix.EEXIST) {
			return -1, err
		}
		fd, err = unix.Openat(parentFD, name, flags, 0)
		if err == nil {
			if chmodErr := unix.Fchmod(fd, 0o755); chmodErr != nil { // mkdir honors the umask
				_ = unix.Close(fd)
				return -1, chmodErr
			}
		}
	}
	if err != nil {
		return -1, fmt.Errorf("not a plain directory: %w", err)
	}
	return fd, nil
}
