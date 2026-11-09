package lease

import (
	"sort"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

// The self-development shot job's state copy names kinds by default: every
// built-in kind except the ones the owner has not decided to hand to an
// engineer. (A capsule spawn takes no store copy; there is no capsule lease.)

// capsuleWithheld are the built-in kinds the default kind set leaves out.
var capsuleWithheld = map[string]bool{
	"choir.texture_document": true,
	"choir.texture_revision": true,
	"choir.lease":            true,
}

// CapsuleStoreKinds is the default kind set of a capsule's store lease: every
// built-in kind but the withheld ones and any mail kind.
func CapsuleStoreKinds() []string {
	var kinds []string
	for _, reg := range objectgraph.DefaultRegistry().AllKinds() {
		k := string(reg.Kind)
		if capsuleWithheld[k] || strings.Contains(k, "mail") {
			continue
		}
		kinds = append(kinds, k)
	}
	sort.Strings(kinds)
	return kinds
}
