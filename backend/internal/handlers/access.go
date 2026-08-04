package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"syndra/internal/db"
	"syndra/internal/directory"
	"syndra/internal/models"
	"syndra/internal/services"
)

type UpsertDirectGrantRequest struct {
	ProjectID string `json:"project_id"`
	RoleKey   string `json:"role_key"`
	// GrantedBy is a fallback for demo/local-dev where the request context has
	// no authenticated principal (API-key auth doesn't carry user identity).
	// In production (Zitadel JWT auth) the authenticated subject takes
	// precedence over this body field — clients should not send it.
	GrantedBy    string `json:"granted_by,omitempty"`
	Reason       string `json:"reason"`
	DurationDays int    `json:"duration_days"`
}

type CreateAccessRequestRequest struct {
	RequesterID   string `json:"requester_id"`
	ProjectID     string `json:"project_id"`
	RoleKey       string `json:"role_key"`
	Justification string `json:"justification"`
	DurationDays  int    `json:"duration_days"`
}

type ResolveAccessRequestRequest struct {
	Status string `json:"status"`
	// ReviewerID is a fallback for demo/local-dev. In production the
	// authenticated subject from the bearer token is used instead.
	ReviewerID string `json:"reviewer_id,omitempty"`
	ReviewNote string `json:"review_note"`
}

// resolveActor prefers the authenticated principal from the request context
// (set by withUserAuth from a Zitadel-issued JWT). Falls back to a
// caller-supplied body value when the context principal is empty (demo/API-key
// mode). Returns "system" only when both are empty.
func resolveActor(r *http.Request, bodyValue string) string {
	if id := getAdminUserID(r.Context()); id != "" {
		return id
	}
	if id := strings.TrimSpace(bodyValue); id != "" {
		return id
	}
	return "system"
}

func handleGetUserDirectGrants(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("id")
	grants, err := services.UserDirectGrants(r.Context(), userID)
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, grants)
}

func handleUpsertUserDirectGrant(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("id")
	var req UpsertDirectGrantRequest
	if err := decodeJSONStrict(r.Body, &req); err != nil {
		jsonValidationErrorResponse(w, "Invalid JSON payload", map[string]string{"body": err.Error()})
		return
	}
	if !trimmedNonEmpty(userID) {
		jsonValidationErrorResponse(w, "id path parameter is required", map[string]string{"id": "required"})
		return
	}
	if !trimmedNonEmpty(req.ProjectID) || !trimmedNonEmpty(req.RoleKey) {
		jsonValidationErrorResponse(w, "project_id and role_key are required", map[string]string{
			"project_id": "required",
			"role_key":   "required",
		})
		return
	}
	if req.DurationDays < 0 {
		jsonValidationErrorResponse(w, "duration_days must be zero or greater", map[string]string{"duration_days": "min=0"})
		return
	}

	grantedBy := resolveActor(r, req.GrantedBy)

	var expiresAt *time.Time
	if req.DurationDays > 0 {
		expiry := time.Now().UTC().Add(time.Duration(req.DurationDays) * 24 * time.Hour)
		expiresAt = &expiry
	}

	// The grant is recorded in the durable ledger + outbox in one transaction
	// (audit included); the Zitadel mutation happens later during the
	// operator-triggered drain. This is the B4/D3 single-mutation-authority
	// path — no direct Zitadel call from the handler.
	payload, _ := json.Marshal(req)
	res, err := dbEnqueueDirectGrantPropagation(r.Context(), db.EnqueueParams{
		UserID:      userID,
		ProjectID:   req.ProjectID,
		RoleKeys:    []string{req.RoleKey},
		GrantedBy:   grantedBy,
		Reason:      req.Reason,
		ExpiresAt:   expiresAt,
		Source:      "direct",
		OpType:      "add",
		PayloadJSON: string(payload),
	})
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	// Rebuild the compiled cache from the just-committed ledger row FIRST, so
	// access is effective immediately — before (and independent of) the
	// best-effort inline drain, and on a context detached from the request so a
	// client disconnect after commit cannot leave access ineffective.
	rebuildUserCacheDetached(r.Context(), userID)

	// Inline "apply now" for single mutations from inline forms (design §7 Q1):
	// the operator opts in via ?apply=true to drain immediately rather than
	// resuming from the dashboard. Targeted to THIS row only — applying one grant
	// must not project unrelated mutations an operator left queued. A drain
	// failure is non-fatal — the row stays pending and the operator can resume
	// later. We then report this row's own post-drain status.
	if r.URL.Query().Get("apply") == "true" {
		if _, derr := svcDrainPropagationRow(r.Context(), res.OutboxID); derr == nil {
			if st, serr := dbGetPropagationStatus(r.Context(), res.OutboxID); serr == nil && st != "" {
				res.Status = st
			}
		}
	}

	jsonResponse(w, http.StatusAccepted, res)
}

