package deskmanifest

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// The grant catalogue (modular architecture plan MA4.1, section 6 B1). A
// desk's authority is the union of the grants it holds; its verbs, capsule
// verbs, modules and budgets are computed from them at registration and are
// never written beside them. No privilege keys on a desk name: every power a
// built-in desk has today is a grant here, and a package may only ever ask
// for the grants marked Package.
//
// A grant is a name and, for the few that take one, arguments:
//
//	precommit.rule(<kinds>)  the closed rule kinds a Precommit may carry
//	cell.deadline(<dur>)     the host's bound on a cell that sets no timeout
//	run.deadline(<dur>)      the bound on the whole run, not one cell
//
// A model floor (the old model.floor(<tier>)) is not a grant: the management
// task's pin and free-model filter were deleted (ec8b4e90d), so the task
// resolves its model like any role and there is no power left to name.

// Grant is one entry of a desk's authority.
type Grant struct {
	Name string
	Args []string
}

// G builds a grant.
func G(name string, args ...string) Grant { return Grant{Name: name, Args: args} }

// String renders the grant as name or name(arg,arg).
func (g Grant) String() string {
	if len(g.Args) == 0 {
		return g.Name
	}
	return g.Name + "(" + strings.Join(g.Args, ",") + ")"
}

type argKind int

const (
	argNone argKind = iota
	argKinds
	argDuration
)

type grantSpec struct {
	// Verbs, CapsuleVerbs and Modules are what holding the grant adds.
	Verbs        []string
	CapsuleVerbs []string
	Modules      []string
	// Privileged grants are first-party only: no package may request them
	// (plan section 5.1, publishing levels).
	Privileged bool
	Arg        argKind
	// Done marks the lifecycle grant a role receives instead of inheriting.
	Lifecycle bool
}

// PrecommitRuleKinds are the closed rule kinds of types.PrecommitRuleKinds,
// the argument of precommit.rule. A test in agentcore pins the two lists
// equal; this package imports only the standard library.
var PrecommitRuleKinds = []string{
	"tests_pass", "freeze_within", "paths_confined", "verify_pass", "completes",
	"judge_accepts", "freeze_minutes", "owner_approves", "approval_minutes",
	"approved_within_rounds", "report_schema_valid", "citations_resolve",
}

var capsuleMutateVerbs = []string{"exec", "read_file", "write_file", "list_dir", "stat", "mkdir", "remove",
	"kill_session", "go_eval", "init_session", "close_session", "job_start", "job_status", "job_stop"}

