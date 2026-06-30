// Redaction middleware for the trace ingestion path.
//
// This file wires the PII redaction pipeline (internal/pii) into trace event
// persistence as a middleware stage: events → PII redaction → persistence.
// The regex redactor always runs (fast, deterministic); the SLM redactor is
// opt-in via RedactionConfig.EnableSLM and requires a running Ollama instance.
// Redaction failure stores the event envelope with a warning, but drops unsafe
// payload bytes before persistence.

package trace

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/yusefmosiah/go-choir/internal/pii"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// RedactionConfig controls the PII redaction pipeline attached to the trace
// ingestion path.
type RedactionConfig struct {
	// EnableSLM enables the optional SLM-based redactor (requires a running
	// Ollama instance). Defaults to false: only the deterministic regex
	// redactor runs, which is the v0 production path.
	EnableSLM bool

	// SLMModel is the Ollama model name used when EnableSLM is true (e.g.
	// "llama3.2:3b", "qwen2.5:7b"). Required when EnableSLM is true.
	SLMModel string

	// SLMBaseURL overrides the Ollama base URL (default
	// http://localhost:11434). Empty uses the SLM redactor default.
	SLMBaseURL string

	// SLMTimeout sets the per-request SLM timeout. Zero uses the SLM
	// redactor default (30s).
	SLMTimeout time.Duration
}

// NewPipelineFromConfig builds a PII redaction pipeline from the given config.
// The regex redactor always runs. When EnableSLM is true the SLM redactor is
// used as the primary strategy with the regex redactor as its fallback, so the
// deterministic path still covers every event when the model is unavailable.
func NewPipelineFromConfig(cfg RedactionConfig) *pii.Pipeline {
	if !cfg.EnableSLM {
		return pii.NewPipeline(pii.NewRegexRedactor())
	}
	opts := []pii.SLMOption{
		pii.WithSLMFallback(pii.NewRegexRedactor()),
	}
	if cfg.SLMBaseURL != "" {
		opts = append(opts, pii.WithSLMBaseURL(cfg.SLMBaseURL))
	}
	if cfg.SLMTimeout > 0 {
		opts = append(opts, pii.WithSLMTimeout(cfg.SLMTimeout))
	}
	return pii.NewPipeline(pii.NewSLMRedactor(cfg.SLMModel, opts...))
}

// RedactingStore wraps a Store and runs PII redaction on every Append before
// delegating to the underlying store. It implements the Store interface so it
// can be used as a drop-in middleware:
//
//	store, _ := trace.NewSQLiteStore(":memory:")
//	redacting := trace.NewRedactingStore(store, trace.NewPipelineFromConfig(cfg))
//
// Redaction is applied to the event payload at ingestion (before persistence).
// Structural envelope fields (ID, RunID, EventType, Actor, Tool, OwnerID,
// TrajectoryID, Seq, StreamSeq, CreatedAt) are not user content and are passed
// through unchanged — only the JSON Payload is scanned, because that is where
// LLM I/O, tool arguments, and message content live.
//
// Redaction failure does not block event storage: on error the event envelope is
// stored with a warning, but the unsafe payload is replaced before persistence.
type RedactingStore struct {
	inner    Store
	pipeline *pii.Pipeline
	logf     func(format string, args ...any)
}

// RedactingStoreOption configures a RedactingStore.
type RedactingStoreOption func(*RedactingStore)

// WithRedactionLogger sets the warning logger used when redaction fails. The
// default is the standard library log package. The logger receives format
// strings with redactor diagnostics (no raw PII is ever logged — the pipeline
// strips Finding.Match before exposing reports).
func WithRedactionLogger(logf func(format string, args ...any)) RedactingStoreOption {
	return func(r *RedactingStore) { r.logf = logf }
}

