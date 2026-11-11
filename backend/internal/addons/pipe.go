package addons

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"

	"syndra/internal/db"
)

// The add-on pipe: the one place a request leaves for a target.
//
// Every call used to hand-copy `allow → credentialFor → [claim] → doAuthenticated
// → record`, about twelve times, so a new endpoint that forgot a step never
// tripped the breaker or skipped its trace. Now there is one copy, and a guard
// test (pipe_guard_test.go) fails if another file in this package reaches the
// transport or the breaker directly.
//
// What the pipe guarantees:
//
//   - the breaker is consulted before the credential is loaded, and every answer
//     is recorded against it (SkipBreaker is the one declared exception);
//   - a Write is refused unless it carries a trace made BEFORE the call: a
//     claimable addon_operations row (Claim) or an outbox row's call id (TraceID);
//   - every Write's body carries its call_id (contract 2: inside the signature,
//     where contract 1 sent an Idempotency-Key header the add-on mostly ignored);
//   - the claim is taken after every other refusal and immediately before the
//     request, so a call that never left never spends its record;
//   - a shape refusal (a contract or field the add-on does not speak, or a 404
//     on a feature's route) is unreached and makes the manifest be read again.
//
// operate adds the record itself (row and audit line, then settle) for the
// mutations that have no row of their own: PutSetting and SetLifecycle.

type sendSpec struct {
	Method, Path string
	Body         []byte
	// Write is anything that changes the target. A read, including a POST read
	// such as /plan, leaves no row and needs no trace.
	Write bool
	// Claim takes the one-shot claim on the operation record.
	Claim func(context.Context) error
	// TraceID is the call id of a durable row made elsewhere (the outbox row for
	// an entitlement apply), already in the body. A Write needs this or Claim.
	TraceID string
	// SkipBreaker leaves the breaker neither consulted nor fed. Only lifecycle
	// uses it: it is the call an operator makes to stop the add-on serving work,
	// and a refusal there must not count as the add-on failing to answer.
	SkipBreaker bool
	// Fresh asks for a read of the app now (feature freshness, design §4): the
	// `fresh=true` query on a GET, `"fresh": true` in an operation's body. Sent
	// only to an add-on declaring the feature, and refused outside the reads in
	// scope (freshScope): never /plan, /apply or /stats.
	Fresh bool
}

// send is the one request path. A non-nil error means nothing left the process;
// the returned response is then an unreached outcome carrying the same error.
func (a *Addon) send(ctx context.Context, s sendSpec) (CallResponse, error) {
	refuse := func(err error) (CallResponse, error) {
		return CallResponse{Outcome: OutcomeUnreached, Err: err}, err
	}
	// A feature's route is called only on an add-on whose manifest declares
	// the feature (design §2 rule 2): an add-on that did not is never asked,
	// and the breaker never hears of it.
	if err := a.declares(featureOf(s.Path)); err != nil {
		return refuse(err)
	}
	if s.Write && s.Claim == nil && s.TraceID == "" {
		return refuse(ErrNoCallRecord)
	}
	if s.Fresh {
		if !freshScope(s) {
			return refuse(fmt.Errorf("%w: %s %s", ErrFreshOutOfScope, s.Method, s.Path))
		}
		// An add-on without the feature is never sent its key (design §2 rule
		// 2): it reads the app as it always does.
		if a.declares("freshness") == nil {
			if s.Method == http.MethodGet {
				s.Path += "?fresh=true"
			} else {
				body, err := withJSONField(s.Body, "fresh", true)
				if err != nil {
					return refuse(err)
				}
				s.Body = body
			}
		}
	}
	if !s.SkipBreaker && !a.br.allow(timeNow()) {
		return refuse(fmt.Errorf("%w: %s", ErrCircuitOpen, a.Registration.Target))
	}
	cred, err := credentialFor(a.Registration)
	if err != nil {
		return refuse(fmt.Errorf("addon %s: %w", a.Registration.Target, err))
	}
	if s.Claim != nil {
		if err := s.Claim(ctx); err != nil {
			return refuse(err)
		}
	}
	resp := doAuthenticated(ctx, cred, s.Method, a.Registration.BaseURL+s.Path, s.Body, callTimeout)
	if resp.Status == http.StatusNotFound && featureRoute(s.Path) && (resp.Code == "" || resp.Code == codeUnknownRoute) {
		// A route the backend called because the manifest declared its
		// feature, answered as no such route: the add-on no longer serves it
		// (design §11 — there is no UNKNOWN_FEATURE code). Nothing was done.
		// A 404 with any other code (no such setting, no adoptable account)
		// is the add-on answering about the request, and stays a refusal.
		resp.Outcome, resp.ShapeRefusal = OutcomeUnreached, true
	}
	if resp.ShapeRefusal {
		log.Printf("[ADDON] %s refused %s %s for its shape (%d %s); reading its manifest again",
			a.Registration.Target, s.Method, s.Path, resp.Status, resp.Code)
		rereadAfterShapeRefusal(a)
	}
	if !s.SkipBreaker {
		a.br.record(timeNow(), resp)
	}
	return resp, nil
}