// catalogue is the closed set of grants. Every verb belongs to exactly one
// grant, so a verb set and a grant set determine each other (VerbGrant).
var catalogue = map[string]grantSpec{
	// The semantic-act tier (the app set).
	"act.cast":          {Verbs: []string{"Cast"}},
	"act.ask":           {Verbs: []string{"Ask"}},
	"act.reply":         {Verbs: []string{"Reply"}},
	"act.cancel":        {Verbs: []string{"CancelAct"}},
	"act.escalate":      {Verbs: []string{"Escalate"}},
	"act.precommit":     {Verbs: []string{"Precommit"}},
	"act.report":        {Verbs: []string{"Report"}},
	"act.report_packet": {Verbs: []string{"ReportPacket"}},
	"act.resolve":       {Verbs: []string{"Resolve"}},
	"act.disagree":      {Verbs: []string{"Disagreement"}},
	"captures.read":     {Verbs: []string{"ReadCapture"}},
	"lease.request":     {Verbs: []string{"RequestLease"}},
	// The Mail custodian's read of the one email a mail.read lease names.
	"mail.read": {Verbs: []string{"ReadMail"}},
	// A package desk's own objects (MA3.8): the four verbs are one grant, and
	// reach only the desk's own package kinds.
	"objects.own":         {Verbs: []string{"Put", "Get", "List", "Retract"}},
	"lifecycle.done":      {Verbs: []string{"Done"}, Lifecycle: true},
	"updates.read":        {Modules: []string{ModuleUpdatesRead}},
	"calibration.read":    {Verbs: []string{"Calibration"}},
	"calibration.restate": {Verbs: []string{"Restate"}},

	// Rule-settled precommit over a closed kind set: only the reducer binds
	// a rule to an assignment, and only a holder may stage one.
	"precommit.rule": {Privileged: true, Arg: argKinds},

	// Management's powers.
	"cancel.assignment": {Verbs: []string{"CancelAssignment"}, Privileged: true},
	"intake.read":       {Verbs: []string{"Stories"}},
	"intake.dispose":    {Verbs: []string{"Feed", "Decline"}, Privileged: true},
	"product.api":       {Verbs: []string{"ProductAPI"}, Privileged: true},
	"delegate.task":     {Verbs: []string{"Delegate", "CancelTask"}, Privileged: true},
	"assignment.verify": {Verbs: []string{"Verify", "VerifyFindings"}, Privileged: true},
	"assignment.shots":  {Verbs: []string{"ShotStart", "ShotWait", "ShotVerdict", "Screens"}, Privileged: true},
	// The install review (desk-install-review-2026-11-08): the sole holder
	// reads the packet of a widened-ceiling install and seals the verdict.
	"assignment.install_review": {Verbs: []string{"InstallPacket", "InstallVerdict"}, Privileged: true},
	// The admission ledger (plan MA4.9, D9): the holder admits engineering
	// work, receives the requests other desks aim at an assignment desk, and
	// is the custodian every request kind's materiality names. It carries no
	// verb: the reducer and the address resolver read it.
	"admit.assignment": {Privileged: true},
	// The platform update (MA4.9): the host's platform-update path acts as
	// this grant's holder, and each canonical event of an update names it.
	// It carries no verb: a platform update is a platform-signed offer the
	// host applies, never a cell's call.
	"platform.update": {Privileged: true},
	// The assignment ledger's performer (MA4.10, D8): the holder performs the
	// work an admitted assignment names, in its own instance and capsule. The
	// assignment ledger asks this grant for the performing desk instead of a
	// name. It carries no verb: the capsule verbs are capsule.mutate's.
	"perform.assignment": {Privileged: true},

	// Texture's document verbs.
	"doc.read":                {Verbs: []string{"ReadDoc"}},
	"commit.texture_revision": {Verbs: []string{"Write"}, Privileged: true},
	"texture.turn":            {Verbs: []string{"Wait", "Reject"}, Privileged: true},

	// Research's world authority.
	"world.read":     {Verbs: []string{"SearchStart", "Searches", "SearchWait", "Read", "SearchWireCorpus"}, Privileged: true},
	"source.poll":    {Verbs: []string{"PollSource", "SourceAPI"}, Privileged: true},
	"evidence.write": {Verbs: []string{"SaveEvidence"}, Privileged: true},
	"evidence.read":  {Verbs: []string{"ReadEvidence", "ListEvidence"}},
	"run.memory":     {Verbs: []string{"RunMemoryEntry"}},
	"research.spawn": {Verbs: []string{"Research"}, Privileged: true},
	"system1.call":   {Verbs: []string{"System1"}, Privileged: true},
	"story.relate":   {Verbs: []string{"RelateStories", "Relate", "StoryAnalysis"}, Privileged: true},
	// The Texture custodian's guarded corpus search (plan 8.6): the verb is
	// served only under a texture.search lease the custodian issues to the
	// research desk's Texture child (internal/lease, store.TextureSearchUnderLease).
	"texture.search": {Verbs: []string{"SearchTexture"}, Privileged: true},
	"capsule.read":   {CapsuleVerbs: []string{"read_file", "list_dir", "stat", "go_eval"}, Privileged: true},

	// Engineering's capsule-bound mutation.
	"capsule.mutate":   {Verbs: []string{"WriteFile", "Exec", "Start", "Poll", "Stop"}, CapsuleVerbs: capsuleMutateVerbs, Privileged: true},
	"selfdev.freeze":   {Verbs: []string{"Freeze"}, Privileged: true},
	"selfdev.complete": {Verbs: []string{"Complete"}, Privileged: true},

	// Budgets.
	"cell.deadline": {Arg: argDuration, Privileged: true},
	"run.deadline":  {Arg: argDuration, Privileged: true},
}

