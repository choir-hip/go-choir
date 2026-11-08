package deskmanifest

import "strings"

// GrantCommitRevision is the sole-holder grant of the document custodian: the
// desk that writes canonical document revisions (today Texture). Code that
// once compared a profile to the Texture name, or built "texture:<doc>", asks
// this grant (MA4.11 follow-on; Phase 5 names gate).
const GrantCommitRevision = "commit.texture_revision"

// DocumentCustodianName is the registered name of the document custodian
// (commit.texture_revision), with the same fail-closed fallback as the other
// holder names: with no holder it is a name no desk and no address matches.
func DocumentCustodianName() string { return holderName(GrantCommitRevision) }

// IsDocumentCustodian reports whether the name is the document custodian.
func IsDocumentCustodian(name string) bool { return Holds(name, GrantCommitRevision) }

// DocumentInbox is the custodian's persistent inbox and agent id for one
// document: <custodian>:<docID>. The id is stored on the tape.
func DocumentInbox(docID string) string { return DocumentCustodianName() + ":" + docID }

// IsDocumentInbox reports whether the address is a document custodian inbox
// (the custodian's name, a colon, then anything).
func IsDocumentInbox(inbox string) bool { return strings.HasPrefix(inbox, DocumentCustodianName()+":") }

// DocumentIDFromInbox is the part of a custodian inbox after the prefix (no
// trimming, no emptiness check), or false for any other address.
func DocumentIDFromInbox(inbox string) (string, bool) {
	return strings.CutPrefix(inbox, DocumentCustodianName()+":")
}

// DocumentArtifactPrefix is the prefix of a document artifact reference:
// <custodian>://documents/.
func DocumentArtifactPrefix() string { return DocumentCustodianName() + "://documents/" }

// DocumentArtifact is the artifact reference of one document.
func DocumentArtifact(docID string) string { return DocumentArtifactPrefix() + docID }

// FrontDoorName is the registered name of the desk the owner's prompt first
// reaches: the one system desk with no cell carrier (it routes, it writes no
// Go). Code that once compared a profile to the conductor name asks this. With
// none registered it is a name no desk and no address matches.
func FrontDoorName() string {
	mu.RLock()
	defer mu.RUnlock()
	for n, d := range registry {
		if d.Kind == KindSystem && d.Carrier == CarrierNone {
			return n
		}
	}
	return unregistered + ":front-door"
}

// IsFrontDoor reports whether the name is the front-door desk.
func IsFrontDoor(name string) bool { return name == FrontDoorName() }

// TaskRoleName is the registered name of the run role the admitter delegates
// bounded tasks to: the role whose parent grant is admit.assignment. With none
// registered it is a name no desk and no address matches.
func TaskRoleName() string {
	mu.RLock()
	defer mu.RUnlock()
	for n, d := range registry {
		if d.Kind == KindRole && d.Parent == GrantAdmitAssignment {
			return n
		}
	}
	return unregistered + ":task-role"
}

// IsTaskRole reports whether the name is the admitter's task role.
func IsTaskRole(name string) bool { return name == TaskRoleName() }

// InResearchFamily reports whether the name is the research parent or one of
// the roles it spawns (a role whose parent grant is research.spawn).
func InResearchFamily(name string) bool {
	if SpawnsResearch(name) {
		return true
	}
	mu.RLock()
	defer mu.RUnlock()
	d, ok := registry[name]
	return ok && d.Kind == KindRole && d.Parent == GrantResearchSpawn
}

// CellDeskNames lists the system desks whose cells run in session workers,
// sorted.
func CellDeskNames() []string {
	return namesWhere(func(d Desk) bool { return d.Kind == KindSystem && d.Carrier == CarrierDeskWorker })
}

// CellRoleNames lists the roles whose cells run in session workers, sorted.
func CellRoleNames() []string {
	return namesWhere(func(d Desk) bool { return d.Kind == KindRole && d.Carrier == CarrierDeskWorker })
}
