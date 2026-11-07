// Package deskmanifest is the one table of what each desk may read, write,
// call, spawn and spend (SDC ARCH-02, launch item L1-04).
//
// Before it, a desk's authority was written in six places that nobody read
// together: agentprofile.PolicyFor (message targets, since deleted), capsule.RoleVerbSets
// (broker verbs), yaegikernel.NewChoirScope (handle actions) and
// deskModuleSets (verbs), agentcore ResolveAddress (inbox routing, from each desk's Instance) and
// deskCarrierLive (carrier). The daemon's frame check, the cell confinement
// profile and the memory budgets read this table instead, so a desk added
// without an entry has no authority: every consumer treats an unknown name
// as deny.
//
// The package imports only the standard library so the daemon, the kernel,
// the capsule and agentprofile can all read it. Actions and verbs are plain
// strings with the spelling of yaegikernel.BrokerAction and the choir
// symbols; tests in those packages pin the spelling.
//
// Desk authority (CLAUDE.md "Desk Ontology"): Texture is the sole writer of
// canonical document revisions; management owns computer-wide coherence and
// admits engineering through delegated casts; engineering performs
// capsule-bound mutation; research has read-only world-evidence authority.
// The table only narrows what the code granted before; parity tests in
// yaegikernel pin each desk's table independently, so a widening shows up
// in review as an edit to a test.
package deskmanifest

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// Desk names, in the spelling the code already uses.
const (
	Management  = "management"
	Texture     = "texture"
	Research    = "research"
	Engineering = "engineering"
	Conductor   = "conductor"
	// Mail is the Mail app agent desk (plan MA4.7): the package desk of the
	// first-party Mail package, app:mail, with the package writer desk:mail.
	// There is no system Mail desk; the carrier-less email row was absorbed
	// into it (Renamed keeps older records' "email" meaning it). It is the
	// custodian of the owner's mail, which stays in maild and is reached only
	// through the Mail desk's service seam (internal/maildesk). Only the
	// in-tree first-party Mail package registers it (FirstPartyAppSlug).
	Mail           = "app:mail"
	ManagementTask = "management_task"
	// ResearchWeb and ResearchTexture are the two child roles of the research
	// desk's fan-out (plan MA4.6, section 8.6). The web child reads the world
	// and holds no lease; the Texture child holds the Texture custodian's
	// texture.search lease and reads nothing else. They replaced the single
	// research_child role.
	ResearchWeb     = "research_web"
	ResearchTexture = "research_texture"
)

// Kind separates the desks a person addresses from the roles a desk spawns.
type Kind string

const (
	// KindSystem is a persistent root desk.
	KindSystem Kind = "system"
	// KindRole is a run role of a system desk (a management task, a research
	// child): not addressable, no desk of its own.
	KindRole Kind = "role"
	// KindApp is an app agent's desk, registered from an app package.
	KindApp Kind = "app"
)

// Carrier is where a desk's model-written Go runs.
type Carrier string

const (
	// CarrierDeskWorker is a confined desk-session worker process.
	CarrierDeskWorker Carrier = "desk_worker"
	// CarrierCapsule is a guest capsule (engineering).
	CarrierCapsule Carrier = "capsule"
	// CarrierNone is a desk with no cell carrier (the conductor).
	CarrierNone Carrier = "none"
)

// Taint is ARCH-05's class. A worker of one class never serves the other.
type Taint string

const (
	// TaintUntrustedInput desks read attacker-controlled content from the
	// world directly (research: web pages; the Mail desk: mail bodies).
	TaintUntrustedInput Taint = "untrusted_input"
	// TaintPrivileged desks hold authority to act (product API, canonical
	// writes, capsule mutation). Untrusted content reaches them only as
	// records and reports, never as a call or shared memory.
	TaintPrivileged Taint = "privileged"
)

// RestartContract is CLAUDE.md "Restarts End Work": a crash closes the desk's
// open obligations with the visible fate "interrupted by a restart" and a
// planned update resumes them once, behind a consumed marker. Every desk has
// this one; an app desk may not declare another.
const RestartContract = "crash_closes_planned_resumes_once"

// Sandbox is a desk worker's confinement profile (SEC-01).
type Sandbox struct {
	// Profile names the profile: "desk_cell" (the desk worker's: own
	// unprivileged uid, empty network namespace, Landlock, deny-by-default
	// seccomp with no sockets, exec or fork), "capsule" (engineering's, in
	// internal/capsule) or "none".
	Profile string
	// UID is the numeric uid and gid the worker runs as. Distinct per
	// worker class that must not signal or read another; 0 for no worker.
	// Numeric, so a worker never depends on a passwd entry: the image
	// declares matching users for operators, the spawner does not look
	// them up.
	UID uint32
}

// Memory is a desk's budgets (RES-01 to RES-04). Zero means "not
// applicable".
type Memory struct {
	// WorkerMaxBytes is a desk worker's cgroup memory.max: the kernel kills
	// the worker past it, never the daemon (RES-02).
	WorkerMaxBytes uint64
	// WorkerHighBytes is the worker's memory.high, or 0 for none. Past
	// memory.high the kernel throttles the worker's allocations instead of
	// killing it, and with no swap to reclaim into a runaway cell crawls for
	// minutes before it reaches memory.max (measured in a Linux container:
	// 5272 throttle events in 90 s, no kill). A desk worker's budget is the
	// hard limit alone, so a runaway is killed promptly and the desk is told.
	WorkerHighBytes uint64
	// CapsuleRequestedBytes is what a capsule of this desk asks for
	// (RES-04); admission counts it, memory.high is set to it.
	CapsuleRequestedBytes int64
}

