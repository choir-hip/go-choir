package agentcore

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/lease"
	"github.com/yusefmosiah/go-choir/internal/maildesk"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/trust"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// Leases (MA3.4). The lease service lives in internal/lease; this file is how
// the runtime uses it: the run's context carries the authority and the run's
// id (so the reader gate and the capture frame handler can ask), a finished
// run ends its leases, the boot pass ends every lease the last process left
// open, and a desk's lease request is made from the run record.

// Leases is the lease service, or nil on a runtime without a store. The
// autoputer's owner routes list and revoke through it.
func (rt *Runtime) Leases() *lease.Service {
	if rt == nil {
		return nil
	}
	return rt.leases
}

// leaseAuthority is the authority a run's context carries; a nil service
// attaches nothing.
func (rt *Runtime) leaseAuthority() objectgraph.LeaseAuthority {
	if rt == nil || rt.leases == nil {
		return nil
	}
	return rt.leases
}

// endRunLeases ends the leases of a run that finished or was closed.
func (rt *Runtime) endRunLeases(ctx context.Context, runID, reason string) {
	if rt == nil || rt.leases == nil || strings.TrimSpace(runID) == "" {
		return
	}
	if n := rt.leases.EndRun(ctx, runID, reason); n > 0 {
		log.Printf("runtime: ended %d lease(s) of run %s (%s)", n, runID, reason)
	}
}

// endLeasesAtBoot reads the leases the tape holds as active and ends all of
// them with the fate "interrupted by a restart". A lease is per-task and a
// run is never resumed with its lease, whether the restart crashed (the runs
// are closed) or was planned (a resumed run asks again).
func (rt *Runtime) endLeasesAtBoot(ctx context.Context) {
	if rt.leases == nil {
		return
	}
	ownerID := strings.TrimSpace(rt.selfdevRouteOwnerID)
	if ownerID == "" {
		ownerID = strings.TrimSpace(os.Getenv("CHOIR_OWNER_ID"))
	}
	computerID := strings.TrimSpace(rt.TextureComputerID())
	if ownerID == "" {
		return
	}
	if err := rt.leases.Load(ctx, ownerID, computerID); err != nil {
		log.Printf("runtime: leases at boot: load: %v", err)
		return
	}
	if n := rt.leases.EndAll(ctx, lease.ReasonRestart); n > 0 {
		log.Printf("runtime: ended %d lease(s) open before this boot as interrupted by a restart", n)
	}
}

// RequestLease is the host side of a desk's lease request. The desk, run,
// agent and model come from the run record, never from the cell; the cell
// supplies the scope, the purpose and the request fields (p_success, weight,
// settlement).
func (rt *Runtime) RequestLease(ctx context.Context, rec *types.RunRecord, scope lease.Scope, purpose string, duration time.Duration, fields *types.RequestFields) (lease.Record, error) {
	if rt.leases == nil {
		return lease.Record{}, lease.ErrRefused
	}
	rt.feedLeaseTrust(rec.OwnerID, rec.ComputerID)
	return rt.leases.Request(ctx, lease.Request{
		OwnerID: rec.OwnerID, ComputerID: rec.ComputerID, Desk: rec.AgentProfile, RunID: rec.RunID,
		AgentID: rec.AgentID, ModelID: recordModelID(ctx, rec), Scope: scope, Purpose: purpose, Duration: duration, Fields: fields,
	})
}

// leaseTrustEvery is how often a lease request re-derives the trust view the
// lease policy reads. It is well inside lease.TrustMaxAge, so a desk that asks
// often reads a fresh view, and a view older than that bound is no view.
const leaseTrustEvery = 30 * time.Second

// leaseTrustTimeout bounds one derivation; past it the policy keeps its last
// view, or none, and a lease is per-task.
const leaseTrustTimeout = 10 * time.Second

