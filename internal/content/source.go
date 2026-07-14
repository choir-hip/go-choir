package content

import (
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/provideriface"
)

const textureShortcutExt = ".texture"

func NormalizeSourcePath(raw string) string {
	cleaned := path.Clean("/" + strings.TrimSpace(raw))
	cleaned = strings.TrimPrefix(cleaned, "/")
	if cleaned == "." {
		return ""
	}
	return cleaned
}

func IsTextureShortcutPath(sourcePath string) bool {
	ext := path.Ext(strings.TrimSpace(sourcePath))
	return strings.EqualFold(ext, textureShortcutExt)
}

func ReadSourceFileBytes(sourcePath string) ([]byte, bool) {
	sourcePath = NormalizeSourcePath(sourcePath)
	if sourcePath == "" || IsTextureShortcutPath(sourcePath) {
		return nil, false
	}
	filesRoot := provideriface.ResolveFilesRoot("")
	absPath := filepath.Join(filesRoot, filepath.FromSlash(sourcePath))
	cleanRoot, err := filepath.Abs(filesRoot)
	if err != nil {
		return nil, false
	}
	cleanPath, err := filepath.Abs(absPath)
	if err != nil {
		return nil, false
	}
	if cleanPath != cleanRoot && !strings.HasPrefix(cleanPath, cleanRoot+string(os.PathSeparator)) {
		return nil, false
	}
	info, err := os.Stat(cleanPath)
	if err != nil || info.IsDir() || info.Size() > maxImportedDocumentBytes {
		return nil, false
	}
	data, err := os.ReadFile(cleanPath)
	if err != nil {
		return nil, false
	}
	return data, true
}
