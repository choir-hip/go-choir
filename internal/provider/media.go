package provider

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"image/draw"
	_ "image/gif" // registers GIF for DecodeConfig/Decode
	_ "image/jpeg"
	"image/png"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/yusefmosiah/go-choir/internal/modelcatalog"
)

// Image limits, applied at the gateway before any adapter serializes an
// image (docs/problems/models-configured-text-only-2026-10-15.md, failure
// mode 7). Anthropic refuses images over 5 MB or 8000 px a side and resizes
// long edges over 1568 px itself, so sending more costs bytes and buys
// nothing.
const (
	MaxImagesPerRequest = 20
	// MaxImageInputBytes bounds an encoded image before it is decoded.
	MaxImageInputBytes = 20 << 20
	// MaxImageBytes bounds an encoded image after downscaling.
	MaxImageBytes = 5 << 20
	// MaxImagePixels bounds the header-declared size, checked before
	// decoding, so a small file with huge dimensions is refused unread.
	MaxImagePixels = 50_000_000
	// MaxImageLongEdge is the long edge a larger PNG/JPEG/GIF is
	// downscaled to.
	MaxImageLongEdge = 1568
	// MaxConcurrentImageDecodes bounds the images decoded at once,
	// process-wide (docs/problems/gateway-request-bodies-unbounded-2026-10-22.md,
	// bunch 109). Decoded pixels are outside the gateway's body budget: one
	// image at MaxImagePixels is up to 400 MB decoded (16-bit RGBA PNG) plus
	// the 200 MB RGBA copy downscale makes, about 620 MB with its 20 MiB
	// input, which is decoded from base64 inside the slot too (bunch 116,
	// F1); four are about 2.5 GB, which fits beside the body budget's heap
	// under the gateway unit's GOMEMLIMIT and MemoryMax.
	MaxConcurrentImageDecodes = 4
	// MaxImageDecodesPerCaller bounds the decode slots one caller (the
	// computer, as the gateway's body share is keyed) holds or waits on
	// at once, so one computer's queue of large images leaves the other
	// slots to every other computer (bunch 116, F2).
	MaxImageDecodesPerCaller = 2
)

// imageDecodeSlots holds one token per base64 image being decoded,
// normalized and downscaled.
var imageDecodeSlots = make(chan struct{}, MaxConcurrentImageDecodes)

// imageDecodeCallers holds each caller's share of the decode slots while
// any of its images holds or waits on one.
var imageDecodeCallers = struct {
	mu sync.Mutex
	m  map[string]*callerDecodeSlots
}{m: map[string]*callerDecodeSlots{}}

type callerDecodeSlots struct {
	ch   chan struct{}
	refs int
}

// imageDecodeHook, when set (tests), runs inside a decode slot with the
// slot's caller.
var imageDecodeHook func(caller string)

// ErrImageDecodeWait wraps the context's error when a call ends while it
// waits for a decode slot. It is not a MediaError: the image was not
// refused, and the breaker does not count it.
var ErrImageDecodeWait = errors.New("call ended while waiting for an image decode slot")

type imageDecodeCallerKey struct{}

// WithImageDecodeCaller names the caller whose images ctx decodes; the
// gateway sets it to the computer on every model call.
func WithImageDecodeCaller(ctx context.Context, caller string) context.Context {
	return context.WithValue(ctx, imageDecodeCallerKey{}, caller)
}

// ImageDecodeCaller is the caller WithImageDecodeCaller set, or "".
func ImageDecodeCaller(ctx context.Context) string {
	caller, _ := ctx.Value(imageDecodeCallerKey{}).(string)
	return caller
}