// feedLeaseTrust derives the trust view (as GET /api/trust does) and pushes
// its rows to the lease policy (MA7.4) and the act gate (MA7.8), at most once
// per leaseTrustEvery per owner: the policy reads the rows for a lease's
// duration, a row whose breaker is set revokes the holder's leases in its
// class, and the gate reads them for whether a material act is autonomous.
// It then renews or ends the leases past their expiry. The derivation runs
// here, on the lease request and act paths and outside any store read,
// because the reader gate asks the lease service from inside store reads and
// neither the service nor the gate reads the store. It reads under its own
// context, without the requesting desk's reader.
func (rt *Runtime) feedLeaseTrust(ownerID, computerID string) {
	if rt == nil || (rt.leases == nil && rt.actGate == nil) || rt.store == nil || strings.TrimSpace(ownerID) == "" {
		return
	}
	rt.leaseTrustMu.Lock()
	defer rt.leaseTrustMu.Unlock()
	if fed := rt.leaseTrustFed[ownerID]; !fed.IsZero() && time.Since(fed) < leaseTrustEvery {
		return
	}
	if strings.TrimSpace(computerID) == "" {
		computerID = rt.TextureComputerID()
	}
	ctx, cancel := context.WithTimeout(context.Background(), leaseTrustTimeout)
	defer cancel()
	in, err := rt.trustInputs(ctx, ownerID, computerID)
	if err != nil {
		log.Printf("runtime: lease policy: trust view: %v", err)
		return
	}
	view := trust.Derive(in, trust.DefaultParams())
	if rt.leaseTrustFed == nil {
		rt.leaseTrustFed = map[string]time.Time{}
	}
	rt.leaseTrustFed[ownerID] = time.Now()
	if rt.actGate != nil {
		rt.actGate.Observe(ownerID, view.Rows)
	}
	if rt.leases == nil {
		return
	}
	if n := rt.leases.ObserveTrust(ctx, ownerID, view.Rows); n > 0 {
		log.Printf("runtime: lease policy: the breaker revoked %d lease(s) on a fidelity event", n)
	}
	rt.leases.Sweep(ctx)
}

// requestLeaseEgress serves choir.RequestLease from a desk cell. The holder
// is the session's desk role and the run is the cell's run, both bound by the
// frame layer; the cell supplies only the scope, the purpose and the request
// fields. The result is the lease as JSON; a refusal is an error the cell
// reads.
func (rt *Runtime) requestLeaseEgress(ctx context.Context, deskRole string, exec toolregistry.ExecutionContext, payload json.RawMessage) (json.RawMessage, error) {
	if rt == nil || rt.leases == nil {
		return nil, fmt.Errorf("choir.RequestLease: no lease service on this computer")
	}
	var req yaegikernel.RequestLeasePayload
	dec := json.NewDecoder(bytes.NewReader(payload))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		return nil, fmt.Errorf("choir.RequestLease: decode the call: %w", err)
	}
	rec := exec.RunRecord
	if rec == nil {
		rec = &types.RunRecord{RunID: exec.RunID, OwnerID: exec.OwnerID, ComputerID: exec.ComputerID, AgentID: exec.AgentID}
	}
	held := *rec
	held.AgentProfile = strings.TrimSpace(deskRole)
	if held.RunID == "" {
		held.RunID = exec.RunID
	}
	// A cell that was aborted (a forged frame, the host's kill) asks for
	// nothing: its context is cancelled before the breaker trips, and a grant
	// that raced the abort is revoked at once (MA6.6 review r1, S2).
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("choir.RequestLease: the cell was aborted: %w", err)
	}
	var granted lease.Record
	var err error
	if len(req.Grants) > 0 || len(req.Refs) > 0 {
		granted, err = rt.requestMailLease(ctx, &held, req)
	} else {
		granted, err = rt.RequestLease(ctx, &held,
			lease.Scope{Kinds: req.Kinds, CaptureIDs: req.CaptureIDs, CaptureLabels: req.CaptureLabels},
			req.Purpose, time.Duration(req.DurationSeconds)*time.Second, req.Req)
	}
	if err == nil && ctx.Err() != nil {
		_ = rt.leases.Revoke(context.WithoutCancel(ctx), granted.LeaseID, "the cell that asked was aborted")
		return nil, fmt.Errorf("choir.RequestLease: the cell was aborted: %w", ctx.Err())
	}
	if err != nil {
		if errors.Is(err, lease.ErrRefused) {
			return nil, fmt.Errorf("choir.RequestLease: %w", err)
		}
		return nil, err
	}
	return json.Marshal(granted)
}

