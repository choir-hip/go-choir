package researchtools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/sources"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
)

// A test_fixture source (L2a R2-7) serves test data. Research's PollSource
// must not write it into the shared capture store as if it were news; only
// a schedule marked as a fixture run polls it. Written before the code.
func TestResearchPollSourceRefusesATestFixture(t *testing.T) {
	reg := &sources.Registry{UserAgent: "test", Sources: []sources.Source{{ID: "fixture:x", Type: sources.SourceTypeRSS, Name: "Fixture", URL: "https://fixture.example/feed.xml", TOSClass: sources.TOSClassTestFixture}}}
	deps := Dependencies{Sources: reg}
	ctx := toolregistry.WithExecutionContext(context.Background(), toolregistry.ExecutionContext{
		RunID: "research-run", OwnerID: "owner-1", ComputerID: "computer-1", Profile: agentprofile.Research,
	})
	_, err := deps.HostEgress(ctx, "poll_source", json.RawMessage(`{"source_id":"fixture:x"}`))
	if err == nil || !strings.Contains(err.Error(), sources.TOSClassTestFixture) {
		t.Fatalf("research polled a test fixture: %v", err)
	}
}
