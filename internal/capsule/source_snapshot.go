package capsule

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

type canonicalTreeEntry struct {
	Path          string `json:"path"`
	Type          string `json:"type"`
	Mode          uint32 `json:"mode"`
	ContentDigest string `json:"content_digest,omitempty"`
}

// digestCanonicalSubjectTree defines the one subject/candidate digest domain:
// a complete reconstructable /workspace/platform tree. Cache, tmpfs, overlay
// metadata, and every path outside that root are deliberately excluded.
func digestCanonicalSubjectTree(ctx context.Context, root string) (string, error) {
	root = filepath.Clean(root)
	if !filepath.IsAbs(root) {
		return "", fmt.Errorf("subject tree root must be absolute")
	}
	var entries []canonicalTreeEntry
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if path == root {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil || relative == "." || filepath.IsAbs(relative) || relative == ".." || strings.HasPrefix(relative, ".."+string(os.PathSeparator)) {
			return fmt.Errorf("subject tree contains unsafe path %q", path)
		}
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		item := canonicalTreeEntry{Path: filepath.ToSlash(relative)}
		switch {
		case info.IsDir():
			item.Type, item.Mode = "directory", 0o755
		case info.Mode().IsRegular():
			item.Type, item.Mode = "file", 0o644
			if info.Mode()&0o111 != 0 {
				item.Mode = 0o755
			}
			input, err := os.Open(path)
			if err != nil {
				return err
			}
			hash := sha256.New()
			_, copyErr := io.Copy(hash, &contextReader{ctx: ctx, reader: input})
			closeErr := input.Close()
			if copyErr != nil || closeErr != nil {
				return errors.Join(copyErr, closeErr)
			}
			item.ContentDigest = hex.EncodeToString(hash.Sum(nil))
		case info.Mode()&os.ModeSymlink != 0:
			item.Type, item.Mode = "symlink", 0o777
			target, err := os.Readlink(path)
			if err != nil {
				return err
			}
			digest := sha256.Sum256([]byte(target))
			item.ContentDigest = hex.EncodeToString(digest[:])
		default:
			return fmt.Errorf("subject tree refuses special file %q", item.Path)
		}
		entries = append(entries, item)
		return nil
	})
	if err != nil {
		return "", err
	}
	if len(entries) == 0 {
		return "", fmt.Errorf("subject tree is empty")
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	canonical, err := json.Marshal(entries)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(canonical)
	return hex.EncodeToString(digest[:]), nil
}

func copyCanonicalSubjectTree(ctx context.Context, source, target string) error {
	source, target = filepath.Clean(source), filepath.Clean(target)
	if !filepath.IsAbs(source) || !filepath.IsAbs(target) || source == target {
		return fmt.Errorf("subject copy requires distinct absolute roots")
	}
	return filepath.WalkDir(source, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		relative, err := filepath.Rel(source, path)
		if err != nil || filepath.IsAbs(relative) || relative == ".." || strings.HasPrefix(relative, ".."+string(os.PathSeparator)) {
			return fmt.Errorf("subject copy contains unsafe path")
		}
		if relative == "." {
			return os.MkdirAll(target, 0o755)
		}
		destination := filepath.Join(target, relative)
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		switch {
		case info.IsDir():
			return os.Mkdir(destination, 0o755)
		case info.Mode().IsRegular():
			input, err := os.Open(path)
			if err != nil {
				return err
			}
			mode := os.FileMode(0o644)
			if info.Mode()&0o111 != 0 {
				mode = 0o755
			}
			output, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
			if err != nil {
				_ = input.Close()
				return err
			}
			_, copyErr := io.Copy(output, &contextReader{ctx: ctx, reader: input})
			closeErr := errors.Join(input.Close(), output.Close())
			return errors.Join(copyErr, closeErr)
		case info.Mode()&os.ModeSymlink != 0:
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			cleanLink := filepath.Clean(link)
			if (filepath.IsAbs(cleanLink) && !strings.HasPrefix(cleanLink, "/nix/store/")) ||
				(!filepath.IsAbs(cleanLink) && (cleanLink == ".." || strings.HasPrefix(cleanLink, ".."+string(os.PathSeparator)))) {
				return fmt.Errorf("subject copy refuses escaping symlink %q", relative)
			}
			return os.Symlink(link, destination)
		default:
			return fmt.Errorf("subject copy refuses special file %q", relative)
		}
	})
}

