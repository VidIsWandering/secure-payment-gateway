package service

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsPrivateIP(t *testing.T) {
	blocked := []string{
		"127.0.0.1", "::1", // loopback
		"10.1.2.3", "172.16.0.1", "172.31.255.255", "192.168.1.1", "fd00::1", // private
		"169.254.169.254", "169.254.170.2", "fe80::1", // link-local incl. cloud metadata
		"0.0.0.0", "0.1.2.3", "::", // unspecified / "this network"
		"100.64.0.1",       // CGNAT
		"224.0.0.1",        // multicast
		"::ffff:127.0.0.1", // IPv4-mapped loopback
		"240.0.0.1",        // reserved
	}
	for _, s := range blocked {
		assert.True(t, isPrivateIP(net.ParseIP(s)), "%s should be blocked", s)
	}

	public := []string{"8.8.8.8", "1.1.1.1", "172.32.0.1", "2606:4700:4700::1111"}
	for _, s := range public {
		assert.False(t, isPrivateIP(net.ParseIP(s)), "%s should be allowed", s)
	}
}

func TestValidateWebhookURL(t *testing.T) {
	tests := []struct {
		url        string
		allowLocal bool
		wantErr    bool
	}{
		{"", false, false},
		{"https://8.8.8.8/hook", false, false},
		{"ftp://8.8.8.8/hook", false, true},
		{"http://8.8.8.8/hook", false, true}, // https required for public targets
		{"https://10.0.0.5/hook", false, true},
		{"https://169.254.169.254/latest/meta-data", false, true},
		{"https://[::1]/hook", false, true},
		{"https:///no-host", false, true},
		// Local targets are only accepted in development mode.
		{"http://localhost:9000/webhook", false, true},
		{"https://127.0.0.1/hook", false, true},
		{"http://host.docker.internal:9000/webhook", false, true},
		{"http://localhost:9000/webhook", true, false},
		{"http://host.docker.internal:9000/webhook", true, false},
		{"http://10.0.0.5/hook", true, true}, // allowLocal is not a general private-IP bypass
	}
	for _, tt := range tests {
		err := ValidateWebhookURL(tt.url, tt.allowLocal)
		if tt.wantErr {
			assert.Error(t, err, "url=%q allowLocal=%v", tt.url, tt.allowLocal)
		} else {
			assert.NoError(t, err, "url=%q allowLocal=%v", tt.url, tt.allowLocal)
		}
	}
}

func TestNewWebhookHTTPClient_BlocksPrivateTargetsAtConnectTime(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close() // listens on 127.0.0.1

	// Production client: the connection itself is refused, even though the
	// URL never went through ValidateWebhookURL (e.g. after DNS rebinding).
	resp, err := NewWebhookHTTPClient(2*time.Second, false).Get(srv.URL)
	if resp != nil {
		_ = resp.Body.Close()
	}
	require.Error(t, err)
	assert.Contains(t, err.Error(), "SSRF")

	// Development client may reach local receivers.
	resp, err = NewWebhookHTTPClient(2*time.Second, true).Get(srv.URL)
	require.NoError(t, err)
	_ = resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestNewWebhookHTTPClient_DoesNotFollowRedirects(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://169.254.169.254/latest/meta-data", http.StatusFound)
	}))
	defer srv.Close()

	resp, err := NewWebhookHTTPClient(2*time.Second, true).Get(srv.URL)
	require.NoError(t, err)
	_ = resp.Body.Close()
	assert.Equal(t, http.StatusFound, resp.StatusCode, "a redirect is returned as-is and counts as a failed delivery")
}
