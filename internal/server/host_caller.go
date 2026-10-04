package server

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/netip"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
)

var (
	hostGatewayOnce sync.Once
	hostGateway     atomic.Pointer[netip.Addr]
)

// HostSourcedCaller reports whether r arrived from the guest's host peer or an
// in-guest caller. Guest user identity and internal guest routes must not trust
// caller-asserted headers: a guest can set those headers but cannot spoof the
// host peer source address after the tap anti-spoofing rule is installed.
//
// The host peer is the guest default gateway. A missing or malformed route is
// refused closed: the guest image always provides /proc/net/route, and allowing
// a request when its gateway cannot be established would restore header trust.
func HostSourcedCaller(r *http.Request) bool {
	if r == nil {
		return false
	}
	if r.RemoteAddr == "" || strings.HasPrefix(r.RemoteAddr, "@") || strings.HasPrefix(r.RemoteAddr, "/") {
		return true
	}

	remote, ok := remoteAddrIP(r.RemoteAddr)
	if !ok {
		return false
	}
	if remote.IsLoopback() || isTestNet1(remote) {
		return true
	}

	hostGatewayOnce.Do(cacheHostGateway)
	gateway := hostGateway.Load()
	if gateway == nil {
		// Do not permanently cache a transient /proc read failure. The next
		// request refreshes it, while this request remains fail-closed.
		if parsed, err := defaultGateway(); err != nil {
			log.Printf("server: refusing guest authority request: cannot determine default gateway: %v", err)
			return false
		} else {
			hostGateway.CompareAndSwap(nil, &parsed)
			gateway = hostGateway.Load()
		}
	}
	return gateway != nil && remote == *gateway
}

// HostPeerCaller is the strict form of HostSourcedCaller: only the host peer
// itself (the guest's default gateway on the tap link) qualifies. In-guest
// callers — loopback and the guest's own services — do not, so diagnostic
// surfaces like /internal/diag/tcp-dial cannot be driven by untrusted guest
// children (terminal, zot, capsule workloads). Synthetic callers (empty and
// TEST-NET-1 addresses) still pass for tests; loopback does not.
func HostPeerCaller(r *http.Request) bool {
	if r == nil {
		return false
	}
	if r.RemoteAddr == "" || strings.HasPrefix(r.RemoteAddr, "@") || strings.HasPrefix(r.RemoteAddr, "/") {
		return true
	}
	remote, ok := remoteAddrIP(r.RemoteAddr)
	if !ok {
		return false
	}
	if remote.IsLoopback() {
		return false
	}
	if isTestNet1(remote) {
		return true
	}
	hostGatewayOnce.Do(cacheHostGateway)
	gateway := hostGateway.Load()
	if gateway == nil {
		if parsed, err := defaultGateway(); err != nil {
			log.Printf("server: refusing host-peer request: cannot determine default gateway: %v", err)
			return false
		} else {
			hostGateway.CompareAndSwap(nil, &parsed)
			gateway = hostGateway.Load()
		}
	}
	return gateway != nil && remote == *gateway
}

func cacheHostGateway() {
	gateway, err := defaultGateway()
	if err != nil {
		log.Printf("server: cannot determine guest default gateway; host-sourced authority requests will be refused: %v", err)
		return
	}
	hostGateway.Store(&gateway)
}

func remoteAddrIP(remoteAddr string) (netip.Addr, bool) {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	ip, err := netip.ParseAddr(host)
	if err != nil {
		return netip.Addr{}, false
	}
	return ip.Unmap(), true
}

func isTestNet1(ip netip.Addr) bool {
	if !ip.Is4() {
		return false
	}
	octets := ip.As4()
	return octets[0] == 192 && octets[1] == 0 && octets[2] == 2
}

func defaultGateway() (netip.Addr, error) {
	file, err := os.Open("/proc/net/route")
	if err != nil {
		return netip.Addr{}, err
	}
	defer func() { _ = file.Close() }()
	return parseDefaultGateway(file)
}

func parseDefaultGateway(routeTable io.Reader) (netip.Addr, error) {
	scanner := bufio.NewScanner(routeTable)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return netip.Addr{}, err
		}
		return netip.Addr{}, fmt.Errorf("route table is empty")
	}
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 4 || fields[1] != "00000000" {
			continue
		}
		flags, err := strconv.ParseUint(fields[3], 16, 32)
		if err != nil || flags&0x2 == 0 {
			continue
		}
		gateway, err := strconv.ParseUint(fields[2], 16, 32)
		if err != nil {
			continue
		}
		return netip.AddrFrom4([4]byte{byte(gateway), byte(gateway >> 8), byte(gateway >> 16), byte(gateway >> 24)}), nil
	}
	if err := scanner.Err(); err != nil {
		return netip.Addr{}, err
	}
	return netip.Addr{}, fmt.Errorf("no default gateway route")
}
