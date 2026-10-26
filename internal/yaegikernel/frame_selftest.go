package yaegikernel

import (
	"context"
	"encoding/json"
	"fmt"
	"sync/atomic"
	"time"
)

// The forged-frame probe of the isolation self-test (POST
// /api/runtime/isolation-selftest, scripts/desk_cell_confinement_probe.py).
//
// It drives the deployed binary's own frame admission (DeskSessionWorker with
// the manifest loaded in this process) from a hand-driven worker end of a real
// socketpair, once per desk role: every host-served action, a frame claiming
// another desk, a wrong, missing and foreign slot token, a stale epoch, an
// unknown protocol version, an emit, and a header with another wire version.
// The handlers behind the frames are counters, not the production tools, so the
// probe has no effect; what it exercises is the verdict.
//
// The receipt carries raw observations. It does not judge them against the
// manifest (that would be the manifest agreeing with itself): the staging
// script holds the expected table itself.

// FrameBoundaryRoles are the desk roles the probe covers.
var FrameBoundaryRoles = []string{"management", "texture", "research", SessionRoleResearchChild, SessionRoleManagementTask}

// FrameBoundaryActions are every action a frame can name.
var FrameBoundaryActions = []BrokerAction{
	ActionExec, ActionReadFile, ActionWriteFile, ActionListDir, ActionMessage, ActionEmit,
	ActionSearchStart, ActionRead, ActionSearchWireCorpus, ActionSaveEvidence, ActionReadEvidence,
	ActionListEvidence, ActionGetRunMemoryEntry, ActionProductAPI, ActionResearch, ActionPollSource,
	ActionReadCapture, ActionStories, ActionSystem1, ActionRelateStories, ActionRelate, ActionSourceAPI,
}

// FrameBoundaryRole is one desk role's observations.
type FrameBoundaryRole struct {
	Role string `json:"role"`
	// Served and Refused partition FrameBoundaryActions by what the daemon did
	// with a correctly signed frame naming each (emit goes on its own stream).
	Served  []string `json:"served"`
	Refused []string `json:"refused"`
	// ClaimedOtherDesk: a frame whose payload and handle claim management,
	// asking for product_api, and what the daemon did (served only for
	// management itself).
	ClaimedOtherDesk string `json:"claimed_other_desk"`
	// The forged cases: what the daemon did, and whether the worker was ended.
	WrongToken       string `json:"wrong_token"`
	WrongTokenKilled bool   `json:"wrong_token_killed"`
	MissingToken     string `json:"missing_token"`
	ForeignToken     string `json:"foreign_token"`
	StaleEpoch       string `json:"stale_epoch"`
	BadProtocol      string `json:"bad_protocol"`
	EmitStream       string `json:"emit_stream"`
	// Handler calls the refused frames reached (must be zero for them).
	RefusedReachedHandler int `json:"refused_reached_handler"`
	// RefusalsReported is how many refusals the daemon's report hook saw.
	RefusalsReported int `json:"refusals_reported"`
	// BadVersionHeader is what reading a header with another wire version did.
	BadVersionHeader string `json:"bad_version_header"`
	Error            string `json:"error,omitempty"`
}

// FrameBoundaryReceipt is the probe's result.
type FrameBoundaryReceipt struct {
	Roles []FrameBoundaryRole `json:"roles"`
}

type frameProbe struct {
	w        *DeskSessionWorker
	wfc      *FramedConn
	token    string
	handler  atomic.Int32
	reported atomic.Int32
	done     chan error
	closeFn  func()
}

func newFrameProbe(role string) (*frameProbe, error) {
	host, worker, err := SocketPair()
	if err != nil {
		return nil, err
	}
	p := &frameProbe{wfc: NewFramedConn(worker), token: "frame-probe-slot-token-0123456789abcdef", done: make(chan error, 1)}
	p.closeFn = func() { host.Close(); worker.Close() }
	p.w = &DeskSessionWorker{
		framed:    NewFramedConn(host),
		slotToken: []byte(p.token),
		cfg: DeskSessionWorkerConfig{
			Session: SessionWorkerConfig{Role: role, Epoch: 1},
			Egress: func(ctx context.Context, a BrokerAction, payload json.RawMessage) (json.RawMessage, error) {
				p.handler.Add(1)
				return json.RawMessage(`{}`), nil
			},
			Emit: func(ctx context.Context, e EmitPayload) (EmitResult, error) {
				p.handler.Add(1)
				return EmitResult{}, nil
			},
			Refused: func(ctx context.Context, r FrameRefusal) { p.reported.Add(1) },
		},
	}
	go func() {
		_, err := p.w.EvalCell(context.Background(), "probe", nil, nil, nil, nil, nil, nil)
		p.done <- err
	}()
	_ = p.wfc.conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	if _, _, err := p.wfc.ReadFrame(); err != nil {
		p.closeFn()
		return nil, fmt.Errorf("read the cell request: %w", err)
	}
	return p, nil
}

