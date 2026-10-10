package researchtools

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/yusefmosiah/go-choir/internal/toolregistry"
)

// HostEgress is the desk cell carrier's host-mediated research boundary (R3r):
// a yaegi cell calls choir.<Verb>, the call crosses the session socket as a
// StreamBrokerEgress frame keyed by the tool's action name, and the host
// resolves it here against the same deps-bound tool the registry holds — so
// egress charging, source/service resolution, and the bounded projection are
// identical to the tool path. The cell never holds an open socket — it sees
// a bounded result string or a budget/unavailable error.
//
// Each deps set builds its tool table once; the funcs are the registry's own
// closures, so there is one source of truth for behavior and one ledger.
func (d *Dependencies) HostEgress(ctx context.Context, action string, payload json.RawMessage) (json.RawMessage, error) {
	fn, err := d.hostEgressTool(action)
	if err != nil {
		return nil, err
	}
	out, err := fn(ctx, payload)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(out), nil
}

// hostEgressTool resolves the deps-bound ToolFunc for one broker action. The
// action string is the tool name (web_search, source_search, fetch_url,
// import_*, read_*, list_*, search_wire_corpus, evidence/run-memory verbs).
func (d *Dependencies) hostEgressTool(action string) (toolregistry.ToolFunc, error) {
	table := d.egressToolTable()
	fn, ok := table[action]
	if !ok {
		return nil, fmt.Errorf("host egress: unsupported action %q", action)
	}
	return fn, nil
}

var (
	egressTableMu sync.Mutex
	egressTables  = map[*Dependencies]map[string]toolregistry.ToolFunc{}
)

// egressToolTable lazily builds the deps-bound network/content/evidence tool
// funcs once per Dependencies, keyed by tool name. Reusing the real tool
// constructors keeps the egress budget, service resolution, and output
// projection identical between the tool and cell-verb surfaces.
func (d *Dependencies) egressToolTable() map[string]toolregistry.ToolFunc {
	egressTableMu.Lock()
	defer egressTableMu.Unlock()
	// Keyed on the runtime's own *Dependencies, so the table is built once
	// per deps set (a value receiver's address was fresh on every call, so
	// the cache never hit and grew per call).
	key := d
	if t, ok := egressTables[key]; ok {
		return t
	}
	t := map[string]toolregistry.ToolFunc{}
	// Network verbs (egress-metered): search + fetch + source.
	for _, tool := range []toolregistry.Tool{
		newWebSearchTool(d.Search, *d),
		newFetchURLTool(d.HTTP, *d),
		newSourceSearchTool(d.Source, *d),
		newImportDocumentContentTool(*d),
		newImportURLContentTool(*d),
		newReadContentItemTool(*d),
		newListContentItemSelectorsTool(*d),
		newReadContentItemSelectorTool(*d),
		newSearchWireCorpusTool(*d),
	} {
		if tool.Func == nil {
			continue
		}
		t[tool.Name] = tool.Func
	}
	egressTables[key] = t
	return t
}
