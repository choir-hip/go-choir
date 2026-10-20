package imageblob

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/provider"
)

// The asset view of the store (docs/reports/texture-media-and-autoradio-
// design-2026-10-20.md section 4). A digest names bytes; the store says what
// kind they are and which Content-Type serves them, and re-hashes on every
// read. The browser route (GET /api/assets/sha256/<hex>) reads through it.
//
// Only images are served now, because screenshots (visual evidence) are the
// first asset. Audio and video are typed here so the route and the Texture
// embed block do not change shape when they arrive, and they are refused
// until then.

// Kind is the class of an asset.
type Kind string

const (
	// KindImage: PNG, JPEG, GIF and WebP, the only served kind today.
	KindImage Kind = "image"
	// KindAudio and KindVideo are typed TODOs (texture media design T1 and
	// T4; video is also sequenced after the filmstrip in the visual evidence
	// design, section 10.4): the store neither accepts nor serves them yet.
	KindAudio Kind = "audio"
	KindVideo Kind = "video"
)

// Supported reports whether the store serves the kind.
func (k Kind) Supported() bool { return k == KindImage }

// KindOfContentType classifies a media type, or returns "" for one that is
// no asset kind.
func KindOfContentType(contentType string) Kind {
	switch {
	case strings.HasPrefix(contentType, "image/"):
		return KindImage
	case strings.HasPrefix(contentType, "audio/"):
		return KindAudio
	case strings.HasPrefix(contentType, "video/"):
		return KindVideo
	}
	return ""
}

var (
	// ErrBadDigest: the digest is not 64 lowercase hex characters.
	ErrBadDigest = errors.New("asset digest must be 64 lowercase hex characters")
	// ErrNotFound: the store holds no such digest.
	ErrNotFound = errors.New("asset is not in this computer's store")
	// ErrCorrupt: the stored bytes no longer hash to their name.
	ErrCorrupt = errors.New("asset failed its content hash")
	// ErrKindNotSupported: the bytes are not a kind the store serves yet.
	ErrKindNotSupported = errors.New("asset kind is not supported yet")
)

// Asset is one stored object, verified against its digest.
type Asset struct {
	Digest      string
	Ref         string
	Kind        Kind
	ContentType string
	Data        []byte
}

// Asset reads the object named by a lowercase hex sha256 digest, re-hashes
// it, and classifies it. It never follows anything but the store's own flat
// layout: the digest is validated before it touches the filesystem.
func (s *Store) Asset(digest string) (Asset, error) {
	if s == nil || s.dir == "" {
		return Asset{}, fmt.Errorf("image store is not configured")
	}
	if !validDigest(digest) {
		return Asset{}, ErrBadDigest
	}
	data, err := os.ReadFile(s.path(digest))
	if err != nil {
		if os.IsNotExist(err) {
			return Asset{}, ErrNotFound
		}
		return Asset{}, fmt.Errorf("read asset: %w", err)
	}
	if sha256Hex(data) != digest {
		return Asset{}, ErrCorrupt
	}
	contentType := provider.SniffImageType(data)
	if contentType == "" {
		return Asset{}, ErrKindNotSupported
	}
	return Asset{Digest: digest, Ref: "artifact:sha256:" + digest, Kind: KindOfContentType(contentType), ContentType: contentType, Data: data}, nil
}

// Has reports whether the store holds the digest, without reading it.
func (s *Store) Has(digest string) bool {
	if s == nil || s.dir == "" || !validDigest(digest) {
		return false
	}
	info, err := os.Stat(s.path(digest))
	return err == nil && info.Mode().IsRegular()
}

func validDigest(digest string) bool {
	if len(digest) != 64 {
		return false
	}
	for _, c := range digest {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
