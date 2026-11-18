package agentcore

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/yusefmosiah/go-choir/internal/notebook"
	"io"
	"log"
	"math"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"sync/atomic"
	"time"
	_ "time/tzdata" // the listener's tz must resolve on a guest with no zoneinfo
	"unicode"

	"github.com/yusefmosiah/go-choir/internal/appdesk"
	"github.com/yusefmosiah/go-choir/internal/calibration"
	"github.com/yusefmosiah/go-choir/internal/modelpolicy"
	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// POST /api/autoradio/programme: the Autoradio desk's programme call (station
// L1 of docs/definitions/choir-autoradio-learning-engine-2026-11-14.md; design
// docs/design/autoradio-learning-engine-2026-11-14.md sections 3, 9 and 12;
// docs/design/autoradio-dj-programme-2026-11-11.md slice 2). One streamed
// model call writes one stretch: one JSON object per line, each validated as
// it completes. For each valid line, in this order, the handler appends the
// segment record, appends the forecast claim (a content segment only), and
// sends the `segment` event, so no event can precede its forecast.
//
// This handler never writes an outcome: it has no air event, no settlement and
// no lesson, and does not call the reserved settle method (design 4.1; a scan
// test holds that). A crash never resumes work: a stretch the process did not
// finish has its start, its segment records and its claims and no index, and a
// retry is a new request_id.

const (
	programmeMaxBody     = 256 << 10
	programmeMaxCands    = 40
	programmeTextCands   = 10
	programmeMaxCandText = 8 << 10
	programmeMaxEdition  = 32 << 10

	programmeFallbackLine = "Sorry, I lost the thread of the programme."
	programmeCachePrefix  = "autoradio-programme:"
)

var (
	programmeStretchesTotal atomic.Int64
	programmeCallsTotal     atomic.Int64
	programmeSegmentsTotal  atomic.Int64
	programmeDroppedTotal   atomic.Int64
	// programmeDroppedTakeCandidates counts take candidates dropped at validation.
	programmeDroppedTakeCandidates atomic.Int64
)

// Deadlines and a bound on output. These are not limits on our model calls
// (CLAUDE.md: no rate bucket, in-flight cap, spacing or call quota); they end
// a call that has stalled or runs away, and are bounds on what the model can
// cause. The values are invented: neither the design nor the turn route names
// one (the gateway client's own deadline is 10m30s). Variables so a test can
// shorten them.
var (
	programmeWallDeadline = 120 * time.Second // the whole walk of the pool
	programmeIdleDeadline = 30 * time.Second  // between two deltas
)

// The size of one write. A request with cause open is answered by one
// opening; every other request by one to six entries. This is the timing fact
// of the route, not a quota on calls: a provider that does not stream makes the
// first sound wait for the whole answer, so the answer is kept small. The next
// write is a new request, and the extend trigger on the phone is timing.
const (
	programmeMaxSegments = 6
	programmeOpenSegment = 1
)

func (c *programmeCall) maxSegments() int {
	if c.req.Cause == "open" {
		return programmeOpenSegment
	}
	return programmeMaxSegments
}

// programmePrompt is the programme prompt; a variable so a test can empty it.
var programmePrompt = appdesk.ProgrammePrompt

// programmeClipsPrompt is the section added to the prompt when, and only when,
// the request lists clips.
var programmeClipsPrompt = appdesk.ProgrammeClipsPrompt

// programmeTakesPrompt is the section added when, and only when, the call's
// input lists a take (after the publisher's own takes and the muted ones are
// removed).
var programmeTakesPrompt = appdesk.ProgrammeTakesPrompt

func (r programmeRequest) hasClips() bool {
	for _, c := range r.Candidates {
		if len(c.Clips) > 0 {
			return true
		}
	}
	return false
}

var programmeCauses = map[string]bool{"open": true, "extend": true, "turn": true, "result": true}
var programmeVersionRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)
var programmeIntRe = regexp.MustCompile(`^[0-9]{1,12}$`)

type programmeCandidate struct {
	Route   string          `json:"route"`
	Version json.RawMessage `json:"version"`
	Title   string          `json:"title"`
	Dek     string          `json:"dek"`
	Text    string          `json:"text,omitempty"`
	// Clips are the podcast clips that source this story, copied by the phone
	// from GET /api/platform/podcast-clips (docs/design/podcast-clips-2026-11-15.md 4.2).
	Clips []programmeClip `json:"clips,omitempty"`

	// A take candidate (kind "take", design 11.5 of
	// docs/design/autoradio-learned-document-2026-11-17.md) is a listener's
	// recorded voice, published and listed by GET /api/platform/takes. It
	// carries no route, version or dek; the audio address stays on the phone.
	Kind            string             `json:"kind,omitempty"`
	TakeID          string             `json:"take_id,omitempty"`
	Publisher       string             `json:"publisher,omitempty"`
	Transcript      string             `json:"transcript,omitempty"`
	DurationSeconds float64            `json:"duration_seconds,omitempty"`
	RespondsTo      *programmeResponds `json:"responds_to,omitempty"`
	Record          string             `json:"record,omitempty"`

	ref string
}

// programmeResponds is the story on air when a take was recorded.
type programmeResponds struct {
	Route string `json:"route"`
	Title string `json:"title"`
}

func (c programmeCandidate) isTake() bool { return c.Kind == "take" }

