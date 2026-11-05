// Package kernelservice is the manifest of the kernel's services (modular
// architecture plan MA3.1, section 8.2).
//
// A desk's manifest declares the object kinds the desk is the custodian of
// (deskmanifest.Desk.OwnedKinds). A kernel service is not a desk, but it
// writes kinds too: the commons every desk shares, the publication layer,
// the owner's files, the owner-act shell state. Registering a service here
// is the declaration its kernel provider makes, and it is the same shape: a
// name and the kinds the service is the only writer of. The object graph
// derives each kind's custodian from the desk manifests and from these
// service manifests, and refuses to load when a kind has none or two.
//
// The package imports only the standard library, as deskmanifest does, so the
// object graph can read it. Kinds are plain strings with the spelling of
// objectgraph.ObjectKind; the object graph's load check pins that each one is
// a registered kind.
package kernelservice

import (
	"fmt"
	"regexp"
	"sort"
	"sync"
)

// Service names, in the spelling of the writers they derive
// (objectgraph.Writer "system:<name>").
const (
	// Kernel is the commons: the precommitment-record substrate, the
	// ledger, inboxes, wake outbox and run bookkeeping every desk shares.
	Kernel = "kernel"
	// Corpusd is the publication layer (Store B) and the world's content.
	Corpusd = "corpusd"
	// Store is the computer's store acting for itself, not for a desk.
	Store = "store"
	// OwnerFiles is the owner's files and file objects.
	OwnerFiles = "ownerfiles"
	// Shell is the shell and owner-act state: settings, content, the
	// desktop.
	Shell = "shell"
	// Lease is the lease service (MA3.4, R20): the custodian-issued grants
	// that let a desk read a kind or capture it does not keep. Its records are
	// readable by the owner and the service only.
	Lease = "lease"
	// Autoradio is the radio's media kinds; reserved, no code writes them.
	Autoradio = "autoradio"
)

// Manifest is one kernel service's declaration.
type Manifest struct {
	Name string
	// OwnedKinds are the object kinds the service is the only writer of.
	OwnedKinds []string
}

var serviceName = regexp.MustCompile(`^[a-z][a-z0-9]{1,31}$`)

var (
	mu       sync.RWMutex
	registry = map[string]Manifest{}
)

// builtin is the table of the services the computer and the platform ship.
// Every service takes the one path, Register, at init.
var builtin = []Manifest{
	{Name: Kernel, OwnedKinds: []string{
		// Runs, agents, events and the work they leave.
		"choir.agent", "choir.run", "choir.event", "choir.trajectory", "choir.work_item",
		"choir.run_acceptance", "choir.run_continuation", "choir.run_memory_entry",
		// Messages, inboxes and the lifecycle of commands.
		"choir.channel_message", "choir.coagent_mailbox", "choir.inbox_delivery", "choir.inbox_command",
		"choir.inbox_cursor", "choir.inbox_head", "choir.conversation_head",
		"choir.lifecycle_cancel_intent", "choir.lifecycle_command", "choir.lifecycle_event", "choir.lifecycle_sequence",
		// The ledger and the wake outbox.
		"choir.commitment_record", "choir.actor_wake_outbox", "choir.scheduler_sequence", "choir.migration_marker",
		// The engineering-assignment lifecycle is a store-mediated protocol
		// between desks, not one desk's data: open (management's act, with
		// engineering's rebase claim), bind (the assignment with engineering's
		// run, capability and capsule claims), report (engineering's report with
		// management's assignment and candidate) and cancel (management, with
		// engineering's report) each commit one batch that mixes the desks, and
		// the writer gate checks a batch against one writer. So the records
		// that ride in those batches are the commons (MA3.1, review B1).
		"choir.co_super_assignment", "choir.co_super_assignment_report", "choir.co_super_subject_candidate",
		"choir.co_super_capability_claim", "choir.co_super_capsule_claim", "choir.co_super_run_claim",
		"choir.rebase_notice_claim",
		// Install records and worker updates.
		"choir.package_install", "choir.release_adoption", "choir.worker_update",
		// The desk package event log (MA4.4).
		"choir.desk_package_event",
	}},
	{Name: Corpusd, OwnedKinds: []string{
		"choir.web_capture", "choir.subject", "choir.publication", "choir.publication_version",
		"choir.publication_proposal", "choir.public_route", "choir.artifact_manifest", "choir.artifact_blob",
		"choir.provenance_entity", "choir.provenance_activity", "choir.provenance_agent",
		"choir.consent_record", "choir.review_record", "choir.retrieval_source", "choir.retrieval_span",
		"choir.retrieval_manifest", "choir.publication_source_entity", "choir.publication_transclusion",
		"choir.publication_policy", "choir.verifier_attestation", "choir.publication_resolved_blocks",
		"choir.publication_check", "choir.support_judgement", "choir.entity", "choir.entity_review",
		"choir.entity_stopword", "choir.software_release", "choir.package_release", "choir.release_listing",
	}},
	{Name: Store, OwnedKinds: []string{"choir.stream_seq_lease"}},
	{Name: OwnerFiles, OwnedKinds: []string{"choir.file", "choir.file_path"}},
	{Name: Shell, OwnedKinds: []string{
		"choir.computer_setting", "choir.content_item", "choir.podcast_subscription", "choir.browser_session",
		"choir.desktop_session", "choir.desktop_app_instance",
	}},
	{Name: Lease, OwnedKinds: []string{"choir.lease"}},
	{Name: Autoradio, OwnedKinds: []string{
		"choir.media_item", "choir.audio_recording", "choir.transcript", "choir.autoradio_run_sheet",
	}},
}

func init() {
	for _, m := range builtin {
		if err := Register(m); err != nil {
			panic("kernelservice: " + err.Error())
		}
	}
}

// Register adds a service. It refuses a bad or existing name, so a service
// can never replace another. Whether its kinds exist, and whether another
// manifest claims one, is the object graph's load check.
func Register(m Manifest) error {
	if !serviceName.MatchString(m.Name) {
		return fmt.Errorf("service name %q must match %s", m.Name, serviceName)
	}
	mu.Lock()
	defer mu.Unlock()
	if _, exists := registry[m.Name]; exists {
		return fmt.Errorf("service %s is already registered", m.Name)
	}
	m.OwnedKinds = append([]string{}, m.OwnedKinds...)
	registry[m.Name] = m
	return nil
}

// All lists every registered service, sorted by name.
func All() []Manifest {
	mu.RLock()
	defer mu.RUnlock()
	out := make([]Manifest, 0, len(registry))
	for _, m := range registry {
		m.OwnedKinds = append([]string{}, m.OwnedKinds...)
		out = append(out, m)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}