// acquireImageDecodeSlot takes one of the MaxConcurrentImageDecodes slots,
// first one of the caller's MaxImageDecodesPerCaller (a caller-less ctx
// takes the process slot only), and gives up when ctx ends.
func acquireImageDecodeSlot(ctx context.Context) (func(), error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrImageDecodeWait, err)
	}
	caller := ImageDecodeCaller(ctx)
	var mine *callerDecodeSlots
	drop := func() {
		if mine == nil {
			return
		}
		imageDecodeCallers.mu.Lock()
		if mine.refs--; mine.refs == 0 {
			delete(imageDecodeCallers.m, caller)
		}
		imageDecodeCallers.mu.Unlock()
	}
	if caller != "" {
		imageDecodeCallers.mu.Lock()
		mine = imageDecodeCallers.m[caller]
		if mine == nil {
			mine = &callerDecodeSlots{ch: make(chan struct{}, MaxImageDecodesPerCaller)}
			imageDecodeCallers.m[caller] = mine
		}
		mine.refs++
		imageDecodeCallers.mu.Unlock()
		select {
		case mine.ch <- struct{}{}:
		case <-ctx.Done():
			drop()
			return nil, fmt.Errorf("%w: %w", ErrImageDecodeWait, ctx.Err())
		}
	}
	select {
	case imageDecodeSlots <- struct{}{}:
	case <-ctx.Done():
		if mine != nil {
			<-mine.ch
			drop()
		}
		return nil, fmt.Errorf("%w: %w", ErrImageDecodeWait, ctx.Err())
	}
	release := func() {
		<-imageDecodeSlots
		if mine != nil {
			<-mine.ch
			drop()
		}
	}
	if imageDecodeHook != nil {
		imageDecodeHook(caller)
	}
	return release, nil
}

// MediaError is an image the gateway refused before calling the upstream.
// It is the caller's error: the gateway answers 400, the guest does not
// retry, and the provider breaker does not count it.
type MediaError struct{ msg string }

func (e *MediaError) Error() string { return e.msg }

func mediaErrorf(format string, args ...any) error {
	return &MediaError{msg: fmt.Sprintf(format, args...)}
}

// IsMediaError reports whether err is (or wraps) a refused image.
func IsMediaError(err error) bool {
	var mediaErr *MediaError
	return errors.As(err, &mediaErr)
}

// urlImageSourcesAllowed names the providers whose wire documents URL image
// sources. Bedrock takes base64 only; OpenCode and Z.AI URL fetching is
// UNVERIFIED. Elsewhere a caller inlines the image or passes an
// artifact_ref the computer resolves.
func urlImageSourcesAllowed(providerName string) bool {
	switch strings.TrimSpace(providerName) {
	case "chatgpt", "fireworks":
		return true
	default:
		return false
	}
}

// prepareMediaRequest checks every image block against the model's
// capability and the image limits, and returns the request with oversized
// images downscaled. A request without images is returned unchanged.
// Errors never quote image data. Each base64 image waits for a decode slot
// (acquireImageDecodeSlot) until ctx ends.
func prepareMediaRequest(ctx context.Context, providerName, modelID string, req LLMRequest) (LLMRequest, error) {
	count := 0
	for _, msg := range req.Messages {
		for _, block := range msg.Content {
			if block.Type == "image" {
				count++
			}
		}
	}
	if count == 0 {
		return req, nil
	}
	if refusal := modelcatalog.ImageRefusal(modelID); refusal != "" {
		return req, &MediaError{msg: refusal}
	}
	if count > MaxImagesPerRequest {
		return req, mediaErrorf("request carries %d images; the limit is %d", count, MaxImagesPerRequest)
	}
	messages := make([]Message, len(req.Messages))
	for i, msg := range req.Messages {
		messages[i] = msg
		if !messageHasImage(msg) {
			continue
		}
		content := make([]Block, len(msg.Content))
		copy(content, msg.Content)
		for j, block := range content {
			if block.Type != "image" {
				continue
			}
			source, err := prepareImageSource(ctx, providerName, block.Source)
			if err != nil {
				return req, err
			}
			block.Source = source
			content[j] = block
		}
		messages[i].Content = content
	}
	req.Messages = messages
	return req, nil
}

func messageHasImage(msg Message) bool {
	for _, block := range msg.Content {
		if block.Type == "image" {
			return true
		}
	}
	return false
}

