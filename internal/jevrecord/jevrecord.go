// Package jevrecord is the one record every Jev judgment leaves, success or
// failure, so thresholds can be calibrated from what was asked, what Jev
// answered and what the answer fed (owner, 2026-11-09: "the more we call
// Jev, the better, because that's going to be a key part of our learning
// loop"). It is observation, not authority: the file is a guest-local (or
// host-local) append-only JSONL log, never the tape, because at 500 to 5,000
// calls an hour the record would be 18 to 180 MB a day of replicated tape
// (docs/evidence/jev-measurement-2026-11-10.md, "Where the record lives").
package jevrecord

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

const (
	// Schema names the record version.
	Schema = "choir.jev_call.v1"
	// MaxRecordBytes bounds one record; a larger one sheds its excerpts,
	// then its trailing answers.
	MaxRecordBytes = 8 << 10
	// MaxFileBytes and KeepFiles bound the log: at most KeepFiles files of
	// MaxFileBytes each.
	MaxFileBytes = 32 << 20
	KeepFiles    = 4
	maxQueue     = 2048

	excerptRunes  = 160
	optionsMax    = 12
	topProbs      = 6
	errorRunes    = 200
	requestedName = "~typesafe/jev-latest"
)

// Meta is what the caller knows that the wire does not: which site is asking
// and what the answer will feed.
type Meta struct {
	// Site names the call site (intake_labels, event_layer, support_judge,
	// subcall_judge, desk_cast_judge, inert_judge, system1_verb,
	// prompt_router, disclosure_gate, story_analysis, front_importance).
	Site string
	// DecisionID is the id of the decision the answer feeds: the label's
	// capture, the event chunk's first capture, the revision, the cast, the
	// System 1 request event, the story and revision. A later outcome joins
	// on it.
	DecisionID string
	RunID      string
	Scope      string
	// Threshold is the threshold in force when the answer was read.
	Threshold string
	// NoText leaves the question text and options out of the record (digest
	// and name only): for a site whose questions are authored by an agent,
	// which may quote private text.
	NoText bool
	// QuestionDecisions maps a question's name to the decision it feeds,
	// for a call that carries the questions of several decisions (intake
	// labels put the questions of up to four captures in one call, so one
	// DecisionID cannot join them all). A question absent here feeds
	// DecisionID.
	QuestionDecisions map[string]string
}

// Question is one question as asked: its options, and a digest plus a
// bounded excerpt of its text.
type Question struct {
	Name    string   `json:"name"`
	Type    string   `json:"type,omitempty"`
	Options []string `json:"options,omitempty"`
	// MoreOptions counts options beyond those listed.
	MoreOptions int    `json:"more_options,omitempty"`
	Digest      string `json:"digest"`
	Excerpt     string `json:"excerpt,omitempty"`
	// DecisionID is the decision this question feeds when it differs from
	// the record's (a batched call); empty means the record's.
	DecisionID string `json:"decision_id,omitempty"`
}

// Answer is one question's answer: the chosen option and the top of its
// distribution.
type Answer struct {
	Choice string             `json:"choice,omitempty"`
	P      map[string]float64 `json:"p,omitempty"`
}

// Record is one Jev call.
type Record struct {
	Schema      string            `json:"schema"`
	CallID      string            `json:"call_id"`
	At          string            `json:"at"`
	Site        string            `json:"site"`
	DecisionID  string            `json:"decision_id,omitempty"`
	RunID       string            `json:"run_id,omitempty"`
	Scope       string            `json:"scope,omitempty"`
	Threshold   string            `json:"threshold,omitempty"`
	Questions   []Question        `json:"questions"`
	StateDigest string            `json:"state_digest"`
	StateBytes  int               `json:"state_bytes"`
	Answers     map[string]Answer `json:"answers,omitempty"`
	// AnswersCut counts answers dropped to fit MaxRecordBytes.
	AnswersCut   int     `json:"answers_cut,omitempty"`
	Requested    string  `json:"requested_model"`
	Model        string  `json:"model,omitempty"`
	Provider     string  `json:"provider,omitempty"`
	ResponseID   string  `json:"response_id,omitempty"`
	InputTokens  int     `json:"input_tokens,omitempty"`
	OutputTokens int     `json:"output_tokens,omitempty"`
	CostUSD      float64 `json:"cost_usd,omitempty"`
	LatencyMS    int64   `json:"latency_ms"`
	OK           bool    `json:"ok"`
	Error        string  `json:"error,omitempty"`
}

// Recorder appends records to a rotated JSONL file. Record never blocks the
// caller on the disk: a record goes on a bounded queue (a full queue drops it
// and counts the drop) and a drainer goroutine that exists only while the
// queue is not empty writes it, so there is nothing resident to leak.
type Recorder struct {
	path    string
	maxFile int64
	keep    int

	mu       sync.Mutex
	queue    [][]byte
	draining bool
	closed   bool
	wg       sync.WaitGroup
	dropped  atomic.Int64
}

