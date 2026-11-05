package agentcore

import (
	"testing"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// The grant arguments are the closed sets the code enforces (MA4.1): the rule
// kinds of types and the task deadline.
func TestGrantArgumentsMatchTheCodeTheyBound(t *testing.T) {
	if len(deskmanifest.PrecommitRuleKinds) != len(types.PrecommitRuleKinds) {
		t.Fatalf("precommit.rule kinds %v differ from types %v", deskmanifest.PrecommitRuleKinds, types.PrecommitRuleKinds)
	}
	for i, k := range types.PrecommitRuleKinds {
		if deskmanifest.PrecommitRuleKinds[i] != string(k) {
			t.Fatalf("precommit.rule kind %d is %q, types has %q", i, deskmanifest.PrecommitRuleKinds[i], k)
		}
	}
	task, _ := deskmanifest.Lookup(agentprofile.ManagementTask)
	if task.RunDeadline() != managementTaskDeadline {
		t.Fatalf("run.deadline grant %s, managementTaskDeadline %s", task.RunDeadline(), managementTaskDeadline)
	}
}