// verbGrant maps each verb to the one grant that carries it.
var verbGrant = func() map[string]string {
	m := map[string]string{}
	for name, spec := range catalogue {
		for _, v := range spec.Verbs {
			if prev, dup := m[v]; dup {
				panic(fmt.Sprintf("deskmanifest: verb %s is in grants %s and %s", v, prev, name))
			}
			m[v] = name
		}
	}
	return m
}()

// GrantNames lists the catalogue, sorted.
func GrantNames() []string {
	out := make([]string, 0, len(catalogue))
	for n := range catalogue {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// GrantPrivileged reports whether the named grant is first-party only. An
// unknown grant is reported privileged: unknown is deny.
func GrantPrivileged(name string) bool {
	spec, ok := catalogue[name]
	return !ok || spec.Privileged
}

// VerbGrant is the grant that carries a verb.
func VerbGrant(verb string) (string, bool) {
	g, ok := verbGrant[verb]
	return g, ok
}

// Grant returns the desk's grant of that name.
func (d Desk) Grant(name string) (Grant, bool) {
	for _, g := range d.Grants {
		if g.Name == name {
			return g, true
		}
	}
	return Grant{}, false
}

// HasGrant reports whether the desk holds the named grant.
func (d Desk) HasGrant(name string) bool { _, ok := d.Grant(name); return ok }

// Grants with exactly one holder on a computer (MA4.9): the admission ledger
// and the platform update. Registration refuses a second holder, so the
// question "who admits" or "who updates" has one answer and never a union.
const (
	GrantAdmitAssignment = "admit.assignment"
	GrantPlatformUpdate  = "platform.update"
	// GrantPerformAssignment is the performer of admitted assignments
	// (MA4.10): one holder, the other half of the assignment ledger.
	GrantPerformAssignment = "perform.assignment"
	// GrantResearchSpawn is the research parent's grant (MA4.11): the one desk
	// that starts research children, so the registry names it.
	GrantResearchSpawn = "research.spawn"
	// GrantInstallReview is the judge of widened-ceiling installs: one
	// holder, so "who reviews an install" has one answer.
	GrantInstallReview = "assignment.install_review"
)

var soleHolderGrants = map[string]bool{GrantAdmitAssignment: true, GrantPlatformUpdate: true, GrantPerformAssignment: true, GrantResearchSpawn: true, GrantInstallReview: true}

// GrantHolder is the one registered desk that holds a sole-holder grant
// (admit.assignment, platform.update). It reports false for any other grant
// and when no desk holds it: unknown is deny, never a default desk.
func GrantHolder(name string) (Desk, bool) {
	if !soleHolderGrants[name] {
		return Desk{}, false
	}
	mu.RLock()
	defer mu.RUnlock()
	for _, d := range registry {
		if d.HasGrant(name) {
			return d.clone(), true
		}
	}
	return Desk{}, false
}

// Holds reports whether the desk named name holds the grant. An unknown desk
// holds nothing.
func Holds(name, grant string) bool {
	mu.RLock()
	defer mu.RUnlock()
	d, ok := registry[name]
	return ok && d.HasGrant(grant)
}

// MayRule reports whether the desk may stage a Precommit carrying a rule of
// this kind (precommit.rule).
func (d Desk) MayRule(kind string) bool {
	g, ok := d.Grant("precommit.rule")
	if !ok {
		return false
	}
	for _, a := range g.Args {
		if a == kind {
			return true
		}
	}
	return false
}

// RulePrecommit reports whether the desk's Precommits are rule-settled.
func (d Desk) RulePrecommit() bool { return d.HasGrant("precommit.rule") }

// RunDeadline is the bound on the whole run from its start, or 0.
func (d Desk) RunDeadline() time.Duration {
	g, ok := d.Grant("run.deadline")
	if !ok || len(g.Args) != 1 {
		return 0
	}
	v, _ := time.ParseDuration(g.Args[0])
	return v
}

func cloneGrants(in []Grant) []Grant {
	if in == nil {
		return nil
	}
	out := make([]Grant, len(in))
	for i, g := range in {
		out[i] = Grant{Name: g.Name, Args: append([]string(nil), g.Args...)}
	}
	return out
}

func sameGrants(a, b []Grant) bool {
	if len(a) != len(b) {
		return false
	}
	as, bs := grantStrings(a), grantStrings(b)
	for i := range as {
		if as[i] != bs[i] {
			return false
		}
	}
	return true
}

func grantStrings(in []Grant) []string {
	out := make([]string, len(in))
	for i, g := range in {
		out[i] = g.String()
	}
	sort.Strings(out)
	return out
}

// GrantStrings renders the desk's grants sorted, for the Library and tests.
func (d Desk) GrantStrings() []string { return grantStrings(d.Grants) }

// GrantsForVerbs is the grant set that carries exactly these verbs; it
// reports the first verb no grant carries.
func GrantsForVerbs(verbs []string) ([]Grant, error) {
	seen := map[string]bool{}
	var out []Grant
	for _, v := range verbs {
		g, ok := verbGrant[v]
		if !ok {
			return nil, fmt.Errorf("verb %q is carried by no grant", v)
		}
		if !seen[g] {
			seen[g] = true
			out = append(out, G(g))
		}
	}
	return out, nil
}

// resolve computes the verb, capsule verb, module and deadline fields from
// the grants. A row that declares grants has no other source for them; a row
// with none (the carrier-less conductor and mail) is left as it is. It
// refuses an unknown grant, a bad argument and a verb set the grants do not
// produce.
func (d *Desk) resolve() error {
	// A per_owner desk's persistent identity is its instance (MA4.9): one
	// agent per owner, <desk>:<owner>. It is computed, never declared beside.
	if d.Instance == InstancePerOwner && d.Identity == IdentityRun {
		d.Identity = IdentityOwner
	}
	if len(d.Grants) == 0 {
		return nil
	}
	verbs, capsule, modules := map[string]bool{}, map[string]bool{}, map[string]bool{}
	seen := map[string]bool{}
	for _, g := range d.Grants {
		spec, ok := catalogue[g.Name]
		if !ok {
			return fmt.Errorf("desk %s: grant %q is not in the catalogue", d.Name, g.Name)
		}
		if seen[g.Name] {
			return fmt.Errorf("desk %s: grant %s is held twice", d.Name, g.Name)
		}
		seen[g.Name] = true
		switch spec.Arg {
		case argNone:
			if len(g.Args) != 0 {
				return fmt.Errorf("desk %s: grant %s takes no argument", d.Name, g.Name)
			}
		case argKinds:
			if len(g.Args) == 0 {
				return fmt.Errorf("desk %s: grant %s needs at least one kind", d.Name, g.Name)
			}
			for _, a := range g.Args {
				if !contains(PrecommitRuleKinds, a) {
					return fmt.Errorf("desk %s: grant %s names %q, which is not a closed rule kind", d.Name, g.Name, a)
				}
			}
		case argDuration:
			if len(g.Args) != 1 {
				return fmt.Errorf("desk %s: grant %s takes one duration", d.Name, g.Name)
			}
			if v, err := time.ParseDuration(g.Args[0]); err != nil || v <= 0 {
				return fmt.Errorf("desk %s: grant %s: %q is not a positive duration", d.Name, g.Name, g.Args[0])
			}
		}
		for _, v := range spec.Verbs {
			verbs[v] = true
		}
		for _, v := range spec.CapsuleVerbs {
			capsule[v] = true
		}
		for _, m := range spec.Modules {
			modules[m] = true
		}
	}
	if len(d.Verbs) > 0 && !sameSet(d.Verbs, verbs) {
		return fmt.Errorf("desk %s: verbs %v are not what its grants produce", d.Name, d.Verbs)
	}
	d.Verbs = sortedKeys(verbs)
	d.CapsuleVerbs = sortedKeys(capsule)
	d.Modules = sortedKeys(modules)
	if len(d.Modules) == 0 {
		d.Modules = nil
	}
	d.CellDeadline = 0
	if g, ok := d.Grant("cell.deadline"); ok {
		d.CellDeadline, _ = time.ParseDuration(g.Args[0])
	}
	return nil
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func sameSet(list []string, set map[string]bool) bool {
	if len(list) != len(set) {
		return false
	}
	for _, x := range list {
		if !set[x] {
			return false
		}
	}
	return true
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// validateRole enforces the role-narrow rule (plan section 6 B1): a role may
// only narrow its parent, except that a role marked EndsRun receives the
// lifecycle grant (done) its parent lacks. The parent is the system desk that
// shares the role's worker uid, so no name is consulted.
func validateRole(d Desk, rows func(func(Desk))) error {
	if d.Kind != KindRole || len(d.Grants) == 0 {
		return nil
	}
	var parent *Desk
	rows(func(p Desk) {
		if p.Kind == KindSystem && p.Sandbox.UID == d.Sandbox.UID && d.Sandbox.UID != 0 {
			cp := p
			parent = &cp
		}
	})
	if parent == nil {
		return fmt.Errorf("desk %s: a role needs a parent system desk (same worker uid) to narrow", d.Name)
	}
	for _, g := range d.Grants {
		spec := catalogue[g.Name]
		if spec.Lifecycle {
			if !d.EndsRun {
				return fmt.Errorf("desk %s: only a role that ends its run holds %s", d.Name, g.Name)
			}
			continue
		}
		if spec.Arg == argDuration {
			continue // a budget is the role's own
		}
		if !parent.HasGrant(g.Name) {
			return fmt.Errorf("desk %s: grant %s widens its parent: a role may only narrow", d.Name, g.Name)
		}
	}
	return nil
}

// AssignmentPerformer is the one desk that performs admitted assignments (the
// perform.assignment grant, MA4.10). With no holder it reports false: unknown
// is deny, never a default desk.
func AssignmentPerformer() (Desk, bool) { return GrantHolder(GrantPerformAssignment) }

// AssignmentInbox is the inbox and persistent agent id of the performer's
// instance for one assignment: <performer>:<assignmentID>. An empty id, or
// no performer, has none.
func AssignmentInbox(assignmentID string) (string, bool) {
	assignmentID = strings.TrimSpace(assignmentID)
	d, ok := AssignmentPerformer()
	if !ok || assignmentID == "" {
		return "", false
	}
	return d.Name + ":" + assignmentID, true
}

// AssignmentIDFromInbox is the inverse of AssignmentInbox: the assignment id
// of a performer inbox, or false for any other address (a bare desk name, an
// empty id, another desk's inbox).
func AssignmentIDFromInbox(inbox string) (string, bool) {
	d, ok := AssignmentPerformer()
	if !ok {
		return "", false
	}
	id, ok := strings.CutPrefix(strings.TrimSpace(inbox), d.Name+":")
	id = strings.TrimSpace(id)
	if !ok || id == "" {
		return "", false
	}
	return id, true
}

// IsAssignmentInbox reports whether the address is a performer inbox.
func IsAssignmentInbox(inbox string) bool {
	_, ok := AssignmentIDFromInbox(inbox)
	return ok
}

// IsAssignmentPerformer reports whether the name is the performing desk.
func IsAssignmentPerformer(name string) bool {
	d, ok := AssignmentPerformer()
	return ok && d.Name == strings.TrimSpace(name)
}

// MutatesCapsule reports whether the named desk may mutate a capsule
// (capsule.mutate). An unknown desk, and a run role, mutate nothing.
func MutatesCapsule(name string) bool { return Holds(name, "capsule.mutate") }

// ReadsCapsules reports whether the named desk may inspect capsules
// (capsule.read).
func ReadsCapsules(name string) bool { return Holds(name, "capsule.read") }

// AdmitsAssignments reports whether the named desk holds the admission ledger
// (admit.assignment, MA4.9): the desk that admits engineering work. Code that
// once compared a profile to the management name asks this.
func AdmitsAssignments(name string) bool { return Holds(name, GrantAdmitAssignment) }

// SpawnsResearch reports whether the named desk may start research children
// (research.spawn): the research parent, never a child role.
func SpawnsResearch(name string) bool { return Holds(name, GrantResearchSpawn) }

// unregistered is what a holder lookup yields with no holder: a name that is
// no desk and matches no stored address, so a comparison fails closed.
const unregistered = "unregistered-holder"

func holderName(grant string) string {
	if d, ok := GrantHolder(grant); ok {
		return d.Name
	}
	return unregistered + ":" + grant
}

// AdmitterName is the registered name of the desk that holds the admission
// ledger (admit.assignment). Event actors, run profiles and inboxes that were
// spelled with the management name ask this. With no holder it is a name no
// desk and no address matches.
func AdmitterName() string { return holderName(GrantAdmitAssignment) }

// ResearcherName is the registered name of the research parent
// (research.spawn), with the same fail-closed fallback.
func ResearcherName() string { return holderName(GrantResearchSpawn) }

// AdmitterInbox is the admitter's persistent inbox for one owner:
// <admitter>:<ownerID>.
func AdmitterInbox(ownerID string) string { return AdmitterName() + ":" + ownerID }

// IsAdmitterInbox reports whether the address is an admitter inbox of any
// owner (the admitter's name, a colon, then anything).
func IsAdmitterInbox(inbox string) bool { return strings.HasPrefix(inbox, AdmitterName()+":") }

// ResearchInbox is the research parent's inbox for one document:
// <researcher>:<docID>.
func ResearchInbox(docID string) string { return ResearcherName() + ":" + docID }

// IsResearchInbox reports whether the address is a research parent inbox.
func IsResearchInbox(inbox string) bool { return strings.HasPrefix(inbox, ResearcherName()+":") }

// PerformerName is the registered name of the desk that performs admitted
// assignments (perform.assignment), with the fail-closed fallback.
func PerformerName() string { return holderName(GrantPerformAssignment) }

// PerformerInbox is the performer's inbox for one assignment, like
// AssignmentInbox but never failing: with no performer it is an address no
// stored record carries.
func PerformerInbox(assignmentID string) string { return PerformerName() + ":" + assignmentID }

// ParentOf is the desk a role belongs to: the sole holder of the grant the
// role's Parent field names (enforce flip, section 7.3). It reports false for
// a desk that is no role, a role with no parent and a parent grant nobody
// holds: unknown is deny. It never reads a name prefix.
func ParentOf(role string) (Desk, bool) {
	d, ok := Lookup(role)
	if !ok || d.Kind != KindRole || d.Parent == "" {
		return Desk{}, false
	}
	p, ok := GrantHolder(d.Parent)
	if !ok || p.Kind != KindSystem {
		return Desk{}, false
	}
	return p, true
}

// AppInboxDesk reports the registered package desk an inbox address belongs to
// (app:<package>:<instance>), so a record landing in it wakes that desk like
// any records desk. It reports false for an address no registered app desk
// owns.
func AppInboxDesk(inbox string) (Desk, bool) {
	parts := strings.SplitN(strings.TrimSpace(inbox), ":", 3)
	if len(parts) < 3 || parts[0] != "app" {
		return Desk{}, false
	}
	d, ok := Lookup("app:" + parts[1])
	return d, ok && d.Kind == KindApp
}

// IsAppInbox reports whether the address is a registered package desk inbox.
func IsAppInbox(inbox string) bool { _, ok := AppInboxDesk(inbox); return ok }
