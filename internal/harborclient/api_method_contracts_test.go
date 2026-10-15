package harborclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
)

type capturedHarborRequest struct {
	method string
	path   string
	query  url.Values
	body   map[string]json.RawMessage
}

//nolint:gocyclo // A single contract matrix keeps endpoint, filter, and method coverage consistent.
func TestHarborResourceMethodsUseExpectedEndpointsAndFilters(t *testing.T) {
	var requests []capturedHarborRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured := capturedHarborRequest{method: r.Method, path: r.URL.EscapedPath(), query: r.URL.Query()}
		if r.Body != nil {
			var body map[string]json.RawMessage
			if err := json.NewDecoder(r.Body).Decode(&body); err == nil {
				captured.body = body
			}
		}
		requests = append(requests, captured)

		switch {
		case r.Method == http.MethodDelete:
			http.NotFound(w, r)
		case r.Method == http.MethodGet && r.URL.Path == "/api/v2.0/users/current/permissions":
			_, _ = io.WriteString(w, `[]`)
		case r.Method == http.MethodGet && r.URL.Query().Has("page"):
			_, _ = io.WriteString(w, `[]`)
		case r.Method == http.MethodGet:
			_, _ = io.WriteString(w, `{}`)
		case r.Method == http.MethodPost && r.URL.Path == "/api/v2.0/roles":
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"id":42}`)
		case r.Method == http.MethodPost && r.URL.Path == "/api/v2.0/robots":
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"id":42,"name":"robot","secret":"created-secret"}`)
		case r.Method == http.MethodPost && r.URL.Path == "/api/v2.0/scanners":
			w.Header().Set("Location", "/api/v2.0/scanners/scanner-uuid")
			w.WriteHeader(http.StatusCreated)
		case r.Method == http.MethodPost && r.URL.Path == "/api/v2.0/p2p/preheat/instances":
			w.WriteHeader(http.StatusCreated)
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/preheat/policies"):
			w.WriteHeader(http.StatusCreated)
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/immutabletagrules"):
			w.WriteHeader(http.StatusCreated)
		case r.Method == http.MethodPatch && r.URL.Path == "/api/v2.0/robots/15":
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"secret":"rotated-secret"}`)
		case r.Method == http.MethodPost || r.Method == http.MethodPatch:
			w.Header().Set("Location", "/api/v2.0/resources/42")
			w.WriteHeader(http.StatusCreated)
		default:
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	defer server.Close()
	client := NewWithHTTPClient(server.URL, "robot", "token", server.Client())
	ctx := context.Background()
	type methodCase struct {
		name    string
		verb    string
		path    string
		filters map[string]string
		paged   bool
		call    func() error
	}
	numericID := func(id int, err error) error {
		if err != nil {
			return err
		}
		if id != 42 {
			return fmt.Errorf("created ID = %d, want 42", id)
		}
		return nil
	}
	noErr := func(err error) error { return err }
	cases := []methodCase{
		{name: "ping", verb: http.MethodGet, path: "/api/v2.0/ping", call: func() error { return client.Ping(ctx) }},
		{name: "authentication", verb: http.MethodGet, path: "/api/v2.0/users/current/permissions", call: func() error { return client.CheckAuthentication(ctx) }},
		{name: "list projects", verb: http.MethodGet, path: "/api/v2.0/projects", paged: true, call: func() error { _, err := client.ListProjects(ctx); return err }},
		{name: "find project", verb: http.MethodGet, path: "/api/v2.0/projects", filters: map[string]string{"q": "name=team"}, paged: true, call: func() error { _, err := client.FindProjectByName(ctx, "team"); return err }},
		{name: "get project", verb: http.MethodGet, path: "/api/v2.0/projects/7", call: func() error { _, err := client.GetProjectByID(ctx, 7); return err }},
		{name: "create project", verb: http.MethodPost, path: "/api/v2.0/projects", call: func() error {
			id, err := client.CreateProject(ctx, CreateProjectRequest{ProjectName: "team"})
			return numericID(id, err)
		}},
		{name: "update project", verb: http.MethodPut, path: "/api/v2.0/projects/7", call: func() error { return noErr(client.UpdateProject(ctx, 7, CreateProjectRequest{ProjectName: "team"})) }},
		{name: "delete project", verb: http.MethodDelete, path: "/api/v2.0/projects/7", call: func() error { return client.DeleteProject(ctx, 7) }},
		{name: "find registry", verb: http.MethodGet, path: "/api/v2.0/registries", filters: map[string]string{"q": "name=upstream"}, paged: true, call: func() error { _, err := client.FindRegistryByName(ctx, "upstream"); return err }},
		{name: "get registry", verb: http.MethodGet, path: "/api/v2.0/registries/8", call: func() error { _, err := client.GetRegistryByID(ctx, 8); return err }},
		{name: "create registry", verb: http.MethodPost, path: "/api/v2.0/registries", call: func() error {
			id, err := client.CreateRegistry(ctx, CreateRegistryRequest{Name: "upstream"})
			return numericID(id, err)
		}},
		{name: "update registry", verb: http.MethodPut, path: "/api/v2.0/registries/8", call: func() error { return client.UpdateRegistry(ctx, 8, UpdateRegistryRequest{Name: "upstream"}) }},
		{name: "delete registry", verb: http.MethodDelete, path: "/api/v2.0/registries/8", call: func() error { return client.DeleteRegistry(ctx, 8) }},
		{name: "list labels", verb: http.MethodGet, path: "/api/v2.0/labels", filters: map[string]string{"name": "release", "scope": "p", "project_id": "17"}, paged: true, call: func() error {
			projectID := 17
			_, err := client.ListLabels(ctx, "release", "p", &projectID)
			return err
		}},
		{name: "get label", verb: http.MethodGet, path: "/api/v2.0/labels/9", call: func() error { _, err := client.GetLabel(ctx, 9); return err }},
		{name: "create label", verb: http.MethodPost, path: "/api/v2.0/labels", call: func() error { id, err := client.CreateLabel(ctx, Label{Name: "release"}); return numericID(id, err) }},
		{name: "update label", verb: http.MethodPut, path: "/api/v2.0/labels/9", call: func() error { return client.UpdateLabel(ctx, 9, Label{Name: "release"}) }},
		{name: "delete label", verb: http.MethodDelete, path: "/api/v2.0/labels/9", call: func() error { return client.DeleteLabel(ctx, 9) }},
		{name: "list project members", verb: http.MethodGet, path: "/api/v2.0/projects/library/members", paged: true, call: func() error { _, err := client.ListProjectMembers(ctx, "library"); return err }},
		{name: "create project member", verb: http.MethodPost, path: "/api/v2.0/projects/library/members", call: func() error {
			id, err := client.CreateProjectMember(ctx, "library", CreateMemberRequest{RoleID: 2, MemberUser: &MemberUser{Username: "alice"}})
			return numericID(id, err)
		}},
		{name: "get project member", verb: http.MethodGet, path: "/api/v2.0/projects/library/members/12", call: func() error { _, err := client.GetProjectMember(ctx, "library", 12); return err }},
		{name: "update member role", verb: http.MethodPut, path: "/api/v2.0/projects/library/members/12", call: func() error { return client.UpdateProjectMemberRole(ctx, "library", 12, 4) }},
		{name: "delete project member", verb: http.MethodDelete, path: "/api/v2.0/projects/library/members/12", call: func() error { return client.DeleteProjectMember(ctx, "library", 12) }},
		{name: "list users", verb: http.MethodGet, path: "/api/v2.0/users", filters: map[string]string{"q": "username=alice"}, paged: true, call: func() error { _, err := client.ListUsers(ctx, "username=alice"); return err }},
		{name: "get user", verb: http.MethodGet, path: "/api/v2.0/users/13", call: func() error { _, err := client.GetUserByID(ctx, 13); return err }},
		{name: "create user", verb: http.MethodPost, path: "/api/v2.0/users", call: func() error {
			id, err := client.CreateUser(ctx, CreateUserRequest{Username: "alice"})
			return numericID(id, err)
		}},
		{name: "update user", verb: http.MethodPut, path: "/api/v2.0/users/13", call: func() error { return client.UpdateUser(ctx, 13, CreateUserRequest{Realname: "Alice"}) }},
		{name: "delete user", verb: http.MethodDelete, path: "/api/v2.0/users/13", call: func() error { return client.DeleteUser(ctx, 13) }},
		{name: "list user groups", verb: http.MethodGet, path: "/api/v2.0/usergroups", paged: true, call: func() error { _, err := client.ListUserGroups(ctx); return err }},
		{name: "create user group", verb: http.MethodPost, path: "/api/v2.0/usergroups", call: func() error {
			id, err := client.CreateUserGroup(ctx, UserGroup{GroupName: "developers"})
			return numericID(id, err)
		}},
		{name: "get user group", verb: http.MethodGet, path: "/api/v2.0/usergroups/14", call: func() error { _, err := client.GetUserGroup(ctx, 14); return err }},
		{name: "update user group", verb: http.MethodPut, path: "/api/v2.0/usergroups/14", call: func() error { return client.UpdateUserGroup(ctx, 14, UserGroup{GroupName: "developers"}) }},
		{name: "delete user group", verb: http.MethodDelete, path: "/api/v2.0/usergroups/14", call: func() error { return client.DeleteUserGroup(ctx, 14) }},
		{name: "list robots", verb: http.MethodGet, path: "/api/v2.0/robots", filters: map[string]string{"q": "name=robot"}, paged: true, call: func() error { _, err := client.ListRobots(ctx, "name=robot"); return err }},
		{name: "get robot", verb: http.MethodGet, path: "/api/v2.0/robots/15", call: func() error { _, err := client.GetRobotByID(ctx, 15); return err }},
		{name: "create robot", verb: http.MethodPost, path: "/api/v2.0/robots", call: func() error {
			created, err := client.CreateRobot(ctx, RobotCreateRequest{Name: "robot"})
			if err == nil && (created.ID != 42 || created.Secret != "created-secret") {
				return fmt.Errorf("created robot = %#v", created)
			}
			return err
		}},
		{name: "update robot", verb: http.MethodPut, path: "/api/v2.0/robots/15", call: func() error { return client.UpdateRobot(ctx, 15, Robot{Name: "robot"}) }},
		{name: "refresh robot secret", verb: http.MethodPatch, path: "/api/v2.0/robots/15", call: func() error {
			rotated, err := client.RefreshRobotSecret(ctx, 15, "next-secret")
			if err == nil && rotated.Secret != "rotated-secret" {
				return fmt.Errorf("rotated secret response = %#v", rotated)
			}
			return err
		}},
		{name: "delete robot", verb: http.MethodDelete, path: "/api/v2.0/robots/15", call: func() error { return client.DeleteRobot(ctx, 15) }},
		{name: "list scanners", verb: http.MethodGet, path: "/api/v2.0/scanners", paged: true, call: func() error { _, err := client.ListScanners(ctx); return err }},
		{name: "get scanner", verb: http.MethodGet, path: "/api/v2.0/scanners/scanner-uuid", call: func() error { _, err := client.GetScanner(ctx, "scanner-uuid"); return err }},
		{name: "create scanner", verb: http.MethodPost, path: "/api/v2.0/scanners", call: func() error {
			id, err := client.CreateScanner(ctx, ScannerRegistrationReq{Name: "scanner"})
			if err == nil && id != "scanner-uuid" {
				return fmt.Errorf("created scanner ID = %q", id)
			}
			return err
		}},
		{name: "update scanner", verb: http.MethodPut, path: "/api/v2.0/scanners/scanner-uuid", call: func() error {
			return client.UpdateScanner(ctx, "scanner-uuid", ScannerRegistrationReq{Name: "scanner"})
		}},
		{name: "delete scanner", verb: http.MethodDelete, path: "/api/v2.0/scanners/scanner-uuid", call: func() error { return client.DeleteScanner(ctx, "scanner-uuid") }},
		{name: "set default scanner", verb: http.MethodPatch, path: "/api/v2.0/scanners/scanner-uuid", call: func() error { return client.SetDefaultScanner(ctx, "scanner-uuid", false) }},
		{name: "get configurations", verb: http.MethodGet, path: "/api/v2.0/configurations", call: func() error { _, err := client.GetConfigurations(ctx); return err }},
		{name: "update configurations", verb: http.MethodPut, path: "/api/v2.0/configurations", call: func() error { return client.UpdateConfigurations(ctx, map[string]any{"scan_all_policy": true}) }},
		{name: "list quotas", verb: http.MethodGet, path: "/api/v2.0/quotas", filters: map[string]string{"reference": "project", "reference_id": "17"}, paged: true, call: func() error { _, err := client.ListQuotas(ctx, "project", "17"); return err }},
		{name: "get quota", verb: http.MethodGet, path: "/api/v2.0/quotas/17", call: func() error { _, err := client.GetQuota(ctx, 17); return err }},
		{name: "update quota", verb: http.MethodPut, path: "/api/v2.0/quotas/17", call: func() error { return client.UpdateQuota(ctx, 17, map[string]int64{"storage": 100}) }},
		{name: "get GC schedule", verb: http.MethodGet, path: "/api/v2.0/system/gc/schedule", call: func() error { _, err := client.GetGCSchedule(ctx); return err }},
		{name: "create GC schedule", verb: http.MethodPost, path: "/api/v2.0/system/gc/schedule", call: func() error { return client.CreateGCSchedule(ctx, Schedule{}) }},
		{name: "update GC schedule", verb: http.MethodPut, path: "/api/v2.0/system/gc/schedule", call: func() error { return client.UpdateGCSchedule(ctx, Schedule{}) }},
		{name: "get purge schedule", verb: http.MethodGet, path: "/api/v2.0/system/purgeaudit/schedule", call: func() error { _, err := client.GetPurgeSchedule(ctx); return err }},
		{name: "create purge schedule", verb: http.MethodPost, path: "/api/v2.0/system/purgeaudit/schedule", call: func() error { return client.CreatePurgeSchedule(ctx, Schedule{}) }},
		{name: "update purge schedule", verb: http.MethodPut, path: "/api/v2.0/system/purgeaudit/schedule", call: func() error { return client.UpdatePurgeSchedule(ctx, Schedule{}) }},
		{name: "get scan-all schedule", verb: http.MethodGet, path: "/api/v2.0/system/scanAll/schedule", call: func() error { _, err := client.GetScanAllSchedule(ctx); return err }},
		{name: "create scan-all schedule", verb: http.MethodPost, path: "/api/v2.0/system/scanAll/schedule", call: func() error { return client.CreateScanAllSchedule(ctx, Schedule{}) }},
		{name: "update scan-all schedule", verb: http.MethodPut, path: "/api/v2.0/system/scanAll/schedule", call: func() error { return client.UpdateScanAllSchedule(ctx, Schedule{}) }},
		{name: "get retention", verb: http.MethodGet, path: "/api/v2.0/retentions/18", call: func() error { _, err := client.GetRetentionByID(ctx, 18); return err }},
		{name: "create retention", verb: http.MethodPost, path: "/api/v2.0/retentions", call: func() error {
			id, err := client.CreateRetention(ctx, RetentionPolicy{Algorithm: "or"})
			return numericID(id, err)
		}},
		{name: "update retention", verb: http.MethodPut, path: "/api/v2.0/retentions/18", call: func() error { return client.UpdateRetention(ctx, 18, RetentionPolicy{Algorithm: "or"}) }},
		{name: "delete retention", verb: http.MethodDelete, path: "/api/v2.0/retentions/18", call: func() error { return client.DeleteRetention(ctx, 18) }},
		{name: "list replication policies", verb: http.MethodGet, path: "/api/v2.0/replication/policies", filters: map[string]string{"name": "mirror"}, paged: true, call: func() error { _, err := client.ListReplicationPolicies(ctx, "mirror"); return err }},
		{name: "get replication policy", verb: http.MethodGet, path: "/api/v2.0/replication/policies/19", call: func() error { _, err := client.GetReplicationPolicy(ctx, 19); return err }},
		{name: "create replication policy", verb: http.MethodPost, path: "/api/v2.0/replication/policies", call: func() error {
			id, err := client.CreateReplicationPolicy(ctx, ReplicationPolicy{Name: "mirror"})
			return numericID(id, err)
		}},
		{name: "update replication policy", verb: http.MethodPut, path: "/api/v2.0/replication/policies/19", call: func() error { return client.UpdateReplicationPolicy(ctx, 19, ReplicationPolicy{Name: "mirror"}) }},
		{name: "delete replication policy", verb: http.MethodDelete, path: "/api/v2.0/replication/policies/19", call: func() error { return client.DeleteReplicationPolicy(ctx, 19) }},
		{name: "list immutable rules", verb: http.MethodGet, path: "/api/v2.0/projects/library/immutabletagrules", paged: true, call: func() error { _, err := client.ListImmutableRules(ctx, "library"); return err }},
		{name: "create immutable rule", verb: http.MethodPost, path: "/api/v2.0/projects/library/immutabletagrules", call: func() error { return client.CreateImmutableRule(ctx, "library", ImmutableRule{Action: "immutable"}) }},
		{name: "update immutable rule", verb: http.MethodPut, path: "/api/v2.0/projects/library/immutabletagrules/20", call: func() error {
			return client.UpdateImmutableRule(ctx, "library", 20, ImmutableRule{Action: "immutable"})
		}},
		{name: "delete immutable rule", verb: http.MethodDelete, path: "/api/v2.0/projects/library/immutabletagrules/20", call: func() error { return client.DeleteImmutableRule(ctx, "library", 20) }},
		{name: "list webhook policies", verb: http.MethodGet, path: "/api/v2.0/projects/library/webhook/policies", paged: true, call: func() error { _, err := client.ListWebhookPolicies(ctx, "library"); return err }},
		{name: "get webhook policy", verb: http.MethodGet, path: "/api/v2.0/projects/library/webhook/policies/21", call: func() error { _, err := client.GetWebhookPolicy(ctx, "library", 21); return err }},
		{name: "create webhook policy", verb: http.MethodPost, path: "/api/v2.0/projects/library/webhook/policies", call: func() error {
			id, err := client.CreateWebhookPolicy(ctx, "library", WebhookPolicy{Name: "notify"})
			return numericID(id, err)
		}},
		{name: "update webhook policy", verb: http.MethodPut, path: "/api/v2.0/projects/library/webhook/policies/21", call: func() error { return client.UpdateWebhookPolicy(ctx, "library", 21, WebhookPolicy{Name: "notify"}) }},
		{name: "delete webhook policy", verb: http.MethodDelete, path: "/api/v2.0/projects/library/webhook/policies/21", call: func() error { return client.DeleteWebhookPolicy(ctx, "library", 21) }},
		{name: "list Harbor roles", verb: http.MethodGet, path: "/api/v2.0/roles", paged: true, call: func() error { _, err := client.ListHarborRoles(ctx); return err }},
		{name: "get Harbor role", verb: http.MethodGet, path: "/api/v2.0/roles/22", call: func() error { _, err := client.GetHarborRole(ctx, 22); return err }},
		{name: "create Harbor role", verb: http.MethodPost, path: "/api/v2.0/roles", call: func() error {
			id, err := client.CreateHarborRole(ctx, HarborRole{Name: "auditor"})
			return numericID(id, err)
		}},
		{name: "update Harbor role", verb: http.MethodPut, path: "/api/v2.0/roles/22", call: func() error { return client.UpdateHarborRole(ctx, 22, HarborRole{Name: "auditor"}) }},
		{name: "delete Harbor role", verb: http.MethodDelete, path: "/api/v2.0/roles/22", call: func() error { return client.DeleteHarborRole(ctx, 22) }},
		{name: "get system CVE allowlist", verb: http.MethodGet, path: "/api/v2.0/system/CVEAllowlist", call: func() error { _, err := client.GetSystemCVEAllowlist(ctx); return err }},
		{name: "update system CVE allowlist", verb: http.MethodPut, path: "/api/v2.0/system/CVEAllowlist", call: func() error {
			return client.UpdateSystemCVEAllowlist(ctx, SystemCVEAllowlist{Items: []SystemCVEAllowlistItem{}})
		}},
		{name: "list preheat instances", verb: http.MethodGet, path: "/api/v2.0/p2p/preheat/instances", paged: true, call: func() error { _, err := client.ListPreheatInstances(ctx); return err }},
		{name: "get escaped preheat instance", verb: http.MethodGet, path: "/api/v2.0/p2p/preheat/instances/cache%2Fprovider", call: func() error { _, err := client.GetPreheatInstance(ctx, "cache/provider"); return err }},
		{name: "create preheat instance", verb: http.MethodPost, path: "/api/v2.0/p2p/preheat/instances", call: func() error {
			id, err := client.CreatePreheatInstance(ctx, PreheatInstance{Name: "cache"})
			if err == nil && id != 0 {
				return fmt.Errorf("preheat instance create ID = %d, want no Harbor ID", id)
			}
			return err
		}},
		{name: "update escaped preheat instance", verb: http.MethodPut, path: "/api/v2.0/p2p/preheat/instances/cache%2Fprovider", call: func() error {
			return client.UpdatePreheatInstance(ctx, "cache/provider", PreheatInstance{Name: "cache/provider"})
		}},
		{name: "delete escaped preheat instance", verb: http.MethodDelete, path: "/api/v2.0/p2p/preheat/instances/cache%2Fprovider", call: func() error { return client.DeletePreheatInstance(ctx, "cache/provider") }},
		{name: "list escaped preheat policies", verb: http.MethodGet, path: "/api/v2.0/projects/team%2Fproject/preheat/policies", paged: true, call: func() error { _, err := client.ListPreheatPolicies(ctx, "team/project"); return err }},
		{name: "get escaped preheat policy", verb: http.MethodGet, path: "/api/v2.0/projects/team%2Fproject/preheat/policies/warm%2Fcold", call: func() error { _, err := client.GetPreheatPolicy(ctx, "team/project", "warm/cold"); return err }},
		{name: "create escaped preheat policy", verb: http.MethodPost, path: "/api/v2.0/projects/team%2Fproject/preheat/policies", call: func() error {
			id, err := client.CreatePreheatPolicy(ctx, "team/project", PreheatPolicy{Name: "warm/cold"})
			if err == nil && id != 0 {
				return fmt.Errorf("preheat policy create ID = %d, want no Harbor ID", id)
			}
			return err
		}},
		{name: "update escaped preheat policy", verb: http.MethodPut, path: "/api/v2.0/projects/team%2Fproject/preheat/policies/warm%2Fcold", call: func() error {
			return client.UpdatePreheatPolicy(ctx, "team/project", "warm/cold", PreheatPolicy{Name: "warm/cold"})
		}},
		{name: "delete escaped preheat policy", verb: http.MethodDelete, path: "/api/v2.0/projects/team%2Fproject/preheat/policies/warm%2Fcold", call: func() error { return client.DeletePreheatPolicy(ctx, "team/project", "warm/cold") }},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			before := len(requests)
			if err := test.call(); err != nil {
				t.Fatalf("client call: %v", err)
			}
			if len(requests) != before+1 {
				t.Fatalf("request count changed from %d to %d; want exactly one", before, len(requests))
			}
			request := requests[before]
			if request.method != test.verb || request.path != test.path {
				t.Fatalf("request = %s %s, want %s %s", request.method, request.path, test.verb, test.path)
			}
			for key, want := range test.filters {
				if got := request.query.Get(key); got != want {
					t.Errorf("query %s = %q, want %q", key, got, want)
				}
			}
			if test.paged && (request.query.Get("page") != "1" || request.query.Get("page_size") != "100") {
				t.Errorf("list pagination query = %v, want page=1 and page_size=100", request.query)
			}
		})
	}

	assertRequestBodyInt(t, requests, http.MethodPut, "/api/v2.0/projects/library/members/12", "role_id", 4)
	assertRequestBodyBool(t, requests, http.MethodPatch, "/api/v2.0/scanners/scanner-uuid", "is_default", false)
	assertRequestBodyString(t, requests, http.MethodPatch, "/api/v2.0/robots/15", "secret", "next-secret")
	assertRequestBodyIntMap(t, requests, http.MethodPut, "/api/v2.0/quotas/17", "hard", map[string]int64{"storage": 100})
}

func assertRequestBodyInt(t *testing.T, requests []capturedHarborRequest, method, path, field string, want int) {
	t.Helper()
	var got int
	if err := findRequestBody(requests, method, path, field, &got); err != nil || got != want {
		t.Fatalf("%s %s body field %s = %d, %v; want %d", method, path, field, got, err, want)
	}
}

func assertRequestBodyBool(t *testing.T, requests []capturedHarborRequest, method, path, field string, want bool) {
	t.Helper()
	var got bool
	if err := findRequestBody(requests, method, path, field, &got); err != nil || got != want {
		t.Fatalf("%s %s body field %s = %t, %v; want %t", method, path, field, got, err, want)
	}
}

func assertRequestBodyString(t *testing.T, requests []capturedHarborRequest, method, path, field, want string) {
	t.Helper()
	var got string
	if err := findRequestBody(requests, method, path, field, &got); err != nil || got != want {
		t.Fatalf("%s %s body field %s = %q, %v; want %q", method, path, field, got, err, want)
	}
}

func assertRequestBodyIntMap(t *testing.T, requests []capturedHarborRequest, method, path, field string, want map[string]int64) {
	t.Helper()
	var got map[string]int64
	if err := findRequestBody(requests, method, path, field, &got); err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("%s %s body field %s = %#v, %v; want %#v", method, path, field, got, err, want)
	}
}

func findRequestBody(requests []capturedHarborRequest, method, path, field string, out any) error {
	for _, request := range requests {
		if request.method != method || request.path != path {
			continue
		}
		raw, ok := request.body[field]
		if !ok {
			return fmt.Errorf("field is absent")
		}
		return json.Unmarshal(raw, out)
	}
	return fmt.Errorf("request %s %s was not captured", method, path)
}
