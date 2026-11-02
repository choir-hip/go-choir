package autoputer

// The network kinds of the isolation drill (guest isolation I5): net-conns
// holds many TCP connections open, net-flood pushes bytes down a few. Both
// may only dial a literal IPv4 address inside drillNetAllow: the deprecated
// 6to4 relay block 192.88.99.0/24, which no network routes and which the
// operator answers from a network namespace on Node B, so a drill never
// loads a third party. It is not private space: Node B's CHOIR-GUEST-EGRESS
// (mangle) drops a guest's packets to every RFC 1918 block, so a 10.x target
// is unreachable by design (the first deployed drill, 2026-11-02). Seconds
// are capped at drillNetMaxSeconds.

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

const (
	drillNetMaxSeconds = 60
	drillNetMaxConns   = 12000
	drillNetMaxStreams = 16
	drillNetDialers    = 256
	drillNetDialWait   = 3 * time.Second
	drillNetChunk      = 64 << 10
)

// drillNetAllow is the only block a network drill may dial.
var drillNetAllow = netip.MustParsePrefix("192.88.99.0/24")

// parseDrillTarget accepts only host:port with a literal IPv4 inside
// drillNetAllow and a port of 1..65535. No name is ever resolved.
func parseDrillTarget(raw string) (string, error) {
	ap, err := netip.ParseAddrPort(raw)
	if err != nil {
		return "", fmt.Errorf("target must be ip:port: %w", err)
	}
	a := ap.Addr()
	if !a.Is4() || !drillNetAllow.Contains(a) || ap.Port() == 0 {
		return "", fmt.Errorf("target must be an IPv4 address in %s", drillNetAllow)
	}
	return ap.String(), nil
}

func raiseNoFile(want uint64) {
	var l syscall.Rlimit
	if syscall.Getrlimit(syscall.RLIMIT_NOFILE, &l) != nil {
		return
	}
	if uint64(l.Cur) >= want {
		return
	}
	n := want
	if uint64(l.Max) < n {
		n = uint64(l.Max)
	}
	l.Cur = n
	_ = syscall.Setrlimit(syscall.RLIMIT_NOFILE, &l)
}

// drillNetConns opens count connections to target (at most drillNetDialers
// dialing at once), holds those that opened until ctx ends, then closes them.
// A connection the host's connlimit drops times out at the dial and counts
// in Failed.
func drillNetConns(ctx context.Context, target string, count int) drillReport {
	raiseNoFile(uint64(count) + 256)
	start := time.Now()
	var opened, failed atomic.Int64
	var mu sync.Mutex
	var held []net.Conn
	jobs := make(chan struct{})
	var wg sync.WaitGroup
	for range drillNetDialers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			d := net.Dialer{Timeout: drillNetDialWait}
			for range jobs {
				c, err := d.DialContext(ctx, "tcp4", target)
				if err != nil {
					failed.Add(1)
					continue
				}
				opened.Add(1)
				mu.Lock()
				held = append(held, c)
				mu.Unlock()
			}
		}()
	}
	// The dial phase is paced to use at most half the window.
	pace := time.Duration(0)
	if dl, ok := ctx.Deadline(); ok {
		pace = time.Until(dl) / 2 / time.Duration(count)
	}
feed:
	for i := 0; i < count; i++ {
		select {
		case jobs <- struct{}{}:
		case <-ctx.Done():
			break feed
		}
		if pace > 0 {
			time.Sleep(pace)
		}
	}
	close(jobs)
	wg.Wait()
	<-ctx.Done()
	mu.Lock()
	for _, c := range held {
		_ = c.Close()
	}
	mu.Unlock()
	return drillReport{Seconds: time.Since(start).Seconds(), Ops: int64(count), Opened: opened.Load(), Failed: failed.Load()}
}

// drillNetFlood writes zeros down streams connections until ctx ends.
func drillNetFlood(ctx context.Context, target string, streams int) drillReport {
	start := time.Now()
	var total, opened, failed atomic.Int64
	var wg sync.WaitGroup
	buf := make([]byte, drillNetChunk)
	for range streams {
		wg.Add(1)
		go func() {
			defer wg.Done()
			d := net.Dialer{Timeout: drillNetDialWait}
			c, err := d.DialContext(ctx, "tcp4", target)
			if err != nil {
				failed.Add(1)
				return
			}
			opened.Add(1)
			defer c.Close()
			go func() { <-ctx.Done(); _ = c.SetWriteDeadline(time.Now()) }()
			for ctx.Err() == nil {
				n, err := c.Write(buf)
				total.Add(int64(n))
				if err != nil {
					return
				}
			}
		}()
	}
	wg.Wait()
	return drillReport{Seconds: time.Since(start).Seconds(), Bytes: total.Load(), Opened: opened.Load(), Failed: failed.Load()}
}

func drillNetParams(kind string, q interface{ Get(string) string }) (target string, n int, msg string) {
	target, err := parseDrillTarget(q.Get("target"))
	if err != nil {
		return "", 0, err.Error()
	}
	if kind == "net-conns" {
		n, err = strconv.Atoi(q.Get("count"))
		if err != nil || n < 1 || n > drillNetMaxConns {
			return "", 0, fmt.Sprintf("count must be 1..%d", drillNetMaxConns)
		}
		return target, n, ""
	}
	n = 4
	if raw := q.Get("streams"); raw != "" {
		n, err = strconv.Atoi(raw)
		if err != nil || n < 1 || n > drillNetMaxStreams {
			return "", 0, fmt.Sprintf("streams must be 1..%d", drillNetMaxStreams)
		}
	}
	return target, n, ""
}
