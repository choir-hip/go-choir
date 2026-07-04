package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/base/model"
	"github.com/yusefmosiah/go-choir/internal/computerversion"
	"github.com/yusefmosiah/go-choir/internal/server"
)

func TestOpenPersistentHandlerFeedsReadOnlyCurrentStateSource(t *testing.T) {
	root := t.TempDir()
	cfg := PersistentHandlerConfig{
		JournalPath: filepath.Join(root, "base.sqlite"),
		BlobRoot:    filepath.Join(root, "blobs"),
	}
	validator, secret := fakeValidator(t, "choir_sk_persistent", []string{ScopeWriteBase, ScopeReadBase})
	persistent, err := OpenPersistentHandler(cfg, validator)
	if err != nil {
		t.Fatalf("open persistent handler: %v", err)
	}
	persistent.Handler.SetClock(func() time.Time { return time.Date(2026, 7, 4, 13, 0, 0, 0, time.UTC) })

	rr := do(t, persistent.Routes(), http.MethodPost, "/api/base/blobs", secret, bytes.NewReader([]byte("persistent base api state")))
	if rr.Code != http.StatusOK {
		t.Fatalf("put blob status: %d body: %s", rr.Code, rr.Body.String())
	}
	var blobResp putBlobResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &blobResp); err != nil {
		t.Fatalf("decode blob response: %v", err)
	}

	itemID := model.ItemID("base_item_persistent_observation")
	itemBody, _ := json.Marshal(putItemRequest{
		ItemID:       itemID,
		EventType:    model.EventCreate,
		Kind:         model.KindFile,
		Name:         "persistent.txt",
		ParentItemID: "base_item_root",
		VersionID:    "base_ver_persistent_observation",
		BlobRef:      blobResp.BlobRef,
		ContentHash:  blobResp.SHA256,
		MediaType:    "text/plain",
	})
	rr = do(t, persistent.Routes(), http.MethodPost, "/api/base/items", secret, bytes.NewReader(itemBody))
	if rr.Code != http.StatusOK {
		t.Fatalf("put item status: %d body: %s", rr.Code, rr.Body.String())
	}
	if err := persistent.Close(); err != nil {
		t.Fatalf("close persistent handler: %v", err)
	}

	source, err := computerversion.OpenBaseCurrentStateSource(computerversion.BaseCurrentStatePaths{
		JournalPath: cfg.JournalPath,
		BlobRoot:    cfg.BlobRoot,
	})
	if err != nil {
		t.Fatalf("open read-only source: %v", err)
	}
	defer source.Close()
	observationSet, err := source.ObservationSet(context.Background(), "persistent-base-api", computerversion.ComputerVersion{
		CodeRef:            "test-code-ref",
		ArtifactProgramRef: computerversion.ArtifactProgramRef("base-sqlite:" + cfg.JournalPath),
	})
	if err != nil {
		t.Fatalf("observe current state: %v", err)
	}
	var sawItem, sawBlob bool
	for _, observation := range observationSet.Observations {
		switch {
		case observation.Kind == computerversion.ObservationFileManifest && observation.Key == string(itemID):
			sawItem = strings.Contains(observation.Value, string(blobResp.BlobRef))
		case observation.Kind == computerversion.ObservationBlobSet && observation.Key == string(blobResp.BlobRef):
			sawBlob = strings.Contains(observation.Value, blobResp.SHA256)
		}
	}
	if !sawItem || !sawBlob {
		t.Fatalf("missing persisted observations: sawItem=%v sawBlob=%v set=%#v", sawItem, sawBlob, observationSet.Observations)
	}
}

func TestRegisterPersistentRoutesOnSharedServer(t *testing.T) {
	root := t.TempDir()
	cfg := PersistentHandlerConfig{
		JournalPath: filepath.Join(root, "base.sqlite"),
		BlobRoot:    filepath.Join(root, "blobs"),
	}
	validator, secret := fakeValidator(t, "choir_sk_route", []string{ScopeWriteBase, ScopeReadBase})
	persistent, err := OpenPersistentHandler(cfg, validator)
	if err != nil {
		t.Fatalf("open persistent handler: %v", err)
	}
	defer persistent.Close()

	srv := server.NewServer("base-local-harness", "0")
	if err := RegisterPersistentRoutes(srv, persistent); err != nil {
		t.Fatalf("register routes: %v", err)
	}

	rr := do(t, srv, http.MethodPost, "/api/base/blobs", secret, bytes.NewReader([]byte("shared server route state")))
	if rr.Code != http.StatusOK {
		t.Fatalf("put blob through shared server status: %d body: %s", rr.Code, rr.Body.String())
	}
	var blobResp putBlobResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &blobResp); err != nil {
		t.Fatalf("decode blob response: %v", err)
	}
	if blobResp.BlobRef == "" {
		t.Fatal("empty blob ref")
	}

	health := do(t, srv, http.MethodGet, "/health", "", nil)
	if health.Code != http.StatusOK {
		t.Fatalf("shared server health route was not preserved: %d", health.Code)
	}
}

func TestOpenPersistentHandlerRejectsMissingPaths(t *testing.T) {
	validator, _ := fakeValidator(t, "choir_sk_persistent", []string{ScopeWriteBase, ScopeReadBase})
	if _, err := OpenPersistentHandler(PersistentHandlerConfig{BlobRoot: filepath.Join(t.TempDir(), "blobs")}, validator); err == nil {
		t.Fatal("expected missing journal path to fail")
	}
	if _, err := OpenPersistentHandler(PersistentHandlerConfig{JournalPath: filepath.Join(t.TempDir(), "base.sqlite")}, validator); err == nil {
		t.Fatal("expected missing blob root to fail")
	}
}
