package sourcecontract

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"strings"
)

//go:embed source_contract_schema.json
var sourceContractSchemaFS embed.FS

const sourceContractSchemaPath = "source_contract_schema.json"

type sourceContractSchema struct {
	EvidenceStates       map[string]sourceContractState `json:"evidence_states"`
	ReaderArtifactStates map[string]sourceContractState `json:"reader_artifact_states"`
	SelectorKinds        map[string]sourceContractState `json:"selector_kinds"`
	OpenSurfaces         map[string]sourceContractState `json:"open_surfaces"`
}

type sourceContractState struct {
	Aliases    []string `json:"aliases"`
	Label      string   `json:"label"`
	Relational bool     `json:"relational"`
}

var (
	embeddedSourceContractSchemaRaw = mustReadSourceContractSchema()
	embeddedSourceContractSchema    = mustParseSourceContractSchema(embeddedSourceContractSchemaRaw)
)

func mustReadSourceContractSchema() []byte {
	raw, err := sourceContractSchemaFS.ReadFile(sourceContractSchemaPath)
	if err != nil {
		panic(err)
	}
	return raw
}

func mustParseSourceContractSchema(raw []byte) sourceContractSchema {
	var schema sourceContractSchema
	if err := json.Unmarshal(raw, &schema); err != nil {
		panic(err)
	}
	return schema
}

func normalizeToken(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	normalized = strings.ReplaceAll(normalized, "-", "_")
	normalized = strings.ReplaceAll(normalized, " ", "_")
	return normalized
}

func canonicalFromSchema(entries map[string]sourceContractState, value string) string {
	normalized := normalizeToken(value)
	for canonical, spec := range entries {
		if normalized == canonical {
			return canonical
		}
		for _, alias := range spec.Aliases {
			if normalized == normalizeToken(alias) {
				return canonical
			}
		}
	}
	return ""
}

func SourceContractSchemaHash() string {
	sum := sha256.Sum256(embeddedSourceContractSchemaRaw)
	return hex.EncodeToString(sum[:])
}
