package cycle

import (
	"context"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/sourcegraph"
	"github.com/yusefmosiah/go-choir/internal/sources"
)

type WebCaptureGraphProjectionConfig = sourcegraph.WebCaptureGraphProjectionConfig
type WebCaptureGraphProjectionResult = sourcegraph.WebCaptureGraphProjectionResult

// WriteWebCaptureGraphObjects projects persisted sourcecycled source items into
// graph-native web captures. It does not create Texture publications or body
// source_ref citations; those remain downstream decisions.
func WriteWebCaptureGraphObjects(ctx context.Context, graph *objectgraph.Service, items []sources.Item, cfg WebCaptureGraphProjectionConfig) (WebCaptureGraphProjectionResult, error) {
	return sourcegraph.WriteWebCaptureGraphObjects(ctx, graph, items, cfg)
}
