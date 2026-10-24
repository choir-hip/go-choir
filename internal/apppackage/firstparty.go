package apppackage

// firstPartyKind and firstPartyPackage are the rows of the generated table.
// The table itself (firstPartyPackages) is generated into firstparty_gen.go by
// frontend/scripts/gen-package-kinds.mjs; CI runs it with --check.
type firstPartyKind struct {
	Kind                string
	Identity            string
	Versioned           bool
	OwnerEdits          bool
	SchemaVersion       int
	ReadsSchemaVersions []int
	Schema              string // compact JSON
}

type firstPartyPackage struct {
	Name  string
	Kinds []firstPartyKind
}
