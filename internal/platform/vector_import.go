package platform

// Import of vectors made elsewhere (station V2; the Mac makes the first pass,
// station V3). The file is input to corpusd, which stays the only writer: a
// row goes through the checks a worker-made vector goes through (the model's
// hash, text_sha against the passage this host cuts, public text only) and is
// written by the same functions, with a sampled re-embedding on top. The form is an operator route on corpusd's host loopback port
// (POST /internal/platform/vectors/import, X-Internal-Caller plus the host
// credential, like artifact-gc) rather than a subcommand: a subcommand would
// be a second process with its own database credentials and its own embedder
// client, and the route already has both, and the loaded stories' and
// captures' text beside it. The file is posted from the host (curl
// --data-binary to 127.0.0.1:8086); the route answers 202 with a job id and
// GET ?job= reports it.
//
// File: one JSON object a line (docs/reference/EMBEDDINGGEMMA.md, "The first
// pass file"): kind ("story_passage" or "capture"), id (a publication VERSION's
// canonical id, or a capture's), ordinal, text_sha (SHA-256 of the passage
// text), model (the GGUF's SHA-256), dim (256) and v (base64 of 256
// little-endian float16 values). No text.
//
// The file is refused whole when any row names another model or dimension, or
// is malformed, and when a sample re-embedded here by the Node B embedder
// does not agree (cosine 0.999 or more) with the file's vector. Past those, a
// row is written only if this host cuts the same passage (text_sha equal) and
// the text may have a vector (public); a row that does not match is counted
// and dropped, so a different cut can never be mistaken for ours. A story
// version is written whole or not at all.

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	mrand "math/rand"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/embedder"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

const (
	vectorKindStory   = "story_passage"
	vectorKindCapture = "capture"
	// vectorImportMinCosine is the agreement a re-embedded sample must reach.
	vectorImportMinCosine = 0.999
	// vectorImportSamples is how many rows are re-embedded, and the least a
	// large file may be checked by: a file with fewer checkable rows than
	// the floor (that many, or half the file when it is small) is refused.
	vectorImportSamples = 32
	// vectorImportMaxLine bounds a line: a row is about 1 KB (two ids, a
	// 684-character vector).
	vectorImportMaxLine = 4096
	// vectorImportCandidates bounds how many rows a sample examines to find
	// its checkable ones.
	vectorImportCandidates = 400
	vectorImportDirName    = "vector-import"
)

// vectorImportMaxBytes bounds a posted or inboxed file (the first pass is about
// 215,000 rows of 700 bytes). A variable so a test can shrink it.
var vectorImportMaxBytes int64 = 2 << 30

// vectorImportMaxGroupRows bounds the rows held for one story version: a
// version of a quarter of a megabyte is under 200 passages. A variable so a
// test can shrink it.
var vectorImportMaxGroupRows = 5000

// ErrVectorImportRefused marks a file refused whole.
var ErrVectorImportRefused = errors.New("vector import refused")

type vectorFileRow struct {
	Kind    string `json:"kind"`
	ID      string `json:"id"`
	Ordinal int    `json:"ordinal"`
	TextSHA string `json:"text_sha"`
	Model   string `json:"model"`
	Dim     int    `json:"dim"`
	V       string `json:"v"`
}

// decodeHalfVector reads a row's v: 256 little-endian float16 values.
func decodeHalfVector(b64 string) ([]float32, error) {
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, fmt.Errorf("v is not base64: %w", err)
	}
	if len(raw) != 2*embedder.Dim {
		return nil, fmt.Errorf("v is %d bytes, want %d", len(raw), 2*embedder.Dim)
	}
	out := make([]float32, embedder.Dim)
	var sum float64
	for i := range out {
		f, ok := halfToFloat32(uint16(raw[2*i]) | uint16(raw[2*i+1])<<8)
		if !ok {
			return nil, errors.New("v holds a non-finite value")
		}
		out[i] = f
		sum += float64(f) * float64(f)
	}
	if sum == 0 {
		return nil, errors.New("v is the zero vector")
	}
	return out, nil
}

