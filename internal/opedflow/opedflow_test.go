// Package opedflow is the repeatable artifact of the two-computer op-ed flow
// (handoff section 6 items 4 and 5, short of the model-run desk turn and the
// browser): a real corpusd service and two real computers in one process, the
// paper's and a reader's, each polling the service the way a bound guest does.
//
//	reader publishes a fork of the paper's story as an op-ed
//	-> paper's oped_intake adopts it, the gate judges it, the verdict is on the
//	   paper's tape and posted to corpusd
//	-> an accepted op-ed wakes the story's desk (an oped_wake in its inbox)
//	-> the reader's notice_intake brings the verdict (same hash) to the
//	   reader's changelog
//	-> the paper publishes a response citing the op-ed; the reader's changelog
//	   shows paper_responded.
package opedflow

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/agentcore"
	contentowner "github.com/yusefmosiah/go-choir/internal/content"
	"github.com/yusefmosiah/go-choir/internal/dbx"
	"github.com/yusefmosiah/go-choir/internal/dbx/dbxtest"
	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
	"github.com/yusefmosiah/go-choir/internal/events"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/platform"
	"github.com/yusefmosiah/go-choir/internal/provider"
	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
	"os"
)

func TestMain(m *testing.M) { os.Exit(dbxtest.Main(m)) }

// serviceFeed is a bound guest's view of corpusd: the owner is fixed by who it is.
type serviceFeed struct {
	svc   *platform.Service
	owner string
}

func (f serviceFeed) Opeds(ctx context.Context, after string, n int) (out agentcore.OpedFeedWire, err error) {
	feed, err := f.svc.ListOpedsForOwner(ctx, f.owner, after, n)
	return out, roundTrip(feed, err, &out)
}
func (f serviceFeed) Notices(ctx context.Context, after string, n int) (out agentcore.NoticeFeedWire, err error) {
	feed, err := f.svc.ListNoticesForOwner(ctx, f.owner, after, n)
	return out, roundTrip(feed, err, &out)
}
func (f serviceFeed) ReadVersion(ctx context.Context, id string) (out agentcore.PublicationVersionWire, err error) {
	v, err := f.svc.ReadPublicationVersion(ctx, id)
	return out, roundTrip(v, err, &out)
}
func (f serviceFeed) PostOpedVerdict(ctx context.Context, w agentcore.OpedVerdictWire) error {
	_, err := f.svc.PostOpedVerdict(ctx, platform.OpedVerdictPost{OwnerID: f.owner, VersionID: w.VersionID, Run: w.Run,
		Verdict: w.Verdict, Reason: w.Reason, VerdictHash: w.VerdictHash, TapeRecordID: w.TapeRecordID})
	return err
}

// roundTrip converts the service's type to the computer's wire type, as the HTTP hop would.
func roundTrip(in any, err error, out any) error {
	if err != nil {
		return err
	}
	raw, err := json.Marshal(in)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, out)
}

type acceptingJudge struct{}

func (acceptingJudge) Judge(_ context.Context, _ json.RawMessage, questions map[string]json.RawMessage) (json.RawMessage, error) {
	answers := map[string]any{}
	for name := range questions {
		answers[name] = map[string]any{"choice": "no", "probabilities": map[string]float64{"yes": 0.02, "no": 0.98}}
	}
	return json.Marshal(map[string]any{"id": "j", "model": "~typesafe/jev-latest", "answers": answers})
}

