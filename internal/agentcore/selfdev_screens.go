package agentcore

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/imageblob"
	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// Screens as files (docs/problems/selfdev-screens-are-not-files-2026-11-03.md).
// The engineer takes its own screenshots with Playwright and the store
// Chromium, writes them to capsule.ScreensDir, and Freeze stages them in the
// bundle bound by sha256. At a fresh Freeze the runtime saves each one as a
// file in the computer's file tree (Self-development/<operation>/screens/)
// and in the asset store under the same digest: the Texture desk embeds the
// file, the owner opens it in Files, and the judge's inspect_bundle re-hashes
// the same bytes.

// imageStore is this computer's content-addressed asset store.
func (rt *Runtime) imageStore() *imageblob.Store {
	return imageblob.New(imageblob.DirForStorePath(rt.cfg.StorePath))
}

// filesRoot is the computer's file tree (the owner's Files).
func (rt *Runtime) filesRoot() string {
	return provideriface.ResolveFilesRoot("")
}

// freezeScreensKey carries the count of saved screens from the Freeze to
// settleFreezePrecommits, which removes it before the result reaches the cell.
const freezeScreensKey = "_screens_saved"

// screenAssetStore is the asset store's write side (imageblob.Store).
type screenAssetStore interface {
	Put(data []byte) (ref, mediaType string, err error)
}

var plainOperationID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)

// recordFreezeScreens saves the staged screens and fills the Freeze result.
// frozenRoot is the published bundle directory. A failed save does not fail
// the Freeze, because the bundle is the record: each row says whether it was
// saved, and screen_errors says why not.
func recordFreezeScreens(images screenAssetStore, filesRoot, frozenRoot, operationID string, screens []capsule.ScreenFile, out map[string]any) {
	if len(screens) == 0 {
		return
	}
	dir := yaegikernel.SelfDevelopmentScreensDir(operationID)
	var rows []map[string]any
	var failures []string
	saved := 0
	for _, screen := range screens {
		file := dir + "/" + screen.Name
		err := saveFrozenScreen(images, filesRoot, frozenRoot, operationID, screen)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", screen.Name, err))
		} else {
			saved++
		}
		rows = append(rows, map[string]any{
			"name": screen.Name, "sha256": screen.SHA256, "file": file, "image_ref": "artifact:sha256:" + screen.SHA256,
			"width": screen.Width, "height": screen.Height, "bytes": screen.Bytes, "blank": screen.Blank, "saved": err == nil,
		})
	}
	out["screens"] = rows
	if len(failures) > 0 {
		out["screen_errors"] = failures
	}
	out[freezeScreensKey] = saved
}

// saveFrozenScreen reads one staged screen, checks its bytes are the cited
// digest, puts it in the asset store and writes it into the file tree.
func saveFrozenScreen(images screenAssetStore, filesRoot, frozenRoot, operationID string, screen capsule.ScreenFile) error {
	if !plainOperationID.MatchString(operationID) || strings.Contains(operationID, "..") || !capsule.ValidScreenName(screen.Name) {
		return errors.New("not a plain operation id and screen name")
	}
	data, err := os.ReadFile(filepath.Join(frozenRoot, filepath.FromSlash(capsule.BundleScreensDir), screen.Name))
	if err != nil {
		return fmt.Errorf("staged screen unreadable: %w", err)
	}
	sum := sha256.Sum256(data)
	if hex.EncodeToString(sum[:]) != screen.SHA256 {
		return errors.New("staged bytes are not the bundle's screen")
	}
	if images == nil {
		return errors.New("no asset store")
	}
	if ref, _, err := images.Put(data); err != nil {
		return fmt.Errorf("asset store write: %w", err)
	} else if ref != "artifact:sha256:"+screen.SHA256 {
		return fmt.Errorf("asset store filed it as %s", ref)
	}
	if strings.TrimSpace(filesRoot) == "" {
		return errors.New("no file tree")
	}
	target, err := mkdirBeneath(filesRoot, yaegikernel.SelfDevelopmentScreensDir(operationID))
	if err != nil {
		return fmt.Errorf("create screens directory: %w", err)
	}
	final := filepath.Join(target, screen.Name)
	// Idempotent by digest: a file that already holds these bytes stays.
	if info, err := os.Lstat(final); err == nil && info.Mode().IsRegular() {
		if existing, err := os.ReadFile(final); err == nil {
			if have := sha256.Sum256(existing); hex.EncodeToString(have[:]) == screen.SHA256 {
				return nil
			}
		}
	}
	tmp, err := os.CreateTemp(target, ".screen-*")
	if err != nil {
		return err
	}
	_, werr := tmp.Write(data)
	serr := tmp.Sync()
	cerr := tmp.Close()
	if err := errors.Join(werr, serr, cerr, os.Chmod(tmp.Name(), 0o644)); err != nil {
		_ = os.Remove(tmp.Name())
		return err
	}
	// rename replaces a symlink at final rather than following it.
	if err := os.Rename(tmp.Name(), final); err != nil {
		_ = os.Remove(tmp.Name())
		return err
	}
	return syncDir(target)
}