func makeSubjectTreeReadOnly(root string) error {
	var dirs []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		if info.IsDir() {
			dirs = append(dirs, path)
			return nil
		}
		if info.Mode().IsRegular() {
			mode := os.FileMode(0o444)
			if info.Mode()&0o111 != 0 {
				mode = 0o555
			}
			return os.Chmod(path, mode)
		}
		return nil
	})
	if err != nil {
		return err
	}
	sort.Slice(dirs, func(i, j int) bool { return len(dirs[i]) > len(dirs[j]) })
	for _, dir := range dirs {
		if err := os.Chmod(dir, 0o755); err != nil {
			return err
		}
	}
	return nil
}

func immutableGitCommitIdentity(ctx context.Context, source string) (string, error) {
	for _, args := range [][]string{{"diff", "--quiet", "--"}, {"diff", "--cached", "--quiet", "--"}} {
		command := exec.CommandContext(ctx, "git", append([]string{"-C", source}, args...)...)
		if err := command.Run(); err != nil {
			if contextErr := ctx.Err(); contextErr != nil {
				return "", contextErr
			}
			return "", fmt.Errorf("source snapshot refuses dirty tracked files")
		}
	}
	raw, err := exec.CommandContext(ctx, "git", "-C", source, "rev-parse", "--verify", "HEAD^{commit}").Output()
	if err != nil {
		return "", fmt.Errorf("source snapshot commit identity: %w", err)
	}
	commit := strings.TrimSpace(string(raw))
	decoded, err := hex.DecodeString(commit)
	if err != nil || (len(decoded) != 20 && len(decoded) != 32) {
		return "", fmt.Errorf("source snapshot commit identity is invalid")
	}
	return commit, nil
}

type immutableGitFile struct{ mode, oid, path string }

func immutableCommitInventory(ctx context.Context, source, commit string) ([]immutableGitFile, error) {
	raw, err := exec.CommandContext(ctx, "git", "-C", source, "ls-tree", "-rz", "-r", "--full-tree", commit).Output()
	if err != nil {
		return nil, fmt.Errorf("source snapshot tree inventory: %w", err)
	}
	var tracked []immutableGitFile
	for _, record := range strings.Split(string(raw), "\x00") {
		if record == "" {
			continue
		}
		tab := strings.IndexByte(record, '\t')
		if tab <= 0 {
			return nil, fmt.Errorf("source snapshot malformed tree inventory")
		}
		fields, path := strings.Fields(record[:tab]), record[tab+1:]
		clean := filepath.Clean(filepath.FromSlash(path))
		if len(fields) != 3 || fields[1] != "blob" || (fields[0] != "100644" && fields[0] != "100755" && fields[0] != "120000") || clean == "." || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) {
			return nil, fmt.Errorf("source snapshot refuses tree path %q", path)
		}
		decodedOID, decodeErr := hex.DecodeString(fields[2])
		if decodeErr != nil || (len(decodedOID) != 20 && len(decodedOID) != 32) {
			return nil, fmt.Errorf("source snapshot refuses tree object for %q", path)
		}
		tracked = append(tracked, immutableGitFile{fields[0], fields[2], clean})
	}
	sort.Slice(tracked, func(i, j int) bool { return tracked[i].path < tracked[j].path })
	if len(tracked) == 0 {
		return nil, fmt.Errorf("source snapshot tree inventory is empty")
	}
	return tracked, nil
}

func readGitBlob(ctx context.Context, source, oid string, consume func(io.Reader) error) error {
	command := exec.CommandContext(ctx, "git", "-C", source, "cat-file", "blob", oid)
	input, err := command.StdoutPipe()
	if err != nil {
		return err
	}
	if err := command.Start(); err != nil {
		return err
	}
	consumeErr := consume(&contextReader{ctx: ctx, reader: input})
	waitErr := command.Wait()
	if contextErr := ctx.Err(); contextErr != nil {
		return errors.Join(contextErr, consumeErr, waitErr)
	}
	return errors.Join(consumeErr, waitErr)
}

