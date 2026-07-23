package routeledger

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerversion"
	"github.com/yusefmosiah/go-choir/internal/store"
)

const (
	testApprovalRef    ApprovalRef             = "approval:sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	testCertificateRef PromotionCertificateRef = "certificate:sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
)

func TestParseRouteSlotIDRejectsNonCanonicalComponents(t *testing.T) {
	for _, slotID := range []string{
		"computer:owner\ninjected:primary",
		"computer:owner:primary\x00",
		"computer: owner:primary",
	} {
		if _, _, err := ParseRouteSlotID(slotID); err == nil {
			t.Fatalf("ParseRouteSlotID(%q) accepted non-canonical input", slotID)
		}
	}
	ownerID, computerID, err := ParseRouteSlotID("computer:owner:primary")
	if err != nil || ownerID != "owner" || computerID != "primary" {
		t.Fatalf("canonical route slot parse=(%q,%q,%v)", ownerID, computerID, err)
	}
}

func TestMemoryLedgerTransitionContract(t *testing.T) {
	ledger := NewMemoryLedger()
	slotID := mustSlotID(t, "owner-a", "primary")
	v1 := version("code:one", "program:one")
	v2 := version("code:two", "program:two")

	bootstrap := TransitionCommand{RouteSlotID: slotID,
		Kind: TransitionBootstrap,
		New:  v1, ApprovalRef: testApprovalRef, PromotionCertificateRef: testCertificateRef, IdempotencyKey: "idempotency:bootstrap-a"}
	slot, first, err := ledger.Transition(context.Background(), bootstrap)
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	if slot.Generation != 1 || !SameVersion(slot.Current, v1) || slot.LatestReceiptID != first.ID {
		t.Fatalf("bootstrap slot/receipt mismatch: slot=%+v receipt=%+v", slot, first)
	}
	if !ReceiptMatchesCommand(first, bootstrap) {
		t.Fatal("bootstrap receipt did not match exact command")
	}
	forgedCommand := bootstrap
	forgedCommand.PromotionCertificateRef = "certificate:forged"
	if ReceiptMatchesCommand(first, forgedCommand) {
		t.Fatal("receipt matched a different promotion certificate")
	}

	replayedSlot, replayed, err := ledger.Transition(context.Background(), bootstrap)
	if err != nil {
		t.Fatalf("idempotent replay: %v", err)
	}
	if replayed.ID != first.ID || replayedSlot.Generation != 1 {
		t.Fatalf("idempotent replay changed result: slot=%+v receipt=%+v", replayedSlot, replayed)
	}

	promote := TransitionCommand{RouteSlotID: slotID,
		Kind:               TransitionPromote,
		Old:                v1,
		New:                v2,
		ExpectedGeneration: 1, ApprovalRef: testApprovalRef, PromotionCertificateRef: testCertificateRef, IdempotencyKey: "idempotency:promote-b"}
	slot, promoted, err := ledger.Transition(context.Background(), promote)
	if err != nil {
		t.Fatalf("promote: %v", err)
	}
	if slot.Generation != 2 || promoted.CommittedGeneration != 2 || !SameVersion(slot.Current, v2) {
		t.Fatalf("promotion mismatch: slot=%+v receipt=%+v", slot, promoted)
	}

	resolved, latest, err := ledger.Resolve(context.Background(), slotID)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if resolved.Generation != 2 || latest.ID != promoted.ID || !SameVersion(latest.New, resolved.Current) {
		t.Fatalf("resolve did not join latest receipt: slot=%+v receipt=%+v", resolved, latest)
	}

	currentAfterReplay, originalReceipt, err := ledger.Transition(context.Background(), bootstrap)
	if err != nil {
		t.Fatalf("replay after later transition: %v", err)
	}
	if currentAfterReplay.Generation != 2 || originalReceipt.ID != first.ID {
		t.Fatalf("late replay did not preserve current route and original receipt: slot=%+v receipt=%+v", currentAfterReplay, originalReceipt)
	}

	rolledBack, rollbackReceipt, err := ledger.Transition(context.Background(), TransitionCommand{RouteSlotID: slotID, Kind: TransitionRollback, Old: v2, New: v1, ExpectedGeneration: 2, ApprovalRef: testApprovalRef, PromotionCertificateRef: testCertificateRef, RollbackTargetReceiptID: first.ID, IdempotencyKey: "idempotency:rollback-a"})
	if err != nil {
		t.Fatalf("rollback: %v", err)
	}
	if rolledBack.Generation != 3 || rollbackReceipt.CommittedGeneration != 3 || !SameVersion(rolledBack.Current, v1) {
		t.Fatalf("rollback mismatch: slot=%+v receipt=%+v", rolledBack, rollbackReceipt)
	}
}

