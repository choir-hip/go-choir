package api

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerversion"

	"github.com/yusefmosiah/go-choir/internal/auth"
	"github.com/yusefmosiah/go-choir/internal/base/blob"
	"github.com/yusefmosiah/go-choir/internal/base/journal"
	"github.com/yusefmosiah/go-choir/internal/base/model"
)

// mockValidator is a test APIKeyValidator that returns a fixed key.
type mockValidator struct {
	key     *auth.APIKey
	user    *auth.User
	touched bool
}

func (m *mockValidator) GetAPIKeyByHash(ctx context.Context, keyHash string) (*auth.APIKey, error) {
	h := sha256.Sum256([]byte(m.key.Label + "-secret")) // not real, just deterministic
	_ = h
	// We match by a stored hash field set in newMockValidator.
	if keyHash == m.key.Label { // abuse Label to carry expected hash for simplicity
		return m.key, nil
	}
	return nil, errNotFound
}

func (m *mockValidator) TouchAPIKeyLastUsed(ctx context.Context, keyID string) error {
	m.touched = true
	return nil
}

func (m *mockValidator) GetUserByID(id string) (*auth.User, error) {
	if id == m.user.ID {
		return m.user, nil
	}
	return nil, errNotFound
}

var errNotFound = errors.New("not found")

// fakeValidator returns a validator that authenticates a single key whose
// secret is `secret`. The key hash is SHA-256(secret).
func fakeValidator(t *testing.T, secret string, scopes []string) (*mockValidator, string) {
	t.Helper()
	h := sha256.Sum256([]byte(secret))
	keyHash := hex.EncodeToString(h[:])
	user := &auth.User{ID: "user_test", Email: "test@choir.local", CreatedAt: time.Now().UTC()}
	ak := &auth.APIKey{
		ID:     "ak_test",
		UserID: user.ID,
		Label:  keyHash, // carry the expected hash through Label
		Scopes: scopes,
	}
	return &mockValidator{key: ak, user: user}, secret
}

func newHandler(t *testing.T, scopes []string) (*Handler, *mockValidator, string, *blob.Store, *journal.MemJournal) {
	t.Helper()
	dir := t.TempDir()
	bs, err := blob.NewStore(dir)
	if err != nil {
		t.Fatalf("blob store: %v", err)
	}
	jr := journal.NewMemJournal()
	v, secret := fakeValidator(t, "choir_sk_testsecret", scopes)
	h := NewHandler(bs, jr, v)
	h.SetClock(func() time.Time { return time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC) })
	return h, v, secret, bs, jr
}

func do(t *testing.T, h http.Handler, method, path, secret string, body io.Reader) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, body)
	if secret != "" {
		req.Header.Set("Authorization", "Bearer "+secret)
	}
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}

func TestPutBlobSuccess(t *testing.T) {
	h, _, secret, _, _ := newHandler(t, []string{ScopeWriteBase, ScopeReadBase})
	data := []byte("blob content for api test")
	rr := do(t, h.Routes(), http.MethodPost, "/api/base/blobs", secret, bytes.NewReader(data))
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d body: %s", rr.Code, rr.Body.String())
	}
	var resp putBlobResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	sum := sha256.Sum256(data)
	wantHex := hex.EncodeToString(sum[:])
	if string(resp.BlobRef) != "sha256:"+wantHex {
		t.Fatalf("blob_ref: got %q want sha256:%s", resp.BlobRef, wantHex)
	}
	if resp.SizeBytes != int64(len(data)) {
		t.Fatalf("size: got %d want %d", resp.SizeBytes, len(data))
	}
}

func TestPutBlobNoAuth(t *testing.T) {
	h, _, _, _, _ := newHandler(t, []string{ScopeWriteBase})
	rr := do(t, h.Routes(), http.MethodPost, "/api/base/blobs", "", bytes.NewReader([]byte("x")))
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status: got %d want 401", rr.Code)
	}
}

