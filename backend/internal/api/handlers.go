package api

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"vermilion/internal/db"
	"vermilion/internal/redmine"

	"github.com/go-chi/chi/v5"
)

type Handlers struct {
	Repo *db.Repo
}

func (h *Handlers) RegisterRoutes(r chi.Router) {
	r.Get("/healthz", Healthz)

	r.Route("/api", func(r chi.Router) {
		r.Get("/projects", h.handleListProjects)
		r.Get("/board", h.handleGetBoard)
		r.Get("/board/statuses", h.handleListStatuses)
		r.Put("/issues/{issueID}/status", h.handleUpdateIssueStatus)
		r.Get("/issues/{issueID}", h.handleGetIssue)
		r.Post("/issues/{issueID}/comments", h.handleAddComment)
	})
}

// --- header extraction ---

type reqContext struct {
	redmineURL        string
	apiKey            string
	projectIdentifier string
}

func extractHeaders(r *http.Request) (reqContext, error) {
	rc := reqContext{
		redmineURL:        strings.TrimSpace(r.Header.Get("X-Redmine-URL")),
		apiKey:            strings.TrimSpace(r.Header.Get("X-Redmine-API-Key")),
		projectIdentifier: strings.TrimSpace(r.Header.Get("X-Redmine-Project-Identifier")),
	}
	if rc.apiKey == "" {
		return rc, errors.New("missing X-Redmine-API-Key header")
	}
	if rc.redmineURL == "" {
		return rc, errors.New("missing X-Redmine-URL header")
	}
	return rc, nil
}

