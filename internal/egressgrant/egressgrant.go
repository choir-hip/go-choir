// Package egressgrant is the grant object for open egress (NET-05,
// docs/design/egress-grant-object-2026-10-21.md): one holder-typed record
// shared by engineering assignments (L1, L3, L4 on top of the L2 default)
// and app-desk manifest egress entries (NET-10 §2.1). A grant is validated
// fail closed, is time-limited, and is decided only by a verified owner-act
// receipt bound to its exact grant id. Revoking, expiring, exhausting and
// interrupting only narrow, so they need no owner act.
//
// The package is pure and unwired: nothing stores, serves or enforces a
// grant yet. The wiring is designed in §5 of the design doc.
package egressgrant

import (
	"errors"
	"fmt"
	"net/netip"
	"slices"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/owneract"
)

const (
	// Schema names the record version.
	Schema = "choir.egress_grant/v1"
	// IDPrefix prefixes every grant id.
	IDPrefix = "egress-grant:sha256:"
	// ActDecide is the owner-act kind of a decide. Its subject is exactly
	// {grant_id, decision}. It moves into internal/owneract when the proxy
	// mints it (design §5.2).
	ActDecide = "egress.grant.decide"
)

// Holder kinds.
const (
	HolderEngineeringAssignment = "engineering_assignment"
	HolderAppDesk               = "app_desk"
)

// Levels from the capability ladder (grants design §4.0a). L2 is the
// compiled-in default, never a grant.
const (
	LevelL1 = "L1" // fetch and mount
	LevelL3 = "L3" // open web through the proxy
	LevelL4 = "L4" // full network
)

// Dispositions.
const (
	DispositionRequested   = "requested"
	DispositionGranted     = "granted"
	DispositionDenied      = "denied"
	DispositionRevoked     = "revoked"
	DispositionExpired     = "expired"
	DispositionExhausted   = "exhausted"
	DispositionInterrupted = "interrupted"
)

// Verdicts an owner act may carry.
const (
	VerdictGrant = "grant"
	VerdictDeny  = "deny"
)

// ReasonRestart is the close reason a crash boot gives an undecided request.
const ReasonRestart = "interrupted by a restart"

// ConditionNoPrivateData is required on L4: full network only for capsules
// with no private data mounted (grants design §4.0a).
const ConditionNoPrivateData = "no_private_data_mounted"

var knownConditions = []string{ConditionNoPrivateData}

// Effect classes and data ceilings (NET-10 §2.1).
const (
	EffectRead            = "read"
	EffectIdempotentWrite = "idempotent-write"
	EffectVisibleWrite    = "visible-write"
	EffectMoney           = "money"
	EffectIrreversible    = "irreversible"
)

var effectClasses = []string{EffectRead, EffectIdempotentWrite, EffectVisibleWrite, EffectMoney, EffectIrreversible}

var dataCeilings = []string{"public", "owner-private", "third-party-private"}

