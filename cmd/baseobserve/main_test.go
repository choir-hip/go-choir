package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/auth"
	"github.com/yusefmosiah/go-choir/internal/base/api"
	"github.com/yusefmosiah/go-choir/internal/base/model"
	"github.com/yusefmosiah/go-choir/internal/computerversion"
	"github.com/yusefmosiah/go-choir/internal/server"
)

type blobResponse struct {
	BlobRef model.BlobRef `json:"blob_ref"`
	SHA256  string        `json:"sha256"`
}

func TestRunEmitsReadOnlyBaseCurrentStateObservationSet(t *testing.T) {
	root := t.TempDir()
	journalPath := filepath.Join(root, "base.sqlite")
	blobRoot := filepath.Join(root, "blobs")
	authDBPath := filepath.Join(root, "auth.sqlite")
	secret := createAuthSecret(t, authDBPath)
	itemID, blobResp := writeBaseStateThroughPersistentAPI(t, journalPath, blobRoot, secret, authDBPath)

	var stdout, stderr bytes.Buffer
	code := run([]string{
		"--journal", journalPath,
		"--blob-root", blobRoot,
		"--code-ref", "test-code-ref",
		"--artifact-program-ref", "base-sqlite:" + journalPath,
		"--name", "baseobserve-test",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run exit=%d stderr=%s", code, stderr.String())
	}
	var set computerversion.ObservationSet
	if err := json.Unmarshal(stdout.Bytes(), &set); err != nil {
		t.Fatalf("decode observation set: %v\n%s", err, stdout.String())
	}
	if set.Name != "baseobserve-test" {
		t.Fatalf("name = %q", set.Name)
	}
	var sawItem, sawBlob bool
	for _, observation := range set.Observations {
		switch {
		case observation.Kind == computerversion.ObservationFileManifest && observation.Key == string(itemID):
			sawItem = strings.Contains(observation.Value, string(blobResp.BlobRef))
		case observation.Kind == computerversion.ObservationBlobSet && observation.Key == string(blobResp.BlobRef):
			sawBlob = strings.Contains(observation.Value, blobResp.SHA256)
		}
	}
	if !sawItem || !sawBlob {
		t.Fatalf("missing observed item/blob: sawItem=%v sawBlob=%v set=%#v", sawItem, sawBlob, set.Observations)
	}
}

func TestRunObservationSetFeedsBaseCurrentStateFileProjectionCompare(t *testing.T) {
	root := t.TempDir()
	journalPath := filepath.Join(root, "base.sqlite")
	blobRoot := filepath.Join(root, "blobs")
	authDBPath := filepath.Join(root, "auth.sqlite")
	secret := createAuthSecret(t, authDBPath)
	itemID, blobResp := writeBaseStateThroughPersistentAPI(t, journalPath, blobRoot, secret, authDBPath)

	var stdout, stderr bytes.Buffer
	code := run([]string{
		"--journal", journalPath,
		"--blob-root", blobRoot,
		"--code-ref", "test-code-ref",
		"--artifact-program-ref", "base-sqlite:" + journalPath,
		"--name", "baseobserve-compare-test",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run exit=%d stderr=%s", code, stderr.String())
	}

	var set computerversion.ObservationSet
	if err := json.Unmarshal(stdout.Bytes(), &set); err != nil {
		t.Fatalf("decode observation set: %v\n%s", err, stdout.String())
	}
	if set.Name != "baseobserve-compare-test" {
		t.Fatalf("name = %q", set.Name)
	}
	if !observationContains(set, computerversion.ObservationFileManifest, string(itemID), string(blobResp.BlobRef)) {
		t.Fatalf("missing fixture item observation for %s in %#v", itemID, set.Observations)
	}
	if !observationContains(set, computerversion.ObservationBlobSet, string(blobResp.BlobRef), blobResp.SHA256) {
		t.Fatalf("missing fixture blob observation for %s in %#v", blobResp.BlobRef, set.Observations)
	}

	result, err := computerversion.CompareBaseCurrentStateToFileProjection(context.Background(), set, set)
	if err != nil {
		t.Fatalf("compare current-state to file projection: %v", err)
	}
	if !result.Equivalent() {
		t.Fatalf("expected equivalent result, got %#v", result)
	}
	if len(result.Differences) != 0 || len(result.Unsupported) != 0 {
		t.Fatalf("equivalent result reported differences/unsupported: %#v", result)
	}
}

func TestRunDoesNotCreateMissingObservationRoots(t *testing.T) {
	root := t.TempDir()
	journalPath := filepath.Join(root, "missing.sqlite")
	blobRoot := filepath.Join(root, "missing-blobs")
	var stdout, stderr bytes.Buffer
	code := run([]string{
		"--journal", journalPath,
		"--blob-root", blobRoot,
		"--code-ref", "test-code-ref",
		"--artifact-program-ref", "base-sqlite:" + journalPath,
	}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("expected missing roots to fail, stdout=%s", stdout.String())
	}
	if _, err := os.Stat(journalPath); !os.IsNotExist(err) {
		t.Fatalf("baseobserve created missing journal or unexpected stat error: %v", err)
	}
	if _, err := os.Stat(blobRoot); !os.IsNotExist(err) {
		t.Fatalf("baseobserve created missing blob root or unexpected stat error: %v", err)
	}
}

func TestParseConfigRequiresComputerVersionRefs(t *testing.T) {
	_, err := parseConfig([]string{"--journal", "base.sqlite", "--blob-root", "blobs"}, &bytes.Buffer{})
	if err == nil {
		t.Fatal("expected missing refs to fail")
	}
	if !strings.Contains(err.Error(), "--code-ref") {
		t.Fatalf("error = %q, want missing code ref", err.Error())
	}
}

func observationContains(set computerversion.ObservationSet, kind computerversion.ObservationKind, key, valuePart string) bool {
	for _, observation := range set.Observations {
		if observation.Kind == kind && observation.Key == key && strings.Contains(observation.Value, valuePart) {
			return true
		}
	}
	return false
}

func createAuthSecret(t *testing.T, authDBPath string) string {
	t.Helper()
	store, err := auth.OpenStore(authDBPath)
	if err != nil {
		t.Fatalf("open auth store: %v", err)
	}
	defer store.Close()
	user, err := store.CreateUser("baseobserve-user", "baseobserve@example.com")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	_, secret, err := store.CreateAPIKey(context.Background(), user.ID, "baseobserve", []string{api.ScopeReadBase, api.ScopeWriteBase}, nil)
	if err != nil {
		t.Fatalf("create api key: %v", err)
	}
	return secret
}

func writeBaseStateThroughPersistentAPI(t *testing.T, journalPath, blobRoot, secret, authDBPath string) (model.ItemID, blobResponse) {
	t.Helper()
	store, err := auth.OpenStore(authDBPath)
	if err != nil {
		t.Fatalf("open auth store: %v", err)
	}
	persistent, err := api.OpenPersistentHandler(api.PersistentHandlerConfig{JournalPath: journalPath, BlobRoot: blobRoot}, store)
	if err != nil {
		t.Fatalf("open persistent handler: %v", err)
	}
	srv := server.NewServer("baseobserve-test", "0")
	if err := api.RegisterPersistentRoutes(srv, persistent); err != nil {
		t.Fatalf("register persistent routes: %v", err)
	}
	blobResp := putBlob(t, srv, secret, []byte("baseobserve bytes"))
	itemID := model.ItemID("base_item_observe_command")
	itemBody, _ := json.Marshal(map[string]any{
		"item_id":        itemID,
		"event_type":     model.EventCreate,
		"kind":           model.KindFile,
		"name":           "observe.txt",
		"parent_item_id": "base_item_root",
		"version_id":     "base_ver_observe_command",
		"blob_ref":       blobResp.BlobRef,
		"content_hash":   blobResp.SHA256,
		"media_type":     "text/plain",
	})
	rr := doRequest(t, srv, http.MethodPost, "/api/base/items", secret, bytes.NewReader(itemBody))
	if rr.Code != http.StatusOK {
		t.Fatalf("put item status: %d body: %s", rr.Code, rr.Body.String())
	}
	if err := persistent.Close(); err != nil {
		t.Fatalf("close persistent handler: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("close auth store: %v", err)
	}
	return itemID, blobResp
}

func putBlob(t *testing.T, srv *server.Server, secret string, data []byte) blobResponse {
	t.Helper()
	rr := doRequest(t, srv, http.MethodPost, "/api/base/blobs", secret, bytes.NewReader(data))
	if rr.Code != http.StatusOK {
		t.Fatalf("put blob status: %d body: %s", rr.Code, rr.Body.String())
	}
	var resp blobResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode blob response: %v", err)
	}
	return resp
}

func doRequest(t *testing.T, srv *server.Server, method, path, secret string, body *bytes.Reader) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, body)
	if secret != "" {
		req.Header.Set("Authorization", "Bearer "+secret)
	}
	if method == http.MethodPost && strings.HasSuffix(path, "/items") {
		req.Header.Set("Content-Type", "application/json")
	}
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	return rr
}
