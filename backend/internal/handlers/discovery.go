package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"time"

	"syndra/internal/db"
	"syndra/internal/directory"
	"syndra/internal/zitadel"
)

// --- Request types ---

type assignGrantRequest struct {
	ProjectID string   `json:"projectId"`
	RoleKeys  []string `json:"roleKeys"`
}

type updateGrantRequest struct {
	RoleKeys []string `json:"roleKeys"`
}

type createProjectRoleRequest struct {
	RoleKey     string `json:"roleKey"`
	DisplayName string `json:"displayName"`
	Group       string `json:"group"`
}

type updateProjectRoleRequest struct {
	DisplayName string `json:"displayName"`
	Group       string `json:"group"`
}

// paginatedResponse wraps a search result with explicit pagination metadata so
// the consumer always knows whether results are truncated.
//
// ObservedAt/Complete are set only by the two grants endpoints, which observe
// rather than list live (see observationFields) — every other paginated
// endpoint still lists Zitadel directly and leaves them nil, which omitempty
// drops rather than rendering as a false "not checked yet".
type paginatedResponse struct {
	Items      any        `json:"items"`
	Total      int        `json:"total"`
	Limit      int        `json:"limit"`
	Offset     int        `json:"offset"`
	ObservedAt *time.Time `json:"observed_at,omitempty"`
	Complete   *bool      `json:"complete,omitempty"`
}

// observationFields turns a just-made observation into the two fields a
// grants response adds, so the caller can state how fresh and how whole the
// answer is. A zero ObservedAt means nothing was actually observed this
// request (no Zitadel client configured) — omitted entirely rather than
// rendered as a zero-time reading, which would be indistinguishable from a
// genuine observation made at the Unix epoch.
func observationFields(o db.Observation) (*time.Time, *bool) {
	if o.ObservedAt.IsZero() {
		return nil, nil
	}
	at := o.ObservedAt
	complete := o.Complete
	return &at, &complete
}

// asUserGrants renders the observed shape in the same field names the
// Zitadel-shape response has always used, so this is the one place that
// changed rather than every consumer of it.
func asUserGrants(g []db.ObservedGrant) []zitadel.UserGrant {
	out := make([]zitadel.UserGrant, 0, len(g))
	for _, x := range g {
		out = append(out, zitadel.UserGrant{ID: x.GrantID, UserID: x.UserID, ProjectID: x.ProjectID, RoleKeys: x.RoleKeys})
	}
	return out
}

// paginateObservedGrants applies the caller's limit/offset to a person's full
// observed set. ObservedGrantsFor returns everything for one person — small
// at makerspace scale — so pagination happens in memory rather than adding a
// second, narrower query.
func paginateObservedGrants(all []db.ObservedGrant, limit, offset int) ([]db.ObservedGrant, int) {
	total := len(all)
	if offset >= total {
		return nil, total
	}
	end := offset + limit
	if limit <= 0 {
		// Never "everything". `parseSearchParams` guarantees a positive limit
		// today, so this cannot fire — and a page size that silently means
		// unbounded, resting on a promise made in another function, is the kind
		// of contract that holds until somebody adds a second caller.
		limit = zitadel.DefaultSearchLimit
		end = offset + limit
	}
	if end > total {
		end = total
	}
	return all[offset:end], total
}

// parseSearchParams extracts ?limit=N&offset=N from query string, applying defaults.
func parseSearchParams(r *http.Request) zitadel.SearchParams {
	p := zitadel.SearchParams{Limit: zitadel.DefaultSearchLimit}
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 1000 {
			p.Limit = n
		}
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			p.Offset = n
		}
	}
	return p
}

// --- Users ---