var httpMethods = []string{"GET", "HEAD", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"}

// Hard caps; per-grant budgets are chosen inside them (design §7.3).
const (
	MaxRequestsCap        int64 = 100_000
	MaxDownloadBytesCap   int64 = 16 << 30
	MaxUploadBytesCap     int64 = 256 << 20
	MaxAssignmentLifetime       = 24 * time.Hour
	MaxAppDeskLifetime          = 90 * 24 * time.Hour
	maxDestinations             = 64
)

// Check errors.
var (
	ErrNotGranted  = errors.New("egress grant: not granted")
	ErrNotYetValid = errors.New("egress grant: not yet valid")
	ErrExpired     = errors.New("egress grant: expired")
	ErrOutOfScope  = errors.New("egress grant: use is outside the grant")
	ErrInvalid     = errors.New("egress grant: invalid")
)

// Holder is who the grant is for.
type Holder struct {
	Kind string `json:"kind"`
	Ref  string `json:"ref"` // assignment id or desk id
}

// Destination is one reachable endpoint. Host "*" is the wildcard (L3, L4,
// engineering only). Port 0 means the scheme default (443 for https) or,
// at L4, any port.
type Destination struct {
	Scheme     string `json:"scheme"`
	Host       string `json:"host"`
	Port       int    `json:"port"`
	PathPrefix string `json:"path_prefix,omitempty"`
}

// Budget bounds one grant's use.
type Budget struct {
	MaxRequests      int64 `json:"max_requests"`
	MaxDownloadBytes int64 `json:"max_download_bytes"`
	MaxUploadBytes   int64 `json:"max_upload_bytes"`
}

// Declaration is the request form: an engineering request_grant spec or an
// app-desk manifest egress entry. The grant id is its digest.
type Declaration struct {
	Holder       Holder        `json:"holder"`
	ComputerID   string        `json:"computer_id"`
	Level        string        `json:"level"`
	Purpose      string        `json:"purpose,omitempty"`
	Destinations []Destination `json:"destinations"`
	Methods      []string      `json:"methods,omitempty"`
	Budget       Budget        `json:"budget"`
	NotBefore    time.Time     `json:"not_before"`
	ExpiresAt    time.Time     `json:"expires_at"`
	Conditions   []string      `json:"conditions,omitempty"`

	// App-desk manifest fields (NET-10 §2.1); empty for engineering.
	Verb              string `json:"verb,omitempty"`
	EffectClass       string `json:"effect_class,omitempty"`
	OperationContract string `json:"operation_contract,omitempty"`
	DataCeiling       string `json:"data_ceiling,omitempty"`
	Credential        string `json:"credential,omitempty"`
}

// Decision is the owner act that decided the grant.
type Decision struct {
	Verdict           string    `json:"verdict"`
	OwnerUserID       string    `json:"owner_user_id"`
	OwnerActReceiptID string    `json:"owner_act_receipt_id"`
	OwnerActDigest    string    `json:"owner_act_digest"`
	At                time.Time `json:"at"`
}

// Grant is the record.
type Grant struct {
	Schema  string `json:"schema"`
	GrantID string `json:"grant_id"`
	Declaration
	Disposition string    `json:"disposition"`
	Decision    *Decision `json:"decision,omitempty"`
	RequestedAt time.Time `json:"requested_at"`
	ClosedAt    time.Time `json:"closed_at,omitzero"`
	CloseReason string    `json:"close_reason,omitempty"`
}

// Use is one attempted connection or request an enforcement point checks.
// Port 0 means the scheme default.
type Use struct {
	Level  string
	Scheme string
	Host   string
	Port   int
	Method string
	Path   string
}

// Request normalizes and validates a declaration and returns it as a
// requested grant.
func Request(d Declaration, now time.Time) (Grant, error) {
	d = normalize(d)
	if err := validateDeclaration(d); err != nil {
		return Grant{}, err
	}
	id, err := grantID(d)
	if err != nil {
		return Grant{}, err
	}
	return Grant{Schema: Schema, GrantID: id, Declaration: d, Disposition: DispositionRequested, RequestedAt: now.UTC().Truncate(time.Second)}, nil
}

// Validate re-checks a stored record: its declaration, its id, and that its
// disposition and decision agree. It cannot re-verify the owner act (the
// receipt's TTL has passed); host-side trust is the design's §5.5.
func (g Grant) Validate() error {
	if g.Schema != Schema {
		return invalid("schema %q", g.Schema)
	}
	if normalized := normalize(g.Declaration); !sameDeclaration(normalized, g.Declaration) {
		return invalid("declaration is not normalized")
	}
	if err := validateDeclaration(g.Declaration); err != nil {
		return err
	}
	id, err := grantID(g.Declaration)
	if err != nil {
		return err
	}
	if id != g.GrantID {
		return invalid("grant id does not match the declaration")
	}
	switch g.Disposition {
	case DispositionRequested, DispositionInterrupted:
		if g.Decision != nil {
			return invalid("%s grant carries a decision", g.Disposition)
		}
	case DispositionGranted, DispositionExpired, DispositionExhausted:
		if g.Decision == nil || g.Decision.Verdict != VerdictGrant {
			return invalid("%s grant has no grant decision", g.Disposition)
		}
	case DispositionDenied:
		if g.Decision == nil || g.Decision.Verdict != VerdictDeny {
			return invalid("denied grant has no deny decision")
		}
	case DispositionRevoked:
		if g.Decision != nil && g.Decision.Verdict != VerdictGrant {
			return invalid("revoked grant has a non-grant decision")
		}
	default:
		return invalid("disposition %q", g.Disposition)
	}
	if g.Decision != nil && (strings.TrimSpace(g.Decision.OwnerUserID) == "" || g.Decision.OwnerActReceiptID == "" || g.Decision.OwnerActDigest == "") {
		return invalid("decision has no owner act")
	}
	return nil
}

// Decide applies the owner's verdict. The receipt must verify under the
// host owner-act key for this computer, be of kind ActDecide, and carry
// exactly {grant_id, decision} for this grant and verdict. Only a requested
// grant can be decided, so a replayed receipt changes nothing.
func Decide(g Grant, verdict string, receipt computerevent.Receipt, verifier *owneract.Verifier, now time.Time) (Grant, error) {
	if err := g.Validate(); err != nil {
		return g, err
	}
	if g.Disposition != DispositionRequested {
		return g, invalid("cannot decide a %s grant", g.Disposition)
	}
	if verdict != VerdictGrant && verdict != VerdictDeny {
		return g, invalid("verdict %q", verdict)
	}
	if verifier == nil {
		return g, invalid("no owner-act verifier")
	}
	verified, err := verifier.Verify(receipt, g.ComputerID, ActDecide, now)
	if err != nil {
		return g, fmt.Errorf("%w: %v", ErrInvalid, err)
	}
	if len(verified.Subject) != 2 || verified.SubjectString("grant_id") != g.GrantID || verified.SubjectString("decision") != verdict {
		return g, invalid("owner act subject does not bind this grant and verdict")
	}
	now = now.UTC().Truncate(time.Second)
	if verdict == VerdictGrant && !now.Before(g.ExpiresAt) {
		return g, invalid("grant expired before it was decided")
	}
	out := g
	out.Decision = &Decision{
		Verdict: verdict, OwnerUserID: verified.OwnerUserID, OwnerActReceiptID: verified.ReceiptID,
		OwnerActDigest: verified.Digest, At: now,
	}
	if verdict == VerdictGrant {
		out.Disposition = DispositionGranted
	} else {
		out.Disposition = DispositionDenied
		out.ClosedAt = now
	}
	return out, nil
}

// Revoke closes a requested or granted grant. Revocation only narrows.
func Revoke(g Grant, reason string, now time.Time) (Grant, error) {
	return Close(g, DispositionRevoked, reason, now)
}

// Close moves a grant to a terminal, narrowing disposition: revoked from
// requested or granted; interrupted from requested (a crash boot closes an
// undecided request, never re-requesting it); expired or exhausted from
// granted. A closed grant never reopens.
func Close(g Grant, disposition, reason string, now time.Time) (Grant, error) {
	if err := g.Validate(); err != nil {
		return g, err
	}
	allowed := map[string][]string{
		DispositionRevoked:     {DispositionRequested, DispositionGranted},
		DispositionInterrupted: {DispositionRequested},
		DispositionExpired:     {DispositionGranted},
		DispositionExhausted:   {DispositionGranted},
	}[disposition]
	if !slices.Contains(allowed, g.Disposition) {
		return g, invalid("cannot close a %s grant as %q", g.Disposition, disposition)
	}
	out := g
	out.Disposition = disposition
	out.ClosedAt = now.UTC().Truncate(time.Second)
	out.CloseReason = strings.TrimSpace(reason)
	return out, nil
}

// Check allows a use only when the record validates, is granted, now is in
// [not_before, expires_at), and the use's level, scheme, host, port, path
// and method are covered. Enforcement points call it on every request.
func Check(g Grant, use Use, now time.Time) error {
	if err := g.Validate(); err != nil {
		return err
	}
	if g.Disposition != DispositionGranted {
		return fmt.Errorf("%w (%s)", ErrNotGranted, g.Disposition)
	}
	if now.Before(g.NotBefore) {
		return ErrNotYetValid
	}
	if !now.Before(g.ExpiresAt) {
		return ErrExpired
	}
	if use.Level != g.Level {
		return fmt.Errorf("%w: level %q", ErrOutOfScope, use.Level)
	}
	host := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(use.Host)), ".")
	if err := grantableHost(host); err != nil {
		return fmt.Errorf("%w: %v", ErrOutOfScope, err)
	}
	scheme := strings.ToLower(strings.TrimSpace(use.Scheme))
	port := use.Port
	if port == 0 && scheme == "https" {
		port = 443
	}
	if g.Level != LevelL4 {
		method := strings.ToUpper(strings.TrimSpace(use.Method))
		if !slices.Contains(g.Methods, method) {
			return fmt.Errorf("%w: method %q", ErrOutOfScope, method)
		}
	}
	for _, dest := range g.Destinations {
		if dest.Scheme != scheme || (dest.Host != "*" && dest.Host != host) || (dest.Port != 0 && dest.Port != port) {
			continue
		}
		if dest.PathPrefix != "" && !pathUnder(use.Path, dest.PathPrefix) {
			continue
		}
		return nil
	}
	return fmt.Errorf("%w: %s://%s:%d%s", ErrOutOfScope, scheme, host, port, use.Path)
}

