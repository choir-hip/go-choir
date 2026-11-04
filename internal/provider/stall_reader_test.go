package provider

import (
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

func withWindows(t *testing.T, first, idle time.Duration) {
	of, oi := streamFirstByteWindow, streamIdleWindow
	streamFirstByteWindow, streamIdleWindow = first, idle
	t.Cleanup(func() { streamFirstByteWindow, streamIdleWindow = of, oi })
}

func readAllWithin(t *testing.T, r io.Reader) error {
	t.Helper()
	done := make(chan error, 1)
	go func() { _, err := io.ReadAll(r); done <- err }()
	select {
	case err := <-done:
		return err
	case <-time.After(5 * time.Second):
		t.Fatal("read did not return")
		return nil
	}
}

func TestStallFirstByte(t *testing.T) {
	withWindows(t, 50*time.Millisecond, time.Second)
	pr, pw := io.Pipe()
	defer pw.Close()
	err := readAllWithin(t, withStallWatchdog(pr))
	if !errors.Is(err, errStreamStalled) || !strings.Contains(err.Error(), "stream stalled: no bytes for 0s") {
		t.Fatalf("err = %v", err)
	}
}

func TestStallMidStream(t *testing.T) {
	withWindows(t, time.Second, 50*time.Millisecond)
	pr, pw := io.Pipe()
	defer pw.Close()
	go pw.Write([]byte("data: x\n\n"))
	if err := readAllWithin(t, withStallWatchdog(pr)); !errors.Is(err, errStreamStalled) {
		t.Fatalf("err = %v", err)
	}
}

func TestSlowFirstByteWithinWindowAndTrickleSurvive(t *testing.T) {
	withWindows(t, 300*time.Millisecond, 150*time.Millisecond)
	pr, pw := io.Pipe()
	r := withStallWatchdog(pr)
	go func() {
		time.Sleep(150 * time.Millisecond) // silent thinking, inside the window
		for i := 0; i < 5; i++ {
			pw.Write([]byte(": ping\n"))
			time.Sleep(100 * time.Millisecond) // total > idle window, each gap inside it
		}
		pw.Close()
	}()
	if err := readAllWithin(t, r); err != nil {
		t.Fatalf("err = %v", err)
	}
	if firstByteOf(r) < 100*time.Millisecond {
		t.Fatalf("ttfb = %v", firstByteOf(r))
	}
}

func TestStopAndUncloseable(t *testing.T) {
	withWindows(t, 30*time.Millisecond, 30*time.Millisecond)
	if r := strings.NewReader("x"); withStallWatchdog(r) != io.Reader(r) {
		t.Fatal("uncloseable body must stay unwrapped")
	}
	pr, pw := io.Pipe()
	r := withStallWatchdog(pr)
	stopStallWatchdog(r)
	time.Sleep(100 * time.Millisecond)
	go pw.Write([]byte("ok"))
	buf := make([]byte, 2)
	if _, err := r.Read(buf); err != nil {
		t.Fatalf("stopped watchdog still fired: %v", err)
	}
}

func TestOpenAIStreamStallSurfacesAsReadStreamError(t *testing.T) {
	withWindows(t, 50*time.Millisecond, 50*time.Millisecond)
	pr, pw := io.Pipe()
	defer pw.Close()
	done := make(chan error, 1)
	go func() {
		_, err := parseOpenAIStream(pr, "m", "chatgpt", func(StreamChunk) {})
		done <- err
	}()
	select {
	case err := <-done:
		if err == nil || !strings.Contains(err.Error(), "chatgpt: read stream: stream stalled: no bytes for") {
			t.Fatalf("err = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("parser did not abort")
	}
}
