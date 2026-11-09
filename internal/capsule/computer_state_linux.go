//go:build linux

package capsule

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sys/unix"

	"github.com/yusefmosiah/go-choir/internal/ownerfiles"
)

// computerLayerOwner owns the layer's directories: the capsule's mapped uid
// in production; tests that run unprivileged use their own.
var computerLayerOwner, computerLayerGroup = capsuleNamespaceHostID, capsuleNamespaceHostID

// SetComputerFilesRoot names the computer's files root, the source of an
// engineering capsule's /computer/files, and the directories under it the
// snapshot leaves out by structure: the capsule's own source (sourceRoot,
// when it is under root) and the owner's mailbox (ownerfiles.MailDir). The
// root's symlinks are resolved here; empty leaves every capsule without it.
func (e *Executor) SetComputerFilesRoot(root, sourceRoot string) {
	e.computerFilesRoot, e.computerFilesLeaveOut = computerFilesLeaveOutFor(root, sourceRoot)
}

// computerFilesLeaveOutFor resolves root and names the directories a files
// snapshot leaves out by structure: the capsule's own source and the mailbox.
// An empty root gives none.
func computerFilesLeaveOutFor(root, sourceRoot string) (string, map[string]string) {
	if strings.TrimSpace(root) == "" {
		return "", nil
	}
	resolved, err := filepath.EvalSymlinks(filepath.Clean(root))
	if err != nil {
		resolved = filepath.Clean(root)
	}
	leaveOut := map[string]string{ownerfiles.MailDir: LeaveOutMail}
	if strings.TrimSpace(sourceRoot) != "" {
		source := filepath.Clean(sourceRoot)
		if r, err := filepath.EvalSymlinks(source); err == nil {
			source = r
		}
		if rel, err := filepath.Rel(resolved, source); err == nil && rel != "." && pathWithin(source, resolved) {
			leaveOut[filepath.ToSlash(rel)] = LeaveOutSource
		}
	}
	return resolved, leaveOut
}

// prepareComputerState decides and builds a spawn's files layer: withheld
// from a capsule holding the owner's raw model credential (a correctness
// refusal, not a switch: no copy beside a credential that reaches outside
// the computer), else built, in claude mode too. It
// returns the receipt and the layer path ("" for none). The store copy
// follows only for a receipt marked provided.
func (e *Executor) prepareComputerState(ctx context.Context, spec SpawnSpec, delegateToken, base string) (*ComputerStateReceipt, string) {
	if reason := computerStateWithheld(spec, delegateToken); reason != "" {
		return withheldComputerState(reason), ""
	}
	layer, receipt := e.buildComputerStateLayer(ctx, base)
	receipt.ComputerState = ComputerStateProvided
	return &receipt, layer
}