func pathUnder(path, prefix string) bool {
	if path == prefix {
		return true
	}
	if strings.HasSuffix(prefix, "/") {
		return strings.HasPrefix(path, prefix)
	}
	return strings.HasPrefix(path, prefix+"/")
}

func normalize(d Declaration) Declaration {
	d.Holder.Kind = strings.TrimSpace(d.Holder.Kind)
	d.Holder.Ref = strings.TrimSpace(d.Holder.Ref)
	d.ComputerID = strings.TrimSpace(d.ComputerID)
	d.Level = strings.TrimSpace(d.Level)
	d.Purpose = strings.TrimSpace(d.Purpose)
	dests := make([]Destination, 0, len(d.Destinations))
	for _, dest := range d.Destinations {
		dest.Scheme = strings.ToLower(strings.TrimSpace(dest.Scheme))
		dest.Host = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(dest.Host)), ".")
		dest.PathPrefix = strings.TrimSpace(dest.PathPrefix)
		if dest.Port == 0 && dest.Scheme == "https" {
			dest.Port = 443
		}
		dests = append(dests, dest)
	}
	slices.SortFunc(dests, func(a, b Destination) int {
		return strings.Compare(a.Scheme+"\x00"+a.Host+"\x00"+fmt.Sprint(a.Port)+"\x00"+a.PathPrefix,
			b.Scheme+"\x00"+b.Host+"\x00"+fmt.Sprint(b.Port)+"\x00"+b.PathPrefix)
	})
	d.Destinations = slices.Compact(dests)
	if len(d.Destinations) == 0 {
		d.Destinations = nil
	}
	if d.Level != LevelL4 && len(d.Methods) == 0 {
		d.Methods = []string{"GET", "HEAD"}
	}
	d.Methods = sortedUnique(d.Methods, strings.ToUpper)
	d.Conditions = sortedUnique(d.Conditions, strings.ToLower)
	if !d.NotBefore.IsZero() {
		d.NotBefore = d.NotBefore.UTC().Truncate(time.Second)
	}
	if !d.ExpiresAt.IsZero() {
		d.ExpiresAt = d.ExpiresAt.UTC().Truncate(time.Second)
	}
	for _, field := range []*string{&d.Verb, &d.EffectClass, &d.OperationContract, &d.DataCeiling, &d.Credential} {
		*field = strings.TrimSpace(*field)
	}
	return d
}

