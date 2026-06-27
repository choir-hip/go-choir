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
	workspacePath, dbName, err := runtimeObjectGraphDoltWorkspace(cfg, s)
	if err != nil {
		return nil, err
	}
	doltStore, err := objectgraph.OpenDoltStore(workspacePath, dbName)
	if err != nil {
		return nil, err
	}
	return objectgraph.NewService(objectgraph.Config{
		Memory:  objectgraph.NewMemoryStore(),
		Durable: doltStore,
	}), nil
}

func runtimeObjectGraphDoltWorkspace(cfg Config, s *store.Store) (workspacePath, dbName string, err error) {
	base := ""
	if s != nil {
		base = strings.TrimSpace(s.Path())
	}
	if base == "" {
		base = strings.TrimSpace(cfg.StorePath)
	}
	if base == "" {
		return "", "", fmt.Errorf("runtime store path is required")
	}
	dir := filepath.Dir(base)
	if dir == "" || dir == "." {
		dir = "/tmp/go-choir-m3"
	}
	return filepath.Join(dir, "objectgraph-dolt"), "objectgraph", nil
}
