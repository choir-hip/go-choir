package agentcore

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/researchtools"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Assignment inputs. The capsule's egress allowlist does not reach public
// data hosts (research is the world-evidence desk), so evidence that
// research captured and management names in its engineering Cast reaches the
// capsule as files: the Cast's "captures" (the typed field every Cast
// already carries, on the tape) are resolved here, before spawn, to the
// named capture bodies, staged bounded (capsule.StageInputs), and Spawn lays
// them read-only at capsule.InputsDir beside a manifest.json.
//
// Custody: only the captures the Cast on the tape names, only web captures
// (an email id or any other kind is refused: mail is the Mail agent's
// custody and is never materialized), and an owner-private capture only
// when the engineering desk may read it (captureServedTo, the gate
// choir.ReadCapture applies); engineering holds no lease at spawn, so those
// are refused with the reason in the manifest. A ref that cannot be read is
// a manifest refusal, never a failed spawn. No secret scan: computers hold
// no secrets (owner ruling).

// inputsStageTimeout bounds fetching every named body before spawn.
const inputsStageTimeout = 45 * time.Second

// assignmentCaptureSource is the host's read of shared captures: the record
// (metadata) and the raw bytes as fetched.
type assignmentCaptureSource interface {
	ReadCapture(ctx context.Context, captureID string, fromBlock, maxChars int) (json.RawMessage, error)
	OpenCaptureBody(ctx context.Context, contentBlobID string) (io.ReadCloser, error)
}

func (rt *Runtime) assignmentCaptures() assignmentCaptureSource {
	if rt.captureInputs != nil {
		return rt.captureInputs
	}
	if base := strings.TrimSpace(rt.cfg.CorpusdURL); base != "" {
		return researchtools.NewCorpusdCaptureSink(base)
	}
	return nil
}

