package yaegikernel

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"
	"sync"

	"golang.org/x/sys/unix"
)

// Multiplexed worker transport (RLM Target Architecture, Step 2): the broker
// and its Yaegi session worker talk over a Unix domain socketpair, not raw
// stdin/stdout pipes. Frames multiplex logical streams over the one socket so
// cell traffic, cancellation, and (future) live stdout/stderr chunks never
// share an unstructured byte pipe.
//
// Frame wire format: 8-byte header + payload.
// Header: payload length uint32 big-endian, stream byte, 3 reserved bytes.
// Max payload is 4 MiB; larger payloads are a protocol error, never silently
// truncated.

// Worker streams multiplexed over the session socket.
const (
	// StreamCell carries SessionFrame requests (broker->worker) and
	// SessionResult responses (worker->broker) as JSON payloads.
	StreamCell = 0
	// StreamCancel carries broker->worker cancellation for the in-flight
	// cell. The worker abandons the cell; the session is poisoned.
	StreamCancel = 1
	// StreamStdout and StreamStderr carry worker->broker live output chunks.
	// Reserved in v1 (output still rides the result frame); receivers must
	// tolerate and ignore them.
	StreamStdout = 2
	StreamStderr = 3
)

// MaxFramePayload bounds one frame. Cells exceeding it are rejected before
// send; the tray quotas (16 intents, 256 KiB aggregate) sit far below it.
const MaxFramePayload = 4 << 20

const frameHeaderLen = 8

// FramedConn is a concurrency-safe framed Unix socket endpoint. Writes from
// multiple goroutines serialize; reads are single-owner (one reader loop).
type FramedConn struct {
	conn net.Conn
	wmu  sync.Mutex
}

// NewFramedConn wraps an established stream socket (one socketpair end).
func NewFramedConn(conn net.Conn) *FramedConn {
	return &FramedConn{conn: conn}
}

// WriteFrame sends one payload on a stream. Payloads over MaxFramePayload
// are rejected without touching the wire.
func (c *FramedConn) WriteFrame(stream byte, payload []byte) error {
	if len(payload) > MaxFramePayload {
		return fmt.Errorf("frame: payload %d bytes exceeds %d", len(payload), MaxFramePayload)
	}
	var hdr [frameHeaderLen]byte
	binary.BigEndian.PutUint32(hdr[0:4], uint32(len(payload)))
	hdr[4] = stream
	c.wmu.Lock()
	defer c.wmu.Unlock()
	if _, err := c.conn.Write(hdr[:]); err != nil {
		return fmt.Errorf("frame: write header: %w", err)
	}
	if _, err := c.conn.Write(payload); err != nil {
		return fmt.Errorf("frame: write payload: %w", err)
	}
	return nil
}

// ReadFrame blocks for the next frame, returning its stream and payload.
// Unknown streams are returned verbatim; the caller decides to tolerate them.
func (c *FramedConn) ReadFrame() (byte, []byte, error) {
	var hdr [frameHeaderLen]byte
	if _, err := io.ReadFull(c.conn, hdr[:]); err != nil {
		return 0, nil, fmt.Errorf("frame: read header: %w", err)
	}
	length := binary.BigEndian.Uint32(hdr[0:4])
	if length > MaxFramePayload {
		return 0, nil, fmt.Errorf("frame: peer advertised %d bytes, exceeds %d", length, MaxFramePayload)
	}
	payload := make([]byte, length)
	if _, err := io.ReadFull(c.conn, payload); err != nil {
		return 0, nil, fmt.Errorf("frame: read payload: %w", err)
	}
	return hdr[4], payload, nil
}

// Close closes the underlying socket.
func (c *FramedConn) Close() error {
	return c.conn.Close()
}

// SocketPair returns two connected Unix domain stream sockets. The broker
// keeps one end and passes the other to the session worker as an inherited
// file descriptor, so worker traffic never touches process stdio.
func SocketPair() (net.Conn, net.Conn, error) {
	fds, err := unix.Socketpair(unix.AF_UNIX, unix.SOCK_STREAM, 0)
	if err != nil {
		return nil, nil, fmt.Errorf("socketpair: %w", err)
	}
	fileA := os.NewFile(uintptr(fds[0]), "session-sock-a")
	fileB := os.NewFile(uintptr(fds[1]), "session-sock-b")
	connA, err := net.FileConn(fileA)
	_ = fileA.Close()
	if err != nil {
		_ = unix.Close(fds[0])
		_ = unix.Close(fds[1])
		return nil, nil, fmt.Errorf("socketpair fileconn a: %w", err)
	}
	connB, err := net.FileConn(fileB)
	_ = fileB.Close()
	if err != nil {
		_ = connA.Close()
		_ = unix.Close(fds[1])
		return nil, nil, fmt.Errorf("socketpair fileconn b: %w", err)
	}
	return connA, connB, nil
}
