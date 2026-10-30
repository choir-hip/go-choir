package guestdolt

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/yusefmosiah/go-choir/internal/store"
)

// createStoreSchema opens the real store in dir (creating its SQLite state
// file and schema) and closes it, returning the state file's path. The
// sidecar directory the store keeps beside it is discarded with dir: the live
// sidecar is the legacy workspace directory itself and stays in place.
func createStoreSchema(dir string) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	marker := filepath.Join(dir, "store.db")
	st, err := store.Open(marker)
	if err != nil {
		return "", fmt.Errorf("store.Open: %w", err)
	}
	if err := st.Close(); err != nil {
		return "", fmt.Errorf("store.Close: %w", err)
	}
	return store.StateDBPath(marker), nil
}
