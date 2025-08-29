package downloader

import (
	"crypto/tls"
	"net"
	"net/http"
	"time"

	"modfetch/internal/config"
)

func newHTTPClient(cfg *config.Config) *http.Client {
	timeout := time.Duration(cfg.Network.TimeoutSeconds) * time.Second
	if timeout <= 0 { timeout = 60 * time.Second }
	tr := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout:   10 * time.Second,
		IdleConnTimeout:       90 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   10,
		TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
		},
	}
	return &http.Client{Transport: tr, Timeout: timeout}
}