// buildComputerStateLayer builds <base>/computer-lower for a spawn that asked
// for the computer's state and returns its path ("" when the capsule starts
// without it) and the receipt. It never fails the spawn: a layer it cannot
// build whole is removed and the receipt names why.
func (e *Executor) buildComputerStateLayer(ctx context.Context, base string) (string, ComputerStateReceipt) {
	receipt := ComputerStateReceipt{}
	layer := filepath.Join(base, computerLowerName)
	files := filepath.Join(layer, "computer", "files")
	refuse := func(reason string) (string, ComputerStateReceipt) {
		_ = removeLinkedTree(layer)
		receipt.Files = false
		receipt.Refused = reason
		receipt.LeftOut, receipt.LeftOutCounts, receipt.LeftOutMore = nil, nil, 0
		receipt.LinkedFiles, receipt.LinkedBytes, receipt.Directories = 0, 0, 0
		return "", receipt
	}
	if e.computerFilesRoot == "" {
		return refuse("no files root configured")
	}
	if info, err := os.Lstat(e.computerFilesRoot); err != nil || !info.IsDir() {
		return refuse("files root is unavailable")
	}
	// The root and the capsule state must be disjoint: a walk into its own
	// destination, or a layer inside the tree it copies, is refused.
	if state := filepath.Clean(e.stateDir); pathWithin(state, e.computerFilesRoot) || pathWithin(e.computerFilesRoot, state) {
		return refuse("files root and capsule state overlap")
	}
	if err := mkdirOwned(layer); err != nil {
		return refuse("create layer: " + err.Error())
	}
	if err := mkdirOwned(filepath.Join(layer, "computer")); err != nil {
		return refuse("create layer: " + err.Error())
	}
	if err := mkdirOwned(files); err != nil {
		return refuse("create layer: " + err.Error())
	}
	var walkErr error
	asked := time.Now()
	_ = ownerfiles.Exclusive(func() error {
		receipt.LockedAt = time.Now().UTC()
		receipt.LockWaitMS = receipt.LockedAt.Sub(asked).Milliseconds()
		walkErr = snapshotFilesTree(ctx, e.computerFilesRoot, files, e.computerFilesLeaveOut, &receipt)
		receipt.LockHeldMS = time.Since(receipt.LockedAt).Milliseconds()
		return nil
	})
	if walkErr != nil {
		return refuse("snapshot: " + walkErr.Error())
	}
	receipt.Files = true
	return layer, receipt
}

// pathWithin reports that child is parent or below it.
func pathWithin(child, parent string) bool {
	rel, err := filepath.Rel(parent, child)
	return err == nil && (rel == "." || (rel != ".." && !strings.HasPrefix(rel, "../")))
}

// computerStoreFreeBytes reads the state volume's free space; tests replace it.
var computerStoreFreeBytes = stateFreeBytes

// chownTreeNoFollow gives every entry of root (root included) to uid:gid,
// never following a link. It runs on the store copy, which the store wrote
// fresh in the capsule's upper.
func chownTreeNoFollow(root string, uid, gid int) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		return os.Lchown(path, uid, gid)
	})
}

// writeComputerStateReceipt writes the receipt read-only at
// /computer/receipt.json (in the upper, beside the store copy) for the
// engineer and the proofs; the Capsule keeps the same record.
func writeComputerStateReceipt(upperDir string, receipt ComputerStateReceipt) error {
	// Through file descriptors, never following a link: a seeded upper is
	// sandbox-written (a resume), so "computer" or "receipt.json" may be a
	// symlink the host must not write through.
	raw, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		return err
	}
	upperFD, err := unix.Open(upperDir, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return err
	}
	defer unix.Close(upperFD)
	dirFD, err := openOrCreateDirAt(upperFD, "computer")
	if err != nil {
		return err
	}
	defer unix.Close(dirFD)
	if err := unix.Fchown(dirFD, computerLayerOwner, computerLayerGroup); err != nil {
		return err
	}
	if err := unix.Unlinkat(dirFD, "receipt.json", 0); err != nil && !errors.Is(err, unix.ENOENT) {
		return err
	}
	fileFD, err := unix.Openat(dirFD, "receipt.json", unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0o444)
	if err != nil {
		return err
	}
	file := os.NewFile(uintptr(fileFD), "receipt.json")
	defer file.Close()
	_, err = file.Write(append(raw, '\n'))
	return err
}

// mkdirOwned creates one directory of the layer: 0755, owned by the
// capsule's mapped uid, so the capsule reads it and its writes there land in
// its upper.
func mkdirOwned(dir string) error {
	if err := os.Mkdir(dir, 0o755); err != nil {
		return err
	}
	if err := os.Chmod(dir, 0o755); err != nil {
		return err
	}
	return os.Lchown(dir, computerLayerOwner, computerLayerGroup)
}