func prepareImageSource(ctx context.Context, providerName string, source *MediaSource) (*MediaSource, error) {
	if source == nil {
		return nil, mediaErrorf("image block missing source")
	}
	switch strings.TrimSpace(source.Kind) {
	case "base64":
		data := strings.TrimSpace(source.Data)
		if data == "" {
			return nil, mediaErrorf("image block base64 source missing data")
		}
		if base64.StdEncoding.DecodedLen(len(data)) > MaxImageInputBytes+3 {
			return nil, mediaErrorf("image is over %d bytes before resizing", MaxImageInputBytes)
		}
		// The raw copy, up to MaxImageInputBytes, exists only inside a
		// decode slot (bunch 116, F1).
		release, err := acquireImageDecodeSlot(ctx)
		if err != nil {
			return nil, err
		}
		defer release()
		raw, err := base64.StdEncoding.DecodeString(data)
		if err != nil {
			return nil, mediaErrorf("image block base64 source is not valid standard base64: %v", err)
		}
		normalized, mimeType, err := normalizeImage(raw)
		if err != nil {
			return nil, err
		}
		out := *source
		out.MIMEType = mimeType
		if !bytes.Equal(normalized, raw) {
			out.Data = base64.StdEncoding.EncodeToString(normalized)
		} else {
			out.Data = data
		}
		return &out, nil
	case "url":
		url := strings.TrimSpace(source.URL)
		if url == "" {
			return nil, mediaErrorf("image block url source missing url")
		}
		if !strings.HasPrefix(url, "https://") && !strings.HasPrefix(url, "http://") {
			return nil, mediaErrorf("image block url source must be an absolute http(s) URL")
		}
		if !urlImageSourcesAllowed(providerName) {
			return nil, mediaErrorf("provider %s does not take image URLs; inline the image as base64 or pass an artifact_ref", providerName)
		}
		return source, nil
	case "artifact_ref":
		return nil, mediaErrorf("image artifact_ref %q must be resolved by the computer before the gateway call", source.Ref)
	default:
		return nil, mediaErrorf("unsupported image source kind %q", source.Kind)
	}
}

// NormalizeImage applies the image limits to one encoded image and returns
// the bytes to send and their sniffed media type. A PNG, JPEG or GIF whose
// long edge exceeds MaxImageLongEdge is downscaled (area average) and
// re-encoded as PNG. The result is a pure function of the input bytes, so a
// re-sent image keeps its prompt-cache prefix. Other image types (WebP)
// cannot be decoded here and pass only within MaxImageBytes. It runs in a
// decode slot.
func NormalizeImage(raw []byte) ([]byte, string, error) {
	release, err := acquireImageDecodeSlot(context.Background())
	if err != nil {
		return nil, "", err
	}
	defer release()
	return normalizeImage(raw)
}