// programmeClip is what the call knows of a clip: enough to introduce it,
// never its audio address (the phone holds that).
type programmeClip struct {
	ClipID       string  `json:"clip_id"`
	Show         string  `json:"show"`
	EpisodeTitle string  `json:"episode_title"`
	Published    string  `json:"published,omitempty"`
	Seconds      float64 `json:"seconds"`
	Quote        string  `json:"quote"`
}

const (
	programmeMaxTakes        = 3
	programmeTakeLeadMax     = 400
	programmeTakeTextMax     = 2000
	programmeMaxClipsPerCand = 3
	programmeClipMinSeconds  = 5
	programmeClipMaxSeconds  = 300
	programmeClipQuoteMax    = 400
	programmeClipLineMax     = 200
)

var programmeClipIDRe = regexp.MustCompile(`^clip-[0-9a-f]{12,64}$`)

// programmeMuteRe reads a mute from a notebook line (design 11.7).
var programmeMuteRe = regexp.MustCompile(`(?i)never play takes by (voice-[0-9a-f]{6})`)

type programmeRequest struct {
	RequestID   string               `json:"request_id"`
	Cause       string               `json:"cause"`
	LocalTime   string               `json:"local_time,omitempty"`
	TZ          string               `json:"tz,omitempty"`
	Candidates  []programmeCandidate `json:"candidates,omitempty"`
	EditionText string               `json:"edition_text,omitempty"`
	// AheadTitles are the titles of the entries already ahead of the one on
	// air, in play order. They let the desk avoid repeating them and write a
	// transition into them. At most 20, each at most 300 bytes.
	AheadTitles []string `json:"ahead_titles,omitempty"`
}

const (
	programmeMaxAhead      = 20
	programmeMaxAheadTitle = 300
)

// parseProgrammeRequest validates the body. These are bounds on what a
// stranger can cause (design 12.7), not timers; every refusal is 400, because
// the reader's cap is above anything a valid request can be.
func parseProgrammeRequest(body []byte) (programmeRequest, error) {
	var req programmeRequest
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		return req, fmt.Errorf("decode the request: %v", err)
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return req, errors.New("decode the request: trailing data")
	}
	if !autoradioTurnID.MatchString(req.RequestID) {
		return req, errors.New("request_id must be 1 to 64 characters of letters, digits and _ . -")
	}
	if !programmeCauses[req.Cause] {
		return req, errors.New("cause must be open, extend, turn or result")
	}
	if req.LocalTime != "" {
		if _, err := time.Parse(time.RFC3339, req.LocalTime); err != nil {
			return req, errors.New("local_time must be RFC 3339")
		}
	}
	if req.TZ != "" {
		if len(req.TZ) > 64 {
			return req, errors.New("tz is too long")
		}
		// A name this guest cannot resolve is not a refusal: tz only decides how
		// the local time is shown, and local_time carries its own offset.
	}
	if len(req.EditionText) > programmeMaxEdition {
		return req, fmt.Errorf("edition_text exceeds %d bytes", programmeMaxEdition)
	}
	if len(req.AheadTitles) > programmeMaxAhead {
		return req, fmt.Errorf("ahead_titles holds at most %d items", programmeMaxAhead)
	}
	for i, t := range req.AheadTitles {
		if len(t) > programmeMaxAheadTitle {
			return req, fmt.Errorf("ahead_titles[%d] exceeds %d bytes", i, programmeMaxAheadTitle)
		}
	}
	if len(req.Candidates) > programmeMaxCands {
		return req, fmt.Errorf("candidates holds at most %d items", programmeMaxCands)
	}
	seen := map[string]bool{}
	clipSeen := map[string]bool{}
	takes := 0
	kept := req.Candidates[:0]
	droppedTakes := 0
	var lastDropErr error
	for i := range req.Candidates {
		c := &req.Candidates[i]
		if err := c.check(i); err != nil {
			if c.Kind == "take" {
				// A take is a stranger's published words, copied by the phone from
				// the list. One that does not validate is dropped and counted (one
				// log line per request); it never fails another listener's
				// programme request. No client on main sends takes yet; the phone
				// lane (origin/lane/autoradio-integ16, RadioProgramme.swift) is what
				// was read, and it does not depend on a 400 for a malformed take.
				droppedTakes++
				lastDropErr = err
				programmeDroppedTakeCandidates.Add(1)
				continue
			}
			return req, err
		}
		if c.isTake() {
			if takes++; takes > programmeMaxTakes {
				return req, fmt.Errorf("candidates carries more than %d takes", programmeMaxTakes)
			}
		}
		if seen[c.ref] {
			return req, fmt.Errorf("candidates[%d] repeats the ref %s", i, c.ref)
		}
		seen[c.ref] = true
		kept = append(kept, *c)
		c = &kept[len(kept)-1]
		for j := range c.Clips {
			if err := c.Clips[j].check(i, j); err != nil {
				return req, err
			}
			if clipSeen[c.Clips[j].ClipID] {
				return req, fmt.Errorf("candidates[%d].clips[%d] repeats the clip_id %s", i, j, c.Clips[j].ClipID)
			}
			clipSeen[c.Clips[j].ClipID] = true
		}
	}
	if droppedTakes > 0 {
		log.Printf("autoradio programme: request %s: %d take candidate(s) dropped, the last: %v", req.RequestID, droppedTakes, lastDropErr)
	}
	// Clip: the filtered slice shares the decoded array; nothing may grow into it.
	req.Candidates = slices.Clip(kept)
	return req, nil
}

