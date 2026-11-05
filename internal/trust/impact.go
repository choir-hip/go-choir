// Package trust is the Bayesian trust system of plan section 10 and the trust
// draft (docs/design/bayesian-trust-2026-11-04.md): the impact table as locked
// data, evidence extracted from settled records, and the derived trust view
// per subject and action class. Nothing here is stored: a view is a box score
// recomputed from the records, with the parameters it used logged beside it.
package trust

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/yusefmosiah/go-choir/internal/types"
)

//go:embed impact_table.json
var impactTableJSON []byte

// PinnedImpactTableDigest pins the table's bytes. The table is locked data:
// changing it changes every view that used it, so the change must bump the
// table's version and this pin together, in one reviewed commit.
const PinnedImpactTableDigest = "4e07508865d30d1c7b685dba34f836961f256c63ed66564edb10fbe67c1d64f0"

// ClassRow is one impact class.
type ClassRow struct {
	Class    types.ImpactClass `json:"class"`
	Name     string            `json:"name"`
	Impact   float64           `json:"impact"`
	Tau      *float64          `json:"tau"`
	Examples string            `json:"examples"`
}

// GrantRow maps one grant to its impact class.
type GrantRow struct {
	Grant  string            `json:"grant"`
	Family bool              `json:"family,omitempty"`
	Impact types.ImpactClass `json:"impact"`
	Why    string            `json:"why"`
}

// NotImpactRow names something that is logged and deliberately not classed.
type NotImpactRow struct {
	Name string `json:"name"`
	Why  string `json:"why"`
}

// Table is the decoded impact table.
type Table struct {
	Version   string         `json:"version"`
	Classes   []ClassRow     `json:"classes"`
	Grants    []GrantRow     `json:"grants"`
	NotImpact []NotImpactRow `json:"not_impact"`
}

var (
	tableOnce sync.Once
	table     Table
	tableErr  error
)

func loadTable() {
	tableOnce.Do(func() {
		dec := json.NewDecoder(strings.NewReader(string(impactTableJSON)))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&table); err != nil {
			tableErr = fmt.Errorf("trust: impact table: %w", err)
			return
		}
		seen := map[string]bool{}
		for _, g := range table.Grants {
			if seen[g.Grant] {
				tableErr = fmt.Errorf("trust: impact table lists %q twice", g.Grant)
				return
			}
			seen[g.Grant] = true
			if r, ok := g.Impact.Rank(); !ok || g.Impact == "" || r < 0 {
				tableErr = fmt.Errorf("trust: impact row %q has class %q", g.Grant, g.Impact)
				return
			}
		}
		if len(table.Classes) != 5 {
			tableErr = fmt.Errorf("trust: impact table needs I0 to I4")
		}
	})
	if tableErr != nil {
		panic(tableErr)
	}
}

// ImpactTable returns the locked table.
func ImpactTable() Table { loadTable(); return table }

// ImpactTableDigest is the sha256 of the table's bytes.
func ImpactTableDigest() string {
	sum := sha256.Sum256(impactTableJSON)
	return hex.EncodeToString(sum[:])
}

// ImpactNumber is the governance number of a class (0, 1, 3, 10, 30).
func ImpactNumber(c types.ImpactClass) float64 {
	loadTable()
	if c == "" {
		c = types.ImpactI0
	}
	for _, r := range table.Classes {
		if r.Class == c {
			return r.Impact
		}
	}
	return 0
}

// Tau is the autonomy threshold of a class; ok is false for I0 (never gated).
func Tau(c types.ImpactClass) (float64, bool) {
	loadTable()
	for _, r := range table.Classes {
		if r.Class == c && r.Tau != nil {
			return *r.Tau, true
		}
	}
	return 0, false
}

// GrantImpact resolves a grant name to its class: an exact row, or
// family.<segment> for a family row. The bare family name is not a grant.
func GrantImpact(name string) (types.ImpactClass, bool) {
	loadTable()
	for _, g := range table.Grants {
		if !g.Family && g.Grant == name {
			return g.Impact, true
		}
	}
	if i := strings.LastIndex(name, "."); i > 0 {
		for _, g := range table.Grants {
			if g.Family && g.Grant == name[:i] {
				return g.Impact, true
			}
		}
	}
	return "", false
}

// ActionClass is a grant crossed with its impact class (trust draft 3.3):
// trust earned drafting mail does not transfer to sending it.
type ActionClass struct {
	Grant  string            `json:"grant"`
	Impact types.ImpactClass `json:"impact"`
}

func (a ActionClass) String() string { return a.Grant + "/" + string(a.Impact) }

func maxClass(a, b types.ImpactClass) types.ImpactClass {
	ra, _ := a.Rank()
	rb, _ := b.Rank()
	if rb > ra {
		return b
	}
	return a
}

// ClassOf is the action class of a record. The grants the record declares
// (Action.DeclaredCapabilities) decide first: the highest-impact declared
// grant names the class and the grant. A record that declares none is the act
// of its request kind, under desk.request. The impact is the higher of the
// grant's table class and the class frozen with the record, so a sender can
// raise a class and never lower it.
func ClassOf(rec types.CommitmentRecord) ActionClass {
	frozen := rec.ImpactClass
	if rec.MaterialityProcedure != nil {
		frozen = maxClass(frozen, rec.MaterialityProcedure.ImpactClass)
	}
	if frozen == "" {
		frozen = types.ImpactI0
	}
	best := ActionClass{}
	bestRank := -1
	if rec.Action != nil {
		for _, c := range rec.Action.DeclaredCapabilities {
			if g, ok := GrantImpact(strings.TrimSpace(c)); ok {
				if r, _ := g.Rank(); r > bestRank {
					best, bestRank = ActionClass{Grant: strings.TrimSpace(c), Impact: g}, r
				}
			}
		}
	}
	if bestRank < 0 {
		g, _ := GrantImpact("desk.request")
		best = ActionClass{Grant: "desk.request", Impact: g}
	}
	best.Impact = maxClass(best.Impact, frozen)
	return best
}

func containsFold(s, sub string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(sub))
}