func newComputer(t *testing.T, id string, feed agentcore.PlatformFeed) (*agentcore.Runtime, *store.Store) {
	t.Helper()
	dir := t.TempDir()
	s, err := store.Open(filepath.Join(dir, "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	bus := events.NewEventBus()
	rt := agentcore.New(provideriface.Config{ComputerID: id, StorePath: filepath.Join(dir, "state.db"), PromptRoot: filepath.Join(dir, "prompts")},
		s, bus, provider.NewStubProvider(0), agentcore.WithContentService(contentowner.NewService(s, bus)))
	rt.SetDispatchActor(func(context.Context, string, string, string, string, string, string, string) error { return nil })
	rt.SetPlatformFeed(feed)
	rt.SetCandidateJudge(acceptingJudge{})
	t.Cleanup(rt.Stop)
	return rt, s
}

func TestTwoComputersOpedFlow(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	control, err := dbx.OpenPostgres(dbxtest.PostgresDSN(t), dbx.PGOptions{})
	if err != nil {
		t.Fatal(err)
	}
	graph, err := dbx.OpenPostgres(dbxtest.PostgresDSN(t), dbx.PGOptions{})
	if err != nil {
		t.Fatal(err)
	}
	ps := platform.NewStore(control, graph)
	if err := ps.Bootstrap(ctx); err != nil {
		t.Fatal(err)
	}
	if err := ps.MigrateRetrieval(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ps.Close() })
	svc := platform.NewService(ps, filepath.Join(root, "artifacts"), filepath.Join(root, "signing.key"))

	const paperOwner, readerOwner = "paper-owner", "reader-owner"
	paper, paperStore := newComputer(t, "computer-paper", serviceFeed{svc, paperOwner})
	reader, readerStore := newComputer(t, "computer-reader", serviceFeed{svc, readerOwner})
	seedStoryDoc(t, paperStore, paperOwner, "computer-paper", "doc-story")
	public := []byte(`{"visibility":"public","route":"public"}`)

	story, err := svc.PublishTexture(ctx, platform.PublishTextureRequest{OwnerID: paperOwner, SourceDocID: "doc-story", SourceRevisionID: "doc-story-r1",
		RevisionNumber: 1, Title: "Harbor reopens", Content: "The harbor reopens at dawn.", PublishMode: platform.PublishModeAuto})
	if err != nil {
		t.Fatal(err)
	}
	storyRef := platform.PublishParentRef{PublicationVersionID: story.PublicationVersionID, ManifestHash: story.ManifestHash}
	op, err := svc.PublishTexture(ctx, platform.PublishTextureRequest{OwnerID: readerOwner, SourceDocID: "doc-fork", SourceRevisionID: "doc-fork-r1",
		RevisionNumber: 1, Title: "My view", Content: "The ferry crews were not consulted.", PublishMode: platform.PublishModeOpinion,
		AuthorLabel: "writer-aa11bb", AccessPolicy: public, Parents: []platform.PublishParentRef{storyRef}})
	if err != nil {
		t.Fatal(err)
	}

	// Publishing armed neither computer's polls here (the proxy tells them); arm them as it does.
	paper.EnsurePlatformPolls(ctx, paperOwner, "computer-paper")
	reader.EnsurePlatformPolls(ctx, readerOwner, "computer-reader")
	if err := paper.RunPlatformPollNow(ctx, paperOwner, store.PlatformPollOpedIntake); err != nil {
		t.Fatal(err)
	}
	// The verdict is on the paper's tape and at corpusd with the same hash; an accepted op-ed woke the desk.
	state, err := svc.OpedState(ctx, op.PublicationVersionID)
	if err != nil || state.Verdict != "accepted" {
		t.Fatalf("corpusd state = %+v %v", state, err)
	}
	tape, err := paperStore.GetCommitmentRecord(ctx, paperOwner, "computer-paper", types.OpedVerdictRecordID(op.PublicationVersionID, 1))
	if err != nil || tape == nil || tape.Oped.Verdict.Verdict != types.OpedAccepted {
		t.Fatalf("paper tape verdict = %+v %v", tape, err)
	}
	wakes, _ := paperStore.ListInboxRecords(ctx, paperOwner, "computer-paper", deskmanifest.DocumentInbox("doc-story"), 0, 0)
	if len(wakes) != 1 || !store.RecordWakesReader(wakes[0]) || wakes[0].Oped.Wake.VersionID != op.PublicationVersionID {
		t.Fatalf("the story desk's inbox = %+v", wakes)
	}
	// The reader hears the verdict.
	if err := reader.RunPlatformPollNow(ctx, readerOwner, store.PlatformPollNoticeIntake); err != nil {
		t.Fatal(err)
	}
	kinds := func() map[string]string {
		evs, err := readerStore.ListChangelogEvents(ctx, readerOwner, "computer-reader", types.ChangelogQuery{IncludeMinor: true})
		if err != nil {
			t.Fatal(err)
		}
		out := map[string]string{}
		for _, e := range evs {
			out[string(e.Changelog.Kind)+"/"+e.Changelog.Relation] = e.Changelog.VerdictHash
		}
		return out
	}
	if got := kinds(); got["oped_verdict/"] != state.VerdictHash || state.VerdictHash == "" {
		t.Fatalf("reader changelog = %v, want the verdict with hash %q", got, state.VerdictHash)
	}
	// The paper responds, citing the op-ed; the reader hears that too.
	opRef := platform.PublishParentRef{PublicationVersionID: op.PublicationVersionID, ManifestHash: op.ManifestHash}
	if _, err := svc.PublishTexture(ctx, platform.PublishTextureRequest{OwnerID: paperOwner, SourceDocID: "doc-story", SourceRevisionID: "doc-story-r2",
		RevisionNumber: 2, Title: "Harbor reopens", Content: "The harbor reopens at dawn; ferry crews object.", PublishMode: platform.PublishModeAuto,
		Parents: []platform.PublishParentRef{opRef}}); err != nil {
		t.Fatal(err)
	}
	if err := reader.RunPlatformPollNow(ctx, readerOwner, store.PlatformPollNoticeIntake); err != nil {
		t.Fatal(err)
	}
	if got := kinds(); got["parent_used/paper_responded"] == "" && !hasKey(got, "parent_used/paper_responded") {
		t.Fatalf("reader changelog = %v, want paper_responded", got)
	}
}

func hasKey(m map[string]string, k string) bool { _, ok := m[k]; return ok }

// seedStoryDoc gives the paper's computer the story's document, as it holds it in life.
func seedStoryDoc(t *testing.T, s *store.Store, owner, computer, doc string) {
	t.Helper()
	agent := deskmanifest.DocumentInbox(doc)
	req := types.StartLifecycleRequest{
		OwnerID: owner, ComputerID: computer, CommandID: "start:" + doc, TrajectoryID: "trajectory-" + doc, Kind: types.TrajectoryKindDocument,
		SettlementRule:  types.SettlementRule{Version: types.LifecycleReducerVersion, RequireNoOpenWorkItems: true, RequiredSubjectRefs: []string{"artifact"}},
		SubjectRefs:     map[string]string{"artifact": "texture://documents/" + doc, "doc_id": doc},
		InitialWork:     types.WorkItemRecord{WorkItemID: "work:" + doc, Objective: "report", AssignedAgentID: agent, AuthorityProfile: "texture"},
		InitialDocument: types.Document{DocID: doc, Title: "Harbor reopens"},
		InitialRevision: types.Revision{RevisionID: "rev:" + doc, AuthorKind: types.AuthorUser, AuthorLabel: owner, Content: "The harbor reopens at dawn."},
		Agent:           types.AgentRecord{AgentID: agent, OwnerID: owner, ComputerID: computer, Profile: "texture", Role: "texture", ChannelID: doc},
	}
	req.StartRequestDigest, _ = store.ComputeStartLifecycleRequestDigest(req)
	// The document is Texture's kind: the seed writes it as its custodian.
	w, ok := objectgraph.CustodianOf("choir.texture_document")
	if !ok {
		t.Fatal("choir.texture_document has no custodian")
	}
	if _, err := s.StartLifecycle(objectgraph.WithWriter(context.Background(), w), req); err != nil {
		t.Fatal(err)
	}
}
