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
	rawCommit, err := exec.CommandContext(ctx, "git", "-C", source, "rev-parse", "--verify", "HEAD^{commit}").Output()
	if err != nil {
		if contextErr := ctx.Err(); contextErr != nil {
			return "", contextErr
		}
		return "", fmt.Errorf("source snapshot commit identity: %w", err)
	}
	commit := strings.TrimSpace(string(rawCommit))
	decodedCommit, decodeErr := hex.DecodeString(commit)
	if decodeErr != nil || (len(decodedCommit) != 20 && len(decodedCommit) != 32) {
		return "", fmt.Errorf("source snapshot commit identity is invalid")
	}
	return copyImmutableCommitTree(ctx, source, commit, target)
}

func copyImmutableCommitTree(ctx context.Context, source, commit, target string) (string, error) {
	raw, err := exec.CommandContext(ctx, "git", "-C", source, "ls-tree", "-rz", "--full-tree", commit).Output()
	if err != nil {
		if contextErr := ctx.Err(); contextErr != nil {
			return "", contextErr
		}
		return "", fmt.Errorf("source snapshot tree inventory: %w", err)
	}
	type trackedFile struct {
		mode string
		oid  string
		path string
	}
	var tracked []trackedFile
	for _, record := range strings.Split(string(raw), "\x00") {
		if record == "" {
			continue
		}
		tab := strings.IndexByte(record, '\t')
		if tab <= 0 {
			return "", fmt.Errorf("source snapshot malformed tree inventory")
		}
		fields := strings.Fields(record[:tab])
		path := record[tab+1:]
		clean := filepath.Clean(filepath.FromSlash(path))
		if len(fields) != 3 || fields[1] != "blob" ||
			(fields[0] != "100644" && fields[0] != "100755" && fields[0] != "120000") ||
			clean == "." || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) {
			return "", fmt.Errorf("source snapshot refuses tree path %q", path)
		}
		decodedOID, decodeErr := hex.DecodeString(fields[2])
		if decodeErr != nil || (len(decodedOID) != 20 && len(decodedOID) != 32) {
			return "", fmt.Errorf("source snapshot refuses tree object for %q", path)
		}
		tracked = append(tracked, trackedFile{mode: fields[0], oid: fields[2], path: clean})
	}
	sort.Slice(tracked, func(i, j int) bool { return tracked[i].path < tracked[j].path })
	if len(tracked) == 0 {
		return "", fmt.Errorf("source snapshot tree inventory is empty")
	}
	if err := os.MkdirAll(target, 0o755); err != nil {
		return "", err
	}
	hash := sha256.New()
	_, _ = fmt.Fprintf(hash, "commit\x00%s\x00", commit)
	directories := map[string]bool{target: true}
	for _, file := range tracked {
		if err := ctx.Err(); err != nil {
			return "", err
		}
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
		_, _ = fmt.Fprintf(hash, "%s\x00%s\x00%s\x00", file.mode, filepath.ToSlash(file.path), file.oid)
		if file.mode == "120000" {
			rawLink, err := exec.CommandContext(ctx, "git", "-C", source, "cat-file", "blob", file.oid).Output()
			if err != nil {
				if contextErr := ctx.Err(); contextErr != nil {
					return "", contextErr
				}
				return "", fmt.Errorf("source snapshot read symlink object %q: %w", file.path, err)
			}
			link := string(rawLink)
			cleanLink := filepath.Clean(link)
			if (filepath.IsAbs(cleanLink) && !strings.HasPrefix(cleanLink, "/nix/store/")) ||
				(!filepath.IsAbs(cleanLink) && (cleanLink == ".." || strings.HasPrefix(cleanLink, ".."+string(os.PathSeparator)))) {
				return "", fmt.Errorf("source snapshot refuses escaping symlink %q", file.path)
			}
			if err := os.Symlink(link, targetPath); err != nil {
				return "", err
			}
			continue
		}
		mode := os.FileMode(0o444)
		if file.mode == "100755" {
			mode = 0o555
		}
		output, err := os.OpenFile(targetPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
		if err != nil {
			return "", err
		}
		command := exec.CommandContext(ctx, "git", "-C", source, "cat-file", "blob", file.oid)
		input, err := command.StdoutPipe()
		if err != nil {
			_ = output.Close()
			return "", err
		}
		if err := command.Start(); err != nil {
			_ = output.Close()
			return "", err
		}
		_, copyErr := io.Copy(output, &contextReader{ctx: ctx, reader: input})
		waitErr := command.Wait()
		closeErr := output.Close()
		if copyErr != nil || waitErr != nil || closeErr != nil {
			if contextErr := ctx.Err(); contextErr != nil {
				return "", errors.Join(contextErr, copyErr, waitErr, closeErr)
			}
			return "", errors.Join(copyErr, waitErr, closeErr)
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
