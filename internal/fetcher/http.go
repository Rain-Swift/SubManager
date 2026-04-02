package fetcher

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	UAMihomo          = "clash.meta/1.18.0"
	UAClashVerge      = "clash-verge/v1.3.8"
	UAClashForWindows = "ClashforWindows/0.20.39"
)

type Request struct {
	URL             string
	Headers         map[string]string
	UserAgent       string
	Timeout         time.Duration
	RetryAttempts   int
	RetryBackoff    time.Duration
	CacheKey        string
	CacheTTL        time.Duration
	IfNoneMatch     string
	IfModifiedSince string
	AllowStale      bool
	RateLimitKey    string
	MinInterval     time.Duration
}

type Artifact struct {
	Body         []byte
	ContentType  string
	ETag         string
	LastModified string
	FetchedAt    time.Time
	FromCache    bool
	NotModified  bool
	StatusCode   int
}

type Options struct {
	CacheDir         string
	DefaultUserAgent string
	MaxBodyBytes     int64
}

type HTTPFetcher struct {
	client           *http.Client
	defaultUserAgent string
	maxBodyBytes     int64
	cacheDir         string

	rateMu           sync.Mutex
	lastRequestByKey map[string]time.Time
}

type cacheEntry struct {
	Body         []byte    `json:"-"`
	ContentType  string    `json:"content_type"`
	ETag         string    `json:"etag"`
	LastModified string    `json:"last_modified"`
	FetchedAt    time.Time `json:"fetched_at"`
}

func NewHTTPFetcher() *HTTPFetcher {
	return NewHTTPFetcherWithOptions(Options{
		CacheDir:         filepath.Join(".", "data", "fetch-cache"),
		DefaultUserAgent: UAMihomo,
		MaxBodyBytes:     10 << 20,
	})
}

func NewHTTPFetcherWithOptions(options Options) *HTTPFetcher {
	cacheDir := options.CacheDir
	if cacheDir == "" {
		cacheDir = filepath.Join(".", "data", "fetch-cache")
	}
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		cacheDir = ""
	}

	defaultUserAgent := options.DefaultUserAgent
	if defaultUserAgent == "" {
		defaultUserAgent = UAMihomo
	}
	maxBodyBytes := options.MaxBodyBytes
	if maxBodyBytes <= 0 {
		maxBodyBytes = 10 << 20
	}

	return &HTTPFetcher{
		client: &http.Client{
			Transport: http.DefaultTransport,
		},
		defaultUserAgent: defaultUserAgent,
		maxBodyBytes:     maxBodyBytes,
		cacheDir:         cacheDir,
		lastRequestByKey: make(map[string]time.Time),
	}
}

func (f *HTTPFetcher) Fetch(ctx context.Context, req Request) (Artifact, error) {
	if req.URL == "" {
		return Artifact{}, fmt.Errorf("fetcher: empty url")
	}

	cacheKey := firstNonEmpty(req.CacheKey, req.URL)
	cacheEntry, _ := f.readCache(cacheKey)
	if req.CacheTTL > 0 && cacheEntry != nil && time.Since(cacheEntry.FetchedAt) <= req.CacheTTL {
		return artifactFromCache(cacheEntry, http.StatusOK, true, false), nil
	}

	if err := f.waitRateLimit(ctx, firstNonEmpty(req.RateLimitKey, cacheKey), req.MinInterval); err != nil {
		return Artifact{}, err
	}

	attempts := 1 + maxInt(req.RetryAttempts, 0)
	backoff := req.RetryBackoff
	if backoff <= 0 {
		backoff = 500 * time.Millisecond
	}
	if req.AllowStale || cacheEntry != nil {
		req.AllowStale = true
	}

	for attempt := 0; attempt < attempts; attempt++ {
		artifact, retry, err := f.fetchOnce(ctx, req, cacheEntry)
		if err == nil {
			if artifact.StatusCode == http.StatusNotModified && cacheEntry != nil {
				cacheArtifact := artifactFromCache(cacheEntry, http.StatusNotModified, true, true)
				cacheArtifact.ETag = firstNonEmpty(artifact.ETag, cacheArtifact.ETag)
				cacheArtifact.LastModified = firstNonEmpty(artifact.LastModified, cacheArtifact.LastModified)
				cacheArtifact.FetchedAt = time.Now().UTC()
				return cacheArtifact, nil
			}
			if artifact.StatusCode >= 200 && artifact.StatusCode < 300 {
				artifact.FetchedAt = time.Now().UTC()
				if cacheKey != "" {
					_ = f.writeCache(cacheKey, cacheEntryFromArtifact(artifact))
				}
				return artifact, nil
			}
		}

		if !retry || attempt == attempts-1 {
			if req.AllowStale && cacheEntry != nil {
				return artifactFromCache(cacheEntry, http.StatusOK, true, false), nil
			}
			if err != nil {
				return Artifact{}, err
			}
			return Artifact{}, fmt.Errorf("fetcher: unexpected status %d", artifact.StatusCode)
		}

		if err := sleepContext(ctx, backoffForAttempt(backoff, attempt)); err != nil {
			return Artifact{}, err
		}
	}

	if req.AllowStale && cacheEntry != nil {
		return artifactFromCache(cacheEntry, http.StatusOK, true, false), nil
	}
	return Artifact{}, fmt.Errorf("fetcher: exhausted retries")
}

