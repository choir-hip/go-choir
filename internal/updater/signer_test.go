package updater

import (
	"context"
	"crypto/ed25519"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

type testReceiptSigner struct {
	key computerevent.SigningKey
}

func (s testReceiptSigner) PublicKey(context.Context) (computerevent.SignerRef, ed25519.PublicKey, error) {
	return s.key.SignerRef, append(ed25519.PublicKey(nil), s.key.PrivateKey.Public().(ed25519.PublicKey)...), nil
}

func (s testReceiptSigner) SignReceipt(_ context.Context, kind, issuer string, fields map[string]any, issuedAt time.Time) (computerevent.Receipt, error) {
	return computerevent.NewSignedReceipt(kind, issuer, fields, []computerevent.SigningKey{s.key}, issuedAt)
}