func (k *programmeClip) check(i, j int) error {
	at := fmt.Sprintf("candidates[%d].clips[%d]", i, j)
	if !programmeClipIDRe.MatchString(k.ClipID) {
		return fmt.Errorf("%s.clip_id is not a clip id", at)
	}
	for name, v := range map[string]*string{"show": &k.Show, "episode_title": &k.EpisodeTitle} {
		*v = strings.TrimSpace(*v)
		if *v == "" || len(*v) > programmeClipLineMax || hasControl(*v) {
			return fmt.Errorf("%s.%s must be one line of 1 to %d bytes", at, name, programmeClipLineMax)
		}
	}
	if k.Published != "" {
		if _, err := time.Parse(time.RFC3339, k.Published); err != nil {
			return fmt.Errorf("%s.published must be RFC 3339", at)
		}
	}
	if !(k.Seconds >= programmeClipMinSeconds && k.Seconds <= programmeClipMaxSeconds) {
		return fmt.Errorf("%s.seconds must be %d to %d", at, programmeClipMinSeconds, programmeClipMaxSeconds)
	}
	if k.Quote = strings.TrimSpace(k.Quote); k.Quote == "" || len(k.Quote) > programmeClipQuoteMax {
		return fmt.Errorf("%s.quote must be 1 to %d bytes", at, programmeClipQuoteMax)
	}
	return nil
}

// checkTake validates a take candidate: what the phone copies from the takes
// list. A take has no route, version, dek, text or clips.
func (c *programmeCandidate) checkTake(i int) error {
	at := fmt.Sprintf("candidates[%d]", i)
	if c.Route != "" || len(c.Version) > 0 || c.Dek != "" || c.Text != "" || len(c.Clips) > 0 {
		return fmt.Errorf("%s is a take, which has no route, version, dek, text or clips", at)
	}
	if !types.TakeIDRe.MatchString(c.TakeID) {
		return fmt.Errorf("%s.take_id is not a take id", at)
	}
	if !types.TakePublisherRe.MatchString(c.Publisher) {
		return fmt.Errorf("%s.publisher is not a voice- handle", at)
	}
	c.Title = strings.TrimSpace(c.Title)
	if c.Title == "" || len(c.Title) > 300 || hasControl(c.Title) {
		return fmt.Errorf("%s.title must be one line of 1 to 300 bytes", at)
	}
	if c.Transcript = strings.TrimSpace(c.Transcript); c.Transcript == "" || len(c.Transcript) > programmeTakeTextMax {
		return fmt.Errorf("%s.transcript must be 1 to %d bytes", at, programmeTakeTextMax)
	}
	// A take is stored with its header's length, so the bounds are the
	// header's (one pair shared with publish), not the claim's 3 to 60.
	if !(c.DurationSeconds >= types.TakeHeaderMinSeconds && c.DurationSeconds <= types.TakeHeaderMaxSeconds) {
		return fmt.Errorf("%s.duration_seconds must be %.1f to %.1f", at, types.TakeHeaderMinSeconds, types.TakeHeaderMaxSeconds)
	}
	if r := c.RespondsTo; r != nil {
		r.Route, r.Title = strings.TrimSpace(r.Route), strings.TrimSpace(r.Title)
		if r.Route == "" || len(r.Route) > 300 || len(r.Title) > 300 || hasControl(r.Route) || hasControl(r.Title) {
			return fmt.Errorf("%s.responds_to must be {route, title} of one line each", at)
		}
	}
	if c.Record = strings.TrimSpace(c.Record); len(c.Record) > 200 || hasControl(c.Record) {
		return fmt.Errorf("%s.record must be one line of at most 200 bytes", at)
	}
	c.ref = "take:" + c.TakeID
	return nil
}

func (c *programmeCandidate) check(i int) error {
	if c.Kind != "" {
		if c.Kind != "take" {
			return fmt.Errorf("candidates[%d].kind must be absent or take", i)
		}
		return c.checkTake(i)
	}
	if c.TakeID != "" || c.Publisher != "" || c.Transcript != "" || c.DurationSeconds != 0 || c.RespondsTo != nil || c.Record != "" {
		return fmt.Errorf("candidates[%d] carries take fields without kind take", i)
	}
	if c.Route == "" || len(c.Route) > 256 || strings.ContainsAny(c.Route, "@#") ||
		strings.IndexFunc(c.Route, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) >= 0 {
		return fmt.Errorf("candidates[%d].route must be 1 to 256 bytes with no space, control character, @ or #", i)
	}
	version := strings.TrimSpace(string(c.Version))
	if strings.HasPrefix(version, `"`) {
		var s string
		if err := json.Unmarshal(c.Version, &s); err != nil || !programmeVersionRe.MatchString(s) {
			return fmt.Errorf("candidates[%d].version is not a version", i)
		}
		version = s
	} else if !programmeIntRe.MatchString(version) {
		return fmt.Errorf("candidates[%d].version must be a whole number or a short string", i)
	}
	c.Title = strings.TrimSpace(c.Title)
	if c.Title == "" || len(c.Title) > 300 || hasControl(c.Title) {
		return fmt.Errorf("candidates[%d].title must be one line of 1 to 300 bytes", i)
	}
	if c.Dek = strings.TrimSpace(c.Dek); c.Dek == "" || len(c.Dek) > 1000 {
		return fmt.Errorf("candidates[%d].dek must be 1 to 1000 bytes (design 12.7: each candidate has title, dek, route and version)", i)
	}
	if len(c.Clips) > programmeMaxClipsPerCand {
		return fmt.Errorf("candidates[%d] carries %d clips, at most %d", i, len(c.Clips), programmeMaxClipsPerCand)
	}
	if c.Text != "" {
		if i >= programmeTextCands {
			return fmt.Errorf("candidates[%d] carries text, which only the first %d items may", i, programmeTextCands)
		}
		if len(c.Text) > programmeMaxCandText {
			return fmt.Errorf("candidates[%d].text exceeds %d bytes", i, programmeMaxCandText)
		}
	}
	c.ref = "pub:" + c.Route + "@v" + version
	return nil
}