func TestMemoryLedgerConcurrentCASHasOneWinner(t *testing.T) {
	ledger := NewMemoryLedger()
	slotID := mustSlotID(t, "owner", "primary")
	base := version("code:base", "program:base")
	_, _, err := ledger.Transition(context.Background(), TransitionCommand{RouteSlotID: slotID, Kind: TransitionBootstrap, New: base, ApprovalRef: testApprovalRef, PromotionCertificateRef: testCertificateRef, IdempotencyKey: "idempotency:bootstrap"})
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}

	start := make(chan struct{})
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for i, candidate := range []computerversion.ComputerVersion{version("code:a", "program:a"), version("code:b", "program:b")} {
		wg.Add(1)
		go func(i int, candidate computerversion.ComputerVersion) {
			defer wg.Done()
			<-start
			_, _, err := ledger.Transition(context.Background(), TransitionCommand{RouteSlotID: slotID, Kind: TransitionPromote, Old: base, New: candidate, ExpectedGeneration: 1, ApprovalRef: testApprovalRef, PromotionCertificateRef: testCertificateRef, IdempotencyKey: IdempotencyKey(fmt.Sprintf("idempotency:memory-cas-%d", i))})
			results <- err
		}(i, candidate)
	}
	close(start)
	wg.Wait()
	close(results)
	wins, stale := 0, 0
	for err := range results {
		switch {
		case err == nil:
			wins++
		case errors.Is(err, ErrStaleTransition):
			stale++
		default:
			t.Fatalf("unexpected CAS result: %v", err)
		}
	}
	if wins != 1 || stale != 1 {
		t.Fatalf("CAS results: wins=%d stale=%d", wins, stale)
	}
}

func TestMemoryLedgerRefusesMutationOnInvalidTransitions(t *testing.T) {
	ledger := NewMemoryLedger()
	slotID := mustSlotID(t, "owner", "primary")
	base := version("code:base", "program:base")
	_, _, err := ledger.Transition(context.Background(), TransitionCommand{RouteSlotID: slotID, Kind: TransitionBootstrap, New: base, ApprovalRef: testApprovalRef, PromotionCertificateRef: testCertificateRef, IdempotencyKey: "idempotency:bootstrap"})
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}

	bad := TransitionCommand{RouteSlotID: slotID, Kind: TransitionPromote, Old: version("wrong", "wrong"), New: version("next", "next"), ExpectedGeneration: 1, ApprovalRef: testApprovalRef, PromotionCertificateRef: testCertificateRef, IdempotencyKey: "idempotency:bad"}
	if _, _, err := ledger.Transition(context.Background(), bad); !errors.Is(err, ErrStaleTransition) {
		t.Fatalf("stale transition error = %v", err)
	}
	resolved, _, err := ledger.Resolve(context.Background(), slotID)
	if err != nil {
		t.Fatalf("resolve after refusal: %v", err)
	}
	if resolved.Generation != 1 || !SameVersion(resolved.Current, base) {
		t.Fatalf("refused transition mutated slot: %+v", resolved)
	}

	reused := bad
	reused.Old = base
	reused.IdempotencyKey = "idempotency:bootstrap"
	if _, _, err := ledger.Transition(context.Background(), reused); !errors.Is(err, ErrIdempotencyReuse) {
		t.Fatalf("idempotency reuse error = %v", err)
	}
}

func transitionSQLForTest(ledger *SQLLedger, ctx context.Context, command TransitionCommand) (Slot, TransitionReceipt, error) {
	return ledger.transitionValidated(ctx, command, nil, false)
}

func transitionSQLWithEvidenceForTest(ledger *SQLLedger, ctx context.Context, command TransitionCommand, evidence []AuthorizationEvidence) (Slot, TransitionReceipt, error) {
	return ledger.transitionValidated(ctx, command, evidence, false)
}