// mkdirBeneath creates rel (slash separated, already validated plain names)
// under root one component at a time, refusing a symlink or a non-directory
// at any component, as internal/autoputer/files.go refuses a symlinked parent.
// It returns the directory's path.
func mkdirBeneath(root, rel string) (string, error) {
	cur := root
	for _, part := range strings.Split(rel, "/") {
		if part == "" || part == "." || part == ".." {
			return "", fmt.Errorf("unsafe path component %q", part)
		}
		cur = filepath.Join(cur, part)
		info, err := os.Lstat(cur)
		switch {
		case err == nil:
			if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
				return "", fmt.Errorf("%s is not a plain directory", cur)
			}
		case errors.Is(err, os.ErrNotExist):
			if err := os.Mkdir(cur, 0o755); err != nil && !errors.Is(err, os.ErrExist) {
				return "", err
			}
			if info, err := os.Lstat(cur); err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
				return "", fmt.Errorf("%s is not a plain directory", cur)
			}
		default:
			return "", err
		}
	}
	return cur, nil
}

// syncDir fsyncs a directory so a rename in it survives a hard kill.
func syncDir(dir string) error {
	f, err := os.Open(dir)
	if err != nil {
		return err
	}
	return errors.Join(f.Sync(), f.Close())
}

// resaveReplayedScreens re-saves a frozen bundle's screens at a replayed
// Freeze: a crash between the Executing->Frozen transition and
// recordFreezeScreens leaves them unsaved, and nothing else repairs that. It
// is idempotent by digest. out gets the rows and screen_errors; freezeScreensKey
// counts only the screens that were not saved before, so a replay of a
// complete freeze asks Texture for nothing.
func (toolCtx *CapsuleToolCtx) resaveReplayedScreens(operation selfdev.Operation, out map[string]any) {
	if toolCtx == nil || strings.TrimSpace(toolCtx.UpdaterRoot) == "" || operation.BundleDigest == "" {
		return
	}
	frozenRoot := filepath.Join(toolCtx.UpdaterRoot, "incoming", operation.BundleDigest)
	bundle, _, err := readBoundSelfDevelopmentBundle(frozenRoot, operation)
	if err != nil || len(bundle.Screens) == 0 {
		return
	}
	missing := 0
	for _, screen := range bundle.Screens {
		if !screenFileHolds(toolCtx.FilesRoot, operation.OperationID, screen) {
			missing++
		}
	}
	recordFreezeScreens(toolCtx.Images, toolCtx.FilesRoot, frozenRoot, operation.OperationID, bundle.Screens, out)
	saved, _ := out[freezeScreensKey].(int)
	repaired := min(missing, saved)
	out[freezeScreensKey] = repaired
}

// screenFileHolds says the file tree already holds screen's bytes.
func screenFileHolds(filesRoot, operationID string, screen capsule.ScreenFile) bool {
	if strings.TrimSpace(filesRoot) == "" || !plainOperationID.MatchString(operationID) || !capsule.ValidScreenName(screen.Name) {
		return false
	}
	path := filepath.Join(filesRoot, filepath.FromSlash(yaegikernel.SelfDevelopmentScreensDir(operationID)), screen.Name)
	if info, err := os.Lstat(path); err != nil || !info.Mode().IsRegular() {
		return false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]) == screen.SHA256
}

// ScreensListing is a frozen candidate's screens as Texture reads them: the
// digests and files come only from the operation's bound bundle on this
// computer, so no desk and no model can present another image as the
// candidate's.
type ScreensListing struct {
	OperationID  string
	TrajectoryID string
	ComputerID   string
	BundleDigest string
	Screens      []ScreenListingRow
}

// ScreenListingRow is one screen: its file in the computer's tree, its
// digest and size, and whether the asset store holds the bytes.
type ScreenListingRow struct {
	Name       string
	File       string
	SHA256     string
	Width      int
	Height     int
	Bytes      int64
	Blank      bool
	Resolvable bool
}

// CandidateScreens lists operationID's screens from its bound frozen bundle.
// It refuses before Freeze, for a bundle the operation does not bind, and
// for a candidate with no screens.
func (rt *Runtime) CandidateScreens(ctx context.Context, computerID, operationID string) (ScreensListing, error) {
	if rt == nil || rt.selfdevOperations == nil {
		return ScreensListing{}, errors.New("self-development operations unavailable")
	}
	operation, err := rt.selfdevOperations.Get(ctx, computerID, operationID)
	if err != nil {
		return ScreensListing{}, errors.New("self-development operation not found")
	}
	if operation.State == selfdev.StateRequested || operation.State == selfdev.StateExecuting || operation.BundleDigest == "" {
		return ScreensListing{}, errors.New("operation is " + operation.State + "; screens exist only once the candidate is frozen")
	}
	if strings.TrimSpace(rt.selfdevUpdaterRoot) == "" || !computerevent.IsSHA256(operation.BundleDigest) {
		return ScreensListing{}, errors.New("self-development bundle store unavailable or the digest is malformed")
	}
	bundle, _, err := readBoundSelfDevelopmentBundle(filepath.Join(rt.selfdevUpdaterRoot, "incoming", operation.BundleDigest), operation)
	if err != nil {
		return ScreensListing{}, err
	}
	if len(bundle.Screens) == 0 {
		return ScreensListing{}, errors.New("the candidate carries no screens")
	}
	store := rt.imageStore()
	out := ScreensListing{OperationID: operation.OperationID, TrajectoryID: operation.TrajectoryID, ComputerID: operation.ComputerID, BundleDigest: operation.BundleDigest}
	dir := yaegikernel.SelfDevelopmentScreensDir(operation.OperationID)
	for _, s := range bundle.Screens {
		out.Screens = append(out.Screens, ScreenListingRow{
			Name: s.Name, File: dir + "/" + s.Name, SHA256: s.SHA256, Width: s.Width, Height: s.Height, Bytes: s.Bytes, Blank: s.Blank,
			Resolvable: store.Has(s.SHA256),
		})
	}
	return out, nil
}