func sortedUnique(values []string, fold func(string) string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		out = append(out, fold(strings.TrimSpace(value)))
	}
	slices.Sort(out)
	return slices.Compact(out)
}

func sameDeclaration(a, b Declaration) bool {
	left, err1 := computerevent.CanonicalJSON(a)
	right, err2 := computerevent.CanonicalJSON(b)
	return err1 == nil && err2 == nil && string(left) == string(right) &&
		a.NotBefore.Location() == b.NotBefore.Location() && a.ExpiresAt.Location() == b.ExpiresAt.Location()
}

func validateDeclaration(d Declaration) error {
	appDesk := false
	switch d.Holder.Kind {
	case HolderEngineeringAssignment:
	case HolderAppDesk:
		appDesk = true
	default:
		return invalid("holder kind %q", d.Holder.Kind)
	}
	if d.Holder.Ref == "" {
		return invalid("holder has no ref")
	}
	if d.ComputerID == "" {
		return invalid("no computer id")
	}
	switch d.Level {
	case LevelL1, LevelL3:
	case LevelL4:
		if appDesk {
			return invalid("an app desk cannot hold L4")
		}
		if !slices.Contains(d.Conditions, ConditionNoPrivateData) {
			return invalid("L4 requires %s", ConditionNoPrivateData)
		}
	default:
		return invalid("level %q", d.Level)
	}
	if len(d.Destinations) == 0 || len(d.Destinations) > maxDestinations {
		return invalid("%d destinations", len(d.Destinations))
	}
	for _, dest := range d.Destinations {
		if err := validateDestination(d.Level, appDesk, dest); err != nil {
			return err
		}
	}
	if d.Level == LevelL4 {
		if len(d.Methods) != 0 {
			return invalid("L4 has no HTTP methods")
		}
	} else {
		for _, method := range d.Methods {
			if !slices.Contains(httpMethods, method) {
				return invalid("method %q", method)
			}
		}
	}
	for _, condition := range d.Conditions {
		if !slices.Contains(knownConditions, condition) {
			return invalid("condition %q", condition)
		}
	}
	b := d.Budget
	if b.MaxRequests <= 0 || b.MaxDownloadBytes <= 0 || b.MaxUploadBytes <= 0 {
		return invalid("budget fields must be positive")
	}
	if b.MaxRequests > MaxRequestsCap || b.MaxDownloadBytes > MaxDownloadBytesCap || b.MaxUploadBytes > MaxUploadBytesCap {
		return invalid("budget above the hard cap")
	}
	if d.NotBefore.IsZero() || d.ExpiresAt.IsZero() || !d.ExpiresAt.After(d.NotBefore) {
		return invalid("grant needs not_before before expires_at")
	}
	maxLifetime := MaxAssignmentLifetime
	if appDesk {
		maxLifetime = MaxAppDeskLifetime
	}
	if d.ExpiresAt.Sub(d.NotBefore) > maxLifetime {
		return invalid("lifetime over %s", maxLifetime)
	}
	if !appDesk {
		if d.Verb != "" || d.EffectClass != "" || d.OperationContract != "" || d.DataCeiling != "" || d.Credential != "" {
			return invalid("app-desk fields on an engineering grant")
		}
		return nil
	}
	if d.Verb == "" {
		return invalid("app-desk grant has no verb")
	}
	if !slices.Contains(effectClasses, d.EffectClass) {
		return invalid("effect class %q", d.EffectClass)
	}
	if d.EffectClass == EffectVisibleWrite && d.OperationContract == "" {
		return invalid("visible-write needs an operation contract")
	}
	if d.DataCeiling != "" && !slices.Contains(dataCeilings, d.DataCeiling) {
		return invalid("data ceiling %q", d.DataCeiling)
	}
	return nil
}

