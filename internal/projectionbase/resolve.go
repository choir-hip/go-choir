package projectionbase

import (
	"context"
	"fmt"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

// TailSource reads immutable tail pages. BaseSource satisfies it; tests stub it.
type TailSource interface {
	TailPage(ctx context.Context, computerID string, afterSequence uint64, pageSize int) ([]computerevent.DurableEvent, error)
}

// resolveTargetPageSize bounds each target-resolution page. Resolution is
// tail-bounded by construction: it starts after the watermark and stops at
// the first page whose head chain reaches the target.
const resolveTargetPageSize = 500

// resolveTargetMaxPages caps target resolution. A tail longer than the cap
// refuses rather than enumerating without bound; the cap documents the
// largest supported tail, not a prefix scan.
const resolveTargetMaxPages = 2000

// ResolveTargetSequence proves the recovery target is the watermark or
// descends from it using immutable tape: it returns the sequence whose
// resulting head equals targetHead. A degenerate W=H target resolves without
// paging; a target that never appears — older than W, foreign, or corrupt —
// refuses via ErrBaseRefused before any replay.
func ResolveTargetSequence(ctx context.Context, tail TailSource, computerID, targetHead string, fromSequence uint64, baseHead string) (uint64, error) {
	computerID = strings.TrimSpace(computerID)
	targetHead = strings.ToLower(strings.TrimSpace(targetHead))
	if !computerevent.IsSHA256(targetHead) || tail == nil || computerID == "" {
		return 0, fmt.Errorf("%w: target resolution requires computer, source, and head", ErrBaseRefused)
	}
	if targetHead == strings.ToLower(strings.TrimSpace(baseHead)) {
		return fromSequence, nil
	}
	after := fromSequence
	for page := 0; page < resolveTargetMaxPages; page++ {
		if err := ctx.Err(); err != nil {
			return 0, err
		}
		records, err := tail.TailPage(ctx, computerID, after, resolveTargetPageSize)
		if err != nil {
			return 0, fmt.Errorf("%w: read tail after %d: %v", ErrBaseRefused, after, err)
		}
		if len(records) == 0 {
			return 0, fmt.Errorf("%w: target is not a descendant of watermark %d", ErrBaseRefused, fromSequence)
		}
		for _, record := range records {
			after = record.Request.Event.Sequence
			if record.Request.Next.CanonicalEventHead == targetHead {
				return after, nil
			}
		}
		if len(records) < resolveTargetPageSize {
			return 0, fmt.Errorf("%w: target is not a descendant of watermark %d", ErrBaseRefused, fromSequence)
		}
	}
	return 0, fmt.Errorf("%w: target beyond resolution cap after watermark %d", ErrBaseRefused, fromSequence)
}
