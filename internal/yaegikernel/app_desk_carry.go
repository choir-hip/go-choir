package yaegikernel

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
)

// appDeskEnv (after the session prefix) carries an app desk's row from the
// daemon to the desk worker it re-executes. The worker's registry starts as
// the static system table, so without it an app desk's cells hold no verbs
// (docs/problems/app-desk-registration-does-not-cross-worker-execve-2026-10-21.md).
// The daemon renders it only for a role its own manifest has as an app desk,
// which only appdesk.Activate (CHOIR_APP_DESKS=on) registers. Frame admission
// stays the daemon's own lookup; the carry only sets the worker's exports.
const appDeskEnv = "APP_DESK"

// appDeskCarry is the env entry for role, or "" when role is not an app desk
// the daemon has.
func appDeskCarry(prefix, role string) string {
	d, ok := deskmanifest.Lookup(role)
	if !ok || d.Kind != deskmanifest.KindApp {
		return ""
	}
	raw, err := json.Marshal(d.AppSpec())
	if err != nil {
		return ""
	}
	return prefix + appDeskEnv + "=" + string(raw)
}

// RegisterCarriedAppDesk is the worker-side half: it rebuilds the carried row
// through deskmanifest.NewAppDesk (the install checks) and registers it before
// the first cell. It refuses, and the worker must not start, when an app role
// has no row, a non-app role is handed one, the row names another desk, or the
// row fails the app desk checks. A system role with no carry is a no-op.
func RegisterCarriedAppDesk(getenv func(string) string, prefix string) error {
	if getenv == nil {
		getenv = os.Getenv
	}
	prefix = sessionEnvPrefix(prefix)
	role := getenv(prefix + "ROLE")
	raw := getenv(prefix + appDeskEnv)
	isApp := strings.HasPrefix(role, "app:")
	switch {
	case raw == "" && !isApp:
		return nil
	case raw == "":
		return fmt.Errorf("app desk %s: the daemon carried no row", role)
	case !isApp:
		return fmt.Errorf("role %q is not an app desk but was handed an app desk row", role)
	}
	dec := json.NewDecoder(bytes.NewReader([]byte(raw)))
	dec.DisallowUnknownFields()
	var spec deskmanifest.AppDeskSpec
	if err := dec.Decode(&spec); err != nil {
		return fmt.Errorf("app desk %s: carried row: %w", role, err)
	}
	if spec.Name != role {
		return fmt.Errorf("app desk %s: carried row names %q", role, spec.Name)
	}
	d, err := deskmanifest.NewAppDesk(spec)
	if err != nil {
		return fmt.Errorf("app desk %s: carried row refused: %w", role, err)
	}
	if have, ok := deskmanifest.Lookup(role); ok {
		if !have.Equal(d) {
			return fmt.Errorf("app desk %s: carried row differs from the registered one", role)
		}
		return nil
	}
	return deskmanifest.RegisterApp(d)
}
