// Package actorruntime adapts the durable actor runtime (internal/actor) to
// the surface that cmd/autoputer/main.go expects from the old runtime
// (internal/runtime).

// The Adapter retains a named runtime core for business logic (tool loops,
// coagent spawning, state transitions, wire synthesis) and replaces the
// runtime's concurrency substrate (startRunAsync, channels, agentWaiters,
// 15 mutexes) with the actor runtime's single-mutex mailbox model.
//
// The actor handler (handler.go) is the execution boundary: HandleUpdate calls
// runtime.ExecuteActivationSync synchronously. The actor goroutine IS the run
// goroutine. Park-resume is via the actor's memory snapshot (a compact resume
// pointer; the store holds the full conversation history).
package actorruntime

import (
	"context"
	"database/sql"
	"encoding/binary"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"

	"github.com/yusefmosiah/go-choir/internal/actor"
	"github.com/yusefmosiah/go-choir/internal/agentcore"
	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/events"
	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/textureowner"
	"github.com/yusefmosiah/go-choir/internal/trace"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// RuntimeOption configures optional adapter components.
type RuntimeOption func(*Adapter)

// WithTraceStore mounts a Dolt-backed trace observability store into the
// runtime core so trace events are persisted alongside existing event
// recording. This is a passthrough to runtime.WithTraceStore; the adapter does
// not own the store connection (the caller manages the *sql.DB lifecycle).
func WithTraceStore(s trace.Store) RuntimeOption {
	return func(a *Adapter) {
		agentcore.WithTraceStore(s)(a.Runtime)
	}
}

// WithOnActorFailure sets a callback invoked when an actor's activation dies
// from a panic. The callback receives the agent ID and the error. It must
// not block. When not set, failures are logged only.
func WithOnActorFailure(fn func(agentID string, err error)) RuntimeOption {
	return func(a *Adapter) {
		if fn != nil {
			a.onActorFailure = fn
		}
	}
}

// WithKernelMode is retained for source compatibility but is a no-op: the
// dispatcher is always the sole delivery authority (ontology kernel). The
// legacy channel-mailbox runtime was removed at the cutover.
func WithKernelMode() RuntimeOption {
	return func(a *Adapter) {}
}

// Adapter owns actor dispatch and lifecycle around an explicitly named runtime
// business-logic core. Naming the field keeps the runtime method set from being
// promoted onto the adapter.
//
// The Adapter sets a dispatch function on the runtime core: when the business
// logic activates a run or wakes a coagent, the dispatch function sends actor
// messages through actor.Send.
type Adapter struct {
	Runtime *agentcore.Runtime

	cfg          provideriface.Config
	store        *store.Store
	bus          *events.EventBus
	provider     provideriface.Provider
	actorRT      *actor.Runtime
	log          *actor.SQLiteLog
	handler      *actorHandler
	textureOwner *textureowner.Handler
	logDB        *sql.DB
	logPath      string

	// Actor runtime options (applied before actorRT construction).
	onActorFailure actor.FailureFunc // supervisor callback for actor deaths
	startOnce      sync.Once
	started        bool

	dispatchMu     sync.Mutex
	dispatchReady  bool
	bootDispatches []actor.Update
}

// New constructs the adapter. The runtime core's ActorBridge is set to the
// adapter, so run activations and coagent wakes go through actor.Send.
func New(cfg provideriface.Config, s *store.Store, bus *events.EventBus, provider provideriface.Provider, coreOpts []agentcore.RuntimeOption, opts ...RuntimeOption) *Adapter {
	rt := agentcore.New(cfg, s, bus, provider, coreOpts...)

	a := &Adapter{
		Runtime:  rt,
		cfg:      cfg,
		store:    s,
		bus:      bus,
		provider: provider,
	}

	for _, opt := range opts {
		opt(a)
	}

	// Open a separate SQLite database for the actor durable log. The store
	// uses Dolt (MySQL-compatible); the actor log uses SQLite. The file
	// lives alongside the store so it survives restarts.
	logPath := actorLogPath(s.Path())
	logDB, err := sql.Open("sqlite", logPath+"?_pragma=busy_timeout(60000)&_pragma=journal_mode(WAL)")
	if err != nil {
		log.Fatalf("actorruntime: open actor log db: %v", err)
	}
	logDB.SetMaxOpenConns(1)
	logDB.SetMaxIdleConns(1)
	actorLog, err := actor.NewSQLiteLog(logDB)
	if err != nil {
		_ = logDB.Close()
		log.Fatalf("actorruntime: init actor log schema: %v", err)
	}
	a.log = actorLog
	a.logDB = logDB
	a.logPath = logPath

	// Create the handler and actor runtime. Texture ownership is bound by the
	// composition root before Start.
	handler := newActorHandler(a.Runtime, nil)
	a.handler = handler
	actorOpts := actor.Options{}
	if a.onActorFailure != nil {
		actorOpts.OnActorFailure = a.onActorFailure
	}
	// The dispatcher is the sole delivery authority (ontology kernel). The
	// legacy channel-mailbox runtime and its boot Sweep are removed - pending
	// delivery is a tape projection, not a scan.
	// Bound delivery: a wake that fails (or crashes the process) MaxAttempts
	// times is routed to the dead-letter sink mailbox instead of looping
	// forever. The sink is a scoped mailbox whose unknown-kind handler drops
	// delivery_failed events (actorruntime/handler.go default case).
	sinkMailbox := scopedActorMailboxID(cfg.ComputerID, cfg.ComputerID, "delivery-poison")
	a.actorRT = actor.NewKernelRuntime(actorLog, actorLog, handler, actorOpts, actor.DispatcherOptions{
		MaxAttempts: 8,
		ErrorSink:   sinkMailbox,
	})
	rt.SetKernelMode()

	// Wire the dispatch function. From this point, rt.activate(rec)
	// sends an actor message and rt.wakeUpdatedCoagent(...) sends an
	// actor message. No fallback path exists.
	rt.SetCheckedDispatchActor(a.dispatch)
	rt.SetScheduleActor(a.schedule)

	return a
}

func actorDispatchUpdateID(ownerID, computerID, toAgentID, kind, content, trajectoryID, fromAgentID string) string {
	canonicalKind := strings.TrimSpace(kind)
	if canonicalKind == "coagent_result" {
		return types.ActorWakeUpdateID(ownerID, computerID, toAgentID, canonicalKind, content, trajectoryID, fromAgentID)
	}
	canonicalContent := strings.TrimSpace(content)
	if canonicalContent == "" {
		return uuid.New().String()
	}
	// Deterministic content-derived identity for every kind: the logical wake
	// {to,kind,content,trajectory,from} maps to one update_id, so a recovery
	// sweep re-minting the same wake after a restart hits the same durable
	// row (and attempt budget) instead of escaping the poison bound. Two
	// distinct occurrences with identical content are the same logical wake.

	// Length prefixes make the occurrence identity injective even when an
	// authored identifier contains a delimiter used by an older encoding.
	identity := make([]byte, 0, 256)
	for _, field := range []string{
		"choir:actor-dispatch:v2",
		strings.TrimSpace(ownerID),
		strings.TrimSpace(computerID),
		strings.TrimSpace(toAgentID),
		canonicalKind,
		canonicalContent,
		strings.TrimSpace(trajectoryID),
		strings.TrimSpace(fromAgentID),
	} {
		identity = binary.AppendUvarint(identity, uint64(len(field)))
		identity = append(identity, field...)
	}
	return uuid.NewSHA1(uuid.NameSpaceOID, identity).String()
}

var errTextureRecoveryBaseNotProcessed = errors.New("actorruntime: Texture recovery base is not processed")

// canonicalTextureDispatch upgrades every Texture wake to a complete
// Store-derived occurrence. Compatibility with the pre-repair producer digest
// exists only at this boundary: the durable actor row always receives the new
// exact identity and authenticated envelope.
func (a *Adapter) canonicalTextureDispatch(ctx context.Context, ownerID, computerID, toAgentID, kind, content, trajectoryID, fromAgentID string) (string, string, string, error) {
	if strings.TrimSpace(kind) != "coagent_result" || !strings.HasPrefix(strings.TrimSpace(toAgentID), agentprofile.Texture+":") {
		return content, trajectoryID, fromAgentID, nil
	}
	if occurrence, err := agentcore.DecodeTextureActorOccurrence(content); err == nil {
		if occurrence.OwnerID != ownerID || occurrence.ComputerID != computerID || occurrence.TargetAgentID != toAgentID {
			return "", "", "", fmt.Errorf("actorruntime: Texture occurrence envelope scope mismatch")
		}
		if strings.TrimSpace(trajectoryID) != "" && occurrence.TrajectoryID != strings.TrimSpace(trajectoryID) {
			return "", "", "", fmt.Errorf("actorruntime: Texture occurrence trajectory mismatch")
		}
		source := occurrence.ProducerAgentID
		if occurrence.Kind == agentcore.TextureActorOccurrenceDocumentRevision {
			source = "owner:" + occurrence.OwnerID
		}
		if strings.TrimSpace(fromAgentID) != "" && strings.TrimSpace(fromAgentID) != source {
			return "", "", "", fmt.Errorf("actorruntime: Texture occurrence source mismatch")
		}
		if occurrence.RecoveryRunID != "" || occurrence.RecoveryTailID != "" || occurrence.RecoveryHeadID != "" || occurrence.RecoveryMutation != "" {
			base := occurrence
			base.RecoveryRunID, base.RecoveryTailID, base.RecoveryHeadID, base.RecoveryMutation = "", "", "", ""
			baseContent, encodeErr := agentcore.EncodeTextureActorOccurrence(base)
			if encodeErr != nil {
				return "", "", "", encodeErr
			}
			baseID := actorDispatchUpdateID(ownerID, computerID, toAgentID, kind, baseContent, occurrence.TrajectoryID, source)
			exists, processed, statusErr := a.log.UpdateStatus(ctx, scopedActorMailboxID(ownerID, computerID, toAgentID), baseID)
			if statusErr != nil {
				return "", "", "", fmt.Errorf("actorruntime: inspect Texture recovery base: %w", statusErr)
			}
			if !exists || !processed {
				return "", "", "", errTextureRecoveryBaseNotProcessed
			}
		}
		return content, occurrence.TrajectoryID, source, nil
	}

	// Pre-repair producer wakes carried only the v2 update digest. Resolve it
	// against the exact target's pending canonical rows, refusing ambiguity.
	updates, err := a.store.ListAllPendingLifecycleUpdates(ctx, ownerID, computerID, toAgentID)
	if err != nil {
		return "", "", "", fmt.Errorf("actorruntime: resolve Texture producer occurrence: %w", err)
	}
	var matched *types.CoagentSourcePacket
	for i := range updates {
		candidate := updates[i]
		if candidate.Direction != types.LifecyclePacketDirectionProducerReport {
			continue
		}
		if strings.TrimSpace(content) == agentcore.LifecycleControlActorOccurrenceContent(candidate) ||
			strings.TrimSpace(content) == strings.TrimSpace(candidate.UpdateID) ||
			strings.TrimSpace(content) == strings.TrimSpace(candidate.Content) {
			if matched != nil {
				return "", "", "", fmt.Errorf("actorruntime: ambiguous legacy Texture producer occurrence")
			}
			copy := candidate
			matched = &copy
		}
	}
	if matched != nil {
		o, occurrenceErr := agentcore.TextureProducerReportOccurrence(*matched)
		if occurrenceErr != nil {
			return "", "", "", occurrenceErr
		}
		encoded, encodeErr := agentcore.EncodeTextureActorOccurrence(o)
		if encodeErr != nil {
			return "", "", "", encodeErr
		}
		return encoded, o.TrajectoryID, o.ProducerAgentID, nil
	}

	// Pre-repair owner wakes carried only the revision id and no authenticated
	// trajectory/source. Resolve the exact document-bound owner revision.
	docID := strings.TrimPrefix(strings.TrimSpace(toAgentID), agentprofile.Texture+":")
	doc, docErr := a.store.GetLifecycleDocument(ctx, ownerID, computerID, docID)
	if docErr == nil && strings.TrimSpace(doc.TrajectoryID) != "" {
		revision, revErr := a.store.GetLifecycleRevision(ctx, ownerID, computerID, strings.TrimSpace(content))
		if revErr == nil && revision.AuthorKind == types.AuthorUser && revision.DocID == docID {
			o, occurrenceErr := agentcore.TextureDocumentRevisionOccurrence(revision, "", 0, 0)
			if occurrenceErr != nil {
				return "", "", "", occurrenceErr
			}
			encoded, encodeErr := agentcore.EncodeTextureActorOccurrence(o)
			if encodeErr != nil {
				return "", "", "", encodeErr
			}
			return encoded, o.TrajectoryID, "owner:" + o.OwnerID, nil
		}
	}
	return "", "", "", fmt.Errorf("actorruntime: Texture wake has no exact pending canonical occurrence")
}

// dispatch is the function hook that the runtime core calls to send actor
// messages. It is set via rt.SetCheckedDispatchActor(a.dispatch).
func (a *Adapter) dispatch(ctx context.Context, ownerID, computerID, toAgentID, kind, content, trajectoryID, fromAgentID string) error {
	return a.dispatchAt(ctx, ownerID, computerID, toAgentID, kind, content, trajectoryID, fromAgentID, time.Time{})
}

// schedule appends a durable deferred update. Runtime calls it only in kernel
// mode, where actor.Runtime.Send hands delivery to the dispatcher due-index.
func (a *Adapter) schedule(ctx context.Context, ownerID, computerID, toAgentID, kind, content, trajectoryID, fromAgentID string, notBefore time.Time) error {
	if notBefore.IsZero() {
		return fmt.Errorf("actorruntime: schedule requires not_before")
	}
	return a.dispatchAt(ctx, ownerID, computerID, toAgentID, kind, content, trajectoryID, fromAgentID, notBefore.UTC())
}

func (a *Adapter) dispatchAt(ctx context.Context, ownerID, computerID, toAgentID, kind, content, trajectoryID, fromAgentID string, notBefore time.Time) error {
	ownerID, computerID, toAgentID = strings.TrimSpace(ownerID), strings.TrimSpace(computerID), strings.TrimSpace(toAgentID)
	if ownerID == "" || computerID == "" || toAgentID == "" {
		return fmt.Errorf("actorruntime: dispatch: owner_id, computer_id, and to_agent_id are required")
	}
	var canonicalErr error
	content, trajectoryID, fromAgentID, canonicalErr = a.canonicalTextureDispatch(ctx, ownerID, computerID, toAgentID, kind, content, trajectoryID, fromAgentID)
	if canonicalErr != nil {
		if errors.Is(canonicalErr, errTextureRecoveryBaseNotProcessed) {
			return nil
		}
		return canonicalErr
	}
	updateID := actorDispatchUpdateID(ownerID, computerID, toAgentID, kind, content, trajectoryID, fromAgentID)
	u := actor.Update{
		UpdateID:     updateID,
		ToAgentID:    scopedActorMailboxID(ownerID, computerID, toAgentID),
		FromAgentID:  fromAgentID,
		Kind:         kind,
		Content:      content,
		TrajectoryID: trajectoryID,
		CreatedAt:    time.Now().UTC(),
		NotBefore:    notBefore,
	}
	a.dispatchMu.Lock()
	if !a.dispatchReady {
		// Before actor delivery is released, dispatch success means the exact
		// occurrence is already durable in SQLite. A crash cannot lose a cold
		// activation or canonical lifecycle trigger in a process-local queue.
		_, err := a.log.Append(ctx, u)
		a.dispatchMu.Unlock()
		if err != nil {
			return fmt.Errorf("actorruntime: boot dispatch append: %w", err)
		}
		return nil
	}
	a.dispatchMu.Unlock()
	// Inside a dispatcher activation, emissions buffer into the fenced
	// commit: they become durable only when the activation's {emitted + head}
	// commit succeeds. Outside an activation (boot/API-initiated), send
	// immediately.
	if buf := actor.EmissionsFromCtx(ctx); buf != nil {
		buf.Add(u)
		return nil
	}
	return a.actorRT.Send(ctx, u)
}

func (a *Adapter) flushBootDispatches(ctx context.Context) error {
	for {
		a.dispatchMu.Lock()
		if len(a.bootDispatches) == 0 {
			a.dispatchReady = true
			a.dispatchMu.Unlock()
			return nil
		}
		pending := a.bootDispatches
		a.bootDispatches = nil
		a.dispatchMu.Unlock()

		for index, update := range pending {
			attempt := 0
			for {
				err := a.actorRT.Send(ctx, update)
				if err == nil {
					break
				}
				attempt++
				if attempt == 1 || attempt%20 == 0 {
					log.Printf("actorruntime: retry boot dispatch to %s: %v", update.ToAgentID, err)
				}
				timer := time.NewTimer(50 * time.Millisecond)
				select {
				case <-ctx.Done():
					if !timer.Stop() {
						<-timer.C
					}
					a.dispatchMu.Lock()
					a.bootDispatches = append(pending[index:], a.bootDispatches...)
					a.dispatchMu.Unlock()
					return ctx.Err()
				case <-timer.C:
				}
			}
		}
	}
}

// BindTextureOwner installs the concrete Texture lifecycle owner before actor
// processing begins. It is a direct owner composition, not a callback seam.
func (a *Adapter) BindTextureOwner(owner *textureowner.Handler) error {
	if owner == nil {
		return fmt.Errorf("actorruntime: bind Texture owner: nil owner")
	}
	if a.started {
		return fmt.Errorf("actorruntime: bind Texture owner after start")
	}
	a.textureOwner = owner
	a.handler.textureOwner = owner
	return nil
}

func (a *Adapter) migrateLegacyActorMailboxes(ctx context.Context) error {
	mailboxIDs, err := a.log.MailboxIdentities(ctx)
	if err != nil {
		return fmt.Errorf("inspect durable mailbox identities: %w", err)
	}
	plan := make([]actor.MailboxRebind, 0)
	var orphans []string
	for _, mailboxID := range mailboxIDs {
		if _, _, _, parseErr := parseScopedActorMailboxID(mailboxID); parseErr == nil {
			continue
		}
		agent, resolveErr := a.store.ResolveLegacyAgentScope(ctx, a.cfg.ComputerID, mailboxID)
		if resolveErr != nil {
			if errors.Is(resolveErr, store.ErrNotFound) {
				orphans = append(orphans, mailboxID)
				continue
			}
			return fmt.Errorf("resolve legacy durable mailbox %q: %w", mailboxID, resolveErr)
		}
		plan = append(plan, actor.MailboxRebind{
			LegacyID: mailboxID,
			ScopedID: scopedActorMailboxID(agent.OwnerID, agent.ComputerID, agent.AgentID),
		})
	}
	if len(orphans) > 0 {
		pruned, pruneErr := a.log.PruneMailboxes(ctx, orphans)
		if pruneErr != nil {
			return fmt.Errorf("prune orphaned legacy durable mailboxes: %w", pruneErr)
		}
		if pruned {
			log.Printf("actorruntime: pruned %d orphaned legacy durable mailboxes", len(orphans))
		}
	}
	migrated, err := a.log.RebindMailboxes(ctx, plan)
	if err != nil {
		return fmt.Errorf("rebind legacy durable mailboxes: %w", err)
	}
	if migrated {
		log.Printf("actorruntime: rebound %d legacy durable mailbox identities to owner/computer scope", len(plan))
	}
	mailboxIDs, err = a.log.MailboxIdentities(ctx)
	if err != nil {
		return fmt.Errorf("verify durable mailbox identities: %w", err)
	}
	for _, mailboxID := range mailboxIDs {
		if _, _, _, err := parseScopedActorMailboxID(mailboxID); err != nil {
			return fmt.Errorf("unsupported legacy durable mailbox %q: %w", mailboxID, err)
		}
	}
	return nil
}

// recoverParkedLifecycleMailboxSnapshots re-appends the exact canonical
// lifecycle-control occurrence only when a parked snapshot and a durable
// pending control agree. ReconcileParkedLifecycleCoagentWake derives the same
// content, and actorDispatchUpdateID is content-deterministic, so this boot
// convergence can only hit the existing SQLite tape row; it cannot mint a
// process-local wake or a second delivery identity.
func (a *Adapter) recoverParkedLifecycleMailboxSnapshots(ctx context.Context) error {
	mailboxIDs, err := a.log.MailboxIdentities(ctx)
	if err != nil {
		return fmt.Errorf("inspect lifecycle actor snapshots: %w", err)
	}
	for _, mailboxID := range mailboxIDs {
		ownerID, computerID, agentID, scopeErr := parseScopedActorMailboxID(mailboxID)
		if scopeErr != nil {
			return scopeErr
		}
		memory, loadErr := a.log.LoadSnapshot(ctx, mailboxID)
		if loadErr != nil {
			return fmt.Errorf("load lifecycle actor snapshot %q: %w", mailboxID, loadErr)
		}
		resume, decodeErr := decodeResumeState(memory)
		if decodeErr != nil {
			return fmt.Errorf("decode lifecycle actor snapshot %q: %w", mailboxID, decodeErr)
		}
		if strings.TrimSpace(resume.RunID) == "" {
			continue
		}
		rec, runErr := a.store.GetLifecycleRun(ctx, ownerID, computerID, resume.RunID)
		if runErr != nil {
			if errors.Is(runErr, store.ErrNotFound) {
				continue
			}
			return fmt.Errorf("load lifecycle actor snapshot run %s: %w", resume.RunID, runErr)
		}
		snapProfile, _ := agentprofile.Canonical(rec.AgentProfile)
		snapRole, _ := agentprofile.Canonical(rec.AgentRole)
		if rec.OwnerID != ownerID || rec.ComputerID != computerID || rec.AgentID != agentID ||
			snapProfile != agentprofile.Research ||
			snapRole != agentprofile.Research ||
			strings.TrimSpace(metadataString(rec.Metadata, "request_source")) != "lifecycle_texture_control" ||
			(rec.State != types.RunPassivated && rec.State != types.RunBlocked) {
			continue
		}
		pending, pendingErr := a.store.ListAllPendingLifecycleUpdates(ctx, ownerID, computerID, agentID)
		if pendingErr != nil {
			return fmt.Errorf("list lifecycle actor snapshot pending controls for run %s: %w", resume.RunID, pendingErr)
		}
		if len(pending) == 0 {
			continue
		}
		recovered, recoverErr := a.Runtime.ReconcileParkedLifecycleCoagentWake(ctx, ownerID, agentID, resume.RunID)
		if recoverErr != nil {
			return fmt.Errorf("recover lifecycle actor snapshot run %s: %w", resume.RunID, recoverErr)
		}
		if recovered == nil || strings.TrimSpace(recovered.RunID) != strings.TrimSpace(resume.RunID) ||
			strings.TrimSpace(metadataString(recovered.Metadata, "request_source")) != "lifecycle_texture_control" {
			return fmt.Errorf("recover lifecycle actor snapshot run %s returned noncanonical run", resume.RunID)
		}
	}
	return nil
}

// Start keeps actor delivery paused while the generic core and concrete Texture
// owner reconcile durable state. Only after both scans finish are boot
// dispatches released and the actor log swept.
func (a *Adapter) Start(ctx context.Context) error {
	if err := a.migrateLegacyActorMailboxes(ctx); err != nil {
		return fmt.Errorf("actorruntime: %w", err)
	}
	if a.textureOwner == nil {
		subjects, err := a.Runtime.Store().ListLifecycleSubjects(ctx, a.Runtime.TextureComputerID())
		if err != nil {
			return fmt.Errorf("actorruntime: inspect durable Texture subjects: %w", err)
		}
		for _, subject := range subjects {
			subjectProfile, _ := agentprofile.Canonical(subject.Profile)
			subjectRole, _ := agentprofile.Canonical(subject.Role)
			if subjectProfile == agentprofile.Texture ||
				subjectRole == agentprofile.Texture ||
				strings.HasPrefix(strings.TrimSpace(subject.AgentID), agentprofile.Texture+":") {
				return fmt.Errorf("actorruntime: Texture owner is not bound for durable subject %s", subject.AgentID)
			}
		}
	}
	if err := a.recoverParkedLifecycleMailboxSnapshots(ctx); err != nil {
		return fmt.Errorf("actorruntime: recover lifecycle actor snapshots: %w", err)
	}
	// Refresh-runtime may intentionally remove the persistent updater current
	// pointer. Rejoin the immutable baseline before the service is considered
	// started; failures remain observable as a fail-closed 503 at the surface.
	for attempt := range 10 {
		if err := a.Runtime.EnsureComputerSurface(ctx); err == nil {
			break
		} else if attempt == 9 {
			log.Printf("actorruntime: computer surface baseline bootstrap deferred: %v", err)
		} else {
			select {
			case <-ctx.Done():
				break
			case <-time.After(250 * time.Millisecond):
			}
		}
	}
	a.Runtime.Start(ctx)
	if a.textureOwner != nil {
		if err := a.textureOwner.Start(ctx); err != nil {
			return fmt.Errorf("actorruntime: reconcile Texture owner: %w", err)
		}
	}
	if err := a.flushBootDispatches(ctx); err != nil {
		return fmt.Errorf("actorruntime: boot dispatch flush: %w", err)
	}
	// Kernel mode: the dispatcher's pending projection is the recovery rule -
	// no boot sweep. Start the dispatcher loop.
	a.actorRT.StartKernel(ctx)
	a.startOnce.Do(func() { a.started = true })
	return nil
}

// Stop gracefully shuts down the actor runtime and the runtime core.
func (a *Adapter) Stop() {
	a.actorRT.Stop()
	a.Runtime.Stop()
	if a.logDB != nil {
		_ = a.logDB.Close()
	}
}

// Drain gracefully shuts down the actor runtime with a timeout, then stops
// the runtime core. In-flight actor handlers receive a cancellation context;
// actors that do not finish within the timeout are logged (their partial side
// effects are visible in the durable log). This is the backpressure-aware
// alternative to Stop.
func (a *Adapter) Drain(timeout time.Duration) {
	a.actorRT.Drain(timeout)
	a.Runtime.Stop()
	if a.logDB != nil {
		_ = a.logDB.Close()
	}
}

// ActorRuntime returns the underlying actor runtime (for diagnostics/tests).
func (a *Adapter) ActorRuntime() *actor.Runtime {
	return a.actorRT
}

// actorLogPath derives the actor log SQLite file path from the store path.
func actorLogPath(storePath string) string {
	dir := filepath.Dir(storePath)
	base := filepath.Base(storePath)
	return filepath.Join(dir, base+"-actor.db")
}

// Cleanup removes the actor log file (for tests).
func (a *Adapter) cleanupLog() {
	if a.logDB != nil {
		_ = a.logDB.Close()
		a.logDB = nil
	}
	if a.logPath != "" {
		_ = os.Remove(a.logPath)
		_ = os.Remove(a.logPath + "-wal")
		_ = os.Remove(a.logPath + "-shm")
	}
}
