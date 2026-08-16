package decisionpolicy

import _ "embed"

//go:embed reversible-selfdev-v1.json
var reversibleSelfDevV1JSON []byte

func ReversibleSelfDevV1Bytes() []byte {
	out := make([]byte, len(reversibleSelfDevV1JSON))
	copy(out, reversibleSelfDevV1JSON)
	return out
}