// normalizeImage is NormalizeImage inside a slot the caller holds.
func normalizeImage(raw []byte) ([]byte, string, error) {
	if len(raw) > MaxImageInputBytes {
		return nil, "", mediaErrorf("image is %d bytes; the limit before resizing is %d", len(raw), MaxImageInputBytes)
	}
	mimeType := SniffImageType(raw)
	if mimeType == "" {
		return nil, "", mediaErrorf("image bytes are not a PNG, JPEG, GIF or WebP image")
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil {
		if len(raw) > MaxImageBytes {
			return nil, "", mediaErrorf("%s image is %d bytes; the limit is %d", mimeType, len(raw), MaxImageBytes)
		}
		return raw, mimeType, nil
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || int64(cfg.Width)*int64(cfg.Height) > MaxImagePixels {
		return nil, "", mediaErrorf("image is %dx%d; the limit is %d pixels", cfg.Width, cfg.Height, MaxImagePixels)
	}
	if max(cfg.Width, cfg.Height) <= MaxImageLongEdge {
		if len(raw) > MaxImageBytes {
			return nil, "", mediaErrorf("image is %d bytes; the limit is %d", len(raw), MaxImageBytes)
		}
		return raw, mimeType, nil
	}
	scaled, err := decodeAndDownscale(raw)
	if err != nil {
		return nil, "", mediaErrorf("decode %s image: %v", mimeType, err)
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, scaled); err != nil {
		return nil, "", mediaErrorf("re-encode image: %v", err)
	}
	if buf.Len() > MaxImageBytes {
		return nil, "", mediaErrorf("image is %d bytes after resizing; the limit is %d", buf.Len(), MaxImageBytes)
	}
	return buf.Bytes(), "image/png", nil
}

// decodeAndDownscale decodes raw and downscales it to MaxImageLongEdge.
// Its caller holds a decode slot, so the full-size buffers live only
// inside one.
func decodeAndDownscale(raw []byte) (*image.RGBA, error) {
	src, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	return downscale(src, MaxImageLongEdge), nil
}

// SniffImageType returns the media type of an image Choir forwards, or ""
// for anything else.
func SniffImageType(raw []byte) string {
	switch mimeType := http.DetectContentType(raw); mimeType {
	case "image/png", "image/jpeg", "image/gif", "image/webp":
		return mimeType
	default:
		return ""
	}
}

// downscale resizes src so its long edge is longEdge, averaging each
// destination pixel's source box (premultiplied RGBA).
func downscale(src image.Image, longEdge int) *image.RGBA {
	b := src.Bounds()
	sw, sh := b.Dx(), b.Dy()
	dw, dh := longEdge, longEdge
	if sw >= sh {
		dh = max(1, int((int64(sh)*int64(longEdge)+int64(sw)/2)/int64(sw)))
	} else {
		dw = max(1, int((int64(sw)*int64(longEdge)+int64(sh)/2)/int64(sh)))
	}
	rgba := image.NewRGBA(image.Rect(0, 0, sw, sh))
	draw.Draw(rgba, rgba.Bounds(), src, b.Min, draw.Src)
	dst := image.NewRGBA(image.Rect(0, 0, dw, dh))
	for dy := 0; dy < dh; dy++ {
		y0, y1 := dy*sh/dh, (dy+1)*sh/dh
		if y1 <= y0 {
			y1 = y0 + 1
		}
		for dx := 0; dx < dw; dx++ {
			x0, x1 := dx*sw/dw, (dx+1)*sw/dw
			if x1 <= x0 {
				x1 = x0 + 1
			}
			var r, g, bl, a, n uint64
			for y := y0; y < y1; y++ {
				row := rgba.Pix[y*rgba.Stride+x0*4 : y*rgba.Stride+x1*4]
				for i := 0; i < len(row); i += 4 {
					r += uint64(row[i])
					g += uint64(row[i+1])
					bl += uint64(row[i+2])
					a += uint64(row[i+3])
					n++
				}
			}
			o := dy*dst.Stride + dx*4
			dst.Pix[o] = uint8((r + n/2) / n)
			dst.Pix[o+1] = uint8((g + n/2) / n)
			dst.Pix[o+2] = uint8((bl + n/2) / n)
			dst.Pix[o+3] = uint8((a + n/2) / n)
		}
	}
	return dst
}

// MediaStats summarizes a request's images for logs: counts, sizes and
// short content hashes, never data.
type MediaStats struct {
	Images int
	Bytes  int
	Refs   []string
}

// String renders the stats for a log line.
func (s MediaStats) String() string {
	if s.Images == 0 {
		return "images=0"
	}
	return fmt.Sprintf("images=%d image_bytes=%d image_sha256=%s", s.Images, s.Bytes, strings.Join(s.Refs, ","))
}

// SummarizeMedia counts a request's image blocks. Base64 images are named
// by the first 12 hex digits of the sha256 of their bytes (the digest of
// their artifact:sha256 ref), URLs by "url", refs by their own digest. A
// base64 image is hashed as it is decoded, never held whole (bunch 116,
// F1: this runs for the log line, outside any decode slot).
func SummarizeMedia(messages []Message) MediaStats {
	var stats MediaStats
	for _, msg := range messages {
		for _, block := range msg.Content {
			if block.Type != "image" || block.Source == nil {
				continue
			}
			stats.Images++
			switch strings.TrimSpace(block.Source.Kind) {
			case "base64":
				hash := sha256.New()
				n, err := io.Copy(hash, base64.NewDecoder(base64.StdEncoding, strings.NewReader(strings.TrimSpace(block.Source.Data))))
				if err != nil {
					stats.Refs = append(stats.Refs, "invalid")
					continue
				}
				stats.Bytes += int(n)
				stats.Refs = append(stats.Refs, hex.EncodeToString(hash.Sum(nil))[:12])
			case "artifact_ref":
				ref := strings.TrimPrefix(strings.TrimSpace(block.Source.Ref), "artifact:sha256:")
				if len(ref) > 12 {
					ref = ref[:12]
				}
				stats.Refs = append(stats.Refs, ref)
			default:
				stats.Refs = append(stats.Refs, "url")
			}
		}
	}
	return stats
}
