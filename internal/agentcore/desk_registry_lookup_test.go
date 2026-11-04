package agentcore

import (
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
)

// MA2.5 contract: the closed desk lists in agentcore (carrier, agent and
// channel identity, cell deadline, texture agent ids) are lookups in the desk
// registry, and each answers as the list it replaced.

func TestDeskCarrierLiveEqualsTheClosedList(t *testing.T) {
	live := map[string]bool{agentprofile.Management: true, agentprofile.Texture: true, agentprofile.Research: true}
	for _, name := range deskmanifest.Names() {
		d, _ := deskmanifest.Lookup(name)
		if d.Kind == deskmanifest.KindApp {
			continue
		}
		if deskCarrierLive(name) != live[name] {
			t.Errorf("deskCarrierLive(%s) = %v", name, deskCarrierLive(name))
		}
	}
	if deskCarrierLive("nobody") || deskCarrierLive("") {
		t.Error("an unknown desk is live")
	}
}

func TestDefaultAgentAndChannelIdentityFollowTheRow(t *testing.T) {
	meta := map[string]any{"doc_id": "doc1"}
	cases := []struct{ profile, owner, agent, channel string }{
		{agentprofile.Management, "o1", "management:o1", "management:o1"},
		{agentprofile.Conductor, "o1", "conductor:o1", ""},
		{agentprofile.Texture, "o1", "texture:doc1", "doc1"},
	}
	for _, c := range cases {
		agent := defaultAgentID(c.profile, c.owner, meta)
		if agent != c.agent {
			t.Errorf("defaultAgentID(%s) = %q, want %q", c.profile, agent, c.agent)
		}
		if got := defaultChannelID(c.profile, meta, nil, agent); got != c.channel {
			t.Errorf("defaultChannelID(%s) = %q, want %q", c.profile, got, c.channel)
		}
	}
	// Per-run identities mint a fresh id and have no channel.
	for _, p := range []string{agentprofile.Research, agentprofile.Engineering, agentprofile.Email} {
		a, b := defaultAgentID(p, "o1", meta), defaultAgentID(p, "o1", meta)
		if a == b || strings.Contains(a, ":") {
			t.Errorf("%s agent ids %q %q are not per run", p, a, b)
		}
		if got := defaultChannelID(p, meta, nil, a); got != "" {
			t.Errorf("%s channel = %q", p, got)
		}
	}
	// A texture run with no document, or a management run with no owner, is per run.
	if id := defaultAgentID(agentprofile.Texture, "o1", nil); strings.HasPrefix(id, "texture:") {
		t.Errorf("subjectless texture id = %q", id)
	}
	if id := defaultAgentID(agentprofile.Management, "", nil); strings.HasPrefix(id, "management") {
		t.Errorf("ownerless management id = %q", id)
	}
}

func TestDeskCellHostDeadlineFollowsTheRow(t *testing.T) {
	t.Setenv(managementCellDeadlineEnv, "")
	for _, name := range deskmanifest.Names() {
		d, _ := deskmanifest.Lookup(name)
		want := time.Duration(0)
		if name == deskmanifest.Management || name == deskmanifest.ManagementTask {
			want = 300 * time.Second
		}
		if d.Kind == deskmanifest.KindApp {
			continue
		}
		if got := deskCellHostDeadline(name); got != want {
			t.Errorf("deskCellHostDeadline(%s) = %v, want %v", name, got, want)
		}
	}
	if deskCellHostDeadline("nobody") != 0 {
		t.Error("an unknown desk has a deadline")
	}
}

func TestIsTextureAgentIDChecksThePrefixOnce(t *testing.T) {
	if !isTextureAgentID(" texture:doc1") || isTextureAgentID("research:doc1") || isTextureAgentID("textures:x") || isTextureAgentID("") {
		t.Fatal("isTextureAgentID")
	}
}