// Instance is the shape of a desk's inbox: how many of the desk there are
// and what names one (MA1.2). The address resolver reads it to turn a
// recipient name into the registered inbox; a desk with no instance has no
// inbox that records reach.
type Instance string

const (
	// InstanceNone: the desk has no records inbox.
	InstanceNone Instance = ""
	// InstancePerOwner: one per owner, <desk>:<owner>. It is also the desk's
	// persistent identity: its agent id is the same <desk>:<owner>
	// (OwnerInstance), so the inbox and the agent never disagree (MA4.9).
	InstancePerOwner Instance = "per_owner"
	// InstanceSubject: one per subject (a document), <desk>:<doc>.
	InstanceSubject Instance = "subject"
	// InstanceAssignment: one per assignment, <desk>:<assignment>; only
	// the holder of the assignment kind opens one.
	InstanceAssignment Instance = "assignment"
)

// Desk is one row of the manifest.
type Desk struct {
	Name    string
	Kind    Kind
	Carrier Carrier

	// Verbs are the choir verbs the desk's cells may call beyond the
	// observation tier every cell has (ReadFile, ListDir, Context, Records,
	// Pack, Help, CellOut).
	Verbs []string
	// CapsuleVerbs are the broker verbs of a desk that runs in a capsule
	// (capsule.RoleVerbSets).
	CapsuleVerbs []string

	// Instance is the shape of the desk's records inbox (MA1.2).
	Instance Instance

	Sandbox Sandbox
	Taint   Taint
	Memory  Memory

	// OwnedKinds are the object kinds the desk is the custodian of, the only
	// writer of, in the spelling of the object graph (objectgraph.ObjectKind).
	// It is the one source of custody: the object graph derives each kind's
	// writer from it at load and refuses a kind claimed twice or not at all
	// (MA3.1). The address resolver reads the assignment kind: its single
	// holder admits engineering work and receives forwarded engineering
	// requests (a test pins one holder).
	OwnedKinds []string
	// LeaseKinds and LeaseCaptures are the desk's lease ceiling (MA3.4, R20):
	// the most it may ever REQUEST of a custodian. LeaseKinds names the object
	// kinds a lease may cover for the desk; LeaseCaptures admits a lease whose
	// scope names captures by id or label. The ceiling is declared at install
	// and shown by the Library; the lease service issues inside it and never
	// widens it. A system desk declares none: its reads are the custody
	// table's, not leases.
	LeaseKinds    []string
	LeaseCaptures bool
	// LeaseGrants names the first-party typed grants (a custodian's guarded
	// service API, such as mail.read) the desk may ask the grant's custodian
	// for: the grant part of the same ceiling. The lease service checks the
	// name against its grant table at issue and at every use.
	LeaseGrants []string
	// FirstParty marks an app desk an in-tree first-party package carries
	// under a reserved slug (the Mail desk, MA4.7). Only NewAppDesk sets it.
	FirstParty bool
	// Packaged marks a system desk that the in-tree first-party package of
	// the desk's own name carries: the desk's window is that package's app,
	// and app and desk land together (the Texture package, slice T1). Only
	// that first-party package may take the desk's name (CarriedByPackage).
	// It grants nothing, so Equal does not compare it and the desk protocol
	// lock does not carry it.
	Packaged bool
	// Accepts are the plain-language intents an app desk takes from the
	// prompt bar (its package's contributes.desks[].accepts, plan MA4.8): the
	// prompt router's candidate descriptions. They grant nothing, so Equal
	// (authority) does not compare them and the desk protocol lock does not
	// carry them; a desk with none is never a router candidate. A system desk
	// declares none.
	Accepts []string
	// ModelRole is the modelpolicy role key. Declared; no consumer yet.
	ModelRole string
	// Restart is the restart contract. Declared; every desk has the same.
	Restart string

	// Profile reports that the desk's name is an agent profile and a model
	// policy role: agentprofile.Canonical and modelpolicy.NormalizeRole
	// resolve it (MA2.5, desks design F1 and F2). A role that only borrows
	// its parent's policy (a research child) is not one.
	Profile bool
	// Prompt reports that the desk has a stored role prompt (promptstore).
	Prompt bool
	// Identity is how the desk's agent id is minted (F6).
	Identity Identity
	// OwnChannel reports that the desk's channel id is its agent id (a
	// persistent desk whose runs share one channel).
	OwnChannel bool
	// CellDeadline is the host's bound on a cell that sets no timeout of its
	// own, or 0 for none (F11).
	CellDeadline time.Duration
	// Modules are named groups of observation verbs beyond the base tier
	// (F12). "updates.read" is the packet observation Updates.
	Modules []string

	// Grants are the desk's authority (MA4.1, grants.go): the one source of
	// Verbs, CapsuleVerbs, Modules and CellDeadline, which are computed from
	// them at registration. A row with none (the carrier-less conductor and
	// email) keeps those fields as written.
	Grants []Grant
	// EndsRun marks a role whose run ends with its own Done: it receives the
	// lifecycle grant its parent lacks (plan section 6 B1).
	EndsRun bool
}

// Identity is how a desk's agent id is minted (desks design F6).
type Identity string

const (
	// IdentityRun: a fresh id per run.
	IdentityRun Identity = ""
	// IdentityOwner: <desk>:<owner>, one agent per owner.
	IdentityOwner Identity = "owner"
	// IdentitySubject: <desk>:<subject>, one agent per subject (a document).
	IdentitySubject Identity = "subject"
)

// MaxWakesPerHour bounds how often an app desk wakes: the runtime refuses a
// run of an app desk past it in a rolling hour, and a package manifest whose
// schedules would wake one desk more often than this is refused at install
// (MA3.9). It is a rate bound on a computer's own resources, not a spend
// ceiling (R8).
const MaxWakesPerHour = 120