func handleListZitadelUsers(w http.ResponseWriter, r *http.Request) {
	p := parseSearchParams(r)
	result, err := zitadelListUsers(r.Context(), p)
	if err != nil {
		jsonErrorResponse(w, http.StatusBadGateway, "ZITADEL_ERROR", err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, paginatedResponse{
		Items: result.Items, Total: result.Total, Limit: p.Limit, Offset: p.Offset,
	})
}

func handleGetZitadelUser(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("id")
	if !trimmedNonEmpty(userID) {
		jsonValidationErrorResponse(w, "user id is required", map[string]string{"id": "required"})
		return
	}

	user, err := zitadelGetUser(r.Context(), userID)
	if err != nil {
		jsonErrorResponse(w, http.StatusBadGateway, "ZITADEL_ERROR", err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, user)
}

// --- Projects ---

func handleListZitadelProjects(w http.ResponseWriter, r *http.Request) {
	p := parseSearchParams(r)
	result, err := zitadelListProjects(r.Context(), p)
	if err != nil {
		jsonErrorResponse(w, http.StatusBadGateway, "ZITADEL_ERROR", err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, paginatedResponse{
		Items: result.Items, Total: result.Total, Limit: p.Limit, Offset: p.Offset,
	})
}

// --- Project Roles ---

func handleListZitadelProjectRoles(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")
	if !trimmedNonEmpty(projectID) {
		jsonValidationErrorResponse(w, "project id is required", map[string]string{"id": "required"})
		return
	}

	p := parseSearchParams(r)
	result, err := zitadelListProjectRoles(r.Context(), projectID, p)
	if err != nil {
		jsonErrorResponse(w, http.StatusBadGateway, "ZITADEL_ERROR", err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, paginatedResponse{
		Items: result.Items, Total: result.Total, Limit: p.Limit, Offset: p.Offset,
	})
}

func handleCreateZitadelProjectRole(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")
	if !trimmedNonEmpty(projectID) {
		jsonValidationErrorResponse(w, "project id is required", map[string]string{"id": "required"})
		return
	}

	var req createProjectRoleRequest
	if err := decodeJSONStrict(r.Body, &req); err != nil {
		jsonValidationErrorResponse(w, "invalid request body", map[string]string{"body": err.Error()})
		return
	}
	if !trimmedNonEmpty(req.RoleKey) {
		jsonValidationErrorResponse(w, "roleKey is required", map[string]string{"roleKey": "required"})
		return
	}

	if err := zitadelAddProjectRole(r.Context(), projectID, req.RoleKey, req.DisplayName, req.Group); err != nil {
		jsonErrorResponse(w, http.StatusBadGateway, "ZITADEL_ERROR", err.Error())
		return
	}
	directory.Default.InvalidateProject(projectID)
	jsonResponse(w, http.StatusCreated, map[string]string{"status": "created"})
}

func handleUpdateZitadelProjectRole(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")
	roleKey := r.PathValue("key")
	if !trimmedNonEmpty(projectID) || !trimmedNonEmpty(roleKey) {
		jsonValidationErrorResponse(w, "project id and role key are required", map[string]string{"id": "required", "key": "required"})
		return
	}

	var req updateProjectRoleRequest
	if err := decodeJSONStrict(r.Body, &req); err != nil {
		jsonValidationErrorResponse(w, "invalid request body", map[string]string{"body": err.Error()})
		return
	}

	if err := zitadelUpdateProjectRole(r.Context(), projectID, roleKey, req.DisplayName, req.Group); err != nil {
		jsonErrorResponse(w, http.StatusBadGateway, "ZITADEL_ERROR", err.Error())
		return
	}
	directory.Default.InvalidateProject(projectID)
	jsonResponse(w, http.StatusOK, map[string]string{"status": "updated"})
}

func handleDeleteZitadelProjectRole(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")
	roleKey := r.PathValue("key")
	if !trimmedNonEmpty(projectID) || !trimmedNonEmpty(roleKey) {
		jsonValidationErrorResponse(w, "project id and role key are required", map[string]string{"id": "required", "key": "required"})
		return
	}

	if err := zitadelDeleteProjectRole(r.Context(), projectID, roleKey); err != nil {
		jsonErrorResponse(w, http.StatusBadGateway, "ZITADEL_ERROR", err.Error())
		return
	}
	directory.Default.InvalidateProject(projectID)
	jsonResponse(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// --- Grants ---

// handleListAllZitadelGrants observes the whole org (a live Zitadel read,
// recorded as it happens — see internal/observe) and then answers from what
// the store now holds, rather than from the page the live call itself
// returned. That is what makes this the same answer db.ObservedGrantsFor and
// every other reader of zitadel_grants_index would give.
func handleListAllZitadelGrants(w http.ResponseWriter, r *http.Request) {
	p := parseSearchParams(r)
	obs, err := observeOrg(r.Context())
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	items, total, err := dbObservedGrantsPage(r.Context(), p.Limit, p.Offset)
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	observedAt, complete := observationFields(obs)
	jsonResponse(w, http.StatusOK, paginatedResponse{
		Items: asUserGrants(items), Total: total, Limit: p.Limit, Offset: p.Offset,
		ObservedAt: observedAt, Complete: complete,
	})
}

// handleListZitadelUserGrants is the same act at one person's scope: observe,
// then answer from the store.
func handleListZitadelUserGrants(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("id")
	if !trimmedNonEmpty(userID) {
		jsonValidationErrorResponse(w, "user id is required", map[string]string{"id": "required"})
		return
	}

	p := parseSearchParams(r)
	obs, err := observeUser(r.Context(), userID)
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	all, err := dbObservedGrantsFor(r.Context(), userID)
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	items, total := paginateObservedGrants(all, p.Limit, p.Offset)
	observedAt, complete := observationFields(obs)
	jsonResponse(w, http.StatusOK, paginatedResponse{
		Items: asUserGrants(items), Total: total, Limit: p.Limit, Offset: p.Offset,
		ObservedAt: observedAt, Complete: complete,
	})
}

func handleAssignZitadelGrant(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("id")
	if !trimmedNonEmpty(userID) {
		jsonValidationErrorResponse(w, "user id is required", map[string]string{"id": "required"})
		return
	}

	var req assignGrantRequest
	if err := decodeJSONStrict(r.Body, &req); err != nil {
		jsonValidationErrorResponse(w, "invalid request body", map[string]string{"body": err.Error()})
		return
	}
	if !trimmedNonEmpty(req.ProjectID) || len(req.RoleKeys) == 0 {
		jsonValidationErrorResponse(w, "projectId and roleKeys are required", map[string]string{
			"projectId": "required",
			"roleKeys":  "at least one role key required",
		})
		return
	}

	// B4/D3: enqueue through the durable ledger+outbox instead of mutating
	// Zitadel directly. The operator drains it explicitly; the legacy URL still
	// resolves, only the response shape (202 + outbox handle) changed.
	payload, _ := json.Marshal(req)
	res, err := dbEnqueueDirectGrantPropagation(r.Context(), db.EnqueueParams{
		UserID:      userID,
		ProjectID:   req.ProjectID,
		RoleKeys:    req.RoleKeys,
		GrantedBy:   resolveActor(r, ""),
		Source:      "direct",
		OpType:      "add",
		PayloadJSON: string(payload),
	})
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	jsonResponse(w, http.StatusAccepted, res)
}

// resolveGrantTarget recovers (projectID, roleKeys) for a Zitadel grant
// aggregate ID — needed to record a replace/revoke intent in the outbox. The
// webhook-maintained index is the fast path; on a miss it falls back to a live
// Zitadel listing (zitadel_grant_lookup.go).
func resolveGrantTarget(ctx context.Context, userID, grantID string) (string, []string, error) {
	if idx, err := dbGetGrantIndex(ctx, grantID); err == nil {
		return idx.ProjectID, idx.RoleKeys, nil
	}
	g, err := dbListUserGrantsLive(ctx, userID, grantID)
	if err != nil {
		return "", nil, err
	}
	return g.ProjectID, g.RoleKeys, nil
}

func handleUpdateZitadelGrant(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("id")
	grantID := r.PathValue("grantId")
	if !trimmedNonEmpty(userID) || !trimmedNonEmpty(grantID) {
		jsonValidationErrorResponse(w, "user id and grant id are required", map[string]string{"id": "required", "grantId": "required"})
		return
	}

	var req updateGrantRequest
	if err := decodeJSONStrict(r.Body, &req); err != nil {
		jsonValidationErrorResponse(w, "invalid request body", map[string]string{"body": err.Error()})
		return
	}
	if len(req.RoleKeys) == 0 {
		jsonValidationErrorResponse(w, "roleKeys is required", map[string]string{"roleKeys": "at least one role key required"})
		return
	}

	projectID, _, err := resolveGrantTarget(r.Context(), userID, grantID)
	if err != nil {
		jsonErrorResponse(w, http.StatusBadGateway, "ZITADEL_ERROR", err.Error())
		return
	}
	payload, _ := json.Marshal(req)
	res, err := dbEnqueueDirectGrantPropagation(r.Context(), db.EnqueueParams{
		UserID:         userID,
		ProjectID:      projectID,
		RoleKeys:       req.RoleKeys,
		GrantedBy:      resolveActor(r, ""),
		Source:         "direct",
		OpType:         "replace",
		ZitadelGrantID: grantID,
		PayloadJSON:    string(payload),
	})
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	jsonResponse(w, http.StatusAccepted, res)
}

func handleRemoveZitadelGrant(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("id")
	grantID := r.PathValue("grantId")
	if !trimmedNonEmpty(userID) || !trimmedNonEmpty(grantID) {
		jsonValidationErrorResponse(w, "user id and grant id are required", map[string]string{"id": "required", "grantId": "required"})
		return
	}

	projectID, roleKeys, err := resolveGrantTarget(r.Context(), userID, grantID)
	if err != nil {
		jsonErrorResponse(w, http.StatusBadGateway, "ZITADEL_ERROR", err.Error())
		return
	}
	res, err := dbEnqueueDirectGrantPropagation(r.Context(), db.EnqueueParams{
		UserID:         userID,
		ProjectID:      projectID,
		RoleKeys:       roleKeys,
		GrantedBy:      resolveActor(r, ""),
		Source:         "direct",
		OpType:         "revoke",
		ZitadelGrantID: grantID,
		PayloadJSON:    "{}",
	})
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	jsonResponse(w, http.StatusAccepted, res)
}

// --- Health ---

// zitadelHealthResponse is the diagnostic payload for the M2M smoke test.
type zitadelHealthResponse struct {
	Status    string `json:"status"`
	Mode      string `json:"mode"`
	Domain    string `json:"domain,omitempty"`
	Projects  int    `json:"projects_total,omitempty"`
	LatencyMs int64  `json:"latency_ms,omitempty"`
	Error     string `json:"error,omitempty"`
}

// handleZitadelHealth exercises the full M2M path end-to-end: the key file is
// re-read on demand for the token assertion, exchanged for an access token
// against /oauth/v2/token, and then used for a minimal Management API call.
// Gated by the shared API key so operators can verify the service-account
// configuration without needing a user bearer token.
//
// GET /api/v1/zitadel/health
func handleZitadelHealth(w http.ResponseWriter, r *http.Request) {
	domain := os.Getenv("ZITADEL_DOMAIN")

	if zitadel.MgmtClient == nil {
		jsonResponse(w, http.StatusServiceUnavailable, zitadelHealthResponse{
			Status: "disabled",
			Mode:   "local-policy-only",
			Domain: domain,
			Error:  "Management client not initialized — check ZITADEL_DOMAIN, ZITADEL_MACHINE_KEY_PATH, and backend startup logs",
		})
		return
	}

	start := time.Now()
	// limit=1 keeps the response tiny while still forcing a real API round-trip.
	result, err := zitadelListProjects(r.Context(), zitadel.SearchParams{Limit: 1})
	latency := time.Since(start).Milliseconds()

	if err != nil {
		jsonResponse(w, http.StatusBadGateway, zitadelHealthResponse{
			Status:    "error",
			Mode:      "live",
			Domain:    domain,
			LatencyMs: latency,
			Error:     err.Error(),
		})
		return
	}

	jsonResponse(w, http.StatusOK, zitadelHealthResponse{
		Status:    "ok",
		Mode:      "live",
		Domain:    domain,
		Projects:  result.Total,
		LatencyMs: latency,
	})
}
