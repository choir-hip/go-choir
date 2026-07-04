package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/auth"
	"github.com/yusefmosiah/go-choir/internal/base/api"
	"github.com/yusefmosiah/go-choir/internal/base/model"
	"github.com/yusefmosiah/go-choir/internal/server"
)

const (
	defaultPortEnv    = "BASE_API_HARNESS_PORT"
	journalPathEnv    = "BASE_API_JOURNAL_PATH"
	blobRootEnv       = "BASE_API_BLOB_ROOT"
	authDBPathEnv     = "BASE_API_AUTH_DB_PATH"
	defaultListenPort = "8795"
)

type config struct {
	port        string
	journalPath string
	blobRoot    string
	authDBPath  string
	seedFixture bool
}

type configuredServer struct {
	server      *server.Server
	authStore   *auth.Store
	persistent  *api.PersistentHandler
	journalPath string
	blobRoot    string
	authDBPath  string
}

type fixtureOutput struct {
	JournalPath string        `json:"journal_path"`
	BlobRoot    string        `json:"blob_root"`
	AuthDBPath  string        `json:"auth_db_path"`
	ItemID      model.ItemID  `json:"item_id"`
	BlobRef     model.BlobRef `json:"blob_ref"`
	SHA256      string        `json:"sha256"`
}

type fixtureBlobResponse struct {
	BlobRef model.BlobRef `json:"blob_ref"`
	SHA256  string        `json:"sha256"`
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	cfg, err := parseConfig(args, stderr)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	configured, err := openConfiguredServer(cfg)
	if err != nil {
		fmt.Fprintf(stderr, "baseharness: %v\n", err)
		return 1
	}
	defer func() {
		if err := configured.Close(); err != nil {
			log.Printf("baseharness: close: %v", err)
		}
	}()
	if cfg.seedFixture {
		fixture, err := configured.SeedFixture()
		if err != nil {
			fmt.Fprintf(stderr, "baseharness: seed fixture: %v\n", err)
			return 1
		}
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(fixture); err != nil {
			fmt.Fprintf(stderr, "baseharness: encode fixture: %v\n", err)
			return 1
		}
		return 0
	}
	fmt.Fprintf(stdout, "baseharness: serving /api/base/ with journal=%s blob_root=%s auth_db=%s\n", cfg.journalPath, cfg.blobRoot, cfg.authDBPath)
	configured.server.Start()
	return 0
}

func parseConfig(args []string, stderr io.Writer) (config, error) {
	fs := flag.NewFlagSet("baseharness", flag.ContinueOnError)
	fs.SetOutput(stderr)
	cfg := config{}
	fs.StringVar(&cfg.port, "port", envOr(defaultPortEnv, defaultListenPort), "local listen port")
	fs.StringVar(&cfg.journalPath, "journal", os.Getenv(journalPathEnv), "Base SQLite journal path")
	fs.StringVar(&cfg.blobRoot, "blob-root", os.Getenv(blobRootEnv), "Base blob store root")
	fs.StringVar(&cfg.authDBPath, "auth-db", os.Getenv(authDBPathEnv), "auth SQLite database path used for API keys")
	fs.BoolVar(&cfg.seedFixture, "seed-fixture", false, "create one local Base blob/item fixture and exit without listening")
	if err := fs.Parse(args); err != nil {
		return config{}, err
	}
	if err := cfg.validate(); err != nil {
		return config{}, err
	}
	return cfg, nil
}

func (cfg config) validate() error {
	if strings.TrimSpace(cfg.journalPath) == "" {
		return fmt.Errorf("baseharness: --journal or %s is required", journalPathEnv)
	}
	if strings.TrimSpace(cfg.blobRoot) == "" {
		return fmt.Errorf("baseharness: --blob-root or %s is required", blobRootEnv)
	}
	if strings.TrimSpace(cfg.authDBPath) == "" {
		return fmt.Errorf("baseharness: --auth-db or %s is required", authDBPathEnv)
	}
	if strings.TrimSpace(cfg.port) == "" {
		return fmt.Errorf("baseharness: --port or %s is required", defaultPortEnv)
	}
	return nil
}