// ModuleUpdatesRead is the module that grants the Updates observation.
const ModuleUpdatesRead = "updates.read"

// Verb dispatch classes. A verb either calls a host-served action over the
// frame protocol (the daemon checks it against the role table), is served by
// the worker's own broker inside its confinement (local), or dispatches
// nothing (it stages tray intents or reads the cell frame).
type dispatch struct {
	action string
	local  bool
}

// verbDispatch is read from the s.call sites in yaegikernel/choir.go,
// search_store.go and system1.go. TestVerbDispatchMatchesKernel pins it.
var verbDispatch = map[string]dispatch{
	"ReadFile":  {"read_file", true},
	"ListDir":   {"list_dir", true},
	"WriteFile": {"write_file", true},
	"Exec":      {"exec", true},

	"SearchStart":      {"search_start", false},
	"Read":             {"read", false},
	"PollSource":       {"poll_source", false},
	"SourceAPI":        {"source_api", false},
	"ReadCapture":      {"read_capture", false},
	"SearchWireCorpus": {"search_wire_corpus", false},
	"SaveEvidence":     {"save_evidence", false},
	"ReadEvidence":     {"read_evidence", false},
	"ListEvidence":     {"list_evidence", false},
	"RunMemoryEntry":   {"get_run_memory_entry", false},
	"ProductAPI":       {"product_api", false},
	"Stories":          {"stories", false},
	"ShotStart":        {"shot_start", false},
	"ShotWait":         {"shot_wait", false},
	"Screens":          {"screens", false},
	"Research":         {"research", false},
	"SearchTexture":    {"texture_search", false},
	"System1":          {"system1", false},
	"RelateStories":    {"relate_stories", false},
	"Relate":           {"relate", false},
	"RequestLease":     {"request_lease", false},
	"Get":              {"object_get", false},
	"List":             {"object_list", false},
}

// VerbAction reports the action a verb dispatches, whether the worker serves
// it locally, and whether the verb dispatches one at all.
func VerbAction(verb string) (action string, local, ok bool) {
	d, found := verbDispatch[verb]
	return d.action, d.local, found
}

const mib = 1 << 20

