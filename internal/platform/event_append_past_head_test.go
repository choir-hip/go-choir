package platform

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

// A re-send of an accepted request returns the stored receipt even after the
// host's head has moved past its sequence: the guest's kept row can be settled
// late. (existingComputerEventReceipt runs before the head is reduced.)
func TestAppendRouteAnswersAResendAfterTheHeadMovedPast(t *testing.T) {
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
	post := func(body []byte) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodPost, "/internal/computers/events/append", bytes.NewReader(body))
		r.RemoteAddr = "172.16.0.2:40000"
		r.Header.Set("Authorization", "Bearer "+f.guestToken)
		w := httptest.NewRecorder()
		f.handler.HandleComputerEventAppend(w, r)
		return w
	}
	first := accepted(t, head, AuthorityPlatformUpdate, "", digestOf('c'), nil, digestOf('0'))
	firstBody, err := json.Marshal(first)
	if err != nil {
		t.Fatal(err)
	}
	stored := post(firstBody)
	if stored.Code != http.StatusCreated {
		t.Fatalf("first append: %d %s", stored.Code, stored.Body.String())
	}
	// The chain moves past the first event's sequence (a later event commits).
	if _, err := f.store.db.Exec(`UPDATE computer_event_heads SET sequence = sequence + 1, canonical_event_head = ? WHERE computer_id = ?`, digestOf('7'), f.computerID); err != nil {
		t.Fatal(err)
	}
	again := post(firstBody)
	if again.Code != http.StatusCreated || again.Body.String() != stored.Body.String() {
		t.Fatalf("re-send after the head moved past: %d %s, want 201 and the stored receipt", again.Code, again.Body.String())
	}
}
