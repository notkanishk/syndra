package handlers

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"syndra/internal/addons"
	"syndra/internal/db"
	"syndra/internal/services"
)

// Role-to-target mapping CRUD (change `addon-platform` group 7).
//
// Validation is split, and the split is the design. Syndra checks STRUCTURE:
// the field is one the add-on's manifest declares, it is not a lifecycle field,
// and no binding already exists for that (target, project, role, field). The
// add-on checks REFERENCE: that `lab_makers` actually resolves on its target.
// Syndra cannot do the second — it does not know what the value means — so a
// mapping write is validated through the add-on and refused if it cannot
// confirm the referent.
//
// The structural half runs first, and runs even when the add-on is unreachable.
// A field the schema does not declare is wrong whatever the target says, and
// spending a network call to be told so would make an outage look like a
// validation failure.

type mappingRequest struct {
	Target    string `json:"target"`
	ProjectID string `json:"project_id"`
	RoleKey   string `json:"role_key"`
	Field     string `json:"field"`
	Value     string `json:"value"`
	// PlanID cites the rehearsal that showed who this reaches. Required
	// whenever the role has holders — a mapping on a role nobody holds is a
	// definition, and there is nothing to review.
	PlanID string `json:"plan_id,omitempty"`
}

func handleListRoleMappings(w http.ResponseWriter, r *http.Request) {
	rows, err := dbListRoleMappings(r.Context(), r.URL.Query().Get("target"))
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	if rows == nil {
		rows = []db.RoleMapping{}
	}
	jsonResponse(w, http.StatusOK, map[string]any{"mappings": rows})
}

func handleCreateRoleMapping(w http.ResponseWriter, r *http.Request) {
	var req mappingRequest
	if err := decodeJSONStrict(r.Body, &req); err != nil {
		jsonValidationErrorResponse(w, "Invalid JSON payload", map[string]string{"body": err.Error()})
		return
	}
	field, value, _, err := validateMappingAgainstTarget(r.Context(), req.Target, req.Field, req.Value)
	if err != nil {
		writeMappingError(w, err)
		return
	}

	// The normalised pair, never `req`.
	//
	// Through the same path edit and delete take: an approval cited whenever
	// the role has holders, the write and the convergences in one transaction
	// under the access lock. Creating a mapping used to write the row and stop
	// — and because entitlements are DERIVED from mappings, the row alone
	// changed what every holder was entitled to while nothing queued it.
	//
	// Nothing else would have found them either. The periodic reconciler walks
	// existing bindings, so a person who has never been bound to this target is
	// in no list it reads.
	created, converged, err := createMappingAndConverge(r.Context(), db.RoleMapping{
		Target: req.Target, ProjectID: req.ProjectID, RoleKey: req.RoleKey,
		Field: field, Value: value, CreatedBy: resolveActor(r, ""),
	}, resolveActor(r, ""), req.PlanID)
	if err != nil {
		writeMappingPlanError(w, err)
		return
	}
	jsonResponse(w, http.StatusCreated, map[string]any{
		"mapping": created,
		// Queued, never applied. The mapping is written here and the people it
		// reaches are converged by the drain.
		"queued_convergences": converged,
	})
}

type mappingValueRequest struct {
	Value string `json:"value"`
	// PlanID cites the rehearsal that showed who this moves. Required whenever
	// the mapping has holders — an edit that reaches nobody is a change to a
	// definition and there is nothing to review.
	PlanID string `json:"plan_id,omitempty"`
}

func handleUpdateRoleMapping(w http.ResponseWriter, r *http.Request) {
	var req mappingValueRequest
	if err := decodeJSONStrict(r.Body, &req); err != nil {
		jsonValidationErrorResponse(w, "Invalid JSON payload", map[string]string{"body": err.Error()})
		return
	}
	// Read first, so the value is validated against the target this mapping is
	// actually on rather than one the caller names. Only the value moves:
	// re-pointing a mapping at a different role is deleting one binding and
	// creating another, with a different cohort on each side.
	existing, err := dbGetRoleMapping(r.Context(), r.PathValue("id"))
	if err != nil {
		writeMappingError(w, err)
		return
	}
	_, value, _, err := validateMappingAgainstTarget(r.Context(), existing.Target, existing.Field, req.Value)
	if err != nil {
		writeMappingError(w, err)
		return
	}
	applyMappingRequest(w, r, existing, planSurfaceMappingEdit, req.PlanID, value, "updated")
}