// system is the manifest of the system desks and roles. Verbs are the module
// sets of yaegikernel.deskModuleSets; every other field is read from the
// table it replaces.
var system = []Desk{
	{
		Name: Management, Kind: KindSystem, Carrier: CarrierDeskWorker,
		Instance: InstancePerOwner,
		// Management delegates engineering work and reports; it does not
		// touch the filesystem (mutation is capsule-bound under
		// engineering). It reads the captures a capture arrival names and
		// never fetches the world.
		Grants: []Grant{G("act.cast"), G("act.ask"), G("act.reply"), G("act.cancel"), G("cancel.assignment"),
			G("act.escalate"), G("act.precommit"), G("act.report"), G("act.report_packet"), G("act.resolve"), G("act.disagree"),
			G("product.api"), G("captures.read"), G("intake.read"), G("intake.dispose"), G("assignment.verify"), G("assignment.shots"),
			G("delegate.task"), G("calibration.read"), G("calibration.restate"),
			G("updates.read"), G("cell.deadline", "5m0s"),
			// The admission ledger and the platform update (MA4.9): one
			// holder each, first-party only.
			G("admit.assignment"), G("platform.update")},
		Sandbox: Sandbox{Profile: "desk_cell", UID: 61001},
		Taint:   TaintPrivileged,
		Memory:  Memory{WorkerMaxBytes: 768 * mib},
		OwnedKinds: []string{
			// Assignments and their admission, the owner's intake and the
			// Autopaper source schedules (plan section 8.2).
			"choir.capture_disposition",
			"choir.management_task", "choir.capture_label", "choir.capture_label_override",
			"choir.world_event", "choir.event_membership", "choir.source_schedule",
		},
		ModelRole: Management, Restart: RestartContract,
		Profile: true, Prompt: true, OwnChannel: true,
	},
	{
		Name: Texture, Kind: KindSystem, Carrier: CarrierDeskWorker,
		Instance: InstanceSubject,
		// Texture writes its document with one verb, Write, and closes a
		// turn with Done, Wait or Reject. It is the sole writer of canonical
		// document revisions. It reads the captures its frame names; it
		// never fetches the world.
		Grants: []Grant{G("act.cast"), G("act.ask"), G("doc.read"), G("captures.read"),
			G("commit.texture_revision"), G("lifecycle.done"), G("texture.turn"), G("updates.read")},
		Sandbox: Sandbox{Profile: "desk_cell", UID: 61002},
		Taint:   TaintPrivileged,
		Memory:  Memory{WorkerMaxBytes: 768 * mib},
		OwnedKinds: []string{
			"choir.texture_document", "choir.texture_revision", "choir.texture_decision", "choir.texture_root",
			"choir.source_entity", "choir.source_ref", "choir.citation_check", "choir.support_verdict",
			"choir.story_library_head", "choir.story_index",
		},
		ModelRole: Texture, Restart: RestartContract,
		Packaged: true,
		Profile:  true, Prompt: true, Identity: IdentitySubject,
	},
	{
		Name: Research, Kind: KindSystem, Carrier: CarrierDeskWorker,
		Instance: InstanceSubject,
		// Research observes the world read-only but has full message
		// authority: read-only world access is not read-only messaging. Its
		// grants are the union its two child roles narrow (plan 8.6):
		// world.read for research_web, texture.search for research_texture.
		// The desk's own run holds no texture.search lease, so its own
		// SearchTexture is refused by the custodian; only the Texture child
		// is issued one.
		Grants: []Grant{G("act.cast"), G("act.ask"), G("act.reply"), G("act.cancel"),
			G("act.escalate"), G("act.precommit"), G("act.report"), G("act.report_packet"), G("act.resolve"), G("act.disagree"),
			G("world.read"), G("texture.search"), G("captures.read"), G("source.poll"), G("evidence.write"), G("evidence.read"), G("run.memory"),
			G("research.spawn"), G("system1.call"), G("story.relate"), G("capsule.read")},
		Sandbox:    Sandbox{Profile: "desk_cell", UID: 61003},
		Taint:      TaintUntrustedInput,
		Memory:     Memory{WorkerMaxBytes: 768 * mib},
		OwnedKinds: []string{"choir.agent_evidence"},
		ModelRole:  Research, Restart: RestartContract,
		Profile: true, Prompt: true,
	},
	{
		Name: Engineering, Kind: KindSystem, Carrier: CarrierCapsule,
		Instance: InstanceAssignment,
		// Engineering mutates inside its capsule, converses with
		// management on records, and ends its assignment with Complete,
		// freezing a candidate first.
		Grants: []Grant{G("capsule.mutate"), G("selfdev.complete"), G("selfdev.freeze"),
			G("act.precommit"), G("precommit.rule", PrecommitRuleKinds...), G("act.report"), G("act.ask"), G("act.reply"),
			G("calibration.read"), G("calibration.restate"),
			// The performer of admitted assignments (MA4.10): one holder.
			G("perform.assignment")},
		Sandbox: Sandbox{Profile: "capsule"},
		Taint:   TaintPrivileged,
		// A capsule asks for what the cmd/autoputer link needs (measured
		// peak 1.50 GiB stripped, 1.80 GiB with DWARF).
		Memory: Memory{CapsuleRequestedBytes: 2 << 30},
		OwnedKinds: []string{

			"choir.co_super_slot", "choir.engineering_lineage",
		},
		ModelRole: Engineering, Restart: RestartContract,
		Profile: true, Prompt: true,
	},
	{
		Name: ManagementTask, Kind: KindRole, Carrier: CarrierDeskWorker,
		// A management task disposes of the captures its lease names. It
		// has no filesystem, assignment, message, product API or ledger
		// authority: one level of delegation.
		Grants: []Grant{G("act.cast"), G("captures.read"), G("intake.read"), G("intake.dispose"), G("lifecycle.done"),
			G("cell.deadline", "5m0s"), G("run.deadline", "10m0s")},
		EndsRun:   true,
		Sandbox:   Sandbox{Profile: "desk_cell", UID: 61001},
		Taint:     TaintPrivileged,
		Memory:    Memory{WorkerMaxBytes: 384 * mib},
		ModelRole: ManagementTask, Restart: RestartContract,
		Profile: true,
	},
	{
		Name: ResearchWeb, Kind: KindRole, Carrier: CarrierDeskWorker,
		// The research desk's web child (plan 8.6) reads the world and
		// returns its findings to the parent as a Report. It holds no lease,
		// so no owner-private Texture text reaches it, and it cannot message,
		// write or fan out.
		Grants:    []Grant{G("world.read"), G("captures.read"), G("evidence.read")},
		Sandbox:   Sandbox{Profile: "desk_cell", UID: 61003},
		Taint:     TaintUntrustedInput,
		Memory:    Memory{WorkerMaxBytes: 384 * mib},
		ModelRole: ResearchWeb, Restart: RestartContract,
	},
	{
		Name: ResearchTexture, Kind: KindRole, Carrier: CarrierDeskWorker,
		// The research desk's Texture child (plan 8.6) searches the owner's
		// Texture corpus through the custodian's guarded search, under the
		// texture.search lease the Texture custodian issues it for the call.
		// It holds no world.read: it has no outbound channel, and returns its
		// findings to the parent as a Report labelled owner_private:texture.
		Grants:    []Grant{G("texture.search")},
		Sandbox:   Sandbox{Profile: "desk_cell", UID: 61003},
		Taint:     TaintUntrustedInput,
		Memory:    Memory{WorkerMaxBytes: 384 * mib},
		ModelRole: ResearchTexture, Restart: RestartContract,
	},
	{
		Name: Conductor, Kind: KindSystem, Carrier: CarrierNone,
		Sandbox: Sandbox{Profile: "none"},
		Taint:   TaintPrivileged, ModelRole: Conductor, Restart: RestartContract,
		Profile: true, Prompt: true, Identity: IdentityOwner,
	},
}

var (
	mu       sync.RWMutex
	registry = map[string]Desk{}
)

// init loads the built-in table through register, the one path every desk
// takes (MA2.5, desks design D1): nothing is a special case in code.
func init() {
	for i := range system {
		// The table holds grants; resolve computes the verbs and budgets once,
		// so the table and the registry hold the same rows.
		if err := system[i].resolve(); err != nil {
			panic("deskmanifest: " + err.Error())
		}
		if err := register(system[i]); err != nil {
			panic("deskmanifest: " + err.Error())
		}
	}
}

