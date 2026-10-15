package harborclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
)

func TestHTTPErrorClassificationAndRetentionGoneSemantics(t *testing.T) {
	for _, test := range []struct {
		code    int
		want404 bool
		want409 bool
		want403 bool
	}{
		{code: http.StatusNotFound, want404: true},
		{code: http.StatusConflict, want409: true},
		{code: http.StatusForbidden, want403: true},
		{code: http.StatusBadRequest},
	} {
		err := &HTTPError{StatusCode: test.code, Message: "failure"}
		if IsNotFound(err) != test.want404 || IsConflict(err) != test.want409 || IsForbidden(err) != test.want403 {
			t.Fatalf("status helpers for %d returned unexpected classification", test.code)
		}
		if want := fmt.Sprintf("harbor API %d – failure", test.code); err.Error() != want {
			t.Fatalf("HTTPError.Error() = %q, want %q", err.Error(), want)
		}
	}
	if IsNotFound(nil) || IsConflict(errors.New("not found")) || IsForbidden(&HTTPError{StatusCode: http.StatusNotFound}) {
		t.Fatal("HTTP status helpers misclassified nil or unrelated errors")
	}
	if !isRetentionGone(&HTTPError{StatusCode: http.StatusBadRequest, Message: "no such Retention policy"}) {
		t.Fatal("Harbor's retention-specific 400 was not recognized as already gone")
	}
	if isRetentionGone(&HTTPError{StatusCode: http.StatusBadRequest, Message: "invalid retention policy"}) || isRetentionGone(&HTTPError{StatusCode: http.StatusNotFound}) || isRetentionGone(nil) {
		t.Fatal("unrelated errors were treated as missing retention policy")
	}
}

