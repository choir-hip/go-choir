package agentcore

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/store"
)

// The per-computer intake enable (L2a A6 / R2-7,
// docs/archive/reports/l2a-synthesis-intake-design-2026-10-14.md). Ways it can fail,
// written before the code:
//   - a computer with no setting and no source schedule reads as on, so
//     management carries intake text where no intake can happen (the
//     default is on, owner 2026-11-03, but only where sources exist);
//   - an unauthenticated caller, or management through choir.ProductAPI,
//     turns it on: it is an owner act;
//   - one computer's setting turns another's on (it is per computer);
//   - an unknown key or a non-boolean value is stored, so a typo reads as
//     a setting nobody can see;
//   - a store error reads as on;
//   - the setting is in memory only, so a restart turns it off silently.

func TestIntakeLabelsSettingIsOnByDefaultAndSetByTheOwner(t *testing.T) {
	ctx := context.Background()
	rt, s := testRuntime(t)
	owner, computer := "owner-settings", rt.cfg.ComputerID
	// Owner, 2026-11-03: nothing is off by default, and intake runs where
	// source schedules exist. Nothing stored and no schedule: off.
	if rt.intakeLabelsEnabled(ctx, owner, computer) {
		t.Fatal("intake.labels reads on with nothing stored on a computer without a source schedule")
	}
	if err := s.PutSourceSchedule(ctx, store.SourceSchedule{OwnerID: owner, ComputerID: "another-computer", SourceID: "rss:elsewhere"}); err != nil {
		t.Fatal(err)
	}
	if rt.intakeLabelsEnabled(ctx, owner, computer) {
		t.Fatal("another computer's source schedule turned intake.labels on here")
	}
	if err := s.PutSourceSchedule(ctx, store.SourceSchedule{OwnerID: owner, ComputerID: computer, SourceID: "rss:here"}); err != nil {
		t.Fatal(err)
	}
	if !rt.intakeLabelsEnabled(ctx, owner, computer) {
		t.Fatal("intake.labels reads off with nothing stored on a computer with a source schedule; the default is on")
	}
	handler := &APIHandler{rt: rt}
	call := func(method, body string, auth bool) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, "/api/computer-settings", strings.NewReader(body))
		if auth {
			req.Header.Set("X-Authenticated-User", owner)
		}
		rec := httptest.NewRecorder()
		handler.HandleComputerSettings(rec, req)
		return rec
	}
	// The owner turns it off first; the refused requests below must not
	// turn it back on.
	if rec := call(http.MethodPost, `{"key":"intake.labels","value":false}`, true); rec.Code != http.StatusOK {
		t.Fatalf("owner off = %d %s", rec.Code, rec.Body)
	}
	if rec := call(http.MethodPost, `{"key":"intake.labels","value":true}`, false); rec.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated set = %d", rec.Code)
	}
	if rec := call(http.MethodPost, `{"key":"intake.lables","value":true}`, true); rec.Code != http.StatusBadRequest {
		t.Fatalf("unknown key = %d %s", rec.Code, rec.Body)
	}
	if rec := call(http.MethodPost, `{"key":"intake.labels","value":"yes"}`, true); rec.Code != http.StatusBadRequest {
		t.Fatalf("non-boolean value = %d %s", rec.Code, rec.Body)
	}
	if rt.intakeLabelsEnabled(ctx, owner, computer) {
		t.Fatal("a refused request turned intake.labels on")
	}

	// GET lists the known settings, the stored off with the defaults.
	rec := call(http.MethodGet, "", true)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"key":"intake.labels"`) || !strings.Contains(rec.Body.String(), `"value":false`) {
		t.Fatalf("list = %d %s; want intake.labels false", rec.Code, rec.Body)
	}

	if rec := call(http.MethodPost, `{"key":"intake.labels","value":true}`, true); rec.Code != http.StatusOK {
		t.Fatalf("set = %d %s", rec.Code, rec.Body)
	}
	if !rt.intakeLabelsEnabled(ctx, owner, computer) {
		t.Fatal("intake.labels is off after the owner set it")
	}
	// Stored durably as a computer_setting object for this computer only.
	got, err := s.GetComputerSetting(ctx, owner, computer, store.SettingIntakeLabels)
	if err != nil || !got.Value || got.ComputerID != computer {
		t.Fatalf("stored setting = %+v, %v", got, err)
	}
	if err := s.PutComputerSetting(ctx, store.ComputerSetting{OwnerID: owner, ComputerID: "another-computer", Key: store.SettingIntakeLabels, Value: false}); err != nil {
		t.Fatal(err)
	}
	if rt.intakeLabelsEnabled(ctx, owner, "another-computer") || !rt.intakeLabelsEnabled(ctx, owner, computer) {
		t.Fatal("one computer's setting changed another's")
	}
	rec = call(http.MethodGet, "", true)
	var listed struct {
		Settings []store.ComputerSetting `json:"settings"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &listed); err != nil || len(listed.Settings) != len(store.KnownComputerSettings) {
		t.Fatalf("list after set = %s (%v)", rec.Body, err)
	}
	for _, setting := range listed.Settings {
		if want := setting.Key == store.SettingIntakeLabels || store.KnownComputerSettings[setting.Key]; setting.Value != want {
			t.Fatalf("list after set: %s = %v, want %v (%s)", setting.Key, setting.Value, want, rec.Body)
		}
	}

	// Disabling is the rollback.
	if rec := call(http.MethodPost, `{"key":"intake.labels","value":false}`, true); rec.Code != http.StatusOK {
		t.Fatalf("unset = %d %s", rec.Code, rec.Body)
	}
	if rt.intakeLabelsEnabled(ctx, owner, computer) {
		t.Fatal("intake.labels is on after the owner turned it off")
	}

	// Management's ProductAPI cannot reach it.
	if err := validateProductAPIRoute(http.MethodPost, "/api/computer-settings"); err == nil {
		t.Fatal("management's ProductAPI may set computer settings; it is an owner act")
	}
}
