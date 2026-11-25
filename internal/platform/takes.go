package platform

// Takes in the Autopaper object graph (docs/design/autoradio-learned-document-
// 2026-11-17.md section 11, as built: docs/reference/AUTORADIO-TAKES.md).
//
// A take is a listener's recorded voice. Three kinds, written by corpusd and
// nobody else: choir.take (the metadata, the transcript and the state),
// choir.take_audio (the bytes, kept apart so a list never reads them) and
// choir.take_outcome (one row per listener per take: heard or skipped, the
// trust stroke across listeners).
//
// Rule 1 of the learning engine: only another listener's act settles. An
// outcome is refused when it comes from the publisher's own account, only the
// first outcome of a listener for a take stands, and an outcome from a drill
// computer is stored with drill=true and never counted.

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/yusefmosiah/go-choir/internal/computertiers"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

const (
	ogKindTake        = objectgraph.ObjectKind("choir.take")
	ogKindTakeAudio   = objectgraph.ObjectKind("choir.take_audio")
	ogKindTakeOutcome = objectgraph.ObjectKind("choir.take_outcome")

	TakeMinSeconds = 3.0
	TakeMaxSeconds = 60.0
	TakeMaxBytes   = 512 << 10
	// TakeDailyLimit is how many takes one publisher may publish in 24 hours.
	// A bound on what strangers can cause, not on our own model calls.
	TakeDailyLimit       = 10
	TakeTranscriptMax    = 2000
	takeListWindow       = 7 * 24 * time.Hour
	takeListDefaultLimit = 50
	takeListMaxLimit     = 100
	takeDurationSlack    = 2.0 // seconds the header may differ from the claim
	// The header's own length may sit a little outside the claim's 3 to 60 s:
	// Apple's AAC encoder adds about 0.11 s of priming to the container (60.000 s
	// of audio has an mvhd of 60.117 s) and a recorder stops a moment after its
	// timer. An honest full-length take must never be refused for container
	// padding or stop latency, so the header may be 2.5 to 62.0 s while the
	// claim stays 3 to 60 and the claim-to-header slack stays 2 s.
	TakeHeaderMinSeconds = types.TakeHeaderMinSeconds
	TakeHeaderMaxSeconds = types.TakeHeaderMaxSeconds
	// takeScan bounds the rows one scan reads (live takes, one publisher's
	// takes); a scan that reaches it is logged, never silent.
	takeScan        = 2000
	takeOutcomeScan = 5000
	takeRespondsMax = 300

	TakeStateLive    = "live"
	TakeStateDeleted = "deleted"
	TakeStateYanked  = "yanked"

	TakeHeard   = "heard"
	TakeSkipped = "skipped"
)

var (
	ErrTakeInvalid    = errors.New("invalid take")
	ErrTakeDailyLimit = errors.New("daily take limit reached")
	ErrTakeNotFound   = errors.New("take not found")
	ErrTakeGone       = errors.New("take is deleted or yanked")
	ErrTakeOwn        = errors.New("a publisher's own plays never count")
	ErrTakeRemoved    = errors.New("this take was removed and is not published again")
	// ErrTakeUnavailable is a correctness refusal: the service cannot tell
	// who a listener is, or whether a computer is a drill computer, so it
	// counts nothing rather than guess.
	ErrTakeUnavailable = errors.New("takes cannot count this outcome right now")
)

var takeClaimIDRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:/@-]{7,199}$`)

func takeInvalid(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrTakeInvalid, fmt.Sprintf(format, args...))
}

// TakeRespondsTo is the story on air when a take was recorded: its route and
// title, kept for provenance.
type TakeRespondsTo struct {
	Route string `json:"route"`
	Title string `json:"title"`
}

// TakeIn is a publish request, as the proxy builds it from the caller's
// credential and the multipart the phone sent.
type TakeIn struct {
	PublisherKey    string          `json:"publisher_key"`
	Publisher       string          `json:"publisher"`
	DurationSeconds float64         `json:"duration_seconds"`
	SHA256          string          `json:"sha256"`
	AudioBase64     string          `json:"audio_base64"`
	Transcript      string          `json:"transcript"`
	RespondsTo      *TakeRespondsTo `json:"responds_to,omitempty"`
	// Drill marks a take made by the proof script: never counted anywhere.
	Drill bool `json:"drill,omitempty"`
}

// takeRow is a stored choir.take.
type takeRow struct {
	TakeID          string          `json:"take_id"`
	Publisher       string          `json:"publisher"`
	PublisherKey    string          `json:"publisher_key"`
	PublishedAt     string          `json:"published_at"`
	DurationSeconds float64         `json:"duration_seconds"`
	SHA256          string          `json:"sha256"`
	Bytes           int             `json:"bytes"`
	Transcript      string          `json:"transcript"`
	RespondsTo      *TakeRespondsTo `json:"responds_to,omitempty"`
	State           string          `json:"state"`
	Drill           bool            `json:"drill,omitempty"`
}

type takeAudioRow struct {
	AudioBase64 string `json:"audio_base64"`
}

type takeOutcomeRow struct {
	TakeID    string `json:"take_id"`
	Publisher string `json:"publisher"`
	// PublisherKey keys the record: the handle is display only, and two
	// publishers can share one.
	PublisherKey string `json:"publisher_key"`
	ListenerKey  string `json:"listener_key"`
	Outcome      string `json:"outcome"`
	At           string `json:"at"`
	Drill        bool   `json:"drill"`
	ClaimID      string `json:"claim_id,omitempty"`
}

// Take is the public shape of a take: every field of choir.take except the
// publisher key, plus where the bytes are and the publisher's record.
type Take struct {
	TakeID          string             `json:"take_id"`
	Publisher       string             `json:"publisher"`
	PublishedAt     string             `json:"published_at"`
	DurationSeconds float64            `json:"duration_seconds"`
	SHA256          string             `json:"sha256"`
	Bytes           int                `json:"bytes"`
	Transcript      string             `json:"transcript"`
	RespondsTo      *TakeRespondsTo    `json:"responds_to,omitempty"`
	State           string             `json:"state"`
	AudioURL        string             `json:"audio_url,omitempty"`
	Record          *TakeRecordSummary `json:"record,omitempty"`
	// Mine is the publisher's own view of this one take: its outcomes.
	Outcomes *TakeOutcomeCounts `json:"outcomes,omitempty"`
}

// TakesResponse is the list route's answer.
type TakesResponse struct {
	GeneratedAt time.Time `json:"generated_at"`
	Takes       []Take    `json:"takes"`
}

// TakeOutcomeCounts are the counted and the drill outcomes of one take.
type TakeOutcomeCounts struct {
	Heard        int `json:"heard"`
	Skipped      int `json:"skipped"`
	DrillHeard   int `json:"drill_heard"`
	DrillSkipped int `json:"drill_skipped"`
}

// TakeRecordSummary is a publisher's public record: counts by distinct
// listener-and-take, computed from other listeners' outcomes only. Drill rows
// are never in these numbers.
type TakeRecordSummary struct {
	Publisher string `json:"publisher"`
	New       bool   `json:"new"`
	// Offered is the outcomes reported: forecasts made on a take of this
	// publisher that settled heard or skipped.
	Offered   int `json:"offered"`
	Heard     int `json:"heard"`
	Skipped   int `json:"skipped"`
	Listeners int `json:"listeners"`
	Takes     int `json:"takes"`
	// Text is the line the DJ reads (JSON name summary).
	Text string `json:"summary"`
}

// TakeOutcomeView is one row about a publisher's takes, read by the publisher
// only: an opaque listener key, never an identity.
type TakeOutcomeView struct {
	TakeID      string `json:"take_id"`
	ListenerKey string `json:"listener_key"`
	Outcome     string `json:"outcome"`
	At          string `json:"at"`
	Drill       bool   `json:"drill"`
}

// TakeRecord is the publisher's whole record.
type TakeRecord struct {
	// Summary is flattened into the record: offered, heard, skipped,
	// listeners, takes, new and the one-line summary, as the phone reads it.
	Summary TakeRecordSummary `json:"-"`
	// Drill is the separate drill line: outcomes from drill computers.
	Drill TakeOutcomeCounts `json:"drill"`
	// Rows are every outcome about the publisher's takes; present only when
	// the caller is the publisher.
	Rows []TakeOutcomeView `json:"rows,omitempty"`
}

// MarshalJSON flattens the summary into the record.
func (r TakeRecord) MarshalJSON() ([]byte, error) {
	type flat struct {
		TakeRecordSummary
		Drill TakeOutcomeCounts `json:"drill"`
		Rows  []TakeOutcomeView `json:"rows,omitempty"`
	}
	return json.Marshal(flat{r.Summary, r.Drill, r.Rows})
}

// TakeOutcomeIn is a listener's report.
type TakeOutcomeIn struct {
	TakeID string `json:"take_id"`
	// ListenerUserKey is TakePublisherKey of the reporting account: compared
	// with the publisher's key, never stored.
	ListenerUserKey  string `json:"listener_user_key"`
	ListenerComputer string `json:"listener_computer_id"`
	Outcome          string `json:"outcome"`
	ClaimID          string `json:"claim_id"`
}

func takeID(publisherKey, sha string) string {
	sum := sha256.Sum256([]byte("choir-take:" + publisherKey + ":" + sha))
	return "take-" + hex.EncodeToString(sum[:])[:16]
}

func (s *Service) takeStore() (*ObjectGraphStore, error) { return s.podcastStore() }

// takeListenerKey is the opaque, stable key of a listener's computer: an HMAC
// under the platform signing key. Without a real key there is no honest key to
// give, so it refuses (never a hash under a constant).
func (s *Service) takeListenerKey(computerID string) (string, error) {
	if s == nil || s.signingKey == nil || len(s.signingKey.Private) == 0 {
		return "", fmt.Errorf("%w: no signing key", ErrTakeUnavailable)
	}
	m := hmac.New(sha256.New, s.signingKey.Private)
	m.Write([]byte("take-listener:" + computerID))
	return hex.EncodeToString(m.Sum(nil)), nil
}

var (
	takeTiersMu     sync.Mutex
	takeTiers       computertiers.Tiers
	takeTiersLoaded bool
)

// TakeDrillComputer reports whether a computer is a drill computer: a named
// test computer or the operator proof computer (d@e). When the tier file
// cannot be read it returns an error (ErrTakeUnavailable) and the caller
// refuses: a drill computer is never counted as a real one. A failed load is
// not remembered.
func TakeDrillComputer(computerID string) (bool, error) {
	takeTiersMu.Lock()
	defer takeTiersMu.Unlock()
	if !takeTiersLoaded {
		t, err := computertiers.Load()
		if err != nil {
			return false, fmt.Errorf("%w: computer tiers: %v", ErrTakeUnavailable, err)
		}
		takeTiers, takeTiersLoaded = t, true
	}
	return takeTiers.TestComputer(computerID) || takeTiers.OperatorProofComputer(computerID), nil
}

// takeAudioSeconds reads the length from the file's own header: the mvhd box
// of an ISO base media file (an .m4a). It also refuses a file that is not one.
func takeAudioSeconds(b []byte) (float64, error) {
	if len(b) < 16 || string(b[4:8]) != "ftyp" {
		return 0, takeInvalid("the audio is not an mp4 file (no ftyp box)")
	}
	moov, ok := mp4Child(b, "moov")
	if !ok {
		return 0, takeInvalid("the audio has no moov box")
	}
	mvhd, ok := mp4Child(moov, "mvhd")
	if !ok || len(mvhd) < 20 {
		return 0, takeInvalid("the audio has no mvhd header")
	}
	var scale, dur uint64
	switch mvhd[0] {
	case 0:
		if len(mvhd) < 20 {
			return 0, takeInvalid("short mvhd")
		}
		scale, dur = uint64(binary.BigEndian.Uint32(mvhd[12:16])), uint64(binary.BigEndian.Uint32(mvhd[16:20]))
	case 1:
		if len(mvhd) < 32 {
			return 0, takeInvalid("short mvhd")
		}
		scale, dur = uint64(binary.BigEndian.Uint32(mvhd[20:24])), binary.BigEndian.Uint64(mvhd[24:32])
	default:
		return 0, takeInvalid("unknown mvhd version")
	}
	if scale == 0 {
		return 0, takeInvalid("the audio header has a zero timescale")
	}
	return float64(dur) / float64(scale), nil
}

// mp4Child returns the payload of the first box named name at this level.
func mp4Child(b []byte, name string) ([]byte, bool) {
	for len(b) >= 8 {
		size := uint64(binary.BigEndian.Uint32(b[0:4]))
		head := 8
		if size == 1 && len(b) >= 16 {
			size, head = binary.BigEndian.Uint64(b[8:16]), 16
		}
		if size == 0 {
			size = uint64(len(b))
		}
		if size < uint64(head) || size > uint64(len(b)) {
			return nil, false
		}
		if string(b[4:8]) == name {
			return b[head:size], true
		}
		b = b[size:]
	}
	return nil, false
}

func (s *Service) getTakeRow(ctx context.Context, og *ObjectGraphStore, id string) (*takeRow, error) {
	canonical, err := podcastCanonical(ogKindTake, id)
	if err != nil {
		return nil, err
	}
	obj, err := og.GetObject(ctx, canonical)
	if err != nil {
		if isNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	if obj.Tombstone {
		return nil, nil
	}
	var row takeRow
	if err := json.Unmarshal(obj.Body, &row); err != nil {
		return nil, fmt.Errorf("take %s: %w", id, err)
	}
	return &row, nil
}

func (s *Service) putTakeRow(ctx context.Context, og *ObjectGraphStore, row takeRow, now time.Time) error {
	obj, err := podcastObject(ogKindTake, row.TakeID, row, map[string]any{
		"publisher_key": row.PublisherKey, "publisher": row.Publisher, "state": row.State, "published_at": row.PublishedAt,
	}, now)
	if err != nil {
		return err
	}
	return og.PutObject(corpusdWriter(ctx), obj)
}

func (r takeRow) public() Take {
	t := Take{TakeID: r.TakeID, Publisher: r.Publisher, PublishedAt: r.PublishedAt, DurationSeconds: r.DurationSeconds,
		SHA256: r.SHA256, Bytes: r.Bytes, Transcript: r.Transcript, RespondsTo: r.RespondsTo, State: r.State}
	if r.State == TakeStateLive {
		t.AudioURL = "/api/platform/takes/" + r.TakeID + "/audio"
	}
	return t
}

// PublishTake checks and stores one take. The same audio by the same
// publisher is the same take: a repeat of a live one answers it again.
func (s *Service) PublishTake(ctx context.Context, in TakeIn) (*Take, error) {
	if len(in.PublisherKey) != 64 || !types.TakePublisherRe.MatchString(in.Publisher) {
		return nil, takeInvalid("the publisher is not named")
	}
	if !validFileCASDigest(in.SHA256) {
		return nil, takeInvalid("sha256 must be 64 lower-case hex digits")
	}
	if len(in.AudioBase64) > base64.StdEncoding.EncodedLen(TakeMaxBytes) {
		return nil, takeInvalid("the audio is over %d bytes", TakeMaxBytes)
	}
	audio, err := base64.StdEncoding.DecodeString(in.AudioBase64)
	if err != nil || len(audio) == 0 {
		return nil, takeInvalid("the audio is not base64")
	}
	if len(audio) > TakeMaxBytes {
		return nil, takeInvalid("the audio is over %d bytes", TakeMaxBytes)
	}
	if sum := sha256.Sum256(audio); hex.EncodeToString(sum[:]) != in.SHA256 {
		return nil, takeInvalid("the audio does not match its sha256")
	}
	secs, err := takeAudioSeconds(audio)
	if err != nil {
		return nil, err
	}
	if math.IsNaN(in.DurationSeconds) || math.IsInf(in.DurationSeconds, 0) || in.DurationSeconds < TakeMinSeconds || in.DurationSeconds > TakeMaxSeconds {
		return nil, takeInvalid("duration_seconds must be %.0f to %.0f", TakeMinSeconds, TakeMaxSeconds)
	}
	if secs < TakeHeaderMinSeconds || secs > TakeHeaderMaxSeconds {
		return nil, takeInvalid("the audio header says %.1f seconds, outside %.1f to %.1f", secs, TakeHeaderMinSeconds, TakeHeaderMaxSeconds)
	}
	if math.Abs(secs-in.DurationSeconds) > takeDurationSlack {
		return nil, takeInvalid("duration_seconds %.1f does not match the audio header's %.1f", in.DurationSeconds, secs)
	}
	transcript := strings.TrimSpace(in.Transcript)
	if transcript == "" || len(transcript) > TakeTranscriptMax || !utf8.ValidString(transcript) || strings.ContainsRune(transcript, 0) {
		return nil, takeInvalid("the transcript must be 1 to %d bytes of text", TakeTranscriptMax)
	}
	var responds *TakeRespondsTo
	if r := in.RespondsTo; r != nil {
		route, err := boundField("responds_to.route", r.Route, takeRespondsMax)
		if err == nil && route == "" {
			err = podcastInvalid("responds_to.route is empty")
		}
		title := ""
		if err == nil {
			title, err = boundField("responds_to.title", r.Title, takeRespondsMax)
		}
		if err != nil {
			return nil, takeInvalid("%v", err)
		}
		responds = &TakeRespondsTo{Route: route, Title: title}
	}
	og, err := s.takeStore()
	if err != nil {
		return nil, err
	}
	s.takeMu.Lock()
	defer s.takeMu.Unlock()
	id := takeID(in.PublisherKey, in.SHA256)
	if existing, err := s.getTakeRow(ctx, og, id); err != nil {
		return nil, err
	} else if existing != nil {
		if existing.State != TakeStateLive {
			return nil, ErrTakeRemoved
		}
		t := existing.public()
		return &t, nil
	}
	mine, err := og.ListObjectsByMetadata(ctx, string(ogKindTake), "$.publisher_key", in.PublisherKey, takeScan)
	if err != nil {
		return nil, err
	}
	if len(mine) >= takeScan {
		takeBoundLog("corpusd: takes: one publisher's takes reached the scan bound %d; the daily count may be low", takeScan)
	}
	cutoff := time.Now().UTC().Add(-24 * time.Hour)
	recent := 0
	for _, o := range mine {
		var r takeRow
		if json.Unmarshal(o.Body, &r) != nil || r.Drill {
			continue
		}
		if at, err := time.Parse(time.RFC3339, r.PublishedAt); err == nil && at.After(cutoff) {
			recent++
		}
	}
	if !in.Drill && recent >= TakeDailyLimit {
		return nil, fmt.Errorf("%w: %d takes in the last 24 hours, the limit is %d", ErrTakeDailyLimit, recent, TakeDailyLimit)
	}
	now := time.Now().UTC()
	row := takeRow{TakeID: id, Publisher: in.Publisher, PublisherKey: in.PublisherKey, PublishedAt: now.Format(time.RFC3339),
		DurationSeconds: math.Round(secs*10) / 10, SHA256: in.SHA256, Bytes: len(audio), Transcript: transcript,
		RespondsTo: responds, State: TakeStateLive, Drill: in.Drill}
	audioObj, err := podcastObject(ogKindTakeAudio, id, takeAudioRow{AudioBase64: in.AudioBase64}, map[string]any{"take_id": id}, now)
	if err != nil {
		return nil, err
	}
	// The bytes first: a take row never names audio that is not there.
	if err := og.PutObject(corpusdWriter(ctx), audioObj); err != nil {
		return nil, err
	}
	if err := s.putTakeRow(ctx, og, row, now); err != nil {
		return nil, err
	}
	t := row.public()
	return &t, nil
}

// takeBoundLog logs that a scan reached its bound, at most once a minute.
var (
	takeBoundLogMu   sync.Mutex
	takeBoundLogLast time.Time
)

func takeBoundLog(format string, args ...any) {
	takeBoundLogMu.Lock()
	defer takeBoundLogMu.Unlock()
	if time.Since(takeBoundLogLast) < time.Minute {
		return
	}
	takeBoundLogLast = time.Now()
	log.Printf(format, args...)
}

// liveTakes reads the live takes of the window by their state metadata, newest
// first (the store orders by updated_at, and a live take is written once, so
// that is publication order), so deleted and yanked rows never take a place in
// the window. It asks the store for a small number of rows first and widens only
// when drill rows or the window's cut leave fewer than limit: a list of 50 does
// not hydrate 2,000 bodies. The widest scan is takeScan; reaching it is logged.
func (s *Service) liveTakes(ctx context.Context, og *ObjectGraphStore, limit int, includeDrill bool, cutoff time.Time) ([]takeRow, error) {
	for n := limit * 2; ; n *= 4 {
		if n > takeScan {
			n = takeScan
		}
		objs, err := og.ListObjectsByMetadata(ctx, string(ogKindTake), "$.state", TakeStateLive, n)
		if err != nil {
			return nil, err
		}
		var out []takeRow
		for _, o := range objs {
			var r takeRow
			if o.Tombstone || json.Unmarshal(o.Body, &r) != nil || r.State != TakeStateLive || (r.Drill && !includeDrill) {
				continue
			}
			if at, err := time.Parse(time.RFC3339, r.PublishedAt); err != nil || at.Before(cutoff) {
				continue
			}
			out = append(out, r)
		}
		sort.SliceStable(out, func(i, j int) bool { return out[i].PublishedAt > out[j].PublishedAt })
		if len(out) >= limit || len(objs) < n {
			return out, nil
		}
		if n >= takeScan {
			takeBoundLog("corpusd: takes: the live-take scan reached its bound %d; older live takes are not listed", takeScan)
			return out, nil
		}
	}
}

// ListTakes is the public list: the newest live takes of the last 7 days,
// each with its publisher's record summary. A drill take is listed only to a
// drill computer, so a real listener is never offered one.
func (s *Service) ListTakes(ctx context.Context, limit int, includeDrill bool) (*TakesResponse, error) {
	if limit <= 0 {
		limit = takeListDefaultLimit
	}
	if limit > takeListMaxLimit {
		limit = takeListMaxLimit
	}
	og, err := s.takeStore()
	if err != nil {
		return nil, err
	}
	cutoff := time.Now().UTC().Add(-takeListWindow)
	rows, err := s.liveTakes(ctx, og, limit, includeDrill, cutoff)
	if err != nil {
		return nil, err
	}
	records := map[string]*TakeRecordSummary{}
	resp := &TakesResponse{GeneratedAt: time.Now().UTC(), Takes: []Take{}}
	for _, r := range rows {
		if len(resp.Takes) >= limit {
			break
		}
		t := r.public()
		rec, ok := records[r.PublisherKey]
		if !ok {
			full, err := s.takeRecord(ctx, og, r.Publisher, r.PublisherKey, false)
			if err != nil {
				return nil, err
			}
			rec = &full.Summary
			records[r.PublisherKey] = rec
		}
		t.Record = rec
		resp.Takes = append(resp.Takes, t)
	}
	return resp, nil
}

// ListMyTakes is the publisher's own list: their live takes, newest first,
// each with the outcomes it has.
func (s *Service) ListMyTakes(ctx context.Context, publisherKey string) (*TakesResponse, error) {
	if len(publisherKey) != 64 {
		return nil, takeInvalid("the publisher is not named")
	}
	og, err := s.takeStore()
	if err != nil {
		return nil, err
	}
	objs, err := og.ListObjectsByMetadata(ctx, string(ogKindTake), "$.publisher_key", publisherKey, takeScan)
	if err != nil {
		return nil, err
	}
	var rows []takeRow
	for _, o := range objs {
		var r takeRow
		if json.Unmarshal(o.Body, &r) == nil && r.State == TakeStateLive {
			rows = append(rows, r)
		}
	}
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].PublishedAt > rows[j].PublishedAt })
	resp := &TakesResponse{GeneratedAt: time.Now().UTC(), Takes: []Take{}}
	per := map[string]*TakeOutcomeCounts{}
	if len(rows) > 0 {
		// One publisher: one record read covers every take of theirs.
		full, err := s.takeRecord(ctx, og, rows[0].Publisher, publisherKey, true)
		if err != nil {
			return nil, err
		}
		for _, row := range full.Rows {
			c := per[row.TakeID]
			if c == nil {
				c = &TakeOutcomeCounts{}
				per[row.TakeID] = c
			}
			switch {
			case row.Drill && row.Outcome == TakeHeard:
				c.DrillHeard++
			case row.Drill:
				c.DrillSkipped++
			case row.Outcome == TakeHeard:
				c.Heard++
			default:
				c.Skipped++
			}
		}
	}
	for _, r := range rows {
		t := r.public()
		t.Outcomes = per[r.TakeID]
		if t.Outcomes == nil {
			t.Outcomes = &TakeOutcomeCounts{}
		}
		resp.Takes = append(resp.Takes, t)
	}
	return resp, nil
}

// TakeAudio returns a live take's bytes; (nil, nil) when it is not live.
func (s *Service) TakeAudio(ctx context.Context, id string) ([]byte, error) {
	if !types.TakeIDRe.MatchString(id) {
		return nil, nil
	}
	og, err := s.takeStore()
	if err != nil {
		return nil, err
	}
	row, err := s.getTakeRow(ctx, og, id)
	if err != nil || row == nil || row.State != TakeStateLive {
		return nil, err
	}
	canonical, err := podcastCanonical(ogKindTakeAudio, id)
	if err != nil {
		return nil, err
	}
	obj, err := og.GetObject(ctx, canonical)
	if err != nil {
		if isNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	var a takeAudioRow
	if err := json.Unmarshal(obj.Body, &a); err != nil {
		return nil, err
	}
	return base64.StdEncoding.DecodeString(a.AudioBase64)
}

func (s *Service) removeTake(ctx context.Context, og *ObjectGraphStore, row *takeRow, state string) error {
	canonical, err := podcastCanonical(ogKindTakeAudio, row.TakeID)
	if err != nil {
		return err
	}
	// The bodies the rows had in the artifact store, read before the rows are
	// rewritten: a body of 1 KiB or more lives there, and would stay until the
	// GC's seven day grace if it were only left unreferenced.
	var released []string
	if takeCanonical, cerr := podcastCanonical(ogKindTake, row.TakeID); cerr == nil {
		released = append(released, og.bodyRefOf(ctx, takeCanonical))
	}
	released = append(released, og.bodyRefOf(ctx, canonical))
	row.State = state
	// What is not needed for counts goes with the take: the transcript and
	// the story it answered. The row keeps its id, handle, times, size, hash
	// and state; the outcome rows stay as counts.
	row.Transcript, row.RespondsTo = "", nil
	if err := s.putTakeRow(ctx, og, *row, time.Now().UTC()); err != nil {
		return err
	}
	// The bytes are deleted; the outcome rows stay as counts.
	if err := og.DeleteObject(ctx, canonical); err != nil {
		return err
	}
	og.releaseBodyRefs(released...)
	return nil
}

// finishRemovedTake removes what a take's removal leaves behind when it was
// interrupted: the audio row and its artifact-store body. A take already fully
// removed has nothing to do.
func (s *Service) finishRemovedTake(ctx context.Context, og *ObjectGraphStore, row *takeRow) error {
	canonical, err := podcastCanonical(ogKindTakeAudio, row.TakeID)
	if err != nil {
		return err
	}
	ref := og.bodyRefOf(ctx, canonical)
	if _, err := og.GetObject(ctx, canonical); err != nil {
		if isNotFound(err) {
			return nil
		}
		return err
	}
	if err := og.DeleteObject(ctx, canonical); err != nil {
		return err
	}
	og.releaseBodyRefs(ref)
	return nil
}

// DeleteTake lets a publisher delete their own take. A take that is another
// publisher's answers as not found.
func (s *Service) DeleteTake(ctx context.Context, id, publisherKey string) error {
	og, err := s.takeStore()
	if err != nil {
		return err
	}
	s.takeMu.Lock()
	defer s.takeMu.Unlock()
	row, err := s.getTakeRow(ctx, og, id)
	if err != nil {
		return err
	}
	if row == nil || row.PublisherKey != publisherKey || len(publisherKey) != 64 {
		return ErrTakeNotFound
	}
	if row.State != TakeStateLive {
		return nil
	}
	return s.removeTake(ctx, og, row, TakeStateDeleted)
}

// YankTake is the operator's removal of any take.
func (s *Service) YankTake(ctx context.Context, id string) error {
	og, err := s.takeStore()
	if err != nil {
		return err
	}
	s.takeMu.Lock()
	defer s.takeMu.Unlock()
	row, err := s.getTakeRow(ctx, og, id)
	if err != nil {
		return err
	}
	if row == nil {
		return ErrTakeNotFound
	}
	if row.State == TakeStateYanked {
		return nil
	}
	return s.removeTake(ctx, og, row, TakeStateYanked)
}

// RecordTakeOutcome stores one listener's outcome for one take. The first
// outcome of a listener for a take stands; a second changes nothing and says
// so (stored=false).
func (s *Service) RecordTakeOutcome(ctx context.Context, in TakeOutcomeIn) (stored bool, err error) {
	if !types.TakeIDRe.MatchString(in.TakeID) {
		return false, takeInvalid("take_id is not a take id")
	}
	if in.Outcome != TakeHeard && in.Outcome != TakeSkipped {
		return false, takeInvalid("outcome must be heard or skipped")
	}
	if !takeClaimIDRe.MatchString(in.ClaimID) {
		return false, takeInvalid("claim_id is not a claim id")
	}
	if strings.TrimSpace(in.ListenerComputer) == "" || len(in.ListenerUserKey) != 64 {
		return false, takeInvalid("the listener is not named")
	}
	og, err := s.takeStore()
	if err != nil {
		return false, err
	}
	row, err := s.getTakeRow(ctx, og, in.TakeID)
	if err != nil {
		return false, err
	}
	if row == nil {
		return false, ErrTakeNotFound
	}
	if row.State != TakeStateLive {
		return false, ErrTakeGone
	}
	if row.PublisherKey == in.ListenerUserKey {
		return false, ErrTakeOwn
	}
	lk, err := s.takeListenerKey(in.ListenerComputer)
	if err != nil {
		return false, err
	}
	listenerDrill, err := TakeDrillComputer(in.ListenerComputer)
	if err != nil {
		return false, err
	}
	out := takeOutcomeRow{TakeID: in.TakeID, Publisher: row.Publisher, PublisherKey: row.PublisherKey, ListenerKey: lk, Outcome: in.Outcome,
		At: time.Now().UTC().Format(time.RFC3339), Drill: row.Drill || listenerDrill, ClaimID: in.ClaimID}
	obj, err := podcastObject(ogKindTakeOutcome, in.TakeID+"|"+lk, out, map[string]any{
		"take_id": in.TakeID, "publisher": row.Publisher, "publisher_key": row.PublisherKey, "drill": out.Drill,
	}, time.Now().UTC())
	if err != nil {
		return false, err
	}
	return og.InsertObjectIfAbsent(corpusdWriter(ctx), obj)
}

// TakeRecord is a publisher's record by handle. withRows is set only for the
// publisher themselves.
func (s *Service) TakeRecord(ctx context.Context, publisher, asPublisherKey string) (*TakeRecord, error) {
	if !types.TakePublisherRe.MatchString(publisher) {
		return nil, takeInvalid("publisher is not a publisher handle")
	}
	og, err := s.takeStore()
	if err != nil {
		return nil, err
	}
	// A handle names a publisher for display; the record is keyed by the
	// publisher's key. A caller who published under this handle reads their own
	// record, with its rows. Anyone else reads the record of the key that most
	// recently published under it (a 24-bit handle can be shared; the counts of
	// two publishers are never pooled).
	objs, err := og.ListObjectsByMetadata(ctx, string(ogKindTake), "$.publisher", publisher, takeScan)
	if err != nil {
		return nil, err
	}
	if len(objs) >= takeScan {
		takeBoundLog("corpusd: takes: a handle's takes reached the scan bound %d", takeScan)
	}
	key, newest := "", ""
	for _, o := range objs {
		var r takeRow
		if json.Unmarshal(o.Body, &r) != nil || r.Publisher != publisher {
			continue
		}
		if len(asPublisherKey) == 64 && r.PublisherKey == asPublisherKey {
			key, newest = asPublisherKey, "\xff"
			break
		}
		if r.PublishedAt > newest {
			key, newest = r.PublisherKey, r.PublishedAt
		}
	}
	return s.takeRecord(ctx, og, publisher, key, key != "" && key == asPublisherKey)
}

// takeRecord computes a publisher key's record. publisher is the display
// handle. Per-listener rows are returned only with withRows (the caller is the
// publisher). An empty key is a publisher with no takes: a new record.
func (s *Service) takeRecord(ctx context.Context, og *ObjectGraphStore, publisher, key string, withRows bool) (*TakeRecord, error) {
	var objs []objectgraph.Object
	if key != "" {
		var err error
		if objs, err = og.ListObjectsByMetadata(ctx, string(ogKindTakeOutcome), "$.publisher_key", key, takeOutcomeScan); err != nil {
			return nil, err
		}
		if len(objs) >= takeOutcomeScan {
			takeBoundLog("corpusd: takes: a publisher's outcome scan reached its bound %d; their record undercounts", takeOutcomeScan)
		}
	}
	rec := &TakeRecord{}
	listeners, takes := map[string]bool{}, map[string]bool{}
	var rows []TakeOutcomeView
	for _, o := range objs {
		var r takeOutcomeRow
		if json.Unmarshal(o.Body, &r) != nil {
			continue
		}
		if withRows {
			rows = append(rows, TakeOutcomeView{TakeID: r.TakeID, ListenerKey: r.ListenerKey, Outcome: r.Outcome, At: r.At, Drill: r.Drill})
		}
		if r.Drill {
			if r.Outcome == TakeHeard {
				rec.Drill.DrillHeard++
			} else {
				rec.Drill.DrillSkipped++
			}
			continue
		}
		rec.Summary.Offered++
		if r.Outcome == TakeHeard {
			rec.Summary.Heard++
		} else {
			rec.Summary.Skipped++
		}
		listeners[r.ListenerKey] = true
		takes[r.TakeID] = true
	}
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].At > rows[j].At })
	rec.Rows = rows
	rec.Summary.Publisher, rec.Summary.Listeners, rec.Summary.Takes = publisher, len(listeners), len(takes)
	rec.Summary.New = rec.Summary.Offered == 0
	if rec.Summary.New {
		rec.Summary.Text = "new: no outcomes yet"
	} else {
		rec.Summary.Text = fmt.Sprintf("heard through %d of %d times, by %d distinct listeners, over %d takes",
			rec.Summary.Heard, rec.Summary.Offered, rec.Summary.Listeners, rec.Summary.Takes)
	}
	return rec, nil
}