// snapshotFilesTree hardlinks every kept regular file of root into dest. It
// walks by descriptor: each directory is opened O_NOFOLLOW relative to its
// parent, each entry is examined with AT_SYMLINK_NOFOLLOW, and linkat never
// follows a link, so nothing outside root is reached. Files are linked, never
// chmodded or copied; a file without o+r is left out. The caller holds the
// files write lock. Any failure to build a kept entry fails the whole walk:
// a layer is whole or absent.
func snapshotFilesTree(ctx context.Context, root, dest string, leaveOut map[string]string, receipt *ComputerStateReceipt) error {
	return snapshotFilesTreeOwned(ctx, root, dest, leaveOut, receipt, computerLayerOwner, computerLayerGroup, false)
}

// snapshotFilesTreeOwned is snapshotFilesTree with the owner of the
// directories it makes.
func snapshotFilesTreeOwned(ctx context.Context, root, dest string, leaveOut map[string]string, receipt *ComputerStateReceipt, uid, gid int, readOnly bool) error {
	rootFD, err := unix.Open(root, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return fmt.Errorf("open files root: %w", err)
	}
	defer unix.Close(rootFD)
	var rootStat unix.Stat_t
	if err := unix.Fstat(rootFD, &rootStat); err != nil {
		return fmt.Errorf("stat files root: %w", err)
	}
	destFD, err := unix.Open(dest, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return fmt.Errorf("open layer: %w", err)
	}
	defer unix.Close(destFD)
	rootMnt, mntKnown := mountIDAt(rootFD, "")
	w := snapshotWalk{ctx: ctx, dev: rootStat.Dev, mnt: rootMnt, mntKnown: mntKnown, leaveOut: leaveOut, receipt: receipt, uid: uid, gid: gid, readOnly: readOnly}
	return w.dir(rootFD, destFD, "")
}

type snapshotWalk struct {
	ctx      context.Context
	dev      uint64
	mnt      uint64
	mntKnown bool
	leaveOut map[string]string
	receipt  *ComputerStateReceipt
	uid, gid int
	// readOnly leaves out a file anyone but its owner could write (the shot
	// layer links live inodes and only reads them).
	readOnly bool
}

