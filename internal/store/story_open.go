package store

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/texturedoc"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Opening a story (Cut 2 2b; pre-flight review A6). A management Cast
// addressed to types.StoryOpenInbox creates, in its own commit, the story's
// document and trajectory and delivers the Cast to the new texture:<doc>:
//
//   - desk origin management, recorded on the trajectory with the opening
//     record and the publish mode;
//   - an empty seed revision authored by no owner (appagent kind, role
//     story_seed), because a document needs a head; it never publishes and
//     mints no owner_revision wake;
//   - no work item, so no lifecycle_work_assigned wake and no Texture run
//     racing the Cast: Texture's first turn starts from the Cast record's
//     own wake;
//   - ids derived from (owner, computer, command id, record id), so a
//     retried commit replays and a different command under the same id
//     conflicts on the absent-object conditions; no second story.

const storyTitleMaxRunes = 160

// storyOpeningIDs derives the story's ids from the opening command and
// record.
func storyOpeningIDs(ownerID, computerID, commandID, recordID string) (docID, trajectoryID, seedRevisionID string) {
	key := strings.Join([]string{"choir:texture:story", ownerID, computerID, commandID, recordID}, "\x00")
	id := func(part string) string { return uuid.NewSHA1(uuid.NameSpaceOID, []byte(key+"\x00"+part)).String() }
	return id("document"), id("trajectory"), id("seed")
}

func storyTitle(objective string) string {
	title := strings.Join(strings.Fields(objective), " ")
	if runes := []rune(title); len(runes) > storyTitleMaxRunes {
		title = strings.TrimSpace(string(runes[:storyTitleMaxRunes]))
	}
	if title == "" {
		title = "Story"
	}
	return title
}

// storyTitleAfterRevision is a story's title once Texture commits revision:
// the headline the revision leads with, else the title it had. Texture
// titles the story; the opening Cast's objective is only a working title
// until Texture's first headline (docs/problems/instruction-text-published-
// as-story-titles-2026-10-14.md: management's briefs, "Write the story of
// ...", stood as the published titles because nothing replaced them).
// Documents that are not stories keep their titles.
func storyTitleAfterRevision(trajectory types.TrajectoryRecord, title string, revision types.Revision) string {
	if trajectory.Kind != types.TrajectoryKindStory {
		return title
	}
	headline := texturedoc.Headline(revision.BodyDoc)
	if headline == "" {
		return title
	}
	return storyTitle(headline)
}

// planStoryOpenings rewrites every Cast to StoryOpenInbox to its new
// story's inbox and returns the objects, conditions and edges that create
// the stories, for the caller's batch. opened names the opening records.
func (s *Store) planStoryOpenings(ctx context.Context, ownerID, computerID, commandID string, records []types.CommitmentRecord, now time.Time) ([]types.CommitmentRecord, []objectgraph.Object, []objectgraph.ObjectCondition, []objectgraph.Edge, map[string]bool, error) {
	var objects []objectgraph.Object
	var conditions []objectgraph.ObjectCondition
	var edges []objectgraph.Edge
	opened := map[string]bool{}
	out := records
	copied := false
	for i, rec := range records {
		if strings.TrimSpace(rec.Inbox) != types.StoryOpenInbox {
			continue
		}
		management := agentprofile.Management + ":" + ownerID
		if !rec.IsCast() || strings.TrimSpace(rec.SenderInbox) != management {
			return nil, nil, nil, nil, nil, inboxRefused("record %s: only a Cast from %s opens a story at %s", rec.RecordID, management, types.StoryOpenInbox)
		}
		if strings.TrimSpace(rec.RecordID) == "" {
			return nil, nil, nil, nil, nil, inboxRefused("an opening Cast needs a record id")
		}
		docID, trajectoryID, seedID := storyOpeningIDs(ownerID, computerID, commandID, strings.TrimSpace(rec.RecordID))
		textureInbox := agentprofile.Texture + ":" + docID
		objs, conds, edge, err := planStoryObjects(ownerID, computerID, docID, trajectoryID, seedID, rec, now)
		if err != nil {
			return nil, nil, nil, nil, nil, err
		}
		objects, conditions, edges = append(objects, objs...), append(conditions, conds...), append(edges, edge)
		if !copied {
			out = append([]types.CommitmentRecord(nil), records...)
			copied = true
		}
		rec.Inbox, rec.Addressee = textureInbox, textureInbox
		out[i] = rec
		opened[strings.TrimSpace(rec.RecordID)] = true
	}
	_ = ctx
	return out, objects, conditions, edges, opened, nil
}

