package platform

// Takes in corpusd (docs/reference/AUTORADIO-TAKES.md). Ways it can fail,
// written before the code:
//   1. a file that is not an mp4, whose bytes do not match its sha256, whose
//      header length is outside 3 to 60 s or far from the claimed length, or
//      that is over 512 KB, is stored;
//   2. an empty or over-long transcript is stored;
//   3. an eleventh take in 24 hours by one publisher is stored (the bound);
//   4. the same audio by the same publisher is two takes;
//   5. a deleted or yanked take is still listed, still served, or counts a
//      new outcome; its bytes survive the delete;
//   6. a publisher deletes another publisher's take;
//   7. the public list carries the publisher key, or a drill take reaches a
//      listener that is not a drill computer;
//   8. an outcome from the publisher's own account is stored (rule 1);
//   9. a listener moves the record by more than one outcome per take (the
//      first stands);
//  10. an outcome from a drill computer (d@e) is counted, or is not marked;
//  11. the record is computed from anything but other listeners' outcomes, or
//      a listener's identity reaches anyone but as an opaque key;
//  12. the publisher cannot read every row about their takes, or a stranger can;
//  13. the audio route serves anything but audio/mp4.

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/types"
)

const takesDrillComputer = "computer-59c902c5d46caecc0a180a8557164e72"

func mp4Box(name string, payload ...[]byte) []byte {
	body := bytes.Join(payload, nil)
	out := make([]byte, 8, 8+len(body))
	binary.BigEndian.PutUint32(out[0:4], uint32(8+len(body)))
	copy(out[4:8], name)
	return append(out, body...)
}

// takeFixtureAudio is a structurally valid m4a of the given length: ftyp,
// moov with an mvhd, mdat. It is not decodable and need not be.
func takeFixtureAudio(secs float64, extra int) []byte {
	mvhd := make([]byte, 100)
	binary.BigEndian.PutUint32(mvhd[12:16], 1000)
	binary.BigEndian.PutUint32(mvhd[16:20], uint32(secs*1000))
	b := mp4Box("ftyp", []byte("M4A \x00\x00\x00\x00M4A mp42isom"))
	b = append(b, mp4Box("moov", mp4Box("mvhd", mvhd))...)
	b = append(b, mp4Box("mdat", bytes.Repeat([]byte{1}, 64+extra))...)
	return b
}

func takeIn(user string, audio []byte, secs float64) TakeIn {
	sum := sha256.Sum256(audio)
	return TakeIn{
		PublisherKey: types.TakePublisherKey(user), Publisher: types.TakePublisherHandle(user),
		DurationSeconds: secs, SHA256: hex.EncodeToString(sum[:]), AudioBase64: base64.StdEncoding.EncodeToString(audio),
		Transcript: "I think the harbor matters more than they say.",
		RespondsTo: &TakeRespondsTo{Route: "/pub/texture/harbor-abc123", Title: "Harbor reopens"},
	}
}

func newTakesService(t *testing.T) *Service {
	t.Helper()
	store, root := openTestPlatformStore(t)
	return NewService(store, filepath.Join(root, "artifacts"), filepath.Join(root, "signing.key"))
}

func mustPublish(t *testing.T, svc *Service, user string, extra int) *Take {
	t.Helper()
	take, err := svc.PublishTake(context.Background(), takeIn(user, takeFixtureAudio(10, extra), 10))
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	return take
}

func outcome(t *testing.T, svc *Service, takeID, listener, computer, verdict string) (bool, error) {
	t.Helper()
	return svc.RecordTakeOutcome(context.Background(), TakeOutcomeIn{
		TakeID: takeID, ListenerUserKey: types.TakePublisherKey(listener), ListenerComputer: computer,
		Outcome: verdict, ClaimID: "claim-" + strings.Repeat("a", 20),
	})
}

