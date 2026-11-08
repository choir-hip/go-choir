package maildesk

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// HTTPMailbox is the Mailbox of a guest computer: maild is a host service, so
// the guest reads the owner's mail through maild's message route over its tap
// (RUNTIME_MAILD_URL). maild binds the tap caller to a live ownership for the
// asserted owner (S1a) and serves a guest read-only; nothing is copied into
// the guest. The Mail custodian service decides whether a desk may ask; this
// only fetches the one ref it has decided.
type HTTPMailbox struct {
	base   string
	client *http.Client
}

// NewHTTPMailbox returns the mailbox of the maild service at base.
func NewHTTPMailbox(base string) *HTTPMailbox {
	return &HTTPMailbox{base: strings.TrimRight(strings.TrimSpace(base), "/"), client: &http.Client{Timeout: 15 * time.Second}}
}

const maxMailBytes = 4 << 20

// Message fetches one message of ownerID's mailbox by ref.
func (m *HTTPMailbox) Message(ctx context.Context, ownerID, ref string) (Message, error) {
	if m == nil || m.base == "" {
		return Message{}, errors.New("maildesk: no maild on this computer")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, m.base+"/api/email/messages/"+url.PathEscape(ref), nil)
	if err != nil {
		return Message{}, err
	}
	req.Header.Set("X-Authenticated-User", ownerID)
	resp, err := m.client.Do(req)
	if err != nil {
		return Message{}, fmt.Errorf("maildesk: maild: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return Message{}, ErrNoMessage
	}
	if resp.StatusCode != http.StatusOK {
		return Message{}, fmt.Errorf("maildesk: maild answered %d", resp.StatusCode)
	}
	var d struct {
		Message struct {
			ID          string `json:"id"`
			FromAddress string `json:"from_address"`
			FromDisplay string `json:"from_display"`
			Subject     string `json:"subject"`
			TrustStatus string `json:"trust_status"`
			ReceivedAt  string `json:"received_at"`
			SentAt      string `json:"sent_at"`
		} `json:"message"`
		TextBody string `json:"text_body"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxMailBytes)).Decode(&d); err != nil {
		return Message{}, fmt.Errorf("maildesk: maild answer: %w", err)
	}
	if strings.TrimSpace(d.Message.ID) == "" {
		return Message{}, fmt.Errorf("maildesk: maild answered a message with no id: %w", ErrNoMessage)
	}
	at := d.Message.ReceivedAt
	if at == "" {
		at = d.Message.SentAt
	}
	return Message{Ref: d.Message.ID, From: d.Message.FromAddress, FromDisplay: d.Message.FromDisplay, Subject: d.Message.Subject,
		TextBody: d.TextBody, ReceivedAt: at, TrustStatus: d.Message.TrustStatus}, nil
}