// halfToFloat32 converts an IEEE 754 half; ok is false for infinity and NaN.
func halfToFloat32(h uint16) (float32, bool) {
	sign := uint32(h>>15) & 1
	exp := int(h>>10) & 0x1f
	frac := uint32(h & 0x3ff)
	switch {
	case exp == 31:
		return 0, false
	case exp == 0 && frac == 0:
		return math.Float32frombits(sign << 31), true
	case exp == 0:
		f := float32(frac) * float32(math.Ldexp(1, -24))
		if sign == 1 {
			f = -f
		}
		return f, true
	}
	return math.Float32frombits(sign<<31 | uint32(exp-15+127)<<23 | frac<<13), true
}

// VectorImportJob is the state of an import.
type VectorImportJob struct {
	JobID      string        `json:"job_id"`
	State      string        `json:"state"` // running | done | refused | failed
	Error      string        `json:"error,omitempty"`
	StartedAt  time.Time     `json:"started_at"`
	FinishedAt *time.Time    `json:"finished_at,omitempty"`
	Result     *VectorImport `json:"result,omitempty"`
	// Source is "route" (POST /internal/platform/vectors/import) or "inbox"
	// (a file in the spool inbox); File is the inbox file's name.
	Source   string         `json:"source,omitempty"`
	File     string         `json:"file,omitempty"`
	Rejected map[string]int `json:"rejected,omitempty"`
}

// VectorImport is what an import did.
type VectorImport struct {
	Rows    int `json:"rows"`
	Written int `json:"written"`
	// Rejected counts dropped rows by reason: text_mismatch, not_found,
	// not_public, bad_ordinal, incomplete_version, no_text.
	Rejected map[string]int `json:"rejected,omitempty"`
	// Samples re-embedded, and the lowest cosine among them.
	Samples   int     `json:"samples"`
	MinCosine float64 `json:"min_cosine"`
	// SampleSeed seeded the random choice of the sample, so a refused file's
	// sample can be chosen again.
	SampleSeed int64 `json:"sample_seed"`
}

func (r *VectorImport) reject(reason string, n int) {
	if r.Rejected == nil {
		r.Rejected = map[string]int{}
	}
	r.Rejected[reason] += n
}

// scanVectorFile calls fn for every row of the file, in order; i counts the
// rows (blank lines are not rows). A line over vectorImportMaxLine refuses the
// file.
func scanVectorFile(path string, fn func(i int, row vectorFileRow, v []float32) error) error {
	f, err := os.Open(path)
	if err != nil {
		// A file that is not there or whose permissions forbid reading is a fault of
		// the file: refused. Any other open failure (too many open files, an I/O
		// error) is the host's state: returned plain, so the file waits.
		if errors.Is(err, os.ErrPermission) || errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("%w: the file cannot be read: %v", ErrVectorImportRefused, err)
		}
		return fmt.Errorf("vector import: open: %w", err)
	}
	defer f.Close()
	rd := bufio.NewReaderSize(f, vectorImportMaxLine)
	i := 0
	for lineNo := 1; ; lineNo++ {
		line, err := rd.ReadSlice('\n')
		if errors.Is(err, bufio.ErrBufferFull) {
			return fmt.Errorf("%w: line %d is longer than %d bytes", ErrVectorImportRefused, lineNo, vectorImportMaxLine)
		}
		if len(bytes.TrimSpace(line)) > 0 {
			var row vectorFileRow
			if jerr := json.Unmarshal(line, &row); jerr != nil {
				return fmt.Errorf("%w: line %d is not a row: %v", ErrVectorImportRefused, lineNo, jerr)
			}
			v, derr := decodeHalfVector(row.V)
			if derr != nil {
				return fmt.Errorf("%w: line %d: %v", ErrVectorImportRefused, lineNo, derr)
			}
			if err := fn(i, row, v); err != nil {
				return err
			}
			i++
		}
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
	}
}