func canonicalImmutableCommitDigest(ctx context.Context, source, commit string) (string, error) {
	tracked, err := immutableCommitInventory(ctx, source, commit)
	if err != nil {
		return "", err
	}
	directories := map[string]bool{}
	entries := make([]canonicalTreeEntry, 0, len(tracked)*2)
	for _, file := range tracked {
		for dir := filepath.Dir(file.path); dir != "."; dir = filepath.Dir(dir) {
			directories[filepath.ToSlash(dir)] = true
		}
		item := canonicalTreeEntry{Path: filepath.ToSlash(file.path)}
		if file.mode == "120000" {
			item.Type, item.Mode = "symlink", 0o777
			var target strings.Builder
			if err := readGitBlob(ctx, source, file.oid, func(r io.Reader) error { _, err := io.Copy(&target, r); return err }); err != nil {
				return "", err
			}
			link := target.String()
			cleanLink := filepath.Clean(link)
			if (filepath.IsAbs(cleanLink) && !strings.HasPrefix(cleanLink, "/nix/store/")) || (!filepath.IsAbs(cleanLink) && (cleanLink == ".." || strings.HasPrefix(cleanLink, ".."+string(os.PathSeparator)))) {
				return "", fmt.Errorf("source snapshot refuses escaping symlink %q", file.path)
			}
			digest := sha256.Sum256([]byte(link))
			item.ContentDigest = hex.EncodeToString(digest[:])
		} else {
			item.Type, item.Mode = "file", 0o644
			if file.mode == "100755" {
				item.Mode = 0o755
			}
			hash := sha256.New()
			if err := readGitBlob(ctx, source, file.oid, func(r io.Reader) error { _, err := io.Copy(hash, r); return err }); err != nil {
				return "", err
			}
			item.ContentDigest = hex.EncodeToString(hash.Sum(nil))
		}
		entries = append(entries, item)
	}
	for dir := range directories {
		entries = append(entries, canonicalTreeEntry{Path: dir, Type: "directory", Mode: 0o755})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	canonical, err := json.Marshal(entries)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(canonical)
	return hex.EncodeToString(digest[:]), nil
}

func copyImmutableSourceTree(ctx context.Context, source, target string) (string, error) {
	source, target = filepath.Clean(source), filepath.Clean(target)
	if !filepath.IsAbs(source) || !filepath.IsAbs(target) || strings.Contains(source, ":") || strings.Contains(target, ":") {
		return "", fmt.Errorf("source snapshot requires absolute colon-free paths")
	}
	commit, err := immutableGitCommitIdentity(ctx, source)
	if err != nil {
		return "", err
	}
	return copyImmutableCommitTree(ctx, source, commit, target)
}

func copyImmutableCommitTree(ctx context.Context, source, commit, target string) (string, error) {
	tracked, err := immutableCommitInventory(ctx, source, commit)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(target, 0o755); err != nil {
		return "", err
	}
	for _, file := range tracked {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		targetPath := filepath.Join(target, file.path)
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return "", err
		}
		if file.mode == "120000" {
			var targetValue strings.Builder
			if err := readGitBlob(ctx, source, file.oid, func(r io.Reader) error { _, err := io.Copy(&targetValue, r); return err }); err != nil {
				return "", err
			}
			link, cleanLink := targetValue.String(), filepath.Clean(targetValue.String())
			if (filepath.IsAbs(cleanLink) && !strings.HasPrefix(cleanLink, "/nix/store/")) || (!filepath.IsAbs(cleanLink) && (cleanLink == ".." || strings.HasPrefix(cleanLink, ".."+string(os.PathSeparator)))) {
				return "", fmt.Errorf("source snapshot refuses escaping symlink %q", file.path)
			}
			if err := os.Symlink(link, targetPath); err != nil {
				return "", err
			}
			continue
		}
		mode := os.FileMode(0o644)
		if file.mode == "100755" {
			mode = 0o755
		}
		output, err := os.OpenFile(targetPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
		if err != nil {
			return "", err
		}
		copyErr := readGitBlob(ctx, source, file.oid, func(r io.Reader) error { _, err := io.Copy(output, r); return err })
		closeErr := output.Close()
		if copyErr != nil || closeErr != nil {
			return "", errors.Join(copyErr, closeErr)
		}
	}
	digest, err := digestCanonicalSubjectTree(ctx, target)
	if err != nil {
		return "", err
	}
	expected, err := canonicalImmutableCommitDigest(ctx, source, commit)
	if err != nil || expected != digest {
		return "", fmt.Errorf("source snapshot canonical digest mismatch")
	}
	if err := makeSubjectTreeReadOnly(target); err != nil {
		return "", err
	}
	return digest, nil
}

func requireFrozenComputerSurfaceSource(root string) error {
	index := filepath.Join(filepath.Clean(root), "frontend", "index.html")
	info, err := os.Stat(index)
	if err != nil || info.IsDir() {
		return fmt.Errorf("capsule freeze: computer-surface frontend source is underivable")
	}
	return nil
}