// register validates a desk and adds it to the registry. It is unexported:
// only init (the built-in table) and RegisterApp (which applies the app
// discipline first) call it, so no other path can place a row with more
// authority than its kind allows. It refuses a name that exists, so a desk can
// never replace another.
func register(d Desk) error {
	d = d.clone()
	if err := d.resolve(); err != nil {
		return err
	}
	if err := validateStructure(d); err != nil {
		return err
	}
	mu.Lock()
	defer mu.Unlock()
	if err := validateRole(d, func(f func(Desk)) {
		for _, r := range registry {
			f(r)
		}
	}); err != nil {
		return err
	}
	if _, exists := registry[d.Name]; exists {
		return fmt.Errorf("desk %s is already registered", d.Name)
	}
	for g := range soleHolderGrants {
		if !d.HasGrant(g) {
			continue
		}
		for _, o := range registry {
			if o.HasGrant(g) {
				return fmt.Errorf("desk %s: grant %s has one holder, and %s holds it", d.Name, g, o.Name)
			}
		}
	}
	registry[d.Name] = d.clone()
	return nil
}

func (d Desk) clone() Desk {
	cp := func(in []string) []string {
		if in == nil {
			return nil
		}
		return append([]string{}, in...)
	}
	d.Verbs = cp(d.Verbs)
	d.CapsuleVerbs = cp(d.CapsuleVerbs)
	d.OwnedKinds = cp(d.OwnedKinds)
	d.LeaseKinds = cp(d.LeaseKinds)
	d.LeaseGrants = cp(d.LeaseGrants)
	d.Accepts = cp(d.Accepts)
	d.Modules = cp(d.Modules)
	d.Grants = cloneGrants(d.Grants)
	return d
}

// Profile reports whether name is a registered agent profile and whether it
// is a run role. It reads the registry without cloning the row: the hot
// paths (agentprofile.Canonical, IsLive, modelpolicy.NormalizeRole) run per
// record and per run.
func Profile(name string) (isProfile, isRole bool) {
	mu.RLock()
	defer mu.RUnlock()
	d, ok := registry[name]
	if !ok {
		return false, false
	}
	return d.Profile, d.Kind == KindRole
}

// Lookup returns a copy of the desk named name. An unknown name returns
// false and every consumer must treat that as deny: there is no default
// desk.
func Lookup(name string) (Desk, bool) {
	mu.RLock()
	defer mu.RUnlock()
	d, ok := registry[name]
	if !ok {
		return Desk{}, false
	}
	return d.clone(), true
}

