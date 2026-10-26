// Package deskmanifest is the one table of what each desk may read, write,
// call, spawn and spend (SDC ARCH-02, launch item L1-04).
//
// Before it, a desk's authority was written in six places that nobody read
// together: agentprofile.PolicyFor (message targets), capsule.RoleVerbSets
// (broker verbs), yaegikernel.NewChoirScope (handle actions) and
// deskModuleSets (verbs), agentcore recordsRoute (inbox routing) and
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
	"sync"
)

// Desk names, in the spelling the code already uses.
const (
	Management     = "management"
	Texture        = "texture"
	Research       = "research"
	Engineering    = "engineering"
	Conductor      = "conductor"
	Email          = "email"
	ManagementTask = "management_task"
	ResearchChild  = "research_child"
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
	// CarrierNone is a desk with no cell carrier (conductor, email).
	CarrierNone Carrier = "none"
)

// Taint is ARCH-05's class. A worker of one class never serves the other.
type Taint string

const (
	// TaintUntrustedInput desks read attacker-controlled content from the
	// world directly (research: web pages; the email desk: mail bodies).
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
	// MessageTargets are the desks this desk may message (agentprofile).
	MessageTargets []string
	// Addresses are the desks whose inboxes this desk may address by
	// records. Pinned against agentcore recordsRoute; not yet generated
	// from here.
	Addresses []string

	Sandbox Sandbox
	Taint   Taint
	Memory  Memory

	// OwnedKinds are the object kinds the desk is the only writer of.
	// Declared and parity-pinned; no consumer reads it yet (ARCH-04).
	OwnedKinds []string
	// ModelRole is the modelpolicy role key. Declared; no consumer yet.
	ModelRole string
	// Restart is the restart contract. Declared; every desk has the same.
	Restart string
}

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
	"Research":         {"research", false},
	"System1":          {"system1", false},
	"RelateStories":    {"relate_stories", false},
	"Relate":           {"relate", false},
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
		// Management delegates engineering work and reports; it does not
		// touch the filesystem (mutation is capsule-bound under
		// engineering). It reads the captures a capture arrival names and
		// never fetches the world.
		Verbs: []string{"Cast", "Ask", "Note", "Reply",
			"CancelAct", "CancelAssignment", "Escalate", "Precommit", "Report", "ReportPacket", "Resolve", "Disagreement",
			"ProductAPI", "ReadCapture", "Stories", "Feed", "Decline", "Verify", "Delegate", "CancelTask"},
		CapsuleVerbs:   []string{},
		MessageTargets: []string{Texture, Research},
		Addresses:      []string{Texture, Engineering},
		Sandbox:        Sandbox{Profile: "desk_cell", UID: 61001},
		Taint:          TaintPrivileged,
		Memory:         Memory{WorkerMaxBytes: 768 * mib},
		OwnedKinds:     []string{"assignment", "admission"},
		ModelRole:      Management, Restart: RestartContract,
	},
	{
		Name: Texture, Kind: KindSystem, Carrier: CarrierDeskWorker,
		// Texture writes its document with one verb, Write, and closes a
		// turn with Done, Wait or Reject. It is the sole writer of canonical
		// document revisions. It reads the captures its frame names; it
		// never fetches the world.
		Verbs:          []string{"Cast", "Ask", "Note", "ReadDoc", "ReadCapture", "Write", "Done", "Wait", "Reject"},
		MessageTargets: []string{Research, Management},
		Addresses:      []string{Research, Management},
		Sandbox:        Sandbox{Profile: "desk_cell", UID: 61002},
		Taint:          TaintPrivileged,
		Memory:         Memory{WorkerMaxBytes: 768 * mib},
		OwnedKinds:     []string{"document_revision"},
		ModelRole:      Texture, Restart: RestartContract,
	},
	{
		Name: Research, Kind: KindSystem, Carrier: CarrierDeskWorker,
		// Research observes the world read-only but has full message
		// authority: read-only world access is not read-only messaging.
		Verbs: []string{"Cast", "Ask", "Note", "Reply", "CancelAct",
			"Escalate", "Precommit", "Report", "ReportPacket", "Resolve", "Disagreement",
			"SearchStart", "Searches", "SearchWait", "Read", "ReadCapture", "SearchWireCorpus",
			"SaveEvidence", "ReadEvidence", "ListEvidence", "RunMemoryEntry", "Research", "PollSource", "SourceAPI",
			"System1", "RelateStories", "Relate", "StoryAnalysis"},
		CapsuleVerbs:   []string{"read_file", "list_dir", "stat", "go_eval"},
		MessageTargets: []string{Texture},
		Addresses:      []string{Texture},
		Sandbox:        Sandbox{Profile: "desk_cell", UID: 61003},
		Taint:          TaintUntrustedInput,
		Memory:         Memory{WorkerMaxBytes: 768 * mib},
		OwnedKinds:     []string{"evidence"},
		ModelRole:      Research, Restart: RestartContract,
	},
	{
		Name: Engineering, Kind: KindSystem, Carrier: CarrierCapsule,
		// Engineering mutates inside its capsule, converses with
		// management on records, and ends its assignment with Complete,
		// freezing a candidate first.
		Verbs: []string{"WriteFile", "Exec", "Start", "Poll", "Stop", "Complete", "Freeze", "Precommit", "Report", "Ask", "Reply", "Note"},
		CapsuleVerbs: []string{"exec", "read_file", "write_file", "list_dir", "stat", "mkdir", "remove",
			"kill_session", "go_eval", "init_session", "close_session", "job_start", "job_status", "job_stop"},
		MessageTargets: []string{Management},
		Addresses:      []string{Management},
		Sandbox:        Sandbox{Profile: "capsule"},
		Taint:          TaintPrivileged,
		// A capsule asks for what the cmd/autoputer link needs (measured
		// peak 1.50 GiB stripped, 1.80 GiB with DWARF).
		Memory:     Memory{CapsuleRequestedBytes: 2 << 30},
		OwnedKinds: []string{"capsule_effect_bundle"},
		ModelRole:  Engineering, Restart: RestartContract,
	},
	{
		Name: ManagementTask, Kind: KindRole, Carrier: CarrierDeskWorker,
		// A management task disposes of the captures its lease names. It
		// has no filesystem, assignment, message, product API or ledger
		// authority: one level of delegation.
		Verbs:          []string{"Cast", "Note", "ReadCapture", "Stories", "Feed", "Decline", "Done"},
		MessageTargets: []string{},
		Addresses:      []string{Texture},
		Sandbox:        Sandbox{Profile: "desk_cell", UID: 61001},
		Taint:          TaintPrivileged,
		Memory:         Memory{WorkerMaxBytes: 384 * mib},
		ModelRole:      ManagementTask, Restart: RestartContract,
	},
	{
		Name: ResearchChild, Kind: KindRole, Carrier: CarrierDeskWorker,
		// A child research activation reads the world and returns its
		// findings to the parent cell as a value. It cannot message,
		// report or write.
		Verbs: []string{"SearchStart", "Searches", "SearchWait", "Read", "ReadCapture", "SearchWireCorpus",
			"ReadEvidence", "ListEvidence"},
		MessageTargets: []string{},
		Addresses:      []string{},
		Sandbox:        Sandbox{Profile: "desk_cell", UID: 61003},
		Taint:          TaintUntrustedInput,
		Memory:         Memory{WorkerMaxBytes: 384 * mib},
		ModelRole:      ResearchChild, Restart: RestartContract,
	},
	{
		Name: Conductor, Kind: KindSystem, Carrier: CarrierNone,
		MessageTargets: []string{}, Addresses: []string{}, Sandbox: Sandbox{Profile: "none"},
		Taint: TaintPrivileged, ModelRole: Conductor, Restart: RestartContract,
	},
	{
		Name: Email, Kind: KindSystem, Carrier: CarrierNone,
		MessageTargets: []string{}, Addresses: []string{}, Sandbox: Sandbox{Profile: "none"},
		Taint: TaintUntrustedInput, ModelRole: Email, Restart: RestartContract,
	},
}

