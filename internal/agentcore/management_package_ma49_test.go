package agentcore

// MA4.9: management as a package (D9). The failure modes, written before the
// code:
//
//  1. a desk without the grant reaches ProductAPI or the story library on the
//     host (the frame check skipped, a role, a package desk) -> refused by
//     the host's own grant check;
//  2. a platform update applied under a desk named in code rather than the
//     platform.update grant's holder, or under no holder at all -> every
//     canonical event of the update names the holder; with no holder the
//     update is refused before it writes;
//  3. a crash during a platform update: the accepted event is on the tape and
//     the rest is not -> the re-pushed offer resumes the transition once, and
//     every event still names the holder (the restart and marker rules are
//     unchanged);
//  4. two owners sharing one management identity, or a directive to the
//     per-owner desk with no owner falling back to the bare desk name;
//  5. an engineering request from a desk without the admission grant opening
//     an assignment instead of reaching the admission ledger's holder, and an
//     escalation to management not reaching the owner's management inbox.

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
	"github.com/yusefmosiah/go-choir/internal/selfdevprotocol"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
)

func TestProductAPIAndStoriesAreServedOnlyUnderTheirGrants(t *testing.T) {
	rt := &Runtime{}
	ctx := context.Background()
	payload, _ := json.Marshal(map[string]string{"method": "GET", "path": "/api/texture/documents"})
	for _, desk := range []string{agentprofile.Research, agentprofile.Texture, agentprofile.ManagementTask, "app:mail", "app:nobody", ""} {
		_, err := rt.productAPIEgress(ctx, desk, "owner-1", "", payload)
		if err == nil || !strings.Contains(err.Error(), "product.api grant") {
			t.Errorf("ProductAPI from %q: %v, want the grant refusal", desk, err)
		}
	}
	// The holder passes the grant check (and then finds no host bound here).
	if _, err := rt.productAPIEgress(ctx, agentprofile.Management, "owner-1", "", payload); err == nil || strings.Contains(err.Error(), "grant") {
		t.Errorf("ProductAPI from management: %v, want past the grant check", err)
	}
	for _, desk := range []string{agentprofile.Research, agentprofile.Texture, "app:mail"} {
		// The run's role is the holder's: the frame's authority is the
		// session's desk, so a non-holder desk is refused anyway (MA6.3).
		_, err := rt.storiesEgress(ctx, desk, toolregistry.ExecutionContext{Role: agentprofile.Management}, nil)
		if err == nil || !strings.Contains(err.Error(), "intake.read") {
			t.Errorf("Stories from %q: %v, want the grant refusal", desk, err)
		}
	}
	for _, desk := range []string{agentprofile.Management, agentprofile.ManagementTask} {
		if _, err := rt.storiesEgress(ctx, desk, toolregistry.ExecutionContext{Role: agentprofile.Research}, nil); err == nil || strings.Contains(err.Error(), "intake.read") {
			t.Errorf("Stories from %q: %v, want past the grant check", desk, err)
		}
	}
}

func TestPersistentManagementIdentityIsTheAdmissionHoldersOwnerInstance(t *testing.T) {
	holder, ok := deskmanifest.GrantHolder(deskmanifest.GrantAdmitAssignment)
	if !ok {
		t.Fatal("no desk holds the admission ledger")
	}
	a, b := persistentManagementAgentID("owner-a"), persistentManagementAgentID("owner-b")
	if a == b || a != holder.Name+":owner-a" {
		t.Fatalf("identities %q %q", a, b)
	}
	// The minted agent id, the inbox and the persistent id agree.
	if got := defaultAgentID(holder.Name, "owner-a", nil); got != a {
		t.Errorf("defaultAgentID = %q, want %q", got, a)
	}
	route, err := ResolveAddress("research:doc-1", "owner-a", holder.Name)
	if err != nil || route.Inbox != a {
		t.Errorf("route to %s = %+v, %v; want %q", holder.Name, route, err, a)
	}
	// A directive (an Escalate, a Report) to the per-owner desk reaches the
	// owner's instance; with no owner it is refused, never the bare name two
	// owners would share.
	id, err := directiveTargetAgentID(ReductionScope{OwnerID: "owner-a"}, holder.Name)
	if err != nil || id != a {
		t.Errorf("escalation to %s = %q, %v; want %q", holder.Name, id, err, a)
	}
	if id, err := directiveTargetAgentID(ReductionScope{}, holder.Name); err == nil {
		t.Errorf("an ownerless directive to %s resolved to %q", holder.Name, id)
	}
}

