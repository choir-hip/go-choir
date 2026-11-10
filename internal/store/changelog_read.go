package store

import (
	"context"
	"fmt"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// ListChangelogEvents reads an owner's changelog events (the records in
// changelog:<owner>) by level and range, for the changelog app (lane E). It is
// the existing inbox read (ListInboxRecords) with the changelog filter on top;
// no new route. Major events only unless q.IncludeMinor.
func (s *Store) ListChangelogEvents(ctx context.Context, ownerID, computerID string, q types.ChangelogQuery) ([]types.ChangelogEvent, error) {
	ownerID, computerID = strings.TrimSpace(ownerID), strings.TrimSpace(computerID)
	if ownerID == "" || computerID == "" {
		return nil, fmt.Errorf("changelog list requires owner and computer")
	}
	records, err := s.ListInboxRecords(ctx, ownerID, computerID, types.ChangelogInbox(ownerID), q.AfterSeq, 0)
	if err != nil {
		return nil, err
	}
	return types.FilterChangelogRecords(records, q), nil
}