// programmeCall is one call in flight.
type programmeCall struct {
	rt      *Runtime
	ledger  autoradioLedger
	w       http.ResponseWriter
	ownerID string
	req     programmeRequest
	input   *programmeInput

	stretchID string
	startedAt time.Time
	received  time.Time
	firstSeg  time.Time
	// firstDelta is when the first model text delta reached this route, and
	// firstSent when the first segment event was written to the client: with
	// the total they show which hop holds the lines.
	firstDelta time.Time
	firstSent  time.Time

	// The member answering now, and the one that wrote the first segment.
	provider, model string

	segmentIDs []string
	claimIDs   []string
	dropped    []droppedLine
	lineNo     int
	streak     int
	segCount   int
	usage      provideriface.TokenUsage
	hops       []autoradioHop
	timingsAt  map[string]time.Time

	// How the call ended.
	fatal    error // a store write failed: no index, no walk
	runEnded bool  // three invalid lines in a row
	stop     bool
	// cancelUpstream cancels the model call in flight; set when the call must
	// stop reading.
	cancelUpstream context.CancelFunc
	capped         bool // the segment cap ended the call
	// trailing is the last committed segment when it is a transition that leads
	// into "the entry written next" (before_segment is s<its number + 1>): if
	// the response ends there, that entry never comes, and the index says so.
	trailing *programmeSegment
	surplus  int         // lines after the opening of an open request: dropped, and the call ends
	timedOut atomic.Bool // a deadline ended the call
	writeErr error       // the first write error: the client went away
	prompt   string

	// The heard index (api_autoradio_programme_heard.go): the segment list and
	// what its reads found full, and the follow-ups the DJ wrote.
	segRecs   map[string]types.CommitmentRecord
	heardFull []string // the heard reads that reached their limit, for the log
	// heardUnparsable counts the heard settlements whose time did not parse.
	heardUnparsable int
	followups       int

	// The notebook (L5): Texture's service, the notebook lines the model wrote
	// (raw, read after the stream), the title of the entry on air, and the
	// summary the done event and the index carry.
	nb         notebook.Service
	nbLines    []string
	onAirTitle func(ctx context.Context) string
	nbSummary  string

	// sheet is the listener's server run sheet, the door the handler binds.
	sheet               programmeSheet
	placed, placeFailed int
	madeRoom            bool
}

func (c *programmeCall) memberName() string {
	if c.provider == "" {
		return autoradioDesk
	}
	return c.provider + "/" + c.model
}

func (c *programmeCall) timings(done time.Time) map[string]int64 {
	ms := func(t time.Time) int64 {
		if t.IsZero() {
			return -1
		}
		return t.Sub(c.received).Milliseconds()
	}
	out := map[string]int64{"first_segment": ms(c.firstSeg), "done": ms(done)}
	if !c.firstDelta.IsZero() {
		out["first_delta"] = ms(c.firstDelta)
	}
	if !c.firstSent.IsZero() {
		out["first_segment_sent"] = ms(c.firstSent)
	}
	if c.surplus > 0 {
		out["surplus_lines"] = int64(c.surplus)
	}
	return out
}

func (c *programmeCall) begin() {
	h := c.w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache")
	h.Set("X-Accel-Buffering", "no")
	// The server-wide WriteTimeout would cut a stretch's stream; lift it for
	// this response only, as the turn route does.
	_ = http.NewResponseController(c.w).SetWriteDeadline(time.Time{})
	c.w.WriteHeader(http.StatusOK)
	if _, err := io.WriteString(c.w, ": autoradio\n\n"); err != nil && c.writeErr == nil {
		c.writeErr = err
	}
	c.flush()
}

func (c *programmeCall) flush() {
	if f, ok := c.w.(http.Flusher); ok {
		f.Flush()
	}
}

func (c *programmeCall) event(name string, v any) {
	data, err := json.Marshal(v)
	if err != nil {
		log.Printf("autoradio programme: marshal %s event: %v", name, err)
		return
	}
	// One write per event, so a reader of the stream sees whole events.
	if _, err := io.WriteString(c.w, "event: "+name+"\ndata: "+string(data)+"\n\n"); err != nil && c.writeErr == nil {
		c.writeErr = err
	}
	c.flush()
}

func estSeconds(text string) int {
	return int(math.Ceil(float64(len(strings.Fields(text))) / 2.5))
}