func TestEngineeringRequestsReachTheAdmissionLedgersHolder(t *testing.T) {
	holder, _ := deskmanifest.GrantHolder(deskmanifest.GrantAdmitAssignment)
	// Research and Texture do not admit: their Cast to engineering is
	// forwarded to the holder's owner inbox as an admission request.
	for _, from := range []string{"research:doc-1", "texture:doc-1"} {
		route, err := ResolveAddress(from, "owner-a", agentprofile.Engineering)
		if err != nil || !route.Forwarded || route.Inbox != holder.Name+":owner-a" {
			t.Errorf("%s -> engineering = %+v, %v; want forwarded to %s:owner-a", from, route, err, holder.Name)
		}
	}
	// The holder speaks into an assignment it opened, not forwarded.
	route, err := ResolveAddress(holder.Name+":owner-a", "owner-a", agentprofile.Engineering+":asg-1")
	if err != nil || route.Forwarded || route.Inbox != agentprofile.Engineering+":asg-1" {
		t.Errorf("holder -> engineering:asg-1 = %+v, %v", route, err)
	}
	if !holdsAssignments(holder) {
		t.Error("the holder does not hold assignments")
	}
	for _, name := range []string{agentprofile.Research, agentprofile.Texture, agentprofile.Engineering, agentprofile.ManagementTask} {
		d, _ := deskmanifest.Lookup(name)
		if holdsAssignments(d) {
			t.Errorf("%s holds assignments", name)
		}
	}
}

// A platform update applied by the packaged desk: every canonical event names
// the platform.update grant's holder, including after a crash between the
// accepted event and the apply.
func TestPlatformUpdateIsAppliedAsThePlatformUpdateGrantsHolder(t *testing.T) {
	holder, ok := deskmanifest.GrantHolder(deskmanifest.GrantPlatformUpdate)
	if !ok || platformUpdateActor() != holder.Name {
		t.Fatalf("platform update actor %q, holder %q %v", platformUpdateActor(), holder.Name, ok)
	}
	fx := newDerivableSelfDevFixture(t, "computer-platform-update-ma49")
	ctx := context.Background()

	// Crash mid-update: only the accepted event committed.
	offer := fx.mintPlatformUpdateOffer(t, "update-ma49", "<html>ma49</html>", fx.currentHead(t))
	offerDigest := mustOfferDigest(t, offer)
	target, err := selfdevprotocol.Digest(struct {
		ComputerID    string `json:"computer_id"`
		UpdateID      string `json:"update_id"`
		OfferDigest   string `json:"offer_digest"`
		ContentDigest string `json:"content_digest"`
	}{fx.computerID, offer.UpdateID, offerDigest, offer.Manifest.ContentDigest})
	if err != nil {
		t.Fatal(err)
	}
	head, err := fx.store.Head(ctx, fx.computerID)
	if err != nil || head == nil {
		t.Fatalf("head: %v", err)
	}
	id, _ := computerevent.NewEventID()
	accepted := computerevent.Event{
		SchemaVersion: computerevent.SchemaVersionV1, EventID: id, ComputerID: fx.computerID,
		EventKind: computerevent.EventEffectAccepted, OccurredAt: time.Now().UTC().Format(time.RFC3339Nano),
		IdempotencyKey: "platform-update-accepted-" + offer.UpdateID,
		ActorProfile:   platformUpdateActor(), AuthorityRef: "platform-control:update",
		PrivacyClass: "owner", PayloadCommitment: computerevent.ZeroHead,
		ProposedEffectRef: offer.Manifest.ContentDigest, DecisionRef: offerDigest,
		VerifierRefs: offer.VerifierRefs, RequireExpectedHead: true,
		PreviousHead: head.CanonicalEventHead, ExpectedDesiredEventHead: head.DesiredEventHead,
		ExpectedEffectiveEventHead: head.EffectiveEventHead, ExpectedDesiredStateCommitment: head.DesiredStateCommitment,
		ExpectedEffectiveStateCommitment: head.EffectiveStateCommitment, ReducerVersion: computerevent.ReducerVersionV1,
	}
	if _, err := fx.appender.AppendNew(ctx, accepted, computerevent.TransitionInput{TargetStateCommitment: target}, nil); err != nil {
		t.Fatalf("seed accepted: %v", err)
	}
	if _, err := fx.rt.ApplyPlatformUpdate(ctx, offer); err != nil {
		t.Fatalf("resume after the crash: %v", err)
	}
	// A second update, start to end.
	next := fx.mintPlatformUpdateOffer(t, "update-ma49-next", "<html>ma49 next</html>", fx.currentHead(t))
	if _, err := fx.rt.ApplyPlatformUpdate(ctx, next); err != nil {
		t.Fatalf("second update: %v", err)
	}
	n := 0
	for _, record := range fx.cas.snapshot() {
		ev := record.Request.Event
		if !strings.HasPrefix(ev.IdempotencyKey, "platform-update-") {
			continue
		}
		n++
		if ev.ActorProfile != holder.Name {
			t.Errorf("event %s (%s) actor %q, want the platform.update holder %q", ev.EventKind, ev.IdempotencyKey, ev.ActorProfile, holder.Name)
		}
	}
	if n < 10 {
		t.Fatalf("platform update events = %d, want two full updates", n)
	}
}
