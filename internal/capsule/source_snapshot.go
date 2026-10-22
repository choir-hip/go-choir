package capsule

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
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

// symlinkEscapesTree reports whether a symlink at relativePath (relative to
// the tree root) points outside the tree: an absolute target other than a
// /nix/store path, or a relative target that, resolved against the link's
// own directory, climbs above the root. A relative target is judged where it
// lands, not by its spelling: frontend/node_modules/.bin/acorn ->
// ../acorn/bin/acorn stays inside the tree.
func symlinkEscapesTree(relativePath, link string) bool {
	cleanLink := filepath.Clean(link)
	if filepath.IsAbs(cleanLink) {
		return !strings.HasPrefix(cleanLink, "/nix/store/")
	}
	resolved := filepath.Clean(filepath.Join(filepath.Dir(filepath.Clean(relativePath)), cleanLink))
	return resolved == ".." || strings.HasPrefix(resolved, ".."+string(os.PathSeparator)) || filepath.IsAbs(resolved)
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
			if symlinkEscapesTree(relative, link) {
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
		// Overlayfs checks the lower parent directory with the capsule's
		// namespace-mapped UID before it can copy up a new artifact. Keep
		// lower files read-only, but allow directory traversal and copy-up
		// creation; the lower tree is never exposed as a writable path.
		if err := os.Chmod(dir, 0o777); err != nil {
			return err
		}
	}
	return nil
}

func immutableGitCommitIdentity(ctx context.Context, source string) (string, error) {
	for _, args := range [][]string{{"diff", "--quiet", "--"}, {"diff", "--cached", "--quiet", "--"}} {
		command := exec.CommandContext(ctx, "git", append([]string{"-C", source}, args...)...)
		output, err := command.CombinedOutput()
		if err != nil {
			if contextErr := ctx.Err(); contextErr != nil {
				return "", contextErr
			}
			if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
				return "", fmt.Errorf("source snapshot refuses dirty tracked files")
			}
			return "", fmt.Errorf("source snapshot git %s failed (exit %v): %s (%w)", strings.Join(args, " "), err, strings.TrimSpace(string(output)), err)
		}
	}
	raw, err := exec.CommandContext(ctx, "git", "-C", source, "rev-parse", "--verify", "HEAD^{commit}").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("source snapshot commit identity (%s): %w", strings.TrimSpace(string(raw)), err)
	}
	commit := strings.TrimSpace(string(raw))
	decoded, err := hex.DecodeString(commit)
	if err != nil || (len(decoded) != 20 && len(decoded) != 32) {
		return "", fmt.Errorf("source snapshot commit identity is invalid: %q", commit)
	}
	return commit, nil
}

type immutableGitFile struct{ mode, oid, path string }