func TestPublishTakeRefusesWhatIsNotATake(t *testing.T) {
	svc := newTakesService(t)
	ctx := context.Background()
	good := takeFixtureAudio(10, 0)
	for name, mutate := range map[string]func(*TakeIn){
		"not mp4": func(in *TakeIn) {
			junk := bytes.Repeat([]byte("x"), 100)
			*in = takeIn("u1", junk, 10)
		},
		"sha mismatch":    func(in *TakeIn) { in.SHA256 = strings.Repeat("0", 64) },
		"too short":       func(in *TakeIn) { *in = takeIn("u1", takeFixtureAudio(2, 0), 2) },
		"too long":        func(in *TakeIn) { *in = takeIn("u1", takeFixtureAudio(61, 0), 61) },
		"claim mismatch":  func(in *TakeIn) { in.DurationSeconds = 30 },
		"over 512 KB":     func(in *TakeIn) { *in = takeIn("u1", takeFixtureAudio(10, 512<<10), 10) },
		"empty text":      func(in *TakeIn) { in.Transcript = "  " },
		"long text":       func(in *TakeIn) { in.Transcript = strings.Repeat("a", TakeTranscriptMax+1) },
		"no publisher":    func(in *TakeIn) { in.Publisher = "" },
		"bad responds_to": func(in *TakeIn) { in.RespondsTo = &TakeRespondsTo{Route: "a\nb", Title: "t"} },
		"bad base64":      func(in *TakeIn) { in.AudioBase64 = "!!!" },
	} {
		in := takeIn("u1", good, 10)
		mutate(&in)
		if _, err := svc.PublishTake(ctx, in); !errors.Is(err, ErrTakeInvalid) {
			t.Errorf("%s: err = %v, want ErrTakeInvalid", name, err)
		}
	}
	list, err := svc.ListTakes(ctx, 0, true)
	if err != nil || len(list.Takes) != 0 {
		t.Fatalf("refused takes left %d rows (%v)", len(list.Takes), err)
	}
}

func TestPublishListAudioDeleteOfOneTake(t *testing.T) {
	svc := newTakesService(t)
	ctx := context.Background()
	take := mustPublish(t, svc, "u1", 0)
	if !types.TakeIDRe.MatchString(take.TakeID) || take.Publisher != types.TakePublisherHandle("u1") || take.State != TakeStateLive || take.Bytes == 0 {
		t.Fatalf("take = %+v", take)
	}
	again := mustPublish(t, svc, "u1", 0)
	if again.TakeID != take.TakeID {
		t.Fatalf("the same audio by the same publisher is another take: %s vs %s", again.TakeID, take.TakeID)
	}
	list, err := svc.ListTakes(ctx, 0, false)
	if err != nil || len(list.Takes) != 1 {
		t.Fatalf("list = %+v, %v", list, err)
	}
	got := list.Takes[0]
	if got.AudioURL != "/api/platform/takes/"+take.TakeID+"/audio" || got.RespondsTo == nil || got.RespondsTo.Title != "Harbor reopens" || got.Record == nil || !got.Record.New {
		t.Fatalf("listed take = %+v", got)
	}
	if strings.Contains(string(takesJSON(t, list)), types.TakePublisherKey("u1")) {
		t.Fatal("the list carries the publisher key")
	}
	audio, err := svc.TakeAudio(ctx, take.TakeID)
	if err != nil || !bytes.Equal(audio, takeFixtureAudio(10, 0)) {
		t.Fatalf("audio = %d bytes, %v", len(audio), err)
	}
	// Another publisher cannot delete it.
	if err := svc.DeleteTake(ctx, take.TakeID, types.TakePublisherKey("u2")); !errors.Is(err, ErrTakeNotFound) {
		t.Fatalf("a stranger's delete = %v, want not found", err)
	}
	if err := svc.DeleteTake(ctx, take.TakeID, types.TakePublisherKey("u1")); err != nil {
		t.Fatal(err)
	}
	if a, _ := svc.TakeAudio(ctx, take.TakeID); a != nil {
		t.Fatal("deleted audio is still served")
	}
	list, _ = svc.ListTakes(ctx, 0, true)
	mine, _ := svc.ListMyTakes(ctx, types.TakePublisherKey("u1"))
	if len(list.Takes) != 0 || len(mine.Takes) != 0 {
		t.Fatalf("a deleted take is listed: %d public, %d mine", len(list.Takes), len(mine.Takes))
	}
	// The bytes are gone from the graph, not only hidden.
	og, _ := svc.takeStore()
	canonical, _ := podcastCanonical(ogKindTakeAudio, take.TakeID)
	if _, err := og.GetObject(ctx, canonical); err == nil {
		t.Fatal("the audio object survives the delete")
	}
	// A removed take is not published again.
	if _, err := svc.PublishTake(ctx, takeIn("u1", takeFixtureAudio(10, 0), 10)); !errors.Is(err, ErrTakeRemoved) {
		t.Fatalf("republish of a deleted take = %v", err)
	}
	// Delete is idempotent.
	if err := svc.DeleteTake(ctx, take.TakeID, types.TakePublisherKey("u1")); err != nil {
		t.Fatal(err)
	}
}

