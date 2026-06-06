package sources

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var sourceFetchAllowPrivateNetworkForTests bool

func validateSourceFetchURL(raw string) error {
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
	if sourceFetchHostnameBlocked(host) && !sourceFetchAllowPrivateNetworkForTests {
		return fmt.Errorf("source URL host is not allowed")
	}
	if ip := net.ParseIP(host); ip != nil && sourceFetchIPBlocked(ip) && !sourceFetchAllowPrivateNetworkForTests {
		return fmt.Errorf("source URL host is not allowed")
	}
	return nil
}

func sourceFetchHTTPClient(timeout time.Duration) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	dialer := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
	resolver := net.DefaultResolver
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, fmt.Errorf("source fetch address: %w", err)
		}
		if err := validateSourceFetchHost(ctx, resolver, host); err != nil {
			return nil, err
		}
		return dialer.DialContext(ctx, network, net.JoinHostPort(host, port))
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
			return validateSourceFetchURL(req.URL.String())
		},
	}
}

func validateSourceFetchHost(ctx context.Context, resolver *net.Resolver, host string) error {
	host = strings.TrimSpace(strings.Trim(host, "[]"))
	if host == "" {
		return fmt.Errorf("source fetch host is required")
	}
	if sourceFetchHostnameBlocked(host) && !sourceFetchAllowPrivateNetworkForTests {
		return fmt.Errorf("source fetch host resolves to forbidden address")
	}
	if ip := net.ParseIP(host); ip != nil {
		if sourceFetchIPBlocked(ip) && !sourceFetchAllowPrivateNetworkForTests {
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
		if sourceFetchIPBlocked(addr.IP) && !sourceFetchAllowPrivateNetworkForTests {
			return fmt.Errorf("source fetch host resolves to forbidden address")
		}
	}
	return nil
}

func sourceFetchIPBlocked(ip net.IP) bool {
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

func sourceFetchHostnameBlocked(host string) bool {
	host = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(host)), ".")
	return host == "localhost" || strings.HasSuffix(host, ".localhost") || host == "metadata.google.internal"
}