// NewRecorder records to path (rotated beside it).
func NewRecorder(path string) *Recorder {
	return &Recorder{path: path, maxFile: MaxFileBytes, keep: KeepFiles}
}

// Dropped is how many records a full queue or a closed recorder refused.
func (r *Recorder) Dropped() int64 {
	if r == nil {
		return 0
	}
	return r.dropped.Load()
}

// Record queues rec. It is safe on a nil Recorder.
func (r *Recorder) Record(rec Record) {
	if r == nil || strings.TrimSpace(r.path) == "" {
		return
	}
	line := Encode(rec)
	if line == nil {
		return
	}
	r.mu.Lock()
	if r.closed || len(r.queue) >= maxQueue {
		r.mu.Unlock()
		r.dropped.Add(1)
		return
	}
	r.queue = append(r.queue, line)
	start := !r.draining
	if start {
		r.draining = true
		r.wg.Add(1)
	}
	r.mu.Unlock()
	if start {
		go r.drain()
	}
}

// Close stops accepting records and waits for the queue to be written. A
// crash instead loses the unwritten tail, which observation can afford.
func (r *Recorder) Close() {
	if r == nil {
		return
	}
	r.mu.Lock()
	r.closed = true
	r.mu.Unlock()
	r.wg.Wait()
}

func (r *Recorder) drain() {
	defer r.wg.Done()
	for {
		r.mu.Lock()
		batch := r.queue
		r.queue = nil
		if len(batch) == 0 {
			r.draining = false
			r.mu.Unlock()
			return
		}
		r.mu.Unlock()
		if err := r.write(batch); err != nil {
			log.Printf("jev record: %v (%d records lost)", err, len(batch))
		}
	}
}

