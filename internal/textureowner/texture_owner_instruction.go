package textureowner

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/yusefmosiah/go-choir/internal/agentprofile"

	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

const textureOwnerInstructionSchemaV1 = "choir.texture_owner_instruction.v1"

type textureOwnerInstructionRequest struct {
	ClientRequestID        string `json:"client_request_id"`
	Content                string `json:"content"`
	ExpectedHeadRevisionID string `json:"expected_head_revision_id"`
}

type textureOwnerInstructionResponse struct {
	Schema           string                                `json:"schema"`
	InstructionID    string                                `json:"instruction_id"`
	RequestID        string                                `json:"request_id"`
	Kind             types.LifecycleOwnerInstructionKind   `json:"kind"`
	Status           types.LifecycleOwnerInstructionStatus `json:"status"`
	DocID            string                                `json:"doc_id"`
	TrajectoryID     string                                `json:"trajectory_id"`
	TargetAgentID    string                                `json:"target_agent_id"`
	TargetWorkItemID string                                `json:"target_work_item_id"`
	HeadRevisionID   string                                `json:"head_revision_id"`
	Cursor           int64                                 `json:"cursor"`
	Replay           bool                                  `json:"replay"`
}

func textureOwnerOccurrenceIdentity(ownerID, computerID, docID, clientRequestID string) (string, string) {
	sum := sha256.Sum256([]byte(strings.Join([]string{ownerID, computerID, docID, clientRequestID}, "\x00")))
	id := hex.EncodeToString(sum[:])
	return "owner-request-" + id, "owner-instruction-" + id
}

