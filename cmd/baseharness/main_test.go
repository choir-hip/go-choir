package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/auth"
	"github.com/yusefmosiah/go-choir/internal/base/api"
	"github.com/yusefmosiah/go-choir/internal/base/model"
	"github.com/yusefmosiah/go-choir/internal/computerversion"
)

type blobResponse struct {
	BlobRef model.BlobRef `json:"blob_ref"`
	SHA256  string        `json:"sha256"`
}

func TestOpenConfiguredServerFeedsReadOnlyObservationWithRealAuthStore(t *testing.T) {
	root := t.TempDir()
	cfg := config{
		port:        "0",
		journalPath: filepath.Join(root, "base.sqlite"),
		blobRoot:    filepath.Join(root, "blobs"),
		authDBPath:  filepath.Join(root, "auth.sqlite"),
	}
	authStore, err := auth.OpenStore(cfg.authDBPath)
	if err != nil {
		t.Fatalf("open auth store: %v", err)
	}
	user, err := authStore.CreateUser("baseharness-user", "baseharness@example.com")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	_, secret, err := authStore.CreateAPIKey(context.Background(), user.ID, "baseharness", []string{api.ScopeReadBase, api.ScopeWriteBase}, nil)
	if err != nil {
		t.Fatalf("create api key: %v", err)
	}
	if err := authStore.Close(); err != nil {
		t.Fatalf("close setup auth store: %v", err)
	}

	configured, err := openConfiguredServer(cfg)
	if err != nil {
		t.Fatalf("open configured server: %v", err)
	}

	blobResp := putBlob(t, configured, secret, []byte("base harness observed bytes"))
	itemID := model.ItemID("base_item_harness_observation")
	itemBody, _ := json.Marshal(map[string]any{
		"item_id":        itemID,
		"event_type":     model.EventCreate,
		"kind":           model.KindFile,
		"name":           "harness.txt",
		"parent_item_id": "base_item_root",
		"version_id":     "base_ver_harness_observation",
		"blob_ref":       blobResp.BlobRef,
		"content_hash":   blobResp.SHA256,
		"media_type":     "text/plain",
	})
	rr := doRequest(t, configured, http.MethodPost, "/api/base/items", secret, bytes.NewReader(itemBody))
	if rr.Code != http.StatusOK {
		t.Fatalf("put item status: %d body: %s", rr.Code, rr.Body.String())
	}

	if err := configured.Close(); err != nil {
		t.Fatalf("close configured server: %v", err)
	}

	source, err := computerversion.OpenBaseCurrentStateSource(computerversion.BaseCurrentStatePaths{
		JournalPath: cfg.journalPath,
		BlobRoot:    cfg.blobRoot,
	})
	if err != nil {
		t.Fatalf("open current-state source: %v", err)
	}
	defer source.Close()
	observationSet, err := source.ObservationSet(context.Background(), "baseharness", computerversion.ComputerVersion{
		CodeRef:            "test-code-ref",
		ArtifactProgramRef: computerversion.ArtifactProgramRef("base-sqlite:" + cfg.journalPath),
	})
	if err != nil {
		t.Fatalf("load observation set: %v", err)
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
		t.Fatalf("missing harness observations: sawItem=%v sawBlob=%v set=%#v", sawItem, sawBlob, observationSet.Observations)
	}
}

