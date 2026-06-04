package redmine

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultTimeout = 30 * time.Second
	MaxLimit       = 100
)

var (
	ErrBaseURLRequired = errors.New("redmine: base url required")
	ErrAPIKeyRequired  = errors.New("redmine: api key required")
	ErrInvalidBaseURL  = errors.New("redmine: invalid base url")
	ErrNotFound        = errors.New("redmine: resource not found")
	ErrUpstream        = errors.New("redmine: upstream error")
)

type Options struct {
	BaseURL string
	APIKey  string
	Timeout time.Duration
}

type Client struct {
	base   *url.URL
	apiKey string
	hc     *http.Client
}

func New(opts Options) (*Client, error) {
	key := strings.TrimSpace(opts.APIKey)
	if key == "" {
		return nil, ErrAPIKeyRequired
	}
	raw := strings.TrimSpace(opts.BaseURL)
	if raw == "" {
		return nil, ErrBaseURLRequired
	}
	if !strings.HasSuffix(raw, "/") {
		raw += "/"
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "http" && u.Scheme != "https" || u.Host == "" {
		return nil, ErrInvalidBaseURL
	}
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	return &Client{
		base:   u,
		apiKey: key,
		hc:     &http.Client{Timeout: timeout},
	}, nil
}

type CurrentUser struct {
	ID        int32  `json:"id"`
	Login     string `json:"login"`
	FirstName string `json:"firstname"`
	LastName  string `json:"lastname"`
	Mail      string `json:"mail"`
	Admin     bool   `json:"admin"`
}

type Project struct {
	ID         int32  `json:"id"`
	Name       string `json:"name"`
	Identifier string `json:"identifier"`
}

type User struct {
	ID        int32  `json:"id"`
	Login     string `json:"login"`
	FirstName string `json:"firstname"`
	LastName  string `json:"lastname"`
	Mail      string `json:"mail"`
	Admin     bool   `json:"admin"`
}

type Version struct {
	ID          int32  `json:"id"`
	ProjectID   int32  `json:"project_id"`
	Name        string `json:"name"`
	Status      string `json:"status"`
	Description string `json:"description"`
}

type Status struct {
	ID       int32  `json:"id"`
	Name     string `json:"name"`
	IsClosed bool   `json:"is_closed"`
	Position int32  `json:"position"`
}

type Ref struct {
	ID   int32  `json:"id"`
	Name string `json:"name"`
}

type Issue struct {
	ID              int32    `json:"id"`
	ProjectID       int32    `json:"project_id"`
	Tracker         Ref      `json:"tracker"`
	Status          Ref      `json:"status"`
	Priority        Ref      `json:"priority"`
	Author          Ref      `json:"author"`
	AssignedTo      *Ref     `json:"assigned_to"`
	Subject         string   `json:"subject"`
	Description     string   `json:"description"`
	StartDate       string   `json:"start_date,omitempty"`
	DueDate         string   `json:"due_date,omitempty"`
	DoneRatio       int32    `json:"done_ratio"`
	EstimatedHours       float64  `json:"estimated_hours,omitempty"`
	TotalEstimatedHours  float64  `json:"total_estimated_hours,omitempty"`
	SpentHours           float64  `json:"spent_hours,omitempty"`
	TotalSpentHours      float64  `json:"total_spent_hours,omitempty"`
	CreatedOn       string   `json:"created_on"`
	UpdatedOn       string   `json:"updated_on"`
	ClosedOn        string   `json:"closed_on,omitempty"`
	FixedVersion    *Ref     `json:"fixed_version"`
	AllowedStatuses []Status  `json:"allowed_statuses,omitempty"`
	Journals        []Journal `json:"journals,omitempty"`
}

type JournalDetail struct {
	Property string `json:"property"`
	Name     string `json:"name"`
	OldValue string `json:"old_value"`
	NewValue string `json:"new_value"`
}

type Journal struct {
	ID        int32           `json:"id"`
	User      Ref             `json:"user"`
	Notes     string          `json:"notes"`
	CreatedOn string          `json:"created_on"`
	Details   []JournalDetail `json:"details"`
}

type IssueFilter struct {
	ProjectID   int32
	StatusID    string
	AssigneeID  int32
	VersionID   int32
	SubjectLike string
	UpdatedOn   string
}

func (c *Client) do(ctx context.Context, method, path string, query url.Values, body, out any) error {
	u := *c.base
	if u.Path == "" {
		u.Path = "/"
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/" + strings.TrimLeft(path, "/")
	u.RawQuery = ""
	if query != nil {
		u.RawQuery = query.Encode()
	}
	var reader io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("redmine: encode body: %w", err)
		}
		reader = bytes.NewReader(buf)
	}
	req, err := http.NewRequestWithContext(ctx, method, u.String(), reader)
	if err != nil {
		return fmt.Errorf("redmine: build request: %w", err)
	}
	req.Header.Set("X-Redmine-API-Key", c.apiKey)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
		if seeker, ok := reader.(*bytes.Reader); ok {
			data := make([]byte, seeker.Len())
			_, _ = seeker.Read(data)
			_, _ = seeker.Seek(0, io.SeekStart)
			req.GetBody = func() (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(data)), nil }
		}
	}
	resp, err := c.doWithRetry(req, body != nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusUnauthorized, resp.StatusCode == http.StatusForbidden:
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("%w: auth status=%d body=%s", ErrUpstream, resp.StatusCode, strings.TrimSpace(string(raw)))
	case resp.StatusCode == http.StatusNotFound:
		return ErrNotFound
	case resp.StatusCode < 200 || resp.StatusCode >= 300:
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("%w: status=%d body=%s", ErrUpstream, resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return fmt.Errorf("redmine: decode response: %w", err)
		}
	}
	return nil
}

