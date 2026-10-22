// Package appnet is an app package's network declaration: the egress its
// desk asks for and the ingress it may receive (SDC NET-10; design and
// build plan in docs/definitions/choir-launch-fp10-net10-2026-10-22.md).
//
// A declaration never grants. Each egress entry becomes a requested NET-05
// grant (internal/egressgrant) that only an owner act decides, at level L1
// through the host fetch path; each ingress entry names a host service from
// a closed list and an address in the package's namespace, never a port.
// Validation is fail closed: an unknown field, a source with no host
// service, a credential the platform does not hold, or an effect that needs
// per-use confirmation is refused. Widening reports every change that is
// not provably narrower, so an upgrade cannot widen silently.
//
// The package is pure and unwired: nothing imports it yet. CHOIR_APP_NETWORK
// is off unless it is exactly "declare"; off refuses any non-empty
// declaration, which is what internal/appdesk does today.
package appnet

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/egressgrant"
)

// Mode gates whether a declaration may be admitted at all.
type Mode string

const (
	ModeOff     Mode = "off"
	ModeDeclare Mode = "declare"
	// EnvMode is read once by whoever installs; only "declare" turns it on.
	EnvMode = "CHOIR_APP_NETWORK"
)

// ModeFromEnv reads EnvMode. Anything but "declare" is off.
func ModeFromEnv(getenv func(string) string) Mode {
	if getenv == nil {
		getenv = os.Getenv
	}
	if getenv(EnvMode) == string(ModeDeclare) {
		return ModeDeclare
	}
	return ModeOff
}

// Ingress sources. Each is terminated by one host service; a new source is
// a new host service, reviewed as one.
const SourceMail = "mail"

var ingressSources = map[string]string{SourceMail: "maild"}

// Caps.
const (
	MaxEgressEntries  = 16
	MaxIngressEntries = 8
	MaxRenewalDays    = 90
)

// Policy is the network member of an app desk manifest.
type Policy struct {
	Egress  []EgressEntry  `json:"egress,omitempty"`
	Ingress []IngressEntry `json:"ingress,omitempty"`
}

// EgressEntry is a NET-05 app-desk declaration without the fields the
// platform fills in (holder, computer, level, window).
type EgressEntry struct {
	Verb              string                    `json:"verb"`
	Destinations      []egressgrant.Destination `json:"destinations"`
	Methods           []string                  `json:"methods,omitempty"`
	EffectClass       string                    `json:"effect_class"`
	OperationContract string                    `json:"operation_contract,omitempty"`
	DataCeiling       string                    `json:"data_ceiling"`
	Credential        string                    `json:"credential,omitempty"`
	Budget            egressgrant.Budget        `json:"budget"`
	RenewalDays       int                       `json:"renewal_days"`
}

// IngressEntry names a source and an address. Authenticity, taint, record
// shape, batching and volume are fixed by the host service.
type IngressEntry struct {
	Source  string `json:"source"`
	Address string `json:"address"`
}

// Code is a typed refusal.
type Code string

const (
	CodeOff            Code = "network_off"
	CodePackage        Code = "bad_package"
	CodeVerb           Code = "bad_verb"
	CodeDuplicate      Code = "duplicate"
	CodeTooMany        Code = "too_many"
	CodeEffect         Code = "effect_class"
	CodeCeiling        Code = "data_ceiling"
	CodeCredential     Code = "credential_unknown"
	CodeRenewal        Code = "renewal"
	CodeGrant          Code = "grant_invalid"
	CodeIngressSource  Code = "ingress_source"
	CodeIngressAddress Code = "ingress_address"
)

// Problem is one reason a declaration may not be installed.
type Problem struct {
	Code   Code
	Key    string
	Detail string
}

