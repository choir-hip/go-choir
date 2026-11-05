package contract

import (
	"crypto/rand"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/deskprotocol"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, _ := os.Getwd()
	for d := dir; d != filepath.Dir(d); d = filepath.Dir(d) {
		if _, err := os.Stat(filepath.Join(d, "go.mod")); err == nil {
			return d
		}
	}
	t.Fatal("no go.mod")
	return ""
}

func lock(t *testing.T) *deskprotocol.Surface {
	t.Helper()
	l, err := LoadCurrentLock(repoRoot(t))
	if err != nil {
		t.Fatal(err)
	}
	return l
}

func scopeFor(t *testing.T, role, slot string) (*yaegikernel.ChoirScope, yaegikernel.CellHooks) {
	t.Helper()
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		t.Fatal(err)
	}
	issuer, err := yaegikernel.NewHandleIssuer(secret)
	if err != nil {
		t.Fatal(err)
	}
	broker, err := yaegikernel.NewBroker(yaegikernel.BrokerConfig{ComputerID: "computer-contract", CurrentEpoch: 1, AllowedRoot: t.TempDir()}, issuer)
	if err != nil {
		t.Fatal(err)
	}
	scope, err := yaegikernel.NewChoirScope(broker, issuer, "computer-contract", "activation-contract", 1, role, slot)
	if err != nil {
		t.Fatal(err)
	}
	hooks := scope.BindCell()
	hooks.Begin(yaegikernel.SessionFrame{ID: "cell-contract"})
	return scope, hooks
}

type verbCall struct {
	verb string
	call func(s *yaegikernel.ChoirScope) error
}

func ign(_ string, err error) error { return err }

// requestFields are the fields every request carries (MA1.3).
var requestFields = map[string]any{"p_success": 0.8, "weight": 0.3, "settlement": "recipient"}

var precommitBody = map[string]any{
	"question": "Will the build pass?", "distribution": map[string]float64{"yes": 0.8, "no": 0.2}, "resolver": "management",
}

// driven lists, per desk, the verbs the contract drives. A desk's verb set in
// the lock must contain each; the emitted intent must decode under the lock.
func driven() map[string][]verbCall {
	return map[string][]verbCall{
		"management": {
			{"Cast", func(s *yaegikernel.ChoirScope) error {
				return ign(s.Cast("texture", "write the story", nil, requestFields))
			}},
			{"Ask", func(s *yaegikernel.ChoirScope) error { return ign(s.Ask("texture", "which sources?", requestFields)) }},
			{"Reply", func(s *yaegikernel.ChoirScope) error { return ign(s.Reply("texture", "rec-1", "answer")) }},
			{"CancelAct", func(s *yaegikernel.ChoirScope) error { return ign(s.CancelAct("rec-1")) }},
			{"Escalate", func(s *yaegikernel.ChoirScope) error { return ign(s.Escalate("texture", "stuck", requestFields)) }},
			{"Precommit", func(s *yaegikernel.ChoirScope) error { return ign(s.Precommit(precommitBody)) }},
			{"Report", func(s *yaegikernel.ChoirScope) error {
				return ign(s.Report("texture", "claim", []string{"evidence://1"}, "management"))
			}},
			{"Resolve", func(s *yaegikernel.ChoirScope) error {
				return ign(s.Resolve("rec-1", map[string]any{"verdict": "yes", "evidence_refs": []string{"e1"}}))
			}},
			{"Disagreement", func(s *yaegikernel.ChoirScope) error {
				return ign(s.Disagreement(map[string]any{"commitment_id": "rec-1", "scorer_verdict": "no", "resolver_verdict": "yes"}))
			}},
			{"CancelAssignment", func(s *yaegikernel.ChoirScope) error { return ign(s.CancelAssignment("assignment-1", "stale")) }},
			{"CancelTask", func(s *yaegikernel.ChoirScope) error { return ign(s.CancelTask("task-1", "stale")) }},
		},
		"texture": {
			{"Cast", func(s *yaegikernel.ChoirScope) error {
				return ign(s.Cast("research", "find sources", nil, requestFields))
			}},
			{"Ask", func(s *yaegikernel.ChoirScope) error { return ign(s.Ask("management", "ok?", requestFields)) }},
		},
		"research": {
			{"Report", func(s *yaegikernel.ChoirScope) error {
				return ign(s.Report("texture", "claim", []string{"evidence://1"}, "texture"))
			}},
			{"Reply", func(s *yaegikernel.ChoirScope) error { return ign(s.Reply("texture", "rec-1", "answer")) }},
		},
		"engineering": {
			{"Report", func(s *yaegikernel.ChoirScope) error {
				return ign(s.Report("management", "progress", []string{"evidence://1"}, "management"))
			}},
			{"Ask", func(s *yaegikernel.ChoirScope) error { return ign(s.Ask("management", "ok?", requestFields)) }},
			{"Precommit", func(s *yaegikernel.ChoirScope) error { return ign(s.Precommit(precommitBody)) }},
		},
	}
}

// Emit side: each desk, each driven verb: the verb is in the desk's locked
// verb set, the staged intent decodes under the locked StagedIntent with
// nothing outside it, its kind is a locked intent kind.
func TestEmitSideDecodesUnderTheLock(t *testing.T) {
	lk := lock(t)
	kinds := LockedIntentKinds(lk)
	roles := map[string][2]string{"management": {"management", ""}, "texture": {"texture", ""}, "research": {"research", ""}, "engineering": {"engineering", "implementation"}}
	total := 0
	for desk, calls := range driven() {
		row, ok := lk.Desks[desk]
		if !ok {
			t.Fatalf("desk %s is not in the lock", desk)
		}
		for _, c := range calls {
			t.Run(desk+"/"+c.verb, func(t *testing.T) {
				held := false
				for _, v := range row.Verbs {
					held = held || v == c.verb
				}
				if !held {
					t.Fatalf("the lock does not give %s the verb %s", desk, c.verb)
				}
				if v, ok := lk.Verbs[c.verb]; !ok || v.Sig == nil {
					t.Fatalf("verb %s has no locked signature", c.verb)
				}
				scope, hooks := scopeFor(t, roles[desk][0], roles[desk][1])
				if err := c.call(scope); err != nil {
					t.Fatalf("%s.%s: %v", desk, c.verb, err)
				}
				intents := hooks.End()
				if len(intents) == 0 {
					t.Fatalf("%s.%s staged nothing", desk, c.verb)
				}
				for _, in := range intents {
					raw, _ := json.Marshal(in)
					if probs := CheckShape(lk, "yaegikernel.StagedIntent", raw); len(probs) > 0 {
						t.Fatalf("staged intent falls outside the lock: %s\n%s", strings.Join(probs, "\n"), raw)
					}
					if !kinds[in.Kind] {
						t.Fatalf("intent kind %q is not a locked Intent constant", in.Kind)
					}
					total++
				}
			})
		}
	}
	if total < 15 {
		t.Fatalf("only %d intents driven", total)
	}
}
