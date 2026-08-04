package coagentowner

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
)

func TestSpawnAgentRejectsInvalidExplicitProfile(t *testing.T) {
	registry := toolregistry.NewToolRegistry()
	if err := RegisterSpawnTool(registry, nil, nil, agentprofile.PolicyFor(agentprofile.Super)); err != nil {
		t.Fatal(err)
	}
	ctx := toolregistry.WithExecutionContext(context.Background(), toolregistry.ExecutionContext{
		RunID: "parent-run", OwnerID: "user-alice", Profile: agentprofile.Super,
	})

	for _, profile := range []string{"texture", "texture researcher", "research", "research-agent", "Researcher", "coagent"} {
		_, err := registry.Execute(ctx, "spawn_agent", json.RawMessage(`{"objective":"Research the subject.","role":"researcher","profile":"`+profile+`"}`))
		if err == nil {
			t.Fatalf("spawn_agent accepted explicit profile %q outside the caller's allowed targets", profile)
		}
		if got := err.Error(); got != "profile must be one of researcher, co-super" {
			t.Fatalf("spawn_agent profile %q error = %q", profile, got)
		}
	}
}