func (c *programmeCall) segmentEvent(s programmeSegment) map[string]any {
	cites := s.Cites
	if cites == nil {
		cites = []programmeCite{}
	}
	ev := map[string]any{
		"stretch_id": c.stretchID, "segment_id": s.ID, "kind": s.Kind, "title": s.Title, "text": s.Text,
		"cites": cites, "block_count": s.Blocks, "est_seconds": estSeconds(s.Text), "cut_at": "sentence",
		// The entry this segment is on the listener's server run sheet (the
		// phone's own id for it, d-<segment_id>), and, for a content segment,
		// the forecast claim the sheet's entry carries.
		"entry_id": programmeEntryID(s.ID),
	}
	if s.ClaimID != "" {
		ev["claim_id"] = s.ClaimID
	}
	if s.BetweenAfter != "" {
		ev["between"] = map[string]any{"after_segment": s.BetweenAfter, "before_segment": s.BetweenBefore}
	}
	if s.ClipID != "" {
		// Real audio is cut only between segments (design 1.3), and the phone
		// finds the audio address of the clip in the set it fetched.
		ev["cut_at"] = "segment"
		storyRef := ""
		if len(cites) > 0 {
			storyRef = cites[0].Ref
		}
		ev["clip"] = map[string]any{"clip_id": s.ClipID, "story_ref": storyRef, "seconds": s.ClipSeconds}
	}
	if s.TakeID != "" {
		// The publisher's audio is played whole, between segments, never cut.
		ev["cut_at"] = "segment"
		storyRef := ""
		for _, c := range cites {
			if c.Role == "mentioned" {
				storyRef = c.Ref
			}
		}
		ev["take"] = map[string]any{"take_id": s.TakeID, "story_ref": storyRef, "seconds": s.TakeSeconds}
		// The forecast rides flat on the segment, as the phone reads it.
		ev["p_heard"], ev["caused_by"] = s.P, s.CausedBy
	}
	return ev
}

func mintStretchID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return "s" + hex.EncodeToString(b)
}

func (h *APIHandler) HandleAutoradioProgramme(w http.ResponseWriter, r *http.Request) {
	received := time.Now()
	if r.Method != http.MethodPost {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}
	rt := h.rt
	streamer, ok := provideriface.TextStreamProvider(nil), false
	if rt != nil {
		streamer, ok = rt.provider.(provideriface.TextStreamProvider)
	}
	if !ok {
		writeAPIJSON(w, http.StatusServiceUnavailable, apiError{Error: "no streaming model is bound on this computer"})
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, programmeMaxBody))
	if err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			writeAPIJSON(w, http.StatusRequestEntityTooLarge, apiError{Error: fmt.Sprintf("body exceeds %d bytes", programmeMaxBody)})
			return
		}
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "cannot read the body"})
		return
	}
	req, err := parseProgrammeRequest(body)
	if err != nil {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: err.Error()})
		return
	}
	// autoradioInflight is process-local: a computer serves its API from one
	// process, so it holds; two overlapping processes could each answer one
	// request_id (a duplicate model call, not a duplicate claim).
	key := "programme/" + ownerID + "/" + rt.cfg.ComputerID + "/" + req.RequestID
	if _, busy := autoradioInflight.LoadOrStore(key, struct{}{}); busy {
		writeAPIJSON(w, http.StatusConflict, apiError{Error: "this request_id is already being answered"})
		return
	}
	defer autoradioInflight.Delete(key)

	// A disconnect does not stop the call: the stretch commits and a replay
	// of the request_id returns it (programme design 1.2, slice 2).
	ctx := context.WithoutCancel(r.Context())
	c := &programmeCall{rt: rt, ledger: h.programmeLedger(), w: w, ownerID: ownerID, req: req, received: received}
	c.sheet = programmeSheetOf{h: h, ownerID: ownerID}
	c.nb = h.notebook
	c.onAirTitle = func(ctx context.Context) string { return h.sheetOnAirTitle(ctx, ownerID) }

	prior, err := c.get(ctx, requestRecordPrefix+req.RequestID)
	if err != nil {
		log.Printf("autoradio programme: request %s: reading the request record: %v", req.RequestID, err)
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "cannot read the request record"})
		return
	}
	if prior != nil {
		if prior.SubjectRefs["cause"] != req.Cause {
			writeAPIJSON(w, http.StatusConflict, apiError{Error: "this request_id was used for another cause; use a new request_id"})
			return
		}
		c.stretchID = prior.SubjectRefs["stretch_id"]
		c.begin()
		c.replay(ctx)
		return
	}

	prompt, ok := programmePrompt()
	if !ok || strings.TrimSpace(prompt) == "" {
		log.Printf("autoradio programme: request %s: the programme prompt is not shipped", req.RequestID)
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "the programme prompt is not available"})
		return
	}
	if req.hasClips() {
		clips, ok := programmeClipsPrompt()
		if !ok || strings.TrimSpace(clips) == "" {
			log.Printf("autoradio programme: request %s: the podcast clips prompt is not shipped", req.RequestID)
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "the programme prompt is not available"})
			return
		}
		prompt = strings.TrimRight(prompt, "\n") + "\n\n" + strings.TrimSpace(clips) + "\n"
	}
	c.prompt = prompt
	if c.input, err = c.buildInput(ctx); err != nil {
		log.Printf("autoradio programme: request %s: reading the listener's record: %v", req.RequestID, err)
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "cannot read the listener's record"})
		return
	}
	if c.input.hasTakes() {
		takes, ok := programmeTakesPrompt()
		if !ok || strings.TrimSpace(takes) == "" {
			log.Printf("autoradio programme: request %s: the takes prompt is not shipped", req.RequestID)
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "the programme prompt is not available"})
			return
		}
		c.prompt = strings.TrimRight(c.prompt, "\n") + "\n\n" + strings.TrimSpace(takes) + "\n"
	}
	c.stretchID = mintStretchID()
	if err := c.writeStart(ctx); err != nil {
		log.Printf("autoradio programme: request %s: start record not written: %v", req.RequestID, err)
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "cannot record the stretch"})
		return
	}
	c.begin()
	c.run(ctx, streamer)
}