func translateLocalhost(rawURL string) string {
	if strings.ToLower(strings.TrimSpace(os.Getenv("RUNNING_IN_CONTAINER"))) != "true" {
		return rawURL
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	host := u.Hostname()
	if host == "localhost" || host == "127.0.0.1" {
		port := u.Port()
		if port == "" || port == "3000" {
			u.Host = "redmine:3000"
		} else {
			u.Host = "redmine:" + port
		}
	}
	return u.String()
}

func newRedmineClient(rc reqContext) (*redmine.Client, error) {
	baseURL := translateLocalhost(rc.redmineURL)
	return redmine.New(redmine.Options{
		BaseURL: baseURL,
		APIKey:  rc.apiKey,
	})
}

func cacheKey(redmineURL, projectIdentifier string) string {
	h := sha256.Sum256([]byte(redmineURL + projectIdentifier))
	return fmt.Sprintf("%x", h)
}

// --- board payload types ---

type BoardProject struct {
	ID         int32  `json:"id"`
	Name       string `json:"name"`
	Identifier string `json:"identifier"`
}

type BoardStatus struct {
	ID       int32  `json:"id"`
	Name     string `json:"name"`
	IsClosed bool   `json:"isClosed"`
}

type BoardMember struct {
	ID   int32  `json:"id"`
	Name string `json:"name"`
}

type BoardVersion struct {
	ID   int32  `json:"id"`
	Name string `json:"name"`
}

type BoardIssue struct {
	ID             int32   `json:"id"`
	Subject        string  `json:"subject"`
	StatusID       int32   `json:"statusId"`
	AssignedToID   *int32  `json:"assignedToId"`
	AssignedToName string  `json:"assignedToName"`
	UpdatedOn      string  `json:"updatedOn"`
	VersionID      *int32  `json:"versionId"`
	VersionName    string  `json:"versionName"`
	SpentHours     float64 `json:"spentHours"`
	EstimatedHours float64 `json:"estimatedHours"`
}

type BoardPayload struct {
	Project       BoardProject  `json:"project"`
	Statuses      []BoardStatus `json:"statuses"`
	Members       []BoardMember `json:"members"`
	Versions      []BoardVersion `json:"versions"`
	Issues        []BoardIssue  `json:"issues"`
	CachedAt      string        `json:"cachedAt"`
	Stale         bool          `json:"stale"`
	CurrentUserID *int32        `json:"currentUserId,omitempty"`
}

// --- GET /api/projects ---

type ProjectItem struct {
	ID         int32  `json:"id"`
	Name       string `json:"name"`
	Identifier string `json:"identifier"`
}

func (h *Handlers) handleListProjects(w http.ResponseWriter, r *http.Request) {
	rc, err := extractHeaders(r)
	if err != nil {
		Unauthorized(w, err.Error())
		return
	}
	client, err := newRedmineClient(rc)
	if err != nil {
		BadRequest(w, "invalid Redmine URL: "+err.Error(), nil)
		return
	}
	projects, err := client.Projects(r.Context())
	if err != nil {
		if errors.Is(err, redmine.ErrUpstream) {
			Unauthorized(w, "Redmine rejected credentials: "+err.Error())
			return
		}
		Internal(w, err)
		return
	}
	out := make([]ProjectItem, len(projects))
	for i, p := range projects {
		out[i] = ProjectItem{ID: p.ID, Name: p.Name, Identifier: p.Identifier}
	}
	WriteJSON(w, http.StatusOK, out)
}

// --- GET /api/board ---

const (
	cacheFresh = 2 * time.Minute
	cacheMax   = 10 * time.Minute
)

func (h *Handlers) handleGetBoard(w http.ResponseWriter, r *http.Request) {
	rc, err := extractHeaders(r)
	if err != nil {
		Unauthorized(w, err.Error())
		return
	}
	project := strings.TrimSpace(r.URL.Query().Get("project"))
	if project == "" {
		project = rc.projectIdentifier
	}
	if project == "" {
		BadRequest(w, "missing project query parameter or X-Redmine-Project-Identifier header", nil)
		return
	}
	rc.projectIdentifier = project

	key := cacheKey(rc.redmineURL, project)
	force := r.URL.Query().Get("force") == "true"

	// Try cache (skipped when force=true)
	if !force && h.Repo != nil {
		entry, err := h.Repo.GetCache(r.Context(), key)
		if err == nil {
			age := time.Since(entry.CachedAt)
			if age < cacheMax {
				stale := age >= cacheFresh
				if stale {
					// refresh in background
					go h.refreshBoard(rc, key)
				}
				// Return cached data with stale flag
				var payload BoardPayload
				if json.Unmarshal(entry.Payload, &payload) == nil {
					payload.Stale = stale
					payload.CachedAt = entry.CachedAt.UTC().Format(time.RFC3339)
					WriteJSON(w, http.StatusOK, payload)
					return
				}
			}
		}
	}

	// Fetch fresh
	fetchCtx, fetchCancel := context.WithTimeout(r.Context(), 25*time.Second)
	defer fetchCancel()
	payload, err := h.fetchBoard(fetchCtx, rc)
	if err != nil {
		if errors.Is(err, redmine.ErrUpstream) {
			Unauthorized(w, "Redmine error: "+err.Error())
			return
		}
		Internal(w, err)
		return
	}

	// Cache it
	if h.Repo != nil {
		data, _ := json.Marshal(payload)
		if cacheErr := h.Repo.SetCache(r.Context(), key, data); cacheErr != nil {
			log.Printf("vermilion: cache write error: %v", cacheErr)
		}
	}

	payload.CachedAt = time.Now().UTC().Format(time.RFC3339)
	payload.Stale = false
	WriteJSON(w, http.StatusOK, payload)
}

func (h *Handlers) refreshBoard(rc reqContext, key string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	payload, err := h.fetchBoard(ctx, rc)
	if err != nil {
		log.Printf("vermilion: background refresh error: %v", err)
		return
	}
	data, _ := json.Marshal(payload)
	if err := h.Repo.SetCache(ctx, key, data); err != nil {
		log.Printf("vermilion: background cache write error: %v", err)
	}
}

func (h *Handlers) fetchBoard(ctx context.Context, rc reqContext) (BoardPayload, error) {
	client, err := newRedmineClient(rc)
	if err != nil {
		return BoardPayload{}, err
	}

	// Find project by identifier
	log.Printf("vermilion: fetchBoard: fetching projects list to resolve identifier %q", rc.projectIdentifier)
	projects, err := client.Projects(ctx)
	if err != nil {
		return BoardPayload{}, fmt.Errorf("fetch projects: %w", err)
	}
	var proj *redmine.Project
	for i := range projects {
		if projects[i].Identifier == rc.projectIdentifier {
			proj = &projects[i]
			break
		}
	}
	if proj == nil {
		return BoardPayload{}, fmt.Errorf("project %q not found", rc.projectIdentifier)
	}
	log.Printf("vermilion: fetchBoard: resolved project %q -> id=%d; starting parallel fetches", rc.projectIdentifier, proj.ID)

	type statusesResult struct {
		statuses []redmine.Status
		err      error
	}
	type issuesResult struct {
		issues []redmine.Issue
		err    error
	}
	type membersResult struct {
		members []redmine.Member
		err     error
	}
	type versionsResult struct {
		versions []redmine.Version
		err      error
	}

	type currentUserResult struct {
		user redmine.CurrentUser
		err  error
	}

	var wg sync.WaitGroup
	var sr statusesResult
	var ir issuesResult
	var mr membersResult
	var vr versionsResult
	var cur currentUserResult

	wg.Add(5)
	go func() {
		defer wg.Done()
		t := time.Now()
		sr.statuses, sr.err = client.Statuses(ctx)
		log.Printf("vermilion: fetchBoard: statuses done in %s (err=%v)", time.Since(t), sr.err)
	}()
	go func() {
		defer wg.Done()
		t := time.Now()
		ir.issues, ir.err = client.Issues(ctx, redmine.IssueFilter{
			ProjectID: proj.ID,
			StatusID:  "*",
		})
		log.Printf("vermilion: fetchBoard: issues done in %s (count=%d, err=%v)", time.Since(t), len(ir.issues), ir.err)
	}()
	go func() {
		defer wg.Done()
		t := time.Now()
		mr.members, mr.err = client.Members(ctx, proj.ID)
		log.Printf("vermilion: fetchBoard: members done in %s (count=%d, err=%v)", time.Since(t), len(mr.members), mr.err)
	}()
	go func() {
		defer wg.Done()
		t := time.Now()
		vr.versions, vr.err = client.Versions(ctx, proj.ID)
		log.Printf("vermilion: fetchBoard: versions done in %s (count=%d, err=%v)", time.Since(t), len(vr.versions), vr.err)
	}()
	go func() {
		defer wg.Done()
		t := time.Now()
		cur.user, cur.err = client.CurrentUser(ctx)
		log.Printf("vermilion: fetchBoard: current user done in %s (err=%v)", time.Since(t), cur.err)
	}()
	wg.Wait()
	log.Printf("vermilion: fetchBoard: all parallel fetches complete")

	if sr.err != nil {
		return BoardPayload{}, fmt.Errorf("fetch statuses: %w", sr.err)
	}
	if ir.err != nil {
		return BoardPayload{}, fmt.Errorf("fetch issues: %w", ir.err)
	}
	if mr.err != nil {
		log.Printf("vermilion: fetchBoard: members error (non-fatal): %v", mr.err)
	}
	if vr.err != nil {
		log.Printf("vermilion: fetchBoard: versions error (non-fatal): %v", vr.err)
	}

	statuses := make([]BoardStatus, len(sr.statuses))
	for i, s := range sr.statuses {
		statuses[i] = BoardStatus{ID: s.ID, Name: s.Name, IsClosed: s.IsClosed}
	}

	members := make([]BoardMember, 0, len(mr.members))
	for _, m := range mr.members {
		if m.User != nil {
			members = append(members, BoardMember{ID: m.User.ID, Name: m.User.Name})
		}
	}

	versions := make([]BoardVersion, len(vr.versions))
	for i, v := range vr.versions {
		versions[i] = BoardVersion{ID: v.ID, Name: v.Name}
	}

	issues := make([]BoardIssue, len(ir.issues))
	for i, iss := range ir.issues {
		bi := BoardIssue{
			ID:             iss.ID,
			Subject:        iss.Subject,
			StatusID:       iss.Status.ID,
			UpdatedOn:      iss.UpdatedOn,
			SpentHours:     iss.TotalSpentHours,
			EstimatedHours: iss.EstimatedHours,
		}
		if iss.AssignedTo != nil {
			id := iss.AssignedTo.ID
			bi.AssignedToID = &id
			bi.AssignedToName = iss.AssignedTo.Name
		}
		if iss.FixedVersion != nil {
			id := iss.FixedVersion.ID
			bi.VersionID = &id
			bi.VersionName = iss.FixedVersion.Name
		}
		issues[i] = bi
	}

	payload := BoardPayload{
		Project: BoardProject{
			ID:         proj.ID,
			Name:       proj.Name,
			Identifier: proj.Identifier,
		},
		Statuses: statuses,
		Members:  members,
		Versions: versions,
		Issues:   issues,
	}
	if cur.err == nil {
		payload.CurrentUserID = &cur.user.ID
	} else {
		log.Printf("vermilion: fetchBoard: current user error (non-fatal): %v", cur.err)
	}
	return payload, nil
}

// --- PUT /api/issues/:id/status ---

type UpdateStatusRequest struct {
	StatusID int `json:"statusId"`
}

func (h *Handlers) handleUpdateIssueStatus(w http.ResponseWriter, r *http.Request) {
	rc, err := extractHeaders(r)
	if err != nil {
		Unauthorized(w, err.Error())
		return
	}
	idStr := chi.URLParam(r, "issueID")
	issueID, err := strconv.ParseInt(idStr, 10, 32)
	if err != nil || issueID <= 0 {
		BadRequest(w, "invalid issue ID", nil)
		return
	}
	var req UpdateStatusRequest
	if !DecodeJSON(w, r, &req) {
		return
	}
	if req.StatusID <= 0 {
		BadRequest(w, "statusId must be a positive integer", nil)
		return
	}

	client, err := newRedmineClient(rc)
	if err != nil {
		BadRequest(w, "invalid Redmine URL: "+err.Error(), nil)
		return
	}
	if err := client.UpdateIssueStatus(r.Context(), int32(issueID), int32(req.StatusID)); err != nil {
		if errors.Is(err, redmine.ErrNotFound) {
			NotFound(w, "issue not found")
			return
		}
		if errors.Is(err, redmine.ErrUpstream) {
			WriteError(w, http.StatusBadGateway, APIError{Code: CodeUnavailable, Message: "Redmine error: " + err.Error()})
			return
		}
		Internal(w, err)
		return
	}

	if h.Repo != nil {
		key := cacheKey(rc.redmineURL, rc.projectIdentifier)
		if err := h.Repo.DeleteCache(r.Context(), key); err != nil {
			log.Printf("vermilion: cache invalidation error: %v", err)
		}
	}

	// Fetch updated issue to return
	issue, err := client.Issue(r.Context(), int32(issueID))
	if err != nil {
		Internal(w, err)
		return
	}

	bi := BoardIssue{
		ID:        issue.ID,
		Subject:   issue.Subject,
		StatusID:  issue.Status.ID,
		UpdatedOn: issue.UpdatedOn,
	}
	if issue.AssignedTo != nil {
		id := issue.AssignedTo.ID
		bi.AssignedToID = &id
		bi.AssignedToName = issue.AssignedTo.Name
	}
	if issue.FixedVersion != nil {
		id := issue.FixedVersion.ID
		bi.VersionID = &id
		bi.VersionName = issue.FixedVersion.Name
	}
	WriteJSON(w, http.StatusOK, bi)
}

// --- GET /api/board/statuses ---

func (h *Handlers) handleListStatuses(w http.ResponseWriter, r *http.Request) {
	rc, err := extractHeaders(r)
	if err != nil {
		Unauthorized(w, err.Error())
		return
	}
	client, err := newRedmineClient(rc)
	if err != nil {
		BadRequest(w, "invalid Redmine URL: "+err.Error(), nil)
		return
	}
	statuses, err := client.Statuses(r.Context())
	if err != nil {
		Internal(w, err)
		return
	}
	type statusItem struct {
		ID       int32  `json:"id"`
		Name     string `json:"name"`
		IsClosed bool   `json:"isClosed"`
	}
	out := make([]statusItem, len(statuses))
	for i, s := range statuses {
		out[i] = statusItem{ID: s.ID, Name: s.Name, IsClosed: s.IsClosed}
	}
	WriteJSON(w, http.StatusOK, out)
}

// --- GET /api/issues/:id ---

type JournalDetailDTO struct {
	Property string `json:"property"`
	Name     string `json:"name"`
	OldValue string `json:"oldValue"`
	NewValue string `json:"newValue"`
}

type JournalDTO struct {
	ID        int32              `json:"id"`
	UserName  string             `json:"userName"`
	Notes     string             `json:"notes"`
	CreatedOn string             `json:"createdOn"`
	Details   []JournalDetailDTO `json:"details"`
}

type IssueDetailDTO struct {
	ID             int32        `json:"id"`
	Subject        string       `json:"subject"`
	Description    string       `json:"description"`
	StatusID       int32        `json:"statusId"`
	StatusName     string       `json:"statusName"`
	AssignedToID   *int32       `json:"assignedToId"`
	AssignedToName string       `json:"assignedToName"`
	UpdatedOn      string       `json:"updatedOn"`
	CreatedOn      string       `json:"createdOn"`
	VersionID      *int32       `json:"versionId"`
	VersionName    string       `json:"versionName"`
	SpentHours     float64      `json:"spentHours"`
	EstimatedHours float64      `json:"estimatedHours"`
	Journals       []JournalDTO `json:"journals"`
}

func issueToDetailDTO(issue redmine.Issue) IssueDetailDTO {
	dto := IssueDetailDTO{
		ID:             issue.ID,
		Subject:        issue.Subject,
		Description:    issue.Description,
		StatusID:       issue.Status.ID,
		StatusName:     issue.Status.Name,
		UpdatedOn:      issue.UpdatedOn,
		CreatedOn:      issue.CreatedOn,
		SpentHours:     issue.SpentHours,
		EstimatedHours: issue.EstimatedHours,
	}
	if issue.AssignedTo != nil {
		id := issue.AssignedTo.ID
		dto.AssignedToID = &id
		dto.AssignedToName = issue.AssignedTo.Name
	}
	if issue.FixedVersion != nil {
		id := issue.FixedVersion.ID
		dto.VersionID = &id
		dto.VersionName = issue.FixedVersion.Name
	}
	for _, j := range issue.Journals {
		jdto := JournalDTO{
			ID:        j.ID,
			UserName:  j.User.Name,
			Notes:     j.Notes,
			CreatedOn: j.CreatedOn,
		}
		for _, d := range j.Details {
			jdto.Details = append(jdto.Details, JournalDetailDTO{
				Property: d.Property,
				Name:     d.Name,
				OldValue: d.OldValue,
				NewValue: d.NewValue,
			})
		}
		dto.Journals = append(dto.Journals, jdto)
	}
	return dto
}

func (h *Handlers) handleGetIssue(w http.ResponseWriter, r *http.Request) {
	rc, err := extractHeaders(r)
	if err != nil {
		Unauthorized(w, err.Error())
		return
	}
	idStr := chi.URLParam(r, "issueID")
	issueID, err := strconv.ParseInt(idStr, 10, 32)
	if err != nil || issueID <= 0 {
		BadRequest(w, "invalid issue ID", nil)
		return
	}
	client, err := newRedmineClient(rc)
	if err != nil {
		BadRequest(w, "invalid Redmine URL: "+err.Error(), nil)
		return
	}
	issue, err := client.IssueDetail(r.Context(), int32(issueID))
	if err != nil {
		if errors.Is(err, redmine.ErrNotFound) {
			NotFound(w, "issue not found")
			return
		}
		Internal(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, issueToDetailDTO(issue))
}

// --- POST /api/issues/:id/comments ---

func (h *Handlers) handleAddComment(w http.ResponseWriter, r *http.Request) {
	rc, err := extractHeaders(r)
	if err != nil {
		Unauthorized(w, err.Error())
		return
	}
	idStr := chi.URLParam(r, "issueID")
	issueID, err := strconv.ParseInt(idStr, 10, 32)
	if err != nil || issueID <= 0 {
		BadRequest(w, "invalid issue ID", nil)
		return
	}
	var req struct {
		Notes string `json:"notes"`
	}
	if !DecodeJSON(w, r, &req) {
		return
	}
	if strings.TrimSpace(req.Notes) == "" {
		BadRequest(w, "notes cannot be empty", nil)
		return
	}
	client, err := newRedmineClient(rc)
	if err != nil {
		BadRequest(w, "invalid Redmine URL: "+err.Error(), nil)
		return
	}
	if err := client.AddComment(r.Context(), int32(issueID), req.Notes); err != nil {
		Internal(w, err)
		return
	}
	issue, err := client.IssueDetail(r.Context(), int32(issueID))
	if err != nil {
		Internal(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, issueToDetailDTO(issue))
}