func TestTakeDailyLimit(t *testing.T) {
	svc := newTakesService(t)
	for i := 0; i < TakeDailyLimit; i++ {
		mustPublish(t, svc, "u1", i)
	}
	_, err := svc.PublishTake(context.Background(), takeIn("u1", takeFixtureAudio(10, 99), 10))
	if !errors.Is(err, ErrTakeDailyLimit) {
		t.Fatalf("take %d = %v, want the daily limit", TakeDailyLimit+1, err)
	}
	// The bound is per publisher.
	mustPublish(t, svc, "u2", 0)
	// Deleting does not free the quota.
	list, _ := svc.ListMyTakes(context.Background(), types.TakePublisherKey("u1"))
	if err := svc.DeleteTake(context.Background(), list.Takes[0].TakeID, types.TakePublisherKey("u1")); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.PublishTake(context.Background(), takeIn("u1", takeFixtureAudio(10, 99), 10)); !errors.Is(err, ErrTakeDailyLimit) {
		t.Fatalf("after a delete = %v, want the daily limit", err)
	}
}

func TestYankedTakeIsGoneAndCountsNothing(t *testing.T) {
	svc := newTakesService(t)
	ctx := context.Background()
	take := mustPublish(t, svc, "u1", 0)
	if err := svc.YankTake(ctx, take.TakeID); err != nil {
		t.Fatal(err)
	}
	if a, _ := svc.TakeAudio(ctx, take.TakeID); a != nil {
		t.Fatal("yanked audio is served")
	}
	if _, err := outcome(t, svc, take.TakeID, "u2", "computer-"+strings.Repeat("1", 32), TakeHeard); !errors.Is(err, ErrTakeGone) {
		t.Fatalf("outcome on a yanked take = %v", err)
	}
	if err := svc.YankTake(ctx, "take-0000000000000000"); !errors.Is(err, ErrTakeNotFound) {
		t.Fatalf("yank of nothing = %v", err)
	}
}

func TestDrillTakeIsListedOnlyToDrillComputers(t *testing.T) {
	svc := newTakesService(t)
	in := takeIn("u1", takeFixtureAudio(10, 0), 10)
	in.Drill = true
	if _, err := svc.PublishTake(context.Background(), in); err != nil {
		t.Fatal(err)
	}
	if l, _ := svc.ListTakes(context.Background(), 0, false); len(l.Takes) != 0 {
		t.Fatal("a drill take reached a real listener")
	}
	if l, _ := svc.ListTakes(context.Background(), 0, true); len(l.Takes) != 1 {
		t.Fatal("a drill computer cannot read the drill take")
	}
	// Drill takes do not use the day's quota.
	for i := 1; i <= TakeDailyLimit+1; i++ {
		in := takeIn("u1", takeFixtureAudio(10, i), 10)
		in.Drill = true
		if _, err := svc.PublishTake(context.Background(), in); err != nil {
			t.Fatalf("drill take %d: %v", i, err)
		}
	}
}