// replay answers a request_id already written: the stored segments, then how
// the stretch ended. No model call, no write.
func (c *programmeCall) replay(ctx context.Context) {
	c.event("stretch", map[string]any{"stretch_id": c.stretchID, "written_for": c.req.Cause, "request_id": c.req.RequestID})
	idx, err := c.get(ctx, stretchIndexRecordPrefix+c.stretchID)
	if err != nil {
		c.event("error", map[string]any{"stretch_id": c.stretchID, "spoken": programmeFallbackLine, "reason": "cannot read the stretch: " + err.Error()})
		return
	}
	n := -1
	status := "interrupted"
	if idx != nil {
		status = idx.SubjectRefs["status"]
		_, _ = fmt.Sscanf(idx.SubjectRefs["segments"], "%d", &n)
	}
	segs, err := c.readSegments(ctx, c.stretchID, n)
	if err == nil && idx != nil && status == "completed" && len(segs) < n {
		err = fmt.Errorf("the stretch's records are incomplete: %d of %d segments have what they should", len(segs), n)
	}
	if err != nil {
		c.event("error", map[string]any{"stretch_id": c.stretchID, "spoken": programmeFallbackLine, "reason": err.Error()})
		return
	}
	for i := range segs {
		// A replay repairs a placement that failed the first time; a placement
		// that landed answers by its op id and changes nothing.
		c.placeOnSheet(ctx, &segs[i])
		c.event("segment", c.segmentEvent(segs[i]))
	}
	if idx != nil && status == "completed" {
		c.event("done", map[string]any{
			"stretch_id": c.stretchID, "status": "completed", "replayed": true, "segments": len(segs),
			"claims": len(idx.RelatedIDs), "index_record_id": idx.RecordID, "provider": idx.SubjectRefs["provider"], "model": idx.SubjectRefs["model"],
		})
		return
	}
	reason := "the stretch ended without an index; use a new request_id"
	if idx != nil {
		reason = "the stretch ended " + status + "; use a new request_id"
	}
	c.event("error", map[string]any{"stretch_id": c.stretchID, "spoken": programmeFallbackLine, "reason": reason, "replayed": true})
}

func (c *programmeCall) run(ctx context.Context, streamer provideriface.TextStreamProvider) {
	programmeStretchesTotal.Add(1)
	c.event("stretch", map[string]any{"stretch_id": c.stretchID, "written_for": c.req.Cause, "request_id": c.req.RequestID})
	defer func() {
		// A panic is a crash's stand-in: nothing is settled, no index is
		// written, and a retry is a new request.
		if p := recover(); p != nil {
			log.Printf("autoradio programme: stretch %s: panic: %v", c.stretchID, p)
			func() {
				defer func() { _ = recover() }()
				c.event("error", map[string]any{"stretch_id": c.stretchID, "spoken": programmeFallbackLine, "reason": fmt.Sprintf("internal error: %v", p)})
			}()
		}
	}()

	prompt := c.prompt
	promptSum := sha256Hex(prompt)
	stable, volatile := c.frame(c.input, c.localTime())
	chain := c.rt.modelPolicy.PoolChain(ctx, c.ownerID, modelpolicy.AutoradioProgrammeRole)

	reason := "no model answered"
	status := "failed"
	walkCtx, cancelWalk := context.WithTimeout(ctx, programmeWallDeadline)
	defer cancelWalk()
	for _, sel := range chain {
		if walkCtx.Err() != nil {
			c.finish(ctx, "deadline", "the wall-clock deadline of the walk passed", true)
			return
		}
		member := sel.Provider + "/" + sel.Model
		hopStart := time.Now()
		c.provider, c.model = sel.Provider, sel.Model
		c.streak = 0
		var reader lineReader
		effort := strings.ToLower(strings.TrimSpace(sel.ReasoningEffort))
		callCtx, cancel := context.WithCancel(walkCtx)
		c.cancelUpstream = cancel
		// The idle deadline runs between deltas, from the first one (the time to
		// the first delta is under the wall-clock deadline of the walk).
		var idle *time.Timer
		programmeCallsTotal.Add(1)
		resp, err := streamer.StreamText(callCtx, provideriface.TextStreamRequest{
			Provider: sel.Provider, Model: sel.Model, ReasoningEffort: effort,
			ConversationID: c.stretchID, CacheKey: programmeCachePrefix + c.rt.cfg.ComputerID + ":" + promptSum[:8],
			System: prompt, StableUser: stable, User: volatile, MaxTokens: sel.MaxTokens,
		}, func(delta string) {
			if c.firstDelta.IsZero() {
				c.firstDelta = time.Now()
			}
			if idle == nil {
				idle = time.AfterFunc(programmeIdleDeadline, func() { c.timedOut.Store(true); cancel() })
			} else {
				idle.Reset(programmeIdleDeadline)
			}
			if !c.stop {
				reader.push(delta, c.handleLine)
			}
		})
		if idle != nil {
			idle.Stop()
		}
		timedOut := c.timedOut.Load() || errors.Is(walkCtx.Err(), context.DeadlineExceeded)
		cancel()
		hop := autoradioHop{Member: member, MS: time.Since(hopStart).Milliseconds()}
		if err == nil && !c.stop {
			reader.flush(c.handleLine)
		}
		switch {
		case c.fatal != nil:
			hop.Outcome = "store_failed"
			c.hops = append(c.hops, hop)
			c.finish(ctx, "store_failed", "a record could not be written: "+c.fatal.Error(), false)
			return
		case c.runEnded:
			hop.Outcome = "invalid_run"
			c.hops = append(c.hops, hop)
			c.finish(ctx, "invalid_run", fmt.Sprintf("%d invalid lines in a row ended the stretch", programmeInvalidRun), true)
			return
		case c.surplus > 0:
			hop.Outcome = "opening_only"
			c.hops = append(c.hops, hop)
			c.finish(ctx, "completed", "", true)
			return
		case c.capped:
			hop.Outcome = "capped"
			c.hops = append(c.hops, hop)
			c.finish(ctx, "completed", "", true)
			return
		case timedOut:
			hop.Outcome = "deadline"
			c.hops = append(c.hops, hop)
			c.finish(ctx, "deadline", "a deadline passed (wall clock of the walk, or idle between deltas)", true)
			return
		case err != nil && c.segCount > 0:
			hop.Outcome = "failed_mid_stream"
			c.hops = append(c.hops, hop)
			c.finish(ctx, "failed", "the model failed mid-stream: "+err.Error(), true)
			return
		case err != nil:
			hop.Outcome = "refused"
			c.hops = append(c.hops, hop)
			log.Printf("autoradio programme: stretch %s: %s refused before a segment was sent: %v", c.stretchID, member, err)
			reason = member + " refused"
			continue
		case c.segCount == 0:
			hop.Outcome = "empty"
			c.hops = append(c.hops, hop)
			reason = member + " wrote no valid segment"
			continue
		}
		hop.Outcome = "answered"
		c.hops = append(c.hops, hop)
		c.usage = resp.Usage
		c.finish(ctx, "completed", "", true)
		return
	}
	c.finish(ctx, status, reason, true)
}

