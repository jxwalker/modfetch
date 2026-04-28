package metadata

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func routedHTTPClient(t *testing.T, host string, handler http.Handler) *http.Client {
	t.Helper()

	server := httptest.NewTLSServer(handler)
	t.Cleanup(server.Close)

	serverURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse test server URL: %v", err)
	}

	dialer := &net.Dialer{Timeout: 5 * time.Second}
	transport := &http.Transport{
		Proxy: nil,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			hostOnly, _, err := net.SplitHostPort(addr)
			if err != nil {
				hostOnly = addr
			}
			if strings.EqualFold(hostOnly, host) {
				return dialer.DialContext(ctx, network, serverURL.Host)
			}
			return nil, fmt.Errorf("unexpected test network dial to %s", addr)
		},
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // Test server certificate is for localhost.
	}
	t.Cleanup(transport.CloseIdleConnections)

	return &http.Client{Transport: transport}
}