func (c *Client) doWithRetry(req *http.Request, hasBody bool) (*http.Response, error) {
	const attempts = 3
	for attempt := 1; attempt <= attempts; attempt++ {
		clone := req.Clone(req.Context())
		if hasBody && req.Body != nil {
			if req.GetBody == nil {
				return nil, errors.New("redmine: retryable body missing GetBody")
			}
			body, err := req.GetBody()
			if err != nil {
				return nil, fmt.Errorf("redmine: clone body: %w", err)
			}
			clone.Body = body
		}
		resp, err := c.hc.Do(clone)
		if err == nil && !transientStatus(resp.StatusCode) {
			return resp, nil
		}
		if resp != nil {
			_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
			_ = resp.Body.Close()
		}
		if attempt == attempts || !retryableError(err) && (resp == nil || !transientStatus(resp.StatusCode)) {
			if err != nil {
				return nil, fmt.Errorf("redmine: request: %w", err)
			}
			return resp, nil
		}
		select {
		case <-req.Context().Done():
			return nil, fmt.Errorf("redmine: request: %w", req.Context().Err())
		case <-time.After(time.Duration(attempt*100) * time.Millisecond):
		}
	}
	return nil, errors.New("redmine: retry exhausted")
}

func transientStatus(status int) bool {
	return status == http.StatusTooManyRequests || status == http.StatusBadGateway || status == http.StatusServiceUnavailable || status == http.StatusGatewayTimeout
}

func retryableError(err error) bool {
	return err != nil
}

func pageQuery(limit, offset int) url.Values {
	if limit <= 0 {
		limit = 25
	}
	if limit > MaxLimit {
		limit = MaxLimit
	}
	if offset < 0 {
		offset = 0
	}
	return url.Values{
		"limit":  {strconv.Itoa(limit)},
		"offset": {strconv.Itoa(offset)},
	}
}

func walkAll[T any](ctx context.Context, fetch func(ctx context.Context, offset, limit int) ([]T, int, error)) ([]T, error) {
	var out []T
	offset := 0
	for {
		items, total, err := fetch(ctx, offset, MaxLimit)
		if err != nil {
			return nil, err
		}
		out = append(out, items...)
		if total <= 0 || offset+len(items) >= total || len(items) == 0 {
			return out, nil
		}
		offset += len(items)
	}
}

func (c *Client) CurrentUser(ctx context.Context) (CurrentUser, error) {
	var resp struct {
		User CurrentUser `json:"user"`
	}
	if err := c.do(ctx, http.MethodGet, "/users/current.json", nil, nil, &resp); err != nil {
		return CurrentUser{}, err
	}
	return resp.User, nil
}

func (c *Client) Projects(ctx context.Context) ([]Project, error) {
	return walkAll(ctx, func(ctx context.Context, offset, limit int) ([]Project, int, error) {
		var page struct {
			TotalCount int       `json:"total_count"`
			Projects   []Project `json:"projects"`
		}
		if err := c.do(ctx, http.MethodGet, "/projects.json", pageQuery(limit, offset), nil, &page); err != nil {
			return nil, 0, err
		}
		return page.Projects, page.TotalCount, nil
	})
}

func (c *Client) Users(ctx context.Context) ([]User, error) {
	return walkAll(ctx, func(ctx context.Context, offset, limit int) ([]User, int, error) {
		var page struct {
			TotalCount int    `json:"total_count"`
			Users      []User `json:"users"`
		}
		if err := c.do(ctx, http.MethodGet, "/users.json", pageQuery(limit, offset), nil, &page); err != nil {
			return nil, 0, err
		}
		return page.Users, page.TotalCount, nil
	})
}

type Member struct {
	ID   int32 `json:"id"`
	User *Ref  `json:"user"`
}

