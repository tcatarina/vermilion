package redmine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"
)

func newTestClient(t *testing.T, ts *httptest.Server, apiKey string) *Client {
	t.Helper()
	c, err := New(Options{BaseURL: ts.URL, APIKey: apiKey, Timeout: 5 * time.Second})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return c
}

func TestNew_InvalidOptions(t *testing.T) {
	cases := []struct {
		name string
		opts Options
	}{
		{"empty base url", Options{BaseURL: "", APIKey: "k"}},
		{"missing api key", Options{BaseURL: "https://example.com", APIKey: ""}},
		{"whitespace api key", Options{BaseURL: "https://example.com", APIKey: "   "}},
		{"base url without scheme", Options{BaseURL: "example.com/foo", APIKey: "k"}},
		{"unsupported scheme", Options{BaseURL: "ftp://example.com", APIKey: "k"}},
		{"base url without host", Options{BaseURL: "http:///path", APIKey: "k"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, err := New(tc.opts)
			if err == nil {
				t.Fatalf("expected error, got client=%+v", c)
			}
		})
	}
}

func TestNew_ValidOptions(t *testing.T) {
	c, err := New(Options{BaseURL: "https://redmine.example.com/", APIKey: "k"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c == nil {
		t.Fatal("expected non-nil client")
	}
}

func TestClient_CurrentUser_SendsAuthHeader(t *testing.T) {
	var gotKey, gotAccept, gotPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.Header.Get("X-Redmine-API-Key")
		gotAccept = r.Header.Get("Accept")
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"user":{"id":1,"login":"alice","firstname":"A","lastname":"B","mail":"a@b.c","admin":true}}`)
	}))
	defer ts.Close()

	c := newTestClient(t, ts, "secret-api-key")
	u, err := c.CurrentUser(context.Background())
	if err != nil {
		t.Fatalf("CurrentUser: %v", err)
	}
	if gotKey != "secret-api-key" {
		t.Fatalf("expected X-Redmine-API-Key=secret-api-key, got %q", gotKey)
	}
	if !strings.HasPrefix(gotAccept, "application/json") {
		t.Fatalf("expected Accept=application/json, got %q", gotAccept)
	}
	if gotPath != "/users/current.json" {
		t.Fatalf("expected /users/current.json, got %q", gotPath)
	}
	if u.ID != 1 || u.Login != "alice" || !u.Admin {
		t.Fatalf("unexpected user: %+v", u)
	}
}

func TestClient_Projects_PaginatesUntilExhausted(t *testing.T) {
	const total = 5
	const pageSize = 2
	requests := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		end := offset + pageSize
		if end > total {
			end = total
		}
		items := make([]map[string]any, 0, end-offset)
		for i := offset; i < end; i++ {
			items = append(items, map[string]any{
				"id":         i + 1,
				"name":       fmt.Sprintf("p%d", i+1),
				"identifier": fmt.Sprintf("p-%d", i+1),
			})
		}
		resp := map[string]any{
			"total_count": total,
			"offset":      offset,
			"limit":       pageSize,
			"projects":    items,
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	c := newTestClient(t, ts, "k")
	projects, err := c.Projects(context.Background())
	if err != nil {
		t.Fatalf("Projects: %v", err)
	}
	if len(projects) != total {
		t.Fatalf("expected %d projects, got %d", total, len(projects))
	}
	for i, p := range projects {
		if p.ID != int32(i+1) {
			t.Fatalf("project[%d].ID=%d, want %d", i, p.ID, i+1)
		}
		if p.Identifier != fmt.Sprintf("p-%d", i+1) {
			t.Fatalf("project[%d].Identifier=%q, want p-%d", i, p.Identifier, i+1)
		}
	}
	if requests < 3 {
		t.Fatalf("expected at least 3 paginated requests, got %d", requests)
	}
}

func TestClient_Users_PaginatesAndDecodes(t *testing.T) {
	const total = 3
	const pageSize = 2
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		end := offset + pageSize
		if end > total {
			end = total
		}
		items := []map[string]any{}
		for i := offset; i < end; i++ {
			items = append(items, map[string]any{
				"id":        i + 1,
				"login":     fmt.Sprintf("u%d", i+1),
				"firstname": "F",
				"lastname":  "L",
				"mail":      fmt.Sprintf("u%d@x", i+1),
				"admin":     false,
			})
		}
		resp := map[string]any{
			"total_count": total,
			"offset":      offset,
			"limit":       pageSize,
			"users":       items,
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	c := newTestClient(t, ts, "k")
	users, err := c.Users(context.Background())
	if err != nil {
		t.Fatalf("Users: %v", err)
	}
	if len(users) != total {
		t.Fatalf("expected %d users, got %d", total, len(users))
	}
}

func TestClient_Versions_UsesProjectPath(t *testing.T) {
	var gotPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		resp := map[string]any{
			"total_count": 1,
			"offset":      0,
			"limit":       100,
			"versions": []map[string]any{{
				"id":         11,
				"project_id": 7,
				"name":       "v1.0",
				"status":     "open",
			}},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	c := newTestClient(t, ts, "k")
	vs, err := c.Versions(context.Background(), 7)
	if err != nil {
		t.Fatalf("Versions: %v", err)
	}
	if gotPath != "/projects/7/versions.json" {
		t.Fatalf("expected /projects/7/versions.json, got %q", gotPath)
	}
	if len(vs) != 1 || vs[0].Name != "v1.0" {
		t.Fatalf("unexpected versions: %+v", vs)
	}
}

func TestClient_Statuses_NoPagination(t *testing.T) {
	var requests int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		resp := map[string]any{
			"issue_statuses": []map[string]any{
				{"id": 1, "name": "New", "is_closed": false, "position": 1},
				{"id": 2, "name": "Closed", "is_closed": true, "position": 2},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	c := newTestClient(t, ts, "k")
	statuses, err := c.Statuses(context.Background())
	if err != nil {
		t.Fatalf("Statuses: %v", err)
	}
	if requests != 1 {
		t.Fatalf("expected exactly 1 request for Statuses, got %d", requests)
	}
	if len(statuses) != 2 {
		t.Fatalf("expected 2 statuses, got %d", len(statuses))
	}
	if !statuses[1].IsClosed {
		t.Fatalf("expected second status to be closed")
	}
}

func TestClient_Issues_AppliesFilterAndPaginates(t *testing.T) {
	const total = 5
	const pageSize = 2
	requests := 0
	var firstQuery url.Values
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		q := r.URL.Query()
		if requests == 1 {
			firstQuery = q
		}
		offset, _ := strconv.Atoi(q.Get("offset"))
		end := offset + pageSize
		if end > total {
			end = total
		}
		items := []map[string]any{}
		for i := offset; i < end; i++ {
			items = append(items, map[string]any{
				"id":         i + 1,
				"project_id": 7,
				"subject":    fmt.Sprintf("issue-%d", i+1),
				"status":     map[string]any{"id": 1, "name": "New"},
			})
		}
		resp := map[string]any{
			"total_count": total,
			"offset":      offset,
			"limit":       pageSize,
			"issues":      items,
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	c := newTestClient(t, ts, "k")
	issues, err := c.Issues(context.Background(), IssueFilter{ProjectID: 7, StatusID: "open"})
	if err != nil {
		t.Fatalf("Issues: %v", err)
	}
	if len(issues) != total {
		t.Fatalf("expected %d issues, got %d", total, len(issues))
	}
	if requests < 3 {
		t.Fatalf("expected at least 3 paginated requests, got %d", requests)
	}
	if firstQuery.Get("project_id") != "7" {
		t.Fatalf("expected project_id=7, got %q", firstQuery.Get("project_id"))
	}
	if firstQuery.Get("status_id") != "open" {
		t.Fatalf("expected status_id=open, got %q", firstQuery.Get("status_id"))
	}
}

func TestClient_Issue_RequestsAllowedStatuses(t *testing.T) {
	var gotInclude, gotPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotInclude = r.URL.Query().Get("include")
		gotPath = r.URL.Path
		body := `{"issue":{"id":42,"project_id":1,"subject":"x","status":{"id":1,"name":"New"},"allowed_statuses":[{"id":1,"name":"Open","is_closed":false,"position":1},{"id":2,"name":"Closed","is_closed":true,"position":2}]}}`
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, body)
	}))
	defer ts.Close()

	c := newTestClient(t, ts, "k")
	issue, err := c.Issue(context.Background(), 42)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if gotPath != "/issues/42.json" {
		t.Fatalf("expected /issues/42.json, got %q", gotPath)
	}
	if gotInclude != "allowed_statuses" {
		t.Fatalf("expected include=allowed_statuses, got %q", gotInclude)
	}
	if len(issue.AllowedStatuses) != 2 {
		t.Fatalf("expected 2 allowed statuses, got %d", len(issue.AllowedStatuses))
	}
	if issue.AllowedStatuses[0].ID != 1 || issue.AllowedStatuses[0].Name != "Open" {
		t.Fatalf("unexpected first status: %+v", issue.AllowedStatuses[0])
	}
	if !issue.AllowedStatuses[1].IsClosed {
		t.Fatalf("expected second status is_closed=true")
	}
}

func TestClient_UpdateIssueStatus_StatusOnlyPUTBody(t *testing.T) {
	var gotMethod, gotPath, gotContentType string
	var gotBody []byte
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotContentType = r.Header.Get("Content-Type")
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	c := newTestClient(t, ts, "k")
	if err := c.UpdateIssueStatus(context.Background(), 42, 5); err != nil {
		t.Fatalf("UpdateIssueStatus: %v", err)
	}
	if gotMethod != http.MethodPut {
		t.Fatalf("expected PUT, got %s", gotMethod)
	}
	if gotPath != "/issues/42.json" {
		t.Fatalf("expected /issues/42.json, got %q", gotPath)
	}
	if !strings.HasPrefix(gotContentType, "application/json") {
		t.Fatalf("expected application/json content type, got %q", gotContentType)
	}
	var payload struct {
		Issue map[string]json.RawMessage `json:"issue"`
	}
	if err := json.Unmarshal(gotBody, &payload); err != nil {
		t.Fatalf("invalid json body %q: %v", gotBody, err)
	}
	if len(payload.Issue) != 1 {
		t.Fatalf("expected issue body to have exactly 1 field, got %d: %s", len(payload.Issue), gotBody)
	}
	raw, ok := payload.Issue["status_id"]
	if !ok {
		t.Fatalf("expected status_id in body, got: %s", gotBody)
	}
	if string(raw) != "5" {
		t.Fatalf("expected status_id=5, got %s", string(raw))
	}
}

func TestClient_Non2xxReturnsUpstreamError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"errors":["nope"]}`, http.StatusUnprocessableEntity)
	}))
	defer ts.Close()

	c := newTestClient(t, ts, "k")
	_, err := c.CurrentUser(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ErrUpstream) {
		t.Fatalf("expected ErrUpstream, got %v", err)
	}
}

func TestClient_NotFoundSentinel(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer ts.Close()

	c := newTestClient(t, ts, "k")
	_, err := c.Issue(context.Background(), 999)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestClient_ErrorDoesNotLeakAPIKey(t *testing.T) {
	const key = "supersecret-redmine-api-key-xyz"
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusBadGateway)
	}))
	defer ts.Close()

	c := newTestClient(t, ts, key)
	_, err := c.CurrentUser(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if strings.Contains(err.Error(), key) {
		t.Fatalf("error leaks api key: %v", err)
	}
}

func TestClient_RetriesTransientFailures(t *testing.T) {
	requests := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if requests < 3 {
			http.Error(w, "temporary", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"issue_statuses":[{"id":1,"name":"New","is_closed":false,"position":1}]}`)
	}))
	defer ts.Close()

	c := newTestClient(t, ts, "k")
	statuses, err := c.Statuses(context.Background())
	if err != nil {
		t.Fatalf("Statuses: %v", err)
	}
	if len(statuses) != 1 || requests != 3 {
		t.Fatalf("expected one status after 3 attempts, statuses=%d requests=%d", len(statuses), requests)
	}
}
