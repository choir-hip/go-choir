package researchtools

import "github.com/yusefmosiah/go-choir/internal/toolregistry"

// Register installs the research tool constructors on a typed registry. No
// production registry carries them any more (the research desk reaches them
// through the egress table as choir.* cell verbs); the tests use it as a
// harness to drive each tool's Func by name.
func Register(registry *toolregistry.ToolRegistry, deps Dependencies) error {
	for _, tool := range []toolregistry.Tool{
		newSourceSearchTool(deps.Source, deps),
		newFetchURLTool(deps.HTTP, deps),
		newImportDocumentContentTool(deps),
		newImportURLContentTool(deps),
		newReadContentItemTool(deps),
		newListContentItemSelectorsTool(deps),
		newReadContentItemSelectorTool(deps),
		newSearchWireCorpusTool(deps),
	} {
		if err := registry.Register(tool); err != nil {
			return err
		}
	}
	return nil
}
