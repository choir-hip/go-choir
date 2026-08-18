package platform

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

func TestEventArtifactServiceEventsPageBoundsAndEmptyResult(t *testing.T) {
	platformStore, root := openTestPlatformStore(t)
	service := NewService(platformStore, filepath.Join(root, "artifacts"), filepath.Join(root, "platform-signing.key"))
	artifacts, err := NewEventArtifactService(service, platformTestKeyResolver{key: service.signingKey.Public})
	if err != nil {
		t.Fatal(err)
	}

	records, err := artifacts.EventsPage(context.Background(), "computer-replay-page", 0, 1)
	if err != nil {
		t.Fatalf("empty replay page: %v", err)
	}
	if records == nil {
		t.Fatal("empty replay page returned nil records")
	}
	if len(records) != 0 {
		t.Fatalf("empty replay page records = %d, want 0", len(records))
	}
	for _, pageSize := range []int{0, computerevent.EventReplayMaxPageSize + 1} {
		if _, err := artifacts.EventsPage(context.Background(), "computer-replay-page", 0, pageSize); err == nil {
			t.Fatalf("invalid replay page size %d was accepted", pageSize)
		}
	}
}

func TestHandleComputerEventReplayValidatesPageLimit(t *testing.T) {
	platformStore, root := openTestPlatformStore(t)
	service := NewService(platformStore, filepath.Join(root, "artifacts"), filepath.Join(root, "platform-signing.key"))
	artifacts, err := NewEventArtifactService(service, platformTestKeyResolver{key: service.signingKey.Public})
	if err != nil {
		t.Fatal(err)
	}
	cas, err := NewComputerEventCAS(platformStore, "corpusd", service.computerEventSigningKey(), artifacts)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	token, err := MintComputerCapability(ComputerCapability{
		Version: 1, ComputerID: "computer-replay-handler", Scopes: []string{"event:read"},
		ExpiresAt: now.Add(4 * time.Minute).Format(time.RFC3339Nano), Nonce: "replay-handler-test",
	}, service.signingKey.Private)
	if err != nil {
		t.Fatal(err)
	}
	handler := NewHandler(service)
	if err := handler.ConfigureComputerEvents(cas, artifacts, SignedCapabilityVerifier{
		Store: platformStore, PublicKey: service.signingKey.Public, Now: func() time.Time { return now },
	}); err != nil {
		t.Fatal(err)
	}
	request := func(limit string) *httptest.ResponseRecorder {
		query := url.Values{"computer_id": []string{"computer-replay-handler"}, "after_sequence": []string{"0"}}
		if limit != "" {
			query.Set("limit", limit)
		}
		req := httptest.NewRequest(http.MethodGet, "/internal/computers/events/replay?"+query.Encode(), nil)
		req.Header.Set("Authorization", "Bearer "+token)
		response := httptest.NewRecorder()
		handler.HandleComputerEventReplay(response, req)
		return response
	}
	if response := request("1"); response.Code != http.StatusOK {
		t.Fatalf("valid replay page status = %d, body = %s", response.Code, response.Body.String())
	}
	if response := request(strconv.Itoa(computerevent.EventReplayMaxPageSize + 1)); response.Code != http.StatusBadRequest {
		t.Fatalf("oversized replay page status = %d, want %d", response.Code, http.StatusBadRequest)
	}
}
