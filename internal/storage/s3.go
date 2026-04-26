package storage

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	neturl "net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jxwalker/modfetch/internal/config"
	"github.com/jxwalker/modfetch/internal/util"
)

const (
	defaultRegion       = "us-east-1"
	defaultS3Service    = "s3"
	defaultAccessKeyEnv = "AWS_ACCESS_KEY_ID"
	defaultSecretKeyEnv = "AWS_SECRET_ACCESS_KEY"
	defaultSessionEnv   = "AWS_SESSION_TOKEN"
)

type S3Client struct {
	endpoint     *neturl.URL
	region       string
	accessKey    string
	secretKey    string
	sessionToken string
	pathStyle    bool
	httpClient   *http.Client
}

func IsS3URI(s string) bool {
	u, err := neturl.Parse(strings.TrimSpace(s))
	return err == nil && strings.EqualFold(u.Scheme, "s3") && u.Host != "" && strings.TrimPrefix(u.Path, "/") != ""
}

func ParseS3URI(uri string) (bucket, key string, err error) {
	u, err := neturl.Parse(strings.TrimSpace(uri))
	if err != nil {
		return "", "", err
	}
	if !strings.EqualFold(u.Scheme, "s3") {
		return "", "", fmt.Errorf("not an s3 URI: %s", uri)
	}
	bucket = strings.TrimSpace(u.Host)
	key = strings.TrimPrefix(u.EscapedPath(), "/")
	if bucket == "" || key == "" {
		return "", "", fmt.Errorf("s3 URI must include bucket and key: %s", uri)
	}
	if decoded, err := neturl.PathUnescape(key); err == nil {
		key = decoded
	}
	return bucket, key, nil
}

func StagingPath(cfg *config.Config, uri, sourceURL string) (string, error) {
	bucket, key, err := ParseS3URI(uri)
	if err != nil {
		return "", err
	}
	root := ""
	if cfg != nil {
		root = strings.TrimSpace(cfg.General.DataRoot)
		if root == "" {
			root = strings.TrimSpace(cfg.General.DownloadRoot)
		}
	}
	if root == "" {
		return "", errors.New("general.data_root or general.download_root is required for s3 staging")
	}
	h := sha256.Sum256([]byte(sourceURL + "|" + uri))
	base := util.SafeFileName(filepath.Base(key))
	if base == "" || base == "." {
		base = "object"
	}
	return filepath.Join(root, "s3-staging", util.SafeFileName(bucket), hex.EncodeToString(h[:])[:12]+"-"+base), nil
}

func NewS3ClientFromConfig(cfg *config.Config) (*S3Client, error) {
	if cfg == nil {
		return nil, errors.New("config is required")
	}
	s3 := cfg.Storage.S3
	endpoint := strings.TrimSpace(s3.Endpoint)
	if endpoint == "" {
		endpoint = strings.TrimSpace(os.Getenv("MODFETCH_S3_ENDPOINT"))
	}
	if endpoint == "" {
		return nil, errors.New("storage.s3.endpoint or MODFETCH_S3_ENDPOINT is required")
	}
	if !strings.Contains(endpoint, "://") {
		scheme := "https"
		if s3.UseHTTP {
			scheme = "http"
		}
		endpoint = scheme + "://" + endpoint
	}
	u, err := neturl.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("storage.s3.endpoint: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("storage.s3.endpoint must use http or https")
	}
	region := firstNonEmpty(s3.Region, os.Getenv("AWS_REGION"), os.Getenv("AWS_DEFAULT_REGION"), defaultRegion)
	accessEnv := firstNonEmpty(s3.AccessKeyEnv, defaultAccessKeyEnv)
	secretEnv := firstNonEmpty(s3.SecretKeyEnv, defaultSecretKeyEnv)
	sessionEnv := firstNonEmpty(s3.SessionTokenEnv, defaultSessionEnv)
	accessKey := strings.TrimSpace(os.Getenv(accessEnv))
	secretKey := strings.TrimSpace(os.Getenv(secretEnv))
	if accessKey == "" || secretKey == "" {
		return nil, fmt.Errorf("missing S3 credentials: set %s and %s", accessEnv, secretEnv)
	}
	return &S3Client{
		endpoint:     u,
		region:       region,
		accessKey:    accessKey,
		secretKey:    secretKey,
		sessionToken: strings.TrimSpace(os.Getenv(sessionEnv)),
		pathStyle:    s3.PathStyle,
		httpClient:   newS3HTTPClient(cfg),
	}, nil
}

func (c *S3Client) PutFile(ctx context.Context, uri, path, contentType string) error {
	bucket, key, err := ParseS3URI(uri)
	if err != nil {
		return err
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	stat, err := f.Stat()
	if err != nil {
		return err
	}
	hasher := sha256.New()
	if _, err := io.Copy(hasher, f); err != nil {
		return err
	}
	payloadHash := hex.EncodeToString(hasher.Sum(nil))
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return err
	}
	reqURL := c.objectURL(bucket, key)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, reqURL, f)
	if err != nil {
		return err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	req.ContentLength = stat.Size()
	c.sign(req, payloadHash, time.Now().UTC())
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	return fmt.Errorf("s3 put %s failed: %s %s", uri, resp.Status, strings.TrimSpace(string(b)))
}