var (
	// packageName is internal/appdesk's package grammar.
	packageName = regexp.MustCompile(`^[a-z][a-z0-9-]{1,31}$`)
	verbSuffix  = regexp.MustCompile(`^[a-z][a-z0-9_]{0,31}$`)
	// addrSuffix follows a "." after the package name. Package names never
	// contain ".", and neither does a suffix, so an address names exactly
	// one package: the text before its first ".". maild keeps "+" for
	// secret aliases, so "+" is not the separator.
	addrSuffix = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,30}$`)
)

// credentials is the registry of platform-held credentials an entry may
// name. Empty: no credentialed app egress until the gateway injects secrets
// (build plan W7).
var credentials = map[string]bool{}

var effectRank = map[string]int{
	egressgrant.EffectRead:            0,
	egressgrant.EffectIdempotentWrite: 1,
	egressgrant.EffectVisibleWrite:    2,
}

var ceilingRank = map[string]int{"public": 0, "owner-private": 1, "third-party-private": 2}

// Parse decodes a policy, refusing unknown fields and trailing data, so a
// port, listener, forward or tunnel is an error rather than ignored.
func Parse(raw []byte) (Policy, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var p Policy
	if err := dec.Decode(&p); err != nil {
		return Policy{}, fmt.Errorf("appnet: %w", err)
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return Policy{}, errors.New("appnet: trailing data after the policy")
	}
	return p, nil
}

// Admit is the install-time gate: with the mode off, any non-empty policy is
// refused; with it on, the policy is validated.
func Admit(mode Mode, pkg string, p Policy) []Problem {
	if len(p.Egress) == 0 && len(p.Ingress) == 0 {
		return nil
	}
	if mode != ModeDeclare {
		return []Problem{{Code: CodeOff, Detail: fmt.Sprintf("%s is not %q", EnvMode, ModeDeclare)}}
	}
	return Validate(pkg, p)
}

// probeComputer and probeTime let Validate run the grant validator before a
// computer or window exists.
const probeComputer = "appnet-validate"

var probeTime = time.Unix(0, 0).UTC()

// Validate is pure: every reason the policy may not be installed for pkg.
func Validate(pkg string, p Policy) []Problem {
	var out []Problem
	refuse := func(c Code, key, format string, args ...any) {
		out = append(out, Problem{Code: c, Key: key, Detail: fmt.Sprintf(format, args...)})
	}
	if !packageName.MatchString(pkg) {
		refuse(CodePackage, pkg, "package name %q must match %s", pkg, packageName)
	}
	if len(p.Egress) > MaxEgressEntries {
		refuse(CodeTooMany, "", "%d egress entries, at most %d", len(p.Egress), MaxEgressEntries)
	}
	if len(p.Ingress) > MaxIngressEntries {
		refuse(CodeTooMany, "", "%d ingress entries, at most %d", len(p.Ingress), MaxIngressEntries)
	}
	verbs := map[string]bool{}
	for _, e := range p.Egress {
		key := e.Verb
		if suffix, ok := strings.CutPrefix(e.Verb, pkg+"."); !ok || !verbSuffix.MatchString(suffix) {
			refuse(CodeVerb, key, "verb %q is not %s.<name>", e.Verb, pkg)
		}
		if verbs[e.Verb] {
			refuse(CodeDuplicate, key, "verb %q is declared twice", e.Verb)
		}
		verbs[e.Verb] = true
		switch e.EffectClass {
		case egressgrant.EffectRead, egressgrant.EffectIdempotentWrite:
		case egressgrant.EffectVisibleWrite:
			if strings.TrimSpace(e.OperationContract) == "" {
				refuse(CodeEffect, key, "visible-write needs an operation contract")
			}
		case egressgrant.EffectMoney, egressgrant.EffectIrreversible:
			refuse(CodeEffect, key, "effect class %s needs per-use owner confirmation, which does not exist", e.EffectClass)
		default:
			refuse(CodeEffect, key, "effect class %q", e.EffectClass)
		}
		if e.EffectClass == egressgrant.EffectRead {
			// A read is GET and HEAD only. POST is dual-use (a POST to
			// calendar/v3 creates events) and OPTIONS is refused
			// conservatively; a read-over-POST API is a new effect class.
			for _, m := range e.Methods {
				switch strings.ToUpper(strings.TrimSpace(m)) {
				case "GET", "HEAD":
				default:
					refuse(CodeEffect, key, "a read entry lists %s; a read is GET and HEAD only", m)
				}
			}
		}
		if _, ok := ceilingRank[e.DataCeiling]; !ok {
			refuse(CodeCeiling, key, "data ceiling %q", e.DataCeiling)
		}
		if e.Credential != "" && !credentials[e.Credential] {
			refuse(CodeCredential, key, "credential %q is not held by the platform", e.Credential)
		}
		if e.RenewalDays < 1 || e.RenewalDays > MaxRenewalDays {
			refuse(CodeRenewal, key, "renewal %d days, want 1 to %d", e.RenewalDays, MaxRenewalDays)
			continue
		}
		if _, err := egressgrant.Request(declaration(pkg, probeComputer, e, probeTime), probeTime); err != nil {
			refuse(CodeGrant, key, "%v", err)
		}
	}
	ingress := map[string]bool{}
	for _, in := range p.Ingress {
		key := in.Source + ":" + in.Address
		if _, ok := ingressSources[in.Source]; !ok {
			refuse(CodeIngressSource, key, "source %q has no host service", in.Source)
		}
		if !addressInNamespace(pkg, in.Address) {
			refuse(CodeIngressAddress, key, "address %q is not %s or %s.<suffix>", in.Address, pkg, pkg)
		}
		if ingress[key] {
			refuse(CodeDuplicate, key, "ingress %s is declared twice", key)
		}
		ingress[key] = true
	}
	return out
}

func addressInNamespace(pkg, addr string) bool {
	if addr == pkg {
		return packageName.MatchString(pkg)
	}
	suffix, ok := strings.CutPrefix(addr, pkg+".")
	return ok && addrSuffix.MatchString(suffix)
}

// DeskRef is the grant holder ref of a package's desk (appdesk.DeskName).
func DeskRef(pkg string) string { return "app:" + pkg }

func declaration(pkg, computerID string, e EgressEntry, now time.Time) egressgrant.Declaration {
	return egressgrant.Declaration{
		Holder:            egressgrant.Holder{Kind: egressgrant.HolderAppDesk, Ref: DeskRef(pkg)},
		ComputerID:        computerID,
		Level:             egressgrant.LevelL1,
		Destinations:      slices.Clone(e.Destinations),
		Methods:           slices.Clone(e.Methods),
		Budget:            e.Budget,
		NotBefore:         now,
		ExpiresAt:         now.Add(time.Duration(e.RenewalDays) * 24 * time.Hour),
		Verb:              e.Verb,
		EffectClass:       e.EffectClass,
		OperationContract: e.OperationContract,
		DataCeiling:       e.DataCeiling,
		Credential:        e.Credential,
	}
}

// Requests turns an admitted policy's egress entries into requested grants
// for computerID, valid from now for each entry's renewal period. It admits
// the policy under mode first, so with the mode off it mints nothing. A
// requested grant allows nothing until an owner act decides it.
func Requests(mode Mode, pkg, computerID string, p Policy, now time.Time) ([]egressgrant.Grant, error) {
	if ps := Admit(mode, pkg, p); len(ps) > 0 {
		return nil, fmt.Errorf("appnet: policy for %s is invalid: %s %s", pkg, ps[0].Code, ps[0].Detail)
	}
	out := make([]egressgrant.Grant, 0, len(p.Egress))
	for _, e := range p.Egress {
		g, err := egressgrant.Request(declaration(pkg, computerID, e, now), now)
		if err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, nil
}

// Change is one entry's difference between an approved policy and an
// upgrade.
type Change struct {
	Section string // "egress" or "ingress"
	Key     string // the verb, or source:address
	Widens  bool
	Reasons []string
}

// Widens reports whether next asks for anything old did not approve.
func Widens(old, next Policy) bool {
	for _, c := range Widening(old, next) {
		if c.Widens {
			return true
		}
	}
	return false
}

// Widening compares an approved policy with an upgrade, entry by entry. An
// entry is narrower only if every field is provably narrower; anything else,
// including an entry the grant validator refuses, widens.
func Widening(old, next Policy) []Change {
	var out []Change
	oldEgress := map[string]EgressEntry{}
	for _, e := range old.Egress {
		oldEgress[e.Verb] = e
	}
	seen := map[string]bool{}
	for _, e := range next.Egress {
		seen[e.Verb] = true
		prev, ok := oldEgress[e.Verb]
		if !ok {
			out = append(out, Change{Section: "egress", Key: e.Verb, Widens: true, Reasons: []string{"new verb"}})
			continue
		}
		if reasons := egressWidening(prev, e); len(reasons) > 0 {
			out = append(out, Change{Section: "egress", Key: e.Verb, Widens: true, Reasons: reasons})
		} else if !sameEntry(prev, e) {
			out = append(out, Change{Section: "egress", Key: e.Verb, Reasons: []string{"narrowed"}})
		}
	}
	for _, e := range old.Egress {
		if !seen[e.Verb] {
			out = append(out, Change{Section: "egress", Key: e.Verb, Reasons: []string{"removed"}})
		}
	}
	oldIngress := map[string]bool{}
	for _, in := range old.Ingress {
		oldIngress[in.Source+":"+in.Address] = true
	}
	nextIngress := map[string]bool{}
	for _, in := range next.Ingress {
		key := in.Source + ":" + in.Address
		nextIngress[key] = true
		if !oldIngress[key] {
			out = append(out, Change{Section: "ingress", Key: key, Widens: true, Reasons: []string{"new ingress"}})
		}
	}
	for _, in := range old.Ingress {
		if key := in.Source + ":" + in.Address; !nextIngress[key] {
			out = append(out, Change{Section: "ingress", Key: key, Reasons: []string{"removed"}})
		}
	}
	return out
}

func sameEntry(a, b EgressEntry) bool {
	x, err1 := json.Marshal(a)
	y, err2 := json.Marshal(b)
	return err1 == nil && err2 == nil && bytes.Equal(x, y)
}

// egressWidening returns why next is wider than prev, or nil if it is
// provably no wider. Both are normalized by the grant validator first, so
// host case, default ports and default methods compare as the enforcer sees
// them.
func egressWidening(prev, next EgressEntry) []string {
	const pkg, computer = "probe", probeComputer
	normal := func(e EgressEntry) (egressgrant.Declaration, bool) {
		e.Verb = pkg + ".probe"
		g, err := egressgrant.Request(declaration(pkg, computer, e, probeTime), probeTime)
		return g.Declaration, err == nil
	}
	p, okPrev := normal(prev)
	n, okNext := normal(next)
	if !okNext {
		return []string{"the new entry is not a valid grant"}
	}
	if !okPrev {
		return []string{"the approved entry is not a valid grant"}
	}
	var reasons []string
	for _, d := range n.Destinations {
		if !slices.ContainsFunc(p.Destinations, func(o egressgrant.Destination) bool { return covers(o, d) }) {
			reasons = append(reasons, fmt.Sprintf("destination %s://%s:%d%s is not under an approved one", d.Scheme, d.Host, d.Port, d.PathPrefix))
		}
	}
	for _, m := range n.Methods {
		if !slices.Contains(p.Methods, m) {
			reasons = append(reasons, "method "+m)
		}
	}
	if rank(effectRank, n.EffectClass) > rank(effectRank, p.EffectClass) {
		reasons = append(reasons, "effect class "+n.EffectClass)
	}
	if rank(ceilingRank, n.DataCeiling) > rank(ceilingRank, p.DataCeiling) {
		reasons = append(reasons, "data ceiling "+n.DataCeiling)
	}
	if n.Credential != p.Credential {
		reasons = append(reasons, "credential changed")
	}
	if n.OperationContract != p.OperationContract {
		reasons = append(reasons, "operation contract changed")
	}
	if n.Budget.MaxRequests > p.Budget.MaxRequests || n.Budget.MaxDownloadBytes > p.Budget.MaxDownloadBytes || n.Budget.MaxUploadBytes > p.Budget.MaxUploadBytes {
		reasons = append(reasons, "budget raised")
	}
	if next.RenewalDays > prev.RenewalDays {
		reasons = append(reasons, "renewal longer")
	}
	return reasons
}

// rank treats an unknown value as wider than every known one.
func rank(ranks map[string]int, v string) int {
	if r, ok := ranks[v]; ok {
		return r
	}
	return len(ranks) + 1
}

// covers reports whether the approved destination o admits everything d
// does: same scheme, host and port, and d's path prefix under o's.
func covers(o, d egressgrant.Destination) bool {
	if o.Scheme != d.Scheme || o.Host != d.Host || o.Port != d.Port {
		return false
	}
	if o.PathPrefix == "" {
		return true
	}
	if d.PathPrefix == "" {
		return false
	}
	if d.PathPrefix == o.PathPrefix {
		return true
	}
	if strings.HasSuffix(o.PathPrefix, "/") {
		return strings.HasPrefix(d.PathPrefix, o.PathPrefix)
	}
	return strings.HasPrefix(d.PathPrefix, o.PathPrefix+"/")
}
