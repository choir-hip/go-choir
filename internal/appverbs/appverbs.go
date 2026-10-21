// Package appverbs is the app verb registry (ARCH-14; the puter pattern in
// docs/research/puter-everything-2026-10-14/README.md): one verb set per
// app, served to the app's own desk, to other desks and later to external
// agents, with a per-caller allow list. A refusal or failure is an ordinary
// Result the model can act on, never a transport error.
//
// Nothing calls this package yet. The registry is off by default
// (CHOIR_APP_VERBS) and, when off, runs no handler. Rollback: delete the
// package; it has no callers and no state.
package appverbs

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
)

// Result codes returned in a refused or failed Result.
const (
	CodeDisabled    = "disabled"
	CodeUnknownVerb = "unknown_verb"
	CodeNotAllowed  = "not_allowed"
	CodeFailed      = "failed"
)

// Caller names who is invoking a verb, for example "desk:texture".
type Caller struct {
	ID string
}

// Handler runs a verb. The registry has already checked the caller.
type Handler func(ctx context.Context, caller Caller, args json.RawMessage) (json.RawMessage, error)

// Verb is one operation an app offers. Allow lists the caller IDs that may
// run it; an empty list admits nobody (deny by default).
type Verb struct {
	App     string
	Name    string
	Allow   []string
	Handler Handler
}

// Result is the outcome of a call, success or not.
type Result struct {
	OK      bool            `json:"ok"`
	Value   json.RawMessage `json:"value,omitempty"`
	Code    string          `json:"code,omitempty"`
	Message string          `json:"message,omitempty"`
}

// Registry holds the verbs.
type Registry struct {
	enabled bool
	mu      sync.RWMutex
	verbs   map[string]Verb
}

// EnabledFromEnv reads the switch; anything but 1, true, on or yes is off.
func EnabledFromEnv() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("CHOIR_APP_VERBS"))) {
	case "1", "true", "on", "yes":
		return true
	}
	return false
}

// New returns an empty registry.
func New(enabled bool) *Registry {
	return &Registry{enabled: enabled, verbs: map[string]Verb{}}
}

func key(app, name string) string { return app + "/" + name }

// Register adds a verb. A blank app or name, a nil handler or a duplicate
// is refused.
func (r *Registry) Register(v Verb) error {
	v.App, v.Name = strings.TrimSpace(v.App), strings.TrimSpace(v.Name)
	if v.App == "" || v.Name == "" {
		return fmt.Errorf("appverbs: app and verb name are required")
	}
	if v.Handler == nil {
		return fmt.Errorf("appverbs: %s/%s has no handler", v.App, v.Name)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	k := key(v.App, v.Name)
	if _, dup := r.verbs[k]; dup {
		return fmt.Errorf("appverbs: %s already registered", k)
	}
	v.Allow = append([]string(nil), v.Allow...)
	r.verbs[k] = v
	return nil
}

// Call runs a verb for a caller. It never panics and never returns an
// error: every outcome is a Result.
func (r *Registry) Call(ctx context.Context, caller Caller, app, name string, args json.RawMessage) (res Result) {
	if !r.enabled {
		return Result{Code: CodeDisabled, Message: "app verbs are not enabled on this computer"}
	}
	r.mu.RLock()
	v, ok := r.verbs[key(app, name)]
	r.mu.RUnlock()
	if !ok {
		return Result{Code: CodeUnknownVerb, Message: fmt.Sprintf("no verb %s/%s", app, name)}
	}
	if !allowed(v.Allow, caller.ID) {
		return Result{Code: CodeNotAllowed, Message: fmt.Sprintf("caller %q may not run %s/%s", caller.ID, app, name)}
	}
	defer func() {
		if p := recover(); p != nil {
			res = Result{Code: CodeFailed, Message: fmt.Sprintf("%s/%s panicked: %v", app, name, p)}
		}
	}()
	out, err := v.Handler(ctx, caller, args)
	if err != nil {
		return Result{Code: CodeFailed, Message: err.Error()}
	}
	return Result{OK: true, Value: out}
}

func allowed(list []string, id string) bool {
	if strings.TrimSpace(id) == "" {
		return false
	}
	for _, a := range list {
		if a == id {
			return true
		}
	}
	return false
}
