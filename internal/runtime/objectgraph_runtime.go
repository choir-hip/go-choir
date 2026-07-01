package runtime

import (
	"fmt"
	"log"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

// ObjectGraph returns the runtime-owned durable objectgraph service. It is
// initialized lazily so tests and lightweight runtimes that never touch graph
// APIs do not open another database handle.
//
// In production the durable store is an HTTPStore that queries corpusd (the
// platform Dolt SQL server) through corpusd. Tests may inject an in-memory
// store via Config.ObjectGraphStore to avoid requiring a running corpusd.
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
	svc, err := newRuntimeObjectGraphService(rt.cfg)
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

// newRuntimeObjectGraphService builds the runtime objectgraph Service. The
// durable store defaults to an HTTPStore backed by corpusd (via the proxy)
// so object graph data persists in corpusd. When cfg.ObjectGraphStore is set
// (a test seam), it is used directly instead of constructing an HTTPStore.
//
// URL precedence: WirePublishURL is preferred because it is always set
// correctly in deployed VMs (derived from vmctl_url). CorpusdURL is used as
// a fallback (it defaults to http://127.0.0.1:8082 for local dev). The proxy
// routes /internal/platform/objects and /internal/platform/edges to corpusd.
func newRuntimeObjectGraphService(cfg Config) (*objectgraph.Service, error) {
	if cfg.ObjectGraphStore != nil {
		return objectgraph.NewService(objectgraph.Config{
			Memory:  objectgraph.NewMemoryStore(),
			Durable: cfg.ObjectGraphStore,
		}), nil
	}
	// Prefer WirePublishURL — in VMs it's derived from vmctl_url and points
	// to the host proxy. CorpusdURL defaults to localhost for local dev.
	baseURL := strings.TrimSpace(cfg.WirePublishURL)
	if baseURL == "" {
		baseURL = strings.TrimSpace(cfg.CorpusdURL)
	}
	if baseURL == "" {
		return nil, fmt.Errorf("runtime: corpusd URL is required for objectgraph (set RUNTIME_CORPUSD_URL or RUNTIME_WIRE_PUBLISH_URL)")
	}
	httpStore := objectgraph.NewHTTPStore(baseURL)
	return objectgraph.NewService(objectgraph.Config{
		Memory:  objectgraph.NewMemoryStore(),
		Durable: httpStore,
	}), nil
}
