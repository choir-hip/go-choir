package agentcore

import (
	"fmt"
	"sync"

	"github.com/yusefmosiah/go-choir/internal/appdesk"
	"github.com/yusefmosiah/go-choir/internal/apppackage"
	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
)

// A desk from another publisher (R24, Phase 6 acceptance: "only then a
// non-first-party desk package installs"). On a computer whose worker sandbox
// is ready (apppackage.CurrentPlatform().DeskSandbox), an installed release
// that carries one desk registers the RELEASE's own row: decoded from its
// manifest through the same validator every package desk passes
// (appdesk.Validate, deskmanifest.NewAppDesk), FirstParty false, so
// planCellConfinement runs its workers in the non-first-party trust class
// (gVisor, one sandbox per desk) or refuses.
//
// The row is capped by the ceiling the install review judged, never wider:
//
//   - verbs come from the manifest's desk grants through releaseDeskVerbs, a
//     closed table; a grant with no row there adds nothing. ReadMail
//     (deskmanifest.FirstPartyOnlyVerb) is in no row, and appdesk.Validate and
//     NewAppDesk refuse it for a non-first-party desk again;
//   - a privileged grant never reaches here (the manifest validator refuses it
//     for any origin but first-party), and a sole-holder grant is no desk
//     verb;
//   - no lease grants, no owned kinds (a release declares no kinds before
//     S12), no egress, no prompt-bar acceptance: the release states none of
//     them and gets none;
//   - the prompt is the release's own prompt file, read from the verified tree
//     at install and kept as a data file of the install (pkgruntime
//     readDeskPrompts); never an image prompt.

// releaseDeskWorkerBytes is the memory budget of a release's desk worker: a
// platform constant, not the manifest's.
const releaseDeskWorkerBytes = 128 << 20

// releaseDeskVerbs maps a granted desk capability to the app verbs it opens.
// Report and Done are every desk's (a run must be able to end).
var releaseDeskVerbs = map[string][]string{
	"desk.act":           {"Cast", "Ask", "Reply", "Escalate"},
	"desk.act.cancel":    {"CancelAct"},
	"desk.captures.read": {"ReadCapture"},
	"desk.request":       {"RequestLease"},
}

// releaseDeskManifest is the desk manifest a release's package manifest
// implies, or an error when it implies none.
func releaseDeskManifest(m apppackage.Manifest) (appdesk.Manifest, error) {
	if len(m.Contributes.Desks) != 1 || m.Contributes.Desks[0].Name != m.Name {
		return appdesk.Manifest{}, fmt.Errorf("package %s: a release carries exactly one desk, named for the package", m.Name)
	}
	if deskmanifest.FirstPartyAppSlug(m.Name) {
		return appdesk.Manifest{}, fmt.Errorf("package %s: the name is a first-party package's", m.Name)
	}
	seen := map[string]bool{}
	verbs := []string{}
	add := func(vs ...string) {
		for _, v := range vs {
			if !seen[v] {
				seen[v] = true
				verbs = append(verbs, v)
			}
		}
	}
	add("Report", "Done")
	for _, c := range m.Capabilities {
		add(releaseDeskVerbs[c.Grant]...)
	}
	dm := appdesk.Manifest{
		Package: m.Name, Verbs: verbs, Taint: deskmanifest.TaintUntrustedInput,
		WorkerMaxBytes: releaseDeskWorkerBytes, OwnedKinds: []string{},
	}
	if rs := appdesk.Validate(dm); len(rs) > 0 {
		return appdesk.Manifest{}, fmt.Errorf("package %s: the desk row is refused: %s: %s", m.Name, rs[0].Code, rs[0].Detail)
	}
	return dm, nil
}

// releaseDeskPrompts holds the prompt of each non-first-party package desk the
// last desk sync registered, by package. It is a cache of the install's data
// file, refilled from the module store at every sync (boot included).
type releaseDeskPrompts struct {
	mu sync.RWMutex
	m  map[string]string
}

func (p *releaseDeskPrompts) set(pkg, prompt string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.m == nil {
		p.m = map[string]string{}
	}
	p.m[pkg] = prompt
}

func (p *releaseDeskPrompts) drop(pkg string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.m, pkg)
}

func (p *releaseDeskPrompts) get(pkg string) (string, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	s, ok := p.m[pkg]
	return s, ok && s != ""
}
