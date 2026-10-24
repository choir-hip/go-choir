package agentcore

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/computerversion"
	"github.com/yusefmosiah/go-choir/internal/mediastate"
	choirstore "github.com/yusefmosiah/go-choir/internal/store"
)

// These tests pin docs/problems/stock-ui-writes-make-a-computer-checkpoint-ineligible-2026-10-24.md.
// The stock frontend writes media_recents, media_progress and user_preferences
// in ordinary use; while those tables were direct-write (empty_until_supported)
// one opened PDF or one theme change made the computer replay-ineligible, so
// it could not take a checkpoint and could not self-develop.

const mediaStateOwner = "owner-media-replay"

func mediaStateCall(t *testing.T, handler http.HandlerFunc, method, path, body string) map[string]any {
	t.Helper()
	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(method, path, nil)
	} else {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
	}
	req.Header.Set("X-Authenticated-User", mediaStateOwner)
	req.Header.Set("X-Choir-Device", "device-replay")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("%s %s: status %d body=%s", method, path, w.Code, w.Body.String())
	}
	var out map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("%s %s: decode %q: %v", method, path, w.Body.String(), err)
	}
	return out
}

// mediaStateWrites is the stock frontend's ordinary use: open a PDF, play a
// song, change the theme.
func mediaStateWrites(t *testing.T, h *mediastate.Handler) {
	t.Helper()
	mediaStateCall(t, h.HandleMediaRecents, http.MethodPut, "/api/media/recents",
		`{"kind":"pdf","identity":"file:/docs/paper.pdf","title":"Paper","file_name":"paper.pdf","file_path":"/docs/paper.pdf","media_type":"application/pdf"}`)
	mediaStateCall(t, h.HandleMediaProgress, http.MethodPut, "/api/media/progress",
		`{"kind":"audio","identity":"file:/music/song.mp3","current_time":42.125,"duration":180.5,"playback_rate":1.25,"updated_by_device":"device-replay"}`)
	mediaStateCall(t, h.HandleThemePreference, http.MethodPut, "/api/preferences/theme",
		`{"theme":{"mode":"dark","accent":"teal"}}`)
}

func requireMediaStateReplayEligible(t *testing.T, rt *Runtime, computerID string) ReplayCompletenessReport {
	t.Helper()
	report, err := rt.ReplayCompleteness(context.Background(), computerID)
	if err != nil {
		t.Fatalf("replay completeness: %v", err)
	}
	if !report.Eligibility.Eligible || !report.Result.Equivalent() {
		var keys []string
		for _, difference := range report.Result.Differences {
			keys = append(keys, difference.Key)
		}
		t.Fatalf("replay not eligible/equivalent: reason=%q unsupported=%v missing=%v drift=%v differences=%v",
			report.Eligibility.Reason, report.Eligibility.UnsupportedTables, report.Eligibility.MissingTables,
			report.Eligibility.SchemaDrift, keys)
	}
	return report
}