// featureOf is the feature a path's route belongs to (registry/features.json),
// "" for a core route.
func featureOf(path string) string {
	for _, f := range []struct{ prefix, feature string }{
		{"/operations/", "operations"}, {"/settings", "settings"}, {"/reach", "storage.reach"},
		{"/homes", "storage.homes"}, {"/calls/", "calls"}, {"/stats", "stats"},
	} {
		if strings.HasPrefix(path, f.prefix) {
			return f.feature
		}
	}
	return ""
}

func featureRoute(path string) bool { return featureOf(path) != "" }

// ErrFreshOutOfScope is fresh asked of a request it may not ride on.
var ErrFreshOutOfScope = errors.New("addon: fresh is only for the reads in scope")

// freshScope is a request fresh may ride on (design §4): GET /subjects,
// /values/{field}, /reach, /homes, and an effect: read operation. Never a
// write, so never /apply or a write operation, and never /plan or /stats:
// /plan keeps its meaning, computed against a live read and never cooled
// down, and when the app is read for statistics is the add-on's (§9.1).
func freshScope(s sendSpec) bool {
	switch {
	case s.Write:
		return false
	case s.Method == http.MethodGet:
		return s.Path == "/subjects" || s.Path == "/reach" || s.Path == "/homes" || strings.HasPrefix(s.Path, "/values/")
	default:
		return s.Method == http.MethodPost && strings.HasPrefix(s.Path, "/operations/")
	}
}

// ErrFeatureNotDeclared is a call to a feature's route on an add-on whose
// manifest does not declare the feature. Nothing was sent.
var ErrFeatureNotDeclared = errors.New("addon: the add-on does not declare this feature")

// declares refuses a feature the accepted manifest does not declare; "" (a
// core route) is always allowed. No manifest yet is no feature yet.
func (a *Addon) declares(feature string) error {
	if feature == "" {
		return nil
	}
	m, err := a.Manifest()
	if err != nil {
		return err
	}
	if !m.Declares(feature) {
		return fmt.Errorf("%w: %s does not declare %s", ErrFeatureNotDeclared, a.Registration.Target, feature)
	}
	return nil
}

// operate is a Write that owns its record: begin the row and its audit line
// (committed before anything is sent, and no row means no call), send with the
// row's id as claim and idempotency key, then settle how it ended. A refusal
// settles as unreached, which is what an unclaimed row may settle as.
func (a *Addon) operate(ctx context.Context, p db.AddonOperationParams, s sendSpec) (CallResponse, error) {
	// Before the row: a call that will not be sent leaves no record of one.
	if err := a.declares(featureOf(s.Path)); err != nil {
		return CallResponse{Outcome: OutcomeUnreached, Err: err}, err
	}
	id, err := dbBeginAddonOperation(ctx, p)
	if err != nil {
		// No record, no call: the record is the precondition, not a log of it.
		err = fmt.Errorf("addon %s: no record committed, nothing sent: %w", p.Target, err)
		return CallResponse{Outcome: OutcomeUnreached, Err: err}, err
	}
	s.Write = true
	s.Claim = OperationRecord(id, p.Target, p.Operation, p.SubjectID).consume
	// The row's id is the call_id, in the body like every write's.
	body, err := withJSONField(s.Body, "call_id", id)
	if err != nil {
		err = fmt.Errorf("addon %s: encode %s: %w", p.Target, p.Operation, err)
		_ = dbSettleAddonOperation(ctx, id, db.AddonOpUnreached)
		return CallResponse{Outcome: OutcomeUnreached, Err: err}, err
	}
	s.Body = body
	resp, sendErr := a.send(ctx, s)
	if err := dbSettleAddonOperation(ctx, id, StatusFor(resp.Outcome)); err != nil {
		// The row stays `dispatching`, which is the honest state for "sent, and we
		// cannot say what came back"; the unresolved surface shows it.
		log.Printf("[ADDON] %s/%s call_id=%s dispatched but its outcome was not recorded: %v", p.Target, p.Operation, id, err)
	}
	return resp, sendErr
}

// withJSONField is a JSON object body with key set to v.
func withJSONField(body []byte, key string, v any) ([]byte, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(body, &fields); err != nil {
		return nil, err
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	fields[key] = raw
	return json.Marshal(fields)
}

// StatusOf is the persisted status of an operation's reply: no_change when
// the add-on answered that it changed nothing, else StatusFor its outcome.
func StatusOf(resp CallResponse) string {
	if resp.NoChange && resp.Outcome == OutcomeSucceeded {
		return db.AddonOpNoChange
	}
	return StatusFor(resp.Outcome)
}

// StatusFor maps an outcome onto the persisted addon_operations status. Total
// over AllOutcomes; an outcome nobody has heard of is indeterminate, never
// success, so it lands on the surface a human already watches.
func StatusFor(o Outcome) string {
	switch o {
	case OutcomeSucceeded:
		return db.AddonOpSucceeded
	case OutcomeRejected:
		return db.AddonOpRejected
	case OutcomeUnreached:
		return db.AddonOpUnreached
	}
	return db.AddonOpIndeterminate
}
