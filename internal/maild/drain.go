package maild

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

// TargetResolver resolves a computer's inbound HTTP endpoint and authorization capability.
type TargetResolver func(ctx context.Context, computerID string) (guestURL, capabilityToken string, err error)

// DrainWorker delivers spooled messages asynchronously to running MicroVMs.
type DrainWorker struct {
	queue    *SpoolQueue
	resolver TargetResolver
	client   *http.Client
}

// NewDrainWorker creates a new asynchronous mail drain worker.
func NewDrainWorker(queue *SpoolQueue, resolver TargetResolver) *DrainWorker {
	return &DrainWorker{
		queue:    queue,
		resolver: resolver,
		client:   &http.Client{Timeout: 15 * time.Second},
	}
}

// DrainOnce processes one batch of pending spooled messages. Returns number of
// successfully delivered messages.
func (w *DrainWorker) DrainOnce(ctx context.Context, maxBatch int) (int, error) {
	if w == nil || w.queue == nil || w.resolver == nil {
		return 0, fmt.Errorf("drain worker: uninitialized")
	}

	pending, err := w.queue.FetchPending(ctx, maxBatch)
	if err != nil {
		return 0, fmt.Errorf("drain worker: fetch pending: %w", err)
	}

	delivered := 0
	for _, msg := range pending {
		if err := w.deliverSingle(ctx, msg); err != nil {
			// Schedule exponential backoff on failure (1s, 2s, 4s, 8s, up to 1m)
			delay := time.Duration(1<<msg.Attempts) * time.Second
			if delay > time.Minute {
				delay = time.Minute
			}
			_ = w.queue.RecordAttemptFailure(ctx, msg.ID, delay)
			continue
		}
		if err := w.queue.MarkDelivered(ctx, msg.ID); err == nil {
			delivered++
		}
	}
	return delivered, nil
}

func (w *DrainWorker) deliverSingle(ctx context.Context, msg *SpooledMessage) error {
	rawEML, err := os.ReadFile(msg.RawPath)
	if err != nil {
		return fmt.Errorf("read raw eml: %w", err)
	}

	guestURL, token, err := w.resolver(ctx, msg.ComputerID)
	if err != nil {
		return fmt.Errorf("resolve computer endpoint: %w", err)
	}
	guestURL = strings.TrimRight(guestURL, "/")

	reqBody := map[string]string{
		"message_id": msg.MessageID,
		"recipient":  msg.Recipient,
		"sender":     msg.Sender,
		"subject":    msg.Subject,
		"raw_eml":    string(rawEML),
	}
	jsonBytes, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	inboundURL := guestURL + "/api/mail/inbound"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, inboundURL, bytes.NewReader(jsonBytes))
	if err != nil {
		return fmt.Errorf("create http request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := w.client.Do(req)
	if err != nil {
		return fmt.Errorf("do http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("guest returned status %d", resp.StatusCode)
	}
	return nil
}
