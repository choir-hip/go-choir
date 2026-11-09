package textureowner

import (
	"bytes"
	"context"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"mime"
	"os"
	"path/filepath"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/imageblob"
	"github.com/yusefmosiah/go-choir/internal/provider"
	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/texturedoc"
	"github.com/yusefmosiah/go-choir/internal/texturewrite"
)

// textureMedia is the desk's reach to the computer's asset store and the
// owner's Files at Write time (station L8): {{image}} reads a picture the
// engineer saved in Files and stores it; {{file}} checks the file is there.
// Files are read beneath the Files root only: the path is validated, then
// resolved through symlinks and held under the root.
type textureMedia struct {
	images    *imageblob.Store
	filesRoot string
}

var _ texturewrite.Media = (*textureMedia)(nil)

// textureMedia is the media a Write on this computer can place, or nil when
// no runtime is bound (a Write then accepts only image digests).
func (h *Handler) textureMedia() texturewrite.Media {
	if h == nil || h.Core == nil {
		return nil
	}
	return &textureMedia{images: h.Core.AssetStore(), filesRoot: provideriface.ResolveFilesRoot("")}
}

func (m *textureMedia) HasImage(digest string) bool { return m.images.Has(digest) }

// resolve returns the regular file a Files path names, held under the root.
func (m *textureMedia) resolve(filesPath string) (string, os.FileInfo, error) {
	if err := texturedoc.ValidFilesPath(filesPath); err != nil {
		return "", nil, fmt.Errorf("the Files path %v", err)
	}
	root, err := filepath.EvalSymlinks(m.filesRoot)
	if err != nil {
		return "", nil, fmt.Errorf("Files are not reachable")
	}
	abs, err := filepath.EvalSymlinks(filepath.Join(root, filepath.FromSlash(filesPath)))
	if err != nil {
		return "", nil, fmt.Errorf("%s is not in Files; the engineer saves the file there first and reports its path", filesPath)
	}
	if abs != root && !strings.HasPrefix(abs, root+string(filepath.Separator)) {
		return "", nil, fmt.Errorf("%s resolves outside Files", filesPath)
	}
	info, err := os.Stat(abs)
	if err != nil || !info.Mode().IsRegular() {
		return "", nil, fmt.Errorf("%s is not a file in Files", filesPath)
	}
	return abs, info, nil
}

func (m *textureMedia) ImportImage(_ context.Context, filesPath string) (string, int, int, error) {
	abs, info, err := m.resolve(filesPath)
	if err != nil {
		return "", 0, 0, err
	}
	if info.Size() > provider.MaxImageInputBytes {
		return "", 0, 0, fmt.Errorf("%s is %d bytes; an image may be at most %d", filesPath, info.Size(), provider.MaxImageInputBytes)
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return "", 0, 0, fmt.Errorf("read %s: %v", filesPath, err)
	}
	ref, _, err := m.images.Put(data)
	if err != nil {
		return "", 0, 0, fmt.Errorf("%s is not usable as an image (save a PNG, JPEG or GIF; SVG is not stored): %v", filesPath, err)
	}
	digest := strings.TrimPrefix(ref, "artifact:sha256:")
	var width, height int
	if cfg, _, err := image.DecodeConfig(bytes.NewReader(data)); err == nil {
		width, height = cfg.Width, cfg.Height
	}
	return digest, width, height, nil
}

func (m *textureMedia) FileInfo(_ context.Context, filesPath string) (string, error) {
	if _, _, err := m.resolve(filesPath); err != nil {
		return "", err
	}
	if mt, _, err := mime.ParseMediaType(mime.TypeByExtension(strings.ToLower(filepath.Ext(filesPath)))); err == nil && mt != "" {
		return mt, nil
	}
	return "application/octet-stream", nil
}
