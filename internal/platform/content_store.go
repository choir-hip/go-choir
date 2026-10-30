package platform

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

// The host content store: content-addressed bytes on corpusd's artifact
// root, outside every computer's restore set, never in Dolt rows (the
// captures "104 G lesson"). One store, several namespaces:
//
//	sha256/platform-update/<sha>  platform update payloads, host build closures (P5)
//	sha256/release/<sha>          release record source patches (P7)
//
// It generalizes the platform-update blob store
// (docs/archive/reports/p6-p8-release-adopt-2026-10-15.md, "The content store").
const (
	contentNamespacePlatformUpdate = "platform-update"
	contentNamespaceRelease        = "release"
	// contentStoreMaxBytes is the upload cap of every namespace.
	contentStoreMaxBytes int64 = 2 << 30
)

var (
	errContentDigestMismatch = errors.New("content digest mismatch")
	errContentTooLarge       = errors.New("content exceeds the store cap")
	errContentNotFound       = errors.New("content not found")
)

type hostContentStore struct {
	service   *Service
	namespace string
	maxBytes  int64
}

func (h *Handler) contentStore(namespace string) hostContentStore {
	var service *Service
	if h != nil {
		service = h.service
	}
	return hostContentStore{service: service, namespace: namespace, maxBytes: contentStoreMaxBytes}
}

func (c hostContentStore) path(digest string) (string, error) {
	if !computerevent.IsSHA256(digest) {
		return "", fmt.Errorf("content store: sha256 digest required")
	}
	if c.service == nil || c.namespace == "" {
		return "", fmt.Errorf("content store unavailable")
	}
	return c.service.artifactPath(filepath.Join("sha256", c.namespace, digest))
}

// Put streams body into the store under digest, hashing as it copies. Bytes
// over the cap or hashing elsewhere leave nothing behind. An existing blob
// is kept: the store is content addressed.
func (c hostContentStore) Put(digest string, body io.Reader) error {
	target, err := c.path(digest)
	if err != nil {
		return err
	}
	if info, statErr := os.Stat(target); statErr == nil && info.Mode().IsRegular() && c.hashesToName(target, digest) {
		return nil
	}
	// A blob that no longer hashes to its name is replaced by bytes that do.
	if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
		return fmt.Errorf("content store: create dir: %w", err)
	}
	tmp := target + ".tmp-" + fmt.Sprintf("%d", time.Now().UnixNano())
	out, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o640)
	if err != nil {
		return fmt.Errorf("content store: stage temp: %w", err)
	}
	hasher := sha256.New()
	limit := c.maxBytes
	if limit <= 0 {
		limit = contentStoreMaxBytes
	}
	n, copyErr := io.Copy(out, io.TeeReader(io.LimitReader(body, limit+1), hasher))
	closeErr := out.Close()
	switch {
	case copyErr != nil:
		os.Remove(tmp)
		return fmt.Errorf("content store: read upload: %w", copyErr)
	case closeErr != nil:
		os.Remove(tmp)
		return fmt.Errorf("content store: write: %w", closeErr)
	case n > limit:
		os.Remove(tmp)
		return errContentTooLarge
	case hex.EncodeToString(hasher.Sum(nil)) != digest:
		os.Remove(tmp)
		return errContentDigestMismatch
	}
	if err := os.Rename(tmp, target); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("content store: install: %w", err)
	}
	return nil
}

// hashesToName streams the file through sha256 (never buffering it whole).
func (c hostContentStore) hashesToName(path, digest string) bool {
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return false
	}
	return hex.EncodeToString(hasher.Sum(nil)) == digest
}

// Open returns the stored file for streaming; the reader verifies.
func (c hostContentStore) Open(digest string) (*os.File, error) {
	target, err := c.path(digest)
	if err != nil {
		return nil, err
	}
	file, err := os.Open(target)
	if errors.Is(err, os.ErrNotExist) {
		return nil, errContentNotFound
	}
	return file, err
}

// ReadVerified reads a blob of at most max bytes and requires it to hash to
// its name: the store never trusts a file by its name.
func (c hostContentStore) ReadVerified(digest string, max int64) ([]byte, error) {
	file, err := c.Open(digest)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	raw, err := io.ReadAll(io.LimitReader(file, max+1))
	if err != nil {
		return nil, err
	}
	if int64(len(raw)) > max {
		return nil, errContentTooLarge
	}
	if computerevent.DigestBytes(raw) != digest {
		return nil, errContentDigestMismatch
	}
	return raw, nil
}
