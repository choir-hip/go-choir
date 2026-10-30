package computerevent

import (
	"context"
	"sync"
)

// A replay applies events one at a time, and each projection batch names a
// payload the guest fetches from the host: one round trip per event. On d@e
// that held replay to about 200 events a second (docs/definitions/
// choir-data-system-build-2026-10-30.ledger.md). The prefetch fetches a
// page's payloads concurrently before the page applies. It only fetches:
// ResolvePayloads still checks every digest and decrypts in event order, and
// a payload the prefetch missed or failed is fetched again in line, which
// reports the error.
const (
	replayPrefetchWorkers  = 8
	replayPrefetchMaxBytes = 64 << 20
)

// prefetchReplayPayloads returns the raw payloads of the page's projection
// batches that are not already held live, keyed by digest. It stops storing
// once the page's payloads pass replayPrefetchMaxBytes.
func (a *ComputerEventAppender) prefetchReplayPayloads(ctx context.Context, page []DurableEvent) map[string][]byte {
	if a.reader == nil || len(page) == 0 {
		return nil
	}
	digests := make([]string, 0, len(page))
	seen := map[string]bool{}
	for _, record := range page {
		event := record.Request.Event
		if event.EventKind != EventProjectionBatchRecorded || !IsSHA256(event.PayloadCommitment) || seen[event.PayloadCommitment] {
			continue
		}
		if _, live := a.livePayloads[event.PayloadCommitment]; live {
			continue
		}
		seen[event.PayloadCommitment] = true
		digests = append(digests, event.PayloadCommitment)
	}
	if len(digests) < 2 {
		return nil
	}
	var (
		mu    sync.Mutex
		out   = make(map[string][]byte, len(digests))
		bytes int
		wg    sync.WaitGroup
		next  = make(chan string)
	)
	workers := min(replayPrefetchWorkers, len(digests))
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for digest := range next {
				raw, err := a.reader.FetchPayload(ctx, a.computerID, digest)
				if err != nil {
					continue
				}
				mu.Lock()
				if bytes+len(raw) <= replayPrefetchMaxBytes {
					out[digest] = raw
					bytes += len(raw)
				}
				mu.Unlock()
			}
		}()
	}
	for _, digest := range digests {
		if ctx.Err() != nil {
			break
		}
		next <- digest
	}
	close(next)
	wg.Wait()
	return out
}

// prefetchedReader serves prefetched payloads and falls through to the
// underlying reader for the rest.
type prefetchedReader struct {
	computerID string
	held       map[string][]byte
	next       ArtifactReader
}

func (r prefetchedReader) FetchPayload(ctx context.Context, computerID, artifactDigest string) ([]byte, error) {
	if raw, ok := r.held[artifactDigest]; ok && computerID == r.computerID {
		return raw, nil
	}
	return r.next.FetchPayload(ctx, computerID, artifactDigest)
}