func (c *Client) Members(ctx context.Context, projectID int32) ([]Member, error) {
	path := "/projects/" + strconv.FormatInt(int64(projectID), 10) + "/memberships.json"
	return walkAll(ctx, func(ctx context.Context, offset, limit int) ([]Member, int, error) {
		var page struct {
			TotalCount  int      `json:"total_count"`
			Memberships []Member `json:"memberships"`
		}
		if err := c.do(ctx, http.MethodGet, path, pageQuery(limit, offset), nil, &page); err != nil {
			return nil, 0, err
		}
		return page.Memberships, page.TotalCount, nil
	})
}

func (c *Client) Versions(ctx context.Context, projectID int32) ([]Version, error) {
	path := "/projects/" + strconv.FormatInt(int64(projectID), 10) + "/versions.json"
	return walkAll(ctx, func(ctx context.Context, offset, limit int) ([]Version, int, error) {
		var page struct {
			TotalCount int       `json:"total_count"`
			Versions   []Version `json:"versions"`
		}
		if err := c.do(ctx, http.MethodGet, path, pageQuery(limit, offset), nil, &page); err != nil {
			return nil, 0, err
		}
		return page.Versions, page.TotalCount, nil
	})
}

func (c *Client) Statuses(ctx context.Context) ([]Status, error) {
	var resp struct {
		IssueStatuses []Status `json:"issue_statuses"`
	}
	if err := c.do(ctx, http.MethodGet, "/issue_statuses.json", nil, nil, &resp); err != nil {
		return nil, err
	}
	return resp.IssueStatuses, nil
}

func (c *Client) Issues(ctx context.Context, f IssueFilter) ([]Issue, error) {
	base := url.Values{"include": {"spent_time"}}
	if f.ProjectID > 0 {
		base.Set("project_id", strconv.FormatInt(int64(f.ProjectID), 10))
	}
	if f.StatusID != "" {
		base.Set("status_id", f.StatusID)
	}
	if f.AssigneeID > 0 {
		base.Set("assigned_to_id", strconv.FormatInt(int64(f.AssigneeID), 10))
	}
	if f.VersionID > 0 {
		base.Set("fixed_version_id", strconv.FormatInt(int64(f.VersionID), 10))
	}
	if f.SubjectLike != "" {
		base.Set("subject", "~"+f.SubjectLike)
	}
	if f.UpdatedOn != "" {
		base.Set("updated_on", f.UpdatedOn)
	}
	return walkAll(ctx, func(ctx context.Context, offset, limit int) ([]Issue, int, error) {
		q := cloneValues(base)
		q.Set("limit", strconv.Itoa(limit))
		q.Set("offset", strconv.Itoa(offset))
		var page struct {
			TotalCount int     `json:"total_count"`
			Issues     []Issue `json:"issues"`
		}
		if err := c.do(ctx, http.MethodGet, "/issues.json", q, nil, &page); err != nil {
			return nil, 0, err
		}
		return page.Issues, page.TotalCount, nil
	})
}

func cloneValues(src url.Values) url.Values {
	dst := make(url.Values, len(src))
	for k, v := range src {
		dst[k] = append([]string(nil), v...)
	}
	return dst
}

func (c *Client) Issue(ctx context.Context, id int32) (Issue, error) {
	q := url.Values{"include": {"allowed_statuses"}}
	var resp struct {
		Issue Issue `json:"issue"`
	}
	if err := c.do(ctx, http.MethodGet, "/issues/"+strconv.FormatInt(int64(id), 10)+".json", q, nil, &resp); err != nil {
		return Issue{}, err
	}
	return resp.Issue, nil
}

func (c *Client) IssueDetail(ctx context.Context, id int32) (Issue, error) {
	q := url.Values{"include": {"journals"}}
	var resp struct {
		Issue Issue `json:"issue"`
	}
	if err := c.do(ctx, http.MethodGet, "/issues/"+strconv.FormatInt(int64(id), 10)+".json", q, nil, &resp); err != nil {
		return Issue{}, err
	}
	return resp.Issue, nil
}

func (c *Client) AddComment(ctx context.Context, id int32, notes string) error {
	payload := struct {
		Issue struct {
			Notes string `json:"notes"`
		} `json:"issue"`
	}{}
	payload.Issue.Notes = notes
	return c.do(ctx, http.MethodPut, "/issues/"+strconv.FormatInt(int64(id), 10)+".json", nil, payload, nil)
}

func (c *Client) UpdateIssueStatus(ctx context.Context, id, statusID int32) error {
	payload := struct {
		Issue struct {
			StatusID int32 `json:"status_id"`
		} `json:"issue"`
	}{}
	payload.Issue.StatusID = statusID
	return c.do(ctx, http.MethodPut, "/issues/"+strconv.FormatInt(int64(id), 10)+".json", nil, payload, nil)
}