func TestOutcomesAreOtherListenersActsOnly(t *testing.T) {
	svc := newTakesService(t)
	ctx := context.Background()
	t1 := mustPublish(t, svc, "pub", 0)
	t2 := mustPublish(t, svc, "pub", 1)
	comp := func(c string) string { return "computer-" + strings.Repeat(c, 32) }

	// Rule 1: the publisher's own play never counts.
	if _, err := outcome(t, svc, t1.TakeID, "pub", comp("a"), TakeHeard); !errors.Is(err, ErrTakeOwn) {
		t.Fatalf("own outcome = %v, want ErrTakeOwn", err)
	}
	// An unknown take, a bad verdict and a bad claim id are refused.
	if _, err := outcome(t, svc, "take-0000000000000000", "l1", comp("b"), TakeHeard); !errors.Is(err, ErrTakeNotFound) {
		t.Fatalf("unknown take = %v", err)
	}
	if _, err := outcome(t, svc, t1.TakeID, "l1", comp("b"), "liked"); !errors.Is(err, ErrTakeInvalid) {
		t.Fatalf("bad verdict = %v", err)
	}
	if _, err := svc.RecordTakeOutcome(ctx, TakeOutcomeIn{TakeID: t1.TakeID, ListenerUserKey: types.TakePublisherKey("l1"), ListenerComputer: comp("b"), Outcome: TakeHeard, ClaimID: "x y"}); !errors.Is(err, ErrTakeInvalid) {
		t.Fatalf("bad claim id = %v", err)
	}

	// l1 hears t1 and skips t2; l2 skips t1. A repeat by l1 on t1 changes nothing.
	for _, c := range []struct{ take, who, comp, verdict string }{
		{t1.TakeID, "l1", comp("b"), TakeHeard}, {t2.TakeID, "l1", comp("b"), TakeSkipped}, {t1.TakeID, "l2", comp("c"), TakeSkipped},
	} {
		if stored, err := outcome(t, svc, c.take, c.who, c.comp, c.verdict); err != nil || !stored {
			t.Fatalf("outcome %+v: stored=%v err=%v", c, stored, err)
		}
	}
	if stored, err := outcome(t, svc, t1.TakeID, "l1", comp("b"), TakeSkipped); err != nil || stored {
		t.Fatalf("a second outcome by one listener: stored=%v err=%v, want not stored", stored, err)
	}
	// The drill computer's outcomes are marked and never counted.
	if stored, err := outcome(t, svc, t1.TakeID, "drillacct", takesDrillComputer, TakeHeard); err != nil || !stored {
		t.Fatalf("drill outcome: %v %v", stored, err)
	}

	rec, err := svc.TakeRecord(ctx, types.TakePublisherHandle("pub"), "")
	if err != nil {
		t.Fatal(err)
	}
	s := rec.Summary
	if s.Offered != 3 || s.Heard != 1 || s.Skipped != 2 || s.Listeners != 2 || s.Takes != 2 || s.New {
		t.Fatalf("summary = %+v, want 3 offered, 1 heard, 2 skipped, 2 listeners, 2 takes", s)
	}
	if rec.Drill.DrillHeard != 1 || rec.Drill.DrillSkipped != 0 {
		t.Fatalf("drill line = %+v", rec.Drill)
	}
	if len(rec.Rows) != 0 {
		t.Fatal("a stranger reads the rows")
	}
	if !strings.Contains(s.Text, "heard through 1 of 3 times") {
		t.Fatalf("text = %q", s.Text)
	}
	// A stranger asking as someone else still gets no rows.
	if rec, _ = svc.TakeRecord(ctx, types.TakePublisherHandle("pub"), types.TakePublisherKey("l1")); len(rec.Rows) != 0 {
		t.Fatal("a listener reads the publisher's rows")
	}
	// The publisher reads every row, with the drill flag, under an opaque key.
	rec, _ = svc.TakeRecord(ctx, types.TakePublisherHandle("pub"), types.TakePublisherKey("pub"))
	if len(rec.Rows) != 4 {
		t.Fatalf("the publisher reads %d rows, want 4", len(rec.Rows))
	}
	drills := 0
	for _, r := range rec.Rows {
		if r.Drill {
			drills++
		}
		if strings.Contains(r.ListenerKey, "computer-") || strings.Contains(r.ListenerKey, "l1") {
			t.Fatalf("a row names the listener: %+v", r)
		}
	}
	if drills != 1 {
		t.Fatalf("%d drill rows, want 1", drills)
	}
	// The public list carries the summary.
	list, _ := svc.ListTakes(ctx, 0, false)
	if len(list.Takes) != 2 || list.Takes[0].Record == nil || list.Takes[0].Record.Offered != 3 {
		t.Fatalf("list record = %+v", list.Takes)
	}
	// The publisher's own list carries per-take counts, drill apart.
	mine, _ := svc.ListMyTakes(ctx, types.TakePublisherKey("pub"))
	for _, tk := range mine.Takes {
		if tk.TakeID == t1.TakeID && (tk.Outcomes.Heard != 1 || tk.Outcomes.Skipped != 1 || tk.Outcomes.DrillHeard != 1) {
			t.Fatalf("t1 outcomes = %+v", tk.Outcomes)
		}
	}
	// Deleting a take keeps its outcomes as counts.
	if err := svc.DeleteTake(ctx, t1.TakeID, types.TakePublisherKey("pub")); err != nil {
		t.Fatal(err)
	}
	if rec, _ = svc.TakeRecord(ctx, types.TakePublisherHandle("pub"), ""); rec.Summary.Offered != 3 {
		t.Fatalf("delete changed the counts: %+v", rec.Summary)
	}
}