func TestSQLLedgerConcurrentCASHasOneWinner(t *testing.T) {
	productStore, err := store.Open(filepath.Join(t.TempDir(), "runtime.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = productStore.Close() }()
	ledger, catalog := newSQLRouteLedger(t, productStore.DB())
	slotID := mustSlotID(t, "owner-sql-cas", "primary")
	base := pinSQLVersion(t, catalog, "base")
	baseApproval, baseCertificate := pinSQLTransitionEvidence(t, ledger, slotID, base, "cas-base")
	if _, _, err := transitionSQLForTest(ledger, context.Background(), TransitionCommand{RouteSlotID: slotID, Kind: TransitionBootstrap, New: base, ApprovalRef: baseApproval, PromotionCertificateRef: baseCertificate, IdempotencyKey: "idempotency:sql-cas-bootstrap"}); err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	results := make(chan error, 2)
	var wg sync.WaitGroup
	type candidateTransition struct {
		version     computerversion.ComputerVersion
		approval    ApprovalRef
		certificate PromotionCertificateRef
	}
	candidates := make([]candidateTransition, 0, 2)
	for _, tag := range []string{"candidate-a", "candidate-b"} {
		candidate := pinSQLVersion(t, catalog, tag)
		approval, certificate := pinSQLTransitionEvidence(t, ledger, slotID, candidate, tag)
		candidates = append(candidates, candidateTransition{candidate, approval, certificate})
	}
	for i, candidate := range candidates {
		wg.Add(1)
		go func(i int, candidate candidateTransition) {
			defer wg.Done()
			<-start
			_, _, err := transitionSQLForTest(ledger, context.Background(), TransitionCommand{RouteSlotID: slotID, Kind: TransitionPromote, Old: base, New: candidate.version, ExpectedGeneration: 1, ApprovalRef: candidate.approval, PromotionCertificateRef: candidate.certificate, IdempotencyKey: IdempotencyKey(fmt.Sprintf("idempotency:sql-cas-%d", i))})
			results <- err
		}(i, candidate)
	}
	close(start)
	wg.Wait()
	close(results)
	wins, stale := 0, 0
	for err := range results {
		if err == nil {
			wins++
		} else if errors.Is(err, ErrStaleTransition) {
			stale++
		} else {
			t.Fatalf("unexpected SQL CAS result: %v", err)
		}
	}
	if wins != 1 || stale != 1 {
		t.Fatalf("SQL CAS results wins=%d stale=%d", wins, stale)
	}
}

func TestSQLLedgerPersistsSlotAndReceiptAcrossRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "runtime.db")
	productStore, err := store.Open(path)
	if err != nil {
		t.Fatalf("open embedded Dolt: %v", err)
	}
	ledger, catalog := newSQLRouteLedger(t, productStore.DB())
	slotID := mustSlotID(t, "owner-sql", "primary")
	want := pinSQLVersion(t, catalog, "immutable-v1")
	bootstrapApproval, bootstrapCertificate := pinSQLTransitionEvidence(t, ledger, slotID, want, "restart-bootstrap")
	slot, receipt, err := transitionSQLForTest(ledger, context.Background(), TransitionCommand{RouteSlotID: slotID, Kind: TransitionBootstrap, New: want, ApprovalRef: bootstrapApproval, PromotionCertificateRef: bootstrapCertificate, IdempotencyKey: "idempotency:sql-bootstrap"})
	if err != nil {
		t.Fatalf("bootstrap SQL route: %v", err)
	}
	v2 := pinSQLVersion(t, catalog, "immutable-v2")
	promoteApproval, promoteCertificate := pinSQLTransitionEvidence(t, ledger, slotID, v2, "restart-promote")
	promotedSlot, _, err := transitionSQLForTest(ledger, context.Background(), TransitionCommand{RouteSlotID: slotID, Kind: TransitionPromote, Old: want, New: v2, ExpectedGeneration: 1, ApprovalRef: promoteApproval, PromotionCertificateRef: promoteCertificate, IdempotencyKey: "idempotency:sql-promote"})
	if err != nil {
		t.Fatalf("promote SQL route: %v", err)
	}
	replayedSlot, replayedReceipt, err := transitionSQLForTest(ledger, context.Background(), TransitionCommand{RouteSlotID: slotID, Kind: TransitionBootstrap, New: want, ApprovalRef: bootstrapApproval, PromotionCertificateRef: bootstrapCertificate, IdempotencyKey: "idempotency:sql-bootstrap"})
	if err != nil {
		t.Fatalf("late SQL replay: %v", err)
	}
	if replayedSlot != promotedSlot || replayedReceipt.ID != receipt.ID {
		t.Fatalf("late SQL replay slot=%+v receipt=%+v, want current=%+v original=%q", replayedSlot, replayedReceipt, promotedSlot, receipt.ID)
	}
	rollbackApproval, rollbackCertificate := pinSQLTransitionEvidence(t, ledger, slotID, want, "restart-rollback")
	finalSlot, rollbackReceipt, err := transitionSQLForTest(ledger, context.Background(), TransitionCommand{RouteSlotID: slotID, Kind: TransitionRollback, Old: v2, New: want, ExpectedGeneration: 2, ApprovalRef: rollbackApproval, PromotionCertificateRef: rollbackCertificate, RollbackTargetReceiptID: receipt.ID, IdempotencyKey: "idempotency:sql-rollback"})
	if err != nil {
		t.Fatalf("rollback SQL route: %v", err)
	}
	if err := productStore.Close(); err != nil {
		t.Fatalf("close first store: %v", err)
	}

	restartedStore, err := store.Open(path)
	if err != nil {
		t.Fatalf("reopen embedded Dolt: %v", err)
	}
	defer func() { _ = restartedStore.Close() }()
	restarted := NewSQLLedger(restartedStore.DB())
	resolved, latest, err := restarted.Resolve(context.Background(), slotID)
	if err != nil {
		t.Fatalf("resolve SQL route after restart: %v", err)
	}
	if resolved != finalSlot || latest.ID != rollbackReceipt.ID || !SameVersion(latest.New, want) {
		t.Fatalf("persisted join mismatch: initial=%+v got slot=%+v receipt=%+v", slot, resolved, latest)
	}
}