// importStratum is the group a row is sampled from: a story's first passage,
// a later story passage, a capture's lead.
func importStratum(row vectorFileRow) int {
	switch {
	case row.Kind == vectorKindCapture:
		return 2
	case row.Ordinal == 0:
		return 0
	}
	return 1
}

// rowInput is what this host derives for a file row: the prompt input, the
// text's SHA and the key of the row, or a reason it cannot be written.
type rowInput struct {
	Input, TextSHA string
	Owner, Comp    string
	StoryID        string
	Day            time.Time
	reason         string
}

// reasonUnreadableNow is a row the host could not read this time (the database
// or a blob read failed): not a fault of the file; the row is dropped from this
// import, the sample counts it as "cannot be checked now", and the worker fills it.
const reasonUnreadableNow = "unreadable_now"

// importErrReason is the reason for an error reading the objects of a row:
// nothing found is a fact about the file's row; anything else is the host's state.
func importErrReason(err error) string {
	if errors.Is(err, sql.ErrNoRows) {
		return "not_found"
	}
	return reasonUnreadableNow
}

// vectorRowDeriver derives rows' inputs, holding the last story version.
type vectorRowDeriver struct {
	s       *Service
	og      *ObjectGraphStore
	version string
	story   string
	vt      storyVersionText
	vtOK    bool
	vtErr   error
}

func (d *vectorRowDeriver) storyVersion(ctx context.Context, versionID string) (storyID string, vt storyVersionText, ok bool, err error) {
	if d.version == versionID {
		return d.story, d.vt, d.vtOK, d.vtErr
	}
	d.version, d.story, d.vt, d.vtOK, d.vtErr = versionID, "", storyVersionText{}, false, nil
	var storyID2 string
	qerr := d.s.store.corpus().QueryRowContext(ctx, `SELECT from_id FROM og_edges WHERE to_id = ? AND kind = 'has_version' AND tombstone = FALSE LIMIT 1`, versionID).Scan(&storyID2)
	if qerr != nil { // sql.ErrNoRows is "not found"; any other error is the database's state
		d.vtErr = qerr
		return "", d.vt, false, qerr
	}
	d.story = storyID2
	if okSQL, err := d.s.store.sqlEligibleStoryVersions(ctx, []storyVersionRef{{StoryID: storyID2, VersionID: versionID}}); err != nil || !okSQL[versionID] {
		d.vtErr = err
		return d.story, d.vt, false, err
	}
	d.vt, d.vtOK, d.vtErr = d.s.loadStoryVersionText(ctx, d.og, storyID2, versionID)
	return d.story, d.vt, d.vtOK, d.vtErr
}

func (d *vectorRowDeriver) derive(ctx context.Context, row vectorFileRow) rowInput {
	switch row.Kind {
	case vectorKindStory:
		story, vt, ok, err := d.storyVersion(ctx, row.ID)
		if err != nil {
			return rowInput{reason: importErrReason(err)}
		}
		if !ok {
			return rowInput{reason: "not_public"}
		}
		if row.Ordinal < 0 || row.Ordinal >= len(vt.Passages) {
			return rowInput{reason: "bad_ordinal"}
		}
		p := vt.Passages[row.Ordinal]
		return rowInput{Input: embedder.DocumentInput(vt.Title, p), TextSHA: embedder.TextSHA(p), StoryID: story, Day: vt.Day}
	case vectorKindCapture:
		objs, err := d.s.store.captureObjectsByIDAny(ctx, []string{row.ID})
		if err != nil {
			return rowInput{reason: reasonUnreadableNow}
		}
		obj, found := objs[row.ID]
		if !found {
			return rowInput{reason: "not_found"}
		}
		title, hash, ok := captureVectorSource(obj)
		okSQL, serr := d.s.store.sqlEligibleCaptures(ctx, []objectgraph.Object{obj})
		if serr != nil {
			return rowInput{reason: reasonUnreadableNow}
		}
		if !ok || !okSQL[obj.CanonicalID] {
			return rowInput{reason: "not_public"}
		}
		if row.Ordinal != 0 {
			return rowInput{reason: "bad_ordinal"}
		}
		lead, err := captureLeadPassage(d.s.captureBodies, hash)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return rowInput{reason: "no_text"}
			}
			return rowInput{reason: reasonUnreadableNow}
		}
		if title == "" && strings.TrimSpace(lead) == "" {
			return rowInput{reason: "no_text"} // the worker's rule: nothing to embed
		}
		return rowInput{Input: embedder.DocumentInput(title, lead), TextSHA: embedder.TextSHA(lead), Owner: obj.OwnerID, Comp: obj.ComputerID, Day: captureDay(obj)}
	}
	return rowInput{reason: "not_found"}
}

