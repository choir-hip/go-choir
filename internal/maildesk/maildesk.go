// Package maildesk is the Mail desk's custodian service (modular architecture
// plan MA4.7, section 8.2 "Mail ... Mail desk, through the maild service").
//
// The Mail desk absorbed the carrier-less email row (deskmanifest.Mail). Mail
// data stays in maild; the Mail desk is its custodian, and this service is the
// only way a desk reaches it. The seam is Mailbox: maild's own store satisfies
// it (maild.Store.DeskMailbox), so no mail is copied into the guest store.
//
// Another desk never reads mail directly. It asks Mail through a
// precommitment-record request (lease.Service.IssueGrant: a scored precommit
// addressed to the Mail desk) for one email by ref. Mail's rule decides: the
// asking desk's ceiling names mail.read, the request names exactly one ref, the
// ref is a message in the owner's mailbox, and the run holds no other mail.read
// lease (one email per task). The answer is a per-task mail.read lease on the
// tape. Read then serves that ref, and only that ref, for that run; a second
// email is refused at the request and at the read.
//
// Objects the Mail desk writes are package kinds (choir.app.mail.*) written
// by the reducer with the desk's own writer (MA3.8); this slice writes none.
// The Mail UI moves into the Mail package in Phase 5 (C13).
package maildesk

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/lease"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Message is one email as the Mail custodian serves it to a lease holder: the
// owner-visible fields, labelled untrusted external content.
type Message struct {
	Ref         string `json:"ref"`
	From        string `json:"from"`
	FromDisplay string `json:"from_display,omitempty"`
	Subject     string `json:"subject"`
	TextBody    string `json:"text_body"`
	ReceivedAt  string `json:"received_at,omitempty"`
	TrustStatus string `json:"trust_status,omitempty"`
	// Label is the provenance label the read carries (owner_private:mail).
	Label string `json:"label"`
}

// Mailbox is the maild service seam: the Mail custodian's one read of the
// owner's mail. It returns ErrNoMessage when the ref is not a message in
// owner's mailbox.
type Mailbox interface {
	Message(ctx context.Context, ownerID, ref string) (Message, error)
}

// ErrNoMessage: the ref is not a message in the owner's mailbox.
var ErrNoMessage = errors.New("maildesk: no such message")

// ErrNoLease refuses a read the run holds no mail.read lease for.
var ErrNoLease = errors.New("maildesk: mail.read needs a lease on this email held by this run")

// MailLabel is the provenance label of every email the Mail desk serves.
var MailLabel = types.OwnerPrivateLabel("mail")

// Service is the Mail desk's custodian service.
type Service struct {
	mailbox Mailbox
	leases  *lease.Service
}

// New returns the Mail custodian over the maild seam and the lease service.
func New(mailbox Mailbox, leases *lease.Service) *Service {
	return &Service{mailbox: mailbox, leases: leases}
}

// ReadRequest is a desk's ask for one email. Desk, RunID, AgentID and ModelID
// come from the asking run's record in trusted code, never from a cell; Ref,
// Purpose and Fields are what the cell supplies.
type ReadRequest struct {
	OwnerID    string
	ComputerID string
	Desk       string
	RunID      string
	AgentID    string
	ModelID    string
	// Refs are the emails asked for. Mail grants exactly one.
	Refs    []string
	Purpose string
	Fields  *types.RequestFields
}

// RequestRead is Mail's answer to a request for one email: the request is
// recorded as a scored precommitment addressed to the Mail desk, and Mail's
// rule decides. Granted, it returns the per-task mail.read lease; refused, the
// error wraps lease.ErrRefused and the request stays on the tape.
func (s *Service) RequestRead(ctx context.Context, req ReadRequest) (lease.Record, error) {
	if s == nil || s.leases == nil || s.mailbox == nil {
		return lease.Record{}, fmt.Errorf("%w: no Mail custodian on this computer", lease.ErrRefused)
	}
	refs := make([]string, 0, len(req.Refs))
	for _, r := range req.Refs {
		if r = strings.TrimSpace(r); r != "" {
			refs = append(refs, r)
		}
	}
	rule := func(held []lease.Record) error {
		if len(refs) != 1 {
			return fmt.Errorf("%w: Mail grants one email per request, by ref; %d named", lease.ErrRefused, len(refs))
		}
		for _, h := range held {
			for _, g := range h.Scope.Grants {
				if g == lease.GrantMailRead {
					return fmt.Errorf("%w: this task already holds a mail.read lease (%s); Mail grants one email per task", lease.ErrRefused, h.LeaseID)
				}
			}
		}
		if _, err := s.mailbox.Message(ctx, req.OwnerID, refs[0]); err != nil {
			if errors.Is(err, ErrNoMessage) {
				return fmt.Errorf("%w: %q is not a message in the owner's mailbox", lease.ErrRefused, refs[0])
			}
			return fmt.Errorf("%w: Mail could not check the ref: %v", lease.ErrRefused, err)
		}
		return nil
	}
	return s.leases.IssueGrant(ctx, lease.WriterMail, lease.Request{
		OwnerID: req.OwnerID, ComputerID: req.ComputerID, Desk: req.Desk, RunID: req.RunID,
		// A desk's run asks: the origin is stamped here, never supplied.
		AgentID: req.AgentID, ModelID: req.ModelID, Origin: types.OriginDeskRun,
		Scope:   lease.Scope{Grants: []string{lease.GrantMailRead}, Refs: req.Refs},
		Purpose: req.Purpose, Fields: req.Fields,
	}, rule)
}

// Read serves one email to the run ctx carries. The holder is the declared
// reader of ctx (the run's desk, set by trusted code from the run record) and
// the run is the one ctx's lease authority was attached for; the lease must be
// ownerID's and name this ref. Only then does Mail read maild, as the custodian, for that
// ref alone.
func (s *Service) Read(ctx context.Context, ownerID, ref string) (Message, error) {
	if s == nil || s.mailbox == nil {
		return Message{}, ErrNoLease
	}
	holder, ok := objectgraph.ReaderFrom(ctx)
	if !ok {
		return Message{}, ErrNoLease
	}
	ref = strings.TrimSpace(ref)
	if _, ok := objectgraph.LeaseAdmitsGrantRef(ctx, holder, ownerID, lease.GrantMailRead, ref); !ok {
		return Message{}, ErrNoLease
	}
	objectgraph.NoteLeaseRead(holder, "mail:"+ref)
	m, err := s.mailbox.Message(ctx, ownerID, ref)
	if err != nil {
		return Message{}, err
	}
	m.Ref = ref
	m.Label = MailLabel
	return m, nil
}