// handleLine validates one line and, when it is valid, writes and streams it.
// It returns false when the call must stop reading.
func (c *programmeCall) handleLine(line string, tooLong bool) bool {
	if c.stop {
		return false
	}
	if !tooLong && isSkippable(line) {
		return true
	}
	c.lineNo++
	var seg *programmeSegment
	class, reason := lineInvalid, ""
	c.input.nextSeg = c.segCount
	if tooLong {
		reason = fmt.Sprintf("line is over %d bytes", programmeMaxLine)
	} else {
		seg, class, reason = c.input.checkLine(line)
	}
	openDone := c.req.Cause == "open" && c.segCount >= programmeOpenSegment
	switch class {
	case lineLesson:
		return true // the old lesson line: neither counted nor reset
	case lineNotebook:
		// A notebook line is read after the stream (settleNotebook); an open
		// request writes none, and it is neither counted nor reset.
		if c.req.Cause != "open" {
			if len(c.nbLines) < nbMaxProposals {
				c.nbLines = append(c.nbLines, line)
			} else {
				c.dropped = append(c.dropped, droppedLine{N: c.lineNo, Reason: "more than the notebook lines a response may carry"})
			}
		}
		return true
	case lineInvalid:
		if openDone {
			// After the opening of an open request an invalid line is noted and
			// nothing more: it is not a content line and ends nothing.
			c.dropped = append(c.dropped, droppedLine{N: c.lineNo, Reason: reason})
			programmeDroppedTotal.Add(1)
			return true
		}
		return c.drop(reason)
	}
	if openDone {
		// An open request is answered by the opening alone. A valid line after
		// it is dropped, noted once, and the call ends there: the rest of the
		// answer is not read.
		c.surplus++
		c.dropped = append(c.dropped, droppedLine{N: c.lineNo, Reason: "an open request takes the opening only: the line is dropped and the call ends"})
		programmeDroppedTotal.Add(1)
		c.stop = true
		if c.cancelUpstream != nil {
			c.cancelUpstream()
		}
		return false
	}
	if c.req.Cause == "open" && seg.Kind != "opening" {
		return c.drop(fmt.Sprintf("an open request is answered by an opening, not a %s", seg.Kind))
	}
	if c.req.Cause != "open" && seg.Kind == "opening" {
		// An opening exists only when the radio is opened, so a second greeting
		// mid-programme is not sent and not written. It is not a malformed
		// line: it is noted and counted in dropped, but it is not one of the
		// run of invalid lines that ends a call.
		c.dropped = append(c.dropped, droppedLine{N: c.lineNo, Reason: fmt.Sprintf("an opening is written only when the radio is opened, not on a request with cause %q", c.req.Cause)})
		programmeDroppedTotal.Add(1)
		return true
	}
	if c.segCount >= c.maxSegments() {
		c.dropped = append(c.dropped, droppedLine{N: c.lineNo, Reason: fmt.Sprintf("segment cap %d reached: the stretch ended", c.maxSegments())})
		c.capped, c.stop = true, true
		if c.cancelUpstream != nil {
			c.cancelUpstream()
		}
		return false
	}
	seg.lineNo = c.lineNo
	return c.commit(seg)
}

