package apppackage

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
)

const EventSchemaV1 = "choir.app-package-event/v1"

// Op is one package lifecycle act.
type Op string

const (
	OpInstall Op = "install"
	OpEnable  Op = "enable"
	OpDisable Op = "disable"
	OpPurge   Op = "purge"
)

// Status is an installed package's state. An absent package has no status.
type Status string

const (
	StatusInstalled Status = "installed"
	StatusEnabled   Status = "enabled"
	StatusDisabled  Status = "disabled"
)

var (
	ErrInvalidEvent      = errors.New("app package: invalid lifecycle event")
	ErrBrokenChain       = errors.New("app package: lifecycle event chain is broken")
	ErrAlreadyInstalled  = errors.New("app package: already installed")
	ErrNotInstalled      = errors.New("app package: not installed")
	ErrNotEnabled        = errors.New("app package: not enabled")
	ErrInvalidTransition = errors.New("app package: invalid lifecycle transition")
	ErrBindingMismatch   = errors.New("app package: install does not bind its manifest and build")
	ErrUnsupported       = errors.New("app package: not supported yet")
	ErrIncompatibleData  = errors.New("app package: retained data has a schema this version cannot read")
	ErrInvalidDatum      = errors.New("app package: invalid datum")
)

// Event is choir.app-package-event/v1, one lifecycle act on the computer's
// tape. Seq and Prev chain the package events of one computer; the reducer
// never reads At.
type Event struct {
	Schema                    string    `json:"schema"`
	Seq                       uint64    `json:"seq"`
	Prev                      string    `json:"prev,omitempty"`
	Op                        Op        `json:"op"`
	Package                   string    `json:"package"`
	ManifestDigest            string    `json:"manifest_digest,omitempty"`
	BuildDigest               string    `json:"build_digest,omitempty"`
	Manifest                  *Manifest `json:"manifest,omitempty"`
	Build                     *Build    `json:"build,omitempty"`
	RetainedDataSchemaVersion int       `json:"retained_data_schema_version,omitempty"`
	Actor                     string    `json:"actor,omitempty"`
	At                        string    `json:"at,omitempty"`
}

// Digest is the SHA-256 of the event's canonical JSON; the next event's Prev.
func (e Event) Digest() (string, error) { return canonicalDigest(e) }

// Installed is one present package in the inventory.
type Installed struct {
	Name           string   `json:"name"`
	Version        string   `json:"version"`
	Status         Status   `json:"status"`
	ManifestDigest string   `json:"manifest_digest"`
	BuildDigest    string   `json:"build_digest"`
	Manifest       Manifest `json:"manifest"`
	Build          Build    `json:"build"`
	InstalledSeq   uint64   `json:"installed_seq"`
	UpdatedSeq     uint64   `json:"updated_seq"`
}

// Inventory is the fold of a computer's package events: the installed set,
// plus the data schema versions that retain_data purges left behind.
type Inventory struct {
	Seq      uint64               `json:"seq"`
	Head     string               `json:"head,omitempty"`
	Packages map[string]Installed `json:"packages"`
	Retained map[string]int       `json:"retained,omitempty"`
}

// Effect is what a transition asks the store to do in the same batch as its
// event.
type Effect struct {
	// DeleteData: purge under delete_data tombstones the package's data.
	DeleteData bool
}

func NewInventory() Inventory {
	return Inventory{Packages: map[string]Installed{}, Retained: map[string]int{}}
}

// Enabled reports whether name is installed and enabled.
func (inv Inventory) Enabled(name string) bool {
	p, ok := inv.Packages[name]
	return ok && p.Status == StatusEnabled
}