type mappingDeleteRequest struct {
	PlanID string `json:"plan_id,omitempty"`
}

func handleDeleteRoleMapping(w http.ResponseWriter, r *http.Request) {
	// A DELETE with a body, because the citation has to travel somewhere and a
	// query parameter is a place approvals end up in browser history and access
	// logs. An EMPTY body is tolerated — a mapping nobody holds needs no
	// citation — and that is the only thing tolerated.
	//
	// The decode error used to be discarded outright, which tolerated far more
	// than the empty body it was written for: a payload with a misspelled key
	// decoded to an empty struct and was acted on as though no plan had been
	// cited, and a malformed one the same. On the one endpoint in this file
	// that removes access, silently. Every other mutation in the product
	// decodes strictly; this was the exception, and it was not a deliberate
	// one.
	var req mappingDeleteRequest
	if r.Body != nil {
		if err := decodeJSONStrict(r.Body, &req); err != nil && !errors.Is(err, io.EOF) {
			jsonValidationErrorResponse(w, "Invalid JSON payload", map[string]string{"body": err.Error()})
			return
		}
	}
	existing, err := dbGetRoleMapping(r.Context(), r.PathValue("id"))
	if err != nil {
		writeMappingError(w, err)
		return
	}
	applyMappingRequest(w, r, existing, planSurfaceMappingDelete, req.PlanID, "", "deleted")
}

// applyMappingRequest is the shared half: count the cohort, spend the approval
// if there is one to spend, make the change, and queue the convergences.
func applyMappingRequest(w http.ResponseWriter, r *http.Request, m db.RoleMapping, surface, planID, newValue, verb string) {
	holders, err := dbMappingHolders(r.Context(), m.ProjectID, m.RoleKey)
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	converged, err := applyMappingChange(r.Context(), m, surface, resolveActor(r, ""), planID, newValue, len(holders))
	if err != nil {
		writeMappingPlanError(w, err)
		return
	}
	jsonResponse(w, http.StatusOK, map[string]any{
		"status": verb,
		// What was queued, not what was applied. The convergences wait for the
		// drain like every other add-on row, and reporting them as done is the
		// one thing a surface must not say.
		"queued_convergences": converged,
	})
}

// handleMappingHolders reports the cohort a mapping edit or delete would move,
// so the surface can show the blast radius before anything lands.
func handleMappingHolders(w http.ResponseWriter, r *http.Request) {
	m, err := dbGetRoleMapping(r.Context(), r.PathValue("id"))
	if err != nil {
		writeMappingError(w, err)
		return
	}
	holders, err := dbMappingHolders(r.Context(), m.ProjectID, m.RoleKey)
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	if holders == nil {
		holders = []string{}
	}
	jsonResponse(w, http.StatusOK, map[string]any{"mapping": m, "holders": holders, "count": len(holders)})
}

type publishMappingRequest struct {
	Target string `json:"target"`
	Note   string `json:"note"`
}

func handlePublishMappingVersion(w http.ResponseWriter, r *http.Request) {
	var req publishMappingRequest
	if err := decodeJSONStrict(r.Body, &req); err != nil {
		jsonValidationErrorResponse(w, "Invalid JSON payload", map[string]string{"body": err.Error()})
		return
	}
	if strings.TrimSpace(req.Target) == "" {
		jsonValidationErrorResponse(w, "target is required", map[string]string{"target": "required"})
		return
	}
	// The same guard its sibling runs, and skipped here it let an unregistered
	// target reach the foreign key — which answers 500 with raw constraint text,
	// the failure validateMappingAgainstTarget exists to prevent.
	if _, err := addonsEntitlementSchema(req.Target); errors.Is(err, addons.ErrNotRegistered) {
		jsonValidationErrorResponse(w,
			fmt.Sprintf("%s is not a registered add-on target", req.Target),
			map[string]string{"target": "unregistered"})
		return
	}
	// A version with no note is a date with no argument.
	//
	// The note is the only record of why this set was the right one, and its
	// entire reader is somebody months later deciding whether to roll back to
	// it. Rolling back to a version whose reason is blank is a guess — which is
	// exactly what the version history exists to stop being necessary. Refused
	// here rather than only in the form, because a surface that asks nicely is
	// a suggestion.
	if strings.TrimSpace(req.Note) == "" {
		jsonValidationErrorResponse(w, "a note is required to publish a version",
			map[string]string{"note": "required"})
		return
	}
	version, err := dbPublishMappingVersion(r.Context(), req.Target, strings.TrimSpace(req.Note), resolveActor(r, ""))
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	jsonResponse(w, http.StatusCreated, map[string]any{"target": req.Target, "version": version})
}