// commit writes a valid segment and sends its event. A content segment's claim
// is checked before anything is written, so a line the constructor refuses
// leaves no segment record.
func (c *programmeCall) commit(seg *programmeSegment) bool {
	seg.ID = fmt.Sprintf("%s.s%d", c.stretchID, c.segCount)
	seg.BetweenAfter, seg.BetweenBefore = c.qualify(seg.BetweenAfter), c.qualify(seg.BetweenBefore)
	seg.Blocks = len(calibration.SplitBlocks(seg.Text))
	seg.SHA256 = sha256Hex(seg.Text)
	if seg.Content {
		if _, err := calibration.NewAutoradioForecast(c.forecastInput(seg), time.Now()); err != nil {
			return c.drop("the forecast was refused: " + err.Error())
		}
	}
	c.streak = 0
	ctx := context.Background()
	if err := c.writeSegment(ctx, seg); err != nil {
		return c.failStore(err)
	}
	if seg.Content {
		claim, err := calibration.NewAutoradioForecast(c.forecastInput(seg), time.Now())
		if err != nil {
			return c.failStore(err)
		}
		if err := c.writeClaim(ctx, claim); err != nil {
			return c.failStore(err)
		}
		c.claimIDs = append(c.claimIDs, claim.RecordID)
		if seg.Telling == tellingFollowup {
			c.followups++
		}
		seg.ClaimID = claim.RecordID
	}
	// The entry goes on the server's run sheet before the phone hears of it,
	// so an op the phone posts about it always finds it.
	c.placeOnSheet(ctx, seg)
	c.segmentIDs = append(c.segmentIDs, seg.ID)
	c.segCount++
	c.trailing = nil
	if seg.BetweenBefore == fmt.Sprintf("%s.s%d", c.stretchID, c.segCount) {
		c.trailing = seg
	}
	if c.firstSeg.IsZero() {
		c.firstSeg = time.Now()
	}
	programmeSegmentsTotal.Add(1)
	c.event("segment", c.segmentEvent(*seg))
	if c.firstSent.IsZero() {
		c.firstSent = time.Now()
	}
	return true
}

func (c *programmeCall) forecastInput(s *programmeSegment) calibration.AutoradioForecastInput {
	return calibration.AutoradioForecastInput{
		StretchID: c.stretchID, SegmentID: s.ID, Kind: s.Kind, Title: s.Title, Model: c.memberName(), P: s.P,
		Cause: s.Cause, CausedBy: s.CausedBy, LessonID: s.LessonID, TakeID: s.TakeID, Telling: s.Telling, TextSHA256: s.SHA256, Blocks: s.Blocks, StretchStart: c.startedAt,
	}
}

// drop notes an invalid line; the third in a row ends the call.
func (c *programmeCall) drop(reason string) bool {
	c.dropped = append(c.dropped, droppedLine{N: c.lineNo, Reason: reason})
	programmeDroppedTotal.Add(1)
	log.Printf("autoradio programme: stretch %s: line %d dropped: %s", c.stretchID, c.lineNo, reason)
	c.streak++
	if c.streak >= programmeInvalidRun {
		c.runEnded, c.stop = true, true
		if c.cancelUpstream != nil {
			c.cancelUpstream()
		}
		return false
	}
	return true
}

func (c *programmeCall) failStore(err error) bool {
	log.Printf("autoradio programme: stretch %s: write failed: %v", c.stretchID, err)
	c.fatal, c.stop = err, true
	if c.cancelUpstream != nil {
		c.cancelUpstream()
	}
	return false
}

// finish ends the call: the index when the handler itself concluded the call,
// then `done` or `error`.
func (c *programmeCall) finish(ctx context.Context, status, reason string, index bool) {
	if c.trailing != nil {
		// A note only: the event was sent, and the phone drops a transition
		// whose before entry is not the entry that follows it.
		c.dropped = append(c.dropped, droppedLine{N: c.trailing.lineNo, Reason: "the last transition leads into the next entry, which was never written"})
	}
	if status == "completed" && index {
		c.settleNotebook(ctx)
	}
	indexID, indexErr := "", ""
	if index {
		id, err := c.writeIndex(ctx, status)
		indexID = id
		if err != nil {
			indexErr = err.Error()
			log.Printf("autoradio programme: stretch %s: index not written: %v", c.stretchID, err)
		}
	}
	done := time.Now()
	c.logHeard()
	log.Printf("autoradio programme: stretch=%s request=%s status=%s member=%s segments=%d claims=%d dropped=%d tokens_in=%d tokens_out=%d ms{first_segment=%d done=%d} stretches_total=%d model_calls_total=%d",
		c.stretchID, c.req.RequestID, status, c.memberName(), c.segCount, len(c.claimIDs), len(c.dropped), c.usage.InputTokens, c.usage.OutputTokens,
		c.timings(done)["first_segment"], c.timings(done)["done"], programmeStretchesTotal.Load(), programmeCallsTotal.Load())
	if status != "completed" {
		c.event("error", map[string]any{"stretch_id": c.stretchID, "spoken": programmeFallbackLine, "reason": reason, "status": status})
		return
	}
	data := map[string]any{
		"stretch_id": c.stretchID, "status": "completed", "replayed": false, "segments": c.segCount, "claims": len(c.claimIDs),
		"dropped": len(c.dropped), "provider": c.provider, "model": c.model, "index_record_id": indexID,
		"placed": c.placed, "place_failed": c.placeFailed,
		"timings_ms": c.timings(done), "members_tried": c.hops,
		"usage": map[string]any{"input_tokens": c.usage.InputTokens, "output_tokens": c.usage.OutputTokens, "cached_input_tokens": c.usage.CachedInputTokens, "cache_reported": c.usage.CacheReported},
	}
	if indexErr != "" {
		data["index_error"] = indexErr
	}
	if c.nbSummary != "" {
		data["notebook"] = json.RawMessage(c.nbSummary)
	}
	c.event("done", data)
}

// qualify gives an s<n> neighbour of a transition the stretch id, so the event
// and the record carry a segment_id; an ahead:<n> stays as the model wrote it,
// for the phone to resolve against the ahead_titles it sent.
func (c *programmeCall) qualify(ref string) string {
	if strings.HasPrefix(ref, "s") {
		return c.stretchID + "." + ref
	}
	return ref
}
