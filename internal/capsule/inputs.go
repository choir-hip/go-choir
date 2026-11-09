package capsule

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// Inputs: the evidence a management Cast names for an engineering
// assignment (research's captured data), delivered to the capsule as
// read-only files. The capsule's egress allowlist rightly does not reach
// public data hosts (research is the world-evidence desk), so the host
// fetches the named bytes before spawn (StageInputs) and Spawn lays them in
// the source lower at InputsDir beside a manifest.json that lists each file's
// source ref, media type, sha256 and the URL it was captured from, and each
// ref it refused with the reason. A missing or refused ref is a visible
// manifest entry, never a failed spawn. Nothing is copied from a store: only
// the named bodies, bounded, and a hardlink where the staging directory
// shares the lower's filesystem.
const (
	// InputsDir is where an assignment finds its inputs.
	InputsDir = "/var/lib/artifact/inputs"
	// InputsPrefix is InputsDir as a capsule path without the leading slash.
	InputsPrefix = "var/lib/artifact/inputs/"
	// InputsManifestName is the manifest's file name inside InputsDir.
	InputsManifestName = "manifest.json"

	// MaxInputs bounds the files one assignment receives.
	MaxInputs = 20
	// MaxInputBytes bounds one file.
	MaxInputBytes = 50 << 20
	// MaxInputsTotalBytes bounds the set.
	MaxInputsTotalBytes = 200 << 20
	// MaxInputEntries bounds the refs examined; the rest are counted.
	MaxInputEntries = 100
	// MaxInputRefusals bounds the refusals the manifest lists.
	MaxInputRefusals = 20
)

// InputFile is one input the capsule received.
type InputFile struct {
	Name      string `json:"name"`
	Source    string `json:"source"`
	MediaType string `json:"media_type"`
	SHA256    string `json:"sha256"`
	Bytes     int64  `json:"bytes"`
	URL       string `json:"url,omitempty"`
}

// InputRefusal is one named ref the host did not deliver, with the reason.
type InputRefusal struct {
	Source string `json:"source"`
	Reason string `json:"reason"`
}

// InputsManifest is manifest.json.
type InputsManifest struct {
	Files   []InputFile    `json:"files"`
	Refused []InputRefusal `json:"refused"`
}

// InputSource is one named ref, resolved by the host. A non-empty Refusal
// says why it cannot be delivered (not found, owner-private, not a capture);
// otherwise Open reads its bytes. Size is the declared size, or -1.
type InputSource struct {
	Ref       string
	Name      string
	URL       string
	MediaType string
	Size      int64
	Refusal   string
	Open      func(ctx context.Context) (io.ReadCloser, error)
}

// StageInputs writes the sources into dir (created) with the bounds above,
// one temp file and rename each, and then manifest.json. It never fails for a
// ref it cannot deliver: that is a manifest refusal. Only a host write error
// or a cancelled context fails it.
func StageInputs(ctx context.Context, items []InputSource, dir string) (InputsManifest, error) {
	manifest := InputsManifest{Files: []InputFile{}, Refused: []InputRefusal{}}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return manifest, err
	}
	var total int64
	used := map[string]bool{strings.ToLower(InputsManifestName): true}
	refuse := func(ref, reason string) {
		manifest.Refused = append(manifest.Refused, InputRefusal{Source: ref, Reason: reason})
	}
	for i, it := range items {
		if err := ctx.Err(); err != nil {
			return manifest, err
		}
		if i >= MaxInputEntries {
			refuse(fmt.Sprintf("%d more refs", len(items)-i), fmt.Sprintf("not examined: only %d refs are", MaxInputEntries))
			break
		}
		if it.Refusal != "" {
			refuse(it.Ref, it.Refusal)
			continue
		}
		name, err := SanitizeDeliverableName(it.Name)
		if err != nil {
			refuse(it.Ref, err.Error())
			continue
		}
		if len(manifest.Files) >= MaxInputs {
			refuse(it.Ref, fmt.Sprintf("over the %d-file limit", MaxInputs))
			continue
		}
		if used[strings.ToLower(name)] {
			refuse(it.Ref, "another input has the same name after sanitizing")
			continue
		}
		if it.Size > MaxInputBytes {
			refuse(it.Ref, fmt.Sprintf("larger than %d MB", MaxInputBytes>>20))
			continue
		}
		if it.Size >= 0 && total+it.Size > MaxInputsTotalBytes {
			refuse(it.Ref, fmt.Sprintf("the set would exceed %d MB", MaxInputsTotalBytes>>20))
			continue
		}
		if it.Open == nil {
			refuse(it.Ref, "no bytes to read")
			continue
		}
		file, reason, err := stageInputFile(ctx, it, name, dir, MaxInputsTotalBytes-total)
		if err != nil {
			return manifest, err
		}
		if reason != "" {
			refuse(it.Ref, reason)
			continue
		}
		used[strings.ToLower(name)] = true
		total += file.Bytes
		manifest.Files = append(manifest.Files, file)
	}
	if len(manifest.Refused) > MaxInputRefusals {
		more := len(manifest.Refused) - MaxInputRefusals
		manifest.Refused = append(manifest.Refused[:MaxInputRefusals], InputRefusal{Source: fmt.Sprintf("%d more refs", more), Reason: "refused; not listed"})
	}
	raw, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return manifest, err
	}
	return manifest, writeStagedDeliverable(filepath.Join(dir, InputsManifestName), append(raw, '\n'))
}

