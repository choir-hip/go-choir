package store

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/lease"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Lease records (MA3.4): the custodian-issued grants of internal/lease,
// persisted as objects of kind choir.lease, written under the lease service's
// writer. The store implements lease.Persistence; the service keeps the live
// index the reader gate asks.

const ogKindLease = objectgraph.ObjectKind("choir.lease")

// LeaseStore adapts the store to lease.Persistence.
type LeaseStore struct{ s *Store }

// Leases returns the store's lease persistence.
func (s *Store) Leases() LeaseStore { return LeaseStore{s: s} }

func leaseKey(computerID, leaseID string) string {
	return strings.TrimSpace(computerID) + "\x00" + strings.TrimSpace(leaseID)
}

// PutLease writes a lease record, replacing the earlier state of the same
// lease (issue, then its end).
func (l LeaseStore) PutLease(ctx context.Context, rec lease.Record) error {
	if strings.TrimSpace(rec.OwnerID) == "" || strings.TrimSpace(rec.LeaseID) == "" {
		return fmt.Errorf("lease: owner and lease id are required")
	}
	metadata := map[string]any{
		"computer_id": rec.ComputerID, "lease_id": rec.LeaseID, "holder": rec.Holder,
		"run_id": rec.HolderRunID, "state": rec.State, "issued_at": rec.IssuedAt.UTC().Format(time.RFC3339Nano),
	}
	_, err := l.s.ogPut(objectgraph.WithWriter(ctx, objectgraph.WriterLease), ogKindLease, rec.OwnerID, leaseKey(rec.ComputerID, rec.LeaseID), rec, metadata, time.Now().UTC())
	return err
}

// ListLeases returns every lease of the computer, any state.
func (l LeaseStore) ListLeases(ctx context.Context, ownerID, computerID string) ([]lease.Record, error) {
	ctx = objectgraph.WithReader(ctx, objectgraph.WriterLease)
	limit := 200
	for {
		objs, err := l.s.ogListByOwnerAndBody(ctx, ogKindLease, strings.TrimSpace(ownerID),
			[]objectgraph.JSONFieldMatch{{JSONPath: "$.computer_id", Value: strings.TrimSpace(computerID)}}, limit)
		if err != nil {
			return nil, err
		}
		if len(objs) >= limit {
			limit *= 4
			continue
		}
		out := make([]lease.Record, 0, len(objs))
		for _, obj := range objs {
			if obj.Tombstone {
				continue
			}
			var rec lease.Record
			if err := ogDecode(obj, &rec); err != nil {
				return nil, err
			}
			out = append(out, rec)
		}
		return out, nil
	}
}

// AppendRequestRecord writes a lease request's precommitment to the ledger.
func (l LeaseStore) AppendRequestRecord(ctx context.Context, ownerID, computerID string, rec types.CommitmentRecord) (string, error) {
	return l.s.AppendCommitmentRecord(ctx, ownerID, computerID, rec)
}
