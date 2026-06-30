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
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/auth"
	"github.com/yusefmosiah/go-choir/internal/base/blob"
	"github.com/yusefmosiah/go-choir/internal/base/journal"
	"github.com/yusefmosiah/go-choir/internal/base/model"
	"github.com/yusefmosiah/go-choir/internal/base/tree"
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

type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 0
	}
	return len(p), nil
}

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

func TestDeltaAcceptsTrustedProxyIdentity(t *testing.T) {
	h, _, _, _, _ := newHandler(t, []string{ScopeReadBase})
	h.validator = nil
	req := httptest.NewRequest(http.MethodGet, "/api/base/delta?cursor=0", nil)
	req.Header.Set("X-Authenticated-User", "user_test")
	req.Header.Set("X-Authenticated-Email", "test@choir.local")
	req.Header.Set("X-Authenticated-Scopes", "read:base")
	rr := httptest.NewRecorder()

	h.Routes().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d want 200 body: %s", rr.Code, rr.Body.String())
	}
	var resp deltaResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Events) != 0 || resp.Cursor != 0 || resp.Head != 0 {
		t.Fatalf("delta response = %+v, want empty zero response", resp)
	}
}

func TestDeltaRejectsTrustedProxyIdentityWithoutScope(t *testing.T) {
	h, _, _, _, _ := newHandler(t, []string{ScopeReadBase})
	h.validator = nil
	req := httptest.NewRequest(http.MethodGet, "/api/base/delta?cursor=0", nil)
	req.Header.Set("X-Authenticated-User", "user_test")
	rr := httptest.NewRecorder()

	h.Routes().ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("status: got %d want 403 body: %s", rr.Code, rr.Body.String())
	}
}

func TestPutBlobRejectsOversizedBody(t *testing.T) {
	h, _, secret, _, _ := newHandler(t, []string{ScopeWriteBase})
	body := io.LimitReader(zeroReader{}, maxBlobUploadBytes+1)

	rr := do(t, h.Routes(), http.MethodPost, "/api/base/blobs", secret, body)

	if rr.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status: got %d want 413 body: %s", rr.Code, rr.Body.String())
	}
}

func TestGetBlobSuccess(t *testing.T) {
	h, _, secret, bs, jr := newHandler(t, []string{ScopeReadBase})
	data := []byte("downloadable blob content")
	ref, err := bs.Put(data)
	if err != nil {
		t.Fatalf("put blob: %v", err)
	}
	if _, err := jr.Append(model.Event{
		EventID:   "base_evt_blob_owner",
		OwnerID:   "user_test",
		ItemID:    "base_item_blob_owner",
		DeviceID:  "dev1",
		SubjectID: "user_test",
		EventType: model.EventCreate,
		Kind:      model.KindFile,
		BlobRef:   ref,
		PayloadJSON: tree.Payload{
			Name:      "owned.txt",
			Kind:      model.KindFile,
			VersionID: "base_ver_blob_owner",
			BlobRef:   ref,
		}.JSON(),
		CreatedAt: time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("append owner event: %v", err)
	}

	rr := do(t, h.Routes(), http.MethodGet, "/api/base/blobs/"+string(ref), secret, nil)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d want 200 body: %s", rr.Code, rr.Body.String())
	}
	if got := rr.Body.Bytes(); !bytes.Equal(got, data) {
		t.Fatalf("body: got %q want %q", string(got), string(data))
	}
}

func TestGetBlobDoesNotExposeForeignOwnerBlob(t *testing.T) {
	h, _, secret, bs, jr := newHandler(t, []string{ScopeReadBase})
	ref, err := bs.Put([]byte("foreign blob content"))
	if err != nil {
		t.Fatalf("put blob: %v", err)
	}
	if _, err := jr.Append(model.Event{
		EventID:   "base_evt_blob_foreign",
		OwnerID:   "other_owner",
		ItemID:    "base_item_blob_foreign",
		DeviceID:  "dev1",
		SubjectID: "other_owner",
		EventType: model.EventCreate,
		Kind:      model.KindFile,
		BlobRef:   ref,
		PayloadJSON: tree.Payload{
			Name:      "foreign.txt",
			Kind:      model.KindFile,
			VersionID: "base_ver_blob_foreign",
			BlobRef:   ref,
		}.JSON(),
		CreatedAt: time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("append foreign event: %v", err)
	}

	rr := do(t, h.Routes(), http.MethodGet, "/api/base/blobs/"+string(ref), secret, nil)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status: got %d want 404 body: %s", rr.Code, rr.Body.String())
	}
}

func TestGetBlobWrongScope(t *testing.T) {
	h, _, secret, bs, _ := newHandler(t, []string{ScopeWriteBase})
	ref, err := bs.Put([]byte("private blob"))
	if err != nil {
		t.Fatalf("put blob: %v", err)
	}

	rr := do(t, h.Routes(), http.MethodGet, "/api/base/blobs/"+string(ref), secret, nil)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("status: got %d want 403", rr.Code)
	}
}

