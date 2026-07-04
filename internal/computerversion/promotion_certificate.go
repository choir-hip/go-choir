package computerversion

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

const (
	PromotionLedgerPrepared   = "prepared"
	PromotionLedgerVerified   = "verified"
	PromotionLedgerApplied    = "applied"
	PromotionLedgerRolledBack = "rolled_back"
	PromotionHealthOpen       = "open"
	PromotionHealthConfirmed  = "confirmed"
	PromotionHealthFailed     = "failed"
)

// PromotionLedgerCertificate records one ledger's local evidence state inside a
// promotion certificate. It is an observation schema, not a route mutation.
type PromotionLedgerCertificate struct {
	Name  string `json:"name"`
	State string `json:"state"`
}

// PromotionCertificate is a local, non-runtime observation wrapper for promotion
// evidence over concrete ComputerVersion refs. It does not approve, commit,
// revert, or move a route.
type PromotionCertificate struct {
	ID            string                       `json:"id"`
	RouteSlot     string                       `json:"route_slot"`
	Active        ComputerVersion              `json:"active"`
	Candidate     ComputerVersion              `json:"candidate"`
	Base          ComputerVersion              `json:"base"`
	OwnerApproved bool                         `json:"owner_approved"`
	HealthWindow  string                       `json:"health_window"`
	Ledgers       []PromotionLedgerCertificate `json:"ledgers"`
	RollbackRef   string                       `json:"rollback_ref,omitempty"`
	EvidenceRef   string                       `json:"evidence_ref,omitempty"`
}

// ObservationSet serializes the certificate as scoped evidence. The emitted set
// can be compared like any other ObservationSet, but passing comparison means
// only that the certificate observations match; it does not imply a live route
// was promoted.
func (c PromotionCertificate) ObservationSet(name string) (ObservationSet, error) {
	c = c.Normalize()
	if err := c.Validate(); err != nil {
		return ObservationSet{}, err
	}
	value, err := canonicalPromotionCertificate(c)
	if err != nil {
		return ObservationSet{}, err
	}
	if strings.TrimSpace(name) == "" {
		name = "promotion-certificate"
	}
	return ObservationSet{
		Name:     name,
		Version:  c.Candidate,
		Required: []ObservationKind{ObservationPromotionCertificate},
		Observations: []Observation{{
			Kind:  ObservationPromotionCertificate,
			Key:   "promotion:" + c.ID,
			Value: value,
		}},
	}, nil
}

func (c PromotionCertificate) Normalize() PromotionCertificate {
	c.ID = strings.TrimSpace(c.ID)
	c.RouteSlot = strings.TrimSpace(c.RouteSlot)
	c.HealthWindow = strings.TrimSpace(c.HealthWindow)
	c.RollbackRef = strings.TrimSpace(c.RollbackRef)
	c.EvidenceRef = strings.TrimSpace(c.EvidenceRef)
	for i := range c.Ledgers {
		c.Ledgers[i].Name = strings.TrimSpace(c.Ledgers[i].Name)
		c.Ledgers[i].State = strings.TrimSpace(c.Ledgers[i].State)
	}
	sort.Slice(c.Ledgers, func(i, j int) bool { return c.Ledgers[i].Name < c.Ledgers[j].Name })
	return c
}

func (c PromotionCertificate) Validate() error {
	if c.ID == "" {
		return fmt.Errorf("promotion certificate: id is required")
	}
	if c.RouteSlot == "" {
		return fmt.Errorf("promotion certificate: route slot is required")
	}
	if !c.Active.Valid() {
		return fmt.Errorf("promotion certificate: active ComputerVersion is invalid")
	}
	if !c.Candidate.Valid() {
		return fmt.Errorf("promotion certificate: candidate ComputerVersion is invalid")
	}
	if !c.Base.Valid() {
		return fmt.Errorf("promotion certificate: base ComputerVersion is invalid")
	}
	if c.Active == c.Candidate {
		return fmt.Errorf("promotion certificate: active and candidate ComputerVersion must differ")
	}
	if !c.OwnerApproved {
		return fmt.Errorf("promotion certificate: owner approval is required")
	}
	if !validPromotionHealthWindow(c.HealthWindow) {
		return fmt.Errorf("promotion certificate: unsupported health window %q", c.HealthWindow)
	}
	if len(c.Ledgers) == 0 {
		return fmt.Errorf("promotion certificate: at least one ledger certificate is required")
	}
	seen := make(map[string]struct{}, len(c.Ledgers))
	for _, ledger := range c.Ledgers {
		if ledger.Name == "" {
			return fmt.Errorf("promotion certificate: ledger name is required")
		}
		if _, ok := seen[ledger.Name]; ok {
			return fmt.Errorf("promotion certificate: duplicate ledger %q", ledger.Name)
		}
		seen[ledger.Name] = struct{}{}
		if !validPromotionLedgerState(ledger.State) {
			return fmt.Errorf("promotion certificate: ledger %q has unsupported state %q", ledger.Name, ledger.State)
		}
	}
	return nil
}

func validPromotionHealthWindow(state string) bool {
	switch state {
	case PromotionHealthOpen, PromotionHealthConfirmed, PromotionHealthFailed:
		return true
	default:
		return false
	}
}

func validPromotionLedgerState(state string) bool {
	switch state {
	case PromotionLedgerPrepared, PromotionLedgerVerified, PromotionLedgerApplied, PromotionLedgerRolledBack:
		return true
	default:
		return false
	}
}

func canonicalPromotionCertificate(c PromotionCertificate) (string, error) {
	data, err := json.Marshal(c)
	if err != nil {
		return "", fmt.Errorf("promotion certificate: encode: %w", err)
	}
	return string(data), nil
}
