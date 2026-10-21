package appverbs

import (
	"context"
	"encoding/json"
	"testing"
)

func echo(_ context.Context, _ Caller, args json.RawMessage) (json.RawMessage, error) {
	return args, nil
}

// The registry is off by default: with no switch set, every call returns
// an ordinary "disabled" result and no handler runs (ARCH-14).
func TestRegistryOffByDefaultRunsNothing(t *testing.T) {
	r := New(false)
	ran := false
	if err := r.Register(Verb{App: "mail", Name: "send", Allow: []string{"desk:mail"}, Handler: func(context.Context, Caller, json.RawMessage) (json.RawMessage, error) {
		ran = true
		return nil, nil
	}}); err != nil {
		t.Fatal(err)
	}
	res := r.Call(context.Background(), Caller{ID: "desk:mail"}, "mail", "send", nil)
	if res.OK || res.Code != CodeDisabled || ran {
		t.Fatalf("off registry must refuse without running: %+v ran=%v", res, ran)
	}
}

// Enabled, a verb runs only for callers on its allow list; an empty allow
// list admits nobody; refusals are results the model can act on.
func TestRegistryPerCallerAllow(t *testing.T) {
	r := New(true)
	if err := r.Register(Verb{App: "calendar", Name: "add_event", Allow: []string{"desk:texture", "desk:calendar"}, Handler: echo}); err != nil {
		t.Fatal(err)
	}
	if err := r.Register(Verb{App: "calendar", Name: "nobody", Handler: echo}); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if res := r.Call(ctx, Caller{ID: "desk:calendar"}, "calendar", "add_event", json.RawMessage(`{"a":1}`)); !res.OK || string(res.Value) != `{"a":1}` {
		t.Fatalf("allowed caller refused: %+v", res)
	}
	if res := r.Call(ctx, Caller{ID: "desk:research"}, "calendar", "add_event", nil); res.OK || res.Code != CodeNotAllowed || res.Message == "" {
		t.Fatalf("unlisted caller admitted: %+v", res)
	}
	if res := r.Call(ctx, Caller{}, "calendar", "add_event", nil); res.OK || res.Code != CodeNotAllowed {
		t.Fatalf("anonymous caller admitted: %+v", res)
	}
	if res := r.Call(ctx, Caller{ID: "desk:calendar"}, "calendar", "nobody", nil); res.OK || res.Code != CodeNotAllowed {
		t.Fatalf("empty allow list admitted a caller: %+v", res)
	}
	if res := r.Call(ctx, Caller{ID: "desk:calendar"}, "calendar", "missing", nil); res.OK || res.Code != CodeUnknownVerb {
		t.Fatalf("unknown verb: %+v", res)
	}
}

// A handler error and a handler panic come back as results, not as a
// crash; a duplicate or malformed registration is refused.
func TestRegistryErrorsAreResults(t *testing.T) {
	r := New(true)
	boom := func(context.Context, Caller, json.RawMessage) (json.RawMessage, error) {
		return nil, context.DeadlineExceeded
	}
	pan := func(context.Context, Caller, json.RawMessage) (json.RawMessage, error) { panic("x") }
	_ = r.Register(Verb{App: "a", Name: "boom", Allow: []string{"c"}, Handler: boom})
	_ = r.Register(Verb{App: "a", Name: "pan", Allow: []string{"c"}, Handler: pan})
	c := Caller{ID: "c"}
	if res := r.Call(context.Background(), c, "a", "boom", nil); res.OK || res.Code != CodeFailed {
		t.Fatalf("handler error: %+v", res)
	}
	if res := r.Call(context.Background(), c, "a", "pan", nil); res.OK || res.Code != CodeFailed {
		t.Fatalf("handler panic: %+v", res)
	}
	if err := r.Register(Verb{App: "a", Name: "boom", Handler: boom}); err == nil {
		t.Fatal("duplicate accepted")
	}
	if err := r.Register(Verb{App: "", Name: "x", Handler: boom}); err == nil {
		t.Fatal("empty app accepted")
	}
	if err := r.Register(Verb{App: "a", Name: "nohandler"}); err == nil {
		t.Fatal("nil handler accepted")
	}
}

func TestEnabledFromEnvDefaultsOff(t *testing.T) {
	t.Setenv("CHOIR_APP_VERBS", "")
	if EnabledFromEnv() {
		t.Fatal("must default off")
	}
	t.Setenv("CHOIR_APP_VERBS", "on")
	if !EnabledFromEnv() {
		t.Fatal("on must enable")
	}
}
