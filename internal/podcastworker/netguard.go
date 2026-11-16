package podcastworker

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"
)

// The worker fetches addresses strangers choose: a feed a reader reports, the
// enclosure a feed names, a now-playing audio URL. Everything it fetches goes
// through one client whose dialer resolves the name itself, refuses every
// address that is not a public unicast address and dials the address it
// checked, so a redirect, a name that resolves privately and a rebinding
// answer all meet the same check. Only ports 80 and 443 are dialled.
// (Review 1 of 2026-11-16, blocker B1.)

const maxRedirects = 20

var privateNets = func() []*net.IPNet {
	var out []*net.IPNet
	for _, cidr := range []string{
		"0.0.0.0/8", "10.0.0.0/8", "100.64.0.0/10", "127.0.0.0/8", "169.254.0.0/16", "172.16.0.0/12", "192.0.0.0/24", "192.0.2.0/24",
		"192.88.99.0/24", "192.168.0.0/16", "198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "224.0.0.0/4", "240.0.0.0/4",
		"::/96", "64:ff9b::/96", "100::/64", "2001::/32", "2001:db8::/32", "2002::/16", "fc00::/7", "fe80::/10", "ff00::/8",
	} {
		_, n, err := net.ParseCIDR(cidr)
		if err != nil {
			panic(err)
		}
		out = append(out, n)
	}
	return out
}()

// permittedIP reports whether ip is a public unicast address.
func permittedIP(ip net.IP) bool {
	if ip == nil {
		return false
	}
	if v4 := ip.To4(); v4 != nil {
		ip = v4
	}
	if ip.IsUnspecified() || ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsInterfaceLocalMulticast() || ip.IsMulticast() {
		return false
	}
	for _, n := range privateNets {
		if n.Contains(ip) {
			return false
		}
	}
	return true
}

func defaultPermit(ip net.IP, port string) error {
	if port != "80" && port != "443" {
		return fmt.Errorf("port %q is not permitted: only 80 and 443 are dialled", port)
	}
	if !permittedIP(ip) {
		return fmt.Errorf("address %s is not permitted: not a public address", ip)
	}
	return nil
}

type lookupFunc func(ctx context.Context, host string) ([]net.IP, error)
type dialFunc func(ctx context.Context, network, addr string) (net.Conn, error)

func defaultLookup(ctx context.Context, host string) ([]net.IP, error) {
	addrs, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, err
	}
	out := make([]net.IP, 0, len(addrs))
	for _, a := range addrs {
		out = append(out, a.IP)
	}
	return out, nil
}

func guardedDialer(lookup lookupFunc, dial dialFunc) dialFunc {
	return guardedDialerWith(defaultPermit, lookup, dial)
}

func guardedDialerWith(permit func(net.IP, string) error, lookup lookupFunc, dial dialFunc) dialFunc {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, err
		}
		var ips []net.IP
		if ip := net.ParseIP(host); ip != nil {
			ips = []net.IP{ip}
		} else if ips, err = lookup(ctx, host); err != nil {
			return nil, err
		}
		if len(ips) == 0 {
			return nil, fmt.Errorf("%s has no address", host)
		}
		// Every address the name resolves to must be permitted: a name that
		// answers with a mix is refused whole.
		for _, ip := range ips {
			if err := permit(ip, port); err != nil {
				return nil, fmt.Errorf("dial %s: %w", addr, err)
			}
		}
		return dial(ctx, network, net.JoinHostPort(ips[0].String(), port))
	}
}

type guardConfig struct {
	permit func(ip net.IP, port string) error
	lookup lookupFunc
	dial   dialFunc
}

// newGuardedClient builds the client every outside fetch of the worker uses:
// no proxy from the environment, the dial guard, a cap on redirects and only
// http and https.
func newGuardedClient(cfg guardConfig) *http.Client {
	if cfg.permit == nil {
		cfg.permit = defaultPermit
	}
	if cfg.lookup == nil {
		cfg.lookup = defaultLookup
	}
	if cfg.dial == nil {
		d := &net.Dialer{Timeout: 20 * time.Second, KeepAlive: 30 * time.Second}
		cfg.dial = d.DialContext
	}
	return &http.Client{
		Transport: &http.Transport{
			Proxy:                 nil,
			DialContext:           guardedDialerWith(cfg.permit, cfg.lookup, cfg.dial),
			TLSHandshakeTimeout:   20 * time.Second,
			ResponseHeaderTimeout: 90 * time.Second,
			MaxIdleConns:          8,
			IdleConnTimeout:       30 * time.Second,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= maxRedirects {
				return fmt.Errorf("stopped after %d redirects", maxRedirects)
			}
			if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
				return errors.New("redirect to a scheme other than http or https")
			}
			return nil
		},
	}
}

// SafeClient is the guarded client with an overall timeout (0 for none).
func SafeClient(timeout time.Duration) *http.Client {
	c := newGuardedClient(guardConfig{})
	c.Timeout = timeout
	return c
}

// looksLikePlaylist reports whether the start of a download is a playlist or
// concat script: ffmpeg would open the addresses and files such a thing names.
func looksLikePlaylist(head []byte) bool {
	head = bytes.TrimPrefix(head, []byte("\xef\xbb\xbf"))
	head = bytes.TrimLeft(head, " \t\r\n")
	if len(head) > 64 {
		head = head[:64]
	}
	low := strings.ToLower(string(head))
	for _, p := range []string{"#extm3u", "ffconcat", "[playlist]", "<asx"} {
		if strings.HasPrefix(low, p) {
			return true
		}
	}
	return false
}

func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
