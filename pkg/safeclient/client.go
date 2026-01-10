// Package safeclient provides an HTTP client with SSRF protection.
package safeclient

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"syscall"
	"time"
)

var (
	// ErrForbiddenIP is returned when an IP address is in a forbidden range.
	ErrForbiddenIP = errors.New("connection to private/internal IP addresses is forbidden")
)

// forbiddenIPRanges contains all IP ranges that should be blocked to prevent SSRF.
var forbiddenIPRanges = []net.IPNet{
	// IPv4 Private Addresses (RFC 1918)
	{IP: net.IPv4(10, 0, 0, 0), Mask: net.CIDRMask(8, 32)},     // 10.0.0.0/8
	{IP: net.IPv4(172, 16, 0, 0), Mask: net.CIDRMask(12, 32)},  // 172.16.0.0/12
	{IP: net.IPv4(192, 168, 0, 0), Mask: net.CIDRMask(16, 32)}, // 192.168.0.0/16

	// IPv4 Loopback (RFC 1122)
	{IP: net.IPv4(127, 0, 0, 0), Mask: net.CIDRMask(8, 32)}, // 127.0.0.0/8

	// IPv4 Link-Local (RFC 3927)
	{IP: net.IPv4(169, 254, 0, 0), Mask: net.CIDRMask(16, 32)}, // 169.254.0.0/16

	// IPv4 Multicast (RFC 5771)
	{IP: net.IPv4(224, 0, 0, 0), Mask: net.CIDRMask(4, 32)}, // 224.0.0.0/4

	// IPv4 Broadcast
	{IP: net.IPv4(255, 255, 255, 255), Mask: net.CIDRMask(32, 32)}, // 255.255.255.255/32

	// IPv4 Carrier-Grade NAT (RFC 6598)
	{IP: net.IPv4(100, 64, 0, 0), Mask: net.CIDRMask(10, 32)}, // 100.64.0.0/10

	// IPv4 "This" Network (RFC 791)
	{IP: net.IPv4(0, 0, 0, 0), Mask: net.CIDRMask(8, 32)}, // 0.0.0.0/8

	// IPv4 Reserved Documentation (RFC 5737)
	{IP: net.IPv4(192, 0, 2, 0), Mask: net.CIDRMask(24, 32)},    // 192.0.2.0/24 (TEST-NET-1)
	{IP: net.IPv4(198, 51, 100, 0), Mask: net.CIDRMask(24, 32)}, // 198.51.100.0/24 (TEST-NET-2)
	{IP: net.IPv4(203, 0, 113, 0), Mask: net.CIDRMask(24, 32)},  // 203.0.113.0/24 (TEST-NET-3)

	// Cloud Provider Metadata Endpoints
	{IP: net.IPv4(169, 254, 169, 254), Mask: net.CIDRMask(32, 32)}, // AWS/GCP/Azure metadata
}

// IPv6 forbidden ranges
var forbiddenIPv6Ranges = []net.IPNet{
	// IPv6 Loopback
	{IP: net.ParseIP("::1"), Mask: net.CIDRMask(128, 128)},

	// IPv6 Unspecified
	{IP: net.ParseIP("::"), Mask: net.CIDRMask(128, 128)},

	// IPv6 Private/Unique Local (RFC 4193)
	{IP: net.ParseIP("fc00::"), Mask: net.CIDRMask(7, 128)},

	// IPv6 Link-Local (RFC 4291)
	{IP: net.ParseIP("fe80::"), Mask: net.CIDRMask(10, 128)},

	// IPv6 Site-Local (deprecated but still blocked)
	{IP: net.ParseIP("fec0::"), Mask: net.CIDRMask(10, 128)},

	// IPv6 Multicast
	{IP: net.ParseIP("ff00::"), Mask: net.CIDRMask(8, 128)},

	// IPv4-mapped IPv6 addresses (::ffff:0:0/96)
	// We check these separately in isForbiddenIP

	// IPv6 Documentation (RFC 3849)
	{IP: net.ParseIP("2001:db8::"), Mask: net.CIDRMask(32, 128)},
}

// isForbiddenIP checks if an IP address is in a forbidden range.
// This prevents SSRF attacks by blocking connections to internal networks.
func isForbiddenIP(ip net.IP) bool {
	if ip == nil {
		return true
	}

	// Check if it's an IPv4-mapped IPv6 address and extract the IPv4 part
	if ipv4 := ip.To4(); ipv4 != nil {
		ip = ipv4
	}

	// Check IPv4 ranges
	if ip.To4() != nil {
		for _, network := range forbiddenIPRanges {
			if network.Contains(ip) {
				return true
			}
		}
		return false
	}

	// Check IPv6 ranges
	for _, network := range forbiddenIPv6Ranges {
		if network.Contains(ip) {
			return true
		}
	}

	return false
}

// safeDialer returns a net.Dialer with a Control function that validates
// the resolved IP address before allowing the connection.
// This is critical for preventing DNS rebinding attacks.
func safeDialer() *net.Dialer {
	return &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
		Control: func(network, address string, c syscall.RawConn) error {
			// Parse the address to extract the IP
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return fmt.Errorf("failed to parse address: %w", err)
			}

			// Parse the IP
			ip := net.ParseIP(host)
			if ip == nil {
				return fmt.Errorf("invalid IP address: %s", host)
			}

			// Check if the IP is forbidden
			if isForbiddenIP(ip) {
				return ErrForbiddenIP
			}

			return nil
		},
	}
}

// NewSafeHTTPClient creates an HTTP client with SSRF protection.
// The client validates IP addresses at connection time to prevent
// DNS rebinding attacks.
func NewSafeHTTPClient() *http.Client {
	transport := &http.Transport{
		DialContext:           safeDialer().DialContext,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   10,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ForceAttemptHTTP2:     true,
	}

	return &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// Limit redirects to prevent redirect-based attacks
			if len(via) >= 10 {
				return errors.New("stopped after 10 redirects")
			}
			return nil
		},
	}
}

// NewSafeHTTPClientWithTimeout creates an HTTP client with SSRF protection
// and a custom timeout.
func NewSafeHTTPClientWithTimeout(timeout time.Duration) *http.Client {
	client := NewSafeHTTPClient()
	client.Timeout = timeout
	return client
}

// SafeGet performs a GET request with SSRF protection.
func SafeGet(ctx context.Context, url string) (*http.Response, error) {
	client := NewSafeHTTPClient()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	return client.Do(req)
}

// SafePost performs a POST request with SSRF protection.
func SafePost(ctx context.Context, url, contentType string, body []byte) (*http.Response, error) {
	client := NewSafeHTTPClient()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", contentType)

	return client.Do(req)
}

// IsForbiddenIP is exported for testing purposes.
func IsForbiddenIP(ip net.IP) bool {
	return isForbiddenIP(ip)
}
