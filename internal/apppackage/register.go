package apppackage

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sync"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

// ErrSchemaConflict refuses a second registration of a kind whose schema text
// differs from the first. The registry compares the schema reference
// (pkg:<name>@<version>), not the schema; this holds the schema itself to the
// same append-only rule.
var ErrSchemaConflict = errors.New("app package: kind is registered with a different schema")

type compiledKind struct {
	schema *Schema
	text   string // compact JSON of the schema as registered
}

var (
	schemaMu sync.RWMutex
	schemas  = map[objectgraph.ObjectKind]compiledKind{}
)

// SchemaFor returns the compiled schema of a registered package kind.
func SchemaFor(kind objectgraph.ObjectKind) (*Schema, bool) {
	schemaMu.RLock()
	defer schemaMu.RUnlock()
	c, ok := schemas[kind]
	return c.schema, ok
}

// RegisterManifest registers every kind of a validated manifest with the write
// gate and records its schema for body validation. Every kind is attempted and
// the errors are joined. Registration is append-only: a kind that registered
// before a later kind failed stays registered.
func RegisterManifest(m Manifest) error {
	if err := m.Validate(); err != nil {
		return err
	}
	var errs []error
	for _, k := range m.Kinds {
		if err := registerKind(m.Name, k); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func registerKind(pkg string, k Kind) error {
	schema, err := CompileSchema(k.Schema)
	if err != nil {
		return fmt.Errorf("kind %s: %w", k.Kind, err)
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, k.Schema); err != nil {
		return fmt.Errorf("kind %s: %w", k.Kind, err)
	}
	kind := objectgraph.ObjectKind(k.Kind)
	// Held across the registration so two callers cannot both find the schema
	// absent and register different ones.
	schemaMu.Lock()
	defer schemaMu.Unlock()
	if have, ok := schemas[kind]; ok && have.text != compact.String() {
		return fmt.Errorf("%w: %s", ErrSchemaConflict, k.Kind)
	}
	err = objectgraph.RegisterPackageKind(pkg, objectgraph.KindRegistration{
		Kind:         kind,
		IdentityMode: objectgraph.IdentityMode(k.Identity),
		Versioned:    k.Versioned,
		OwnerEdits:   k.OwnerEdits,
		Schema:       fmt.Sprintf("pkg:%s@%d", pkg, k.SchemaVersion),
		Writer:       objectgraph.PackageDeskWriter(pkg),
	})
	if err != nil {
		return err
	}
	schemas[kind] = compiledKind{schema: schema, text: compact.String()}
	return nil
}

// RegisterFirstParty registers the kinds of the first-party packages built into
// this binary (firstparty_gen.go, generated from
// frontend/src/packages/*/choir-package.json). It is idempotent. A package
// whose entry the seam refuses is skipped and named in the returned error; the
// others register. It registers nothing while no first-party manifest declares
// a kind.
func RegisterFirstParty() error {
	var errs []error
	for _, p := range firstPartyPackages {
		for _, k := range p.Kinds {
			kk := Kind{
				Kind: k.Kind, Identity: k.Identity, Versioned: k.Versioned, OwnerEdits: k.OwnerEdits,
				SchemaVersion: k.SchemaVersion, ReadsSchemaVersions: k.ReadsSchemaVersions,
				Schema: json.RawMessage(k.Schema),
			}
			if err := registerKind(p.Name, kk); err != nil {
				errs = append(errs, fmt.Errorf("first-party package %s: %w", p.Name, err))
			}
		}
	}
	return errors.Join(errs...)
}

// RegisterFirstPartyAtBoot is RegisterFirstParty for the boot path: a refusal
// is logged and the computer boots without that kind (a package kind is
// additive; the platform does not depend on one).
func RegisterFirstPartyAtBoot() {
	if err := RegisterFirstParty(); err != nil {
		log.Printf("apppackage: first-party kind registration: %v", err)
	}
}