func (h *Handler) HandleTextureOwnerInstruction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}
	docID := extractDocID(r.URL.Path)
	doc, err := h.getTextureDocument(r.Context(), ownerID, docID)
	if err != nil {
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "document not found"})
		return
	}
	if doc.TrajectoryID == "" || doc.ComputerID == "" {
		writeAPIJSON(w, http.StatusConflict, apiError{Error: "document has no lifecycle owner-instruction authority"})
		return
	}
	kind := types.LifecycleOwnerTell
	if strings.HasSuffix(strings.TrimSuffix(r.URL.Path, "/"), "/correct") {
		kind = types.LifecycleOwnerCorrect
	}
	var input textureOwnerInstructionRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid owner instruction"})
		return
	}
	input.ClientRequestID = strings.TrimSpace(input.ClientRequestID)
	input.Content = strings.TrimSpace(input.Content)
	input.ExpectedHeadRevisionID = strings.TrimSpace(input.ExpectedHeadRevisionID)
	if input.ClientRequestID == "" || input.Content == "" || input.ExpectedHeadRevisionID == "" || len(input.ClientRequestID) > 256 {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "client_request_id, content, and expected_head_revision_id are required"})
		return
	}
	requestID, instructionID := textureOwnerOccurrenceIdentity(ownerID, doc.ComputerID, doc.DocID, input.ClientRequestID)
	snapshot, err := h.Store.GetLifecycleSnapshot(r.Context(), ownerID, doc.ComputerID, doc.TrajectoryID)
	if err != nil {
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "lifecycle not found"})
		return
	}
	if existing, existingErr := h.Store.GetLifecycleOwnerInstruction(r.Context(), ownerID, doc.ComputerID, doc.TrajectoryID, instructionID); existingErr == nil {
		if existing.RequestID != requestID || existing.Kind != kind || existing.Content != input.Content || existing.HeadRevisionID != input.ExpectedHeadRevisionID {
			writeAPIJSON(w, http.StatusConflict, apiError{Error: "owner instruction occurrence conflicts with its durable receipt"})
			return
		}
		writeAPIJSON(w, http.StatusAccepted, textureOwnerInstructionResponse{
			Schema: textureOwnerInstructionSchemaV1, InstructionID: existing.InstructionID, RequestID: existing.RequestID,
			Kind: existing.Kind, Status: existing.Status, DocID: existing.DocumentID, TrajectoryID: existing.TrajectoryID,
			TargetAgentID: existing.TargetAgentID, TargetWorkItemID: existing.TargetWorkItemID,
			HeadRevisionID: existing.HeadRevisionID, Cursor: existing.ReducerSeq, Replay: true,
		})
		return
	} else if !errors.Is(existingErr, store.ErrNotFound) {
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to load owner instruction receipt"})
		return
	}
	if snapshot.Document.DocID != doc.DocID || snapshot.Trajectory.Status != types.TrajectoryLive {
		writeAPIJSON(w, http.StatusConflict, apiError{Error: "document head or lifecycle changed"})
		return
	}
	targetAgentID := "texture:" + doc.DocID
	var targetWorkID string
	for _, work := range snapshot.WorkItems {
		if work.Status == types.WorkItemOpen && work.AssignedAgentID == targetAgentID && work.AuthorityProfile == "texture" {
			if targetWorkID != "" {
				writeAPIJSON(w, http.StatusConflict, apiError{Error: "lifecycle has multiple open Texture target work items"})
				return
			}
			targetWorkID = work.WorkItemID
		}
	}
	if targetWorkID == "" {
		writeAPIJSON(w, http.StatusConflict, apiError{Error: "lifecycle has no open Texture target work item"})
		return
	}
	req := types.QueueLifecycleOwnerInstructionRequest{
		OwnerID: ownerID, ComputerID: doc.ComputerID, CommandID: "queue:" + instructionID,
		RequestID: requestID, InstructionID: instructionID, DocumentID: doc.DocID, TrajectoryID: doc.TrajectoryID,
		TargetAgentID: targetAgentID, TargetWorkItemID: targetWorkID,
		ExpectedLifecycleVersion: snapshot.Trajectory.LifecycleVersion, ExpectedHeadRevisionID: input.ExpectedHeadRevisionID,
		Kind: kind, Content: input.Content,
	}
	req.CommandDigest, _ = store.ComputeQueueLifecycleOwnerInstructionDigest(req)
	result, err := h.Store.QueueLifecycleOwnerInstruction(r.Context(), req)
	if err != nil {
		switch {
		case errors.Is(err, store.ErrLifecycleCommandConflict), errors.Is(err, store.ErrConcurrentStateChange):
			writeAPIJSON(w, http.StatusConflict, apiError{Error: "owner instruction replay or lifecycle conflict"})
		case errors.Is(err, store.ErrNotFound):
			writeAPIJSON(w, http.StatusNotFound, apiError{Error: "lifecycle target not found"})
		default:
			writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "owner instruction rejected"})
		}
		return
	}
	instruction := result.OwnerInstruction
	if instruction == nil {
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "owner instruction receipt missing"})
		return
	}
	if !result.Replay {
		if _, wakeErr := h.ReconcileAgentWake(r.Context(), ownerID, doc.DocID); wakeErr != nil {
			// The durable pending instruction remains authoritative; restart/reconcile
			// may wake it without turning a successful commit into a retry ambiguity.
		}
	}
	writeAPIJSON(w, http.StatusAccepted, textureOwnerInstructionResponse{
		Schema: textureOwnerInstructionSchemaV1, InstructionID: instruction.InstructionID, RequestID: instruction.RequestID,
		Kind: instruction.Kind, Status: instruction.Status, DocID: instruction.DocumentID, TrajectoryID: instruction.TrajectoryID,
		TargetAgentID: instruction.TargetAgentID, TargetWorkItemID: instruction.TargetWorkItemID,
		HeadRevisionID: instruction.HeadRevisionID, Cursor: instruction.ReducerSeq, Replay: result.Replay,
	})
}

type textureLifecycleCreateRequest struct {
	ClientRequestID string `json:"client_request_id"`
	Title           string `json:"title"`
	InitialContent  string `json:"initial_content"`
}

type textureLifecycleCreateResponse struct {
	Schema           string `json:"schema"`
	RequestID        string `json:"request_id"`
	DocID            string `json:"doc_id"`
	RevisionID       string `json:"revision_id"`
	TrajectoryID     string `json:"trajectory_id"`
	TargetAgentID    string `json:"target_agent_id"`
	TargetWorkItemID string `json:"target_work_item_id"`
	Cursor           int64  `json:"cursor"`
	Replay           bool   `json:"replay"`
}

func (h *Handler) HandleTextureLifecycleCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}
	var input textureLifecycleCreateRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&input) != nil {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid lifecycle Texture create request"})
		return
	}
	input.ClientRequestID, input.Title, input.InitialContent = strings.TrimSpace(input.ClientRequestID), strings.TrimSpace(input.Title), strings.TrimSpace(input.InitialContent)
	if input.ClientRequestID == "" || input.Title == "" || input.InitialContent == "" || len(input.ClientRequestID) > 256 {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "client_request_id, title, and initial_content are required"})
		return
	}
	if h.Core == nil || strings.TrimSpace(h.Core.TextureSandboxID()) == "" {
		writeAPIJSON(w, http.StatusServiceUnavailable, apiError{Error: "computer identity unavailable"})
		return
	}
	computerID := strings.TrimSpace(h.Core.TextureSandboxID())
	requestID, occurrenceID := textureOwnerOccurrenceIdentity(ownerID, computerID, "create", input.ClientRequestID)
	key := strings.Join([]string{"choir:texture:owner-create", ownerID, computerID, occurrenceID}, ":")
	docID := uuid.NewSHA1(uuid.NameSpaceOID, []byte(key+":document")).String()
	revisionID := uuid.NewSHA1(uuid.NameSpaceOID, []byte(key+":revision:v0")).String()
	workID := uuid.NewSHA1(uuid.NameSpaceOID, []byte(key+":work:initial")).String()
	trajectoryID := uuid.NewSHA1(uuid.NameSpaceOID, []byte(key+":trajectory")).String()
	agentID := "texture:" + docID
	now := time.Now().UTC()
	start := types.StartLifecycleRequest{
		OwnerID: ownerID, ComputerID: computerID, CommandID: "start:" + occurrenceID, TrajectoryID: trajectoryID, Kind: types.TrajectoryKindDocument,
		SubjectRefs:     map[string]string{"artifact": "texture://documents/" + docID, "doc_id": docID},
		SettlementRule:  types.SettlementRule{Version: types.LifecycleReducerVersion, RequireNoOpenWorkItems: true, RequiredSubjectRefs: []string{"artifact"}},
		InitialWork:     types.WorkItemRecord{WorkItemID: workID, Objective: input.InitialContent, AssignedAgentID: agentID, AuthorityProfile: agentprofile.Texture},
		InitialDocument: types.Document{DocID: docID, OwnerID: ownerID, ComputerID: computerID, TrajectoryID: trajectoryID, Title: input.Title, CreatedAt: now, UpdatedAt: now},
		InitialRevision: types.Revision{RevisionID: revisionID, DocID: docID, OwnerID: ownerID, ComputerID: computerID, TrajectoryID: trajectoryID, AuthorKind: types.AuthorUser, AuthorLabel: ownerID, Content: input.InitialContent, CreatedAt: now},
		Agent:           types.AgentRecord{AgentID: agentID, OwnerID: ownerID, ComputerID: computerID, SandboxID: computerID, Profile: agentprofile.Texture, Role: agentprofile.Texture, ChannelID: docID, CreatedAt: now, UpdatedAt: now},
	}
	start.StartRequestDigest, _ = store.ComputeStartLifecycleRequestDigest(start)
	result, err := h.Store.StartLifecycle(r.Context(), start)
	if err != nil {
		if errors.Is(err, store.ErrLifecycleCommandConflict) {
			writeAPIJSON(w, http.StatusConflict, apiError{Error: "create occurrence conflicts with its durable receipt"})
		} else {
			writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "lifecycle Texture create rejected"})
		}
		return
	}
	if !result.Replay {
		if _, wakeErr := h.ReconcileAgentWake(r.Context(), ownerID, docID); wakeErr != nil { /* durable start remains pending */
		}
	}
	writeAPIJSON(w, http.StatusCreated, textureLifecycleCreateResponse{Schema: "choir.texture_create.v1", RequestID: requestID, DocID: docID, RevisionID: revisionID, TrajectoryID: trajectoryID, TargetAgentID: agentID, TargetWorkItemID: workID, Cursor: result.Trajectory.ReducerSeq, Replay: result.Replay})
}
