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
	managementPolicy, err := agentprofile.PolicyFor(agentprofile.Management)
	if err != nil {
		t.Fatal(err)
	}
	if err := RegisterSpawnTool(registry, nil, nil, managementPolicy); err != nil {
		t.Fatal(err)
	}
	ctx := toolregistry.WithExecutionContext(context.Background(), toolregistry.ExecutionContext{
		RunID: "parent-run", OwnerID: "user-alice", Profile: agentprofile.Management,
	})

	for _, profile := range []string{"texture", "texture research", "management", "engineering", "Research", "conductor"} {
		_, err := registry.Execute(ctx, "spawn_agent", json.RawMessage(`{"objective":"Research the subject.","role":"research","profile":"`+profile+`"}`))
		if err == nil {
			t.Fatalf("spawn_agent accepted explicit profile %q outside the caller's allowed targets", profile)
		}
		if got := err.Error(); got != "profile must be one of research" {
			t.Fatalf("spawn_agent profile %q error = %q", profile, got)
		}
	}
}
