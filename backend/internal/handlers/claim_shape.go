package handlers

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"syndra/internal/claimdebug"
	"syndra/internal/claims"
	"syndra/internal/db"
	"syndra/internal/directory"
	"syndra/internal/models"
	"syndra/internal/services"
)

// Claim shaping endpoints (A-14; contract in openspec/changes/claims-shaping/design.md).
//
//	GET  /api/v1/claim-templates                            the template catalog
//	GET  /api/v1/claim-attributes                           attribute sources and formats
//	GET  /api/v1/projects/{id}/claim-shape                  the project default (read)
//	PUT  /api/v1/projects/{id}/claim-profile                the project default (its one write place)
//	GET  /api/v1/applications/{id}/claim-shape              the app's shape, source, template line
//	PUT  /api/v1/applications/{id}/claim-shape              save the next version
//	POST /api/v1/applications/{id}/claim-shape/preview      the explain step, live and for a draft
//	POST /api/v1/applications/{id}/claim-shape/draft-from   a draft from a template or another app
//	GET  /api/v1/applications/{id}/claim-shape/versions     history
//	POST /api/v1/applications/{id}/claim-shape/versions/{n}/restore  go back (saves it as new)
//	GET|PUT|DELETE /api/v1/applications/{id}/debug          debug mode
//	GET  /api/v1/applications/{id}/debug/sign-ins?after=N   what debug mode saw
//
// Every write drops the data plane's cached shapes for the project, so the
// next token carries the new shape rather than one TTL later.

func handleGetClaimTemplates(w http.ResponseWriter, r *http.Request) {
	jsonResponse(w, http.StatusOK, map[string]any{"templates": claims.Templates()})
}

// handleGetClaimAttributes lists the attribute sources a row may project. The
// UI populates its picker from this rather than a list that would drift.
func handleGetClaimAttributes(w http.ResponseWriter, r *http.Request) {
	jsonResponse(w, http.StatusOK, map[string]any{
		"attributes": claims.Attributes(),
		"formats":    []string{claims.FormatArray, claims.FormatCSV, claims.FormatSpaceDelimited},
	})
}

func handleGetProjectClaimShape(w http.ResponseWriter, r *http.Request) {
	shape, err := svcGetProjectClaimShape(r.Context(), r.PathValue("id"))
	if err != nil {
		viewError(w, err, http.StatusInternalServerError, "CLAIM_SHAPE_ERROR")
		return
	}
	jsonResponse(w, http.StatusOK, shape)
}

func handleSetProjectClaimProfile(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")
	var req services.ProjectDefaultForm
	if err := decodeJSONStrict(r.Body, &req); err != nil {
		jsonValidationErrorResponse(w, "Invalid JSON payload", map[string]string{"body": err.Error()})
		return
	}
	if err := svcSaveProjectClaimProfile(r.Context(), projectID, req); err != nil {
		claimShapeError(w, err)
		return
	}
	invalidateClaimShape(r.Context(), projectID)
	auditClaimShape(r, "claim_profile.updated", projectID)
	handleGetProjectClaimShape(w, r)
}

func handleGetAppClaimShape(w http.ResponseWriter, r *http.Request) {
	shape, seeded, err := svcGetAppClaimShape(r.Context(), r.PathValue("id"))
	if err != nil {
		claimShapeError(w, err)
		return
	}
	if seeded {
		invalidateClaimShape(r.Context(), shape.Application.ProjectID)
	}
	jsonResponse(w, http.StatusOK, shape)
}

func handleSaveAppClaimShape(w http.ResponseWriter, r *http.Request) {
	var req services.SaveClaimShapeRequest
	if err := decodeJSONStrict(r.Body, &req); err != nil {
		jsonValidationErrorResponse(w, "Invalid JSON payload", map[string]string{"body": err.Error()})
		return
	}
	res, err := svcSaveAppClaimShape(r.Context(), r.PathValue("id"), resolveActor(r, ""), req, 0)
	if err != nil {
		claimShapeError(w, err)
		return
	}
	invalidateClaimShape(r.Context(), res.Version.ProjectID)
	auditClaimShape(r, "claim_shape.saved", res.Version.ApplicationID)
	jsonResponse(w, http.StatusOK, res)
}

func handlePreviewAppClaimShape(w http.ResponseWriter, r *http.Request) {
	var req services.PreviewRequest
	if err := decodeJSONStrict(r.Body, &req); err != nil {
		jsonValidationErrorResponse(w, "Invalid JSON payload", map[string]string{"body": err.Error()})
		return
	}
	pv, err := svcPreviewAppClaimShape(r.Context(), r.PathValue("id"), req)
	if err != nil {
		claimShapeError(w, err)
		return
	}
	jsonResponse(w, http.StatusOK, pv)
}

