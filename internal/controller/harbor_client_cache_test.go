package controller

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	harborv1alpha1 "github.com/winrarr/harbor-entity-operator/api/v1alpha1"
	"github.com/winrarr/harbor-entity-operator/internal/harborclient"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func TestRequestGateHoldsPermitUntilResponseBodyCloses(t *testing.T) {
	gate := newRequestGate(1)
	transport := gate.roundTripper(roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("ok"))}, nil
	}))
	request, err := http.NewRequest(http.MethodGet, "https://harbor.example.test", nil)
	if err != nil {
		t.Fatal(err)
	}
	first, err := transport.RoundTrip(request)
	if err != nil {
		t.Fatal(err)
	}
	secondDone := make(chan error, 1)
	go func() {
		response, err := transport.RoundTrip(request)
		if err == nil {
			err = response.Body.Close()
		}
		secondDone <- err
	}()
	select {
	case err := <-secondDone:
		t.Fatalf("second request passed the gate before the first body closed: %v", err)
	default:
	}
	if err := first.Body.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-secondDone:
		if err != nil {
			t.Fatalf("second request after permit release: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("second request did not proceed after response body close")
	}
}

func TestRequestGateHonorsCanceledWaiterAndReleasesFailedRequest(t *testing.T) {
	gate := newRequestGate(1)
	var shouldFail bool
	transport := gate.roundTripper(roundTripFunc(func(*http.Request) (*http.Response, error) {
		if shouldFail {
			return nil, errors.New("transport failed")
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("ok"))}, nil
	}))
	request, _ := http.NewRequest(http.MethodGet, "https://harbor.example.test", nil)
	first, err := transport.RoundTrip(request)
	if err != nil {
		t.Fatal(err)
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	waitingRequest := request.Clone(canceled)
	if _, err := transport.RoundTrip(waitingRequest); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled waiting request error = %v, want context canceled", err)
	}
	if err := first.Body.Close(); err != nil {
		t.Fatal(err)
	}
	shouldFail = true
	if _, err := transport.RoundTrip(request); err == nil {
		t.Fatal("transport failure was not returned")
	}
	shouldFail = false
	response, err := transport.RoundTrip(request)
	if err != nil {
		t.Fatalf("permit remained held after transport failure: %v", err)
	}
	_ = response.Body.Close()
}

func TestHarborClientCacheReusesInvalidatesAndBoundsEntries(t *testing.T) {
	cache := newHarborHTTPClientCache(2)
	created := 0
	create := func() (*harborclient.Client, error) {
		created++
		return harborclient.NewWithHTTPClient("https://harbor.example.test", "", "", &http.Client{}), nil
	}
	first, err := cache.getOrCreate("connection-a", "fingerprint-a", create)
	if err != nil {
		t.Fatal(err)
	}
	reused, err := cache.getOrCreate("connection-a", "fingerprint-a", create)
	if err != nil {
		t.Fatal(err)
	}
	if first != reused || created != 1 {
		t.Fatalf("same connection returned distinct clients or created=%d", created)
	}
	updated, err := cache.getOrCreate("connection-a", "fingerprint-b", create)
	if err != nil {
		t.Fatal(err)
	}
	if updated == first || created != 2 {
		t.Fatalf("changed settings did not replace the cached client; created=%d", created)
	}
	if _, err := cache.getOrCreate("connection-b", "fingerprint-a", create); err != nil {
		t.Fatal(err)
	}
	if _, err := cache.getOrCreate("connection-c", "fingerprint-a", create); err != nil {
		t.Fatal(err)
	}
	if len(cache.entries) != 2 {
		t.Fatalf("cache entries = %d, want bounded capacity 2", len(cache.entries))
	}
	if _, exists := cache.entries["connection-a"]; exists {
		t.Fatal("least recently used connection was not evicted")
	}
}

func TestHarborClientFingerprintChangesWithoutExposingSecrets(t *testing.T) {
	options, err := NewOperatorOptions(OperatorConfig{
		DefaultCreationPolicy:       harborv1alpha1.CreationPolicyCreate,
		HarborRequestTimeout:        30 * time.Second,
		HarborMaxConcurrentRequests: 20,
	})
	if err != nil {
		t.Fatal(err)
	}
	base, err := harborClientFingerprint(options, "https://harbor-a.example.test", "user", "sensitive-password", "ca-a")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(base, "sensitive-password") || strings.Contains(base, "ca-a") {
		t.Fatal("fingerprint exposed connection credentials or CA material")
	}
	variants := []struct {
		name string
		make func() (string, error)
	}{
		{name: "endpoint", make: func() (string, error) {
			return harborClientFingerprint(options, "https://harbor-b.example.test", "user", "sensitive-password", "ca-a")
		}},
		{name: "username", make: func() (string, error) {
			return harborClientFingerprint(options, "https://harbor-a.example.test", "other-user", "sensitive-password", "ca-a")
		}},
		{name: "password", make: func() (string, error) {
			return harborClientFingerprint(options, "https://harbor-a.example.test", "user", "other-password", "ca-a")
		}},
		{name: "ca", make: func() (string, error) {
			return harborClientFingerprint(options, "https://harbor-a.example.test", "user", "sensitive-password", "ca-b")
		}},
		{name: "timeout", make: func() (string, error) {
			changed := options
			changed.harborRequestTimeout = 45 * time.Second
			return harborClientFingerprint(changed, "https://harbor-a.example.test", "user", "sensitive-password", "ca-a")
		}},
		{name: "concurrency setting", make: func() (string, error) {
			changed := options
			changed.harborMaxConcurrentRequests = 10
			return harborClientFingerprint(changed, "https://harbor-a.example.test", "user", "sensitive-password", "ca-a")
		}},
	}
	for _, variant := range variants {
		t.Run(variant.name, func(t *testing.T) {
			got, err := variant.make()
			if err != nil || got == base {
				t.Fatalf("fingerprint for changed %s = %q, %v", variant.name, got, err)
			}
		})
	}
}

func BenchmarkHarborHTTPClientSetup(b *testing.B) {
	options, err := NewOperatorOptions(OperatorConfig{
		DefaultCreationPolicy:       harborv1alpha1.CreationPolicyCreate,
		HarborRequestTimeout:        30 * time.Second,
		HarborMaxConcurrentRequests: 20,
	})
	if err != nil {
		b.Fatal(err)
	}
	identity := "HarborConnection/team/production/uid"
	b.Run("new-transport-every-time", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			if _, err := newHarborClient(options, "https://harbor.example.test", "robot", "token", ""); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("cached-transport", func(b *testing.B) {
		cache := newHarborHTTPClientCache(defaultHarborClientCacheCapacity)
		fingerprint, err := harborClientFingerprint(options, "https://harbor.example.test", "robot", "token", "")
		if err != nil {
			b.Fatal(err)
		}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if _, err := cache.getOrCreate(identity, fingerprint, func() (*harborclient.Client, error) {
				return newHarborClient(options, "https://harbor.example.test", "robot", "token", "")
			}); err != nil {
				b.Fatal(err)
			}
		}
	})
}