func (c *S3Client) DeleteObject(ctx context.Context, uri string) error {
	bucket, key, err := ParseS3URI(uri)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.objectURL(bucket, key), nil)
	if err != nil {
		return err
	}
	emptyHash := sha256.Sum256(nil)
	c.sign(req, hex.EncodeToString(emptyHash[:]), time.Now().UTC())
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	return fmt.Errorf("s3 delete %s failed: %s %s", uri, resp.Status, strings.TrimSpace(string(b)))
}

func (c *S3Client) objectURL(bucket, key string) string {
	u := *c.endpoint
	u.RawQuery = ""
	u.Fragment = ""
	escapedKey := escapeS3Key(key)
	if c.pathStyle || isLocalHost(u.Hostname()) {
		u.Path = joinURLPath(u.Path, bucket, escapedKey)
		return u.String()
	}
	u.Host = bucket + "." + u.Host
	u.Path = joinURLPath(u.Path, escapedKey)
	return u.String()
}

func (c *S3Client) sign(req *http.Request, payloadHash string, now time.Time) {
	amzDate := now.Format("20060102T150405Z")
	shortDate := now.Format("20060102")
	req.Header.Set("X-Amz-Date", amzDate)
	req.Header.Set("X-Amz-Content-Sha256", payloadHash)
	if c.sessionToken != "" {
		req.Header.Set("X-Amz-Security-Token", c.sessionToken)
	}
	headers, signedHeaders := canonicalHeaders(req)
	canonical := strings.Join([]string{
		req.Method,
		canonicalURI(req.URL),
		req.URL.RawQuery,
		headers,
		signedHeaders,
		payloadHash,
	}, "\n")
	scope := shortDate + "/" + c.region + "/" + defaultS3Service + "/aws4_request"
	canonicalHash := sha256.Sum256([]byte(canonical))
	stringToSign := strings.Join([]string{
		"AWS4-HMAC-SHA256",
		amzDate,
		scope,
		hex.EncodeToString(canonicalHash[:]),
	}, "\n")
	signingKey := deriveSigningKey(c.secretKey, shortDate, c.region, defaultS3Service)
	signature := hex.EncodeToString(hmacSHA256(signingKey, []byte(stringToSign)))
	req.Header.Set("Authorization", fmt.Sprintf("AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s", c.accessKey, scope, signedHeaders, signature))
}

func canonicalHeaders(req *http.Request) (string, string) {
	values := map[string]string{
		"host": req.URL.Host,
	}
	for k, vals := range req.Header {
		lk := strings.ToLower(strings.TrimSpace(k))
		if lk == "authorization" {
			continue
		}
		joined := strings.Join(vals, ",")
		values[lk] = strings.Join(strings.Fields(joined), " ")
	}
	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		b.WriteString(k)
		b.WriteByte(':')
		b.WriteString(values[k])
		b.WriteByte('\n')
	}
	return b.String(), strings.Join(keys, ";")
}

func canonicalURI(u *neturl.URL) string {
	if u == nil || u.EscapedPath() == "" {
		return "/"
	}
	return u.EscapedPath()
}

func deriveSigningKey(secret, date, region, service string) []byte {
	kDate := hmacSHA256([]byte("AWS4"+secret), []byte(date))
	kRegion := hmacSHA256(kDate, []byte(region))
	kService := hmacSHA256(kRegion, []byte(service))
	return hmacSHA256(kService, []byte("aws4_request"))
}

func hmacSHA256(key, data []byte) []byte {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write(data)
	return mac.Sum(nil)
}

func escapeS3Key(key string) string {
	parts := strings.Split(key, "/")
	for i, part := range parts {
		parts[i] = neturl.PathEscape(part)
	}
	return strings.Join(parts, "/")
}

func joinURLPath(parts ...string) string {
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.Trim(p, "/")
		if p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return "/"
	}
	return "/" + strings.Join(out, "/")
}

func isLocalHost(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func newS3HTTPClient(cfg *config.Config) *http.Client {
	timeout := 6 * time.Hour
	if cfg != nil && cfg.Network.TimeoutSeconds > 0 {
		timeout = time.Duration(cfg.Network.TimeoutSeconds) * time.Second
	}
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			Proxy:                 http.ProxyFromEnvironment,
			DialContext:           (&net.Dialer{Timeout: 30 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
			TLSHandshakeTimeout:   30 * time.Second,
			ResponseHeaderTimeout: 30 * time.Second,
			IdleConnTimeout:       90 * time.Second,
		},
	}
}
