package service

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

// ValidateWebhookURL checks that a webhook URL is safe to call.
// It blocks:
//   - Non-HTTPS URLs (except localhost for development)
//   - Private/internal IPs (10.x, 172.16-31.x, 192.168.x, 127.x)
//   - Link-local addresses (169.254.x)
//   - IPv6 loopback (::1) and link-local (fe80::)
//   - Hostnames that resolve to private IPs
func ValidateWebhookURL(rawURL string) error {
	if rawURL == "" {
		return nil // empty = no webhook, which is valid
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	// Must be http or https
	if parsed.Scheme != "https" && parsed.Scheme != "http" {
		return fmt.Errorf("webhook URL must use https (got %q)", parsed.Scheme)
	}

	// In production, require HTTPS (allow HTTP only for localhost/dev)
	hostname := parsed.Hostname()
	if parsed.Scheme == "http" && !isLocalhostDev(hostname) {
		return fmt.Errorf("webhook URL must use https for non-localhost targets")
	}

	// Skip DNS resolution check for localhost (dev-friendly)
	if isLocalhostDev(hostname) {
		return nil
	}

	// Resolve hostname and check for private IPs (SSRF prevention)
	ips, err := net.LookupHost(hostname)
	if err != nil {
		// DNS resolution failed — could be an invalid hostname
		return fmt.Errorf("cannot resolve webhook hostname %q: %w", hostname, err)
	}

	for _, ipStr := range ips {
		ip := net.ParseIP(ipStr)
		if ip == nil {
			continue
		}
		if isPrivateIP(ip) {
			return fmt.Errorf("webhook URL resolves to private/reserved IP %s (SSRF blocked)", ipStr)
		}
	}

	return nil
}

// isLocalhostDev returns true for development-only hostnames.
func isLocalhostDev(hostname string) bool {
	return hostname == "localhost" || hostname == "127.0.0.1" || hostname == "::1" || hostname == "host.docker.internal"
}

// isPrivateIP checks if an IP is in a private, loopback, or link-local range.
func isPrivateIP(ip net.IP) bool {
	// Loopback
	if ip.IsLoopback() {
		return true
	}
	// Link-local
	if ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return true
	}

	// Private ranges
	privateRanges := []struct {
		network string
	}{
		{"10.0.0.0/8"},
		{"172.16.0.0/12"},
		{"192.168.0.0/16"},
		{"169.254.0.0/16"}, // AWS metadata, etc.
		{"fc00::/7"},       // IPv6 unique-local
	}

	for _, r := range privateRanges {
		_, cidr, err := net.ParseCIDR(r.network)
		if err != nil {
			continue
		}
		if cidr.Contains(ip) {
			return true
		}
	}

	// Block cloud metadata endpoints
	metadataIPs := []string{
		"169.254.169.254", // AWS/GCP/Azure metadata
		"169.254.170.2",   // AWS ECS credentials
	}
	ipStr := ip.String()
	for _, meta := range metadataIPs {
		if strings.EqualFold(ipStr, meta) {
			return true
		}
	}

	return false
}
