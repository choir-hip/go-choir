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
	target := filepath.Join(filesRoot, filepath.FromSlash(yaegikernel.SelfDevelopmentScreensDir(operationID)))
	if err := os.MkdirAll(target, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", target, err)
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
	if err := os.Rename(tmp.Name(), filepath.Join(target, screen.Name)); err != nil {
		_ = os.Remove(tmp.Name())
		return err
	}
	return nil
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