// requestMailLease routes a cell's mail.read ask to the Mail custodian service
// (internal/maildesk), the only grant a cell may name. Mail decides by its own
// rule (one email, by ref, in the owner's mailbox, one per task); the request
// is recorded as a scored precommitment addressed to the Mail desk whatever
// the answer. The cell's grant list is checked against mail.read here so a
// desk cannot ask for a grant the host issues itself (texture.search).
func (rt *Runtime) requestMailLease(ctx context.Context, rec *types.RunRecord, req yaegikernel.RequestLeasePayload) (lease.Record, error) {
	if len(req.Grants) != 1 || req.Grants[0] != lease.GrantMailRead {
		return lease.Record{}, fmt.Errorf("%w: a cell may ask for the grant %q only, not %v", lease.ErrRefused, lease.GrantMailRead, req.Grants)
	}
	if len(req.Kinds) > 0 || len(req.CaptureIDs) > 0 || len(req.CaptureLabels) > 0 || req.DurationSeconds != 0 {
		return lease.Record{}, fmt.Errorf("%w: a mail.read request names only its one ref", lease.ErrRefused)
	}
	mail := rt.mailService()
	if mail == nil {
		return lease.Record{}, fmt.Errorf("%w: this computer has no mailbox", lease.ErrRefused)
	}
	rt.feedLeaseTrust(rec.OwnerID, rec.ComputerID)
	return mail.RequestRead(ctx, maildesk.ReadRequest{
		OwnerID: rec.OwnerID, ComputerID: rec.ComputerID, Desk: rec.AgentProfile, RunID: rec.RunID,
		AgentID: rec.AgentID, ModelID: recordModelID(ctx, rec), Refs: req.Refs, Purpose: req.Purpose, Fields: req.Req,
	})
}

// mailService is the Mail custodian over this computer's mailbox, or nil when
// the computer has none (no RUNTIME_MAILD_URL).
func (rt *Runtime) mailService() *maildesk.Service {
	if rt == nil || rt.mailbox == nil || rt.leases == nil {
		return nil
	}
	return maildesk.New(rt.mailbox, rt.leases)
}

// readMailEgress serves choir.ReadMail: the email the session's run holds a
// mail.read lease for. The holder is the session's desk and the run is the
// cell's, both from trusted code; the cell supplies only the ref, and Mail
// serves it only when the lease the run holds names that ref.
func (rt *Runtime) readMailEgress(ctx context.Context, deskRole string, exec toolregistry.ExecutionContext, payload json.RawMessage) (json.RawMessage, error) {
	mail := rt.mailService()
	if mail == nil {
		return nil, fmt.Errorf("choir.ReadMail: this computer has no mailbox")
	}
	var req yaegikernel.ReadMailPayload
	if err := decodeStrict(payload, &req); err != nil {
		return nil, fmt.Errorf("choir.ReadMail: decode the call: %w", err)
	}
	holder, ok := objectgraph.DeskWriter(deskRole)
	if !ok {
		return nil, fmt.Errorf("choir.ReadMail: %q is not a registered desk", deskRole)
	}
	readCtx := objectgraph.WithLeases(objectgraph.WithReader(ctx, holder), rt.leaseAuthority(), exec.RunID)
	msg, err := mail.Read(readCtx, exec.OwnerID, req.Ref)
	if err != nil {
		return nil, fmt.Errorf("choir.ReadMail: %w", err)
	}
	return json.Marshal(msg)
}