// mediaStateRows dumps the three tables in key order, every column, as text,
// so live and replayed rows can be compared exactly.
func mediaStateRows(t *testing.T, db *sql.DB) []string {
	t.Helper()
	var out []string
	for _, q := range []string{
		`SELECT CONCAT_WS('|', owner_id, media_kind, media_identity_hash, media_identity, CAST(position_seconds AS CHAR), CAST(duration_seconds AS CHAR), CAST(playback_rate AS CHAR), updated_by_device, CAST(updated_at AS CHAR)) FROM media_progress ORDER BY owner_id, media_kind, media_identity_hash`,
		`SELECT CONCAT_WS('|', owner_id, media_kind, media_identity_hash, media_identity, title, file_name, file_path, source_url, media_type, content_id, CAST(opened_at AS CHAR)) FROM media_recents ORDER BY owner_id, media_kind, media_identity_hash`,
		`SELECT CONCAT_WS('|', owner_id, preference_key, value_json, CAST(updated_at AS CHAR)) FROM user_preferences ORDER BY owner_id, preference_key`,
	} {
		rows, err := db.Query(q)
		if err != nil {
			t.Fatalf("query %q: %v", q, err)
		}
		for rows.Next() {
			var row string
			if err := rows.Scan(&row); err != nil {
				_ = rows.Close()
				t.Fatal(err)
			}
			out = append(out, row)
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		_ = rows.Close()
	}
	return out
}

func requireReplayedRowsMatchLive(t *testing.T, rt *Runtime, computerID string, wantRows int) {
	t.Helper()
	probe, err := rt.openReplayProbe(context.Background(), computerID)
	if err != nil {
		t.Fatal(err)
	}
	defer probe.close()
	live := mediaStateRows(t, rt.store.DB())
	replayed := mediaStateRows(t, probe.replayStore.DB())
	if len(live) != wantRows {
		t.Fatalf("live media-state rows = %d, want %d: %v", len(live), wantRows, live)
	}
	if !reflect.DeepEqual(live, replayed) {
		t.Fatalf("replayed media-state rows differ from live:\nlive   %v\nreplay %v", live, replayed)
	}
}

func TestMediaStateWritesReplayEligibleAndEquivalent(t *testing.T) {
	computerID := "computer-media-state-replay"
	rt, _ := selfdevReplayRuntime(t, computerID)
	mediaStateWrites(t, mediastate.NewHandler(rt.store, nil))
	requireMediaStateReplayEligible(t, rt, computerID)
	requireReplayedRowsMatchLive(t, rt, computerID, 3)
}

// seedPreChangeMediaState writes rows the way the binary before this change
// did: direct SQL, nothing on the tape. b@c's media_recents row is this.
func seedPreChangeMediaState(t *testing.T, db *sql.DB) {
	t.Helper()
	stamp := time.Date(2026, 10, 20, 9, 30, 15, 0, time.UTC).Format(time.RFC3339Nano)
	for _, stmt := range []struct {
		q    string
		args []any
	}{
		{`INSERT INTO media_recents (owner_id, media_kind, media_identity_hash, media_identity, title, file_name, file_path, source_url, media_type, content_id, opened_at)
			VALUES (?, 'pdf', ?, 'file:/docs/old.pdf', 'Old', 'old.pdf', '/docs/old.pdf', '', 'application/pdf', '', ?)`,
			[]any{mediaStateOwner, preChangeIdentityHash("file:/docs/old.pdf"), stamp}},
		{`INSERT INTO media_progress (owner_id, media_kind, media_identity_hash, media_identity, position_seconds, duration_seconds, playback_rate, updated_by_device, updated_at)
			VALUES (?, 'podcast', ?, 'https://feeds.example/ep1.mp3', 1234.5, 3600, 1.5, 'phone', ?)`,
			[]any{mediaStateOwner, preChangeIdentityHash("https://feeds.example/ep1.mp3"), stamp}},
		{`INSERT INTO user_preferences (owner_id, preference_key, value_json, updated_at) VALUES (?, 'theme', '{"mode":"light"}', ?)`,
			[]any{mediaStateOwner, stamp}},
	} {
		if _, err := db.Exec(stmt.q, stmt.args...); err != nil {
			t.Fatalf("seed pre-change row: %v", err)
		}
	}
}

func preChangeIdentityHash(identity string) string {
	// store.mediaIdentityHash: sha256 hex of the trimmed identity.
	sum := sha256.Sum256([]byte(strings.TrimSpace(identity)))
	return hex.EncodeToString(sum[:])
}

func TestMediaStateRowsFromBeforeTheChangeBecomeEligible(t *testing.T) {
	for _, tc := range []struct {
		name     string
		seed     func(t *testing.T, rt *Runtime, computerID string)
		wantRows int
	}{
		{
			// The first tape-bound write of the family records every existing
			// row before its own op, in one batch.
			name: "the first write seeds",
			seed: func(t *testing.T, rt *Runtime, _ string) {
				mediaStateCall(t, mediastate.NewHandler(rt.store, nil).HandleMediaRecents, http.MethodPut, "/api/media/recents",
					`{"kind":"pdf","identity":"file:/docs/new.pdf","title":"New"}`)
			},
			wantRows: 4,
		},
		{
			// A computer nobody writes: the owner-scoped residue import.
			name: "the residue import seeds",
			seed: func(t *testing.T, rt *Runtime, computerID string) {
				report, err := rt.ImportResidueSnapshot(context.Background(), computerID, mediaStateOwner)
				if err != nil {
					t.Fatalf("residue import: %v", err)
				}
				if !report.Appended {
					t.Fatalf("residue import appended nothing: %+v", report)
				}
			},
			wantRows: 3,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			computerID := "computer-media-state-pre-change"
			rt, _ := selfdevReplayRuntime(t, computerID)
			seedPreChangeMediaState(t, rt.store.DB())

			before, err := rt.ReplayCompleteness(context.Background(), computerID)
			if err != nil {
				t.Fatal(err)
			}
			if before.Eligibility.Eligible {
				t.Fatal("control: rows that are not on the tape replayed eligible")
			}

			tc.seed(t, rt, computerID)
			requireMediaStateReplayEligible(t, rt, computerID)
			requireReplayedRowsMatchLive(t, rt, computerID, tc.wantRows)

			// The pre-change rows read back unchanged.
			h := mediastate.NewHandler(rt.store, nil)
			theme := mediaStateCall(t, h.HandleThemePreference, http.MethodGet, "/api/preferences/theme", "")
			if got := fmt.Sprint(theme["theme"]); got != "map[mode:light]" {
				t.Fatalf("pre-change theme = %s", got)
			}
			progress := mediaStateCall(t, h.HandleMediaProgress, http.MethodGet, "/api/media/progress?kind=podcast&identity=https://feeds.example/ep1.mp3", "")
			if progress["current_time"] != 1234.5 || progress["playback_rate"] != 1.5 || progress["updated_by_device"] != "phone" {
				t.Fatalf("pre-change progress = %v", progress)
			}
		})
	}
}