func (w snapshotWalk) dir(srcFD, destFD int, rel string) error {
	if err := w.ctx.Err(); err != nil {
		return err
	}
	dir := os.NewFile(uintptr(dupFD(srcFD)), rel)
	if dir == nil {
		return fmt.Errorf("dup %q", displayRel(rel))
	}
	names, err := dir.Readdirnames(-1)
	_ = dir.Close()
	if err != nil {
		return fmt.Errorf("read %q: %w", displayRel(rel), err)
	}
	for _, name := range names {
		child := path.Join(rel, name)
		var st unix.Stat_t
		if err := unix.Fstatat(srcFD, name, &st, unix.AT_SYMLINK_NOFOLLOW); err != nil {
			if errors.Is(err, unix.ENOENT) {
				continue
			}
			return fmt.Errorf("stat %q: %w", child, err)
		}
		switch st.Mode & unix.S_IFMT {
		case unix.S_IFDIR:
			if reason := w.leaveOut[child]; reason != "" {
				w.receipt.leaveOut(child, reason)
				continue
			}
			if st.Dev != w.dev || w.otherMount(srcFD, name) {
				// A mount inside the files root, another filesystem or a bind
				// mount of the same one (which could show a masked tree): left
				// out.
				w.receipt.leaveOut(child, LeaveOutOtherFilesystem)
				continue
			}
			childFD, err := unix.Openat(srcFD, name, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
			if err != nil {
				return fmt.Errorf("open %q: %w", child, err)
			}
			err = w.subdir(srcFD, childFD, destFD, name, child)
			unix.Close(childFD)
			if err != nil {
				return err
			}
		case unix.S_IFREG:
			if computerFilesTemporary(name) {
				w.receipt.leaveOut(child, LeaveOutTemporary)
				continue
			}
			if st.Mode&0o004 == 0 {
				// Counted, never named: the name of a file the owner kept
				// unreadable is itself not for the capsule.
				w.receipt.countLeaveOut(LeaveOutNotReadable)
				continue
			}
			if w.readOnly && st.Mode&0o022 != 0 {
				w.receipt.countLeaveOut(LeaveOutWritable)
				continue
			}
			if err := w.link(srcFD, destFD, name, child, st); err != nil {
				return err
			}
		case unix.S_IFLNK:
			w.receipt.leaveOut(child, LeaveOutSymlink)
		default:
			w.receipt.leaveOut(child, LeaveOutSpecial)
		}
	}
	return nil
}

// otherMount reports a directory on another mount than the root's (a bind
// mount on the same device included), by statx mount id; a kernel without
// mount ids leaves the device check alone.
func (w snapshotWalk) otherMount(dirFD int, name string) bool {
	if !w.mntKnown {
		return false
	}
	id, ok := mountIDAt(dirFD, name)
	return !ok || id != w.mnt
}

func mountIDAt(dirFD int, name string) (uint64, bool) {
	var stx unix.Statx_t
	flags := unix.AT_SYMLINK_NOFOLLOW
	if name == "" {
		flags |= unix.AT_EMPTY_PATH
	}
	if err := unix.Statx(dirFD, name, flags, unix.STATX_MNT_ID, &stx); err != nil || stx.Mask&unix.STATX_MNT_ID == 0 {
		return 0, false
	}
	return stx.Mnt_id, true
}

func (w snapshotWalk) subdir(srcFD, childFD, destFD int, name, child string) error {
	if err := unix.Mkdirat(destFD, name, 0o755); err != nil {
		return fmt.Errorf("mkdir %q: %w", child, err)
	}
	if err := unix.Fchmodat(destFD, name, 0o755, 0); err != nil {
		return fmt.Errorf("chmod %q: %w", child, err)
	}
	if err := unix.Fchownat(destFD, name, w.uid, w.gid, unix.AT_SYMLINK_NOFOLLOW); err != nil {
		return fmt.Errorf("chown %q: %w", child, err)
	}
	childDest, err := unix.Openat(destFD, name, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return fmt.Errorf("open %q: %w", child, err)
	}
	defer unix.Close(childDest)
	w.receipt.Directories++
	return w.dir(childFD, childDest, child)
}

// link hardlinks one regular file and checks the link is the inode it
// examined: a regular file with o+r. A file that vanished is skipped; any
// other failure fails the walk.
func (w snapshotWalk) link(srcFD, destFD int, name, child string, st unix.Stat_t) error {
	// linkat without AT_SYMLINK_FOLLOW links the entry itself.
	if err := unix.Linkat(srcFD, name, destFD, name, 0); err != nil {
		if errors.Is(err, unix.ENOENT) {
			return nil
		}
		return fmt.Errorf("link %q: %w", child, err)
	}
	var got unix.Stat_t
	if err := unix.Fstatat(destFD, name, &got, unix.AT_SYMLINK_NOFOLLOW); err != nil {
		return fmt.Errorf("check link %q: %w", child, err)
	}
	if got.Dev != st.Dev || got.Ino != st.Ino || got.Mode&unix.S_IFMT != unix.S_IFREG || got.Mode&0o004 == 0 {
		// The entry changed between the stat and the link (a writer outside
		// the files lock): the link is not the file examined.
		if err := unix.Unlinkat(destFD, name, 0); err != nil {
			return fmt.Errorf("unlink a changed entry %q: %w", child, err)
		}
		w.receipt.leaveOut(child, LeaveOutChanged)
		return nil
	}
	w.receipt.LinkedFiles++
	w.receipt.LinkedBytes += st.Size
	return nil
}

func dupFD(fd int) int {
	nfd, err := unix.Dup(fd)
	if err != nil {
		return -1
	}
	unix.CloseOnExec(nfd)
	return nfd
}

func displayRel(rel string) string {
	if rel == "" {
		return "."
	}
	return rel
}

// removeLinkedTree removes a tree of hardlinks without touching a linked
// inode: it never chmods a file (unlinking needs only the directory's
// mode), so the live owner file a link shares keeps its mode and bytes.
func removeLinkedTree(root string) error {
	return removePrivateTree(root)
}
