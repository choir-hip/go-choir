package platform

// Review-panel regression R2 (blocking), written before the fix: a
// concurrent pin must not acknowledge an artifact whose bytes a failed pin
// removed. A pins payload P and its fsync fails while B pins the same P: B
// must never omit P on the strength of an unlocked read of A's unsynced
// bytes. Through the production pin path, with the segment fsync seam.

import (
	"context"
	"errors"
	"os"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

func TestTapeSegmentsConcurrentPinDoesNotAcknowledgeAnArtifactAFailedPinRemoved(t *testing.T) {
	f := newWriteFixture(t, "write", true)
	const id = "computer-failed-pin"
	payload := projectionPayload("shared by two concurrent pins")
	digest := computerevent.DigestBytes(payload)

	entered := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	var mu sync.Mutex
	syncs := 0
	f.service.tapeAppender().SetSyncHookForTest(func(file *os.File) error {
		mu.Lock()
		syncs++
		n := syncs
		mu.Unlock()
		if n == 1 {
			once.Do(func() { close(entered) })
			<-release
			return syscall.EIO
		}
		return file.Sync()
	})

	var aErr, bErr error
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, aErr = f.artifacts.pinPayload(context.Background(), id, payload, computerevent.ProjectionBatchMediaType, "public", platformTestDigest('1'))
	}()
	<-entered // A has written P and is inside its fsync
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, bErr = f.artifacts.pinPayload(context.Background(), id, payload, computerevent.ProjectionBatchMediaType, "public", platformTestDigest('2'))
	}()
	time.Sleep(250 * time.Millisecond) // B reaches its checks while A is blocked
	close(release)
	wg.Wait()

	if !errors.Is(aErr, syscall.EIO) {
		t.Fatalf("A = %v, want the fsync error", aErr)
	}
	if bErr != nil {
		t.Fatalf("B = %v", bErr)
	}
	got, err := f.artifacts.FetchPayload(id, digest)
	if err != nil || string(got) != string(payload) {
		t.Fatalf("B was acknowledged but its payload is not on the tape: %v", err)
	}
}