// ImportVectorFile imports the file at path. It returns ErrVectorImportRefused
// (wrapped) when the file is refused whole; nothing is written then.
func (s *Service) ImportVectorFile(ctx context.Context, path string) (VectorImport, error) {
	res := VectorImport{MinCosine: 1}
	emb := s.store.embedder()
	if emb == nil {
		return res, errors.New("vector import: no embedder configured")
	}
	if err := s.store.ensureVectorTables(ctx); err != nil && !s.store.VectorsReady() {
		return res, err
	}
	// Pass 1: every row names our model and dimension and decodes; the rows'
	// strata are kept (a byte a row) for the sample.
	var strata []int8
	if err := scanVectorFile(path, func(i int, row vectorFileRow, v []float32) error {
		res.Rows++
		if row.Model != embedder.ModelHash {
			return fmt.Errorf("%w: row %d names model %q, this host's is %s", ErrVectorImportRefused, i+1, row.Model, embedder.ModelHash)
		}
		if row.Dim != embedder.Dim {
			return fmt.Errorf("%w: row %d has dim %d, want %d", ErrVectorImportRefused, i+1, row.Dim, embedder.Dim)
		}
		if row.Kind != vectorKindStory && row.Kind != vectorKindCapture {
			return fmt.Errorf("%w: row %d has kind %q", ErrVectorImportRefused, i+1, row.Kind)
		}
		strata = append(strata, int8(importStratum(row)))
		return nil
	}); err != nil {
		return res, err
	}
	if res.Rows == 0 {
		return res, fmt.Errorf("%w: the file has no rows", ErrVectorImportRefused)
	}

	// Pass 2: a random sample, re-embedded here, must agree with the file. The
	// seed is recorded. Candidates are drawn round-robin from the three strata
	// (a story's first passage, a later one, a capture) so each present kind of
	// row is checked; a candidate this host cannot cut identically does not
	// count, and fewer checkable rows than the floor refuses the file.
	var seedBytes [8]byte
	if _, err := rand.Read(seedBytes[:]); err != nil {
		return res, err
	}
	res.SampleSeed = int64(binary.LittleEndian.Uint64(seedBytes[:]) >> 1)
	rng := mrand.New(mrand.NewSource(res.SampleSeed))
	var byStratum [3][]int
	for i, st := range strata {
		byStratum[st] = append(byStratum[st], i)
	}
	for k := range byStratum {
		rng.Shuffle(len(byStratum[k]), func(a, b int) { byStratum[k][a], byStratum[k][b] = byStratum[k][b], byStratum[k][a] })
	}
	var order []int
	for round := 0; len(order) < vectorImportCandidates; round++ {
		added := false
		for k := range byStratum {
			if round < len(byStratum[k]) && len(order) < vectorImportCandidates {
				order = append(order, byStratum[k][round])
				added = true
			}
		}
		if !added {
			break
		}
	}
	wanted := make(map[int]bool, len(order))
	for _, i := range order {
		wanted[i] = true
	}
	type candidate struct {
		row vectorFileRow
		v   []float32
	}
	cands := map[int]candidate{}
	if err := scanVectorFile(path, func(i int, row vectorFileRow, v []float32) error {
		if wanted[i] {
			cands[i] = candidate{row, v}
		}
		return nil
	}); err != nil {
		return res, err
	}
	deriver := &vectorRowDeriver{s: s, og: s.ogStore()}
	floor := min(vectorImportSamples, (res.Rows+1)/2)
	// Rows are drawn in the random stratified order and embedded here in chunks
	// until vectorImportSamples have been CHECKED. A row this host cannot
	// reproduce (its text differs) does not count; a row the host's embedder
	// refuses as too large is not a fault of the file: it is set aside (and
	// dropped in pass 3 as "oversized", for the worker to record its own refusal)
	// and another is drawn; a call that fails in transport is skipped and another
	// drawn. The outcomes are embedInputs's (vector_outcome.go); this loop
	// inspects no error.
	oversizedKeys := map[string]bool{}
	rowKey := func(r vectorFileRow) string { return fmt.Sprintf("%s|%s|%d", r.Kind, r.ID, r.Ordinal) }
	checked, setAside, skippedNow, fileRejected, next := 0, 0, 0, 0, 0 // fileRejected: not found, not public, another cut
	for checked < vectorImportSamples && next < len(order) {
		var inputs []string
		var vecs [][]float32
		var keys []string
		for next < len(order) && len(inputs) < vectorImportSamples-checked {
			c := cands[order[next]]
			next++
			in := deriver.derive(ctx, c.row)
			if in.reason == reasonUnreadableNow {
				skippedNow++
				continue
			}
			if in.reason != "" || in.TextSHA != c.row.TextSHA {
				fileRejected++ // a fault of the file
				continue
			}
			inputs = append(inputs, in.Input)
			vecs = append(vecs, c.v)
			keys = append(keys, rowKey(c.row))
		}
		if len(inputs) == 0 {
			break
		}
		got, refusals, failed, err := s.embedInputs(ctx, "import_sample", inputs)
		if err != nil {
			return res, err
		}
		for i := range inputs {
			switch {
			case got[i] != nil:
				c := embedder.Cosine(got[i], vecs[i])
				checked++
				res.Samples++
				if c < res.MinCosine {
					res.MinCosine = c
				}
			case refusals != nil && refusals[i] != "":
				oversizedKeys[keys[i]] = true
				setAside++
			default:
				_ = failed
				skippedNow++
			}
		}
	}
	if checked < floor {
		// Rows set aside (too large for this host) and rows rejected as the file's
		// own fault (not found, not public, another cut) never come back; rows the
		// server could not answer for now (skippedNow) are the only ones a later
		// pass could add. So even if every skipped row had passed, is the floor
		// reachable?
		//   - No (checked+skippedNow < floor): the shortfall is the file's own. If
		//     the draw covered every row the file is refused. If it was only a
		//     subset (more rows than vectorImportCandidates) the next draw is another
		//     random subset, so one draw proves nothing: the file waits and is
		//     counted (errVectorImportShortDraw); the inbox refuses it when it stays
		//     short vectorInboxShortDraws times in a row.
		//   - Yes: the shortfall may be the host's state, so the file waits and is
		//     not counted, however many rows it has. Nothing bounds this wait: rows
		//     that fail in transport every time while the server is healthy keep the
		//     file in the inbox until a person looks at it (journal line, state retry).
		counts := fmt.Sprintf("%d are too large for this host's embedder, %d failed this call, %d are not found, not public, or cut differently by this host", setAside, skippedNow, fileRejected)
		if checked+skippedNow >= floor {
			return res, fmt.Errorf("%w: only %d rows could be checked, need %d (%s); the file waits", errVectorImportCannotCheck, checked, floor, counts)
		}
		if len(order) < res.Rows {
			return res, fmt.Errorf("%w: %w: only %d rows could be checked, need %d, in a draw of %d of the file's %d rows (%s); the file waits", errVectorImportCannotCheck, errVectorImportShortDraw, checked, floor, len(order), res.Rows, counts)
		}
		return res, fmt.Errorf("%w: only %d rows of the file can be checked against this host's text, need %d (%s; the text or its cut differs, or the rows are too large)", ErrVectorImportRefused, checked, floor, counts)
	}
	if res.MinCosine < vectorImportMinCosine {
		return res, fmt.Errorf("%w: a sample of %d rows (seed %d) agrees only to cosine %.5f, need %.3f", ErrVectorImportRefused, res.Samples, res.SampleSeed, res.MinCosine, vectorImportMinCosine)
	}

	// Pass 3: write. Rows of a story version are contiguous; the version is
	// written whole when every passage is present and matches, else dropped.
	var storyGroup []vectorFileRow
	oversizedID := ""
	var storyVecs [][]float32
	var capBatch []captureVectorRow
	flushStory := func() error {
		if len(storyGroup) == 0 {
			return nil
		}
		defer func() { storyGroup, storyVecs = nil, nil }()
		versionID := storyGroup[0].ID
		story, vt, ok, err := deriver.storyVersion(ctx, versionID)
		if err != nil {
			res.reject(importErrReason(err), len(storyGroup))
			return nil
		}
		if !ok {
			res.reject("not_public", len(storyGroup))
			return nil
		}
		if len(storyGroup) != len(vt.Passages) {
			res.reject("incomplete_version", len(storyGroup))
			return nil
		}
		for _, r := range storyGroup {
			if oversizedKeys[rowKey(r)] {
				// a passage the host's embedder refuses as too large: not the file's
				// fault; the version is left to the worker, which records its own refusal
				res.reject("oversized", len(storyGroup))
				return nil
			}
		}
		byOrd := map[int]int{}
		for i, r := range storyGroup {
			byOrd[r.Ordinal] = i
		}
		rows := make([]storyVectorRow, len(vt.Passages))
		for ord, p := range vt.Passages {
			i, found := byOrd[ord]
			if !found {
				res.reject("incomplete_version", len(storyGroup))
				return nil
			}
			if storyGroup[i].TextSHA != embedder.TextSHA(p) {
				res.reject("text_mismatch", len(storyGroup))
				return nil
			}
			rows[ord] = storyVectorRow{StoryID: story, VersionID: versionID, Ordinal: ord, TextSHA: storyGroup[i].TextSHA, Day: vt.Day, Vec: storyVecs[i]}
		}
		if err := s.store.writeStoryVersionVectors(ctx, story, versionID, rows); err != nil {
			if ctx.Err() != nil {
				return err
			}
			res.reject("write_failed", len(rows))
			log.Printf("corpusd vectors import version=%s not written: %v", versionID, err)
			return nil
		}
		res.Written += len(rows)
		return nil
	}
	flushCaptures := func() error {
		if len(capBatch) == 0 {
			return nil
		}
		defer func() { capBatch = nil }()
		if err := s.store.writeCaptureVectors(ctx, capBatch); err != nil {
			if ctx.Err() != nil {
				return err
			}
			res.reject("write_failed", len(capBatch))
			log.Printf("corpusd vectors import capture batch not written: %v", err)
			return nil
		}
		res.Written += len(capBatch)
		return nil
	}
	err := scanVectorFile(path, func(i int, row vectorFileRow, v []float32) error {
		switch row.Kind {
		case vectorKindStory:
			if row.ID == oversizedID {
				res.reject("oversized_version", 1)
				return nil
			}
			if len(storyGroup) > 0 && storyGroup[0].ID != row.ID {
				if err := flushStory(); err != nil {
					return err
				}
			}
			if len(storyGroup) >= vectorImportMaxGroupRows {
				res.reject("oversized_version", len(storyGroup)+1)
				oversizedID, storyGroup, storyVecs = row.ID, nil, nil
				return nil
			}
			storyGroup = append(storyGroup, row)
			storyVecs = append(storyVecs, v)
		case vectorKindCapture:
			if oversizedKeys[rowKey(row)] {
				res.reject("oversized", 1)
				return nil
			}
			in := deriver.derive(ctx, row)
			switch {
			case in.reason != "":
				res.reject(in.reason, 1)
			case in.TextSHA != row.TextSHA:
				res.reject("text_mismatch", 1)
			default:
				capBatch = append(capBatch, captureVectorRow{ID: row.ID, Owner: in.Owner, Computer: in.Comp, TextSHA: in.TextSHA, Day: in.Day, Vec: v})
				if len(capBatch) >= 200 {
					return flushCaptures()
				}
			}
		}
		return nil
	})
	if err == nil {
		err = flushStory()
	}
	if err == nil {
		err = flushCaptures()
	}
	return res, err
}

