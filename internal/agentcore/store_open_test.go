package agentcore

import "github.com/yusefmosiah/go-choir/internal/store"

// openTestStore opens a store for a test. The SQLite state file is
// <dir>/state.db (store.StateDBPath), so each test passes a path in its own
// directory (t.TempDir).
func openTestStore(dbPath string) (*store.Store, error) {
	return store.Open(dbPath)
}
