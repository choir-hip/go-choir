package agentcore

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"

	"github.com/yusefmosiah/go-choir/internal/types"
)

type liveWSConnectedMessage struct {
	Type      string `json:"type"`
	DesktopID string `json:"desktop_id"`
}

type liveWSEventMessage struct {
	Type         string          `json:"type"`
	EventID      string          `json:"event_id"`
	Seq          int64           `json:"seq"`
	StreamSeq    int64           `json:"stream_seq"`
	Timestamp    string          `json:"ts"`
	RunID        string          `json:"loop_id,omitempty"`
	AgentID      string          `json:"agent_id,omitempty"`
	ChannelID    string          `json:"channel_id,omitempty"`
	TrajectoryID string          `json:"trajectory_id,omitempty"`
	Kind         types.EventKind `json:"kind"`
	Phase        string          `json:"phase,omitempty"`
	Payload      json.RawMessage `json:"payload"`
}

type liveWSAckMessage struct {
	Type      string `json:"type"`
	StreamSeq int64  `json:"stream_seq,omitempty"`
}

var liveWSUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		// Same-origin auth and routing are enforced by the edge proxy. The
		// sandbox runtime should not second-guess proxy origin policy.
		return true
	},
}

// HandleLiveWS upgrades /api/ws into the authenticated user-computer live bus.
// The event table remains canonical; the websocket only delivers notification
// and owner-scoped catch-up messages.
func (h *APIHandler) HandleLiveWS(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}
	desktopID := requestDesktopID(r)
	afterSeq := parseLiveAfterSeq(r)

	conn, err := liveWSUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer func() { _ = conn.Close() }()

	if err := conn.WriteJSON(liveWSConnectedMessage{
		Type:      "connected",
		DesktopID: desktopID,
	}); err != nil {
		return
	}

	if afterSeq > 0 {
		records, err := h.rt.Store().ListEventsByOwnerAfter(r.Context(), ownerID, afterSeq, 500)
		if err != nil {
			log.Printf("runtime live ws: catch-up query failed for %s: %v", ownerID, err)
			return
		}
		for _, rec := range records {
			if !liveEventMatchesDesktop(rec, desktopID) {
				continue
			}
			if err := conn.WriteJSON(liveEventMessage(rec)); err != nil {
				return
			}
		}
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		conn.SetReadLimit(32 * 1024)
		for {
			_, raw, err := conn.ReadMessage()
			if err != nil {
				return
			}
			var msg liveWSAckMessage
			if err := json.Unmarshal(raw, &msg); err == nil && strings.EqualFold(msg.Type, "ack") {
				continue
			}
		}
	}()

	sub := h.rt.EventBus().SubscribeWithBuffer(256)
	defer h.rt.EventBus().Unsubscribe(sub)

	ping := time.NewTicker(30 * time.Second)
	defer ping.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-done:
			return
		case <-ping.C:
			if err := conn.WriteJSON(liveWSAckMessage{Type: "ping"}); err != nil {
				return
			}
		case ev := <-sub:
			rec := ev.Record
			if strings.TrimSpace(rec.OwnerID) != ownerID {
				continue
			}
			if !liveEventMatchesDesktop(rec, desktopID) {
				continue
			}
			if err := conn.WriteJSON(liveEventMessage(rec)); err != nil {
				return
			}
		}
	}
}

func parseLiveAfterSeq(r *http.Request) int64 {
	raw := strings.TrimSpace(r.URL.Query().Get("after_seq"))
	if raw == "" {
		return 0
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || n < 0 {
		return 0
	}
	return n
}

func liveEventMessage(rec types.EventRecord) liveWSEventMessage {
	payload := rec.Payload
	if len(payload) == 0 {
		payload = json.RawMessage(`{}`)
	}
	return liveWSEventMessage{
		Type:         "event",
		EventID:      rec.EventID,
		Seq:          rec.Seq,
		StreamSeq:    rec.StreamSeq,
		Timestamp:    rec.Timestamp.UTC().Format(time.RFC3339Nano),
		RunID:        rec.RunID,
		AgentID:      rec.AgentID,
		ChannelID:    rec.ChannelID,
		TrajectoryID: rec.TrajectoryID,
		Kind:         rec.Kind,
		Phase:        rec.Phase,
		Payload:      payload,
	}
}

func liveEventMatchesDesktop(rec types.EventRecord, desktopID string) bool {
	desktopID = strings.TrimSpace(desktopID)
	if desktopID == "" || desktopID == types.PrimaryDesktopID {
		return true
	}
	if len(rec.Payload) == 0 {
		return true
	}
	var payload struct {
		DesktopID string `json:"desktop_id"`
	}
	if err := json.Unmarshal(rec.Payload, &payload); err != nil {
		return true
	}
	return strings.TrimSpace(payload.DesktopID) == "" || strings.TrimSpace(payload.DesktopID) == desktopID
}