func TestRunSeedFixtureExitsAndPersistsObservableFixture(t *testing.T) {
	root := t.TempDir()
	journalPath := filepath.Join(root, "seed.sqlite")
	blobRoot := filepath.Join(root, "blobs")
	authDBPath := filepath.Join(root, "auth.sqlite")
	var stdout, stderr bytes.Buffer

	code := run([]string{
		"--seed-fixture",
		"--journal", journalPath,
		"--blob-root", blobRoot,
		"--auth-db", authDBPath,
		"--port", "0",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run exit = %d, want 0; stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}

	var fixture fixtureOutput
	if err := json.Unmarshal(stdout.Bytes(), &fixture); err != nil {
		t.Fatalf("decode fixture output: %v; stdout=%s", err, stdout.String())
	}
	if fixture.JournalPath != journalPath {
		t.Fatalf("journal_path = %q, want %q", fixture.JournalPath, journalPath)
	}
	if fixture.BlobRoot != blobRoot {
		t.Fatalf("blob_root = %q, want %q", fixture.BlobRoot, blobRoot)
	}
	if fixture.AuthDBPath != authDBPath {
		t.Fatalf("auth_db_path = %q, want %q", fixture.AuthDBPath, authDBPath)
	}
	if fixture.ItemID == "" {
		t.Fatal("fixture item_id is empty")
	}
	if fixture.BlobRef == "" {
		t.Fatal("fixture blob_ref is empty")
	}
	if fixture.SHA256 == "" {
		t.Fatal("fixture sha256 is empty")
	}

	source, err := computerversion.OpenBaseCurrentStateSource(computerversion.BaseCurrentStatePaths{
		JournalPath: fixture.JournalPath,
		BlobRoot:    fixture.BlobRoot,
	})
	if err != nil {
		t.Fatalf("open current-state source: %v", err)
	}
	defer source.Close()
	observationSet, err := source.ObservationSet(context.Background(), "baseharness-seed-fixture", computerversion.ComputerVersion{
		CodeRef:            "test-code-ref",
		ArtifactProgramRef: computerversion.ArtifactProgramRef("base-sqlite:" + fixture.JournalPath),
	})
	if err != nil {
		t.Fatalf("load observation set: %v", err)
	}

	assertSeedFixtureObservations(t, observationSet, fixture)
}

func TestRunSeedFixtureRejectsMissingExplicitStatePaths(t *testing.T) {
	clearBaseHarnessStateEnv(t)
	root := t.TempDir()
	tests := []struct {
		name     string
		args     []string
		wantText string
	}{
		{
			name: "journal",
			args: []string{
				"--seed-fixture",
				"--blob-root", filepath.Join(root, "missing-journal-blobs"),
				"--auth-db", filepath.Join(root, "missing-journal-auth.sqlite"),
				"--port", "0",
			},
			wantText: "--journal",
		},
		{
			name: "blob root",
			args: []string{
				"--seed-fixture",
				"--journal", filepath.Join(root, "missing-blob.sqlite"),
				"--auth-db", filepath.Join(root, "missing-blob-auth.sqlite"),
				"--port", "0",
			},
			wantText: "--blob-root",
		},
		{
			name: "auth db",
			args: []string{
				"--seed-fixture",
				"--journal", filepath.Join(root, "missing-auth.sqlite"),
				"--blob-root", filepath.Join(root, "missing-auth-blobs"),
				"--port", "0",
			},
			wantText: "--auth-db",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run(tt.args, &stdout, &stderr)
			if code != 2 {
				t.Fatalf("run exit = %d, want 2; stdout=%s stderr=%s", code, stdout.String(), stderr.String())
			}
			if stdout.Len() != 0 {
				t.Fatalf("stdout = %q, want empty", stdout.String())
			}
			if !strings.Contains(stderr.String(), tt.wantText) {
				t.Fatalf("stderr = %q, want %q", stderr.String(), tt.wantText)
			}
		})
	}
}

func TestParseConfigRequiresExplicitStatePaths(t *testing.T) {
	clearBaseHarnessStateEnv(t)
	_, err := parseConfig(nil, &bytes.Buffer{})
	if err == nil {
		t.Fatal("expected missing paths to fail")
	}
	if !strings.Contains(err.Error(), "--journal") {
		t.Fatalf("error = %q, want missing journal", err.Error())
	}
}

func putBlob(t *testing.T, configured *configuredServer, secret string, data []byte) blobResponse {
	t.Helper()
	rr := doRequest(t, configured, http.MethodPost, "/api/base/blobs", secret, bytes.NewReader(data))
	if rr.Code != http.StatusOK {
		t.Fatalf("put blob status: %d body: %s", rr.Code, rr.Body.String())
	}
	var resp blobResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode blob response: %v", err)
	}
	if resp.BlobRef == "" || resp.SHA256 == "" {
		t.Fatalf("incomplete blob response: %#v", resp)
	}
	return resp
}

func doRequest(t *testing.T, configured *configuredServer, method, path, secret string, body *bytes.Reader) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Reader
	if body == nil {
		reader = bytes.NewReader(nil)
	} else {
		reader = body
	}
	req := httptest.NewRequest(method, path, reader)
	if secret != "" {
		req.Header.Set("Authorization", "Bearer "+secret)
	}
	if method == http.MethodPost && strings.HasSuffix(path, "/items") {
		req.Header.Set("Content-Type", "application/json")
	}
	rr := httptest.NewRecorder()
	configured.server.ServeHTTP(rr, req)
	return rr
}

func assertSeedFixtureObservations(t *testing.T, observationSet computerversion.ObservationSet, fixture fixtureOutput) {
	t.Helper()
	var sawItem, sawBlob bool
	for _, observation := range observationSet.Observations {
		switch {
		case observation.Kind == computerversion.ObservationFileManifest && observation.Key == string(fixture.ItemID):
			sawItem = strings.Contains(observation.Value, string(fixture.BlobRef)) && strings.Contains(observation.Value, fixture.SHA256)
		case observation.Kind == computerversion.ObservationBlobSet && observation.Key == string(fixture.BlobRef):
			sawBlob = strings.Contains(observation.Value, fixture.SHA256)
		}
	}
	if !sawItem || !sawBlob {
		t.Fatalf("missing seed fixture observations: sawItem=%v sawBlob=%v set=%#v", sawItem, sawBlob, observationSet.Observations)
	}
}

func clearBaseHarnessStateEnv(t *testing.T) {
	t.Helper()
	t.Setenv(journalPathEnv, "")
	t.Setenv(blobRootEnv, "")
	t.Setenv(authDBPathEnv, "")
}

func TestOpenConfiguredServerRejectsMissingAuthDBPath(t *testing.T) {
	root := t.TempDir()
	_, err := openConfiguredServer(config{
		port:        "0",
		journalPath: filepath.Join(root, "base.sqlite"),
		blobRoot:    filepath.Join(root, "blobs"),
	})
	if err == nil {
		t.Fatal("expected missing auth db path to fail")
	}
	if !strings.Contains(err.Error(), "--auth-db") {
		t.Fatalf("error = %v, want auth-db validation failure", err)
	}
}