// NewRedactingStore returns a RedactingStore that wraps inner and redacts
// every appended event through pipeline before persistence.
func NewRedactingStore(inner Store, pipeline *pii.Pipeline, opts ...RedactingStoreOption) *RedactingStore {
	if inner == nil {
		panic("trace: NewRedactingStore: nil inner store")
	}
	if pipeline == nil {
		panic("trace: NewRedactingStore: nil pipeline")
	}
	r := &RedactingStore{
		inner:    inner,
		pipeline: pipeline,
		logf:     log.Printf,
	}
	for _, o := range opts {
		o(r)
	}
	return r
}

// Pipeline returns the underlying PII redaction pipeline.
func (r *RedactingStore) Pipeline() *pii.Pipeline { return r.pipeline }

// Append redacts the event payload through the PII pipeline, then persists the
// redacted event via the inner store. The caller's *Event is not mutated: a
// shallow copy is redacted and stored. On redaction error the event envelope is
// stored with a failure marker payload so observability is preserved without
// persisting unredacted bytes.
func (r *RedactingStore) Append(ctx context.Context, e *Event) error {
	if e == nil {
		return r.inner.Append(ctx, e)
	}

	redacted := *e // shallow copy; payload replaced below

	if len(e.Payload) > 0 {
		rec := types.EventRecord{
			EventID: e.ID,
			Payload: e.Payload,
		}
		cleaned, report, err := r.pipeline.RedactEvent(rec)
		if err != nil {
			redacted.Payload = redactionFailurePayload()
			r.logf("trace redaction: failed to redact event %q (redactor=%s); persisted payload dropped",
				e.ID, r.pipeline.Redactor().Name())
		} else {
			redacted.Payload = cleaned.Payload
			if report.Changed {
				r.logf("trace redaction: redacted event %q (redactor=%s, findings=%d, fields=%d)",
					e.ID, report.Redactor, report.FindingsCount, report.FieldsRedacted)
			}
		}
	}

	return r.inner.Append(ctx, &redacted)
}

func redactionFailurePayload() json.RawMessage {
	payload, err := json.Marshal(map[string]string{"redaction_error": "payload_dropped"})
	if err != nil {
		return json.RawMessage(`{"redaction_error":"payload_dropped"}`)
	}
	return payload
}

// Get delegates to the inner store.
func (r *RedactingStore) Get(ctx context.Context, id string) (*Event, error) {
	return r.inner.Get(ctx, id)
}

// GetForOwner delegates to the inner store.
func (r *RedactingStore) GetForOwner(ctx context.Context, ownerID, id string) (*Event, error) {
	return r.inner.GetForOwner(ctx, ownerID, id)
}

// ListByRun delegates to the inner store.
func (r *RedactingStore) ListByRun(ctx context.Context, runID string, limit int) ([]Event, error) {
	return r.inner.ListByRun(ctx, runID, limit)
}

// ListByRunForOwner delegates to the inner store.
func (r *RedactingStore) ListByRunForOwner(ctx context.Context, ownerID, runID string, limit int) ([]Event, error) {
	return r.inner.ListByRunForOwner(ctx, ownerID, runID, limit)
}

// ListByOwner delegates to the inner store.
func (r *RedactingStore) ListByOwner(ctx context.Context, ownerID string, limit int) ([]Event, error) {
	return r.inner.ListByOwner(ctx, ownerID, limit)
}

// ListByTrajectory delegates to the inner store.
func (r *RedactingStore) ListByTrajectory(ctx context.Context, ownerID, trajectoryID string, limit int) ([]Event, error) {
	return r.inner.ListByTrajectory(ctx, ownerID, trajectoryID, limit)
}

// Close delegates to the inner store.
func (r *RedactingStore) Close() error { return r.inner.Close() }

// Compile-time assertion that RedactingStore satisfies Store.
var _ Store = (*RedactingStore)(nil)

// redactionWarnLogger is a test-visible sink for redaction warnings. It is
// package-private and used by tests to assert graceful-degradation behavior.
type redactionWarnLogger struct {
	warnings []string
}

func (l *redactionWarnLogger) logf(format string, args ...any) {
	l.warnings = append(l.warnings, fmt.Sprintf(format, args...))
}