func immutableCommitInventory(ctx context.Context, source, commit string) ([]immutableGitFile, error) {
	raw, err := exec.CommandContext(ctx, "git", "-C", source, "ls-tree", "-rz", "-r", "--full-tree", commit).Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && len(exitErr.Stderr) > 0 {
			return nil, fmt.Errorf("source snapshot tree inventory: %w: %s", err, strings.TrimSpace(string(exitErr.Stderr)))
		}
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

// readCommitBlobs reads every blob of tracked, in order, through one
// `git cat-file --batch` process, and hands each to visit as a reader of
// exactly its bytes. It fails closed: a missing or non-blob object, a short
// read, a malformed answer, bytes that do not hash to their object id, or a
// git error ends the read with an error, and the caller must discard
// whatever visit has built (docs/problems/capsule-coldstart-18s-2026-10-22.md,
// option 1: one process per pass, not one per file).
func readCommitBlobs(ctx context.Context, source string, tracked []immutableGitFile, visit func(immutableGitFile, io.Reader) error) error {
	command := exec.CommandContext(ctx, "git", "-C", source, "cat-file", "--batch", "--buffer")
	stdin, err := command.StdinPipe()
	if err != nil {
		return err
	}
	stdout, err := command.StdoutPipe()
	if err != nil {
		return err
	}
	var stderr bytes.Buffer
	command.Stderr = &stderr
	if err := command.Start(); err != nil {
		return err
	}
	written := make(chan error, 1)
	go func() {
		requests := bufio.NewWriter(stdin)
		var writeErr error
		for _, file := range tracked {
			if _, writeErr = requests.WriteString(file.oid + "\n"); writeErr != nil {
				break
			}
		}
		if writeErr == nil {
			writeErr = requests.Flush()
		}
		written <- errors.Join(writeErr, stdin.Close())
	}()
	parseErr := parseBlobBatch(&contextReader{ctx: ctx, reader: stdout}, tracked, visit)
	if parseErr != nil {
		// git may be blocked writing answers nobody will read.
		_ = command.Process.Kill()
	}
	waitErr := command.Wait()
	writeErr := <-written
	if contextErr := ctx.Err(); contextErr != nil {
		return errors.Join(contextErr, parseErr)
	}
	if parseErr != nil {
		return parseErr
	}
	if waitErr != nil || writeErr != nil {
		return fmt.Errorf("source snapshot git cat-file --batch: %s: %w", strings.TrimSpace(stderr.String()), errors.Join(waitErr, writeErr))
	}
	return nil
}

// parseBlobBatch reads the answers of `git cat-file --batch` to the requests
// tracked (one full object id per line, in order): for each, a header
// "<oid> blob <size>\n", exactly size bytes, then "\n"; and nothing after the
// last. Each blob's bytes are hashed in git's object format and must give
// back the requested id, so an object store that answers other bytes is
// refused, which the per-file path never checked.
func parseBlobBatch(r io.Reader, tracked []immutableGitFile, visit func(immutableGitFile, io.Reader) error) error {
	answers := bufio.NewReaderSize(r, 64<<10)
	for _, file := range tracked {
		header, err := answers.ReadString('\n')
		if err != nil {
			if errors.Is(err, io.EOF) {
				err = io.ErrUnexpectedEOF
			}
			return fmt.Errorf("source snapshot batch has no answer for %q: %w", file.path, err)
		}
		fields := strings.Split(strings.TrimSuffix(header, "\n"), " ")
		if len(fields) != 3 || fields[0] != file.oid || fields[1] != "blob" {
			return fmt.Errorf("source snapshot batch refuses answer %q for blob %s (%q)", strings.TrimSpace(header), file.oid, file.path)
		}
		size, err := strconv.ParseInt(fields[2], 10, 64)
		if err != nil || size < 0 {
			return fmt.Errorf("source snapshot batch refuses size %q for %q", fields[2], file.path)
		}
		blob, err := newVerifiedBlob(answers, file.oid, size)
		if err != nil {
			return err
		}
		if err := visit(file, blob); err != nil {
			return err
		}
		if err := blob.finish(); err != nil {
			return fmt.Errorf("source snapshot batch blob %q: %w", file.path, err)
		}
		if terminator, err := answers.ReadByte(); err != nil || terminator != '\n' {
			return fmt.Errorf("source snapshot batch answer for %q is not terminated", file.path)
		}
	}
	if _, err := answers.ReadByte(); !errors.Is(err, io.EOF) {
		if err == nil {
			return fmt.Errorf("source snapshot batch has bytes after the last answer")
		}
		return err
	}
	return nil
}

// verifiedBlob reads exactly one blob's bytes and, at their end, reports
// io.EOF only if they hash to the object id; otherwise the error, so a
// visitor's io.Copy never ends cleanly on wrong bytes.
type verifiedBlob struct {
	reader    io.Reader
	remaining int64
	hash      hash.Hash
	oid       string
	verified  bool
	verdict   error
}

func newVerifiedBlob(reader io.Reader, oid string, size int64) (*verifiedBlob, error) {
	var objectHash hash.Hash
	switch len(oid) {
	case 40:
		objectHash = sha1.New()
	case 64:
		objectHash = sha256.New()
	default:
		return nil, fmt.Errorf("source snapshot refuses object id %q", oid)
	}
	fmt.Fprintf(objectHash, "blob %d\x00", size)
	return &verifiedBlob{reader: reader, remaining: size, hash: objectHash, oid: oid}, nil
}

func (b *verifiedBlob) Read(buffer []byte) (int, error) {
	if b.remaining == 0 {
		return 0, b.verify()
	}
	if int64(len(buffer)) > b.remaining {
		buffer = buffer[:b.remaining]
	}
	n, err := b.reader.Read(buffer)
	b.hash.Write(buffer[:n])
	b.remaining -= int64(n)
	if errors.Is(err, io.EOF) && b.remaining > 0 {
		return n, io.ErrUnexpectedEOF
	}
	if err != nil && !errors.Is(err, io.EOF) {
		return n, err
	}
	return n, nil
}

func (b *verifiedBlob) verify() error {
	if !b.verified {
		b.verified = true
		b.verdict = io.EOF
		if hex.EncodeToString(b.hash.Sum(nil)) != b.oid {
			b.verdict = fmt.Errorf("source snapshot object %s does not hash to its id", b.oid)
		}
	}
	return b.verdict
}

// finish reads what the visitor left of the blob and returns its verdict.
func (b *verifiedBlob) finish() error {
	if _, err := io.Copy(io.Discard, b); err != nil {
		return err
	}
	if b.remaining != 0 {
		return io.ErrUnexpectedEOF
	}
	if err := b.verify(); !errors.Is(err, io.EOF) {
		return err
	}
	return nil
}

func canonicalImmutableCommitDigest(ctx context.Context, source, commit string) (string, error) {
	tracked, err := immutableCommitInventory(ctx, source, commit)
	if err != nil {
		return "", err
	}
	directories := map[string]bool{}
	entries := make([]canonicalTreeEntry, 0, len(tracked)*2)
	err = readCommitBlobs(ctx, source, tracked, func(file immutableGitFile, blob io.Reader) error {
		for dir := filepath.Dir(file.path); dir != "."; dir = filepath.Dir(dir) {
			directories[filepath.ToSlash(dir)] = true
		}
		item := canonicalTreeEntry{Path: filepath.ToSlash(file.path)}
		if file.mode == "120000" {
			item.Type, item.Mode = "symlink", 0o777
			raw, err := io.ReadAll(blob)
			if err != nil {
				return err
			}
			link := string(raw)
			if symlinkEscapesTree(file.path, link) {
				return fmt.Errorf("source snapshot refuses escaping symlink %q", file.path)
			}
			digest := sha256.Sum256(raw)
			item.ContentDigest = hex.EncodeToString(digest[:])
		} else {
			item.Type, item.Mode = "file", 0o644
			if file.mode == "100755" {
				item.Mode = 0o755
			}
			hash := sha256.New()
			if _, err := io.Copy(hash, blob); err != nil {
				return err
			}
			item.ContentDigest = hex.EncodeToString(hash.Sum(nil))
		}
		entries = append(entries, item)
		return nil
	})
	if err != nil {
		return "", err
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
	return copyImmutableCommitTreeTimed(ctx, source, commit, target, "", nil)
}

// copyImmutableCommitTreeTimed copies commit's tree to target through one
// `git cat-file --batch`, hashes the copy in-process, and refuses it unless
// that digest equals expected. A caller holding the digest the binding pins
// passes it as expected, and the commit is not read a second time
// (option 2: the copy's own digest against the binding is the check). With
// expected empty, the commit's digest is derived again (one more batch).
// On any refusal, a target this call created is removed: never a partial
// tree. timer logs a line per pass (nil logs nothing).
func copyImmutableCommitTreeTimed(ctx context.Context, source, commit, target, expected string, timer *spawnPhaseTimer) (_ string, retErr error) {
	tracked, err := immutableCommitInventory(ctx, source, commit)
	if err != nil {
		return "", err
	}
	timer.phase("source_inventory", fmt.Sprintf("files=%d", len(tracked)))
	if _, err := os.Lstat(target); errors.Is(err, os.ErrNotExist) {
		defer func() {
			if retErr != nil {
				retErr = errors.Join(retErr, os.RemoveAll(target))
			}
		}()
	} else if err != nil {
		return "", err
	}
	if err := os.MkdirAll(target, 0o755); err != nil {
		return "", err
	}
	err = readCommitBlobs(ctx, source, tracked, func(file immutableGitFile, blob io.Reader) error {
		targetPath := filepath.Join(target, file.path)
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return err
		}
		if file.mode == "120000" {
			raw, err := io.ReadAll(blob)
			if err != nil {
				return err
			}
			link := string(raw)
			if symlinkEscapesTree(file.path, link) {
				return fmt.Errorf("source snapshot refuses escaping symlink %q", file.path)
			}
			return os.Symlink(link, targetPath)
		}
		mode := os.FileMode(0o644)
		if file.mode == "100755" {
			mode = 0o755
		}
		output, err := os.OpenFile(targetPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(output, blob)
		return errors.Join(copyErr, output.Close())
	})
	if err != nil {
		return "", err
	}
	timer.phase("source_blobs", fmt.Sprintf("files=%d", len(tracked)), "git_procs=1")
	digest, err := digestCanonicalSubjectTree(ctx, target)
	if err != nil {
		return "", err
	}
	timer.phase("source_digest_copy")
	commitPass := []string{fmt.Sprintf("files=%d", len(tracked)), "git_procs=0", "against=expected"}
	if expected == "" {
		if expected, err = canonicalImmutableCommitDigest(ctx, source, commit); err != nil {
			return "", fmt.Errorf("source snapshot canonical digest mismatch: commit %s: %w", commit, err)
		}
		commitPass = []string{fmt.Sprintf("files=%d", len(tracked)), "git_procs=1"}
	}
	if expected != digest {
		return "", fmt.Errorf("source snapshot canonical digest mismatch: expected %s, copied %s", expected, digest)
	}
	timer.phase("source_digest_commit", commitPass...)
	if err := makeSubjectTreeReadOnly(target); err != nil {
		return "", err
	}
	timer.phase("source_read_only")
	return digest, nil
}

// spawnSourceRefusal is Spawn's verdict on its source copy: nil only when
// the copy succeeded and its digest is the one the binding pins. A refusal
// wraps the copy's cause and names the digests, so an object-id mismatch, a
// missing object, a git exit, a deadline and a digest mismatch read
// differently in the logs and in the cancelled assignment's reason.
func spawnSourceRefusal(copyErr error, expected, copied string) error {
	if copyErr != nil {
		return fmt.Errorf("capsule source changed after durable preflight (expected %s): %w", expected, copyErr)
	}
	if copied != expected {
		return fmt.Errorf("capsule source changed after durable preflight: expected %s, copied %s", expected, copied)
	}
	return nil
}

func requireFrozenComputerSurfaceSource(root string) error {
	index := filepath.Join(filepath.Clean(root), "frontend", "index.html")
	info, err := os.Stat(index)
	if err != nil || info.IsDir() {
		return fmt.Errorf("capsule freeze: computer-surface frontend source is underivable")
	}
	return nil
}