func handleGetAccessRequests(w http.ResponseWriter, r *http.Request) {
	status := strings.TrimSpace(r.URL.Query().Get("status"))
	switch status {
	case "", "pending", "approved", "rejected", "withdrawn":
	default:
		jsonValidationErrorResponse(w, "status must be one of pending, approved, rejected, withdrawn", map[string]string{"status": "enum"})
		return
	}
	requests, err := dbGetAccessRequests(r.Context(), status)
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	// Members see only their own requests — the org-wide list (with
	// justifications) is operator-scoped (SC3).
	if !isOperator(r) {
		self := getAdminUserID(r.Context())
		own := make([]models.AccessRequest, 0, len(requests))
		for _, req := range requests {
			if req.RequesterID == self {
				own = append(own, req)
			}
		}
		requests = own
	}
	jsonResponse(w, http.StatusOK, requests)
}

func handleCreateAccessRequest(w http.ResponseWriter, r *http.Request) {
	var req CreateAccessRequestRequest
	if err := decodeJSONStrict(r.Body, &req); err != nil {
		jsonValidationErrorResponse(w, "Invalid JSON payload", map[string]string{"body": err.Error()})
		return
	}
	// A member can only file requests as themselves — the authenticated subject
	// overrides whatever requester_id the client sent (SC8). Operators may file
	// on behalf of another user; dev mode (API-key, no subject) trusts the body.
	if !isOperator(r) {
		req.RequesterID = getAdminUserID(r.Context())
	}
	if !trimmedNonEmpty(req.RequesterID) || !trimmedNonEmpty(req.ProjectID) || !trimmedNonEmpty(req.RoleKey) || !trimmedNonEmpty(req.Justification) {
		jsonValidationErrorResponse(w, "requester_id, project_id, role_key, and justification are required", map[string]string{
			"requester_id":  "required",
			"project_id":    "required",
			"role_key":      "required",
			"justification": "required",
		})
		return
	}
	if req.DurationDays < 0 {
		jsonValidationErrorResponse(w, "duration_days must be zero or greater", map[string]string{"duration_days": "min=0"})
		return
	}

	var durationDays *int
	if req.DurationDays > 0 {
		durationDays = &req.DurationDays
	}
	id, err := dbCreateAccessRequest(r.Context(), req.RequesterID, req.ProjectID, req.RoleKey, req.Justification, durationDays)
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	// Audit actor is the authenticated principal when present (admin acting on
	// behalf of a member, or the member themselves). Falls back to the
	// requester so demo-mode self-requests still attribute correctly.
	actor := resolveActor(r, req.RequesterID)
	_ = dbInsertAuditLog(r.Context(), actor, req.RequesterID, "access_request.created", id)
	jsonResponse(w, http.StatusCreated, map[string]string{"id": id})
}

// handleWithdrawAccessRequest lets the person who filed a request take it back.
//
// Self-only, and self-only for operators too. An operator withdrawing somebody else's ask is a
// rejection with the reviewer's name left off — the decision endpoint exists for that, and it
// records who took it.
//
// A withdrawn request grants nothing, so there is no cascade and nothing to drain; the only
// state that changes is that it leaves the queue.
// POST /api/v1/requests/{id}/withdraw
func handleWithdrawAccessRequest(w http.ResponseWriter, r *http.Request) {
	requestID := r.PathValue("id")
	if !trimmedNonEmpty(requestID) {
		jsonValidationErrorResponse(w, "id path parameter is required", map[string]string{"id": "required"})
		return
	}

	request, err := dbGetAccessRequestByID(r.Context(), requestID)
	if err != nil {
		jsonErrorResponse(w, http.StatusNotFound, "NOT_FOUND", "No such request — it may have been withdrawn.")
		return
	}

	// Production: the authenticated subject must be the requester. Dev mode (API-key auth, no
	// subject) falls through to the requester on the row, and db.WithdrawAccessRequest still
	// scopes the UPDATE by it — the guard is in the statement, not only here.
	actor := getAdminUserID(r.Context())
	if actor != "" && actor != request.RequesterID {
		jsonErrorResponse(w, http.StatusForbidden, "FORBIDDEN", "You can only withdraw your own requests")
		return
	}

	switch err := dbWithdrawAccessRequest(r.Context(), requestID, request.RequesterID); {
	case err == nil:
		_ = dbInsertAuditLog(r.Context(), request.RequesterID, request.RequesterID, "access_request.withdrawn", requestID)
		jsonResponse(w, http.StatusOK, map[string]string{"message": "Request withdrawn"})
	case errors.Is(err, db.ErrRequestNotPending):
		// Somebody decided it in the meantime, or it was already withdrawn. Either way the
		// member's copy of this row is stale, and saying so beats reporting a failure.
		jsonErrorResponse(w, http.StatusConflict, "ALREADY_RESOLVED", "This request has already been decided.")
	default:
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
	}
}

