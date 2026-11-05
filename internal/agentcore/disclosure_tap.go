package agentcore

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// The owner tap (MA3.6): the disclosure path's last step. A disclosure the
// rules table asks about and System 1's distribution does not clear is a
// "pending" disclosure.decision fact on the tape. The owner answers it
// through two owner routes, GET /api/disclosures (what waits, as sentences)
// and POST /api/disclosures/{id}/approve or /deny. Each answer is itself a
// fact on the tape (phase approved or denied, actor owner). Both are owner
// routes: the owner is the authenticated user, and no desk reaches them (the
// ProductAPI allowlist does not list them, and /api/disclosures is blocked by
// name).
//
// Nothing here blocks or resumes. The refused sink returned its fate at the
// time; an approval only lets the next attempt of the same disclosure pass
// once (consumed), and a run a restart closed stays closed.

// disclosureState is the latest step of one disclosure (by digest) that still
// decides a retry.
type disclosureState struct {
	// phase is pending, approved, denied, or empty (nothing standing: never
	// asked, allowed by the rule, or its approval already consumed).
	phase     string
	pendingID string
	payload   disclosurePayload
}

// disclosureFold reads the computer's disclosure facts, oldest first, and
// folds them to the standing state of each digest.
func (rt *Runtime) disclosureFold(ctx context.Context, owner, computer string) (map[string]*disclosureState, []types.EventRecord, error) {
	events, err := rt.store.ListAcknowledgedEventsByChannel(ctx, owner, disclosureChannel(computer), system1TapeLimit)
	if err != nil {
		return nil, nil, err
	}
	states := map[string]*disclosureState{}
	byPending := map[string]string{} // pending event id -> digest
	for _, ev := range events {
		if ev.Kind != types.EventDisclosure {
			continue
		}
		var p disclosurePayload
		if json.Unmarshal(ev.Payload, &p) != nil {
			continue
		}
		switch ev.Phase {
		case disclosurePhasePending:
			byPending[ev.EventID] = p.Digest
			states[p.Digest] = &disclosureState{phase: disclosurePhasePending, pendingID: ev.EventID, payload: p}
		case disclosurePhaseApproved, disclosurePhaseDenied:
			digest := byPending[p.DisclosureID]
			if st := states[digest]; st != nil && st.pendingID == p.DisclosureID {
				st.phase = ev.Phase
			}
		case disclosurePhaseConsumed:
			if digest := byPending[p.DisclosureID]; digest != "" {
				if st := states[digest]; st != nil && st.pendingID == p.DisclosureID && st.phase == disclosurePhaseApproved {
					st.phase = ""
				}
			}
		}
	}
	return states, events, nil
}

// disclosureState is the standing state of one disclosure.
func (rt *Runtime) disclosureState(ctx context.Context, owner, computer, digest string) (disclosureState, error) {
	states, _, err := rt.disclosureFold(ctx, owner, computer)
	if err != nil {
		return disclosureState{}, err
	}
	if st := states[digest]; st != nil {
		return *st, nil
	}
	return disclosureState{}, nil
}

// pendingDisclosure is one waiting disclosure as the owner sees it.
type pendingDisclosure struct {
	DisclosureID string            `json:"disclosure_id"`
	Sentence     string            `json:"sentence"`
	Payload      disclosurePayload `json:"disclosure"`
}

func (p disclosurePayload) sentence() string {
	yes, no, unsure := p.Distribution["yes"], p.Distribution["no"], p.Distribution["unsure"]
	return fmt.Sprintf("Desk %s wants to send data labelled %s to %s (%s): %s System 1 gave yes %.2f, no %.2f, unsure %.2f.",
		p.Desk, strings.Join(p.Labels, ", "), p.Target, p.Sink, p.Summary, yes, no, unsure)
}

type disclosureListResponse struct {
	Disclosures []pendingDisclosure `json:"disclosures"`
}

// HandleDisclosures serves GET /api/disclosures: the disclosures that wait on
// the owner.
func (h *APIHandler) HandleDisclosures(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}
	if h.rt == nil || h.rt.store == nil {
		writeAPIJSON(w, http.StatusOK, disclosureListResponse{Disclosures: []pendingDisclosure{}})
		return
	}
	states, _, err := h.rt.disclosureFold(r.Context(), ownerID, h.rt.TextureComputerID())
	if err != nil {
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: err.Error()})
		return
	}
	out := disclosureListResponse{Disclosures: []pendingDisclosure{}}
	for _, st := range states {
		if st.phase == disclosurePhasePending {
			out.Disclosures = append(out.Disclosures, pendingDisclosure{DisclosureID: st.pendingID, Sentence: st.payload.sentence(), Payload: st.payload})
		}
	}
	writeAPIJSON(w, http.StatusOK, out)
}

// HandleDisclosureRouter serves POST /api/disclosures/{id}/approve and
// /deny: the owner's answer, recorded as a fact on the tape.
func (h *APIHandler) HandleDisclosureRouter(w http.ResponseWriter, r *http.Request) {
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}
	rest := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/disclosures/"), "/")
	parts := strings.Split(rest, "/")
	if len(parts) != 2 || parts[0] == "" || (parts[1] != "approve" && parts[1] != "deny") {
		http.NotFound(w, r)
		return
	}
	id, action := parts[0], parts[1]
	if r.Method != http.MethodPost {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	phase, err := h.rt.answerDisclosure(r.Context(), ownerID, id, action == "approve")
	if err != nil {
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: err.Error()})
		return
	}
	writeAPIJSON(w, http.StatusOK, map[string]string{"disclosure_id": id, "state": phase})
}

// answerDisclosure records the owner's answer to a pending disclosure of the
// owner's own computer. Only a pending disclosure can be answered, and once.
func (rt *Runtime) answerDisclosure(ctx context.Context, owner, id string, approve bool) (string, error) {
	if rt == nil || rt.store == nil {
		return "", fmt.Errorf("no store")
	}
	rt.disclosureMu.Lock()
	defer rt.disclosureMu.Unlock()
	computer := strings.TrimSpace(rt.TextureComputerID())
	states, _, err := rt.disclosureFold(ctx, owner, computer)
	if err != nil {
		return "", err
	}
	var st *disclosureState
	for _, s := range states {
		if s.pendingID == id {
			st = s
		}
	}
	if st == nil || st.phase != disclosurePhasePending {
		return "", fmt.Errorf("no such pending disclosure")
	}
	phase := disclosurePhaseDenied
	if approve {
		phase = disclosurePhaseApproved
	}
	answer := st.payload
	answer.DisclosureID, answer.Actor = id, "owner"
	if _, err := rt.appendDisclosure(ctx, toolregistry.ExecutionContext{OwnerID: owner, ComputerID: computer, RunID: st.payload.RunID, AgentID: "owner"}, owner, computer, phase, answer); err != nil {
		return "", err
	}
	return phase, nil
}
