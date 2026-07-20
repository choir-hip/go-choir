package capsule

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

func copyImmutableSourceTree(ctx context.Context, source, target string) (string, error) {
	source, target = filepath.Clean(source), filepath.Clean(target)
	if !filepath.IsAbs(source) || !filepath.IsAbs(target) || strings.Contains(source, ":") || strings.Contains(target, ":") {
		return "", fmt.Errorf("source snapshot requires absolute colon-free paths")
	}
	for _, args := range [][]string{{"diff", "--quiet", "--"}, {"diff", "--cached", "--quiet", "--"}} {
		command := exec.CommandContext(ctx, "git", append([]string{"-C", source}, args...)...)
		if err := command.Run(); err != nil {
			if contextErr := ctx.Err(); contextErr != nil {
				return "", contextErr
			}
			return "", fmt.Errorf("source snapshot refuses dirty tracked files")
		}
	}
	raw, err := exec.CommandContext(ctx, "git", "-C", source, "ls-files", "-z", "--stage").Output()
	if err != nil {
		if contextErr := ctx.Err(); contextErr != nil {
			return "", contextErr
		}
		return "", fmt.Errorf("source snapshot tracked inventory: %w", err)
	}
	type trackedFile struct {
		mode string
		path string
	}
	var tracked []trackedFile
	for _, record := range strings.Split(string(raw), "\x00") {
		if record == "" {
			continue
		}
		tab := strings.IndexByte(record, '\t')
		if tab <= 0 {
			return "", fmt.Errorf("source snapshot malformed tracked inventory")
		}
		fields := strings.Fields(record[:tab])
		path := record[tab+1:]
		clean := filepath.Clean(filepath.FromSlash(path))
		if len(fields) != 3 || (fields[0] != "100644" && fields[0] != "100755" && fields[0] != "120000") ||
			clean == "." || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) {
			return "", fmt.Errorf("source snapshot refuses tracked path %q", path)
		}
		tracked = append(tracked, trackedFile{mode: fields[0], path: clean})
	}
	sort.Slice(tracked, func(i, j int) bool { return tracked[i].path < tracked[j].path })
	if len(tracked) == 0 {
		return "", fmt.Errorf("source snapshot tracked inventory is empty")
	}
	if err := os.MkdirAll(target, 0o755); err != nil {
		return "", err
	}
	hash := sha256.New()
	directories := map[string]bool{target: true}
	for _, file := range tracked {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		sourcePath := filepath.Join(source, file.path)
		targetPath := filepath.Join(target, file.path)
		parent := filepath.Dir(targetPath)
		if err := os.MkdirAll(parent, 0o755); err != nil {
			return "", err
		}
		for dir := parent; strings.HasPrefix(dir, target); dir = filepath.Dir(dir) {
			directories[dir] = true
			if dir == target {
				break
			}
		}
		info, err := os.Lstat(sourcePath)
		if err != nil {
			return "", err
		}
		_, _ = fmt.Fprintf(hash, "%s\x00%s\x00%d\x00", file.mode, filepath.ToSlash(file.path), info.Size())
		if file.mode == "120000" {
			if info.Mode()&os.ModeSymlink == 0 {
				return "", fmt.Errorf("source snapshot tracked symlink changed type: %q", file.path)
			}
			link, err := os.Readlink(sourcePath)
			cleanLink := filepath.Clean(link)
			if err != nil || (filepath.IsAbs(cleanLink) && !strings.HasPrefix(cleanLink, "/nix/store/")) ||
				(!filepath.IsAbs(cleanLink) && (cleanLink == ".." || strings.HasPrefix(cleanLink, ".."+string(os.PathSeparator)))) {
				return "", fmt.Errorf("source snapshot refuses escaping symlink %q", file.path)
			}
			if err := os.Symlink(link, targetPath); err != nil {
				return "", err
			}
			_, _ = io.WriteString(hash, link+"\x00")
			continue
		}
		if !info.Mode().IsRegular() {
			return "", fmt.Errorf("source snapshot tracked file changed type: %q", file.path)
		}
		input, err := os.Open(sourcePath)
		if err != nil {
			return "", err
		}
		mode := os.FileMode(0o444)
		if file.mode == "100755" {
			mode = 0o555
		}
		output, err := os.OpenFile(targetPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
		if err != nil {
			input.Close()
			return "", err
		}
		_, copyErr := io.Copy(io.MultiWriter(output, hash), &contextReader{ctx: ctx, reader: input})
		closeInputErr, closeOutputErr := input.Close(), output.Close()
		if copyErr != nil || closeInputErr != nil || closeOutputErr != nil {
			return "", errors.Join(copyErr, closeInputErr, closeOutputErr)
		}
	}
	orderedDirectories := make([]string, 0, len(directories))
	for dir := range directories {
		orderedDirectories = append(orderedDirectories, dir)
	}
	sort.Slice(orderedDirectories, func(i, j int) bool { return len(orderedDirectories[i]) > len(orderedDirectories[j]) })
	for _, dir := range orderedDirectories {
		if err := os.Chmod(dir, 0o555); err != nil {
			return "", err
		}
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