func validateDestination(level string, appDesk bool, dest Destination) error {
	switch level {
	case LevelL1, LevelL3:
		if dest.Scheme != "https" {
			return invalid("scheme %q at %s", dest.Scheme, level)
		}
	case LevelL4:
		if dest.Scheme != "tcp" && dest.Scheme != "udp" {
			return invalid("scheme %q at L4", dest.Scheme)
		}
		if dest.PathPrefix != "" {
			return invalid("L4 has no path")
		}
	}
	if dest.Port < 0 || dest.Port > 65535 {
		return invalid("port %d", dest.Port)
	}
	if dest.PathPrefix != "" && !strings.HasPrefix(dest.PathPrefix, "/") {
		return invalid("path prefix %q is not absolute", dest.PathPrefix)
	}
	if dest.Host == "*" {
		if level == LevelL1 || appDesk {
			return invalid("wildcard destination at %s or on an app desk", level)
		}
		return nil
	}
	return grantableHost(dest.Host)
}

// grantableHost refuses every name a grant may never name: IP literals,
// localhost, internal and mDNS suffixes, single labels and malformed names.
// Enforcement re-checks the resolved address.
func grantableHost(host string) error {
	if host == "" {
		return invalid("empty host")
	}
	if _, err := netip.ParseAddr(strings.Trim(host, "[]")); err == nil {
		return invalid("IP literal %q", host)
	}
	if host == "localhost" || strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".internal") || strings.HasSuffix(host, ".local") {
		return invalid("host %q is not public", host)
	}
	labels := strings.Split(host, ".")
	if len(labels) < 2 || len(host) > 253 {
		return invalid("host %q", host)
	}
	for _, label := range labels {
		if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return invalid("host %q", host)
		}
		for _, r := range label {
			if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-') {
				return invalid("host %q", host)
			}
		}
	}
	return nil
}

func grantID(d Declaration) (string, error) {
	raw, err := computerevent.CanonicalJSON(d)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrInvalid, err)
	}
	return IDPrefix + computerevent.DigestBytes(raw), nil
}

func invalid(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalid, fmt.Sprintf(format, args...))
}