func handleRollbackMappingVersion(w http.ResponseWriter, r *http.Request) {
	target := r.PathValue("target")
	version, err := strconv.Atoi(r.PathValue("version"))
	if err != nil || version < 1 {
		jsonValidationErrorResponse(w, "version must be a positive integer", map[string]string{"version": "invalid"})
		return
	}
	// The citation the rehearsal issued. Carried in the body for the same
	// reason the delete's is: an approval in a query parameter ends up in
	// browser history and access logs.
	var req mappingDeleteRequest
	if r.Body != nil {
		if err := decodeJSONStrict(r.Body, &req); err != nil && !errors.Is(err, io.EOF) {
			jsonValidationErrorResponse(w, "Invalid JSON payload", map[string]string{"body": err.Error()})
			return
		}
	}

	actor := resolveActor(r, "")
	converged, err := rollbackAndConverge(r.Context(), target, version, actor, req.PlanID)
	if err != nil {
		// A citation refusal is not a mapping refusal, and an operator is told
		// which of the two to act on.
		writeMappingPlanError(w, err)
		return
	}
	jsonResponse(w, http.StatusOK, map[string]any{
		"status": "rolled_back", "target": target, "version": version,
		// Restoring the bindings is only half of a rollback. The people who
		// hold the affected roles are still converged to what the reverted set
		// said, and nothing else would ever notice: a rollback that changed the
		// definition and left the target alone is the definition and the world
		// disagreeing, silently, which is exactly what a rollback is for undoing.
		"queued_convergences": converged,
	})
}

// validateMappingAgainstTarget runs both halves of the split, structure first.
// Like the allowance validator, it returns the CANONICAL pair rather than a
// verdict. Both sides of the intersection have to be normalised or neither is:
// a mapping stored as `lab_makers ` is one no allowance can ever match, and a
// carve-out on it would be accepted and do nothing.
func validateMappingAgainstTarget(ctx context.Context, target, field, value string) (string, string, addons.Resolution, error) {
	field, value = services.NormaliseTerm(field, value)
	schema, err := addonsEntitlementSchema(target)
	switch {
	case errors.Is(err, addons.ErrNotRegistered):
		// Nothing about a mapping to a target the deployment does not run can
		// be validated, and the foreign key would refuse the row anyway — with
		// a message about a constraint rather than about the deployment.
		return "", "", addons.Resolution{}, fmt.Errorf("%w: %s is not a registered add-on target", db.ErrMappingInvalid, target)
	case err != nil:
		// Registered and never answered. Structure could still be checked
		// against a manifest we do not have, which is to say it could not.
		return "", "", addons.Resolution{}, fmt.Errorf("%w: %s has not published a capability manifest yet, so its entitlement schema is unknown", errMappingTargetSilent, target)
	}

	declared := make([]string, 0, len(schema))
	for _, f := range schema {
		// A lifecycle field the add-on declares is still not bindable. Both
		// sides say so — the manifest flag and the backend's own list — because
		// the manifest is the least trusted input in the system and this is a
		// rule about Syndra's resolver, not about the target.
		if f.Lifecycle || services.IsLifecycleField(f.Name) {
			continue
		}
		declared = append(declared, f.Name)
	}
	if err := services.ValidateMappingField(declared, field); err != nil {
		return "", "", addons.Resolution{}, err
	}

	// Reference, last, and only once structure holds. This is the network call,
	// and spending it to be told a field is misspelled would make an outage
	// look like a validation failure.
	// The resolution comes back whether or not it refused, because "the value is
	// fine" and "nobody could be asked" are different answers and a surface has
	// to be able to say which one it is showing.
	resolution, err := addonsResolvesValue(ctx, target, field, value)
	if err != nil {
		return "", "", resolution, resolutionCarrier{error: err, known: resolution.Known, value: value}
	}
	return field, value, resolution, nil
}