func handleDraftFromAppClaimShape(w http.ResponseWriter, r *http.Request) {
	var req services.DraftFromRequest
	if err := decodeJSONStrict(r.Body, &req); err != nil {
		jsonValidationErrorResponse(w, "Invalid JSON payload", map[string]string{"body": err.Error()})
		return
	}
	d, err := svcDraftFrom(r.Context(), r.PathValue("id"), req)
	if err != nil {
		claimShapeError(w, err)
		return
	}
	jsonResponse(w, http.StatusOK, d)
}

func handleGetAppClaimShapeVersions(w http.ResponseWriter, r *http.Request) {
	hist, err := svcAppClaimShapeHistory(r.Context(), r.PathValue("id"))
	if err != nil {
		claimShapeError(w, err)
		return
	}
	jsonResponse(w, http.StatusOK, map[string]any{"versions": hist})
}

// RestoreClaimShapeRequest carries the I-understand tick for a breaking go-back.
type RestoreClaimShapeRequest struct {
	AcknowledgeBreaking bool `json:"acknowledge_breaking"`
}

func handleRestoreAppClaimShape(w http.ResponseWriter, r *http.Request) {
	n, err := strconv.Atoi(r.PathValue("n"))
	if err != nil || n <= 0 {
		jsonValidationErrorResponse(w, "version must be a positive number", map[string]string{"n": "invalid"})
		return
	}
	var req RestoreClaimShapeRequest
	if err := decodeJSONStrictOrEmpty(r.Body, &req); err != nil {
		jsonValidationErrorResponse(w, "Invalid JSON payload", map[string]string{"body": err.Error()})
		return
	}
	res, err := svcRestoreAppClaimShape(r.Context(), r.PathValue("id"), resolveActor(r, ""), n, req.AcknowledgeBreaking)
	if err != nil {
		claimShapeError(w, err)
		return
	}
	invalidateClaimShape(r.Context(), res.Version.ProjectID)
	auditClaimShape(r, "claim_shape.restored", res.Version.ApplicationID)
	jsonResponse(w, http.StatusOK, res)
}

// claimShapeError answers a claim-shaping failure with a status that says
// whose move it is.
func claimShapeError(w http.ResponseWriter, err error) {
	var invalid *services.ClaimShapeInvalid
	var breaking *services.BreakingUnacknowledged
	switch {
	case errors.As(err, &breaking):
		jsonResponse(w, http.StatusConflict, map[string]any{
			"error": "BREAKING_UNACKNOWLEDGED", "message": breaking.Error(),
			"breaking": breaking.Breaking, "reach": breaking.Reach,
		})
	case errors.As(err, &invalid):
		jsonValidationErrorResponse(w, invalid.Msg, map[string]string{"row": strconv.Itoa(invalid.Row)})
	case errors.Is(err, services.ErrClaimShapeNotFound):
		jsonErrorResponse(w, http.StatusNotFound, "NOT_FOUND", err.Error())
	case errors.Is(err, db.ErrClaimShapeRace):
		jsonErrorResponse(w, http.StatusConflict, "CONFLICT", err.Error())
	default:
		viewError(w, err, http.StatusInternalServerError, "CLAIM_SHAPE_ERROR")
	}
}

// decodeJSONStrictOrEmpty is decodeJSONStrict where an empty body means {}.
func decodeJSONStrictOrEmpty(body io.Reader, dst any) error {
	raw, err := io.ReadAll(io.LimitReader(body, 1<<20))
	if err != nil {
		return err
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil
	}
	return decodeJSONStrict(bytes.NewReader(raw), dst)
}

// invalidateClaimShape drops the data plane's cached shapes for a project.
func invalidateClaimShape(ctx context.Context, projectID string) {
	if err := redisDelKeys(ctx, claimShapeCacheKey(projectID)); err != nil {
		log.Printf("[CLAIM-SHAPE] cache invalidation failed for project=%s: %v (edit lands within TTL)", projectID, err)
	}
}

func auditClaimShape(r *http.Request, action, resourceID string) {
	auditAs(r.Context(), resolveActor(r, ""), resourceID, action, resourceID)
}

func auditAs(ctx context.Context, actor, target, action, resourceID string) {
	if err := dbInsertAuditLog(ctx, actor, target, action, resourceID); err != nil {
		log.Printf("[CLAIM-SHAPE] audit write failed for %s %s: %v", action, resourceID, err)
	}
}

// SeedClaimShapes seeds apps that existed under Actions v1 (see
// services.SeedClaimShapes) and drops the cached shapes of every project it
// wrote into. main runs it at start and hourly.
func SeedClaimShapes(ctx context.Context) error {
	projects, err := svcSeedClaimShapes(ctx)
	for _, p := range projects {
		invalidateClaimShape(ctx, p)
	}
	if len(projects) > 0 {
		log.Printf("[CLAIM-SHAPE] seeded the Actions v1 shape into apps on %d projects", len(projects))
	}
	return err
}

// ---------------------------------------------------------------------------
// Debug mode.

