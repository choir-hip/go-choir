package yaegikernel

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// ReceiptDisposition represents the consequential outcome of an operation.
type ReceiptDisposition string

const (
	DispositionSuccess ReceiptDisposition = "success"
	DispositionFailure ReceiptDisposition = "failure"
	DispositionRefused ReceiptDisposition = "refused"
	DispositionDeath   ReceiptDisposition = "activation_death"
	DispositionRewarm  ReceiptDisposition = "activation_rewarm"
)

// SalientReceipt represents an immutable, host-selected consequential execution event.
type SalientReceipt struct {
	ReceiptID   string             `json:"receipt_id"`
	Action      BrokerAction       `json:"action"`
	ActorID     string             `json:"actor_id"`
	Epoch       uint64             `json:"epoch"`
	Summary     string             `json:"summary"`
	Disposition ReceiptDisposition `json:"disposition"`
	Timestamp   time.Time          `json:"timestamp"`
	EvidenceRef string             `json:"evidence_ref,omitempty"`
}

// ReceiptCollector manages host-owned salient receipts and produces Texture transclusions.
type ReceiptCollector struct {
	mu       sync.Mutex
	receipts []*SalientReceipt
}

// NewReceiptCollector creates a new receipt collector.
func NewReceiptCollector() *ReceiptCollector {
	return &ReceiptCollector{
		receipts: make([]*SalientReceipt, 0),
	}
}

// Record captures a consequential execution receipt.
func (c *ReceiptCollector) Record(receipt *SalientReceipt) error {
	if receipt == nil || receipt.ReceiptID == "" || receipt.ActorID == "" {
		return fmt.Errorf("receipt collector: invalid receipt")
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	if receipt.Timestamp.IsZero() {
		receipt.Timestamp = time.Now().UTC()
	}
	c.receipts = append(c.receipts, receipt)
	return nil
}

// ListSalient returns all receipts matching the host salient policy (every failure,
// refusal, activation death/rewarm, and state mutation).
func (c *ReceiptCollector) ListSalient() []*SalientReceipt {
	c.mu.Lock()
	defer c.mu.Unlock()

	results := make([]*SalientReceipt, len(c.receipts))
	copy(results, c.receipts)
	return results
}

// FormatTextureTransclusion converts a salient receipt into an immutable canonical
// Texture document transclusion block.
func FormatTextureTransclusion(r *SalientReceipt) string {
	if r == nil {
		return ""
	}
	return fmt.Sprintf(
		"```choir:transclusion\nreceipt_id: %s\naction: %s\nactor_id: %s\nepoch: %d\ndisposition: %s\ntimestamp: %s\nsummary: %s\n```\n",
		r.ReceiptID,
		r.Action,
		r.ActorID,
		r.Epoch,
		r.Disposition,
		r.Timestamp.Format(time.RFC3339Nano),
		strings.TrimSpace(r.Summary),
	)
}

// GenerateTextureEvidenceSection renders all salient receipts into a unified Markdown evidence section.
func (c *ReceiptCollector) GenerateTextureEvidenceSection() string {
	receipts := c.ListSalient()
	if len(receipts) == 0 {
		return "## Execution Evidence\n*No consequential execution events recorded.*\n"
	}

	var sb strings.Builder
	sb.WriteString("## Execution Evidence & Transcluded Receipts\n\n")
	for _, r := range receipts {
		sb.WriteString(FormatTextureTransclusion(r))
		sb.WriteString("\n")
	}
	return sb.String()
}