func TestPutBlobWrongScope(t *testing.T) {
	h, _, secret, _, _ := newHandler(t, []string{ScopeReadBase})
	rr := do(t, h.Routes(), http.MethodPost, "/api/base/blobs", secret, bytes.NewReader([]byte("x")))
	if rr.Code != http.StatusForbidden {
		t.Fatalf("status: got %d want 403", rr.Code)
	}
}

func TestPutItemAndDelta(t *testing.T) {
	h, _, secret, _, jr := newHandler(t, []string{ScopeWriteBase, ScopeReadBase})
	itemID := model.ItemID("base_item_" + "abc123")
	body := putItemRequest{
		ItemID:    itemID,
		EventType: model.EventCreate,
		Kind:      model.KindFile,
		Name:      "notes.txt",
		VersionID: model.VersionID("base_ver_v1"),
		BlobRef:   model.BlobRef("sha256:" + strings.Repeat("a", 64)),
	}
	b, _ := json.Marshal(body)
	rr := do(t, h.Routes(), http.MethodPost, "/api/base/items", secret, bytes.NewReader(b))
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d body: %s", rr.Code, rr.Body.String())
	}
	var resp putItemResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.EventID == "" {
		t.Fatal("empty event id")
	}
	if resp.CursorSeq != 1 {
		t.Fatalf("cursor: got %d want 1", resp.CursorSeq)
	}

	// Verify the journal event carries the authenticated subject.
	entries := jr.Entries()
	if len(entries) != 1 {
		t.Fatalf("entries: got %d want 1", len(entries))
	}
	if entries[0].Event.SubjectID != "user_test" {
		t.Fatalf("subject: got %q want user_test", entries[0].Event.SubjectID)
	}

	// Delta query returns the event.
	rr = do(t, h.Routes(), http.MethodGet, "/api/base/delta?cursor=0", secret, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("delta status: %d", rr.Code)
	}
	var dresp deltaResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &dresp); err != nil {
		t.Fatalf("decode delta: %v", err)
	}
	if len(dresp.Events) != 1 {
		t.Fatalf("delta events: got %d want 1", len(dresp.Events))
	}
	if dresp.Cursor != 1 {
		t.Fatalf("delta cursor: got %d want 1", dresp.Cursor)
	}
	if dresp.Head != 1 {
		t.Fatalf("delta head: got %d want 1", dresp.Head)
	}

	// Delta with cursor at head returns no events.
	rr = do(t, h.Routes(), http.MethodGet, "/api/base/delta?cursor=1", secret, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("delta2 status: %d", rr.Code)
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &dresp); err != nil {
		t.Fatalf("decode delta2: %v", err)
	}
	if len(dresp.Events) != 0 {
		t.Fatalf("delta2 events: got %d want 0", len(dresp.Events))
	}
}