func mustSlotID(t *testing.T, ownerID, computerID string) string {
	t.Helper()
	id, err := RouteSlotID(ownerID, computerID)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func version(code, program string) computerversion.ComputerVersion {
	return computerversion.ComputerVersion{CodeRef: computerversion.CodeRef(code), ArtifactProgramRef: computerversion.ArtifactProgramRef(program)}
}

func TestSQLLedgerRefusesUnresolvableInputsAndProtectsRoutedCatalogRows(t *testing.T) {
	productStore, err := store.Open(filepath.Join(t.TempDir(), "runtime.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = productStore.Close() }()
	ledger, catalog := newSQLRouteLedger(t, productStore.DB())
	base := pinSQLVersion(t, catalog, "integrity-base")
	candidate := pinSQLVersion(t, catalog, "integrity-candidate")
	slotID := mustSlotID(t, "owner-integrity", "primary")
	if _, _, err := transitionSQLForTest(ledger, context.Background(), TransitionCommand{
		RouteSlotID: slotID, Kind: TransitionBootstrap, New: base,
		ApprovalRef: testApprovalRef, PromotionCertificateRef: testCertificateRef, IdempotencyKey: "idempotency:invented-evidence",
	}); err == nil {
		t.Fatal("invented authorization evidence advanced route")
	}
	if _, _, err := ledger.Resolve(context.Background(), slotID); !errors.Is(err, ErrSlotNotFound) {
		t.Fatalf("invented evidence mutated route: %v", err)
	}
	baseApproval, baseCertificate := pinSQLTransitionEvidence(t, ledger, slotID, base, "integrity-base")
	if _, _, err := transitionSQLForTest(ledger, context.Background(), TransitionCommand{
		RouteSlotID: slotID, Kind: TransitionBootstrap, New: base,
		ApprovalRef: baseApproval, PromotionCertificateRef: baseCertificate, IdempotencyKey: "idempotency:integrity-bootstrap",
	}); err != nil {
		t.Fatalf("bootstrap joined route: %v", err)
	}
	if _, err := productStore.DB().ExecContext(context.Background(), `DELETE FROM computer_version_code_closures WHERE code_ref = ?`, base.CodeRef); err == nil {
		t.Fatal("deleted a CodeRef referenced by a committed route")
	}
	if _, err := productStore.DB().ExecContext(context.Background(), `UPDATE computer_version_code_closures SET closure_json = '{}' WHERE code_ref = ?`, candidate.CodeRef); err != nil {
		t.Fatalf("tamper candidate declaration: %v", err)
	}
	candidateApproval, candidateCertificate := pinSQLTransitionEvidence(t, ledger, slotID, candidate, "integrity-candidate")
	if _, _, err := transitionSQLForTest(ledger, context.Background(), TransitionCommand{
		RouteSlotID: slotID, Kind: TransitionPromote, Old: base, New: candidate, ExpectedGeneration: 1,
		ApprovalRef: candidateApproval, PromotionCertificateRef: candidateCertificate, IdempotencyKey: "idempotency:integrity-promote",
	}); err == nil {
		t.Fatal("route advanced to a tampered input declaration")
	}
	resolved, _, err := ledger.Resolve(context.Background(), slotID)
	if err != nil {
		t.Fatalf("resolve route after refused transition: %v", err)
	}
	if resolved.Generation != 1 || !SameVersion(resolved.Current, base) {
		t.Fatalf("refused input transition mutated route: %+v", resolved)
	}
}

func TestReceiptMatchesCommandIncludesIdempotencyEvidence(t *testing.T) {
	ledger := NewMemoryLedger()
	command := TransitionCommand{
		RouteSlotID: mustSlotID(t, "owner-receipt", "primary"), Kind: TransitionBootstrap,
		New: version("code:receipt", "program:receipt"), ApprovalRef: testApprovalRef,
		PromotionCertificateRef: testCertificateRef, IdempotencyKey: "idempotency:receipt-command-a",
	}
	_, receipt, err := ledger.Transition(context.Background(), command)
	if err != nil {
		t.Fatal(err)
	}
	command.IdempotencyKey = "idempotency:receipt-command-b"
	if ReceiptMatchesCommand(receipt, command) {
		t.Fatal("receipt matched a command with different idempotency evidence")
	}
}

func TestTransitionEvidenceTypesRejectMalformedRefs(t *testing.T) {
	base := TransitionCommand{
		RouteSlotID: mustSlotID(t, "owner-evidence", "primary"), Kind: TransitionBootstrap,
		New: version("code:evidence", "program:evidence"), ApprovalRef: testApprovalRef,
		PromotionCertificateRef: testCertificateRef, IdempotencyKey: "idempotency:evidence-key",
	}
	for name, mutate := range map[string]func(*TransitionCommand){
		"untyped approval":       func(command *TransitionCommand) { command.ApprovalRef = "ok" },
		"untyped certificate":    func(command *TransitionCommand) { command.PromotionCertificateRef = "ok" },
		"whitespace idempotency": func(command *TransitionCommand) { command.IdempotencyKey = "not valid" },
		"approval as idempotency": func(command *TransitionCommand) {
			command.IdempotencyKey = IdempotencyKey(command.ApprovalRef)
		},
	} {
		t.Run(name, func(t *testing.T) {
			command := base
			mutate(&command)
			if err := command.Validate(); err == nil {
				t.Fatal("malformed typed transition evidence validated")
			}
		})
	}
}

type acceptingContentVerifier struct{}

func (acceptingContentVerifier) VerifyArtifact(context.Context, string, string) error { return nil }

func TestSQLLedgerAtomicEvidenceRollsBackOnStaleTransition(t *testing.T) {
	productStore, err := store.Open(filepath.Join(t.TempDir(), "runtime.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = productStore.Close() }()
	ledger, catalog := newSQLRouteLedger(t, productStore.DB())
	slotID := mustSlotID(t, "owner-atomic", "primary")
	base := pinSQLVersion(t, catalog, "atomic-base")
	candidate := pinSQLVersion(t, catalog, "atomic-candidate")
	baseApproval, baseCertificate := pinSQLTransitionEvidence(t, ledger, slotID, base, "atomic-base")
	if _, _, err := transitionSQLForTest(ledger, t.Context(), TransitionCommand{RouteSlotID: slotID, Kind: TransitionBootstrap, New: base, ApprovalRef: baseApproval, PromotionCertificateRef: baseCertificate, IdempotencyKey: "idempotency:atomic-bootstrap"}); err != nil {
		t.Fatal(err)
	}
	createdAt := time.Date(2026, 7, 16, 3, 0, 0, 0, time.UTC)
	approval, err := NewAuthorizationEvidence(AuthorizationEvidenceApproval, slotID, candidate, json.RawMessage(`{"gate":"stale"}`), createdAt)
	if err != nil {
		t.Fatal(err)
	}
	certificate, err := NewAuthorizationEvidence(AuthorizationEvidencePromotionCertificate, slotID, candidate, json.RawMessage(`{"certificate":"stale"}`), createdAt)
	if err != nil {
		t.Fatal(err)
	}
	command := TransitionCommand{RouteSlotID: slotID, Kind: TransitionPromote, Old: base, New: candidate, ExpectedGeneration: 99, ApprovalRef: ApprovalRef(approval.Ref), PromotionCertificateRef: PromotionCertificateRef(certificate.Ref), IdempotencyKey: "idempotency:atomic-stale"}
	if _, _, err := transitionSQLWithEvidenceForTest(ledger, t.Context(), command, []AuthorizationEvidence{approval, certificate}); !errors.Is(err, ErrStaleTransition) {
		t.Fatalf("atomic stale transition error = %v", err)
	}
	if err := ledger.VerifyTransitionEvidence(t.Context(), command); err == nil {
		t.Fatal("stale atomic transition left authorization evidence committed")
	}
}

func TestSQLLedgerSelfDevelopmentProtectionCommitsAtomically(t *testing.T) {
	productStore, err := store.Open(filepath.Join(t.TempDir(), "runtime.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = productStore.Close() }()
	ledger, catalog := newSQLRouteLedger(t, productStore.DB())
	slotID := mustSlotID(t, "owner-selfdev", "primary")
	base := pinSQLVersion(t, catalog, "selfdev-base")
	candidate := pinSQLVersion(t, catalog, "selfdev-candidate")
	baseApproval, baseCertificate := pinSQLTransitionEvidence(t, ledger, slotID, base, "selfdev-base")
	slot, _, err := transitionSQLForTest(ledger, t.Context(), TransitionCommand{RouteSlotID: slotID, Kind: TransitionBootstrap, New: base, ApprovalRef: baseApproval, PromotionCertificateRef: baseCertificate, IdempotencyKey: "idempotency:selfdev-bootstrap"})
	if err != nil {
		t.Fatal(err)
	}
	approval, certificate := pinSQLTransitionEvidence(t, ledger, slotID, candidate, "selfdev-candidate")
	command := TransitionCommand{RouteSlotID: slotID, Kind: TransitionPromote, Old: base, New: candidate, ExpectedGeneration: slot.Generation, ApprovalRef: approval, PromotionCertificateRef: certificate, IdempotencyKey: "idempotency:selfdev-promote"}
	if _, _, err := ledger.ApplySelfDevelopmentTransition(t.Context(), command, nil); err == nil {
		t.Fatal("self-development transition accepted missing evidence")
	}
	approvalEvidence, err := ledger.ResolveAuthorizationEvidence(t.Context(), string(approval))
	if err != nil {
		t.Fatal(err)
	}
	certificateEvidence, err := ledger.ResolveAuthorizationEvidence(t.Context(), string(certificate))
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := ledger.ApplySelfDevelopmentTransition(t.Context(), command, []AuthorizationEvidence{approvalEvidence, certificateEvidence}); err != nil {
		t.Fatal(err)
	}
	protected, err := ledger.SelfDevelopmentRouteProtected(t.Context(), slotID)
	if err != nil || !protected {
		t.Fatalf("protected=%v err=%v", protected, err)
	}
}

func newSQLRouteLedger(t *testing.T, db *sql.DB) (*SQLLedger, *computerversion.SQLInputCatalog) {
	t.Helper()
	catalog := computerversion.NewSQLInputCatalog(db, acceptingContentVerifier{})
	if err := catalog.EnsureSchema(context.Background()); err != nil {
		t.Fatalf("ensure input catalog schema: %v", err)
	}
	ledger := NewSQLLedger(db)
	if err := ledger.EnsureSchema(context.Background()); err != nil {
		t.Fatalf("ensure route ledger schema: %v", err)
	}
	return ledger, catalog
}

func pinSQLVersion(t *testing.T, catalog *computerversion.SQLInputCatalog, tag string) computerversion.ComputerVersion {
	t.Helper()
	codeDigest := digestString("code-content:" + tag)
	closure, err := computerversion.NewCodeClosure(digestString("source-commit:"+tag), []computerversion.CodeArtifact{{
		Name: "sandbox", SHA256: codeDigest, URI: "artifact+sha256://" + codeDigest + "/tests/" + tag + "/sandbox",
	}}, time.Date(2026, 7, 16, 1, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("new code closure %s: %v", tag, err)
	}
	programDigest := digestString("program-content:" + tag)
	program, err := computerversion.NewArtifactProgram([]computerversion.ArtifactProgramEntry{{
		Kind: "test", ContentSHA256: programDigest, ArtifactURI: "artifact+sha256://" + programDigest + "/tests/" + tag + "/program",
	}}, time.Date(2026, 7, 16, 1, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("new artifact program %s: %v", tag, err)
	}
	if _, err := catalog.PinCode(context.Background(), closure); err != nil {
		t.Fatalf("pin code closure %s: %v", tag, err)
	}
	if _, err := catalog.PinArtifactProgram(context.Background(), program); err != nil {
		t.Fatalf("pin artifact program %s: %v", tag, err)
	}
	return computerversion.ComputerVersion{CodeRef: closure.Ref, ArtifactProgramRef: program.Ref}
}

func pinSQLTransitionEvidence(t *testing.T, ledger *SQLLedger, slotID string, version computerversion.ComputerVersion, tag string) (ApprovalRef, PromotionCertificateRef) {
	t.Helper()
	createdAt := time.Date(2026, 7, 16, 2, 0, 0, 0, time.UTC)
	approval, err := NewAuthorizationEvidence(AuthorizationEvidenceApproval, slotID, version, json.RawMessage(fmt.Sprintf(`{"approval_id":%q}`, tag)), createdAt)
	if err != nil {
		t.Fatalf("new approval evidence %s: %v", tag, err)
	}
	certificate, err := NewAuthorizationEvidence(AuthorizationEvidencePromotionCertificate, slotID, version, json.RawMessage(fmt.Sprintf(`{"certificate_id":%q}`, tag)), createdAt)
	if err != nil {
		t.Fatalf("new certificate evidence %s: %v", tag, err)
	}
	if _, err := ledger.PinAuthorizationEvidence(context.Background(), approval); err != nil {
		t.Fatalf("pin approval evidence %s: %v", tag, err)
	}
	if _, err := ledger.PinAuthorizationEvidence(context.Background(), certificate); err != nil {
		t.Fatalf("pin certificate evidence %s: %v", tag, err)
	}
	return ApprovalRef(approval.Ref), PromotionCertificateRef(certificate.Ref)
}

func digestString(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])
}

func TestMemoryLedgerBootstrapRollbackRemovesOnlyGenerationOneRoute(t *testing.T) {
	ledger := NewMemoryLedger()
	slotID := mustSlotID(t, "owner-bootstrap-rollback", "primary")
	version := version("code:bootstrap-rollback", "program:bootstrap-rollback")
	bootstrap := TransitionCommand{RouteSlotID: slotID, Kind: TransitionBootstrap, New: version, ApprovalRef: testApprovalRef, PromotionCertificateRef: testCertificateRef, IdempotencyKey: "idempotency:bootstrap-rollback-base"}
	_, bootstrapReceipt, err := ledger.Transition(t.Context(), bootstrap)
	if err != nil {
		t.Fatal(err)
	}
	if bootstrapReceipt.ID != BootstrapReceiptID(slotID, bootstrap.IdempotencyKey) {
		t.Fatalf("bootstrap receipt ID = %q, want frozen %q", bootstrapReceipt.ID, BootstrapReceiptID(slotID, bootstrap.IdempotencyKey))
	}
	rollback := TransitionCommand{RouteSlotID: slotID, Kind: TransitionBootstrapRollback, Old: version, New: version, ExpectedGeneration: 1, ApprovalRef: testApprovalRef, PromotionCertificateRef: testCertificateRef, RollbackTargetReceiptID: bootstrapReceipt.ID, IdempotencyKey: "idempotency:bootstrap-rollback-absence"}
	otherSlotID := mustSlotID(t, "owner-bootstrap-rollback-other", "primary")
	_, otherReceipt, err := ledger.Transition(t.Context(), TransitionCommand{RouteSlotID: otherSlotID, Kind: TransitionBootstrap, New: version, ApprovalRef: testApprovalRef, PromotionCertificateRef: testCertificateRef, IdempotencyKey: "idempotency:bootstrap-rollback-other"})
	if err != nil {
		t.Fatal(err)
	}
	crossSlot := rollback
	crossSlot.RollbackTargetReceiptID = otherReceipt.ID
	crossSlot.IdempotencyKey = "idempotency:bootstrap-rollback-cross-slot"
	if _, _, err := ledger.Transition(t.Context(), crossSlot); err == nil {
		t.Fatal("cross-slot bootstrap receipt authorized rollback")
	}
	if current, _, err := ledger.Resolve(t.Context(), slotID); err != nil || current.Generation != 1 {
		t.Fatalf("cross-slot refusal mutated route: slot %+v err %v", current, err)
	}
	slot, receipt, err := ledger.Transition(t.Context(), rollback)
	if err != nil {
		t.Fatal(err)
	}
	if slot != (Slot{}) || receipt.Validate() != nil || receipt.Kind != TransitionBootstrapRollback || receipt.CommittedGeneration != 2 {
		t.Fatalf("bootstrap rollback result = slot %+v receipt %+v", slot, receipt)
	}
	if _, _, err := ledger.Resolve(t.Context(), slotID); !errors.Is(err, ErrSlotNotFound) {
		t.Fatalf("withdrawn route resolve error = %v", err)
	}
	replayedSlot, replayed, err := ledger.Transition(t.Context(), rollback)
	if err != nil || replayedSlot != (Slot{}) || replayed.ID != receipt.ID {
		t.Fatalf("bootstrap rollback replay = slot %+v receipt %+v err %v", replayedSlot, replayed, err)
	}
	stale := rollback
	stale.IdempotencyKey = "idempotency:bootstrap-rollback-stale"
	if _, _, err := ledger.Transition(t.Context(), stale); !errors.Is(err, ErrSlotNotFound) {
		t.Fatalf("non-replay against absent route error = %v", err)
	}
}

func TestSQLLedgerBootstrapRollbackPersistsReceiptAndRouteAbsence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "runtime.db")
	productStore, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	ledger, catalog := newSQLRouteLedger(t, productStore.DB())
	slotID := mustSlotID(t, "owner-sql-bootstrap-rollback", "primary")
	version := pinSQLVersion(t, catalog, "bootstrap-rollback")
	bootstrapApproval, bootstrapCertificate := pinSQLTransitionEvidence(t, ledger, slotID, version, "bootstrap-rollback-base")
	bootstrap := TransitionCommand{RouteSlotID: slotID, Kind: TransitionBootstrap, New: version, ApprovalRef: bootstrapApproval, PromotionCertificateRef: bootstrapCertificate, IdempotencyKey: "idempotency:sql-bootstrap-rollback-base"}
	_, bootstrapReceipt, err := transitionSQLForTest(ledger, t.Context(), bootstrap)
	if err != nil {
		t.Fatal(err)
	}
	rollbackApproval, rollbackCertificate := pinSQLTransitionEvidence(t, ledger, slotID, version, "bootstrap-rollback-absence")
	rollback := TransitionCommand{RouteSlotID: slotID, Kind: TransitionBootstrapRollback, Old: version, New: version, ExpectedGeneration: 1, ApprovalRef: rollbackApproval, PromotionCertificateRef: rollbackCertificate, RollbackTargetReceiptID: bootstrapReceipt.ID, IdempotencyKey: "idempotency:sql-bootstrap-rollback-absence"}
	otherSlotID := mustSlotID(t, "owner-sql-bootstrap-rollback-other", "primary")
	otherApproval, otherCertificate := pinSQLTransitionEvidence(t, ledger, otherSlotID, version, "bootstrap-rollback-other")
	_, otherReceipt, err := transitionSQLForTest(ledger, t.Context(), TransitionCommand{RouteSlotID: otherSlotID, Kind: TransitionBootstrap, New: version, ApprovalRef: otherApproval, PromotionCertificateRef: otherCertificate, IdempotencyKey: "idempotency:sql-bootstrap-rollback-other"})
	if err != nil {
		t.Fatal(err)
	}
	crossSlot := rollback
	crossSlot.RollbackTargetReceiptID = otherReceipt.ID
	crossSlot.IdempotencyKey = "idempotency:sql-bootstrap-rollback-cross-slot"
	if _, _, err := transitionSQLForTest(ledger, t.Context(), crossSlot); err == nil {
		t.Fatal("SQL cross-slot bootstrap receipt authorized rollback")
	}
	if current, _, err := ledger.Resolve(t.Context(), slotID); err != nil || current.Generation != 1 {
		t.Fatalf("SQL cross-slot refusal mutated route: slot %+v err %v", current, err)
	}
	slot, receipt, err := transitionSQLForTest(ledger, t.Context(), rollback)
	if err != nil {
		t.Fatal(err)
	}
	if slot != (Slot{}) || receipt.Validate() != nil || receipt.CommittedGeneration != 2 {
		t.Fatalf("SQL bootstrap rollback = slot %+v receipt %+v", slot, receipt)
	}
	if _, _, err := ledger.Resolve(t.Context(), slotID); !errors.Is(err, ErrSlotNotFound) {
		t.Fatalf("SQL withdrawn route resolve error = %v", err)
	}
	replayedSlot, replayed, err := transitionSQLForTest(ledger, t.Context(), rollback)
	if err != nil || replayedSlot != (Slot{}) || replayed.ID != receipt.ID {
		t.Fatalf("SQL bootstrap rollback replay = slot %+v receipt %+v err %v", replayedSlot, replayed, err)
	}
	if err := productStore.Close(); err != nil {
		t.Fatal(err)
	}
	restartedStore, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = restartedStore.Close() }()
	restarted := NewSQLLedger(restartedStore.DB())
	if _, _, err := restarted.Resolve(t.Context(), slotID); !errors.Is(err, ErrSlotNotFound) {
		t.Fatalf("restarted withdrawn route resolve error = %v", err)
	}
	replayedSlot, replayed, err = transitionSQLForTest(restarted, t.Context(), rollback)
	if err != nil || replayedSlot != (Slot{}) || replayed.ID != receipt.ID {
		t.Fatalf("restarted SQL rollback replay = slot %+v receipt %+v err %v", replayedSlot, replayed, err)
	}
}
