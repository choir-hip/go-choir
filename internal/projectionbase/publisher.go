package projectionbase

import (
	"archive/tar"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Publisher handles atomic, fsync'd publication of ProjectionBase tar artifacts.
type Publisher struct {
	artifactsRoot string
}

// NewPublisher returns a publisher for the given artifacts root directory.
func NewPublisher(artifactsRoot string) *Publisher {
	return &Publisher{artifactsRoot: filepath.Clean(artifactsRoot)}
}

// PublishDir archives the given source directory into a tar blob, computes its sha256,
// and writes it atomically to <artifactsRoot>/sha256/projection-base/<sha256>.
func (p *Publisher) PublishDir(srcDir string) (string, int64, error) {
	srcDir = filepath.Clean(srcDir)
	info, err := os.Stat(srcDir)
	if err != nil || !info.IsDir() {
		return "", 0, fmt.Errorf("publisher: invalid source directory %q: %w", srcDir, err)
	}

	targetDir := filepath.Join(p.artifactsRoot, "sha256", Namespace)
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return "", 0, fmt.Errorf("publisher: create target directory: %w", err)
	}

	// Create a temporary file in the target directory for atomic rename.
	tmpFile, err := os.CreateTemp(targetDir, ".tmp-projection-base-*")
	if err != nil {
		return "", 0, fmt.Errorf("publisher: create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer func() {
		// Clean up temporary file on failure.
		if tmpFile != nil {
			_ = tmpFile.Close()
			_ = os.Remove(tmpPath)
		}
	}()

	hasher := sha256.New()
	multiWriter := io.MultiWriter(tmpFile, hasher)
	tarWriter := tar.NewWriter(multiWriter)

	var totalBytes int64
	err = filepath.Walk(srcDir, func(path string, fi os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relPath, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		if relPath == "." {
			return nil
		}
		// Normalize path separators to forward slash for tar archive portability.
		relPath = filepath.ToSlash(relPath)

		header, err := tar.FileInfoHeader(fi, fi.Name())
		if err != nil {
			return fmt.Errorf("tar header %q: %w", path, err)
		}
		header.Name = relPath

		if err := tarWriter.WriteHeader(header); err != nil {
			return fmt.Errorf("write tar header %q: %w", path, err)
		}

		if fi.Mode().IsRegular() {
			file, err := os.Open(path)
			if err != nil {
				return fmt.Errorf("open file %q: %w", path, err)
			}
			defer file.Close()
			n, err := io.Copy(tarWriter, file)
			if err != nil {
				return fmt.Errorf("copy file %q to tar: %w", path, err)
			}
			totalBytes += n
		}
		return nil
	})

	if err != nil {
		return "", 0, fmt.Errorf("publisher: archive directory: %w", err)
	}

	if err := tarWriter.Close(); err != nil {
		return "", 0, fmt.Errorf("publisher: close tar writer: %w", err)
	}

	// Fsync file before closing.
	if err := tmpFile.Sync(); err != nil {
		return "", 0, fmt.Errorf("publisher: fsync temp file: %w", err)
	}
	tmpInfo, err := tmpFile.Stat()
	if err != nil {
		return "", 0, fmt.Errorf("publisher: stat temp file: %w", err)
	}
	blobSize := tmpInfo.Size()

	if err := tmpFile.Close(); err != nil {
		return "", 0, fmt.Errorf("publisher: close temp file: %w", err)
	}
	tmpFile = nil // Prevent defer from removing after successful close

	digest := hex.EncodeToString(hasher.Sum(nil))
	finalPath := filepath.Join(targetDir, digest)

	// Atomic rename to final path.
	if err := os.Rename(tmpPath, finalPath); err != nil {
		_ = os.Remove(tmpPath)
		return "", 0, fmt.Errorf("publisher: rename to final path: %w", err)
	}

	// Fsync directory to ensure rename is durably written to disk.
	if dirFile, err := os.Open(targetDir); err == nil {
		_ = dirFile.Sync()
		_ = dirFile.Close()
	}

	return digest, blobSize, nil
}

// Unpack extracts a ProjectionBase tar blob into the destination directory.
func Unpack(blobPath, dstDir string) error {
	blobPath = filepath.Clean(blobPath)
	dstDir = filepath.Clean(dstDir)

	file, err := os.Open(blobPath)
	if err != nil {
		return fmt.Errorf("unpack: open blob: %w", err)
	}
	defer file.Close()

	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		return fmt.Errorf("unpack: create destination directory: %w", err)
	}

	tarReader := tar.NewReader(file)
	for {
		header, err := tarReader.Next()
		if errorsIsEOF(err) {
			break
		}
		if err != nil {
			return fmt.Errorf("unpack: read tar header: %w", err)
		}

		// Security: prevent zip-slip attacks by cleaning and checking prefix.
		cleanName := filepath.Clean(header.Name)
		if strings.HasPrefix(cleanName, "..") || strings.HasPrefix(cleanName, "/") {
			return fmt.Errorf("unpack: invalid path in tar: %q", header.Name)
		}

		target := filepath.Join(dstDir, cleanName)
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return fmt.Errorf("unpack: mkdir %q: %w", target, err)
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return fmt.Errorf("unpack: mkdir parent %q: %w", target, err)
			}
			outFile, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR|os.O_TRUNC, header.FileInfo().Mode())
			if err != nil {
				return fmt.Errorf("unpack: create file %q: %w", target, err)
			}
			if _, err := io.Copy(outFile, tarReader); err != nil {
				_ = outFile.Close()
				return fmt.Errorf("unpack: write file %q: %w", target, err)
			}
			_ = outFile.Sync()
			_ = outFile.Close()
		}
	}
	return nil
}

func errorsIsEOF(err error) bool {
	return err == io.EOF
}
