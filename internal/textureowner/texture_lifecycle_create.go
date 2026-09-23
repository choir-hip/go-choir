package textureowner

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/yusefmosiah/go-choir/internal/agentprofile"

	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)
// textureOwnerOccurrenceIdentity derives the deterministic request/occurrence
// identities for one owner-scoped lifecycle command from its client request id.
// The prefixes are stable: they key durable replay receipts, so they must not
// change across deploys.
func textureOwnerOccurrenceIdentity(ownerID, computerID, docID, clientRequestID string) (string, string) {
	sum := sha256.Sum256([]byte(strings.Join([]string{ownerID, computerID, docID, clientRequestID}, "\x00")))
	id := hex.EncodeToString(sum[:])
	return "owner-request-" + id, "owner-instruction-" + id
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
	if h.Core == nil || strings.TrimSpace(h.Core.TextureComputerID()) == "" {
		writeAPIJSON(w, http.StatusServiceUnavailable, apiError{Error: "computer identity unavailable"})
		return
	}
	computerID := strings.TrimSpace(h.Core.TextureComputerID())
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
		Agent:           types.AgentRecord{AgentID: agentID, OwnerID: ownerID, ComputerID: computerID, Profile: agentprofile.Texture, Role: agentprofile.Texture, ChannelID: docID, CreatedAt: now, UpdatedAt: now},
	}
	start.StartRequestDigest, _ = store.ComputeStartLifecycleRequestDigest(start)
	result, err := h.Store.StartLifecycle(r.Context(), start)
	if err != nil {
		if errors.Is(err, store.ErrLifecycleCommandConflict) {
			writeAPIJSON(w, http.StatusConflict, apiError{Error: "create occurrence conflicts with its durable receipt"})
		} else {
			log.Printf("texture api: lifecycle Texture create rejected: %v", err)
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
