package yaegikernel

import (
	"bytes"
	"sync"
	"testing"
)

func TestFrameRoundTrip(t *testing.T) {
	a, b, err := SocketPair()
	if err != nil {
		t.Fatalf("socketpair: %v", err)
	}
	defer a.Close()
	defer b.Close()
	fa, fb := NewFramedConn(a), NewFramedConn(b)
	payload := []byte(`{"id":"cell-1","source":"x := 1"}`)
	if err := fa.WriteFrame(StreamCell, payload); err != nil {
		t.Fatal(err)
	}
	stream, back, err := fb.ReadFrame()
	if err != nil {
		t.Fatal(err)
	}
	if stream != StreamCell || !bytes.Equal(back, payload) {
		t.Fatalf("roundtrip stream=%d payload=%q", stream, back)
	}
}

func TestFrameMultiplexInterleave(t *testing.T) {
	a, b, err := SocketPair()
	if err != nil {
		t.Fatalf("socketpair: %v", err)
	}
	defer a.Close()
	defer b.Close()
	fa, fb := NewFramedConn(a), NewFramedConn(b)
	// Worker emits live output chunks around its result; order is preserved
	// on the single socket and streams stay distinguishable.
	if err := fa.WriteFrame(StreamStdout, []byte("partial")); err != nil {
		t.Fatal(err)
	}
	if err := fa.WriteFrame(StreamCell, []byte(`{"id":"c1"}`)); err != nil {
		t.Fatal(err)
	}
	if err := fa.WriteFrame(StreamStderr, []byte("warn")); err != nil {
		t.Fatal(err)
	}
	want := []struct {
		stream byte
		body   string
	}{
		{StreamStdout, "partial"}, {StreamCell, `{"id":"c1"}`}, {StreamStderr, "warn"},
	}
	for _, w := range want {
		stream, body, err := fb.ReadFrame()
		if err != nil {
			t.Fatal(err)
		}
		if stream != w.stream || string(body) != w.body {
			t.Fatalf("frame = (%d,%q), want (%d,%q)", stream, body, w.stream, w.body)
		}
	}
}

func TestFrameOversizeRejected(t *testing.T) {
	a, b, err := SocketPair()
	if err != nil {
		t.Fatalf("socketpair: %v", err)
	}
	defer a.Close()
	defer b.Close()
	fa := NewFramedConn(a)
	if err := fa.WriteFrame(StreamCell, make([]byte, MaxFramePayload+1)); err == nil {
		t.Fatal("oversize payload must be rejected before send")
	}
}

func TestFrameConcurrentWriters(t *testing.T) {
	a, b, err := SocketPair()
	if err != nil {
		t.Fatalf("socketpair: %v", err)
	}
	defer a.Close()
	defer b.Close()
	fa, fb := NewFramedConn(a), NewFramedConn(b)
	const writers = 8
	var wg sync.WaitGroup
	for i := range writers {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_ = fa.WriteFrame(StreamCell, []byte{byte(i)})
		}(i)
	}
	wg.Wait()
	seen := map[byte]bool{}
	for range writers {
		stream, body, err := fb.ReadFrame()
		if err != nil {
			t.Fatal(err)
		}
		if stream != StreamCell || len(body) != 1 {
			t.Fatalf("corrupt multiplexed frame: (%d,%q)", stream, body)
		}
		seen[body[0]] = true
	}
	if len(seen) != writers {
		t.Fatalf("lost frames under concurrency: %d/%d", len(seen), writers)
	}
}
