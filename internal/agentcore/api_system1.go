package agentcore

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/yusefmosiah/go-choir/internal/jevrecord"
)

// POST /api/system1/judgments: a signed-in client (a phone app on a device
// key) asks System 1 a few closed questions through its own computer. It is
// the in-guest path every Jev site uses (judgeCall, interactive scope), so
// the gateway's limiter bounds it and the Jev record keeps it
// (docs/design/autoradio-system1-route-2026-11-10.md).
const (
	system1RouteMaxBody         = 64 << 10
	system1RouteMaxState        = 48 << 10
	system1RouteMaxQuestions    = 12
	system1RouteMaxChoices      = 8
	system1RouteMaxInstructions = 2000
	system1RouteDeadline        = 4 * time.Second
)

var system1RouteLabel = regexp.MustCompile(`^[a-z0-9_.-]{1,40}$`)

type system1RouteQuestion struct {
	Type         string          `json:"type"`
	Instructions string          `json:"instructions"`
	Criteria     json.RawMessage `json:"criteria,omitempty"`
	Choices      []string        `json:"choices"`
}

type system1RouteRequest struct {
	State     json.RawMessage                 `json:"state"`
	Questions map[string]system1RouteQuestion `json:"questions"`
	Label     string                          `json:"label,omitempty"`
}

type system1RouteAnswer struct {
	Choice        string             `json:"choice,omitempty"`
	Confidence    float64            `json:"confidence,omitempty"`
	Probabilities map[string]float64 `json:"probabilities,omitempty"`
	Error         string             `json:"error,omitempty"`
}

type system1RouteResponse struct {
	Answers    map[string]system1RouteAnswer `json:"answers"`
	UpstreamMS int64                         `json:"upstream_ms"`
	TotalMS    int64                         `json:"total_ms"`
	RecordID   string                        `json:"record_id,omitempty"`
}

func (h *APIHandler) HandleSystem1Judgments(w http.ResponseWriter, r *http.Request) {
	started := time.Now()
	if r.Method != http.MethodPost {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	if _, err := authenticateUser(r); err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}
	rt := h.rt
	if rt == nil || rt.candidateJudge == nil {
		writeAPIJSON(w, http.StatusServiceUnavailable, apiError{Error: "no decision model is bound on this computer"})
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, system1RouteMaxBody))
	if err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			writeAPIJSON(w, http.StatusRequestEntityTooLarge, apiError{Error: fmt.Sprintf("body exceeds %d bytes", system1RouteMaxBody)})
			return
		}
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "cannot read the body"})
		return
	}
	req, state, questions, err := parseSystem1Route(body)
	if err != nil {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: err.Error()})
		return
	}
	site := "client"
	if req.Label != "" {
		site += ":" + req.Label
	}
	recordID := uuid.New().String()
	upstream := time.Now()
	raw, err := rt.judgeCall(r.Context(), jevrecord.Meta{Site: site, DecisionID: recordID, Scope: judgmentScopeInteractive}, system1RouteDeadline, state, questions)
	upstreamMS := time.Since(upstream).Milliseconds()
	if err != nil {
		msg := system1Message(err)
		if strings.Contains(msg, "rate limit exceeded") || strings.Contains(msg, "queue full") {
			w.Header().Set("Retry-After", "1")
			writeAPIJSON(w, http.StatusTooManyRequests, apiError{Error: "judgment rate limit exceeded"})
			return
		}
		writeAPIJSON(w, http.StatusBadGateway, apiError{Error: msg})
		return
	}
	var distribution jevDistribution
	if err := json.Unmarshal(raw, &distribution); err != nil {
		writeAPIJSON(w, http.StatusBadGateway, apiError{Error: "the judge's answer did not decode"})
		return
	}
	out := system1RouteResponse{Answers: make(map[string]system1RouteAnswer, len(req.Questions)), UpstreamMS: upstreamMS, RecordID: recordID}
	for name, q := range req.Questions {
		given, ok := distribution.Answers[name]
		switch {
		case !ok:
			out.Answers[name] = system1RouteAnswer{Error: "the judge gave no answer to this question"}
		case !slices.Contains(q.Choices, given.Choice):
			out.Answers[name] = system1RouteAnswer{Error: "the judge's choice is not one offered"}
		default:
			out.Answers[name] = system1RouteAnswer{Choice: given.Choice, Confidence: given.Probabilities[given.Choice], Probabilities: given.Probabilities}
		}
	}
	out.TotalMS = time.Since(started).Milliseconds()
	writeAPIJSON(w, http.StatusOK, out)
}

// parseSystem1Route validates the body against the route's limits and
// returns the gateway-shaped state and questions.
func parseSystem1Route(body []byte) (system1RouteRequest, json.RawMessage, map[string]json.RawMessage, error) {
	var req system1RouteRequest
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		return req, nil, nil, fmt.Errorf("decode the request: %v", err)
	}
	if dec.More() {
		return req, nil, nil, errors.New("decode the request: trailing data")
	}
	state := bytes.TrimSpace(req.State)
	if len(state) == 0 || state[0] != '{' {
		return req, nil, nil, errors.New("state must be a JSON object")
	}
	if len(state) > system1RouteMaxState {
		return req, nil, nil, fmt.Errorf("state exceeds %d bytes", system1RouteMaxState)
	}
	if n := len(req.Questions); n == 0 || n > system1RouteMaxQuestions {
		return req, nil, nil, fmt.Errorf("questions must number 1 to %d", system1RouteMaxQuestions)
	}
	if req.Label != "" && !system1RouteLabel.MatchString(req.Label) {
		return req, nil, nil, errors.New("label must be 1 to 40 characters of a-z 0-9 _ . -")
	}
	questions := make(map[string]json.RawMessage, len(req.Questions))
	for name, q := range req.Questions {
		if strings.TrimSpace(name) == "" || len(name) > 64 {
			return req, nil, nil, errors.New("question names must be 1 to 64 bytes")
		}
		if q.Type != "choice" {
			return req, nil, nil, fmt.Errorf("question %q: only type choice is supported", name)
		}
		if strings.TrimSpace(q.Instructions) == "" || len(q.Instructions) > system1RouteMaxInstructions {
			return req, nil, nil, fmt.Errorf("question %q: instructions must be 1 to %d bytes", name, system1RouteMaxInstructions)
		}
		if len(q.Choices) < 2 || len(q.Choices) > system1RouteMaxChoices {
			return req, nil, nil, fmt.Errorf("question %q: choices must number 2 to %d", name, system1RouteMaxChoices)
		}
		seen := map[string]bool{}
		for _, c := range q.Choices {
			if strings.TrimSpace(c) == "" || seen[c] {
				return req, nil, nil, fmt.Errorf("question %q: choices must be unique and non-empty", name)
			}
			seen[c] = true
		}
		raw, err := json.Marshal(q)
		if err != nil {
			return req, nil, nil, err
		}
		questions[name] = raw
	}
	return req, json.RawMessage(state), questions, nil
}
