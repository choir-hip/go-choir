package capsule

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

// Deliverables: the files an engineering assignment makes for the owner (a
// PDF report, a chart). The capsule sees Files as a read-only snapshot, so
// nothing written there returns; the assignment writes them to
// DeliverablesDir instead. When the assignment completes, the host stages the
// regular files there (stageDeliverableFiles, opened beneath the capsule root
// without following symlinks) and files the accepted ones in the owner's Files
// at Reports/<short assignment id>/<name>. A refused file is listed with its
// reason in the completion, never dropped silently.
const (
	// DeliverablesDir is where an assignment writes what the owner should get.
	DeliverablesDir = "/var/lib/artifact/deliverables"
	// DeliverablesPrefix is DeliverablesDir as a capsule diff path.
	DeliverablesPrefix = "var/lib/artifact/deliverables/"

	// MaxDeliverables bounds the files one assignment delivers.
	MaxDeliverables = 20
	// MaxDeliverableBytes bounds one file.
	MaxDeliverableBytes = 50 << 20
	// MaxDeliverablesTotalBytes bounds the set.
	MaxDeliverablesTotalBytes = 200 << 20
	// MaxDeliverableEntries bounds the directory entries examined; the rest
	// are counted, not read.
	MaxDeliverableEntries = 100
	// MaxDeliverableRefusals bounds the refusals a completion lists.
	MaxDeliverableRefusals = 20
)

var deliverableNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._ -]{0,95}$`)

// DeliverableFile is one accepted deliverable, staged under a host directory.
type DeliverableFile struct {
	Name      string `json:"name"`
	MediaType string `json:"media_type"`
	SHA256    string `json:"sha256"`
	Bytes     int64  `json:"bytes"`
}

// StagedDeliverables is what StageGrantedDeliverables staged under Dir.
type StagedDeliverables struct {
	Dir     string
	Files   []DeliverableFile
	Refused []DeliverableRefusal
}

// Remove deletes the staged directory.
func (s *StagedDeliverables) Remove() {
	if s != nil && s.Dir != "" {
		_ = os.RemoveAll(s.Dir)
	}
}

// DeliverableRefusal is one file the host did not accept, with the reason.
type DeliverableRefusal struct {
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

// SanitizeDeliverableName makes a flat file name: characters outside letters,
// digits, '.', '_', '-' and space become '_'. It refuses a name that is a
// path (a separator), a dot-file, or one holding "..", rather than rewriting
// it into something the assignment did not mean.
func SanitizeDeliverableName(raw string) (string, error) {
	if raw == "" || strings.ContainsAny(raw, "/\\") {
		return "", errors.New("not a flat file name (nested directories are not delivered)")
	}
	if strings.HasPrefix(raw, ".") {
		return "", errors.New("a dot-file is not delivered")
	}
	if strings.Contains(raw, "..") {
		return "", errors.New("a name holding .. is not delivered")
	}
	var b strings.Builder
	for _, r := range raw {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '.', r == '_', r == '-', r == ' ':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	name := strings.TrimRight(b.String(), " .")
	if !deliverableNamePattern.MatchString(name) {
		return "", errors.New("name is empty or longer than 96 characters")
	}
	return name, nil
}

// SniffDeliverable checks name's extension against its first bytes and
// returns the media type. Only these types are delivered: pdf, png, jpg/jpeg,
// webp, gif, csv, json, txt, md. svg and html are not: Files can serve them
// inline in its own origin.
func SniffDeliverable(name string, head []byte) (string, error) {
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(name), "."))
	switch ext {
	case "pdf":
		if bytes.HasPrefix(head, []byte("%PDF-")) {
			return "application/pdf", nil
		}
	case "png":
		if bytes.HasPrefix(head, []byte("\x89PNG\r\n\x1a\n")) {
			return MediaTypePNG, nil
		}
	case "jpg", "jpeg":
		if bytes.HasPrefix(head, []byte{0xff, 0xd8, 0xff}) {
			return "image/jpeg", nil
		}
	case "webp":
		if len(head) >= 12 && bytes.Equal(head[:4], []byte("RIFF")) && bytes.Equal(head[8:12], []byte("WEBP")) {
			return "image/webp", nil
		}
	case "gif":
		if bytes.HasPrefix(head, []byte("GIF87a")) || bytes.HasPrefix(head, []byte("GIF89a")) {
			return "image/gif", nil
		}
	case "csv":
		if plainText(head) {
			return "text/csv", nil
		}
	case "json":
		if plainText(head) {
			return "application/json", nil
		}
	case "txt":
		if plainText(head) {
			return "text/plain", nil
		}
	case "md":
		if plainText(head) {
			return "text/markdown", nil
		}
	default:
		return "", fmt.Errorf("type .%s is not delivered (pdf, png, jpg, webp, gif, csv, json, txt, md are)", ext)
	}
	return "", fmt.Errorf("content is not a %s", ext)
}

// plainText says head is UTF-8 without NUL bytes (a window may cut the final
// rune).
func plainText(head []byte) bool {
	if bytes.IndexByte(head, 0) >= 0 {
		return false
	}
	for i := 0; i < 4 && len(head) > 0 && !utf8.Valid(head); i++ {
		head = head[:len(head)-1]
	}
	return utf8.Valid(head)
}

// deliverableNames lists the entries the capsule left in DeliverablesDir,
// from its diff: a name per non-deleted, non-directory change. A nested
// path is returned whole so the stager refuses it by name.
func deliverableNames(changes []FileChange) (names []string, links map[string]bool) {
	links = map[string]bool{}
	seen := map[string]bool{}
	for _, change := range changes {
		rel, ok := strings.CutPrefix(strings.TrimPrefix(change.Path, "/"), DeliverablesPrefix)
		if !ok || change.Kind == ChangeDeleted || change.Mode.IsDir() || rel == "" || seen[rel] {
			continue
		}
		seen[rel] = true
		if change.Mode&os.ModeSymlink != 0 {
			links[rel] = true
		}
		names = append(names, rel)
	}
	sort.Strings(names)
	return names, links
}

// stageDeliverableFiles copies the accepted files from the capsule (opened
// through open, which must not follow symlinks) into destDir, one temp file
// and rename each, so a crash leaves no partial file under a final name. It
// bounds count, size and total and sniffs the type. Every
// file it does not accept is in the refusals. destDir is created; a retry
// replaces the same names, so it is idempotent.
func stageDeliverableFiles(ctx context.Context, names []string, links map[string]bool, open func(name string) (screenInput, error), destDir string) ([]DeliverableFile, []DeliverableRefusal, error) {
	if len(names) == 0 {
		return nil, nil, nil
	}
	if err := os.MkdirAll(destDir, 0o700); err != nil {
		return nil, nil, err
	}
	var accepted []DeliverableFile
	var refused []DeliverableRefusal
	used := map[string]bool{}
	var total int64
	for i, raw := range names {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		if i >= MaxDeliverableEntries {
			refused = append(refused, DeliverableRefusal{Name: fmt.Sprintf("%d more entries", len(names)-i), Reason: fmt.Sprintf("not examined: only %d entries are", MaxDeliverableEntries)})
			break
		}
		refuse := func(reason string) { refused = append(refused, DeliverableRefusal{Name: fmt.Sprintf("%q", raw), Reason: reason}) }
		if links[raw] {
			refuse("a symlink is never followed")
			continue
		}
		name, err := SanitizeDeliverableName(raw)
		if err != nil {
			refuse(err.Error())
			continue
		}
		if len(accepted) >= MaxDeliverables {
			refuse(fmt.Sprintf("over the %d-file limit", MaxDeliverables))
			continue
		}
		if used[strings.ToLower(name)] {
			refuse("another file has the same name after sanitizing")
			continue
		}
		input, err := open(raw)
		if err != nil {
			refuse("unreadable (not a plain file beneath the output directory): " + err.Error())
			continue
		}
		info, err := input.Stat()
		if err != nil || !info.Mode().IsRegular() {
			_ = input.Close()
			refuse("not a regular file")
			continue
		}
		if info.Size() > MaxDeliverableBytes {
			_ = input.Close()
			refuse(fmt.Sprintf("larger than %d MB", MaxDeliverableBytes>>20))
			continue
		}
		if total+info.Size() > MaxDeliverablesTotalBytes {
			_ = input.Close()
			refuse(fmt.Sprintf("the set would exceed %d MB", MaxDeliverablesTotalBytes>>20))
			continue
		}
		data, err := io.ReadAll(io.LimitReader(input, MaxDeliverableBytes+1))
		_ = input.Close()
		if err != nil {
			refuse("read failed: " + err.Error())
			continue
		}
		if len(data) == 0 {
			refuse("empty")
			continue
		}
		if len(data) > MaxDeliverableBytes {
			refuse(fmt.Sprintf("larger than %d MB", MaxDeliverableBytes>>20))
			continue
		}
		mediaType, err := SniffDeliverable(name, data[:min(len(data), 8192)])
		if err != nil {
			refuse(err.Error())
			continue
		}
		sum := sha256.Sum256(data)
		if err := writeStagedDeliverable(filepath.Join(destDir, name), data); err != nil {
			return nil, nil, err
		}
		used[strings.ToLower(name)] = true
		total += int64(len(data))
		accepted = append(accepted, DeliverableFile{Name: name, MediaType: mediaType, SHA256: hex.EncodeToString(sum[:]), Bytes: int64(len(data))})
	}
	return accepted, refused, nil
}

// writeStagedDeliverable writes data beside final under a temporary name and
// renames it into place.
func writeStagedDeliverable(final string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(final), ".stage-*")
	if err != nil {
		return err
	}
	_, werr := tmp.Write(data)
	if err := errors.Join(werr, tmp.Chmod(0o600), tmp.Sync(), tmp.Close()); err != nil {
		_ = os.Remove(tmp.Name())
		return err
	}
	if err := os.Rename(tmp.Name(), final); err != nil {
		_ = os.Remove(tmp.Name())
		return err
	}
	return nil
}

// sweepStaleDeliverableStages removes .deliverables-* staging directories a
// crash left in stageRoot, those older than an hour (a live stage is seconds).
func sweepStaleDeliverableStages(stageRoot string) {
	entries, err := os.ReadDir(stageRoot)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), ".deliverables-") {
			continue
		}
		if info, err := entry.Info(); err == nil && time.Since(info.ModTime()) > time.Hour {
			_ = os.RemoveAll(filepath.Join(stageRoot, entry.Name()))
		}
	}
}
