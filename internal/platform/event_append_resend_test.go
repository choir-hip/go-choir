package platform

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

// The guest's appender re-sends the SAME head CAS request when a reply is lost
// (computerevent: resendUnclearCASLocked). Its safety rests on this route:
// an identical request returns the stored receipt and adds no row; the same
// idempotency key with a changed commitment is a conflict (409); a different
// event for a sequence already taken is refused (400). F21 of
// docs/problems/event-appender-wedges-clustering-assessment-2026-11-12.md.
func TestAppendRouteAnswersAResendWithTheStoredReceipt(t *testing.T) {
	f := newAdmissionFixture(t, OwnerActObserve)
	head := f.seedHead(t, f.computerID)
	service := NewService(f.store, filepath.Join(t.TempDir(), "artifacts"), filepath.Join(t.TempDir(), "platform-signing.key"))
	artifacts, err := NewEventArtifactService(service, platformTestKeyResolver{key: service.signingKey.Public})
	if err != nil {
		t.Fatal(err)
	}
	if err := f.handler.ConfigureComputerEvents(f.cas, artifacts, f.handler.eventAuth); err != nil {
		t.Fatal(err)
	}
	post := func(request computerevent.CASRequest) *httptest.ResponseRecorder {
		body, err := json.Marshal(request)
		if err != nil {
			t.Fatal(err)
		}
		r := httptest.NewRequest(http.MethodPost, "/internal/computers/events/append", bytes.NewReader(body))
		r.RemoteAddr = "172.16.0.2:40000"
		r.Header.Set("Authorization", "Bearer "+f.guestToken)
		w := httptest.NewRecorder()
		f.handler.HandleComputerEventAppend(w, r)
		return w
	}
	rows := func() int {
		var n int
		if err := f.store.db.QueryRow(`SELECT COUNT(*) FROM computer_event_append_receipts WHERE computer_id=?`, f.computerID).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	content := digestOf('c')
	request := accepted(t, head, AuthorityPlatformUpdate, "", content, nil, digestOf('0'))

	first := post(request)
	if first.Code != http.StatusCreated {
		t.Fatalf("first append: %d %s", first.Code, first.Body.String())
	}
	if rows() != 1 {
		t.Fatalf("rows after the first append = %d, want 1", rows())
	}

	// The same request again (the guest lost the reply): the stored receipt.
	again := post(request)
	if again.Code != http.StatusCreated || again.Body.String() != first.Body.String() {
		t.Fatalf("re-send: %d %s, want 201 and the stored receipt %s", again.Code, again.Body.String(), first.Body.String())
	}
	if rows() != 1 {
		t.Fatalf("a re-send added a row: %d", rows())
	}

	// The same idempotency key with a changed (self-consistent) commitment.
	changed := request
	changed.Event.VerifierRefs = []string{"verifier:changed"}
	pinIntent, err := computerevent.ComputePinIntentCommitment(changed.Event, changed.Input)
	if err != nil {
		t.Fatal(err)
	}
	changed.PinIntentCommitment = pinIntent
	changed.Event.RequestCommitment, err = computerevent.ComputeRequestCommitment(changed.Event, changed.Input, pinIntent, nil)
	if err != nil {
		t.Fatal(err)
	}
	if changed.EventDigest, err = changed.Event.Digest(); err != nil {
		t.Fatal(err)
	}
	changed.EventArtifactDigest = changed.EventDigest
	if w := post(changed); w.Code != http.StatusConflict {
		t.Fatalf("changed commitment under the same key: %d %s, want 409", w.Code, w.Body.String())
	}

	// A different event for the sequence the first one took.
	other := accepted(t, head, AuthorityPlatformUpdate, "", content, nil, digestOf('0'))
	if w := post(other); w.Code != http.StatusBadRequest && w.Code != http.StatusConflict {
		t.Fatalf("a different event at a taken sequence: %d %s, want a definite refusal (400 or 409)", w.Code, w.Body.String())
	}
	if rows() != 1 {
		t.Fatalf("a refused request added a row: %d", rows())
	}
}