func TestBaseAPIWritesCanFeedReadOnlyCurrentStateObservation(t *testing.T) {
	root := t.TempDir()
	blobRoot := filepath.Join(root, "blobs")
	bs, err := blob.NewStore(blobRoot)
	if err != nil {
		t.Fatalf("blob store: %v", err)
	}
	journalPath := filepath.Join(root, "base.sqlite")
	jr, err := journal.NewSQLiteJournal(journalPath)
	if err != nil {
		t.Fatalf("sqlite journal: %v", err)
	}
	v, secret := fakeValidator(t, "choir_sk_testsecret", []string{ScopeWriteBase, ScopeReadBase})
	h := NewHandler(bs, jr, v)
	h.SetClock(func() time.Time { return time.Date(2026, 7, 4, 12, 0, 0, 0, time.UTC) })

	blobBody := []byte("base api durable state")
	rr := do(t, h.Routes(), http.MethodPost, "/api/base/blobs", secret, bytes.NewReader(blobBody))
	if rr.Code != http.StatusOK {
		t.Fatalf("put blob status: %d body: %s", rr.Code, rr.Body.String())
	}
	var blobResp putBlobResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &blobResp); err != nil {
		t.Fatalf("decode blob: %v", err)
	}

	itemID := model.ItemID("base_item_api_observation")
	itemBody, _ := json.Marshal(putItemRequest{
		ItemID:       itemID,
		EventType:    model.EventCreate,
		Kind:         model.KindFile,
		Name:         "observed.txt",
		ParentItemID: "base_item_root",
		VersionID:    "base_ver_api_observation",
		BlobRef:      blobResp.BlobRef,
		ContentHash:  blobResp.SHA256,
		MediaType:    "text/plain",
	})
	rr = do(t, h.Routes(), http.MethodPost, "/api/base/items", secret, bytes.NewReader(itemBody))
	if rr.Code != http.StatusOK {
		t.Fatalf("put item status: %d body: %s", rr.Code, rr.Body.String())
	}
	if err := jr.Close(); err != nil {
		t.Fatalf("close writable journal: %v", err)
	}

	source, err := computerversion.OpenBaseCurrentStateSource(computerversion.BaseCurrentStatePaths{
		JournalPath: journalPath,
		BlobRoot:    blobRoot,
	})
	if err != nil {
		t.Fatalf("open current state source: %v", err)
	}
	defer source.Close()
	observationSet, err := source.ObservationSet(context.Background(), "base-api-current-state", computerversion.ComputerVersion{
		CodeRef:            "test-code-ref",
		ArtifactProgramRef: computerversion.ArtifactProgramRef("base-sqlite:" + journalPath),
	})
	if err != nil {
		t.Fatalf("observe current state: %v", err)
	}
	kinds := observationSet.RequiredKinds()
	if len(kinds) != 2 || kinds[0] != computerversion.ObservationBlobSet || kinds[1] != computerversion.ObservationFileManifest {
		t.Fatalf("required kinds = %#v", kinds)
	}
	if len(observationSet.Observations) != 2 {
		t.Fatalf("expected file manifest and blob observation, got %#v", observationSet.Observations)
	}
	var sawItem, sawBlob bool
	for _, observation := range observationSet.Observations {
		switch {
		case observation.Kind == computerversion.ObservationFileManifest && observation.Key == string(itemID):
			if !strings.Contains(observation.Value, string(blobResp.BlobRef)) {
				t.Fatalf("file observation does not reference uploaded blob: %s", observation.Value)
			}
			sawItem = true
		case observation.Kind == computerversion.ObservationBlobSet && observation.Key == string(blobResp.BlobRef):
			if !strings.Contains(observation.Value, blobResp.SHA256) {
				t.Fatalf("blob observation does not carry uploaded hash: %s", observation.Value)
			}
			sawBlob = true
		}
	}
	if !sawItem || !sawBlob {
		t.Fatalf("missing observations: sawItem=%v sawBlob=%v set=%#v", sawItem, sawBlob, observationSet.Observations)
	}
}

func TestGetItem(t *testing.T) {
	h, _, secret, _, _ := newHandler(t, []string{ScopeWriteBase, ScopeReadBase})
	itemID := model.ItemID("base_item_getitem")
	body := putItemRequest{
		ItemID:       itemID,
		EventType:    model.EventCreate,
		Kind:         model.KindFolder,
		Name:         "docs",
		ParentItemID: "",
		VersionID:    model.VersionID("base_ver_folder1"),
	}
	b, _ := json.Marshal(body)
	rr := do(t, h.Routes(), http.MethodPost, "/api/base/items", secret, bytes.NewReader(b))
	if rr.Code != http.StatusOK {
		t.Fatalf("create status: %d body: %s", rr.Code, rr.Body.String())
	}

	rr = do(t, h.Routes(), http.MethodGet, "/api/base/items/"+string(itemID), secret, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("get status: %d body: %s", rr.Code, rr.Body.String())
	}
	var resp itemResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Item.ItemID != itemID {
		t.Fatalf("item id: got %q want %q", resp.Item.ItemID, itemID)
	}
	if resp.Item.Name != "docs" {
		t.Fatalf("name: got %q want docs", resp.Item.Name)
	}
}

func TestGetItemNotFound(t *testing.T) {
	h, _, secret, _, _ := newHandler(t, []string{ScopeReadBase})
	rr := do(t, h.Routes(), http.MethodGet, "/api/base/items/base_item_nope", secret, nil)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status: got %d want 404", rr.Code)
	}
}