// castNamedCaptures is the capture ids management's Cast to the assignment
// names, from the record on the tape (never from the cell), without
// duplicates and in the Cast's order.
func (rt *Runtime) castNamedCaptures(ctx context.Context, assignment types.EngineeringAssignment) []string {
	b := assignment.Binding
	cast, err := rt.store.GetCommitmentRecord(ctx, b.OwnerID, b.ComputerID, b.ParentControlID)
	if err != nil || cast == nil || !cast.IsCast() || cast.SenderInbox != b.ParentAgentID || cast.Cast == nil {
		return nil
	}
	seen := map[string]bool{}
	var ids []string
	for _, id := range cast.Cast.Captures {
		if id = strings.TrimSpace(id); id != "" && !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	return ids
}

// inputSourcesForCaptures resolves each named ref to an input source or a
// refusal. engineering is the desk the capture read gate is asked for.
func inputSourcesForCaptures(ctx context.Context, ids []string, src assignmentCaptureSource) []capsule.InputSource {
	reader, readerOK := objectgraph.DeskWriter(canonicalDeskName(assignmentPerformerName()))
	out := make([]capsule.InputSource, 0, len(ids))
	for _, id := range ids {
		out = append(out, inputSourceForCapture(ctx, id, src, reader, readerOK))
	}
	return out
}

func inputSourceForCapture(ctx context.Context, id string, src assignmentCaptureSource, reader objectgraph.Writer, readerOK bool) capsule.InputSource {
	item := capsule.InputSource{Ref: id, Size: -1}
	kind, _, _, err := objectgraph.ParseCanonicalID(id)
	if err != nil || kind != objectgraph.WebCaptureObjectKind {
		item.Refusal = "not a web capture id; only captures are delivered as inputs, and mail is never materialized"
		return item
	}
	if src == nil {
		item.Refusal = "the capture store is unavailable on this computer"
		return item
	}
	raw, err := src.ReadCapture(ctx, id, 0, 1)
	if err != nil {
		if errors.Is(err, researchtools.ErrCaptureNotFound) {
			item.Refusal = "capture not found"
		} else {
			item.Refusal = "capture could not be read: " + err.Error()
		}
		return item
	}
	var meta struct {
		URL           string   `json:"url"`
		MediaType     string   `json:"media_type"`
		ContentBlobID string   `json:"content_blob_id"`
		RawBytes      int64    `json:"raw_bytes"`
		Labels        []string `json:"labels"`
	}
	if err := json.Unmarshal(raw, &meta); err != nil {
		item.Refusal = "capture record is unreadable"
		return item
	}
	if !captureServedTo(reader, readerOK, types.NewLabelSet(meta.Labels...)) {
		item.Refusal = "owner-private capture: engineering holds no lease for it (research can capture the public source as a world capture)"
		return item
	}
	if strings.TrimSpace(meta.ContentBlobID) == "" {
		item.Refusal = "capture has no stored bytes"
		return item
	}
	item.URL, item.MediaType, item.Name = meta.URL, strings.TrimSpace(meta.MediaType), inputFileName(id, meta.URL, meta.MediaType)
	if meta.RawBytes > 0 {
		item.Size = meta.RawBytes
	}
	blob := meta.ContentBlobID
	item.Open = func(ctx context.Context) (io.ReadCloser, error) { return src.OpenCaptureBody(ctx, blob) }
	return item
}

var inputNameJunk = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

var inputExtensions = map[string]string{
	"csv": "csv", "tsv": "tsv", "json": "json", "txt": "txt", "md": "md", "xml": "xml",
	"pdf": "pdf", "html": "html", "htm": "html", "xlsx": "xlsx", "xls": "xls",
}

var inputMediaExtensions = map[string]string{
	"text/csv": "csv", "text/tab-separated-values": "tsv", "application/json": "json",
	"text/plain": "txt", "text/markdown": "md", "application/xml": "xml", "text/xml": "xml",
	"application/pdf": "pdf", "text/html": "html",
}

// inputFileName is a flat, unique name for a capture: a slug of the URL's
// file name and query values (so fredgraph.csv?id=UNRATE and ?id=CPIAUCSL
// differ), the first eight hex digits of the capture id's hash, and an
// extension from the URL or the media type.
func inputFileName(captureID, rawURL, mediaType string) string {
	slug, ext := "", ""
	if u, err := url.Parse(strings.TrimSpace(rawURL)); err == nil {
		base := path.Base(u.Path)
		if base == "." || base == "/" {
			base = ""
		}
		if e := strings.ToLower(strings.TrimPrefix(path.Ext(base), ".")); inputExtensions[e] != "" {
			ext = inputExtensions[e]
		}
		base = strings.TrimSuffix(base, path.Ext(base))
		parts := []string{base}
		q := u.Query()
		keys := make([]string, 0, len(q))
		for k := range q {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			parts = append(parts, q[k]...)
		}
		slug = strings.Join(parts, "-")
		if slug == "" || slug == "-" {
			slug = u.Hostname()
		}
	}
	slug = inputNameJunk.ReplaceAllString(slug, "_")
	for strings.Contains(slug, "..") {
		slug = strings.ReplaceAll(slug, "..", ".")
	}
	slug = strings.Trim(slug, "._-")
	if len(slug) > 48 {
		slug = strings.Trim(slug[:48], "._-")
	}
	if slug == "" {
		slug = "capture"
	}
	if ext == "" {
		media := strings.ToLower(strings.TrimSpace(strings.SplitN(mediaType, ";", 2)[0]))
		if ext = inputMediaExtensions[media]; ext == "" {
			ext = "bin"
		}
	}
	sum := sha256.Sum256([]byte(captureID))
	return slug + "-" + hex.EncodeToString(sum[:4]) + "." + ext
}

// stageAssignmentInputs stages the evidence the assignment's Cast names and
// returns the staging directory for SpawnSpec.InputsDir ("" when the Cast
// names none) and its cleanup. It never fails the spawn: a staging error is
// logged and the capsule starts without inputs.
func (rt *Runtime) stageAssignmentInputs(ctx context.Context, assignment types.EngineeringAssignment) (string, func()) {
	noop := func() {}
	if rt == nil || rt.store == nil {
		return "", noop
	}
	ids := rt.castNamedCaptures(ctx, assignment)
	if len(ids) == 0 {
		return "", noop
	}
	started := time.Now()
	stageRoot := rt.selfdevUpdaterRoot
	if strings.TrimSpace(stageRoot) == "" {
		stageRoot = filepath.Join(filepath.Dir(rt.filesRoot()), ".stage")
	}
	if err := os.MkdirAll(stageRoot, 0o700); err != nil {
		log.Printf("agentcore: assignment %s inputs not staged: %v", assignment.AssignmentID, err)
		return "", noop
	}
	sweepStaleInputStages(stageRoot)
	dir, err := os.MkdirTemp(stageRoot, ".inputs-")
	if err != nil {
		log.Printf("agentcore: assignment %s inputs not staged: %v", assignment.AssignmentID, err)
		return "", noop
	}
	remove := func() { _ = os.RemoveAll(dir) }
	ctx, cancel := context.WithTimeout(ctx, inputsStageTimeout)
	defer cancel()
	manifest, err := capsule.StageInputs(ctx, inputSourcesForCaptures(ctx, ids, rt.assignmentCaptures()), dir)
	if err != nil {
		log.Printf("agentcore: assignment %s inputs not staged: %v", assignment.AssignmentID, err)
		// The capsule still gets a manifest naming every ref and the cause.
		if ferr := capsule.StageInputsFailure(dir, ids, err); ferr != nil {
			remove()
			return "", noop
		}
		return dir, remove
	}
	var bytes int64
	for _, f := range manifest.Files {
		bytes += f.Bytes
	}
	log.Printf("agentcore: assignment %s inputs staged named=%d files=%d refused=%d bytes=%d took_ms=%d",
		assignment.AssignmentID, len(ids), len(manifest.Files), len(manifest.Refused), bytes, time.Since(started).Milliseconds())
	return dir, remove
}

// sweepStaleInputStages removes .inputs-* directories a crash left in
// stageRoot, those older than an hour.
func sweepStaleInputStages(stageRoot string) {
	entries, err := os.ReadDir(stageRoot)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), ".inputs-") {
			continue
		}
		if info, err := entry.Info(); err == nil && time.Since(info.ModTime()) > time.Hour {
			_ = os.RemoveAll(filepath.Join(stageRoot, entry.Name()))
		}
	}
}