func TestNewPublisherRecordIsNew(t *testing.T) {
	svc := newTakesService(t)
	rec, err := svc.TakeRecord(context.Background(), "voice-abcdef", "")
	if err != nil || !rec.Summary.New || rec.Summary.Text != "new: no outcomes yet" {
		t.Fatalf("record = %+v, %v", rec, err)
	}
	if _, err := svc.TakeRecord(context.Background(), "someone@example.com", ""); !errors.Is(err, ErrTakeInvalid) {
		t.Fatalf("a non-handle publisher = %v", err)
	}
}

func hostReq(method, path string, body any) *http.Request {
	var rd *bytes.Reader
	if body != nil {
		raw, _ := json.Marshal(body)
		rd = bytes.NewReader(raw)
	} else {
		rd = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, rd)
	req.RemoteAddr = "127.0.0.1:40000"
	req.Header.Set("X-Internal-Caller", "true")
	return req
}

func TestTakeRoutesAreHostOnly(t *testing.T) {
	svc := newTakesService(t)
	h := NewHandler(svc)
	take := mustPublish(t, svc, "u1", 0)
	for name, call := range map[string]func(*httptest.ResponseRecorder, *http.Request){
		"list":    func(w *httptest.ResponseRecorder, r *http.Request) { h.HandleTakes(w, r) },
		"audio":   func(w *httptest.ResponseRecorder, r *http.Request) { h.HandleTakeAudio(w, r) },
		"record":  func(w *httptest.ResponseRecorder, r *http.Request) { h.HandleTakeRecord(w, r) },
		"delete":  func(w *httptest.ResponseRecorder, r *http.Request) { h.HandleTakeDelete(w, r) },
		"outcome": func(w *httptest.ResponseRecorder, r *http.Request) { h.HandleTakeOutcome(w, r) },
		"yank":    func(w *httptest.ResponseRecorder, r *http.Request) { h.HandleTakeYank(w, r) },
	} {
		method := http.MethodPost
		if name == "list" || name == "audio" || name == "record" {
			method = http.MethodGet
		}
		w := httptest.NewRecorder()
		call(w, httptest.NewRequest(method, "/x?take_id="+take.TakeID, strings.NewReader("{}")))
		if w.Code != http.StatusForbidden {
			t.Fatalf("%s without the host credential = %d", name, w.Code)
		}
	}
}

