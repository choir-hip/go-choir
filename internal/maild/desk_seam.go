package maild

import (
	"context"
	"database/sql"
	"errors"

	"github.com/yusefmosiah/go-choir/internal/maildesk"
)

// DeskMailbox is the maild service seam the Mail desk is custodian of (plan
// MA4.7): the Mail custodian (internal/maildesk) reads one owner-visible
// message by ref through it, and mail never leaves maild otherwise.
func (s *Store) DeskMailbox() maildesk.Mailbox { return deskMailbox{s} }

type deskMailbox struct{ s *Store }

func (m deskMailbox) Message(ctx context.Context, ownerID, ref string) (maildesk.Message, error) {
	msg, err := m.s.GetMessage(ctx, ownerID, ref)
	if errors.Is(err, sql.ErrNoRows) {
		return maildesk.Message{}, maildesk.ErrNoMessage
	}
	if err != nil {
		return maildesk.Message{}, err
	}
	at := msg.ReceivedAt
	if at == "" {
		at = msg.SentAt
	}
	return maildesk.Message{
		Ref: msg.ID, From: msg.FromAddress, FromDisplay: msg.FromDisplay, Subject: msg.Subject,
		TextBody: msg.TextBody, ReceivedAt: at, TrustStatus: msg.TrustStatus,
	}, nil
}