func handleResolveAccessRequest(w http.ResponseWriter, r *http.Request) {
	requestID := r.PathValue("id")
	if !trimmedNonEmpty(requestID) {
		jsonValidationErrorResponse(w, "id path parameter is required", map[string]string{"id": "required"})
		return
	}

	var req ResolveAccessRequestRequest
	if err := decodeJSONStrict(r.Body, &req); err != nil {
		jsonValidationErrorResponse(w, "Invalid JSON payload", map[string]string{"body": err.Error()})
		return
	}

	status := strings.ToLower(strings.TrimSpace(req.Status))
	if status != "approved" && status != "rejected" {
		jsonValidationErrorResponse(w, "status must be approved or rejected", map[string]string{"status": "enum"})
		return
	}

	reviewer := resolveActor(r, req.ReviewerID)
	if status == "approved" && reviewer == "system" {
		jsonValidationErrorResponse(w, "an authenticated reviewer is required when approving a request", map[string]string{"reviewer_id": "required_when=status:approved"})
		return
	}

	switch err := resolveOneAccessRequest(r.Context(), requestID, status, reviewer, req.ReviewNote); {
	case err == nil:
		jsonResponse(w, http.StatusOK, map[string]string{"message": "Request resolved"})
	case errors.Is(err, errRequestNotFound):
		jsonErrorResponse(w, http.StatusNotFound, "NOT_FOUND", err.Error())
	case errors.Is(err, errRequestAlreadyResolved):
		jsonErrorResponse(w, http.StatusConflict, "ALREADY_RESOLVED", err.Error())
	default:
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
	}
}

var (
	errRequestNotFound        = errors.New("access request not found")
	errRequestAlreadyResolved = errors.New("access request is already resolved")
)

// resolveOneAccessRequest is the decision itself, with no HTTP in it.
//
// Extracted so the bulk endpoint runs exactly this code rather than a second
// implementation of it. The sequence below — conditional transaction, race
// guard, cache rebuild, inline drain — is the part that must not diverge: a
// second copy that drifted would leave requests approved but ungranted, which
// re-surfaces later as syndra_only drift and is diagnosed by nobody.
func resolveOneAccessRequest(ctx context.Context, requestID, status, reviewer, reviewNote string) error {
	request, err := dbGetAccessRequestByID(ctx, requestID)
	if err != nil {
		return fmt.Errorf("%w: %s", errRequestNotFound, err.Error())
	}

	// Anything that is not pending is already settled — including 'withdrawn', which the
	// member settled themselves. Enumerating the decided statuses instead would mean every new
	// terminal state silently became decidable again.
	if request.Status != "pending" {
		return fmt.Errorf("%w (already %s)", errRequestAlreadyResolved, request.Status)
	}

	if status == "approved" {
		var expiresAt *time.Time
		if request.DurationDays != nil && *request.DurationDays > 0 {
			expiry := time.Now().UTC().Add(time.Duration(*request.DurationDays) * 24 * time.Hour)
			expiresAt = &expiry
		}
		// An approval creates the same kind of direct grant as the operator grant
		// endpoint, so it flows through the durable ledger+outbox (B4/D3 single
		// mutation authority) rather than the bare upsert — otherwise the grant is
		// invisible to the Pending UI, never projected to Zitadel, and later
		// re-surfaces as syndra_only drift. source_ref ties the grant to the
		// originating request. Resolution + enqueue are ONE conditional
		// transaction: either the request flips to approved AND the grant is
		// enqueued, or neither happens (no approved-but-ungranted state), and the
		// status='pending' guard closes the concurrent approve/reject race.
		payload, _ := json.Marshal(map[string]string{
			"user_id":      request.RequesterID,
			"project_id":   request.ProjectID,
			"role_key":     request.RoleKey,
			"from_request": requestID,
		})
		res, err := dbApproveRequestAndEnqueue(ctx, requestID, reviewer, reviewNote, db.EnqueueParams{
			UserID:      request.RequesterID,
			ProjectID:   request.ProjectID,
			RoleKeys:    []string{request.RoleKey},
			GrantedBy:   reviewer,
			Reason:      "Approved from access request",
			ExpiresAt:   expiresAt,
			Source:      "direct",
			SourceRef:   requestID,
			OpType:      "add",
			PayloadJSON: string(payload),
		})
		if err != nil {
			if errors.Is(err, db.ErrRequestNotPending) {
				return errRequestAlreadyResolved
			}
			return err
		}
		// Rebuild the compiled cache from the just-committed ledger row FIRST, so
		// access is effective immediately — independent of the best-effort inline
		// drain below, and on a context detached from the request so a client
		// disconnect after commit cannot leave access ineffective.
		rebuildUserCacheDetached(ctx, request.RequesterID)

		// Apply inline: the approval is the operator's confirmation, so project the
		// grant to Zitadel now rather than waiting for a separate resume. Targeted
		// to THIS row only (never the global batch), and non-fatal — a drain
		// failure (e.g. Zitadel offline) leaves the row pending in the worklist for
		// a later resume; access already works via the ledger.
		_, _ = svcDrainPropagationRow(ctx, res.OutboxID)
	} else { // rejected — no grant, just the conditional resolution
		if err := dbResolveAccessRequest(ctx, requestID, status, reviewer, reviewNote); err != nil {
			if errors.Is(err, db.ErrRequestNotPending) {
				return errRequestAlreadyResolved
			}
			return err
		}
	}

	_ = dbInsertAuditLog(ctx, reviewer, request.RequesterID, "access_request."+status, requestID)
	return nil
}

