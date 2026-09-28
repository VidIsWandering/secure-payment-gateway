package service

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"syscall"
	"time"
)

// ValidateWebhookURL checks that a merchant webhook URL is safe to call (SSRF
// protection). It requires https and rejects hostnames that resolve to
// private, loopback, link-local or otherwise reserved addresses.
//
// allowLocal is for development only: it additionally accepts http:// and the
// local targets localhost, 127.0.0.1, ::1 and host.docker.internal.
func ValidateWebhookURL(rawURL string, allowLocal bool) error {
	if rawURL == "" {
		return nil // empty = no webhook, which is valid
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	if parsed.Scheme != "https" && parsed.Scheme != "http" {
		return fmt.Errorf("webhook URL must use https (got %q)", parsed.Scheme)
	}

	hostname := parsed.Hostname()
	if hostname == "" {
		return fmt.Errorf("webhook URL has no host")
	}
	local := allowLocal && isLocalhostDev(hostname)
	if parsed.Scheme == "http" && !local {
		return fmt.Errorf("webhook URL must use https for non-local targets")
	}
	if local {
		return nil
	}

	ips, err := net.LookupHost(hostname)
	if err != nil {
		return fmt.Errorf("cannot resolve webhook hostname %q: %w", hostname, err)
	}
	for _, ipStr := range ips {
		if ip := net.ParseIP(ipStr); ip == nil || isPrivateIP(ip) {
			return fmt.Errorf("webhook URL resolves to private/reserved IP %s (SSRF blocked)", ipStr)
		}
	}
	return nil
}

// NewWebhookHTTPClient returns the HTTP client used to deliver webhooks.
// Unless allowLocal is set, it refuses to connect to non-public addresses.
// The check runs on the resolved IP at connect time, so DNS rebinding and
// redirects cannot bypass ValidateWebhookURL. Redirects are not followed.
func NewWebhookHTTPClient(timeout time.Duration, allowLocal bool) *http.Client {
	dialer := &net.Dialer{Timeout: 5 * time.Second}
	if !allowLocal {
		dialer.Control = publicAddressOnly
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DialContext = dialer.DialContext
	transport.Proxy = nil // a proxy would connect on our behalf and bypass the check

	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse // a 3xx counts as a failed delivery
		},
	}
}

// publicAddressOnly is a net.Dialer Control hook that rejects connections to
// non-public IP addresses.
func publicAddressOnly(_, address string, _ syscall.RawConn) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return err
	}
	if ip := net.ParseIP(host); ip == nil || isPrivateIP(ip) {
		return fmt.Errorf("connection to non-public address %s blocked (SSRF)", host)
	}
	return nil
}

// isLocalhostDev returns true for development-only hostnames.
func isLocalhostDev(hostname string) bool {
	return hostname == "localhost" || hostname == "127.0.0.1" || hostname == "::1" || hostname == "host.docker.internal"
}

// reservedNetworks are non-public ranges not covered by the net.IP helpers.
var reservedNetworks = mustParseCIDRs(
	"0.0.0.0/8",     // "this network" — 0.0.0.0 reaches localhost on Linux
	"100.64.0.0/10", // carrier-grade NAT, often internal in cloud VPCs
	"192.0.0.0/24",  // IETF protocol assignments
	"198.18.0.0/15", // benchmarking
	"240.0.0.0/4",   // reserved
)

// isPrivateIP reports whether ip is not a public unicast address: loopback,
// private (RFC 1918 / RFC 4193), link-local (incl. cloud metadata
// 169.254.169.254), unspecified, multicast or reserved.
func isPrivateIP(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() ||
		ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsInterfaceLocalMulticast() || ip.IsMulticast() {
		return true
	}
	for _, n := range reservedNetworks {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

func mustParseCIDRs(cidrs ...string) []*net.IPNet {
	nets := make([]*net.IPNet, 0, len(cidrs))
	for _, c := range cidrs {
		_, n, err := net.ParseCIDR(c)
		if err != nil {
			panic(err)
		}
		nets = append(nets, n)
	}
	return nets
}
