package yaegikernel

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

// ActorStatus represents the lifecycle state of a durable accountable actor.
type ActorStatus string

const (
	ActorStatusIdle       ActorStatus = "idle"
	ActorStatusActive     ActorStatus = "active"
	ActorStatusPassivated ActorStatus = "passivated"
	ActorStatusDead       ActorStatus = "dead"
)

// AccountableActor represents the persistent identity and continuity anchor for an agent.
type AccountableActor struct {
	ActorID      string      `json:"actor_id"`
	Profile      string      `json:"profile"`
	CurrentEpoch uint64      `json:"current_epoch"`
	ModelID      string      `json:"model_id"`
	Status       ActorStatus `json:"status"`
	CreatedAt    time.Time   `json:"created_at"`
	UpdatedAt    time.Time   `json:"updated_at"`
}

// DurableAssignment records an open or completed task assigned to an actor.
type DurableAssignment struct {
	AssignmentID string     `json:"assignment_id"`
	ActorID      string     `json:"actor_id"`
	ParentTaskID string     `json:"parent_task_id,omitempty"`
	Instruction  string     `json:"instruction"`
	Status       string     `json:"status"`
	CreatedAt    time.Time  `json:"created_at"`
	CompletedAt  *time.Time `json:"completed_at,omitempty"`
}

// DurableMessage records a typed inter-agent communication packet.
type DurableMessage struct {
	MessageID   string     `json:"message_id"`
	SenderID    string     `json:"sender_id"`
	RecipientID string     `json:"recipient_id"`
	Kind        string     `json:"kind"`
	Body        string     `json:"body"`
	DeliveredAt time.Time  `json:"delivered_at"`
	AckedAt     *time.Time `json:"acked_at,omitempty"`
}

// DurableObligation records an accountability requirement that must be satisfied before settlement.
type DurableObligation struct {
	ObligationID string    `json:"obligation_id"`
	ActorID      string    `json:"actor_id"`
	Description  string    `json:"description"`
	Satisfied    bool      `json:"satisfied"`
	EvidenceRef  string    `json:"evidence_ref,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

// ActorStateManager manages durable actor lifecycles, rewarm across activation deaths,
// and epoch fencing.
type ActorStateManager struct {
	mu          sync.Mutex
	actors      map[string]*AccountableActor
	assignments map[string][]*DurableAssignment
	messages    map[string][]*DurableMessage
	obligations map[string][]*DurableObligation
}

// NewActorStateManager creates a new in-memory durable actor state manager.
func NewActorStateManager() *ActorStateManager {
	return &ActorStateManager{
		actors:      make(map[string]*AccountableActor),
		assignments: make(map[string][]*DurableAssignment),
		messages:    make(map[string][]*DurableMessage),
		obligations: make(map[string][]*DurableObligation),
	}
}

// RegisterActor creates or updates an accountable actor.
func (m *ActorStateManager) RegisterActor(actorID, profile, modelID string) (*AccountableActor, error) {
	if strings.TrimSpace(actorID) == "" {
		return nil, fmt.Errorf("actor state: actor_id is required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now().UTC()
	if existing, ok := m.actors[actorID]; ok {
		existing.ModelID = modelID
		existing.UpdatedAt = now
		return existing, nil
	}

	actor := &AccountableActor{
		ActorID:      actorID,
		Profile:      profile,
		CurrentEpoch: 1,
		ModelID:      modelID,
		Status:       ActorStatusIdle,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	m.actors[actorID] = actor
	return actor, nil
}

// RecordAssignment stores a durable assignment for an actor.
func (m *ActorStateManager) RecordAssignment(assignment *DurableAssignment) error {
	if assignment == nil || assignment.AssignmentID == "" || assignment.ActorID == "" {
		return fmt.Errorf("actor state: invalid assignment")
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	if assignment.CreatedAt.IsZero() {
		assignment.CreatedAt = time.Now().UTC()
	}
	m.assignments[assignment.ActorID] = append(m.assignments[assignment.ActorID], assignment)
	return nil
}

// RecordMessage stores a durable inter-agent message.
func (m *ActorStateManager) RecordMessage(msg *DurableMessage) error {
	if msg == nil || msg.MessageID == "" || msg.RecipientID == "" {
		return fmt.Errorf("actor state: invalid message")
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	if msg.DeliveredAt.IsZero() {
		msg.DeliveredAt = time.Now().UTC()
	}
	m.messages[msg.RecipientID] = append(m.messages[msg.RecipientID], msg)
	return nil
}

// RecordObligation stores an obligation for an actor.
func (m *ActorStateManager) RecordObligation(ob *DurableObligation) error {
	if ob == nil || ob.ObligationID == "" || ob.ActorID == "" {
		return fmt.Errorf("actor state: invalid obligation")
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	if ob.CreatedAt.IsZero() {
		ob.CreatedAt = time.Now().UTC()
	}
	m.obligations[ob.ActorID] = append(m.obligations[ob.ActorID], ob)
	return nil
}

// RewarmActor advances the activation epoch monotonically, invalidates old handles,
// and returns all open assignments, pending messages, and unsatisfied obligations
// to reconstruct accountability without restoring interpreter heap state.
func (m *ActorStateManager) RewarmActor(ctx context.Context, actorID string, newModelID string) (
	actor *AccountableActor,
	openAssignments []*DurableAssignment,
	pendingMessages []*DurableMessage,
	unsatisfiedObligations []*DurableObligation,
	err error,
) {
	m.mu.Lock()
	defer m.mu.Unlock()

	actor, ok := m.actors[actorID]
	if !ok {
		return nil, nil, nil, nil, fmt.Errorf("actor state: actor %q not found", actorID)
	}

	// 1. Monotonically increment epoch for fencing
	actor.CurrentEpoch++
	if newModelID != "" {
		actor.ModelID = newModelID
	}
	actor.Status = ActorStatusActive
	actor.UpdatedAt = time.Now().UTC()

	// 2. Discover open assignments
	for _, a := range m.assignments[actorID] {
		if a.Status != "completed" && a.Status != "cancelled" {
			openAssignments = append(openAssignments, a)
		}
	}

	// 3. Discover unacknowledged messages
	for _, msg := range m.messages[actorID] {
		if msg.AckedAt == nil {
			pendingMessages = append(pendingMessages, msg)
		}
	}

	// 4. Discover unsatisfied obligations
	for _, ob := range m.obligations[actorID] {
		if !ob.Satisfied {
			unsatisfiedObligations = append(unsatisfiedObligations, ob)
		}
	}

	return actor, openAssignments, pendingMessages, unsatisfiedObligations, nil
}