// beginVectorJob registers a running job, or says one already runs (the route
// and the inbox share this: one import at a time).
func (s *Service) beginVectorJob(source, file string) (*VectorImportJob, bool) {
	v := &s.store.vec
	v.importMu.Lock()
	defer v.importMu.Unlock()
	if v.jobs == nil {
		v.jobs = map[string]*VectorImportJob{}
	}
	if cur := v.jobs[v.lastJob]; cur != nil && cur.State == "running" {
		return cur, false
	}
	var b [6]byte
	_, _ = rand.Read(b[:])
	job := &VectorImportJob{JobID: "vi-" + time.Now().UTC().Format("20060102T150405Z") + "-" + hex.EncodeToString(b[:]), State: "running", StartedAt: time.Now().UTC(), Source: source, File: file}
	v.jobs[job.JobID] = job
	v.jobOrder = append(v.jobOrder, job.JobID)
	v.lastJob = job.JobID
	// finished jobs are kept to a bounded number, as the tape sweep's are
	for len(v.jobOrder) > vectorJobsKept {
		delete(v.jobs, v.jobOrder[0])
		v.jobOrder = v.jobOrder[1:]
	}
	return job, true
}

// vectorJobsKept is how many job records the process remembers.
const vectorJobsKept = 32

// forgetVectorJob drops a job that did not run to an outcome the operator reads
// (an inbox file that waits): it leaves no entry behind, only its journal line.
func (s *Service) forgetVectorJob(job *VectorImportJob) {
	v := &s.store.vec
	v.importMu.Lock()
	defer v.importMu.Unlock()
	delete(v.jobs, job.JobID)
	for i, id := range v.jobOrder {
		if id == job.JobID {
			v.jobOrder = append(v.jobOrder[:i], v.jobOrder[i+1:]...)
			break
		}
	}
	if v.lastJob == job.JobID {
		v.lastJob = ""
		if n := len(v.jobOrder); n > 0 {
			v.lastJob = v.jobOrder[n-1]
		}
	}
}