// planStoryObjects builds one story's trajectory, document, seed revision,
// Texture desk, start event and command receipt, each conditioned absent.
func planStoryObjects(ownerID, computerID, docID, trajectoryID, seedID string, cast types.CommitmentRecord, now time.Time) ([]objectgraph.Object, []objectgraph.ObjectCondition, objectgraph.Edge, error) {
	textureAgentID := agentprofile.Texture + ":" + docID
	commandID := "story-open:" + strings.TrimSpace(cast.RecordID)
	subjectRefs := map[string]string{
		"artifact":                  "texture://documents/" + docID,
		"doc_id":                    docID,
		types.StoryRefOriginDesk:    agentprofile.Management,
		types.StoryRefOpeningRecord: strings.TrimSpace(cast.RecordID),
	}
	if cast.Cast != nil && cast.Cast.Publish == types.CastPublishAuto {
		subjectRefs[types.StoryRefPublish] = types.CastPublishAuto
	}
	rule := types.SettlementRule{Version: types.LifecycleReducerVersion, RequireNoOpenWorkItems: true, RequiredSubjectRefs: []string{"artifact"}}
	if err := validateLifecycleSettlementRule(rule, subjectRefs); err != nil {
		return nil, nil, objectgraph.Edge{}, err
	}
	trajectory := types.TrajectoryRecord{
		TrajectoryID: trajectoryID, OwnerID: ownerID, ComputerID: computerID,
		Kind: types.TrajectoryKindStory, SubjectRefs: subjectRefs, Status: types.TrajectoryLive,
		SettlementRule: rule, LifecycleVersion: 1, ReducerSeq: 1, CreatedAt: now, UpdatedAt: now,
	}
	objective := ""
	if cast.Directive != nil {
		objective = cast.Directive.Objective
	}
	document := types.Document{
		DocID: docID, OwnerID: ownerID, ComputerID: computerID, TrajectoryID: trajectoryID,
		Title: storyTitle(objective), CurrentRevisionID: seedID, CreatedAt: now, UpdatedAt: now,
	}
	seedMeta, err := json.Marshal(map[string]any{
		"revision_role": types.StorySeedRole, "origin_desk": agentprofile.Management,
		"opening_record": strings.TrimSpace(cast.RecordID), "trajectory_id": trajectoryID,
	})
	if err != nil {
		return nil, nil, objectgraph.Edge{}, err
	}
	seedBody, err := json.Marshal(userAuthoredTextStructuredTextureDoc(docID, seedID, ""))
	if err != nil {
		return nil, nil, objectgraph.Edge{}, err
	}
	seed := types.Revision{
		RevisionID: seedID, DocID: docID, OwnerID: ownerID, ComputerID: computerID, TrajectoryID: trajectoryID,
		AuthorKind: types.AuthorAppAgent, AuthorLabel: "system:story-open", BodyDoc: seedBody,
		Citations: json.RawMessage("[]"), Metadata: seedMeta, CreatedAt: now,
	}
	seed, _, _, err = prepareTextureRevisionV2(seed)
	if err != nil {
		return nil, nil, objectgraph.Edge{}, fmt.Errorf("story open: prepare seed revision: %w", err)
	}
	seed.RevisionHash = types.ComputeStructuredRevisionHash("", seed.Content, seed.BodyDoc, seed.SourceEntities, seed.Provenance)
	agent := types.AgentRecord{
		AgentID: textureAgentID, OwnerID: ownerID, ComputerID: computerID,
		Profile: agentprofile.Texture, Role: agentprofile.Texture, ChannelID: docID,
		LifecycleVersion: 1, LastReducerSeq: 1, CreatedAt: now, UpdatedAt: now,
	}
	event := types.LifecycleEvent{
		EventID: commandID + ":1", OwnerID: ownerID, ComputerID: computerID, TrajectoryID: trajectoryID,
		Kind: types.LifecycleTrajectoryStarted, ReducerVersion: types.LifecycleReducerVersion, ReducerSeq: 1,
		CommandID: commandID, ArtifactRefs: []string{docID, seedID}, CreatedAt: now,
	}

	trajectoryObj, err := lifecycleObject(ogKindTrajectory, ownerID, computerID, trajectoryID, trajectory, lifecycleMetadata("trajectory_id", trajectoryID, computerID, trajectoryID, 1), now, now)
	if err != nil {
		return nil, nil, objectgraph.Edge{}, err
	}
	documentMeta := lifecycleMetadata("doc_id", docID, computerID, trajectoryID, 1)
	documentMeta["current_revision_id"] = seedID
	documentObj, err := lifecycleObject(ogKindTexDoc, ownerID, computerID, docID, document, documentMeta, now, now)
	if err != nil {
		return nil, nil, objectgraph.Edge{}, err
	}
	revisionMeta := lifecycleMetadata("revision_id", seedID, computerID, trajectoryID, 1)
	revisionMeta["doc_id"], revisionMeta["revision_hash"] = docID, seed.RevisionHash
	revisionObj, err := lifecycleObject(ogKindTexRev, ownerID, computerID, seedID, seed, revisionMeta, now, now)
	if err != nil {
		return nil, nil, objectgraph.Edge{}, err
	}
	agentMeta := lifecycleMetadata("agent_id", textureAgentID, computerID, trajectoryID, 1)
	agentMeta["channel_id"] = docID
	agentObj, err := lifecycleObject(ogKindAgent, ownerID, computerID, textureAgentID, agent, agentMeta, now, now)
	if err != nil {
		return nil, nil, objectgraph.Edge{}, err
	}
	eventObj, err := lifecycleObject(ogKindLifecycleEvent, ownerID, computerID, event.EventID, event, lifecycleMetadata("event_id", event.EventID, computerID, trajectoryID, 1), now, now)
	if err != nil {
		return nil, nil, objectgraph.Edge{}, err
	}
	receipt := types.LifecycleCommandReceipt{
		CommandID: commandID, Kind: types.LifecycleStart, OwnerID: ownerID, ComputerID: computerID,
		TrajectoryID: trajectoryID, ReducerVersion: types.LifecycleReducerVersion, ReducerSeq: 1,
		ResultEventRefs: []string{eventObj.CanonicalID}, CreatedAt: now,
		StoredResult: &types.LifecycleStoredResult{Trajectory: trajectory, Agent: &agent, Document: &document, Revision: &seed, Events: []types.LifecycleEvent{event}},
	}
	receiptObj, err := lifecycleObject(ogKindLifecycleCmd, ownerID, computerID, commandID, receipt, lifecycleMetadata("command_id", commandID, computerID, trajectoryID, 1), now, now)
	if err != nil {
		return nil, nil, objectgraph.Edge{}, err
	}
	edgeID, err := objectgraph.BuildEdgeID(revisionObj.CanonicalID, documentObj.CanonicalID, ogEdgeDocRevision, json.RawMessage(`{}`))
	if err != nil {
		return nil, nil, objectgraph.Edge{}, err
	}
	edge := objectgraph.Edge{EdgeID: edgeID, FromID: revisionObj.CanonicalID, ToID: documentObj.CanonicalID, Kind: ogEdgeDocRevision, Metadata: json.RawMessage(`{}`), CreatedAt: now}

	objects := []objectgraph.Object{trajectoryObj, documentObj, revisionObj, agentObj, eventObj, receiptObj}
	conditions := make([]objectgraph.ObjectCondition, 0, len(objects))
	for _, obj := range objects {
		conditions = append(conditions, objectgraph.ObjectCondition{CanonicalID: obj.CanonicalID})
	}
	return objects, conditions, edge, nil
}