func (p *frameProbe) request(action BrokerAction, mutate func(*BrokerRequest)) BrokerRequest {
	req := BrokerRequest{ProtocolVersion: ProtocolVersion, RequestID: "probe-" + string(action), HandleRef: "hnd_probe.sig", Epoch: 1,
		Action: action, Payload: json.RawMessage(`{}`), SlotToken: p.token}
	if mutate != nil {
		mutate(&req)
	}
	return req
}

// send writes a request and reports "served", "refused: <why>", or an
// observation of a lost connection.
func (p *frameProbe) send(stream byte, req BrokerRequest) string {
	raw, _ := json.Marshal(req)
	if err := p.wfc.WriteFrame(stream, raw); err != nil {
		return "write failed: " + err.Error()
	}
	_ = p.wfc.conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	got, payload, err := p.wfc.ReadFrame()
	if err != nil {
		return "connection ended: " + err.Error()
	}
	if got != stream+1 {
		return fmt.Sprintf("unexpected stream %d", got)
	}
	var resp BrokerResponse
	if err := json.Unmarshal(payload, &resp); err != nil {
		return "undecodable answer"
	}
	if resp.Success {
		return "served"
	}
	return "refused: " + resp.Error
}

func served(s string) bool { return s == "served" }

// FrameBoundarySelfTest runs the probe for every desk role.
func FrameBoundarySelfTest() FrameBoundaryReceipt {
	var rec FrameBoundaryReceipt
	for _, role := range FrameBoundaryRoles {
		rec.Roles = append(rec.Roles, probeFrameBoundary(role))
	}
	return rec
}

func probeFrameBoundary(role string) FrameBoundaryRole {
	out := FrameBoundaryRole{Role: role, Served: []string{}, Refused: []string{}}
	// One worker per case that ends it; the action matrix shares one.
	p, err := newFrameProbe(role)
	if err != nil {
		out.Error = err.Error()
		return out
	}
	defer p.closeFn()
	for _, action := range FrameBoundaryActions {
		if action == ActionEmit {
			continue
		}
		before := p.handler.Load()
		verdict := p.send(StreamBrokerEgress, p.request(action, nil))
		if served(verdict) {
			out.Served = append(out.Served, string(action))
		} else {
			out.Refused = append(out.Refused, string(action))
			out.RefusedReachedHandler += int(p.handler.Load() - before)
		}
	}
	// A refusal that still reached a handler would be the worst outcome: count
	// handler calls caused by every frame the daemon refused.
	counted := func(stream byte, req BrokerRequest) string {
		before := p.handler.Load()
		verdict := p.send(stream, req)
		if !served(verdict) {
			out.RefusedReachedHandler += int(p.handler.Load() - before)
		}
		return verdict
	}
	out.ClaimedOtherDesk = counted(StreamBrokerEgress, p.request(ActionProductAPI, func(r *BrokerRequest) {
		r.Payload = json.RawMessage(`{"role":"management","desk":"management","actor_profile":"management","method":"POST","path":"/api/engineering/assignments"}`)
	}))
	out.EmitStream = counted(StreamBrokerEmit, p.request(ActionEmit, func(r *BrokerRequest) {
		r.Payload = json.RawMessage(`{"to_desk":"research","kind":"k","body":"b"}`)
	}))
	out.StaleEpoch = counted(StreamBrokerEgress, p.request(ActionReadCapture, func(r *BrokerRequest) { r.Epoch = 9 }))
	out.BadProtocol = counted(StreamBrokerEgress, p.request(ActionReadCapture, func(r *BrokerRequest) { r.ProtocolVersion = 2 }))
	out.RefusalsReported = int(p.reported.Load())

	// A wrong token ends the worker: its own probe, since it kills the
	// connection. Then a missing token and a token from another worker.
	for _, c := range []struct {
		name  string
		token string
		dst   *string
	}{{"wrong", "not-the-token", &out.WrongToken}, {"missing", "", &out.MissingToken}, {"foreign", "frame-probe-slot-token-ffffffffffffffffffffffff", &out.ForeignToken}} {
		fp, err := newFrameProbe(role)
		if err != nil {
			out.Error = err.Error()
			return out
		}
		*c.dst = fp.send(StreamBrokerEgress, fp.request(ActionReadCapture, func(r *BrokerRequest) { r.SlotToken = c.token }))
		select {
		case err := <-fp.done:
			if c.name == "wrong" {
				out.WrongTokenKilled = err != nil && fp.w.Dead()
			}
		case <-time.After(5 * time.Second):
		}
		fp.closeFn()
	}

	// A header with another wire version ends the connection before its payload
	// is read.
	a, b, err := SocketPair()
	if err != nil {
		out.Error = err.Error()
		return out
	}
	defer a.Close()
	defer b.Close()
	_, _ = a.Write([]byte{0, 0x40, 0, 0, StreamCell, 2, 0, 0})
	_ = b.SetReadDeadline(time.Now().Add(3 * time.Second))
	if _, _, err := NewFramedConn(b).ReadFrame(); err != nil {
		out.BadVersionHeader = "refused: " + err.Error()
	} else {
		out.BadVersionHeader = "served"
	}
	return out
}
