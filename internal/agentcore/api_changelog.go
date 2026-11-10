package agentcore

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// The changelog app's read (autopaper contracts 5.2, lane E). The app reads
// the owner's changelog inbox through this one route; it never reads corpusd.
//
//	GET /api/changelog?minor=1&before=<seq>&limit=<n>
//	PUT /api/changelog/seen   {"seq": n}
//
// The owner is the authenticated user and nothing else. A page is newest
// first, bounded, and the scan behind it is bounded too. The seen cursor is a
// user preference on the owner's computer (durable, shared by every browser),
// moves only forward and never past the inbox head.

const (
	changelogDefaultLimit = 50
	changelogMaxLimit     = 200
	// changelogScanWindows bounds the inbox windows one request reads while
	// it looks for matching events (a long run of minor events under the
	// major-only view).
	changelogScanWindows = 8
	// changelogUnseenCap bounds the unseen major count.
	changelogUnseenCap = 100
)

type changelogEventJSON struct {
	InboxSeq  int64                     `json:"inbox_seq"`
	Level     types.ChangelogLevel      `json:"level"`
	Changelog types.CommitmentChangelog `json:"changelog"`
}

type changelogResponse struct {
	Events []changelogEventJSON `json:"events"`
	// NextBefore is the cursor for the next (older) page; zero when there is none.
	NextBefore  int64 `json:"next_before"`
	Head        int64 `json:"head"`
	SeenSeq     int64 `json:"seen_seq"`
	UnseenMajor int   `json:"unseen_major"`
	UnseenMore  bool  `json:"unseen_more"`
}

func changelogSeenKey(computerID string) string { return "changelog.seen." + computerID }

func (h *APIHandler) changelogSeen(r *http.Request, ownerID, computerID string) int64 {
	pref, err := h.rt.Store().GetUserPreference(r.Context(), ownerID, changelogSeenKey(computerID))
	if err != nil {
		return 0
	}
	switch v := pref.Value["seq"].(type) {
	case float64:
		if v > 0 {
			return int64(v)
		}
	case int64:
		return v
	}
	return 0
}

// HandleChangelog serves GET /api/changelog and PUT /api/changelog/seen.
func (h *APIHandler) HandleChangelog(w http.ResponseWriter, r *http.Request) {
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}
	computerID := h.rt.TextureComputerID()
	st := h.rt.Store()
	inbox := types.ChangelogInbox(ownerID)
	head, err := st.InboxHead(r.Context(), ownerID, computerID, inbox)
	if err != nil {
		log.Printf("runtime api: changelog head: %v", err)
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to read changelog"})
		return
	}
	switch {
	case r.URL.Path == "/api/changelog/seen" && r.Method == http.MethodPut:
		h.putChangelogSeen(w, r, ownerID, computerID, head)
		return
	case r.URL.Path == "/api/changelog" && r.Method == http.MethodGet:
	case r.URL.Path == "/api/changelog" || r.URL.Path == "/api/changelog/seen":
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	default:
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "not found"})
		return
	}

	q := r.URL.Query()
	includeMinor := q.Get("minor") == "1" || q.Get("minor") == "true"
	limit := changelogDefaultLimit
	if n, err := strconv.Atoi(strings.TrimSpace(q.Get("limit"))); err == nil && n > 0 {
		limit = n
	}
	if limit > changelogMaxLimit {
		limit = changelogMaxLimit
	}
	through := head
	if raw := strings.TrimSpace(q.Get("before")); raw != "" {
		b, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || b < 0 {
			writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid cursor"})
			return
		}
		if b-1 < through {
			through = b - 1
		}
	}

	resp := changelogResponse{Events: []changelogEventJSON{}, Head: head}
	// Walk backwards one window at a time until the page is full, the inbox
	// is exhausted or the scan bound is spent.
	floor := through
	for win := 0; win < changelogScanWindows && floor > 0 && len(resp.Events) < limit; win++ {
		after := floor - int64(limit)
		if after < 0 {
			after = 0
		}
		recs, err := st.ListInboxRecords(r.Context(), ownerID, computerID, inbox, after, floor)
		if err != nil {
			log.Printf("runtime api: changelog list: %v", err)
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to read changelog"})
			return
		}
		evs := types.FilterChangelogRecords(recs, types.ChangelogQuery{IncludeMinor: includeMinor})
		floor = after
		for i := len(evs) - 1; i >= 0; i-- {
			if len(resp.Events) == limit {
				// The page filled mid-window: the rest is older than the last event.
				floor = resp.Events[len(resp.Events)-1].InboxSeq - 1
				break
			}
			resp.Events = append(resp.Events, changelogEventJSON{InboxSeq: evs[i].InboxSeq, Level: evs[i].Level, Changelog: evs[i].Changelog})
		}
	}
	if floor > 0 {
		resp.NextBefore = floor + 1
	}
	resp.SeenSeq = h.changelogSeen(r, ownerID, computerID)
	if head > resp.SeenSeq {
		unseen, err := st.ListInboxRecords(r.Context(), ownerID, computerID, inbox, resp.SeenSeq, head)
		if err == nil {
			evs := types.FilterChangelogRecords(unseen, types.ChangelogQuery{Limit: changelogUnseenCap + 1})
			resp.UnseenMajor = len(evs)
			if resp.UnseenMajor > changelogUnseenCap {
				resp.UnseenMajor, resp.UnseenMore = changelogUnseenCap, true
			}
		}
	}
	writeAPIJSON(w, http.StatusOK, resp)
}

func (h *APIHandler) putChangelogSeen(w http.ResponseWriter, r *http.Request, ownerID, computerID string, head int64) {
	var body struct {
		Seq int64 `json:"seq"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<10)).Decode(&body); err != nil || body.Seq < 0 {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid seq"})
		return
	}
	if body.Seq > head {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "seq is past the changelog head"})
		return
	}
	seen := h.changelogSeen(r, ownerID, computerID)
	if body.Seq > seen {
		seen = body.Seq
		if _, err := h.rt.Store().SaveUserPreference(r.Context(), types.UserPreference{
			OwnerID: ownerID, PreferenceKey: changelogSeenKey(computerID), Value: map[string]any{"seq": seen},
		}); err != nil {
			log.Printf("runtime api: changelog seen: %v", err)
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to save the seen cursor"})
			return
		}
	}
	writeAPIJSON(w, http.StatusOK, map[string]int64{"seen_seq": seen})
}