// finishVectorJob records a job's outcome and writes its one journal line, the
// same line for both sources:
//
//	corpusd retrieval op=vector_import shape=inbox|route ms= rows= written= refused= sample_min_cosine= err= reasons= job= state= file=
//
// refused is the rows dropped for any reason; reasons names them.
func (s *Service) finishVectorJob(job *VectorImportJob, res VectorImport, err error) VectorImportJob {
	v := &s.store.vec
	v.importMu.Lock()
	defer v.importMu.Unlock()
	now := time.Now().UTC()
	job.FinishedAt = &now
	job.Result = &res
	job.Rejected = res.Rejected
	switch importFate(err) {
	case fateDone:
		job.State = "done"
	case fateRefuse:
		job.State, job.Error = "refused", err.Error()
	default:
		// the route's job is over; an inbox file waits where it is ("retry")
		job.State, job.Error = "failed", err.Error()
		if job.Source == "inbox" {
			job.State = "retry"
		}
	}
	dropped := 0
	var reasons []string
	for k, n := range res.Rejected {
		dropped += n
		reasons = append(reasons, fmt.Sprintf("%s:%d", k, n))
	}
	sort.Strings(reasons)
	errText := strings.NewReplacer(" ", "_", "\n", "_", "\r", "_", "\t", "_").Replace(job.Error)
	if len(errText) > 160 {
		errText = errText[:160]
	}
	log.Printf("corpusd retrieval op=vector_import shape=%s ms=%d rows=%d written=%d refused=%d sample_min_cosine=%.5f err=%s reasons=%s job=%s state=%s file=%s",
		job.Source, now.Sub(job.StartedAt).Milliseconds(), res.Rows, res.Written, dropped, res.MinCosine, errText, strings.Join(reasons, ","), job.JobID, job.State, job.File)
	return *job
}