// normalizeMediaStateResponse drops the server clock's stamps, which differ
// between two runs of the same sequence.
func normalizeMediaStateResponse(v map[string]any) map[string]any {
	out := map[string]any{}
	for k, val := range v {
		switch k {
		case "updated_at", "opened_at":
			continue
		case "items":
			items, _ := val.([]any)
			norm := make([]any, 0, len(items))
			for _, item := range items {
				m, _ := item.(map[string]any)
				norm = append(norm, normalizeMediaStateResponse(m))
			}
			out[k] = norm
		default:
			out[k] = val
		}
	}
	return out
}

// mediaStateOverwrites writes each key twice and reads it back; the second
// write must win, as it did with direct SQL.
func mediaStateOverwrites(t *testing.T, h *mediastate.Handler) []map[string]any {
	t.Helper()
	var out []map[string]any
	call := func(handler http.HandlerFunc, method, path, body string) {
		out = append(out, normalizeMediaStateResponse(mediaStateCall(t, handler, method, path, body)))
	}
	call(h.HandleMediaProgress, http.MethodPut, "/api/media/progress", `{"kind":"audio","identity":"file:/a.mp3","current_time":10,"duration":100}`)
	call(h.HandleMediaProgress, http.MethodPut, "/api/media/progress", `{"kind":"audio","identity":"file:/a.mp3","current_time":20.5,"duration":100,"playback_rate":2,"updated_by_device":"tablet"}`)
	call(h.HandleMediaProgress, http.MethodGet, "/api/media/progress?kind=audio&identity=file:/a.mp3", "")
	call(h.HandleMediaProgress, http.MethodGet, "/api/media/progress?kind=audio&identity=file:/missing.mp3", "")
	call(h.HandleMediaRecents, http.MethodPut, "/api/media/recents", `{"kind":"epub","identity":"file:/b.epub","title":"First title"}`)
	call(h.HandleMediaRecents, http.MethodPut, "/api/media/recents", `{"kind":"epub","identity":"file:/b.epub","title":"Second title","file_name":"b.epub"}`)
	call(h.HandleMediaRecents, http.MethodGet, "/api/media/recents?kind=epub", "")
	call(h.HandleThemePreference, http.MethodGet, "/api/preferences/theme", "")
	call(h.HandleThemePreference, http.MethodPut, "/api/preferences/theme", `{"theme":{"mode":"dark"}}`)
	call(h.HandleThemePreference, http.MethodPut, "/api/preferences/theme", `{"theme":{"mode":"light","contrast":"high"}}`)
	call(h.HandleThemePreference, http.MethodGet, "/api/preferences/theme", "")
	return out
}

func TestMediaStateLastWriteWinsMatchesDirectSQL(t *testing.T) {
	direct, err := choirstore.Open(filepath.Join(t.TempDir(), "direct.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = direct.Close() })
	want := mediaStateOverwrites(t, mediastate.NewHandler(direct, nil))

	computerID := "computer-media-state-lww"
	rt, _ := selfdevReplayRuntime(t, computerID)
	got := mediaStateOverwrites(t, mediastate.NewHandler(rt.store, nil))
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("tape-bound responses differ from direct SQL:\ngot  %v\nwant %v", got, want)
	}
	requireMediaStateReplayEligible(t, rt, computerID)
	requireReplayedRowsMatchLive(t, rt, computerID, 3)
}

func TestMediaStateTablesAreEventProjections(t *testing.T) {
	manifest := CurrentReplayAirworthinessManifest()
	for _, table := range []string{"media_progress", "media_recents", "user_preferences"} {
		if got := manifest.Entries[table]; got != ReplayEventProjection {
			t.Fatalf("%s class=%q, want event_projection", table, got)
		}
	}
	if err := manifest.Validate(); err != nil {
		t.Fatal(err)
	}

	// The refusal names the tables that block it (the CLI's 409 said only
	// "behavior-bearing direct-write tables are non-empty without reducers").
	head := &computerevent.Head{ComputerID: "computer", Sequence: 1, CanonicalEventHead: "head"}
	live := computerversion.ObservationSet{Observations: []computerversion.Observation{{
		Kind: computerversion.ObservationDoltHead, Key: "dolt:texture:table:co_super_slots", Value: "non-empty",
	}}}
	eligibility := replayEligibility(head, head, live, live, computerversion.EquivalenceResult{Status: computerversion.EquivalenceEquivalent})
	if err := eligibility.Error(); err == nil || !strings.Contains(err.Error(), "co_super_slots") {
		t.Fatalf("ineligible error %v does not name the blocking table", err)
	}
}
