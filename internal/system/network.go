package system

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/jxwalker/modfetch/internal/errors"
)

// CheckConnectivity performs basic network connectivity checks
func CheckConnectivity(ctx context.Context) error {
	// 1. DNS resolution check
	if err := checkDNS(ctx); err != nil {
		return err
	}

	// 2. HTTP connectivity check
	if err := checkHTTP(ctx); err != nil {
		return err
	}

	return nil
}

// checkDNS verifies DNS resolution is working
func checkDNS(ctx context.Context) error {
	resolver := &net.Resolver{}
	hosts := []string{"google.com", "cloudflare.com"}

	for _, host := range hosts {
		_, err := resolver.LookupHost(ctx, host)
		if err == nil {
			return nil // At least one succeeded
		}
	}

	return errors.NewFriendlyError(
		"DNS resolution failed - cannot resolve hostnames",
		"Check your network connection and DNS settings:\n"+
			"1. Verify internet connection: ping 8.8.8.8\n"+
			"2. Check DNS servers: cat /etc/resolv.conf\n"+
			"3. Test DNS: nslookup google.com",
	)
}

// checkHTTP verifies HTTPS connectivity
func checkHTTP(ctx context.Context) error {
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	// Try a simple HTTP request
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://www.google.com", nil)
	resp, err := client.Do(req)
	if err != nil {
		// Check for SSL/TLS certificate errors
		if strings.Contains(err.Error(), "certificate") || strings.Contains(err.Error(), "x509") {
			return errors.NewFriendlyError(
				"SSL/TLS certificate verification failed",
				"You may be behind a corporate proxy or firewall:\n"+
					"1. Set CA bundle: export REQUESTS_CA_BUNDLE=/path/to/cert.pem\n"+
					"2. Or disable TLS verification (insecure) in config: tls_verify: false\n"+
					"3. Check proxy settings: echo $HTTP_PROXY $HTTPS_PROXY",
			).WithDetails(err)
		}

		return errors.NetworkError(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	return fmt.Errorf("HTTP connectivity check failed: status %d", resp.StatusCode)
}

// CheckHostReachable checks if a specific host is reachable
func CheckHostReachable(ctx context.Context, host string) error {
	// Try DNS resolution first
	resolver := &net.Resolver{}
	_, err := resolver.LookupHost(ctx, host)
	if err != nil {
		return errors.NewFriendlyError(
			fmt.Sprintf("Cannot resolve host: %s", host),
			"Check that the hostname is correct and your DNS is working",
		).WithDetails(err)
	}

	// Try TCP connection
	dialer := &net.Dialer{Timeout: 5 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(host, "443"))
	if err != nil {
		return errors.NewFriendlyError(
			fmt.Sprintf("Cannot connect to host: %s", host),
			fmt.Sprintf("Host is unreachable:\n"+
				"1. Check internet connection\n"+
				"2. Verify host is not blocked by firewall\n"+
				"3. Try: curl -I https://%s", host),
		).WithDetails(err)
	}
	conn.Close()

	return nil
}

// GetPublicIP returns the public IP address (useful for debugging proxy/NAT issues)
func GetPublicIP(ctx context.Context) (string, error) {
	client := &http.Client{Timeout: 3 * time.Second}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.ipify.org", nil)

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(body), nil
}

// DetectProxySettings returns proxy configuration from environment
func DetectProxySettings() map[string]string {
	proxies := make(map[string]string)

	envVars := []string{"HTTP_PROXY", "HTTPS_PROXY", "NO_PROXY", "http_proxy", "https_proxy", "no_proxy"}

	for _, envVar := range envVars {
		if val := os.Getenv(envVar); val != "" {
			proxies[envVar] = val
		}
	}

	// Also try to detect via ProxyFromEnvironment for HTTP/HTTPS
	dummyReq, _ := http.NewRequest("GET", "http://example.com", nil)
	if proxyURL, _ := http.ProxyFromEnvironment(dummyReq); proxyURL != nil {
		if _, exists := proxies["HTTP_PROXY"]; !exists {
			proxies["HTTP_PROXY"] = proxyURL.String()
		}
	}

	dummyReqHTTPS, _ := http.NewRequest("GET", "https://example.com", nil)
	if proxyURL, _ := http.ProxyFromEnvironment(dummyReqHTTPS); proxyURL != nil {
		if _, exists := proxies["HTTPS_PROXY"]; !exists {
			proxies["HTTPS_PROXY"] = proxyURL.String()
		}
	}

	return proxies
}
