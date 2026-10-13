package controller

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/winrarr/harbor-entity-operator/internal/harborclient"
)

func TestPreheatInstanceUpdateIncludesHarborID(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/api/v2.0/p2p/preheat/instances/dragonfly" {
			http.NotFound(w, r)
			return
		}
		var instance harborclient.PreheatInstance
		if err := json.NewDecoder(r.Body).Decode(&instance); err != nil {
			t.Fatalf("decode update request: %v", err)
		}
		if instance.ID != 42 {
			t.Errorf("update body ID = %d, want 42", instance.ID)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := harborclient.New(server.URL, "admin", "password")
	if err := preheatInstanceOperations().update(context.Background(), client, "dragonfly", 42, &harborclient.PreheatInstance{Name: "dragonfly", Vendor: "dragonfly"}); err != nil {
		t.Fatalf("preheat instance update returned error: %v", err)
	}
}

func TestHarborUserGroupUpdateIncludesHarborID(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/api/v2.0/usergroups/42" {
			http.NotFound(w, r)
			return
		}
		var group harborclient.UserGroup
		if err := json.NewDecoder(r.Body).Decode(&group); err != nil {
			t.Fatalf("decode update request: %v", err)
		}
		if group.ID != 42 {
			t.Errorf("update body ID = %d, want 42", group.ID)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := harborclient.New(server.URL, "admin", "password")
	if err := harborUserGroupOperations().update(context.Background(), client, "developers", 42, &harborclient.UserGroup{GroupName: "developers", GroupType: 2}); err != nil {
		t.Fatalf("user group update returned error: %v", err)
	}
}
