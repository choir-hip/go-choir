package runtime

import (
	"fmt"
	"log"
	"path/filepath"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/store"
)

// ObjectGraph returns the runtime-owned durable objectgraph service. It is
// initialized lazily so tests and lightweight runtimes that never touch graph
// APIs do not open another database handle.
func (rt *Runtime) ObjectGraph() *objectgraph.Service {
	if rt == nil {
		return nil
	}
	rt.objectGraphMu.Lock()
	defer rt.objectGraphMu.Unlock()
	if rt.objectGraph != nil {
		return rt.objectGraph
	}
	if rt.objectGraphInitErr != nil {
		return nil
	}
	svc, err := newRuntimeObjectGraphService(rt.cfg, rt.store)
	if err != nil {
		rt.objectGraphInitErr = err
		log.Printf("runtime: objectgraph unavailable: %v", err)
		return nil
	}
	rt.objectGraph = svc
	return rt.objectGraph
}

func (rt *Runtime) closeObjectGraph() {
	if rt == nil {
		return
	}
	rt.objectGraphMu.Lock()
	defer rt.objectGraphMu.Unlock()
	if rt.objectGraph == nil {
		return
	}
	if err := rt.objectGraph.Close(); err != nil {
		log.Printf("runtime: close objectgraph: %v", err)
	}
	rt.objectGraph = nil
}

func newRuntimeObjectGraphService(cfg Config, s *store.Store) (*objectgraph.Service, error) {
	path, err := runtimeObjectGraphPath(cfg, s)
	if err != nil {
		return nil, err
	}
	sqliteStore, err := objectgraph.NewSQLiteStore(path)
	if err != nil {
		return nil, err
	}
	return objectgraph.NewService(objectgraph.Config{
		Memory: objectgraph.NewMemoryStore(),
		SQLite: sqliteStore,
	}), nil
}

func runtimeObjectGraphPath(cfg Config, s *store.Store) (string, error) {
	base := ""
	if s != nil {
		base = strings.TrimSpace(s.Path())
	}
	if base == "" {
		base = strings.TrimSpace(cfg.StorePath)
	}
	if base == "" {
		return "", fmt.Errorf("runtime store path is required")
	}
	if base == ":memory:" {
		return ":memory:", nil
	}
	return filepath.Join(filepath.Dir(base), filepath.Base(base)+".objectgraph.db"), nil
}
