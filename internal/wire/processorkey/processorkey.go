// Package processorkey provides the shared processor-key derivation logic used
// by both cycle (ingestion handoff) and runtime (universal wire dispatch). It
// lives in a leaf package to break the import cycle cycle -> provider -> runtime.
package processorkey

import (
	"github.com/yusefmosiah/go-choir/internal/sources"
)

// StableRequestID produces a content-hash-derived stable request identifier
// from the kind, cycle ID, and additional parts.
func StableRequestID(kind, cycleID string, parts ...string) string {
	segments := append([]string{kind, cycleID}, parts...)
	return kind + "_" + sources.ContentHash(segments...)[:24]
}
