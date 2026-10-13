package harborclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCreateHarborRoleReadsIDFromResponseBody(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v2.0/roles" {
			http.NotFound(w, r)
			return
		}
		var got HarborRole
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode role request: %v", err)
		}
		if got.Name != "release-reader" || got.Description != "Read release metadata" {
			t.Errorf("unexpected role payload: %#v", got)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":73,"name":"release-reader"}`))
	}))
	defer server.Close()

	id, err := New(server.URL, "admin", "password").CreateHarborRole(context.Background(), HarborRole{
		Name: "release-reader", Description: "Read release metadata",
	})
	if err != nil {
		t.Fatalf("CreateHarborRole returned error: %v", err)
	}
	if id != 73 {
		t.Fatalf("CreateHarborRole returned ID %d, want 73", id)
	}
}

func TestPreheatCreatesAcceptNameBasedLocationHeaders(t *testing.T) {
	t.Parallel()

	paths := make(chan string, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths <- r.URL.Path
		if r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Location", r.URL.Path+"/policy-name")
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	client := New(server.URL, "admin", "password")
	if id, err := client.CreatePreheatInstance(context.Background(), PreheatInstance{Name: "instance-name"}); err != nil || id != 0 {
		t.Fatalf("CreatePreheatInstance = (%d, %v), want (0, nil)", id, err)
	}
	if id, err := client.CreatePreheatPolicy(context.Background(), "project-name", PreheatPolicy{Name: "policy-name"}); err != nil || id != 0 {
		t.Fatalf("CreatePreheatPolicy = (%d, %v), want (0, nil)", id, err)
	}
	if got, want := <-paths, "/api/v2.0/p2p/preheat/instances"; got != want {
		t.Fatalf("preheat instance path = %q, want %q", got, want)
	}
	if got, want := <-paths, "/api/v2.0/projects/project-name/preheat/policies"; got != want {
		t.Fatalf("preheat policy path = %q, want %q", got, want)
	}
}

func TestSystemCVEAllowlistUpdateSendsEmptyItems(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/api/v2.0/system/CVEAllowlist" {
			http.NotFound(w, r)
			return
		}
		var payload map[string]json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode allowlist request: %v", err)
		}
		if got := string(payload["items"]); got != "[]" {
			t.Errorf("empty items encoded as %q, want []", got)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	if err := New(server.URL, "admin", "password").UpdateSystemCVEAllowlist(context.Background(), SystemCVEAllowlist{Items: []SystemCVEAllowlistItem{}}); err != nil {
		t.Fatalf("UpdateSystemCVEAllowlist returned error: %v", err)
	}
}

func TestPreheatCreateRequestUsesSpecProjectNameRoute(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || !strings.HasPrefix(r.URL.Path, "/api/v2.0/projects/library/preheat/policies") {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	if _, err := New(server.URL, "admin", "password").CreatePreheatPolicy(context.Background(), "library", PreheatPolicy{Name: "manual"}); err != nil {
		t.Fatalf("CreatePreheatPolicy returned error: %v", err)
	}
}