// stageInputFile streams one source into dir/name, hashing as it goes. A
// refusal is a reason; only a host write error is an error.
func stageInputFile(ctx context.Context, it InputSource, name, dir string, room int64) (InputFile, string, error) {
	in, err := it.Open(ctx)
	if err != nil {
		return InputFile{}, "could not be read: " + err.Error(), nil
	}
	defer in.Close()
	tmp, err := os.CreateTemp(dir, ".stage-*")
	if err != nil {
		return InputFile{}, "", err
	}
	cleanup := func() { _ = tmp.Close(); _ = os.Remove(tmp.Name()) }
	limit := int64(MaxInputBytes)
	if room < limit {
		limit = room
	}
	sum := sha256.New()
	head := make([]byte, 0, 512)
	n, copyErr := io.Copy(io.MultiWriter(tmp, sum, &headWriter{buf: &head}), io.LimitReader(in, limit+1))
	if copyErr != nil {
		cleanup()
		if ctx.Err() != nil {
			return InputFile{}, "", ctx.Err()
		}
		return InputFile{}, "read failed: " + copyErr.Error(), nil
	}
	switch {
	case n > MaxInputBytes:
		cleanup()
		return InputFile{}, fmt.Sprintf("larger than %d MB", MaxInputBytes>>20), nil
	case n > limit:
		cleanup()
		return InputFile{}, fmt.Sprintf("the set would exceed %d MB", MaxInputsTotalBytes>>20), nil
	case n == 0:
		cleanup()
		return InputFile{}, "empty", nil
	}
	if err := errors.Join(tmp.Chmod(0o600), tmp.Sync(), tmp.Close()); err != nil {
		_ = os.Remove(tmp.Name())
		return InputFile{}, "", err
	}
	if err := os.Rename(tmp.Name(), filepath.Join(dir, name)); err != nil {
		_ = os.Remove(tmp.Name())
		return InputFile{}, "", err
	}
	media := strings.TrimSpace(it.MediaType)
	if media == "" {
		media = http.DetectContentType(head)
	}
	return InputFile{Name: name, Source: it.Ref, MediaType: media, SHA256: hex.EncodeToString(sum.Sum(nil)), Bytes: n, URL: it.URL}, "", nil
}

// headWriter keeps the first 512 bytes written.
type headWriter struct{ buf *[]byte }

func (w *headWriter) Write(p []byte) (int, error) {
	if room := 512 - len(*w.buf); room > 0 {
		*w.buf = append(*w.buf, p[:min(room, len(p))]...)
	}
	return len(p), nil
}

// materializeInputs lays the staged files into dst (the lower's InputsDir):
// a hardlink where src and dst share a filesystem, a copy otherwise. Only
// regular, non-dot files directly in src are taken. It returns the counts
// and bytes.
func materializeInputs(ctx context.Context, src, dst string) (linked, copied int, bytes int64, err error) {
	entries, err := os.ReadDir(src)
	if err != nil {
		return 0, 0, 0, err
	}
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return 0, 0, 0, err
	}
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return linked, copied, bytes, err
		}
		from := filepath.Join(src, entry.Name())
		info, err := os.Lstat(from)
		if err != nil || !info.Mode().IsRegular() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		to := filepath.Join(dst, entry.Name())
		if err := os.Link(from, to); err == nil {
			linked++
			bytes += info.Size()
			continue
		}
		if err := copyInputFile(ctx, from, to); err != nil {
			return linked, copied, bytes, err
		}
		copied++
		bytes += info.Size()
	}
	return linked, copied, bytes, makeSubjectTreeReadOnly(dst)
}

func copyInputFile(ctx context.Context, from, to string) error {
	in, err := os.Open(from)
	if err != nil {
		return err
	}
	out, err := os.OpenFile(to, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		_ = in.Close()
		return err
	}
	_, copyErr := io.Copy(out, &contextReader{ctx: ctx, reader: in})
	return errors.Join(copyErr, in.Close(), out.Close())
}

// StageInputsFailure replaces whatever a failed StageInputs left in dir with
// a manifest that refuses every named ref with the cause, so the capsule
// reads why its inputs are missing instead of finding an empty directory.
func StageInputsFailure(dir string, refs []string, cause error) error {
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	manifest := InputsManifest{Files: []InputFile{}, Refused: []InputRefusal{}}
	reason := "staging the inputs failed"
	if cause != nil {
		reason += ": " + cause.Error()
	}
	for i, ref := range refs {
		if i >= MaxInputRefusals {
			manifest.Refused = append(manifest.Refused, InputRefusal{Source: fmt.Sprintf("%d more refs", len(refs)-i), Reason: reason})
			break
		}
		manifest.Refused = append(manifest.Refused, InputRefusal{Source: ref, Reason: reason})
	}
	raw, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	return writeStagedDeliverable(filepath.Join(dir, InputsManifestName), append(raw, '\n'))
}