func (f *HTTPFetcher) fetchOnce(ctx context.Context, req Request, cached *cacheEntry) (Artifact, bool, error) {
	if req.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, req.Timeout)
		defer cancel()
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, req.URL, nil)
	if err != nil {
		return Artifact{}, false, fmt.Errorf("fetcher: build request: %w", err)
	}

	httpReq.Header.Set("Accept", "*/*")
	userAgent := req.UserAgent
	if userAgent == "" {
		userAgent = f.defaultUserAgent
	}
	httpReq.Header.Set("User-Agent", userAgent)
	for key, value := range req.Headers {
		httpReq.Header.Set(key, value)
	}

	ifNoneMatch := firstNonEmpty(req.IfNoneMatch, fromCacheField(cached, func(in *cacheEntry) string { return in.ETag }))
	if ifNoneMatch != "" {
		httpReq.Header.Set("If-None-Match", ifNoneMatch)
	}
	ifModifiedSince := firstNonEmpty(req.IfModifiedSince, fromCacheField(cached, func(in *cacheEntry) string { return in.LastModified }))
	if ifModifiedSince != "" {
		httpReq.Header.Set("If-Modified-Since", ifModifiedSince)
	}

	resp, err := f.client.Do(httpReq)
	if err != nil {
		return Artifact{}, isRetryableError(err), fmt.Errorf("fetcher: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotModified {
		return Artifact{
			ETag:         resp.Header.Get("ETag"),
			LastModified: resp.Header.Get("Last-Modified"),
			StatusCode:   resp.StatusCode,
		}, false, nil
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Artifact{StatusCode: resp.StatusCode}, isRetryableStatus(resp.StatusCode), nil
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, f.maxBodyBytes+1))
	if err != nil {
		return Artifact{}, true, fmt.Errorf("fetcher: read body: %w", err)
	}
	if int64(len(body)) > f.maxBodyBytes {
		return Artifact{}, false, fmt.Errorf("fetcher: response too large")
	}

	return Artifact{
		Body:         body,
		ContentType:  resp.Header.Get("Content-Type"),
		ETag:         resp.Header.Get("ETag"),
		LastModified: resp.Header.Get("Last-Modified"),
		StatusCode:   resp.StatusCode,
	}, false, nil
}

func (f *HTTPFetcher) waitRateLimit(ctx context.Context, key string, interval time.Duration) error {
	if key == "" || interval <= 0 {
		return nil
	}

	f.rateMu.Lock()
	last := f.lastRequestByKey[key]
	now := time.Now().UTC()
	var waitFor time.Duration

	if last.IsZero() || now.Sub(last) >= interval {
		f.lastRequestByKey[key] = now
	} else {
		waitFor = interval - now.Sub(last)
		f.lastRequestByKey[key] = last.Add(interval)
	}
	f.rateMu.Unlock()

	if waitFor > 0 {
		if err := sleepContext(ctx, waitFor); err != nil {
			return err
		}
	}
	return nil
}

func (f *HTTPFetcher) readCache(key string) (*cacheEntry, error) {
	path, ok := f.cachePath(key)
	if !ok {
		return nil, os.ErrNotExist
	}
	payload, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var entry cacheEntry
	if err := json.Unmarshal(payload, &entry); err != nil {
		return nil, err
	}

	bodyPath := path + ".bin"
	bodyBuf, err := os.ReadFile(bodyPath)
	if err == nil {
		entry.Body = bodyBuf
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	return &entry, nil
}

func (f *HTTPFetcher) writeCache(key string, entry cacheEntry) error {
	path, ok := f.cachePath(key)
	if !ok {
		return nil
	}

	payload, err := json.Marshal(entry)
	if err != nil {
		return err
	}

	tmpMeta := path + ".tmp"
	tmpBin := path + ".bin.tmp"

	if len(entry.Body) > 0 {
		if err := os.WriteFile(tmpBin, entry.Body, 0o644); err != nil {
			return err
		}
	}

	if err := os.WriteFile(tmpMeta, payload, 0o644); err != nil {
		return err
	}

	if len(entry.Body) > 0 {
		_ = os.Rename(tmpBin, path+".bin")
	}
	return os.Rename(tmpMeta, path)
}

func (f *HTTPFetcher) cachePath(key string) (string, bool) {
	if f.cacheDir == "" || stringsTrim(key) == "" {
		return "", false
	}
	hash := sha1.Sum([]byte(key))
	return filepath.Join(f.cacheDir, hex.EncodeToString(hash[:])+".json"), true
}

func cacheEntryFromArtifact(artifact Artifact) cacheEntry {
	return cacheEntry{
		Body:         artifact.Body,
		ContentType:  artifact.ContentType,
		ETag:         artifact.ETag,
		LastModified: artifact.LastModified,
		FetchedAt:    artifact.FetchedAt,
	}
}

func artifactFromCache(entry *cacheEntry, statusCode int, fromCache bool, notModified bool) Artifact {
	return Artifact{
		Body:         append([]byte(nil), entry.Body...),
		ContentType:  entry.ContentType,
		ETag:         entry.ETag,
		LastModified: entry.LastModified,
		FetchedAt:    entry.FetchedAt,
		FromCache:    fromCache,
		NotModified:  notModified,
		StatusCode:   statusCode,
	}
}

func backoffForAttempt(base time.Duration, attempt int) time.Duration {
	if attempt <= 0 {
		return base
	}
	return base * time.Duration(1<<attempt)
}

func isRetryableStatus(code int) bool {
	switch code {
	case http.StatusTooManyRequests, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	default:
		return code >= 500
	}
}

func isRetryableError(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return false
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}
	return false
}

func sleepContext(ctx context.Context, duration time.Duration) error {
	if duration <= 0 {
		return nil
	}
	timer := time.NewTimer(duration)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func fromCacheField(entry *cacheEntry, fn func(*cacheEntry) string) string {
	if entry == nil {
		return ""
	}
	return fn(entry)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if stringsTrim(value) != "" {
			return value
		}
	}
	return ""
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func stringsTrim(value string) string {
	return strings.TrimSpace(value)
}