var (
	mu       sync.RWMutex
	registry = map[string]Desk{}
)

func init() {
	for _, d := range system {
		if _, dup := registry[d.Name]; dup {
			panic("deskmanifest: duplicate desk " + d.Name)
		}
		if err := validateStructure(d); err != nil {
			panic("deskmanifest: " + err.Error())
		}
		registry[d.Name] = d.clone()
	}
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
	d.MessageTargets = cp(d.MessageTargets)
	d.Addresses = cp(d.Addresses)
	d.OwnedKinds = cp(d.OwnedKinds)
	return d
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
// verbs dispatch. Never emit, never message.
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

func validateStructure(d Desk) error {
	if d.Name == "" {
		return fmt.Errorf("a desk has no name")
	}
	if d.Restart != RestartContract {
		return fmt.Errorf("desk %s: restart contract %q is not %q", d.Name, d.Restart, RestartContract)
	}
	if d.Taint == "" {
		return fmt.Errorf("desk %s: no taint class", d.Name)
	}
	for _, v := range d.Verbs {
		if v == "" {
			return fmt.Errorf("desk %s: empty verb", d.Name)
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
		if a, local, ok := VerbAction(v); ok && local && d.Name != Engineering && d.Kind != KindApp {
			if a == "write_file" || a == "exec" {
				return fmt.Errorf("desk %s: only engineering holds %s", d.Name, a)
			}
		}
	}
	return nil
}

var appName = regexp.MustCompile(`^app:[a-z][a-z0-9-]{0,31}$`)

// appDeskVerbs is every verb an app desk may be given: the semantic-act tier
// Texture, management and research share, and capture reads. No egress and no
// world verb: an app desk's egress is declared, owner-approved and added with
// NET-10, not taken from here.
var appDeskVerbs = map[string]bool{
	"Cast": true, "Ask": true, "Note": true, "Reply": true, "CancelAct": true, "Escalate": true,
	"Precommit": true, "Report": true, "ReportPacket": true, "Resolve": true, "Disagreement": true,
	"ReadCapture": true, "Done": true,
}

// AppDeskSpec is what an app package's desk manifest declares (ARCH-03).
type AppDeskSpec struct {
	Name           string
	Verbs          []string
	Taint          Taint
	UID            uint32
	WorkerMaxBytes uint64
	OwnedKinds     []string
}

// NewAppDesk validates an app desk. It can only give an app desk less than a
// system desk has: a verb outside appDeskVerbs, a system desk's name, a
// missing taint class, a uid in the system range or without a memory budget
// is refused.
func NewAppDesk(spec AppDeskSpec) (Desk, error) {
	if !appName.MatchString(spec.Name) {
		return Desk{}, fmt.Errorf("app desk name %q must match app:<slug>", spec.Name)
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
	d := Desk{
		Name: spec.Name, Kind: KindApp, Carrier: CarrierDeskWorker,
		Verbs: append([]string{}, spec.Verbs...), CapsuleVerbs: []string{},
		MessageTargets: []string{}, Addresses: []string{},
		Sandbox:    Sandbox{Profile: "desk_cell", UID: spec.UID},
		Taint:      spec.Taint,
		Memory:     Memory{WorkerMaxBytes: spec.WorkerMaxBytes},
		OwnedKinds: append([]string{}, spec.OwnedKinds...), ModelRole: spec.Name, Restart: RestartContract,
	}
	if err := validateStructure(d); err != nil {
		return Desk{}, err
	}
	return d, nil
}

// RegisterApp adds a validated app desk. It refuses a name that exists, so an
// app can never replace a system desk or another app. ARCH-03 calls it at
// package install.
func RegisterApp(d Desk) error {
	if d.Kind != KindApp {
		return fmt.Errorf("only app desks register: %s is %s", d.Name, d.Kind)
	}
	if err := validateStructure(d); err != nil {
		return err
	}
	for _, v := range d.Verbs {
		if !appDeskVerbs[v] {
			return fmt.Errorf("app desk %s: verb %s is not available to app desks", d.Name, v)
		}
	}
	mu.Lock()
	defer mu.Unlock()
	if _, exists := registry[d.Name]; exists {
		return fmt.Errorf("desk %s is already registered", d.Name)
	}
	registry[d.Name] = d.clone()
	return nil
}

// RetireApp removes an app desk (ARCH-03: a disabled package's desk). A
// system desk is never removed. After it, every consumer treats the name as
// unknown, which is deny.
func RetireApp(name string) error {
	mu.Lock()
	defer mu.Unlock()
	d, ok := registry[name]
	if !ok {
		return fmt.Errorf("desk %s is not registered", name)
	}
	if d.Kind != KindApp {
		return fmt.Errorf("only app desks retire: %s is %s", name, d.Kind)
	}
	delete(registry, name)
	return nil
}

// AppSpec renders an app desk back to the spec NewAppDesk builds it from. A
// desk worker is re-executed with a fresh registry, so the daemon hands it
// this spec and the worker rebuilds the row through NewAppDesk's checks
// (docs/problems/app-desk-registration-does-not-cross-worker-execve-2026-10-21.md).
func (d Desk) AppSpec() AppDeskSpec {
	c := d.clone()
	return AppDeskSpec{Name: c.Name, Verbs: c.Verbs, Taint: c.Taint, UID: c.Sandbox.UID,
		WorkerMaxBytes: c.Memory.WorkerMaxBytes, OwnedKinds: c.OwnedKinds}
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
		same(d.MessageTargets, o.MessageTargets) && same(d.Addresses, o.Addresses) &&
		d.Sandbox == o.Sandbox && d.Taint == o.Taint && d.Memory == o.Memory &&
		same(d.OwnedKinds, o.OwnedKinds) && d.ModelRole == o.ModelRole && d.Restart == o.Restart
}