func TestGetStatus(t *testing.T) {
	h, _, secret, _, _ := newHandler(t, []string{ScopeWriteBase, ScopeReadBase})
	itemID := model.ItemID("base_item_status")
	body := putItemRequest{
		ItemID:    itemID,
		EventType: model.EventCreate,
		Kind:      model.KindFile,
		Name:      "file.txt",
		VersionID: model.VersionID("base_ver_s1"),
		BlobRef:   model.BlobRef("sha256:" + strings.Repeat("b", 64)),
	}
	b, _ := json.Marshal(body)
	do(t, h.Routes(), http.MethodPost, "/api/base/items", secret, bytes.NewReader(b))

	rr := do(t, h.Routes(), http.MethodGet, "/api/base/items/"+string(itemID)+"/status", secret, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status code: %d body: %s", rr.Code, rr.Body.String())
	}
	var resp statusResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.ItemID != itemID {
		t.Fatalf("item id: got %q want %q", resp.ItemID, itemID)
	}
	if resp.State != model.StateSynced {
		t.Fatalf("state: got %q want synced", resp.State)
	}
}

func TestReadScopeRequiredForGets(t *testing.T) {
	h, _, secret, _, _ := newHandler(t, []string{ScopeWriteBase}) // no read scope
	rr := do(t, h.Routes(), http.MethodGet, "/api/base/items/base_item_x", secret, nil)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("status: got %d want 403", rr.Code)
	}
	rr = do(t, h.Routes(), http.MethodGet, "/api/base/delta", secret, nil)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("delta status: got %d want 403", rr.Code)
	}
}

func TestRepairPreview(t *testing.T) {
	h, _, secret, _, _ := newHandler(t, []string{ScopeWriteBase, ScopeReadBase})
	// Remote has an item local doesn't -> download action.
	remoteItem := model.Item{
		ItemID:         model.ItemID("base_item_repair"),
		OwnerID:        "owner",
		Name:           "remote.txt",
		Kind:           model.KindFile,
		CurrentVersion: model.VersionID("base_ver_r1"),
	}
	remoteVer := model.Version{
		VersionID: model.VersionID("base_ver_r1"),
		ItemID:    model.ItemID("base_item_repair"),
		BlobRef:   model.BlobRef("sha256:" + strings.Repeat("c", 64)),
	}
	req := repairPreviewRequest{
		Remote: treeSnapshot{
			Items:    []model.Item{remoteItem},
			Versions: []model.Version{remoteVer},
		},
	}
	b, _ := json.Marshal(req)
	rr := do(t, h.Routes(), http.MethodPost, "/api/base/repair/preview", secret, bytes.NewReader(b))
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d body: %s", rr.Code, rr.Body.String())
	}
	var resp repairPreviewResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Actions) != 1 {
		t.Fatalf("actions: got %d want 1", len(resp.Actions))
	}
	if resp.Actions[0].Type != "download" {
		t.Fatalf("action type: got %q want download", resp.Actions[0].Type)
	}
}

func TestInvalidBearerToken(t *testing.T) {
	h, _, _, _, _ := newHandler(t, []string{ScopeReadBase})
	rr := do(t, h.Routes(), http.MethodGet, "/api/base/delta", "choir_sk_wrong", nil)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status: got %d want 401", rr.Code)
	}
}

func TestNonAPIKeyBearer(t *testing.T) {
	h, _, _, _, _ := newHandler(t, []string{ScopeReadBase})
	rr := do(t, h.Routes(), http.MethodGet, "/api/base/delta", "some-other-token", nil)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status: got %d want 401", rr.Code)
	}
}

func TestNoSecretsInResponses(t *testing.T) {
	h, _, secret, _, _ := newHandler(t, []string{ScopeWriteBase, ScopeReadBase})
	rr := do(t, h.Routes(), http.MethodPost, "/api/base/blobs", secret, bytes.NewReader([]byte("no-leak")))
	if strings.Contains(rr.Body.String(), "choir_sk_") {
		t.Fatalf("response leaked secret: %s", rr.Body.String())
	}
}
