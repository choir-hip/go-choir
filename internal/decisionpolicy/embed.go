package decisionpolicy

import _ "embed"

//go:embed reversible-selfdev-v1.json
var reversibleSelfDevV1JSON []byte

//go:embed irreversible-email-v1.json
var irreversibleEmailV1JSON []byte

//go:embed human-required-v1.json
var humanRequiredV1JSON []byte

func ReversibleSelfDevV1Bytes() []byte {
	out := make([]byte, len(reversibleSelfDevV1JSON))
	copy(out, reversibleSelfDevV1JSON)
	return out
}

func IrreversibleEmailV1Bytes() []byte {
	out := make([]byte, len(irreversibleEmailV1JSON))
	copy(out, irreversibleEmailV1JSON)
	return out
}

func HumanRequiredV1Bytes() []byte {
	out := make([]byte, len(humanRequiredV1JSON))
	copy(out, humanRequiredV1JSON)
	return out
}
