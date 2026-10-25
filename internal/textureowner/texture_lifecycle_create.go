package textureowner

import (
	"context"
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
	// Desk selects the bound desk; "texture" (the default) is the only one.
	// The engineering-bound document (path B) is deleted (C1e step 3):
	// engineering work opens from management's Cast.
	Desk string `json:"desk,omitempty"`
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
	if strings.HasPrefix(input.ClientRequestID, textureRootClientRequestPrefix) {
		// Reserved: the master documents' ids follow from these request ids.
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "client_request_id prefix " + textureRootClientRequestPrefix + " is reserved"})
		return
	}
	if h.Core == nil || strings.TrimSpace(h.Core.TextureComputerID()) == "" {
		writeAPIJSON(w, http.StatusServiceUnavailable, apiError{Error: "computer identity unavailable"})
		return
	}
	computerID := strings.TrimSpace(h.Core.TextureComputerID())
	deskProfile := strings.TrimSpace(input.Desk)
	if deskProfile != "" && deskProfile != agentprofile.Texture {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "desk must be texture"})
		return
	}
	created, err := h.startLifecycleTextureDocument(ownerTextContext(r.Context(), r), ownerID, computerID, input.ClientRequestID, input.Title, input.InitialContent)
	if err != nil {
		if errors.Is(err, store.ErrLifecycleCommandConflict) {
			writeAPIJSON(w, http.StatusConflict, apiError{Error: "create occurrence conflicts with its durable receipt"})
		} else {
			log.Printf("texture api: lifecycle Texture create rejected: %v", err)
			writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "lifecycle Texture create rejected"})
		}
		return
	}
	if !created.Replay {
		if _, wakeErr := h.ReconcileAgentWake(r.Context(), ownerID, created.DocID); wakeErr != nil { /* durable start remains pending */
		}
	}
	writeAPIJSON(w, http.StatusCreated, created)
}

// textureCreateIdentity derives the ordinary create path's durable ids from
// one client request id, so a caller can know the document id before it
// creates the document (the Texture root's ensure step claims its marker
// first).
func textureCreateIdentity(ownerID, computerID, clientRequestID string) (requestID, occurrenceID, docID, revisionID, workID, trajectoryID string) {
	requestID, occurrenceID = textureOwnerOccurrenceIdentity(ownerID, computerID, "create", clientRequestID)
	key := strings.Join([]string{"choir:texture:owner-create", ownerID, computerID, occurrenceID}, ":")
	docID = uuid.NewSHA1(uuid.NameSpaceOID, []byte(key+":document")).String()
	revisionID = uuid.NewSHA1(uuid.NameSpaceOID, []byte(key+":revision:v0")).String()
	workID = uuid.NewSHA1(uuid.NameSpaceOID, []byte(key+":work:initial")).String()
	trajectoryID = uuid.NewSHA1(uuid.NameSpaceOID, []byte(key+":trajectory")).String()
	return requestID, occurrenceID, docID, revisionID, workID, trajectoryID
}

// startLifecycleTextureDocument is the ordinary create path's durable step:
// one document trajectory with an owner-authored v0, its texture:<doc> desk
// and the one initial work item that desk holds. It wakes nothing.
func (h *Handler) startLifecycleTextureDocument(ctx context.Context, ownerID, computerID, clientRequestID, title, initialContent string) (textureLifecycleCreateResponse, error) {
	requestID, occurrenceID, docID, revisionID, workID, trajectoryID := textureCreateIdentity(ownerID, computerID, clientRequestID)
	deskProfile := agentprofile.Texture
	agentID := deskProfile + ":" + docID
	now := time.Now().UTC()
	start := types.StartLifecycleRequest{
		OwnerID: ownerID, ComputerID: computerID, CommandID: "start:" + occurrenceID, TrajectoryID: trajectoryID, Kind: types.TrajectoryKindDocument,
		SubjectRefs:     map[string]string{"artifact": "texture://documents/" + docID, "doc_id": docID},
		SettlementRule:  types.SettlementRule{Version: types.LifecycleReducerVersion, RequireNoOpenWorkItems: true, RequiredSubjectRefs: []string{"artifact"}},
		InitialWork:     types.WorkItemRecord{WorkItemID: workID, Objective: initialContent, AssignedAgentID: agentID, AuthorityProfile: deskProfile},
		InitialDocument: types.Document{DocID: docID, OwnerID: ownerID, ComputerID: computerID, TrajectoryID: trajectoryID, Title: title, CreatedAt: now, UpdatedAt: now},
		InitialRevision: types.Revision{RevisionID: revisionID, DocID: docID, OwnerID: ownerID, ComputerID: computerID, TrajectoryID: trajectoryID, AuthorKind: types.AuthorUser, AuthorLabel: ownerID, Content: initialContent, CreatedAt: now},
		Agent:           types.AgentRecord{AgentID: agentID, OwnerID: ownerID, ComputerID: computerID, Profile: deskProfile, Role: deskProfile, ChannelID: docID, CreatedAt: now, UpdatedAt: now},
	}
	start.StartRequestDigest, _ = store.ComputeStartLifecycleRequestDigest(start)
	result, err := h.Store.StartLifecycle(ctx, start)
	if err != nil {
		return textureLifecycleCreateResponse{}, err
	}
	return textureLifecycleCreateResponse{Schema: "choir.texture_create.v1", RequestID: requestID, DocID: docID, RevisionID: revisionID, TrajectoryID: trajectoryID, TargetAgentID: agentID, TargetWorkItemID: workID, Cursor: result.Trajectory.ReducerSeq, Replay: result.Replay}, nil
}