func handleGetGovernanceSummary(w http.ResponseWriter, r *http.Request) {
	summary, err := services.Governance(r.Context())
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, summary)
}

// cacheRebuildDetachTimeout bounds the detached compiled-cache rebuild so a
// wedged directory/cache call can't leak a goroutine-blocking request.
const cacheRebuildDetachTimeout = 10 * time.Second

// rebuildUserCacheDetached rebuilds the user's compiled claims on a context
// DETACHED from the request lifecycle (client cancellation), bounded by
// cacheRebuildDetachTimeout. Once a grant's ledger row is committed, the rebuild
// that makes access effective must complete regardless of whether the client is
// still connected — otherwise "effective immediately after commit" would hold
// only for clients that wait for the response. It stays synchronous so the
// handler still completes the rebuild before responding; it just no longer
// inherits the request's cancellation.
func rebuildUserCacheDetached(ctx context.Context, userID string) {
	detached, cancel := context.WithTimeout(context.WithoutCancel(ctx), cacheRebuildDetachTimeout)
	defer cancel()
	rebuildUserCacheOrSkip(detached, userID)
}

// rebuildUserCacheOrSkip pulls the project scope from the directory and
// rebuilds the user's compiled claims. On directory failure it logs and
// skips the rebuild, leaving previously compiled claims in place — see
// cacheRebuildProjectIDs for the rationale.
func rebuildUserCacheOrSkip(ctx context.Context, userID string) {
	projectIDs, err := cacheRebuildProjectIDs(ctx)
	if err != nil {
		log.Printf("[ACCESS] Skipping cache rebuild for user %s: directory lookup failed: %v", userID, err)
		return
	}
	cacheRebuildUser(ctx, userID, projectIDs)
}

// cacheRebuildProjectIDs returns the set of project IDs the cache compiler
// should rebuild for. Pulled from directory.Projects (live Zitadel or demo
// fallback) — NOT directory.Applications, because Applications can return a
// partial catalog when a per-project ListApplications call fails. A partial
// project list would cause RebuildUserCache (which deletes every
// mapping:<user>:* key before rebuilding) to silently erase compiled claims
// for any project whose apps were transiently unreachable.
//
// Returns an error on directory failure so the caller can skip the rebuild
// entirely. cache.RebuildUserCache starts by wiping every mapping:<user>:*
// key; calling it with an empty slice would leave the Actions v2 path serving
// degraded (fail_closed or minimal_safe) output for this user until the next
// rebuild triggers. Preserving the last-known-good compiled claims is
// strictly safer than nuking them on a transient Zitadel blip.
func cacheRebuildProjectIDs(ctx context.Context) ([]string, error) {
	projects, err := directory.Default.Projects(ctx)
	if err != nil {
		return nil, err
	}
	projectIDs := make([]string, 0, len(projects))
	for _, p := range projects {
		projectIDs = append(projectIDs, p.ID)
	}
	return projectIDs, nil
}
