package sourcefetch

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var allowPrivateNetworkForTests bool

func SetAllowPrivateNetworkForTests(allow bool) bool {
	previous := allowPrivateNetworkForTests
	allowPrivateNetworkForTests = allow
	return previous
}

func Client(timeout time.Duration) *http.Client {
	dialer := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
	return clientWith(timeout, net.DefaultResolver.LookupIPAddr, dialer.DialContext)
}

// clientWith resolves each dial target once, refuses the dial when any
// answer is forbidden, and dials the checked addresses — never the hostname,
// whose second resolution could rebind to a private address
// (research-fetch-url-has-no-address-guard-2026-10-10.md, D2). TLS still
// verifies the URL's hostname: the transport takes SNI from the request.
func clientWith(timeout time.Duration, lookup func(context.Context, string) ([]net.IPAddr, error), dial func(context.Context, string, string) (net.Conn, error)) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, fmt.Errorf("source fetch address: %w", err)
		}
		host = strings.Trim(host, "[]")
		if hostnameBlocked(host) && !allowPrivateNetworkForTests {
			return nil, fmt.Errorf("source fetch host resolves to forbidden address")
		}
		var addrs []net.IPAddr
		if ip := net.ParseIP(host); ip != nil {
			addrs = []net.IPAddr{{IP: ip}}
		} else if addrs, err = lookup(ctx, host); err != nil {
			return nil, fmt.Errorf("source fetch resolve host: %w", err)
		}
		if len(addrs) == 0 {
			return nil, fmt.Errorf("source fetch resolve host: no addresses")
		}
		for _, addr := range addrs {
			if AddressBlocked(addr.IP) && !allowPrivateNetworkForTests {
				return nil, fmt.Errorf("source fetch host resolves to forbidden address")
			}
		}
		var lastErr error
		for _, addr := range addrs {
			conn, err := dial(ctx, network, net.JoinHostPort(addr.IP.String(), port))
			if err == nil {
				return conn, nil
			}
			lastErr = err
		}
		return nil, lastErr
	}
	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("source fetch redirect limit exceeded")
			}
			if req == nil || req.URL == nil {
				return fmt.Errorf("source fetch redirect target is invalid")
			}
			return ValidateURL(req.URL.String())
		},
	}
}

func ValidateURL(raw string) error {
	if len(strings.TrimSpace(raw)) > 4096 {
		return fmt.Errorf("source URL is too long")
	}
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("source URL must be an absolute http or https URL")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("source URL scheme must be http or https")
	}
	if parsed.User != nil {
		return fmt.Errorf("source URL must not include user info")
	}
	host := parsed.Hostname()
	if host == "" {
		return fmt.Errorf("source URL host is required")
	}
	if hostnameBlocked(host) && !allowPrivateNetworkForTests {
		return fmt.Errorf("source URL host is not allowed")
	}
	if ip := net.ParseIP(host); ip != nil && ipBlocked(ip) && !allowPrivateNetworkForTests {
		return fmt.Errorf("source URL host is not allowed")
	}
	return nil
}

func ValidateHost(ctx context.Context, resolver *net.Resolver, host string) error {
	host = strings.TrimSpace(strings.Trim(host, "[]"))
	if host == "" {
		return fmt.Errorf("source fetch host is required")
	}
	if hostnameBlocked(host) && !allowPrivateNetworkForTests {
		return fmt.Errorf("source fetch host resolves to forbidden address")
	}
	if ip := net.ParseIP(host); ip != nil {
		if ipBlocked(ip) && !allowPrivateNetworkForTests {
			return fmt.Errorf("source fetch host resolves to forbidden address")
		}
		return nil
	}
	addrs, err := resolver.LookupIPAddr(ctx, host)
	if err != nil {
		return fmt.Errorf("source fetch resolve host: %w", err)
	}
	if len(addrs) == 0 {
		return fmt.Errorf("source fetch resolve host: no addresses")
	}
	for _, addr := range addrs {
		if ipBlocked(addr.IP) && !allowPrivateNetworkForTests {
			return fmt.Errorf("source fetch host resolves to forbidden address")
		}
	}
	return nil
}

// AddressBlocked reports whether ip is loopback, private, link-local
// (including cloud metadata), CGNAT, multicast, unspecified or otherwise not
// a public unicast address. It ignores the test override: callers that dial
// a checked address (the capsule egress proxy) inject their own dialer in
// tests instead.
func AddressBlocked(ip net.IP) bool {
	if v6 := ip.To16(); v6 != nil && ip.To4() == nil && v6[0] == 0x00 && v6[1] == 0x64 && v6[2] == 0xff && v6[3] == 0x9b {
		// NAT64 well-known prefix 64:ff9b::/96 embeds an IPv4 address.
		return ipBlocked(net.IP(v6[12:16]))
	}
	return ipBlocked(ip)
}

func ipBlocked(ip net.IP) bool {
	if ip == nil {
		return true
	}
	ip = ip.To16()
	if ip == nil {
		return true
	}
	if v4 := ip.To4(); v4 != nil {
		ip = v4
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() {
		return true
	}
	if v4 := ip.To4(); v4 != nil {
		if v4[0] == 100 && v4[1] >= 64 && v4[1] <= 127 {
			return true
		}
		if v4[0] == 0 || v4[0] >= 224 {
			return true
		}
	}
	return false
}

func hostnameBlocked(host string) bool {
	host = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(host)), ".")
	return host == "localhost" || strings.HasSuffix(host, ".localhost") || host == "metadata.google.internal"
}
