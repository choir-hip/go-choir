package pkgrelease

import (
	"bytes"
	"image/png"
	"regexp"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

// Media (docs/problems/library-releases-have-no-screenshots-2026-11-04.md):
// a release names the screenshots, and at most one walkthrough video, of the
// round that made it. The publisher computer takes them only from the
// applied operation's bound frozen bundle (the shots the judge accepted),
// never from a path. Each item is bound into the release id by sha256; the
// bytes live in the host content store's MediaNamespace and are served under
// the release's own visibility. A record with no media omits the field, so a
// record published before media canonicalizes and hashes as it did.

// MediaNamespace is the host content store namespace of release media,
// separate from the patches so their 2 MiB cap is untouched.
const MediaNamespace = "package-media"

// Media kinds and types.
const (
	MediaKindScreenshot = "screenshot"
	MediaKindVideo      = "video"
	MediaTypePNG        = "image/png"
	MediaTypeWebM       = "video/webm"
)

// Media bounds.
const (
	MaxScreenshots = 8
	MaxVideos      = 1
	// MaxScreenshotBytes holds one screenshot: a 1280x800 UI PNG is a few
	// hundred KiB.
	MaxScreenshotBytes = 2 << 20
	// MaxVideoBytes holds a 20 s walkthrough (Playwright's VP8 at 1 Mbit/s
	// is about 2.5 MB).
	MaxVideoBytes = 8 << 20
	// MaxMediaBytes is the largest single item, the content store's upload
	// cap for MediaNamespace.
	MaxMediaBytes = MaxVideoBytes
	// MaxScreenshotSide bounds each side of a screenshot: desktop
	// (1280x800, 1100x750) and phone (390x844) shots fit; a long full-page
	// capture does not.
	MaxScreenshotSide = 4096
)

var mediaName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,95}\.(png|webm)$`)

// Media is one item a release names.
type Media struct {
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	MediaType string `json:"media_type"`
	SHA256    string `json:"sha256"`
	Bytes     int    `json:"bytes"`
	// Width and Height are a screenshot's pixels; a video has none.
	Width  int `json:"width,omitempty"`
	Height int `json:"height,omitempty"`
}

// validMedia checks the set's shape and bounds.
func validMedia(items []Media) error {
	fail := func(format string, args ...any) error { return refuse(ReasonInvalid, format, args...) }
	shots, videos := 0, 0
	names, digests := map[string]bool{}, map[string]bool{}
	for _, m := range items {
		if !mediaName.MatchString(m.Name) || strings.Contains(m.Name, "..") {
			return fail("media name %q is not a plain .png or .webm name", m.Name)
		}
		if names[m.Name] || digests[m.SHA256] {
			return fail("media %s is named twice", m.Name)
		}
		names[m.Name], digests[m.SHA256] = true, true
		if !computerevent.IsSHA256(m.SHA256) || m.Bytes <= 0 {
			return fail("media %s names a sha256 and a positive size", m.Name)
		}
		switch m.Kind {
		case MediaKindScreenshot:
			shots++
			if m.MediaType != MediaTypePNG || !strings.HasSuffix(m.Name, ".png") {
				return fail("screenshot %s is an image/png .png", m.Name)
			}
			if m.Bytes > MaxScreenshotBytes {
				return refuse(ReasonBlobTooLarge, "screenshot %s is %d bytes; the cap is %d (2 MiB)", m.Name, m.Bytes, MaxScreenshotBytes)
			}
			if m.Width <= 0 || m.Height <= 0 || m.Width > MaxScreenshotSide || m.Height > MaxScreenshotSide {
				return fail("screenshot %s is %dx%d; each side is 1 to %d", m.Name, m.Width, m.Height, MaxScreenshotSide)
			}
		case MediaKindVideo:
			videos++
			if m.MediaType != MediaTypeWebM || !strings.HasSuffix(m.Name, ".webm") {
				return fail("video %s is a video/webm .webm", m.Name)
			}
			if m.Bytes > MaxVideoBytes {
				return refuse(ReasonBlobTooLarge, "video %s is %d bytes; the cap is %d (8 MiB)", m.Name, m.Bytes, MaxVideoBytes)
			}
			if m.Width != 0 || m.Height != 0 {
				return fail("video %s names no size", m.Name)
			}
		default:
			return fail("media %s kind %q is not screenshot or video", m.Name, m.Kind)
		}
	}
	if shots > MaxScreenshots || videos > MaxVideos {
		return fail("a release names at most %d screenshots and %d video; this one names %d and %d", MaxScreenshots, MaxVideos, shots, videos)
	}
	return nil
}

// VerifyMedia requires raw to be item's bytes: the length and digest, then
// a PNG that decodes at the stated size, or a WebM (EBML) header.
func VerifyMedia(item Media, raw []byte) error {
	limit := MaxScreenshotBytes
	if item.Kind == MediaKindVideo {
		limit = MaxVideoBytes
	}
	if len(raw) > limit {
		return refuse(ReasonBlobTooLarge, "media %s is %d bytes; the cap is %d", item.Name, len(raw), limit)
	}
	if len(raw) != item.Bytes || computerevent.DigestBytes(raw) != item.SHA256 {
		return refuse(ReasonBlobMismatch, "the bytes of media %s do not hash to %s with %d bytes", item.Name, item.SHA256, item.Bytes)
	}
	switch item.Kind {
	case MediaKindScreenshot:
		cfg, err := png.DecodeConfig(bytes.NewReader(raw))
		if err != nil || cfg.Width != item.Width || cfg.Height != item.Height {
			return refuse(ReasonBlobMismatch, "media %s is not a %dx%d PNG", item.Name, item.Width, item.Height)
		}
		if _, err := png.Decode(bytes.NewReader(raw)); err != nil {
			return refuse(ReasonBlobMismatch, "media %s does not decode as a PNG", item.Name)
		}
	case MediaKindVideo:
		if !IsWebM(raw) {
			return refuse(ReasonBlobMismatch, "media %s is not a WebM file", item.Name)
		}
	default:
		return refuse(ReasonInvalid, "media %s kind %q is not screenshot or video", item.Name, item.Kind)
	}
	return nil
}

// IsWebM says raw starts with an EBML header whose DocType is webm.
func IsWebM(raw []byte) bool {
	if len(raw) < 8 || !bytes.Equal(raw[:4], []byte{0x1a, 0x45, 0xdf, 0xa3}) {
		return false
	}
	head := raw[:min(len(raw), 64)]
	// DocType (0x4282), a one-byte size of 4, then "webm".
	return bytes.Contains(head, []byte{0x42, 0x82, 0x84, 'w', 'e', 'b', 'm'})
}

// MediaItem returns the record's item with digest, if it names one.
func (r Record) MediaItem(digest string) (Media, bool) {
	for _, m := range r.Media {
		if m.SHA256 == digest {
			return m, true
		}
	}
	return Media{}, false
}