func TestTakeRoutesRoundTrip(t *testing.T) {
	svc := newTakesService(t)
	h := NewHandler(svc)
	audio := takeFixtureAudio(10, 0)
	w := httptest.NewRecorder()
	h.HandleTakes(w, hostReq(http.MethodPost, TakesPath, takeIn("u1", audio, 10)))
	if w.Code != http.StatusCreated {
		t.Fatalf("publish = %d %s", w.Code, w.Body)
	}
	var take Take
	_ = json.Unmarshal(w.Body.Bytes(), &take)

	w = httptest.NewRecorder()
	h.HandleTakeAudio(w, hostReq(http.MethodGet, TakesAudioPath+"?take_id="+take.TakeID, nil))
	if w.Code != http.StatusOK || w.Header().Get("Content-Type") != "audio/mp4" || !bytes.Equal(w.Body.Bytes(), audio) {
		t.Fatalf("audio = %d %q", w.Code, w.Header().Get("Content-Type"))
	}
	// An unknown field in a publish is refused whole.
	w = httptest.NewRecorder()
	h.HandleTakes(w, httptest.NewRequest(http.MethodPost, TakesPath, strings.NewReader(`{"publisher":"x"}`)))
	if w.Code != http.StatusForbidden {
		t.Fatalf("outside publish = %d", w.Code)
	}
	req := hostReq(http.MethodPost, TakesPath, nil)
	req.Body = http.NoBody
	w = httptest.NewRecorder()
	h.HandleTakes(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("empty publish = %d", w.Code)
	}
	// An outcome through the route: stored once, own play forbidden.
	out := TakeOutcomeIn{TakeID: take.TakeID, ListenerUserKey: types.TakePublisherKey("u2"), ListenerComputer: "computer-" + strings.Repeat("e", 32), Outcome: TakeHeard, ClaimID: "claim-" + strings.Repeat("a", 20)}
	for i, want := range []string{`"stored":true`, `"stored":false`} {
		w = httptest.NewRecorder()
		h.HandleTakeOutcome(w, hostReq(http.MethodPost, TakesOutcomePath, out))
		if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), want) {
			t.Fatalf("outcome %d = %d %s", i, w.Code, w.Body)
		}
	}
	out.ListenerUserKey = types.TakePublisherKey("u1")
	w = httptest.NewRecorder()
	h.HandleTakeOutcome(w, hostReq(http.MethodPost, TakesOutcomePath, out))
	if w.Code != http.StatusForbidden {
		t.Fatalf("own outcome = %d", w.Code)
	}
	w = httptest.NewRecorder()
	h.HandleTakeRecord(w, hostReq(http.MethodGet, TakesRecordPath+"?publisher="+take.Publisher+"&as="+types.TakePublisherKey("u1"), nil))
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"rows"`) || !strings.Contains(w.Body.String(), `"offered":1`) {
		t.Fatalf("record = %d %s", w.Code, w.Body)
	}
	w = httptest.NewRecorder()
	h.HandleTakeDelete(w, hostReq(http.MethodPost, TakesDeletePath, map[string]string{"take_id": take.TakeID, "publisher_key": types.TakePublisherKey("u2")}))
	if w.Code != http.StatusNotFound {
		t.Fatalf("stranger delete = %d", w.Code)
	}
	w = httptest.NewRecorder()
	h.HandleTakeDelete(w, hostReq(http.MethodPost, TakesDeletePath, map[string]string{"take_id": take.TakeID, "publisher_key": types.TakePublisherKey("u1")}))
	w2 := httptest.NewRecorder()
	h.HandleTakeAudio(w2, hostReq(http.MethodGet, TakesAudioPath+"?take_id="+take.TakeID, nil))
	if w.Code != http.StatusOK || w2.Code != http.StatusNotFound {
		t.Fatalf("delete = %d, audio after = %d", w.Code, w2.Code)
	}
}

func takesJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestTakeHeaderGraceForContainerPaddingAndStopLatency(t *testing.T) {
	svc := newTakesService(t)
	// Apple's AAC encoder: 60.000 s of audio has an mvhd of 60.117 s.
	if _, err := svc.PublishTake(context.Background(), takeIn("g1", takeFixtureAudio(60.117, 0), 60.0)); err != nil {
		t.Fatalf("60.117 s header with a 60.0 claim refused: %v", err)
	}
	if _, err := svc.PublishTake(context.Background(), takeIn("g2", takeFixtureAudio(62.1, 0), 60.0)); err == nil {
		t.Fatal("a 62.1 s header was accepted")
	}
	if _, err := svc.PublishTake(context.Background(), takeIn("g3", takeFixtureAudio(2.4, 0), 3.0)); err == nil {
		t.Fatal("a 2.4 s header was accepted")
	}
	// The claim stays 3 to 60.
	if _, err := svc.PublishTake(context.Background(), takeIn("g4", takeFixtureAudio(61, 0), 61)); err == nil {
		t.Fatal("a 61 s claim was accepted")
	}
}

// Handles are 24 bits: two publishers can share one. The per-listener rows are
// the caller's own takes' rows, never the other publisher's.
func TestRecordRowsAreOnlyTheCallersTakesWhenHandlesCollide(t *testing.T) {
	svc := newTakesService(t)
	ctx := context.Background()
	in1 := takeIn("c1", takeFixtureAudio(10, 1), 10)
	in2 := takeIn("c2", takeFixtureAudio(10, 2), 10)
	in2.Publisher = in1.Publisher
	t1, err := svc.PublishTake(ctx, in1)
	if err != nil {
		t.Fatal(err)
	}
	t2, err := svc.PublishTake(ctx, in2)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{t1.TakeID, t2.TakeID} {
		if _, err := outcome(t, svc, id, "l1", "computer-"+strings.Repeat("a", 32), TakeHeard); err != nil {
			t.Fatal(err)
		}
	}
	for _, c := range []struct{ user, want string }{{"c1", t1.TakeID}, {"c2", t2.TakeID}} {
		rec, err := svc.TakeRecord(ctx, in1.Publisher, types.TakePublisherKey(c.user))
		if err != nil {
			t.Fatal(err)
		}
		if len(rec.Rows) != 1 || rec.Rows[0].TakeID != c.want {
			t.Errorf("%s reads rows %+v, want only %s", c.user, rec.Rows, c.want)
		}
	}
	mine, err := svc.ListMyTakes(ctx, types.TakePublisherKey("c1"))
	if err != nil || len(mine.Takes) != 1 || mine.Takes[0].Outcomes.Heard != 1 {
		t.Errorf("list mine = %+v %v", mine, err)
	}
	if rec, _ := svc.TakeRecord(ctx, in1.Publisher, types.TakePublisherKey("nobody")); len(rec.Rows) != 0 {
		t.Errorf("a stranger reads rows %+v", rec.Rows)
	}
}

func TestDeleteAndYankBlankTheTranscript(t *testing.T) {
	svc := newTakesService(t)
	ctx := context.Background()
	a := mustPublish(t, svc, "d1", 1)
	b := mustPublish(t, svc, "d2", 2)
	if err := svc.DeleteTake(ctx, a.TakeID, types.TakePublisherKey("d1")); err != nil {
		t.Fatal(err)
	}
	if err := svc.YankTake(ctx, b.TakeID); err != nil {
		t.Fatal(err)
	}
	og, _ := svc.takeStore()
	for _, id := range []string{a.TakeID, b.TakeID} {
		row, err := svc.getTakeRow(ctx, og, id)
		if err != nil || row == nil || row.Transcript != "" || row.RespondsTo != nil || row.State == TakeStateLive {
			t.Errorf("%s after removal = %+v %v", id, row, err)
		}
	}
}

func TestOutcomeNeedsARealSigningKeyAndTheDrillTiers(t *testing.T) {
	svc := newTakesService(t)
	take := mustPublish(t, svc, "k1", 0)
	if _, err := outcome(t, svc, take.TakeID, "k2", "computer-"+strings.Repeat("a", 32), TakeHeard); err != nil {
		t.Fatal(err)
	}
	svc.signingKey = nil
	if _, err := outcome(t, svc, take.TakeID, "k3", "computer-"+strings.Repeat("b", 32), TakeHeard); !errors.Is(err, ErrTakeUnavailable) {
		t.Fatalf("an outcome without a signing key = %v, want ErrTakeUnavailable", err)
	}
	w := httptest.NewRecorder()
	takeFail(w, "outcome", ErrTakeUnavailable)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("ErrTakeUnavailable answers %d, want 503", w.Code)
	}
}