// StartVectorImport runs ImportVectorFile on path in the background and
// returns the job (the route's form). One import runs at a time.
func (s *Service) StartVectorImport(path string, remove bool) (VectorImportJob, error) {
	job, ok := s.beginVectorJob("route", "")
	if !ok {
		return *job, errors.New("a vector import is already running")
	}
	out := *job
	go func() {
		res, err := s.ImportVectorFile(context.Background(), path)
		if remove {
			_ = os.Remove(path)
		}
		s.finishVectorJob(job, res, err)
		s.store.KickVectors()
	}()
	return out, nil
}

func (s *Service) vectorImportRunning() bool {
	v := &s.store.vec
	v.importMu.Lock()
	defer v.importMu.Unlock()
	cur := v.jobs[v.lastJob]
	return cur != nil && cur.State == "running"
}

// removeStaleVectorSpool deletes posted files left by a process that died
// mid-import (an import resumes nothing; it is posted again). The artifact
// collector sweeps named namespaces only (artifact_gc.go artifactGCNamespaces)
// and never reads this directory.
func (s *Service) removeStaleVectorSpool() {
	if s.vectorImportRunning() {
		return
	}
	dir := filepath.Join(s.artifactsRoot, vectorImportDirName)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasPrefix(e.Name(), "import-") {
			_ = os.Remove(filepath.Join(dir, e.Name()))
		}
	}
}