// claimDebug is the one recorder. It explains each capture off the latency
// path and audits a session that ends by itself.
var claimDebug = claimdebug.New(time.Now,
	func(ctx context.Context, c claimdebug.Capture) any {
		if c.ProjectID == "" {
			return nil
		}
		e, err := svcExplainSignIn(ctx, c.ClientID, c.UserID, c.ProjectID)
		if err != nil {
			return map[string]string{"error": err.Error()}
		}
		return e.Checks
	},
	func(s claimdebug.Session) {
		auditAs(context.Background(), "system", s.UserID, "claim_debug.ended", s.ApplicationID)
	},
)

// DebugRequest names whose sign-ins to capture; empty means the caller's own.
type DebugRequest struct {
	UserID string `json:"user_id"`
}

type debugView struct {
	On      bool                `json:"on"`
	Session *claimdebug.Session `json:"session"`
}

func debugState(appID string) debugView {
	s, ok := claimDebug.Session(appID)
	if !ok {
		return debugView{}
	}
	return debugView{On: s.On(time.Now()), Session: &s}
}

func handleGetAppDebug(w http.ResponseWriter, r *http.Request) {
	jsonResponse(w, http.StatusOK, debugState(r.PathValue("id")))
}

// handlePutAppDebug turns debug mode on for exactly one account, or, while
// it's on, switches the account (keeping the end). Audited either way.
func handlePutAppDebug(w http.ResponseWriter, r *http.Request) {
	appID := r.PathValue("id")
	var req DebugRequest
	if err := decodeJSONStrictOrEmpty(r.Body, &req); err != nil {
		jsonValidationErrorResponse(w, "Invalid JSON payload", map[string]string{"body": err.Error()})
		return
	}
	actor := getAdminUserID(r.Context())
	userID := strings.TrimSpace(req.UserID)
	if userID == "" {
		userID = actor
	}
	if userID == "" {
		jsonValidationErrorResponse(w, "Choose whose sign-ins to see.", map[string]string{"user_id": "required"})
		return
	}
	app, ok, err := svcFindApplication(r.Context(), appID)
	if err != nil {
		viewError(w, err, http.StatusInternalServerError, "VIEW_ERROR")
		return
	}
	if !ok {
		jsonErrorResponse(w, http.StatusNotFound, "NOT_FOUND", "application not found")
		return
	}
	if app.ClientID == "" {
		jsonValidationErrorResponse(w, "This app has no OIDC client, so none of its sign-ins reach Syndra.", map[string]string{"id": "no_client"})
		return
	}
	if userID != actor {
		if _, found, err := svcDebugFindUser(r.Context(), userID); err != nil || !found {
			jsonValidationErrorResponse(w, "That person isn't in Zitadel.", map[string]string{"user_id": "unknown"})
			return
		}
	}
	if actor == "" {
		actor = "system"
	}
	s, switched := claimDebug.Start(appID, app.ClientID, userID, actor)
	action := "claim_debug.started"
	if switched {
		action = "claim_debug.switched"
	}
	auditAs(r.Context(), actor, userID, action, appID)
	jsonResponse(w, http.StatusOK, map[string]any{"ends_at": s.EndsAt, "switched": switched, "session": s})
}

func handleDeleteAppDebug(w http.ResponseWriter, r *http.Request) {
	appID := r.PathValue("id")
	actor := resolveActor(r, "")
	if s, stopped := claimDebug.Stop(appID, actor); stopped {
		auditAs(r.Context(), actor, s.UserID, "claim_debug.ended", appID)
	}
	jsonResponse(w, http.StatusOK, debugState(appID))
}

func handleGetAppDebugSignIns(w http.ResponseWriter, r *http.Request) {
	appID := r.PathValue("id")
	var after int64
	if a := r.URL.Query().Get("after"); a != "" {
		n, err := strconv.ParseInt(a, 10, 64)
		if err != nil || n < 0 {
			jsonValidationErrorResponse(w, "after must be a sign-in's seq", map[string]string{"after": "invalid"})
			return
		}
		after = n
	}
	view := debugState(appID)
	jsonResponse(w, http.StatusOK, map[string]any{"on": view.On, "session": view.Session, "sign_ins": claimDebug.SignIns(appID, after)})
}

var (
	svcGetProjectClaimShape    = services.GetProjectClaimShape
	svcSaveProjectClaimProfile = services.SaveProjectClaimProfile
	svcGetAppClaimShape        = services.GetAppClaimShape
	svcSaveAppClaimShape       = services.SaveAppClaimShape
	svcPreviewAppClaimShape    = services.PreviewAppClaimShape
	svcDraftFrom               = services.DraftFrom
	svcAppClaimShapeHistory    = services.AppClaimShapeHistory
	svcRestoreAppClaimShape    = services.RestoreAppClaimShape
	svcSeedClaimShapes         = services.SeedClaimShapes
	svcExplainSignIn           = services.ExplainSignIn
	svcFindApplication         = func(ctx context.Context, appID string) (models.ApplicationCatalog, bool, error) {
		return directory.Default.FindApplication(ctx, appID)
	}
	svcDebugFindUser = func(ctx context.Context, userID string) (models.UserProfile, bool, error) {
		return directory.Default.FindUser(ctx, userID)
	}
)
