package yaegikernel

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
)

// Desk trust classes (MA6.4, owner ruling R40): which desk workers may share
// one gVisor sandbox. The class is fixed by publisher and exposure, never by a
// trust score (trust governs leases, not sandboxes):
//
//   - A: first-party custodians, the privileged first-party desks (management,
//     its tasks, Texture, the Mail desk);
//   - B: first-party desks that read untrusted content (research, its web and
//     Texture children, source intake);
//   - C: one sandbox per other publisher. This computer's own generated desks
//     are stranger code and count as their own publisher.
//
// A desk the manifest does not name has no class: it is not a desk, and the
// root spawner refuses it before a class is asked for.

// DeskTrustClass is one sandbox-sharing class.
type DeskTrustClass string

const (
	TrustClassFirstPartyCustodian DeskTrustClass = "a"
	TrustClassFirstPartyUntrusted DeskTrustClass = "b"
	TrustClassPublisher           DeskTrustClass = "c"
)

// deskSandboxClass is the sandbox a desk's workers share: its class, the key
// that names the one sandbox (the class itself for A and B; the class and the
// publisher for C), and whether the desk is first-party.
type deskSandboxClass struct {
	Class      DeskTrustClass
	Key        string
	FirstParty bool
}

// classifyDesk returns the desk's sandbox class; ok is false for a role the
// manifest does not name as a desk-worker desk.
//
// Publisher of a non-first-party desk: the desk manifest carries no publisher
// field yet, so the key is the app desk's own slug, one sandbox per app desk.
// That never puts two publishers in one sandbox (it can only split one
// publisher's desks over several, at about 49 MiB each); a publisher field on
// the manifest row narrows it to one per publisher (MA6.4 residual).
func classifyDesk(role string) (deskSandboxClass, bool) {
	d, ok := deskmanifest.Lookup(role)
	if !ok || d.Carrier != deskmanifest.CarrierDeskWorker {
		return deskSandboxClass{}, false
	}
	firstParty := d.Kind != deskmanifest.KindApp || d.FirstParty
	if !firstParty {
		return deskSandboxClass{Class: TrustClassPublisher, Key: publisherSandboxKey(d.Name)}, true
	}
	if d.Taint == deskmanifest.TaintUntrustedInput {
		return deskSandboxClass{Class: TrustClassFirstPartyUntrusted, Key: string(TrustClassFirstPartyUntrusted), FirstParty: true}, true
	}
	return deskSandboxClass{Class: TrustClassFirstPartyCustodian, Key: string(TrustClassFirstPartyCustodian), FirstParty: true}, true
}

// DeskTrustClassOf is the trust class a desk's workers run in: "a", "b" or "c"
// (a non-first-party package desk). ok is false for a role that is no
// desk-worker desk.
func DeskTrustClassOf(role string) (DeskTrustClass, bool) {
	cls, ok := classifyDesk(role)
	return cls.Class, ok
}

// publisherSandboxKey is a container-id-safe key for a class C desk: the
// slug's readable form (lower-case letters, digits and dashes, at most 40
// bytes) and a hash of the exact name, so two names that read alike never
// share a sandbox.
func publisherSandboxKey(name string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(strings.TrimPrefix(name, "app:")) {
		if b.Len() >= 40 {
			break
		}
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == '_' || r == '.':
			b.WriteByte('-')
		}
	}
	sum := sha256.Sum256([]byte(name))
	return string(TrustClassPublisher) + "-" + b.String() + "-" + hex.EncodeToString(sum[:4])
}