func TestPutItemRejectsExplicitOtherOwner_whenAuthenticatedUserDiffers(t *testing.T) {
	h, _, secret, _, _ := newHandler(t, []string{ScopeWriteBase, ScopeReadBase})
	body := putItemRequest{
		ItemID:    "base_item_cross_owner",
		OwnerID:   "other_owner",
		EventType: model.EventCreate,
		Kind:      model.KindFolder,
		Name:      "private",
		VersionID: "base_ver_folder_cross_owner",
	}
	b, _ := json.Marshal(body)

	rr := do(t, h.Routes(), http.MethodPost, "/api/base/items", secret, bytes.NewReader(b))

	if rr.Code != http.StatusForbidden {
		t.Fatalf("status: got %d want 403 body: %s", rr.Code, rr.Body.String())
	}
}

func TestPutItemRejectsMissingBlob_whenFileReferencesUnknownBlob(t *testing.T) {
	h, _, secret, _, _ := newHandler(t, []string{ScopeWriteBase, ScopeReadBase})
	body := putItemRequest{
		ItemID:      "base_item_missing_blob",
		EventType:   model.EventCreate,
		Kind:        model.KindFile,
		Name:        "notes.txt",
		VersionID:   "base_ver_missing_blob",
		BlobRef:     model.BlobRef("sha256:" + strings.Repeat("d", 64)),
		ContentHash: strings.Repeat("d", 64),
	}
	b, _ := json.Marshal(body)

	rr := do(t, h.Routes(), http.MethodPost, "/api/base/items", secret, bytes.NewReader(b))

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d want 400 body: %s", rr.Code, rr.Body.String())
	}
}

func TestPutItemRejectsInvalidBlobRef_whenFileHasMalformedBlobRef(t *testing.T) {
	h, _, secret, _, _ := newHandler(t, []string{ScopeWriteBase, ScopeReadBase})
	body := putItemRequest{
		ItemID:    "base_item_invalid_blob",
		EventType: model.EventCreate,
		Kind:      model.KindFile,
		Name:      "notes.txt",
		VersionID: "base_ver_invalid_blob",
		BlobRef:   "sha256:not-valid",
	}
	b, _ := json.Marshal(body)

	rr := do(t, h.Routes(), http.MethodPost, "/api/base/items", secret, bytes.NewReader(b))

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d want 400 body: %s", rr.Code, rr.Body.String())
	}
}

func TestPutItemAndDelta(t *testing.T) {
	h, _, secret, _, jr := newHandler(t, []string{ScopeWriteBase, ScopeReadBase})
	ref, err := h.blobs.Put([]byte("api item body"))
	if err != nil {
		t.Fatalf("put blob: %v", err)
	}
	itemID := model.ItemID("base_item_" + "abc123")
	body := putItemRequest{
		ItemID:    itemID,
		EventType: model.EventCreate,
		Kind:      model.KindFile,
		Name:      "notes.txt",
		VersionID: model.VersionID("base_ver_v1"),
		BlobRef:   ref,
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
	if entries[0].Event.OwnerID != "user_test" {
		t.Fatalf("owner: got %q want user_test", entries[0].Event.OwnerID)
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

func TestDeltaDoesNotExposeOtherOwnerEvents_whenJournalContainsForeignOwner(t *testing.T) {
	h, _, secret, _, jr := newHandler(t, []string{ScopeWriteBase, ScopeReadBase})
	_, err := jr.Append(model.Event{
		EventID:   "base_evt_foreign_owner",
		OwnerID:   "other_owner",
		ItemID:    "base_item_foreign_owner",
		DeviceID:  "dev1",
		SubjectID: "other_owner",
		EventType: model.EventCreate,
		Kind:      model.KindFolder,
		PayloadJSON: tree.Payload{
			Name:      "foreign",
			Kind:      model.KindFolder,
			VersionID: "base_ver_foreign_owner",
		}.JSON(),
		CreatedAt: time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("append foreign event: %v", err)
	}

	rr := do(t, h.Routes(), http.MethodGet, "/api/base/delta?cursor=0", secret, nil)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d want 200 body: %s", rr.Code, rr.Body.String())
	}
	var resp deltaResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Events) != 0 {
		t.Fatalf("events: got %d want 0", len(resp.Events))
	}
	if resp.Head != 0 {
		t.Fatalf("head: got %d want 0", resp.Head)
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

func TestGetItemDoesNotExposeOtherOwnerItem_whenItemIDExistsForForeignOwner(t *testing.T) {
	h, _, secret, _, jr := newHandler(t, []string{ScopeReadBase})
	_, err := jr.Append(model.Event{
		EventID:   "base_evt_foreign_get",
		OwnerID:   "other_owner",
		ItemID:    "base_item_foreign_get",
		DeviceID:  "dev1",
		SubjectID: "other_owner",
		EventType: model.EventCreate,
		Kind:      model.KindFolder,
		PayloadJSON: tree.Payload{
			Name:      "foreign",
			Kind:      model.KindFolder,
			VersionID: "base_ver_foreign_get",
		}.JSON(),
		CreatedAt: time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("append foreign event: %v", err)
	}

	rr := do(t, h.Routes(), http.MethodGet, "/api/base/items/base_item_foreign_get", secret, nil)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status: got %d want 404 body: %s", rr.Code, rr.Body.String())
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
	ref, err := h.blobs.Put([]byte("status body"))
	if err != nil {
		t.Fatalf("put blob: %v", err)
	}
	itemID := model.ItemID("base_item_status")
	body := putItemRequest{
		ItemID:    itemID,
		EventType: model.EventCreate,
		Kind:      model.KindFile,
		Name:      "file.txt",
		VersionID: model.VersionID("base_ver_s1"),
		BlobRef:   ref,
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
