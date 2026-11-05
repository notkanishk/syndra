package handlers

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"syndra/internal/db"
	"syndra/internal/models"
	"syndra/internal/services"
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

// dbLatestOrgObservation is the latest org-wide observation without its rows,
// for the readers that need to know how fresh the store is, not to refresh it.
var dbLatestOrgObservation = db.LatestOrgObservation

// observationMaxAge is how old the store may be before a request observes for
// itself. It is the default sweep cadence: the sweep keeps the store this fresh,
// so an older observation means the sweep is behind and a read is warranted.
const observationMaxAge = 5 * time.Minute

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
		writeUpstream(w, err, "ZITADEL_ERROR")
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
		writeUpstream(w, err, "ZITADEL_ERROR")
		return
	}
	jsonResponse(w, http.StatusOK, user)
}

// --- Projects ---

func handleListZitadelProjects(w http.ResponseWriter, r *http.Request) {
	p := parseSearchParams(r)
	result, err := zitadelListProjects(r.Context(), p)
	if err != nil {
		writeUpstream(w, err, "ZITADEL_ERROR")
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
		writeUpstream(w, err, "ZITADEL_ERROR")
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

	if _, err := svcCreateRole(r.Context(), services.CreateRoleRequest{
		ProjectID:   projectID,
		RoleKey:     strings.TrimSpace(req.RoleKey),
		DisplayName: strings.TrimSpace(req.DisplayName),
		Group:       req.Group,
	}, resolveActor(r, "")); err != nil {
		writeRoleWriteError(w, err)
		return
	}
	jsonResponse(w, http.StatusCreated, map[string]string{"status": "created"})
}

// roleChangeUnknownCode answers a role write whose outcome nobody knows: the
// answer was lost and the read-back failed (services.ErrRoleChangeUnknown).
// Syndra kept its record; the screen must not say nothing changed.
const (
	roleChangeUnknownCode    = "OUTCOME_UNKNOWN"
	roleChangeUnknownMessage = "Zitadel didn't answer, and Syndra couldn't read the role back, so it can't tell whether Zitadel made the change. Syndra kept its record. Open this project again in a minute to see what Zitadel holds."
)

// writeRoleWriteError answers a refused or failed role write from the one
// role pipe (services/roles.go). A delete held by people names them and their
// count in details, so the dialog can ask for that exact count back.
func writeRoleWriteError(w http.ResponseWriter, err error) {
	var inUse *services.RoleInUseError
	var held *services.RoleHeldError
	switch {
	case errors.Is(err, services.ErrInvalidRoleKey):
		jsonValidationErrorResponse(w, "Role keys use only letters, digits, - and _.", map[string]string{"roleKey": "pattern"})
	case errors.Is(err, db.ErrDuplicateRole):
		jsonErrorResponse(w, http.StatusConflict, "CONFLICT", "A role with this key already exists in this project. Nothing was changed.")
	case errors.As(err, &inUse):
		jsonErrorResponse(w, http.StatusConflict, "ROLE_IN_USE", fmt.Sprintf(
			"Syndra still gives this role (%s). Take it out of those first, so each removal is recorded. Nothing was changed.",
			inUseWords(inUse)))
	case errors.As(err, &held) && held.Unknown:
		jsonErrorResponse(w, http.StatusConflict, "HOLDERS_UNKNOWN",
			"Syndra couldn't confirm who holds this role in Zitadel, so it can't say who would lose it. Open Zitadel › Users to read it again, then try once more. Nothing was changed.")
	case errors.As(err, &held):
		names := make([]string, 0, len(held.Holders))
		for _, h := range held.Holders {
			names = append(names, cmp.Or(h.Name, h.Email, h.ID))
		}
		jsonResponse(w, http.StatusConflict, ErrorResponse{
			Error: "ROLE_HELD",
			Message: fmt.Sprintf("%d %s this role in Zitadel and would lose it. Nothing was changed.",
				len(names), oneOrMany(len(names), "person holds", "people hold")),
			Details: map[string]string{"count": strconv.Itoa(len(names)), "holders": strings.Join(names, ", ")},
		})
	case errors.Is(err, services.ErrRoleChangeUnknown):
		// Never "Nothing was changed": the change may have landed, and a
		// delete may already have taken the role from everyone holding it.
		log.Printf("[ROLES] outcome unknown: %v", err)
		jsonErrorResponse(w, http.StatusBadGateway, roleChangeUnknownCode, roleChangeUnknownMessage)
	case errors.Is(err, services.ErrRoleNotChangedInZitadel):
		// Not writeUpstream's generic sentence: this one says what Syndra did
		// (put its record back). The raw rpc text goes to the log only.
		log.Printf("[ROLES] not changed: %v", err)
		jsonErrorResponse(w, http.StatusBadGateway, "ZITADEL_REFUSED", "Zitadel didn't make the change, so Syndra put its record back.")
	default:
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
	}
}

func inUseWords(e *services.RoleInUseError) string {
	var parts []string
	if e.Given > 0 {
		parts = append(parts, fmt.Sprintf("%d %s", e.Given, oneOrMany(e.Given, "person", "people")))
	}
	if e.Bundles > 0 {
		parts = append(parts, fmt.Sprintf("%d bundle%s", e.Bundles, plural(e.Bundles)))
	}
	if e.Rules > 0 {
		parts = append(parts, fmt.Sprintf("%d automatic rule%s", e.Rules, plural(e.Rules)))
	}
	return strings.Join(parts, ", ")
}

func oneOrMany(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
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

	if err := svcUpdateRole(r.Context(), projectID, roleKey, strings.TrimSpace(req.DisplayName), req.Group, resolveActor(r, "")); err != nil {
		writeRoleWriteError(w, err)
		return
	}
	jsonResponse(w, http.StatusOK, map[string]string{"status": "updated"})
}

func handleDeleteZitadelProjectRole(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")
	roleKey := r.PathValue("key")
	if !trimmedNonEmpty(projectID) || !trimmedNonEmpty(roleKey) {
		jsonValidationErrorResponse(w, "project id and role key are required", map[string]string{"id": "required", "key": "required"})
		return
	}

	// confirm_holders is how many people the caller saw would lose the role.
	// Absent means none: a delete that would take it from anybody is refused
	// until the caller names that exact count back (services.DeleteRole).
	confirm := 0
	if raw := r.URL.Query().Get("confirm_holders"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 0 {
			jsonValidationErrorResponse(w, "confirm_holders must be a count", map[string]string{"confirm_holders": "min=0"})
			return
		}
		confirm = n
	}

	if err := svcDeleteRole(r.Context(), projectID, roleKey, resolveActor(r, ""), confirm); err != nil {
		writeRoleWriteError(w, err)
		return
	}
	jsonResponse(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// --- Grants ---

// handleListAllZitadelGrants observes the whole org (a live Zitadel read,
// recorded as it happens — see internal/observe) and then answers from what
// the store now holds, rather than from the page the live call itself
// returned. That is what makes this the same answer db.ObservedGrantsFor and
// every other reader of zitadel_grants_index would give.
//
// Only the first page observes. The caller pages through the one read the
// first page made; observing again for every later page was a full org read
// per page, each replacing the store under the pages before it.
func handleListAllZitadelGrants(w http.ResponseWriter, r *http.Request) {
	p := parseSearchParams(r)
	var (
		obs   db.Observation
		items []db.ObservedGrant
		total int
		err   error
	)
	if p.Offset == 0 {
		if obs, err = observeOrg(r.Context()); err == nil {
			items, total, err = dbObservedGrantsPage(r.Context(), p.Limit, p.Offset)
		}
	} else {
		// One snapshot: a sweep committing between pages must not pair its
		// header with the previous generation's rows.
		obs, items, total, err = dbObservationPage(r.Context(), p.Limit, p.Offset)
		if errors.Is(err, db.ErrNoObservation) {
			obs, items, total, err = db.Observation{}, nil, 0, nil // never observed: omit the fields, as with no client
		}
	}
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

// zitadelGrantChange is what a change on the Zitadel › Users screen did, role
// by role, in the words the person page's removal uses: what was given, what
// goes, and what stays because another source still gives it.
type zitadelGrantChange struct {
	OutboxIDs  []string `json:"outbox_ids"`
	Given      []string `json:"given_roles"`
	Revoked    []string `json:"revoked_roles"`
	Retained   []string `json:"retained_roles"`
	Status     string   `json:"status"`
	WaitReason string   `json:"wait_reason,omitempty"`
	// Roles is each role's own outcome (W8): one change can send some roles,
	// hold one that waits, and keep one a bundle still gives.
	Roles []zitadelRoleChange `json:"roles"`
}

// zitadelRoleChange is one role's part of a change. Status, after delivery:
// applied (in Zitadel now), waiting (held for the operator to send), pending
// (meant to go, not taken yet), failed, or kept (nothing was sent).
type zitadelRoleChange struct {
	Role   string `json:"role"`   // "project/role"
	Change string `json:"change"` // given | revoked | kept
	Status string `json:"status"`
	rows   []string
}

func newZitadelGrantChange() zitadelGrantChange {
	return zitadelGrantChange{OutboxIDs: []string{}, Given: []string{}, Revoked: []string{}, Retained: []string{}, Status: "pending", Roles: []zitadelRoleChange{}}
}

func (res *zitadelGrantChange) add(role, change string, rows ...string) {
	res.Roles = append(res.Roles, zitadelRoleChange{Role: role, Change: change, rows: rows})
	switch change {
	case "given":
		res.Given = append(res.Given, role)
	case "revoked":
		res.Revoked = append(res.Revoked, role)
	case "kept":
		res.Retained = append(res.Retained, role)
	}
}

// roleStatusRank orders statuses worst first: a role carried by several rows
// is as far along as its least delivered row.
var roleStatusRank = map[string]int{"failed": 0, "waiting": 1, "pending": 2, "applied": 3}

// handleAssignZitadelGrant gives roles from the Zitadel › Users screen through
// the person page's pipe: each role must be offered and not already held, and
// each is one direct grant, delivered by the one rule (services.Deliver).
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
	roles := distinctRoleKeys(req.RoleKeys)
	if !trimmedNonEmpty(req.ProjectID) || len(roles) == 0 {
		jsonValidationErrorResponse(w, "projectId and roleKeys are required", map[string]string{
			"projectId": "required",
			"roleKeys":  "at least one role key required",
		})
		return
	}

	if !canGiveRoles(w, r.Context(), userID, req.ProjectID, roles) {
		return
	}
	res := newZitadelGrantChange()
	if err := giveRoles(r.Context(), resolveActor(r, ""), userID, req.ProjectID, roles, &res); err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	finishZitadelGrantChange(w, r, userID, res)
}

// resolveGrantTarget recovers (projectID, roleKeys) for a Zitadel grant
// aggregate ID. The webhook-maintained index is the fast path; on a miss it
// falls back to a live Zitadel listing (zitadel_grant_lookup.go).
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

// handleUpdateZitadelGrant replaces the roles on one Zitadel grant as what it
// is: the roles left out are removed and the new ones given, each through the
// same pipe as a removal or a give on the person page.
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
	want := distinctRoleKeys(req.RoleKeys)
	if len(want) == 0 {
		jsonValidationErrorResponse(w, "roleKeys is required", map[string]string{"roleKeys": "at least one role key required"})
		return
	}

	projectID, current, err := resolveGrantTarget(r.Context(), userID, grantID)
	if err != nil {
		writeUpstream(w, err, "ZITADEL_ERROR")
		return
	}
	adds, removes := setDiff(want, current), setDiff(current, want)

	if !canGiveRoles(w, r.Context(), userID, projectID, adds) {
		return
	}
	actor := resolveActor(r, "")
	res := newZitadelGrantChange()
	if err := removeRoles(r.Context(), actor, userID, projectID, removes, &res); err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	if err := giveRoles(r.Context(), actor, userID, projectID, adds, &res); err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	finishZitadelGrantChange(w, r, userID, res)
}

// handleRemoveZitadelGrant removes the roles on one Zitadel grant the way the
// person page removes access: only what is held directly goes, and a role a
// bundle or a rule still gives stays (services.DeleteDirectGrant).
func handleRemoveZitadelGrant(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("id")
	grantID := r.PathValue("grantId")
	if !trimmedNonEmpty(userID) || !trimmedNonEmpty(grantID) {
		jsonValidationErrorResponse(w, "user id and grant id are required", map[string]string{"id": "required", "grantId": "required"})
		return
	}

	projectID, roleKeys, err := resolveGrantTarget(r.Context(), userID, grantID)
	if err != nil {
		writeUpstream(w, err, "ZITADEL_ERROR")
		return
	}
	res := newZitadelGrantChange()
	if err := removeRoles(r.Context(), resolveActor(r, ""), userID, projectID, roleKeys, &res); err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	finishZitadelGrantChange(w, r, userID, res)
}

// canGiveRoles runs the person page's two checks on every role before any is
// written: Zitadel still offers it, and the person does not already hold it.
// One refusal refuses the whole change, so nothing is half given.
func canGiveRoles(w http.ResponseWriter, ctx context.Context, userID, projectID string, roles []string) bool {
	for _, role := range roles {
		if err := checkRoleOffered(ctx, projectID, role); err != nil {
			writeRoleOfferedError(w, err)
			return false
		}
		holding, err := dbRequestHolding(ctx, userID, projectID, role)
		if err != nil {
			jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
			return false
		}
		if holding.Kind != models.HoldingNone {
			jsonErrorResponse(w, http.StatusConflict, "ALREADY_HELD",
				role+": "+alreadyHeldMessage(holding, "They")+" Nothing was changed.")
			return false
		}
	}
	return true
}

// giveRoles writes one direct grant per role, as the person page does.
func giveRoles(ctx context.Context, actor, userID, projectID string, roles []string, res *zitadelGrantChange) error {
	for _, role := range roles {
		q, err := grantDirect(ctx, db.EnqueueParams{
			UserID:      userID,
			ProjectID:   projectID,
			RoleKeys:    []string{role},
			GrantedBy:   actor,
			Source:      "direct",
			OpType:      "add",
			EndRule:     db.EndKept,
			PayloadJSON: "{}",
		})
		if err != nil {
			return err
		}
		res.OutboxIDs = append(res.OutboxIDs, q.OutboxID)
		res.add(projectID+"/"+role, "given", q.OutboxID)
	}
	return nil
}

// removeRoles takes each role away the way the person page does. A direct
// grant goes through services.DeleteDirectGrant, which revokes only what the
// person loses. A role with no direct grant that Syndra still gives (a bundle,
// a rule) stays. A role Syndra does not give at all is held in Zitadel with
// nothing behind it, so its revoke takes nothing Syndra gives; it is recorded
// like any other revoke.
func removeRoles(ctx context.Context, actor, userID, projectID string, roles []string, res *zitadelGrantChange) error {
	direct, err := svcUserDirectGrants(ctx, userID)
	if err != nil {
		return err
	}
	grantFor := map[string]string{}
	for _, g := range direct {
		if g.ProjectID == projectID {
			grantFor[g.RoleKey] = g.ID
		}
	}
	for _, role := range roles {
		if id, ok := grantFor[role]; ok {
			rm, err := svcDeleteDirectGrant(ctx, userID, id, actor)
			if err != nil {
				return err
			}
			res.OutboxIDs = append(res.OutboxIDs, rm.OutboxIDs...)
			// ponytail: the removal's rows are not split by role, so each role
			// it revoked reports the least delivered of them.
			for _, gone := range rm.Revoked {
				res.add(gone, "revoked", rm.OutboxIDs...)
			}
			for _, kept := range rm.Retained {
				res.add(kept, "kept")
			}
			continue
		}
		// ponytail: read outside the access lock; a bundle given in the same
		// instant queues its own add after this revoke. The planner (W3a)
		// moves this read under the lock.
		held, err := svcUserExpectsRole(ctx, userID, projectID, role)
		if err != nil {
			return err
		}
		if held {
			res.add(projectID+"/"+role, "kept")
			continue
		}
		q, err := grantDirect(ctx, db.EnqueueParams{
			UserID:      userID,
			ProjectID:   projectID,
			RoleKeys:    []string{role},
			GrantedBy:   actor,
			Source:      "direct",
			OpType:      "revoke",
			PayloadJSON: "{}",
		})
		if err != nil {
			return err
		}
		res.OutboxIDs = append(res.OutboxIDs, q.OutboxID)
		res.add(projectID+"/"+role, "revoked", q.OutboxID)
	}
	return nil
}

// finishZitadelGrantChange is the person page's tail: recompile the person's
// claims first, then deliver by the one rule (services.Deliver). Each role
// then reports how far its own rows got, and Status is the least delivered.
func finishZitadelGrantChange(w http.ResponseWriter, r *http.Request, userID string, res zitadelGrantChange) {
	rebuildUserCacheDetachedFn(r.Context(), userID)
	row := map[string]string{}
	if len(res.OutboxIDs) > 0 {
		d := svcDeliver(r.Context(), res.OutboxIDs, r.URL.Query().Get("apply") != "true")
		res.WaitReason = d.WaitReason()
		for _, id := range d.Waiting {
			row[id] = "waiting"
		}
		for _, id := range d.Sent {
			row[id] = "pending"
			if st, err := dbGetPropagationStatus(r.Context(), id); err == nil && (st == "applied" || st == "failed") {
				row[id] = st
			}
		}
	}
	worst := ""
	for i := range res.Roles {
		rc := &res.Roles[i]
		if rc.Change == "kept" {
			rc.Status = "kept"
			continue
		}
		for _, id := range rc.rows {
			if st := cmp.Or(row[id], "pending"); rc.Status == "" || roleStatusRank[st] < roleStatusRank[rc.Status] {
				rc.Status = st
			}
		}
		rc.Status = cmp.Or(rc.Status, "pending")
		if worst == "" || roleStatusRank[rc.Status] < roleStatusRank[worst] {
			worst = rc.Status
		}
	}
	switch worst {
	case "applied", "failed":
		res.Status = worst
	case "":
		res.Status = "unchanged" // every role was kept: nothing was sent
	default:
		res.Status = "pending" // waiting rows are pending in the outbox too
	}
	jsonResponse(w, http.StatusAccepted, res)
}

func distinctRoleKeys(keys []string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, k := range keys {
		k = strings.TrimSpace(k)
		if k != "" && !seen[k] {
			seen[k] = true
			out = append(out, k)
		}
	}
	return out
}

// setDiff is a minus b, in a's order.
func setDiff(a, b []string) []string {
	out := []string{}
	for _, x := range a {
		if !slices.Contains(b, x) {
			out = append(out, x)
		}
	}
	return out
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
		log.Printf("[ZITADEL] health probe failed: %v", err)
		jsonResponse(w, http.StatusBadGateway, zitadelHealthResponse{
			Status:    "error",
			Mode:      "live",
			Domain:    domain,
			LatencyMs: latency,
			Error:     upstreamSentence(http.StatusBadGateway),
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
