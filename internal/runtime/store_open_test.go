package runtime

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/yusefmosiah/go-choir/internal/store"
)

var (
	testStoreTemplateOnce sync.Once
	testStoreTemplatePath string
	testStoreTemplateErr  error
)

// openTestStore is a drop-in replacement for store.Open in tests. Fresh store
// bootstrap (embedded Dolt workspace creation plus schema DDL) costs over a
// second per call; opening a copy of an already-bootstrapped workspace costs a
// third of that. The first call bootstraps a template workspace once per test
// process, and later calls clone it for paths that do not exist yet. Paths
// that already exist are reopened through store.Open unchanged, so tests that
// exercise reopen semantics keep their behavior.
func openTestStore(dbPath string) (*store.Store, error) {
	if _, err := os.Stat(dbPath); err == nil {
		return store.Open(dbPath)
	}

	testStoreTemplateOnce.Do(func() {
		dir, err := os.MkdirTemp("", "go-choir-test-store-template")
		if err != nil {
			testStoreTemplateErr = fmt.Errorf("create template dir: %w", err)
			return
		}
		templateDB := filepath.Join(dir, "template.db")
		s, err := store.Open(templateDB)
		if err != nil {
			testStoreTemplateErr = fmt.Errorf("bootstrap template store: %w", err)
			return
		}
		if err := s.Close(); err != nil {
			testStoreTemplateErr = fmt.Errorf("close template store: %w", err)
			return
		}
		testStoreTemplatePath = templateDB
	})
	if testStoreTemplateErr != nil {
		return nil, testStoreTemplateErr
	}

	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, fmt.Errorf("create test store directory: %w", err)
	}
	srcWorkspace := testStoreWorkspacePath(testStoreTemplatePath)
	dstWorkspace := testStoreWorkspacePath(dbPath)
	if err := os.RemoveAll(dstWorkspace); err != nil {
		return nil, fmt.Errorf("clear stale test workspace: %w", err)
	}
	if copyErr := os.CopyFS(dstWorkspace, os.DirFS(srcWorkspace)); copyErr != nil {
		_ = os.RemoveAll(dstWorkspace)
		return store.Open(dbPath)
	}
	if err := os.WriteFile(dbPath, nil, 0o644); err != nil {
		return nil, fmt.Errorf("write test store marker: %w", err)
	}
	return store.Open(dbPath)
}

// testStoreWorkspacePath mirrors the store package's private workspace path
// derivation: the db path with its extension replaced by ".vtext".
func testStoreWorkspacePath(path string) string {
	trimmed := strings.TrimSuffix(path, filepath.Ext(path))
	if trimmed == "" {
		trimmed = path
	}
	return trimmed + ".vtext"
}