func (r *Recorder) write(batch [][]byte) error {
	if err := os.MkdirAll(filepath.Dir(r.path), 0o700); err != nil {
		return err
	}
	var size int64
	if info, err := os.Stat(r.path); err == nil {
		size = info.Size()
	}
	if size >= r.maxFile {
		r.rotate()
	}
	file, err := os.OpenFile(r.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()
	var buf []byte
	for _, line := range batch {
		buf = append(buf, line...)
	}
	_, err = file.Write(buf)
	return err
}

// rotate shifts path -> path.1 -> ... and drops the oldest, keeping r.keep
// files in all.
func (r *Recorder) rotate() {
	_ = os.Remove(fmt.Sprintf("%s.%d", r.path, r.keep-1))
	for i := r.keep - 2; i >= 1; i-- {
		_ = os.Rename(fmt.Sprintf("%s.%d", r.path, i), fmt.Sprintf("%s.%d", r.path, i+1))
	}
	if r.keep > 1 {
		_ = os.Rename(r.path, r.path+".1")
	} else {
		_ = os.Remove(r.path)
	}
}

// Encode renders rec as one JSON line within MaxRecordBytes, or nil.
func Encode(rec Record) []byte {
	line, err := json.Marshal(rec)
	if err != nil {
		return nil
	}
	if len(line)+1 > MaxRecordBytes {
		for i := range rec.Questions {
			rec.Questions[i].Excerpt = ""
		}
		line, _ = json.Marshal(rec)
	}
	if len(line)+1 > MaxRecordBytes && len(rec.Answers) > 0 {
		names := make([]string, 0, len(rec.Answers))
		for name := range rec.Answers {
			names = append(names, name)
		}
		sort.Strings(names)
		for len(line)+1 > MaxRecordBytes && len(names) > 0 {
			delete(rec.Answers, names[len(names)-1])
			names = names[:len(names)-1]
			rec.AnswersCut++
			line, _ = json.Marshal(rec)
		}
	}
	if len(line)+1 > MaxRecordBytes {
		rec.Questions = nil
		line, _ = json.Marshal(rec)
	}
	return append(line, '\n')
}

// Observe makes one judgment call through call and records it, whether it
// succeeds or fails. It is the one function every site's call goes through.
// rec may be nil. The answer and error are returned exactly as call gave
// them.
func Observe(rec *Recorder, meta Meta, state json.RawMessage, questions map[string]json.RawMessage, call func() (json.RawMessage, error)) (json.RawMessage, error) {
	started := time.Now()
	raw, err := call()
	if rec != nil {
		record := Build(meta, state, questions, raw, err, started, time.Since(started))
		rec.Record(record)
	}
	return raw, err
}

// Build assembles the record of one call.
func Build(meta Meta, state json.RawMessage, questions map[string]json.RawMessage, raw json.RawMessage, callErr error, started time.Time, latency time.Duration) Record {
	rec := Record{
		Schema: Schema, CallID: uuid.New().String(), At: started.UTC().Format(time.RFC3339Nano),
		Site: meta.Site, DecisionID: meta.DecisionID, RunID: meta.RunID, Scope: meta.Scope, Threshold: meta.Threshold,
		Questions: describeQuestions(questions, meta.NoText, meta.QuestionDecisions), StateDigest: digest(state), StateBytes: len(state),
		Requested: requestedName, LatencyMS: latency.Milliseconds(), OK: callErr == nil,
	}
	if callErr != nil {
		rec.Error = clip(callErr.Error(), errorRunes)
		return rec
	}
	var dist struct {
		Model    string `json:"model"`
		Provider string `json:"provider"`
		ID       string `json:"id"`
		Answers  map[string]struct {
			Choice        string             `json:"choice"`
			Probabilities map[string]float64 `json:"probabilities"`
		} `json:"answers"`
		Usage struct {
			InputTokens      int     `json:"input_tokens"`
			OutputTokens     int     `json:"output_tokens"`
			PromptTokens     int     `json:"prompt_tokens"`
			CompletionTokens int     `json:"completion_tokens"`
			Cost             float64 `json:"cost"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(raw, &dist); err != nil {
		// The call succeeded but its answer did not decode: kept as a
		// failure of the reading, with the digest of what came back.
		rec.OK, rec.Error = false, "answer did not decode: "+clip(err.Error(), errorRunes)
		return rec
	}
	rec.Model, rec.Provider, rec.ResponseID = dist.Model, dist.Provider, dist.ID
	rec.InputTokens, rec.OutputTokens, rec.CostUSD = dist.Usage.InputTokens, dist.Usage.OutputTokens, dist.Usage.Cost
	if rec.InputTokens == 0 && rec.OutputTokens == 0 {
		rec.InputTokens, rec.OutputTokens = dist.Usage.PromptTokens, dist.Usage.CompletionTokens
	}
	rec.Answers = make(map[string]Answer, len(dist.Answers))
	for name, a := range dist.Answers {
		rec.Answers[name] = Answer{Choice: a.Choice, P: topP(a.Probabilities, a.Choice)}
	}
	return rec
}

func describeQuestions(questions map[string]json.RawMessage, noText bool, decisions map[string]string) []Question {
	names := make([]string, 0, len(questions))
	for name := range questions {
		names = append(names, name)
	}
	sort.Strings(names)
	out := make([]Question, 0, len(names))
	for _, name := range names {
		raw := questions[name]
		var shape struct {
			Type         string   `json:"type"`
			Instructions string   `json:"instructions"`
			Text         string   `json:"text"`
			Choices      []string `json:"choices"`
			// Criteria is the vendor's option set: its keys are the options.
			Criteria json.RawMessage `json:"criteria"`
		}
		_ = json.Unmarshal(raw, &shape)
		if len(shape.Choices) == 0 {
			shape.Choices = objectKeys(shape.Criteria)
		}
		q := Question{Name: name, Type: shape.Type, DecisionID: decisions[name], Digest: digest(raw), Excerpt: clip(strings.Join(strings.Fields(firstNonEmpty(shape.Instructions, shape.Text)), " "), excerptRunes)}
		if noText {
			q.Excerpt = ""
			q.MoreOptions = len(shape.Choices)
			out = append(out, q)
			continue
		}
		q.Options = shape.Choices
		if len(q.Options) > optionsMax {
			q.MoreOptions = len(q.Options) - optionsMax
			q.Options = q.Options[:optionsMax]
		}
		out = append(out, q)
	}
	return out
}

// objectKeys are a JSON object's keys in the order written, or nil.
func objectKeys(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return nil
	}
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	if tok, err := dec.Token(); err != nil || tok != json.Delim('{') {
		return nil
	}
	var keys []string
	for dec.More() {
		k, err := dec.Token()
		if err != nil {
			return keys
		}
		name, _ := k.(string)
		keys = append(keys, name)
		var skip json.RawMessage
		if dec.Decode(&skip) != nil {
			return keys
		}
	}
	return keys
}

func topP(p map[string]float64, chosen string) map[string]float64 {
	if len(p) <= topProbs {
		return p
	}
	keys := make([]string, 0, len(p))
	for k := range p {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if p[keys[i]] != p[keys[j]] {
			return p[keys[i]] > p[keys[j]]
		}
		return keys[i] < keys[j]
	})
	out := make(map[string]float64, topProbs)
	for _, k := range keys[:topProbs] {
		out[k] = p[k]
	}
	if v, ok := p[chosen]; ok {
		out[chosen] = v
	}
	return out
}

func digest(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:8])
}

func clip(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n]) + "…"
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
