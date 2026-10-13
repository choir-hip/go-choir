package agentcore

import (
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// Owner-scoped read of one inbox's records (Cut 1 L3a; the trace bundle's
// records layer). Records are the work state of the conversations on
// records; the guest store stays the authority and the route is read-only.

// inboxRecordsResponse is the JSON body of GET /api/inboxes/{inbox}/records.
// NextAfter is the seq to pass as ?after= for the next page, or 0 when the
// page is the last one. Head is the inbox head; Cursor is the reading
// desk's cursor on its own inbox.
type inboxRecordsResponse struct {
	Inbox      string                   `json:"inbox"`
	Head       int64                    `json:"head"`
	Cursor     int64                    `json:"cursor"`
	Records    []types.CommitmentRecord `json:"records"`
	OpenRounds []types.ConversationHead `json:"open_rounds"`
	NextAfter  int64                    `json:"next_after"`
}

// HandleInboxRecords serves GET /api/inboxes/{inbox}/records?after=<seq>&limit=<n>.
// The inbox is a desk agent id (texture:<doc>, research:<doc>).
func (h *APIHandler) HandleInboxRecords(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}
	rest := strings.TrimPrefix(r.URL.Path, "/api/inboxes/")
	inbox, tail, ok := strings.Cut(rest, "/")
	if !ok || tail != "records" || strings.TrimSpace(inbox) == "" {
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "not found"})
		return
	}
	after := int64(0)
	if raw := strings.TrimSpace(r.URL.Query().Get("after")); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || parsed < 0 {
			writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid record cursor"})
			return
		}
		after = parsed
	}
	limit := int64(clampRunEventsLimit(r.URL.Query().Get("limit")))
	ctx := r.Context()
	computerID := h.rt.TextureComputerID()
	st := h.rt.Store()
	head, err := st.InboxHead(ctx, ownerID, computerID, inbox)
	if err != nil {
		log.Printf("runtime api: inbox head: %v", err)
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to read inbox"})
		return
	}
	cursor, err := st.ReaderCursor(ctx, ownerID, computerID, inbox, inbox)
	if err != nil {
		log.Printf("runtime api: inbox cursor: %v", err)
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to read inbox"})
		return
	}
	resp := inboxRecordsResponse{Inbox: inbox, Head: head, Cursor: cursor, Records: []types.CommitmentRecord{}, OpenRounds: []types.ConversationHead{}}
	through := head
	if through > after+limit {
		through = after + limit
		resp.NextAfter = through
	}
	if through > after {
		recs, err := st.ListInboxRecords(ctx, ownerID, computerID, inbox, after, through)
		if err != nil {
			log.Printf("runtime api: list inbox records: %v", err)
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to list inbox records"})
			return
		}
		resp.Records = append(resp.Records, recs...)
	}
	if rounds, err := st.OpenRounds(ctx, ownerID, computerID, inbox); err == nil {
		resp.OpenRounds = append(resp.OpenRounds, rounds...)
	}
	writeAPIJSON(w, http.StatusOK, resp)
}