// VectorImportStatus returns a job by id, or the latest when id is empty.
func (s *Service) VectorImportStatus(id string) (VectorImportJob, bool) {
	v := &s.store.vec
	v.importMu.Lock()
	defer v.importMu.Unlock()
	if id == "" {
		id = v.lastJob
	}
	job := v.jobs[id]
	if job == nil {
		return VectorImportJob{}, false
	}
	return *job, true
}

// HandleVectorImport handles POST (post the file as the body) and GET
// (?job=<id>, the latest when empty) on /internal/platform/vectors/import.
func (h *Handler) HandleVectorImport(w http.ResponseWriter, r *http.Request) {
	if !trustedInternalCaller(r) {
		writeJSON(w, http.StatusForbidden, apiError{Error: "internal caller required"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		job, ok := h.service.VectorImportStatus(strings.TrimSpace(r.URL.Query().Get("job")))
		if !ok {
			writeJSON(w, http.StatusNotFound, apiError{Error: "vector import job not found"})
			return
		}
		writeJSON(w, http.StatusOK, job)
	case http.MethodPost:
		// Refuse before reading the body: a second import waits for the first.
		if h.service.vectorImportRunning() {
			writeJSON(w, http.StatusConflict, apiError{Error: "a vector import is already running"})
			return
		}
		dir := filepath.Join(h.service.artifactsRoot, vectorImportDirName)
		if err := os.MkdirAll(dir, 0o750); err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Error: err.Error()})
			return
		}
		f, err := os.CreateTemp(dir, "import-*.ndjson")
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Error: err.Error()})
			return
		}
		_, err = io.Copy(f, http.MaxBytesReader(w, r.Body, vectorImportMaxBytes))
		cerr := f.Close()
		if err == nil {
			err = cerr
		}
		if err != nil {
			_ = os.Remove(f.Name())
			writeJSON(w, http.StatusBadRequest, apiError{Error: "read body: " + err.Error()})
			return
		}
		job, err := h.service.StartVectorImport(f.Name(), true)
		if err != nil {
			_ = os.Remove(f.Name())
			writeJSON(w, http.StatusConflict, apiError{Error: err.Error()})
			return
		}
		writeJSON(w, http.StatusAccepted, job)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
	}
}
