package controller

import (
	"container/list"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"

	"github.com/winrarr/harbor-entity-operator/internal/harborclient"
	"github.com/winrarr/harbor-entity-operator/internal/metrics"
)

const defaultHarborClientCacheCapacity = 64

type requestGate struct {
	permits chan struct{}
}

func newRequestGate(maxConcurrentRequests int) *requestGate {
	if maxConcurrentRequests <= 0 {
		return nil
	}
	return &requestGate{permits: make(chan struct{}, maxConcurrentRequests)}
}

func (g *requestGate) roundTripper(next http.RoundTripper) http.RoundTripper {
	if g == nil {
		return next
	}
	return gatedRoundTripper{next: next, permits: g.permits}
}

type gatedRoundTripper struct {
	next    http.RoundTripper
	permits chan struct{}
}

func (t gatedRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	select {
	case t.permits <- struct{}{}:
	case <-request.Context().Done():
		return nil, request.Context().Err()
	}

	response, err := t.next.RoundTrip(request)
	if err != nil {
		<-t.permits
		return nil, err
	}
	if response == nil || response.Body == nil {
		<-t.permits
		return response, nil
	}
	response.Body = &permitBody{ReadCloser: response.Body, permits: t.permits}
	return response, nil
}

type permitBody struct {
	closeOnce sync.Once
	closeErr  error
	permits   chan struct{}
	io.ReadCloser
}

func (b *permitBody) Close() error {
	b.closeOnce.Do(func() {
		b.closeErr = b.ReadCloser.Close()
		<-b.permits
	})
	return b.closeErr
}

type harborHTTPClientCacheEntry struct {
	identity    string
	fingerprint string
	client      *harborclient.Client
}

type harborHTTPClientCache struct {
	mu       sync.Mutex
	capacity int
	entries  map[string]*list.Element
	lru      list.List
}

func newHarborHTTPClientCache(capacity int) *harborHTTPClientCache {
	if capacity <= 0 {
		capacity = defaultHarborClientCacheCapacity
	}
	return &harborHTTPClientCache{
		capacity: capacity,
		entries:  make(map[string]*list.Element),
	}
}

func (c *harborHTTPClientCache) getOrCreate(
	identity, fingerprint string,
	create func() (*harborclient.Client, error),
) (*harborclient.Client, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if element, ok := c.entries[identity]; ok {
		entry := element.Value.(*harborHTTPClientCacheEntry)
		if entry.fingerprint == fingerprint {
			c.lru.MoveToFront(element)
			metrics.ObserveHarborHTTPClientCache("hit")
			return entry.client, nil
		}
		c.remove(element)
		metrics.ObserveHarborHTTPClientCache("invalidation")
	}

	client, err := create()
	if err != nil {
		return nil, err
	}
	metrics.ObserveHarborHTTPClientCache("miss")
	entry := &harborHTTPClientCacheEntry{identity: identity, fingerprint: fingerprint, client: client}
	c.entries[identity] = c.lru.PushFront(entry)
	for len(c.entries) > c.capacity {
		c.remove(c.lru.Back())
		metrics.ObserveHarborHTTPClientCache("eviction")
	}
	return client, nil
}

func (c *harborHTTPClientCache) remove(element *list.Element) {
	if element == nil {
		return
	}
	entry := element.Value.(*harborHTTPClientCacheEntry)
	delete(c.entries, entry.identity)
	c.lru.Remove(element)
	if entry.client != nil && entry.client.HTTPClient != nil {
		entry.client.HTTPClient.CloseIdleConnections()
	}
}

func harborClientCacheIdentity(conn *connectionConfig) string {
	return fmt.Sprintf("%s/%s/%s/%s", conn.kind, conn.namespace, conn.name, conn.uid)
}

func harborClientFingerprint(options OperatorOptions, baseURL, username, password, caBundle string) (string, error) {
	settings, err := json.Marshal(struct {
		BaseURL               string
		Username              string
		Password              string
		CABundle              string
		RequestTimeout        int64
		MaxConcurrentRequests int
	}{
		BaseURL:               baseURL,
		Username:              username,
		Password:              password,
		CABundle:              caBundle,
		RequestTimeout:        int64(options.requestTimeout()),
		MaxConcurrentRequests: options.maxConcurrentRequests(),
	})
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(settings)
	return hex.EncodeToString(digest[:]), nil
}

func createHarborHTTPClient(options OperatorOptions, caBundle string) (*http.Client, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if caBundle != "" {
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM([]byte(caBundle)) {
			return nil, fmt.Errorf("invalid caBundle: no certificates found")
		}
		transport.TLSClientConfig = &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12}
	}

	return &http.Client{
		Timeout:   options.requestTimeout(),
		Transport: options.harborRequestGate.roundTripper(transport),
	}, nil
}
