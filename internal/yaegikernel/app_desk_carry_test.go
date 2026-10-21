package yaegikernel

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
)

// ARCH-03: an app desk the daemon derived reaches its own desk worker
// (docs/problems/app-desk-registration-does-not-cross-worker-execve-2026-10-21.md).
// Failure modes, written before the code:
//   - the worker of an app role starts with no row and serves cells with no
//     verbs (today's fail-closed state: kept when nothing is carried);
//   - a system role is handed an app row and so gains or loses verbs;
//   - the carried row names a different desk than the role;
//   - the carried row is wider than the app set (a verb outside it, a
//     system uid) and the worker installs it anyway;
//   - the carried row is malformed and the worker starts anyway;
//   - the daemon renders a row for a role it has not derived.

func carryEnv(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func appDeskForCarry(t *testing.T, name string, uid uint32) deskmanifest.Desk {
	t.Helper()
	d, err := deskmanifest.NewAppDesk(deskmanifest.AppDeskSpec{Name: name, Verbs: []string{"Note", "Done", "ReadCapture"},
		Taint: deskmanifest.TaintUntrustedInput, UID: uid, WorkerMaxBytes: 256 << 20})
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func envValue(env []string, key string) (string, bool) {
	for _, kv := range env {
		if strings.HasPrefix(kv, key+"=") {
			return strings.TrimPrefix(kv, key+"="), true
		}
	}
	return "", false
}

func TestAppDeskWorkerWithoutACarryHasNoVerbs(t *testing.T) {
	d := appDeskForCarry(t, "app:carry-none", 62100)
	if err := deskmanifest.RegisterApp(d); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = deskmanifest.RetireApp(d.Name) })
	// In a worker process the registry is the static table; simulate by
	// retiring the row before building the scope.
	_ = deskmanifest.RetireApp(d.Name)
	scope, err := appDeskScope(t, d.Name)
	if err != nil {
		t.Fatal(err)
	}
	if scope.deskModule("Note") || scope.desk != "" {
		t.Fatal("an unknown app role got a desk module")
	}
}

func TestAppDeskCarryRendersOnlyForADerivedAppRole(t *testing.T) {
	d := appDeskForCarry(t, "app:carry-render", 62101)
	if err := deskmanifest.RegisterApp(d); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = deskmanifest.RetireApp(d.Name) })

	cfg := DeskSessionWorkerConfig{Session: SessionWorkerConfig{Role: d.Name}}
	raw, ok := envValue(cfg.sessionEnv(), "CHOIR_DESK_SESSION_"+appDeskEnv)
	if !ok {
		t.Fatal("no app desk carried for a derived app role")
	}
	var spec deskmanifest.AppDeskSpec
	if err := json.Unmarshal([]byte(raw), &spec); err != nil {
		t.Fatalf("carried row is not JSON: %v", err)
	}
	if spec.Name != d.Name || spec.UID != d.Sandbox.UID {
		t.Fatalf("carried row %+v", spec)
	}
	for _, role := range []string{"management", "texture", "research", "app:never-derived", ""} {
		cfg := DeskSessionWorkerConfig{Session: SessionWorkerConfig{Role: role}}
		if _, ok := envValue(cfg.sessionEnv(), "CHOIR_DESK_SESSION_"+appDeskEnv); ok {
			t.Errorf("role %q carried an app desk row", role)
		}
	}
}

func TestAppDeskCarryInstallsTheRowInTheWorker(t *testing.T) {
	d := appDeskForCarry(t, "app:carry-ok", 62102)
	raw, _ := json.Marshal(d.AppSpec())
	env := carryEnv(map[string]string{"CHOIR_DESK_SESSION_ROLE": d.Name, "CHOIR_DESK_SESSION_" + appDeskEnv: string(raw)})
	if err := RegisterCarriedAppDesk(env, ""); err != nil {
		t.Fatalf("carry refused: %v", err)
	}
	t.Cleanup(func() { _ = deskmanifest.RetireApp(d.Name) })
	got, ok := deskmanifest.Lookup(d.Name)
	if !ok || !got.Equal(d) {
		t.Fatalf("worker row %+v, want %+v", got, d)
	}
	// The same carry again (a test process, or a re-registered row) is fine.
	if err := RegisterCarriedAppDesk(env, ""); err != nil {
		t.Fatalf("identical carry refused: %v", err)
	}
	scope, err := appDeskScope(t, d.Name)
	if err != nil {
		t.Fatal(err)
	}
	if !scope.deskModule("Note") || scope.deskModule("ProductAPI") || scope.deskModule("Exec") {
		t.Fatal("app desk scope has the wrong module set")
	}
}

func TestAppDeskCarryRefusals(t *testing.T) {
	good := appDeskForCarry(t, "app:carry-bad", 62103)
	spec := good.AppSpec()
	mk := func(mutate func(*deskmanifest.AppDeskSpec)) string {
		s := spec
		s.Verbs = append([]string{}, spec.Verbs...)
		mutate(&s)
		raw, _ := json.Marshal(s)
		return string(raw)
	}
	cases := map[string]map[string]string{
		"app role, no carry": {"CHOIR_DESK_SESSION_ROLE": good.Name},
		"system role handed a row": {"CHOIR_DESK_SESSION_ROLE": "management",
			"CHOIR_DESK_SESSION_" + appDeskEnv: mk(func(s *deskmanifest.AppDeskSpec) { s.Name = "management" })},
		"row names another desk": {"CHOIR_DESK_SESSION_ROLE": good.Name,
			"CHOIR_DESK_SESSION_" + appDeskEnv: mk(func(s *deskmanifest.AppDeskSpec) { s.Name = "app:other" })},
		"verb outside the app set": {"CHOIR_DESK_SESSION_ROLE": good.Name,
			"CHOIR_DESK_SESSION_" + appDeskEnv: mk(func(s *deskmanifest.AppDeskSpec) { s.Verbs = append(s.Verbs, "ProductAPI") })},
		"system uid": {"CHOIR_DESK_SESSION_ROLE": good.Name,
			"CHOIR_DESK_SESSION_" + appDeskEnv: mk(func(s *deskmanifest.AppDeskSpec) { s.UID = 61001 })},
		"malformed": {"CHOIR_DESK_SESSION_ROLE": good.Name, "CHOIR_DESK_SESSION_" + appDeskEnv: "{not json"},
		"unknown field": {"CHOIR_DESK_SESSION_ROLE": good.Name,
			"CHOIR_DESK_SESSION_" + appDeskEnv: `{"Name":"app:carry-bad","Verbs":["Note"],"Taint":"untrusted_input","UID":62103,"WorkerMaxBytes":1048576,"Kind":"system"}`},
	}
	for name, m := range cases {
		if err := RegisterCarriedAppDesk(carryEnv(m), ""); err == nil {
			t.Errorf("%s: carry accepted", name)
		}
		if _, ok := deskmanifest.Lookup(good.Name); ok {
			t.Fatalf("%s: a refused carry registered the desk", name)
		}
	}
	// A system role with no carry is today's path and stays a no-op.
	if err := RegisterCarriedAppDesk(carryEnv(map[string]string{"CHOIR_DESK_SESSION_ROLE": "texture"}), ""); err != nil {
		t.Fatalf("system role without a carry: %v", err)
	}
}

func appDeskScope(t *testing.T, role string) (*ChoirScope, error) {
	t.Helper()
	broker, issuer, _, _ := testChoirFixture(t)
	return NewChoirScope(broker, issuer, "c", "a", 1, role, "")
}
