package harborclient

import (
	"encoding/json"
	"testing"
)

func TestProjectRequestLeavesOmittedCVEAllowlistUnmanaged(t *testing.T) {
	body, err := json.Marshal(CreateProjectRequest{
		ProjectName: "project",
	})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	var request map[string]json.RawMessage
	if err := json.Unmarshal(body, &request); err != nil {
		t.Fatalf("unmarshal request: %v", err)
	}
	if _, exists := request["cve_allowlist"]; exists {
		t.Fatalf("request unexpectedly contains cve_allowlist: %s", body)
	}
}

func TestProjectRequestCanClearCVEAllowlist(t *testing.T) {
	body, err := json.Marshal(CreateProjectRequest{
		ProjectName:  "project",
		CVEAllowlist: &CVEAllowlist{Items: []CVEAllowlistItem{}},
	})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	var request struct {
		CVEAllowlist struct {
			ExpiresAt *int64            `json:"expires_at"`
			Items     []json.RawMessage `json:"items"`
		} `json:"cve_allowlist"`
	}
	if err := json.Unmarshal(body, &request); err != nil {
		t.Fatalf("unmarshal request: %v", err)
	}
	if request.CVEAllowlist.Items == nil || len(request.CVEAllowlist.Items) != 0 {
		t.Fatalf("cve_allowlist.items = %#v, want an explicit empty array in %s", request.CVEAllowlist.Items, body)
	}
	if request.CVEAllowlist.ExpiresAt != nil {
		t.Fatalf("cve_allowlist.expires_at = %d, want null in %s", *request.CVEAllowlist.ExpiresAt, body)
	}
}

func TestProjectRequestCanSendExplicitEmptyMetadata(t *testing.T) {
	empty := ""
	body, err := json.Marshal(CreateProjectRequest{
		ProjectName: "project",
		Metadata:    ProjectMetadata{ProxyCacheFilterPattern: &empty},
	})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	var request struct {
		Metadata map[string]json.RawMessage `json:"metadata"`
	}
	if err := json.Unmarshal(body, &request); err != nil {
		t.Fatalf("unmarshal request: %v", err)
	}
	value, exists := request.Metadata["proxy_cache_filter_pattern"]
	if !exists || string(value) != `""` {
		t.Fatalf("proxy_cache_filter_pattern = %s, want explicit empty string in %s", value, body)
	}
}