// Names lists installed package names in order.
func (inv Inventory) Names() []string {
	names := make([]string, 0, len(inv.Packages))
	for name := range inv.Packages {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (inv Inventory) clone() Inventory {
	out := Inventory{Seq: inv.Seq, Head: inv.Head, Packages: make(map[string]Installed, len(inv.Packages)), Retained: make(map[string]int, len(inv.Retained))}
	for k, v := range inv.Packages {
		out.Packages[k] = v
	}
	for k, v := range inv.Retained {
		out.Retained[k] = v
	}
	return out
}

// Chain fills an event's schema, seq and prev so it follows inv.
func Chain(inv Inventory, ev Event) (Event, error) {
	ev.Schema = EventSchemaV1
	ev.Seq = inv.Seq + 1
	ev.Prev = inv.Head
	return ev, nil
}

// Reduce folds events, in seq order, from the empty inventory. A gap, a
// duplicate, a reordering or a broken prev link is ErrBrokenChain: the
// inventory is unreadable rather than silently wrong.
func Reduce(events []Event) (Inventory, error) {
	inv := NewInventory()
	for _, ev := range events {
		next, _, err := Apply(inv, ev)
		if err != nil {
			return Inventory{}, fmt.Errorf("app package: reduce seq %d: %w", ev.Seq, err)
		}
		inv = next
	}
	return inv, nil
}

// Apply is the pure transition: it validates ev against inv and returns the
// next inventory and the transition's effect. inv is not modified.
func Apply(inv Inventory, ev Event) (Inventory, Effect, error) {
	if inv.Packages == nil {
		inv = NewInventory()
	}
	if ev.Schema != EventSchemaV1 {
		return Inventory{}, Effect{}, fmt.Errorf("%w: schema %q", ErrInvalidEvent, ev.Schema)
	}
	if ev.Seq != inv.Seq+1 || ev.Prev != inv.Head {
		return Inventory{}, Effect{}, fmt.Errorf("%w: event seq %d prev %q does not follow seq %d head %q", ErrBrokenChain, ev.Seq, ev.Prev, inv.Seq, inv.Head)
	}
	if !namePattern.MatchString(ev.Package) {
		return Inventory{}, Effect{}, fmt.Errorf("%w: package %q", ErrInvalidEvent, ev.Package)
	}
	if ev.Op != OpInstall && (ev.Manifest != nil || ev.Build != nil || ev.ManifestDigest != "" || ev.BuildDigest != "") {
		return Inventory{}, Effect{}, fmt.Errorf("%w: only install carries a manifest and build", ErrInvalidEvent)
	}
	if ev.Op != OpPurge && ev.RetainedDataSchemaVersion != 0 {
		return Inventory{}, Effect{}, fmt.Errorf("%w: only purge records retained data", ErrInvalidEvent)
	}
	digest, err := ev.Digest()
	if err != nil {
		return Inventory{}, Effect{}, err
	}
	next := inv.clone()
	next.Seq = ev.Seq
	next.Head = digest
	current, present := inv.Packages[ev.Package]
	var effect Effect

	switch ev.Op {
	case OpInstall:
		if present {
			return Inventory{}, Effect{}, fmt.Errorf("%w: %s %s (update is S7-1)", ErrAlreadyInstalled, ev.Package, current.Version)
		}
		if err := checkInstallBinding(ev); err != nil {
			return Inventory{}, Effect{}, err
		}
		m := *ev.Manifest
		if retained, ok := inv.Retained[ev.Package]; ok {
			if m.Data.Store == DataStoreNone || !m.ReadsSchema(retained) {
				return Inventory{}, Effect{}, fmt.Errorf("%w: %s retained data at schema %d; %s reads %v", ErrIncompatibleData, ev.Package, retained, m.Version, m.Data.ReadsSchemaVersions)
			}
			delete(next.Retained, ev.Package)
		}
		next.Packages[ev.Package] = Installed{
			Name: m.Name, Version: m.Version, Status: StatusInstalled,
			ManifestDigest: ev.ManifestDigest, BuildDigest: ev.BuildDigest,
			Manifest: m, Build: *ev.Build,
			InstalledSeq: ev.Seq, UpdatedSeq: ev.Seq,
		}
	case OpEnable:
		if !present {
			return Inventory{}, Effect{}, fmt.Errorf("%w: %s", ErrNotInstalled, ev.Package)
		}
		if current.Status == StatusEnabled {
			return Inventory{}, Effect{}, fmt.Errorf("%w: %s is already enabled", ErrInvalidTransition, ev.Package)
		}
		current.Status, current.UpdatedSeq = StatusEnabled, ev.Seq
		next.Packages[ev.Package] = current
	case OpDisable:
		if !present {
			return Inventory{}, Effect{}, fmt.Errorf("%w: %s", ErrNotInstalled, ev.Package)
		}
		if current.Status != StatusEnabled {
			return Inventory{}, Effect{}, fmt.Errorf("%w: %s is %s, not enabled", ErrInvalidTransition, ev.Package, current.Status)
		}
		current.Status, current.UpdatedSeq = StatusDisabled, ev.Seq
		next.Packages[ev.Package] = current
	case OpPurge:
		if !present {
			return Inventory{}, Effect{}, fmt.Errorf("%w: %s", ErrNotInstalled, ev.Package)
		}
		if current.Status == StatusEnabled {
			return Inventory{}, Effect{}, fmt.Errorf("%w: disable %s before purging it", ErrInvalidTransition, ev.Package)
		}
		data := current.Manifest.Data
		retains := data.Store != DataStoreNone && current.Manifest.Restore.OnPurge == OnPurgeRetainData
		want := 0
		if retains {
			want = data.SchemaVersion
		}
		// Retention follows from the installed manifest; the event's field
		// is a self-description for tape readers and may not contradict it.
		if ev.RetainedDataSchemaVersion != 0 && ev.RetainedDataSchemaVersion != want {
			return Inventory{}, Effect{}, fmt.Errorf("%w: purge of %s retains schema %d, event records %d", ErrInvalidEvent, ev.Package, want, ev.RetainedDataSchemaVersion)
		}
		delete(next.Packages, ev.Package)
		if retains {
			next.Retained[ev.Package] = data.SchemaVersion
		} else {
			delete(next.Retained, ev.Package)
			effect.DeleteData = data.Store != DataStoreNone
		}
	default:
		return Inventory{}, Effect{}, fmt.Errorf("%w: op %q", ErrInvalidEvent, ev.Op)
	}
	return next, effect, nil
}

// PurgeEvent returns the purge act for an installed package, recording the
// data it retains as its manifest requires.
func PurgeEvent(inv Inventory, name string) Event {
	ev := Event{Op: OpPurge, Package: name}
	if p, ok := inv.Packages[name]; ok && p.Manifest.Data.Store != DataStoreNone && p.Manifest.Restore.OnPurge == OnPurgeRetainData {
		ev.RetainedDataSchemaVersion = p.Manifest.Data.SchemaVersion
	}
	return ev
}

// checkInstallBinding is design §5.3: the event, manifest and build name the
// same package source and build, and S7-0a's unsupported features are off.
func checkInstallBinding(ev Event) error {
	if ev.Manifest == nil || ev.Build == nil {
		return fmt.Errorf("%w: install carries the manifest and build receipt", ErrBindingMismatch)
	}
	m, b := *ev.Manifest, *ev.Build
	if err := m.Validate(); err != nil {
		return err
	}
	if err := b.Validate(); err != nil {
		return err
	}
	manifestDigest, err := m.Digest()
	if err != nil {
		return err
	}
	buildDigest, err := b.Digest()
	if err != nil {
		return err
	}
	switch {
	case ev.Package != m.Name:
		return fmt.Errorf("%w: event names %s, manifest names %s", ErrBindingMismatch, ev.Package, m.Name)
	case ev.ManifestDigest != manifestDigest:
		return fmt.Errorf("%w: manifest digest", ErrBindingMismatch)
	case ev.BuildDigest != buildDigest:
		return fmt.Errorf("%w: build digest", ErrBindingMismatch)
	case b.ManifestDigest != manifestDigest:
		return fmt.Errorf("%w: build was made from another manifest", ErrBindingMismatch)
	case b.SourceDigest != m.Source.Digest:
		return fmt.Errorf("%w: build was made from another source", ErrBindingMismatch)
	case b.Package != m.Name || b.Version != m.Version || b.Recipe != m.Build.Recipe:
		return fmt.Errorf("%w: build package, version or recipe", ErrBindingMismatch)
	case !equalStrings(b.Paths(), m.Build.Outputs):
		return fmt.Errorf("%w: build files are not the manifest's outputs", ErrBindingMismatch)
	}
	if m.Backend != nil {
		return fmt.Errorf("%w: backend services arrive in S7-2", ErrUnsupported)
	}
	return nil
}

func equalStrings(a, b []string) bool {
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

// MaxDatumBytes bounds one package datum's JSON value.
const MaxDatumBytes = 64 << 10

var datumKeyPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,63}$`)

// Datum is one package_kv/v1 value.
type Datum struct {
	Package       string          `json:"package"`
	Key           string          `json:"key"`
	Value         json.RawMessage `json:"value"`
	SchemaVersion int             `json:"schema_version"`
}

// ValidateDatumKey checks a package_kv/v1 key.
func ValidateDatumKey(key string) error {
	if !datumKeyPattern.MatchString(key) {
		return fmt.Errorf("%w: key %q must match %s", ErrInvalidDatum, key, datumKeyPattern)
	}
	return nil
}

// PrepareDatum checks a write against the inventory and returns the datum
// stamped with the package's data schema version. Writes require an enabled
// package that declares package_kv/v1.
func PrepareDatum(inv Inventory, d Datum) (Datum, error) {
	p, ok := inv.Packages[d.Package]
	if !ok {
		return Datum{}, fmt.Errorf("%w: %s", ErrNotInstalled, d.Package)
	}
	if p.Status != StatusEnabled {
		return Datum{}, fmt.Errorf("%w: %s is %s", ErrNotEnabled, d.Package, p.Status)
	}
	if p.Manifest.Data.Store != DataStoreKVV1 {
		return Datum{}, fmt.Errorf("%w: %s declares no package data", ErrUnsupported, d.Package)
	}
	if err := ValidateDatumKey(d.Key); err != nil {
		return Datum{}, err
	}
	value := bytes.TrimSpace(d.Value)
	if len(value) == 0 || len(value) > MaxDatumBytes || !json.Valid(value) {
		return Datum{}, fmt.Errorf("%w: value must be JSON of at most %d bytes", ErrInvalidDatum, MaxDatumBytes)
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, value); err != nil {
		return Datum{}, fmt.Errorf("%w: %v", ErrInvalidDatum, err)
	}
	return Datum{Package: d.Package, Key: d.Key, Value: compact.Bytes(), SchemaVersion: p.Manifest.Data.SchemaVersion}, nil
}

// ReadableDatum checks that a stored datum may be returned to its package:
// the package is installed and its manifest reads the datum's schema.
func ReadableDatum(inv Inventory, d Datum) error {
	p, ok := inv.Packages[d.Package]
	if !ok {
		return fmt.Errorf("%w: %s", ErrNotInstalled, d.Package)
	}
	if !p.Manifest.ReadsSchema(d.SchemaVersion) {
		return fmt.Errorf("%w: %s/%s is schema %d", ErrIncompatibleData, d.Package, d.Key, d.SchemaVersion)
	}
	return nil
}