func openConfiguredServer(cfg config) (*configuredServer, error) {
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	authStore, err := auth.OpenStore(cfg.authDBPath)
	if err != nil {
		return nil, fmt.Errorf("open auth store: %w", err)
	}
	persistent, err := api.OpenPersistentHandler(api.PersistentHandlerConfig{
		JournalPath: cfg.journalPath,
		BlobRoot:    cfg.blobRoot,
	}, authStore)
	if err != nil {
		_ = authStore.Close()
		return nil, err
	}
	srv := server.NewServer("baseharness", cfg.port)
	if err := api.RegisterPersistentRoutes(srv, persistent); err != nil {
		_ = persistent.Close()
		_ = authStore.Close()
		return nil, err
	}
	return &configuredServer{server: srv, authStore: authStore, persistent: persistent, journalPath: cfg.journalPath, blobRoot: cfg.blobRoot, authDBPath: cfg.authDBPath}, nil
}

func (s *configuredServer) SeedFixture() (fixtureOutput, error) {
	user, err := s.authStore.CreateUser("baseharness-fixture-user", "baseharness-fixture@example.com")
	if err != nil {
		return fixtureOutput{}, fmt.Errorf("create fixture user: %w", err)
	}
	_, secret, err := s.authStore.CreateAPIKey(context.Background(), user.ID, "baseharness-fixture", []string{api.ScopeReadBase, api.ScopeWriteBase}, nil)
	if err != nil {
		return fixtureOutput{}, fmt.Errorf("create fixture api key: %w", err)
	}

	blobResp, err := s.putFixtureBlob(secret, []byte("baseharness fixture bytes"))
	if err != nil {
		return fixtureOutput{}, err
	}
	itemID := model.ItemID("base_item_harness_fixture")
	itemBody, err := json.Marshal(map[string]any{
		"item_id":        itemID,
		"event_type":     model.EventCreate,
		"kind":           model.KindFile,
		"name":           "baseharness-fixture.txt",
		"parent_item_id": "base_item_root",
		"version_id":     "base_ver_harness_fixture",
		"blob_ref":       blobResp.BlobRef,
		"content_hash":   blobResp.SHA256,
		"media_type":     "text/plain",
	})
	if err != nil {
		return fixtureOutput{}, fmt.Errorf("encode fixture item: %w", err)
	}
	if err := s.doFixtureRequest(http.MethodPost, "/api/base/items", secret, bytes.NewReader(itemBody), nil); err != nil {
		return fixtureOutput{}, err
	}
	return fixtureOutput{
		JournalPath: s.journalPath,
		BlobRoot:    s.blobRoot,
		AuthDBPath:  s.authDBPath,
		ItemID:      itemID,
		BlobRef:     blobResp.BlobRef,
		SHA256:      blobResp.SHA256,
	}, nil
}

func (s *configuredServer) putFixtureBlob(secret string, data []byte) (fixtureBlobResponse, error) {
	var resp fixtureBlobResponse
	if err := s.doFixtureRequest(http.MethodPost, "/api/base/blobs", secret, bytes.NewReader(data), &resp); err != nil {
		return fixtureBlobResponse{}, err
	}
	if resp.BlobRef == "" || resp.SHA256 == "" {
		return fixtureBlobResponse{}, fmt.Errorf("incomplete fixture blob response: %#v", resp)
	}
	return resp, nil
}

func (s *configuredServer) doFixtureRequest(method, path, secret string, body *bytes.Reader, out any) error {
	req := httptest.NewRequest(method, path, body)
	req.Header.Set("Authorization", "Bearer "+secret)
	if method == http.MethodPost && strings.HasSuffix(path, "/items") {
		req.Header.Set("Content-Type", "application/json")
	}
	rr := httptest.NewRecorder()
	s.server.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		return fmt.Errorf("%s %s status %d: %s", method, path, rr.Code, rr.Body.String())
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(rr.Body.Bytes(), out); err != nil {
		return fmt.Errorf("decode %s %s response: %w", method, path, err)
	}
	return nil
}

func (s *configuredServer) Close() error {
	if s == nil {
		return nil
	}
	return errors.Join(s.persistent.Close(), s.authStore.Close())
}

func envOr(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