// errMappingTargetSilent separates "the add-on is wrong" from "the add-on has
// not spoken". An operator whose mapping is refused needs to know which.
var errMappingTargetSilent = errors.New("the target has not published its capabilities")

func writeMappingError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, db.ErrMappingExists):
		jsonErrorResponse(w, http.StatusConflict, "MAPPING_EXISTS", err.Error())
	case errors.Is(err, db.ErrMappingNotFound):
		jsonErrorResponse(w, http.StatusNotFound, "MAPPING_NOT_FOUND", err.Error())
	case errors.Is(err, db.ErrMappingInvalid):
		jsonValidationErrorResponse(w, err.Error(), map[string]string{"mapping": "invalid"})
	case errors.Is(err, errMappingTargetSilent):
		// 503, not 400: the request may be perfectly correct and the backend
		// cannot tell yet. Retrying once the add-on answers is the right move.
		jsonErrorResponse(w, http.StatusServiceUnavailable, "TARGET_CAPABILITIES_UNKNOWN", err.Error())
	case errors.Is(err, addons.ErrValueNotResolvable):
		// The add-on's OWN sentinel, not a local restatement of it. This branch
		// used to test a sentinel declared in this file that nothing ever
		// wrapped, so a mapping naming a group the NAS does not have — an
		// operator typo, the single likeliest mistake on this form — fell
		// through to the default and came back 500 DB_ERROR. The target had
		// answered clearly and the operator was shown an internal error.
		// The near misses, when the add-on enumerated any. A typo is answered by
		// seeing the two names it might have been; being told to try again is
		// the one response that cannot help, since the same question gets the
		// same answer.
		details := map[string]string{"value": "unresolvable"}
		if near := nearestValues(err); near != "" {
			details["near"] = near
		}
		jsonValidationErrorResponse(w, err.Error(), details)
	default:
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
	}
}

// handleMappingHistory is a target's published versions, newest first (9.7/9.8).
//
// The whole history rather than a page of it: a makerspace publishes a mapping
// version a handful of times a term, and paging a list that short would cost an
// operator a click to see the thing they came for.
func handleMappingHistory(w http.ResponseWriter, r *http.Request) {
	history, err := dbListMappingHistory(r.Context(), r.PathValue("target"))
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, history)
}

// resolutionCarrier lets a refusal carry what the add-on enumerated, so the
// surface can name near misses without a second call.
type resolutionCarrier struct {
	error
	known []string
	value string
}

func (r resolutionCarrier) Unwrap() error { return r.error }

// nearestValues names up to three enumerated values that share a prefix with
// what was typed, longest prefix first. Empty when nothing is close — a list of
// every group on the NAS is not a suggestion, it is a haystack.
func nearestValues(err error) string {
	var carrier resolutionCarrier
	if !errors.As(err, &carrier) || len(carrier.known) == 0 {
		return ""
	}
	typed := strings.ToLower(carrier.value)
	type scored struct {
		name string
		n    int
	}
	near := []scored{}
	for _, candidate := range carrier.known {
		n := commonPrefix(typed, strings.ToLower(candidate))
		// Three characters, so an unrelated name starting with the same letter
		// is not offered as a correction.
		if n >= 3 {
			near = append(near, scored{candidate, n})
		}
	}
	sort.SliceStable(near, func(i, j int) bool { return near[i].n > near[j].n })
	names := []string{}
	for i, s := range near {
		if i == 3 {
			break
		}
		names = append(names, s.name)
	}
	return strings.Join(names, ", ")
}

func commonPrefix(a, b string) int {
	n := 0
	for n < len(a) && n < len(b) && a[n] == b[n] {
		n++
	}
	return n
}
