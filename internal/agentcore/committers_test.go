package agentcore

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/committer"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// MA5.T1: the Texture committer is registered by kind (texture_revision), and
// a desk hands it a turn only under commit.texture_revision.
//
// Failure modes pinned:
//   - a texture_turn staged by a desk that lacks commit.texture_revision
//     reaches the committer (a non-Texture desk writes a canonical revision);
//   - the refusal is silent (the cell is told nothing, or told it committed);
//   - the idle-trigger answer commits for a desk without the grant;
//   - a second committer for a kind replaces the first, or an empty kind or
//     a nil committer registers;
//   - a dispose unbinds a later registration of the same kind;
//   - the revision identity the committer receives changes shape, so a
//     replayed cell mints a second head instead of replaying the first;
//   - a kind with no committer commits silently.

type identityCommitter struct {
	bodies     []string
	identities []string
}

func (c *identityCommitter) Commit(_ context.Context, _ *types.RunRecord, bodyJSON, revisionIdentity string) (string, error) {
	c.bodies = append(c.bodies, bodyJSON)
	c.identities = append(c.identities, revisionIdentity)
	return "", nil
}

// bindTextureCommitter registers c as the runtime's texture_revision
// committer for the test.
func bindTextureCommitter(t *testing.T, rt *Runtime, c committer.Committer) {
	t.Helper()
	dispose, err := rt.Committers().Register(TextureRevisionKind, c)
	if err != nil {
		t.Fatalf("register texture committer: %v", err)
	}
	t.Cleanup(dispose)
}

func textureTurnReduction(rt *Runtime, role string) *rlmCallReduction {
	return &rlmCallReduction{
		active: true,
		mb:     rt,
		rec:    &types.RunRecord{RunID: "run-1", AgentProfile: role, AgentRole: role},
		scope:  ReductionScope{FromRole: role, RunID: "run-1", CellID: "cell-1", OwnerID: "owner-1"},
	}
}

func TestTextureTurnFromADeskWithoutTheGrantCommitsNothing(t *testing.T) {
	rt := &Runtime{}
	c := &identityCommitter{}
	bindTextureCommitter(t, rt, c)
	in := yaegikernel.StagedIntent{LocalID: "tray-1", Kind: yaegikernel.IntentTextureTurn, Body: `{"base_revision_id":"rev-1","wait":"x"}`}
	for _, role := range []string{agentprofile.Research, agentprofile.Management, agentprofile.Engineering, "app:notes", ""} {
		_, err := textureTurnReduction(rt, role).commitTextureAuthorIntent(context.Background(), in)
		if err == nil || !strings.Contains(err.Error(), "commit.texture_revision") {
			t.Fatalf("desk %q: err = %v, want a refusal naming commit.texture_revision", role, err)
		}
	}
	if _, err := rt.commitDeskTurn(context.Background(), agentprofile.Management, TextureRevisionKind, &types.RunRecord{}, in.Body, "idle-1"); err == nil {
		t.Fatal("the idle answer committed for a desk without the grant")
	}
	if len(c.bodies) != 0 {
		t.Fatalf("committer received %d turns from desks without the grant", len(c.bodies))
	}
}

func TestTextureTurnCommitterReceivesTheCellIdentity(t *testing.T) {
	rt := &Runtime{}
	c := &identityCommitter{}
	bindTextureCommitter(t, rt, c)
	body := `{"base_revision_id":"rev-1","wait":"x"}`
	in := yaegikernel.StagedIntent{LocalID: "tray-1", Kind: yaegikernel.IntentTextureTurn, Body: body}
	for i := 0; i < 2; i++ {
		if _, err := textureTurnReduction(rt, agentprofile.Texture).commitTextureAuthorIntent(context.Background(), in); err != nil {
			t.Fatalf("texture desk turn refused: %v", err)
		}
	}
	// The identity is the one the authorizer seam received before MA5.T1:
	// rlm:<cell>:<local id>:<sha256("texture"\x1f body)[:6]>.
	sum := sha256.Sum256([]byte("texture\x1f" + body))
	want := "rlm:cell-1:tray-1:" + hex.EncodeToString(sum[:6])
	if len(c.identities) != 2 || c.identities[0] != want || c.identities[1] != want || c.bodies[0] != body {
		t.Fatalf("identities = %v, want [%s %s]; bodies = %v", c.identities, want, want, c.bodies)
	}
}

func TestCommitterRegistryRefusesASecondCommitterForAKind(t *testing.T) {
	reg := committer.New()
	first, second := &identityCommitter{}, &identityCommitter{}
	dispose, err := reg.Register(TextureRevisionKind, first)
	if err != nil {
		t.Fatalf("first register: %v", err)
	}
	if _, err := reg.Register(TextureRevisionKind, second); err == nil {
		t.Fatal("a second committer for texture_revision registered")
	}
	if got, ok := reg.Lookup(TextureRevisionKind); !ok || got != first {
		t.Fatalf("lookup = %v, %v; want the first committer", got, ok)
	}
	if _, err := reg.Register(" ", first); err == nil {
		t.Fatal("an empty kind registered")
	}
	if _, err := reg.Register("other", nil); err == nil {
		t.Fatal("a nil committer registered")
	}
	dispose()
	if _, ok := reg.Lookup(TextureRevisionKind); ok {
		t.Fatal("dispose left the committer bound")
	}
	again, err := reg.Register(TextureRevisionKind, second)
	if err != nil {
		t.Fatalf("register after dispose: %v", err)
	}
	defer again()
	dispose() // a stale dispose must not unbind the newer registration
	if got, ok := reg.Lookup(TextureRevisionKind); !ok || got != second {
		t.Fatal("a stale dispose unbound a later registration")
	}
}

func TestTextureTurnWithNoCommitterIsRefused(t *testing.T) {
	rt := &Runtime{}
	in := yaegikernel.StagedIntent{LocalID: "tray-1", Kind: yaegikernel.IntentTextureTurn, Body: `{"wait":"x"}`}
	if _, err := textureTurnReduction(rt, agentprofile.Texture).commitTextureAuthorIntent(context.Background(), in); err == nil || !strings.Contains(err.Error(), "no committer") {
		t.Fatalf("err = %v, want no committer registered", err)
	}
}