// Names lists every desk in the manifest, sorted.
func Names() []string {
	mu.RLock()
	defer mu.RUnlock()
	out := make([]string, 0, len(registry))
	for n := range registry {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// HasModule reports whether the desk holds the named module.
func (d Desk) HasModule(module string) bool {
	for _, m := range d.Modules {
		if m == module {
			return true
		}
	}
	return false
}

// ProfileNames lists the names that are agent profiles, sorted. It includes a
// run role (management_task): a profile is a name Canonical and NormalizeRole
// resolve, not a desk anyone starts work on; Profile reports the role.
func ProfileNames() []string {
	return namesWhere(func(d Desk) bool { return d.Profile })
}

// PromptNames lists the desks that have a stored role prompt, sorted.
func PromptNames() []string {
	return namesWhere(func(d Desk) bool { return d.Prompt })
}

// CellCarrierNames lists the desks whose cells run on the host desk-cell
// carrier (a confined desk worker), roles included, sorted.
func CellCarrierNames() []string {
	return namesWhere(func(d Desk) bool { return d.Carrier == CarrierDeskWorker && d.Profile })
}

func namesWhere(keep func(Desk) bool) []string {
	mu.RLock()
	defer mu.RUnlock()
	var out []string
	for n, d := range registry {
		if keep(d) {
			out = append(out, n)
		}
	}
	sort.Strings(out)
	return out
}

// HasVerb reports whether the desk's cells may call verb.
func (d Desk) HasVerb(verb string) bool {
	for _, v := range d.Verbs {
		if v == verb {
			return true
		}
	}
	return false
}

// LocalActions are the actions the desk's own worker broker serves inside
// its confinement: read_file and list_dir always, write_file and exec only
// for a desk with the WriteFile and Exec verbs (engineering).
func (d Desk) LocalActions() []string {
	out := []string{"read_file", "list_dir"}
	if d.HasVerb("WriteFile") {
		out = append(out, "write_file")
	}
	if d.HasVerb("Exec") {
		out = append(out, "exec")
	}
	return out
}

// FrameActions are the actions the daemon serves from this desk's worker
// over the frame protocol (SEC-02): exactly the host-served actions its
// verbs dispatch.
func (d Desk) FrameActions() []string {
	seen := map[string]bool{}
	var out []string
	for _, v := range d.Verbs {
		if a, local, ok := VerbAction(v); ok && !local && !seen[a] {
			seen[a] = true
			out = append(out, a)
		}
	}
	return out
}

// AllowsFrame reports whether the daemon may serve action from this desk's
// worker.
func (d Desk) AllowsFrame(action string) bool {
	for _, a := range d.FrameActions() {
		if a == action {
			return true
		}
	}
	return false
}

// HandleActions are the scopes of the worker's session handle: local plus
// frame actions.
func (d Desk) HandleActions() []string {
	return append(d.LocalActions(), d.FrameActions()...)
}

// FrameAllowed is the daemon's question: may the worker of the desk named
// name be served action? An unknown desk is never served.
func FrameAllowed(name, action string) bool {
	d, ok := Lookup(name)
	return ok && d.AllowsFrame(action)
}

// retiredVerbs are verbs deleted with the emit and message channels (MA1.1)
// and with the agent Note (MA1.3). A manifest naming one is refused at
// validation.
var retiredVerbs = map[string]bool{"Emit": true, "Emits": true, "Message": true, "Note": true}

func validateStructure(d Desk) error {
	if d.Name == "" {
		return fmt.Errorf("a desk has no name")
	}
	// The verifier roles and the frozen history profiles are resolved by
	// agentprofile and modelpolicy after the registry; a row under one of
	// their names would shadow them.
	switch d.Name {
	case "verifier", "verifier_multimodal", "verifier-multimodal", "processor", "reconciler":
		return fmt.Errorf("desk name %q is reserved", d.Name)
	}
	if d.Restart != RestartContract {
		return fmt.Errorf("desk %s: restart contract %q is not %q", d.Name, d.Restart, RestartContract)
	}
	if d.Taint == "" {
		return fmt.Errorf("desk %s: no taint class", d.Name)
	}
	switch d.Instance {
	case InstanceNone, InstancePerOwner, InstanceSubject, InstanceAssignment:
	default:
		return fmt.Errorf("desk %s: instance %q is not none, per_owner, subject or assignment", d.Name, d.Instance)
	}
	if d.Instance == InstancePerOwner && d.Identity != IdentityOwner {
		return fmt.Errorf("desk %s: a per_owner desk's identity is its instance (<desk>:<owner>), not %q", d.Name, d.Identity)
	}
	if d.Kind == KindRole && d.Instance != InstanceNone {
		return fmt.Errorf("desk %s: a role has no inbox of its own", d.Name)
	}
	for _, v := range d.Verbs {
		if v == "" {
			return fmt.Errorf("desk %s: empty verb", d.Name)
		}
		if retiredVerbs[v] {
			return fmt.Errorf("desk %s: verb %s was deleted (emit and message channels, MA1.1; the Note, MA1.3); reach another desk with a typed record (Cast, Ask, Feed, Decline, ReportPacket)", d.Name, v)
		}
	}
	if d.Carrier == CarrierDeskWorker {
		if d.Sandbox.Profile != "desk_cell" || d.Sandbox.UID == 0 {
			return fmt.Errorf("desk %s: a desk worker needs the desk_cell profile and a non-zero uid", d.Name)
		}
		if d.Memory.WorkerMaxBytes == 0 || (d.Memory.WorkerHighBytes != 0 && d.Memory.WorkerHighBytes >= d.Memory.WorkerMaxBytes) {
			return fmt.Errorf("desk %s: a desk worker needs a memory.max, and memory.high (if any) below it", d.Name)
		}
	}
	for _, v := range d.Verbs {
		if a, local, ok := VerbAction(v); ok && local && !d.HasGrant("capsule.mutate") {
			if a == "write_file" || a == "exec" {
				return fmt.Errorf("desk %s: only a desk holding capsule.mutate holds %s", d.Name, a)
			}
		}
	}
	return nil
}

// reservedAppSlugs are the slugs no app desk may take: every built-in desk
// and role and every shell app name of the package manifest
// (internal/apppackage firstparty_gen.go shellAppNames, which a published
// package may not take; a test there pins that this list covers it). The
// built-in packages (files, image, library-next, reader) may carry a desk
// through the owner's own track, so they are not reserved here and the first-party desks the plan names ahead of their move.
var reservedAppSlugs = map[string]bool{
	"audio": true, "autopaper": true, "browser": true, "calendar": true, "compute-monitor": true,
	"conductor": true, "content": true, "desk": true, "desktop": true, "email": true, "engineering": true,
	"epub": true, "mail": true,
	"management": true, "management-task": true, "management_task": true, "owner": true, "pdf": true,
	"podcast": true, "prompt": true, "pulse": true, "research": true,
	"research-child": true, "research_child": true, "research-web": true, "research_web": true,
	"research-texture": true, "research_texture": true, "settings": true, "shell": true, "slides": true,
	"system": true, "texture": true, "video": true,
	// "app" is the namespace prefix itself (MA3.6 N6).
	"app": true,
}

// renamed maps a desk's former name to its current one, so a record written
// before a rename keeps its meaning. The Mail desk absorbed the email row
// (MA4.7).
var renamed = map[string]string{"email": Mail}

// Renamed reports the current name of a desk that was renamed, for records
// written under the old name.
func Renamed(name string) (string, bool) {
	n, ok := renamed[name]
	return n, ok
}

// firstPartyAppSlugs are the reserved slugs an in-tree first-party package
// carries its own desk under (MA4.7): the slug stays reserved for every other
// package, and only a spec marked FirstParty, which internal/appdesk sets
// solely for the manifest the image ships for that package, may take it.
var firstPartyAppSlugs = map[string]bool{"mail": true}

// CarriedByPackage reports whether name is a registered desk that the
// in-tree first-party package of the same name carries (Desk.Packaged): that
// package, and no other, may take the desk's name.
func CarriedByPackage(name string) bool {
	d, ok := Lookup(name)
	return ok && d.Packaged
}

// FirstPartyAppSlug reports whether slug is a reserved slug an in-tree
// first-party package registers its desk under.
func FirstPartyAppSlug(slug string) bool { return firstPartyAppSlugs[slug] }

// IsReservedAppSlug reports whether slug (the part after "app:") is reserved,
// or names a registered non-app desk.
func IsReservedAppSlug(slug string) bool {
	if reservedAppSlugs[slug] {
		return true
	}
	mu.RLock()
	defer mu.RUnlock()
	d, ok := registry[slug]
	return ok && d.Kind != KindApp
}

var appName = regexp.MustCompile(`^app:[a-z][a-z0-9-]{0,31}$`)

// leaseGrantName is the grammar of a typed grant name (mail.read).
var leaseGrantName = regexp.MustCompile(`^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)+$`)

// appDeskVerbs is every verb an app desk may be given: the semantic-act tier
// Texture, management and research share, and capture reads. No egress and no
// world verb: an app desk's egress is declared, owner-approved and added with
// NET-10, not taken from here.
var appDeskVerbs = map[string]bool{
	"Cast": true, "Ask": true, "Reply": true, "CancelAct": true, "Escalate": true,
	"Precommit": true, "Report": true, "ReportPacket": true, "Resolve": true, "Disagreement": true,
	"ReadCapture": true, "RequestLease": true, "Done": true,
	"Put": true, "Get": true, "List": true, "Retract": true,
}

// AppDeskSpec is what an app package's desk manifest declares (ARCH-03).
type AppDeskSpec struct {
	Name           string
	Verbs          []string
	Taint          Taint
	UID            uint32
	WorkerMaxBytes uint64
	OwnedKinds     []string
	// LeaseKinds and LeaseCaptures are the lease ceiling (Desk.LeaseKinds).
	LeaseKinds    []string
	LeaseCaptures bool
	// LeaseGrants is the grant part of the ceiling (Desk.LeaseGrants).
	LeaseGrants []string
	// FirstParty admits a reserved first-party slug (FirstPartyAppSlug):
	// internal/appdesk sets it only for the desk manifest the image ships for
	// that in-tree package, never from a manifest on the tape alone.
	FirstParty bool
	// Accepts are the prompt-bar intents (Desk.Accepts).
	Accepts []string
}

// NewAppDesk validates an app desk. It can only give an app desk less than a
// system desk has: a verb outside appDeskVerbs, a system desk's name, a
// missing taint class, a uid in the system range or without a memory budget
// is refused.
func NewAppDesk(spec AppDeskSpec) (Desk, error) {
	if !appName.MatchString(spec.Name) {
		return Desk{}, fmt.Errorf("app desk name %q must match app:<slug>", spec.Name)
	}
	slug := spec.Name[len("app:"):]
	if spec.FirstParty && !firstPartyAppSlugs[slug] {
		return Desk{}, fmt.Errorf("app desk %s: only the reserved first-party slugs %v are registered first-party", spec.Name, sortedKeys(firstPartyAppSlugs))
	}
	if IsReservedAppSlug(slug) && !spec.FirstParty {
		return Desk{}, fmt.Errorf("app desk %s: the name is reserved for a built-in or first-party desk or app", spec.Name)
	}
	if spec.Taint != TaintPrivileged && spec.Taint != TaintUntrustedInput {
		return Desk{}, fmt.Errorf("app desk %s: taint class is required", spec.Name)
	}
	if spec.UID < 62000 {
		return Desk{}, fmt.Errorf("app desk %s: uid %d is outside the app range (62000 and up)", spec.Name, spec.UID)
	}
	if spec.WorkerMaxBytes == 0 || spec.WorkerMaxBytes > 512*mib {
		return Desk{}, fmt.Errorf("app desk %s: worker memory budget must be set and at most 512 MiB", spec.Name)
	}
	for _, v := range spec.Verbs {
		if !appDeskVerbs[v] {
			return Desk{}, fmt.Errorf("app desk %s: verb %s is not available to app desks", spec.Name, v)
		}
	}
	for _, k := range spec.LeaseKinds {
		if len(k) <= len("choir.") || k[:len("choir.")] != "choir." || k == "choir.lease" || k == "choir.trust_anchor" || k == "choir.trust_seed" {
			return Desk{}, fmt.Errorf("app desk %s: lease ceiling kind %q is not a leasable object kind", spec.Name, k)
		}
	}
	for _, g := range spec.LeaseGrants {
		if !leaseGrantName.MatchString(g) {
			return Desk{}, fmt.Errorf("app desk %s: lease ceiling grant %q is not a grant name", spec.Name, g)
		}
	}
	if err := validAccepts(spec.Accepts); err != nil {
		return Desk{}, fmt.Errorf("app desk %s: accepts: %w", spec.Name, err)
	}
	d := Desk{
		Name: spec.Name, Kind: KindApp, Carrier: CarrierDeskWorker,
		Verbs: append([]string{}, spec.Verbs...), CapsuleVerbs: []string{},
		Sandbox:    Sandbox{Profile: "desk_cell", UID: spec.UID},
		Taint:      spec.Taint,
		Memory:     Memory{WorkerMaxBytes: spec.WorkerMaxBytes},
		OwnedKinds: append([]string{}, spec.OwnedKinds...), ModelRole: spec.Name, Restart: RestartContract,
		LeaseKinds: append([]string{}, spec.LeaseKinds...), LeaseCaptures: spec.LeaseCaptures,
		LeaseGrants: append([]string{}, spec.LeaseGrants...),
		FirstParty:  spec.FirstParty,
		Accepts:     append([]string(nil), spec.Accepts...),
		Profile:     true,
	}
	if len(d.Accepts) == 0 {
		d.Accepts = nil
	}
	grants, err := GrantsForVerbs(spec.Verbs)
	if err != nil {
		return Desk{}, fmt.Errorf("app desk %s: %w", spec.Name, err)
	}
	d.Grants = grants
	if err := d.resolve(); err != nil {
		return Desk{}, err
	}
	if err := validateStructure(d); err != nil {
		return Desk{}, err
	}
	return d, nil
}

// RegisterApp adds an app desk. It is the registry's only door for a row
// from outside the built-in table, and it applies the whole app discipline:
// the row must be exactly what NewAppDesk builds from the row's own spec (name
// grammar and reserved slugs, uid range, memory budget, appDeskVerbs, no
// capsule verbs, no module, prompt, identity, channel or deadline authority).
// A row with any more authority is refused, never trimmed. It also refuses a
// name that exists, so an app can never replace a built-in desk or another app.
// ARCH-03 calls it at package install.
func RegisterApp(d Desk) error {
	if d.Kind != KindApp {
		return fmt.Errorf("only app desks register: %s is %s", d.Name, d.Kind)
	}
	canonical, err := NewAppDesk(d.AppSpec())
	if err != nil {
		return err
	}
	if !d.Equal(canonical) {
		return fmt.Errorf("app desk %s: the row carries authority an app desk does not have (carrier, sandbox, capsule verbs, modules, prompt, identity, channel, deadline or profile differ from what NewAppDesk builds)", d.Name)
	}
	return register(canonical)
}

// RetireApp removes an app desk (ARCH-03: a disabled package's desk). A
// system desk is never removed. After it, every consumer treats the name as
// unknown, which is deny.
func RetireApp(name string) error {
	mu.Lock()
	d, ok := registry[name]
	if !ok {
		mu.Unlock()
		return fmt.Errorf("desk %s is not registered", name)
	}
	if d.Kind != KindApp {
		mu.Unlock()
		return fmt.Errorf("only app desks retire: %s is %s", name, d.Kind)
	}
	delete(registry, name)
	// Activations that started under the row keep resolving against it
	// (epoch.go); the live epoch ends with the row.
	delete(current, name)
	observers := append([]func(string){}, retireObservers...)
	mu.Unlock()
	for _, f := range observers {
		f(name)
	}
	return nil
}

var retireObservers []func(name string)

// OnRetireApp registers f to be called with the name of every app desk
// RetireApp removes (an uninstall, a yank the computer applied, a package
// disabled). The lease service revokes the desk's leases through it (MA3.4).
// It returns the function that unregisters f.
func OnRetireApp(f func(name string)) (remove func()) {
	mu.Lock()
	defer mu.Unlock()
	retireObservers = append(retireObservers, f)
	i := len(retireObservers) - 1
	return func() {
		mu.Lock()
		defer mu.Unlock()
		if i < len(retireObservers) {
			retireObservers[i] = func(string) {}
		}
	}
}

// AppSpec renders an app desk back to the spec NewAppDesk builds it from. A
// desk worker is re-executed with a fresh registry, so the daemon hands it
// this spec and the worker rebuilds the row through NewAppDesk's checks
// (docs/problems/app-desk-registration-does-not-cross-worker-execve-2026-10-21.md).
func (d Desk) AppSpec() AppDeskSpec {
	c := d.clone()
	return AppDeskSpec{Name: c.Name, Verbs: c.Verbs, Taint: c.Taint, UID: c.Sandbox.UID,
		WorkerMaxBytes: c.Memory.WorkerMaxBytes, OwnedKinds: c.OwnedKinds,
		LeaseKinds: c.LeaseKinds, LeaseCaptures: c.LeaseCaptures, LeaseGrants: c.LeaseGrants,
		FirstParty: c.FirstParty, Accepts: c.Accepts}
}

// Bounds of an app desk's accepts (the prompt router's intents; the package
// manifest's full shape is promptroute.Accept).
const (
	maxAcceptIntents     = 64
	maxAcceptIntentRunes = 200
)

// validAccepts checks an app desk's prompt-bar intents: at most 64, each 1 to
// 200 characters with no control character, none twice.
func validAccepts(intents []string) error {
	if len(intents) > maxAcceptIntents {
		return fmt.Errorf("at most %d intents", maxAcceptIntents)
	}
	seen := map[string]bool{}
	for _, in := range intents {
		n := utf8.RuneCountInString(in)
		if n == 0 || n > maxAcceptIntentRunes || strings.TrimSpace(in) != in {
			return fmt.Errorf("intent %q must be 1-%d characters with no leading or trailing space", in, maxAcceptIntentRunes)
		}
		for _, r := range in {
			if r < 0x20 || r == 0x7f {
				return fmt.Errorf("intent %q has a control character", in)
			}
		}
		if seen[in] {
			return fmt.Errorf("intent %q is declared twice", in)
		}
		seen[in] = true
	}
	return nil
}

// OwnerInstance is the inbox and the persistent agent id of a per_owner desk
// for one owner: <desk>:<owner> (MA4.9). The owner id is trimmed; an empty
// owner, or a desk that is not per_owner, has no instance, so two owners
// never share one id and no caller falls back to a bare desk name.
func (d Desk) OwnerInstance(owner string) (string, bool) {
	owner = strings.TrimSpace(owner)
	if d.Instance != InstancePerOwner || owner == "" {
		return "", false
	}
	return d.Name + ":" + owner, true
}

// Equal reports whether two rows grant the same authority. A nil and an
// empty list are equal.
func (d Desk) Equal(o Desk) bool {
	same := func(a, b []string) bool {
		if len(a) != len(b) {
			return false
		}
		for i := range a {
			if a[i] != b[i] {
				return false
			}
		}
		return true
	}
	return d.Name == o.Name && d.Kind == o.Kind && d.Carrier == o.Carrier &&
		same(d.Verbs, o.Verbs) && same(d.CapsuleVerbs, o.CapsuleVerbs) &&
		d.Instance == o.Instance && d.Sandbox == o.Sandbox && d.Taint == o.Taint && d.Memory == o.Memory &&
		same(d.OwnedKinds, o.OwnedKinds) && same(d.LeaseKinds, o.LeaseKinds) && d.LeaseCaptures == o.LeaseCaptures && same(d.LeaseGrants, o.LeaseGrants) && d.FirstParty == o.FirstParty && d.ModelRole == o.ModelRole && d.Restart == o.Restart &&
		d.Profile == o.Profile && d.Prompt == o.Prompt && d.Identity == o.Identity && d.OwnChannel == o.OwnChannel &&
		d.CellDeadline == o.CellDeadline && same(d.Modules, o.Modules) &&
		sameGrants(d.Grants, o.Grants) && d.EndsRun == o.EndsRun
}
