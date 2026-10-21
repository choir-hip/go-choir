package yaegikernel

import (
	"errors"
	"io"
	"testing"
	"time"
)

// ARCH-07: the frame header carries a wire version in its first reserved byte.
// A peer on another version, or one that sets the remaining reserved bytes, is
// refused before its payload is read. Failure mode F4.

func rawHeader(length uint32, stream, version, r1, r2 byte) []byte {
	return []byte{byte(length >> 24), byte(length >> 16), byte(length >> 8), byte(length), stream, version, r1, r2}
}

func TestFrameHeaderCarriesVersionOne(t *testing.T) {
	a, b, err := SocketPair()
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	defer b.Close()
	if err := NewFramedConn(a).WriteFrame(StreamCell, []byte("hello")); err != nil {
		t.Fatal(err)
	}
	hdr := make([]byte, frameHeaderLen)
	if _, err := io.ReadFull(b, hdr); err != nil {
		t.Fatal(err)
	}
	if hdr[5] != FrameVersion || FrameVersion != 1 {
		t.Fatalf("header version byte = %d (FrameVersion %d), want 1", hdr[5], FrameVersion)
	}
	if hdr[6] != 0 || hdr[7] != 0 {
		t.Fatalf("reserved bytes = %v, want zero", hdr[6:8])
	}
}

func TestFrameRefusesOtherVersionsAndReservedBytes(t *testing.T) {
	cases := map[string][]byte{
		"version-0 (a peer from before the version byte)": rawHeader(4<<20, StreamCell, 0, 0, 0),
		"version-2":      rawHeader(4<<20, StreamCell, 2, 0, 0),
		"version-255":    rawHeader(4<<20, StreamCell, 255, 0, 0),
		"reserved-1-set": rawHeader(4<<20, StreamCell, 1, 1, 0),
		"reserved-2-set": rawHeader(4<<20, StreamCell, 1, 0, 1),
	}
	for name, hdr := range cases {
		name, hdr := name, hdr
		t.Run(name, func(t *testing.T) {
			a, b, err := SocketPair()
			if err != nil {
				t.Fatal(err)
			}
			defer a.Close()
			defer b.Close()
			// The header advertises a 4 MiB payload that is never sent: the
			// refusal must come from the header alone, not from waiting on
			// (or parsing) the payload.
			if _, err := a.Write(hdr); err != nil {
				t.Fatal(err)
			}
			done := make(chan error, 1)
			go func() {
				_, _, err := NewFramedConn(b).ReadFrame()
				done <- err
			}()
			select {
			case err := <-done:
				if !errors.Is(err, ErrFrameVersion) {
					t.Fatalf("want ErrFrameVersion, got %v", err)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("ReadFrame waited on a payload behind an unsupported header")
			}
		})
	}
}