func TestDoEncodesAuthenticatedJSONAndDecodesHarborResponse(t *testing.T) {
	type payload struct {
		Name string `json:"name"`
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v2.0/example" {
			t.Errorf("request = %s %s, want POST /api/v2.0/example", r.Method, r.URL.Path)
		}
		if username, password, ok := r.BasicAuth(); !ok || username != "robot" || password != "token" {
			t.Errorf("BasicAuth = %q/%q/%t, want configured credentials", username, password, ok)
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", got)
		}
		var got payload
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Errorf("decode request body: %v", err)
		} else if got.Name != "team" {
			t.Errorf("request name = %q, want team", got.Name)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"name":"created"}`)
	}))
	defer server.Close()
	client := NewWithHTTPClient(server.URL+"/", "robot", "token", server.Client())
	if client.BaseURL != server.URL {
		t.Fatalf("NewWithHTTPClient BaseURL = %q, want trailing slash removed", client.BaseURL)
	}
	var out payload
	if err := client.post(context.Background(), "/api/v2.0/example", payload{Name: "team"}, &out); err != nil {
		t.Fatalf("post: %v", err)
	}
	if out.Name != "created" {
		t.Fatalf("decoded response = %#v, want created response", out)
	}
}

func TestDoReportsHarborErrorsAndInvalidResponses(t *testing.T) {
	t.Run("non-success body", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusConflict)
			_, _ = io.WriteString(w, "  object already exists \n")
		}))
		defer server.Close()
		var out map[string]any
		_, err := NewWithHTTPClient(server.URL, "", "", server.Client()).do(context.Background(), http.MethodGet, "/resource", nil, &out)
		var harborErr *HTTPError
		if !errors.As(err, &harborErr) || harborErr.StatusCode != http.StatusConflict || harborErr.Message != "object already exists" || !IsConflict(err) {
			t.Fatalf("non-success response error = %#v, want trimmed Harbor conflict", err)
		}
	})

	t.Run("malformed response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = io.WriteString(w, "not-json")
		}))
		defer server.Close()
		var out map[string]any
		_, err := NewWithHTTPClient(server.URL, "", "", server.Client()).do(context.Background(), http.MethodGet, "/resource", nil, &out)
		if err == nil || strings.Contains(err.Error(), "harbor API") {
			t.Fatalf("malformed success response error = %v, want JSON decode error", err)
		}
	})

	t.Run("empty success with response target", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }))
		defer server.Close()
		out := map[string]any{"unchanged": true}
		if _, err := NewWithHTTPClient(server.URL, "", "", server.Client()).do(context.Background(), http.MethodGet, "/resource", nil, &out); err != nil {
			t.Fatalf("204 response: %v", err)
		}
		if !reflect.DeepEqual(out, map[string]any{"unchanged": true}) {
			t.Fatalf("204 response modified decode target: %#v", out)
		}
	})

	if _, err := NewWithHTTPClient("http://example.test", "", "", http.DefaultClient).do(context.Background(), http.MethodGet, ":bad", nil, nil); err == nil {
		t.Fatal("invalid request URL was accepted")
	}
}

func TestGetPagedUsesTotalCountAndDoesNotMutateFilters(t *testing.T) {
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.RequestURI())
		if r.URL.Query().Get("q") != "name=harbor" || r.URL.Query().Get("page_size") != "100" {
			t.Errorf("pagination query = %v, want preserved filter and page size", r.URL.Query())
		}
		w.Header().Set("X-Total-Count", "3")
		if r.URL.Query().Get("page") == "1" {
			_, _ = io.WriteString(w, `[1,2]`)
			return
		}
		_, _ = io.WriteString(w, `[3]`)
	}))
	defer server.Close()
	filters := url.Values{"q": {"name=harbor"}, "label": {"one", "two"}}
	original := filters.Encode()
	got, err := getPaged[int](context.Background(), NewWithHTTPClient(server.URL, "", "", server.Client()), "/items", filters)
	if err != nil || !reflect.DeepEqual(got, []int{1, 2, 3}) {
		t.Fatalf("paged values = %#v, %v", got, err)
	}
	if filters.Encode() != original {
		t.Fatalf("getPaged mutated caller filters: %v", filters)
	}
	if len(paths) != 2 || !strings.Contains(paths[0], "page=1") || !strings.Contains(paths[1], "page=2") {
		t.Fatalf("page requests = %#v, want pages 1 and 2", paths)
	}
}

func TestGetPagedUsesFullPageFallbackWhenTotalIsUnavailable(t *testing.T) {
	var pages []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pages = append(pages, r.URL.Query().Get("page"))
		if r.URL.Query().Get("page") == "1" {
			values := make([]int, defaultListPageSize)
			for i := range values {
				values[i] = i
			}
			_ = json.NewEncoder(w).Encode(values)
			return
		}
		_, _ = io.WriteString(w, `[]`)
	}))
	defer server.Close()
	got, err := getPaged[int](context.Background(), NewWithHTTPClient(server.URL, "", "", server.Client()), "/items", nil)
	if err != nil || len(got) != defaultListPageSize {
		t.Fatalf("fallback page values length = %d, %v", len(got), err)
	}
	if !reflect.DeepEqual(pages, []string{"1", "2"}) {
		t.Fatalf("fallback requested pages = %#v, want [1 2]", pages)
	}
}

func TestCreateLocationIdentifiersAndDeleteAlreadyGone(t *testing.T) {
	t.Run("numeric location", func(t *testing.T) {
		var location string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			location = r.Header.Get("Content-Type")
			w.Header().Set("Location", "https://harbor.example/api/v2.0/projects/73")
			w.WriteHeader(http.StatusCreated)
		}))
		defer server.Close()
		id, err := NewWithHTTPClient(server.URL, "", "", server.Client()).createWithNumericLocationID(context.Background(), "/api/v2.0/projects", map[string]string{"name": "demo"})
		if err != nil || id != 73 || location != "application/json" {
			t.Fatalf("numeric create location result = %d, content-type %q, error %v", id, location, err)
		}
	})

	t.Run("string location", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Location", "/api/v2.0/scanners/scan%2Fuuid")
			w.WriteHeader(http.StatusCreated)
		}))
		defer server.Close()
		id, err := NewWithHTTPClient(server.URL, "", "", server.Client()).createWithStringLocationID(context.Background(), "/api/v2.0/scanners", map[string]string{"name": "scanner"})
		if err != nil || id != "scan%2Fuuid" {
			t.Fatalf("string create location result = %q, %v", id, err)
		}
	})

	for _, location := range []string{"", "/api/v2.0/projects/not-a-number"} {
		_, err := extractLocationID(&http.Response{Header: http.Header{"Location": []string{location}}})
		if err == nil {
			t.Fatalf("invalid numeric Location %q was accepted", location)
		}
	}
	if _, err := extractLocationIDString(&http.Response{Header: make(http.Header)}); err == nil {
		t.Fatal("missing string Location was accepted")
	}

	t.Run("delete treats not found as success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodDelete || r.URL.Path != "/resource" {
				t.Errorf("delete request = %s %s", r.Method, r.URL.Path)
			}
			http.NotFound(w, r)
		}))
		defer server.Close()
		if err := NewWithHTTPClient(server.URL, "", "", server.Client()).deleteIgnoringNotFound(context.Background(), "/resource"); err != nil {
			t.Fatalf("delete of missing resource: %v", err)
		}
	})

	t.Run("optional delete errors", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = io.WriteString(w, "retention already gone")
		}))
		defer server.Close()
		client := NewWithHTTPClient(server.URL, "", "", server.Client())
		ignoreRetentionGone := func(err error) bool {
			harborErr, ok := err.(*HTTPError)
			return ok && harborErr.Message == "retention already gone"
		}
		if err := client.deleteIgnoringNotFound(context.Background(), "/retention", ignoreRetentionGone); err != nil {
			t.Fatalf("optional known-gone error: %v", err)
		}
		if err := client.deleteIgnoringNotFound(context.Background(), "/retention"); err == nil || !strings.Contains(err.Error(), "retention already gone") {
			t.Fatalf("unrecognized delete error = %v, want propagated Harbor error", err)
		}
	})
}

func TestParseRobotIDAcceptsOnlyValidIntegers(t *testing.T) {
	for _, test := range []struct {
		value string
		want  int
	}{
		{value: "42", want: 42},
		{value: "0", want: 0},
		{value: "-7", want: -7},
	} {
		got, err := ParseRobotID(test.value)
		if err != nil || got != test.want {
			t.Fatalf("ParseRobotID(%q) = %d, %v, want %d", test.value, got, err, test.want)
		}
	}
	for _, value := range []string{"", "robot/42", "1.5", "999999999999999999999999999"} {
		if _, err := ParseRobotID(value); err == nil {
			t.Errorf("ParseRobotID(%q) accepted invalid ID", value)
		}
	}
}
