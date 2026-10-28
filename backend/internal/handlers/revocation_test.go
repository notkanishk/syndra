package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"syndra/internal/addons"
	"syndra/internal/db"
	"syndra/internal/services/addonop"
)

// 6.17/6.18 — revocation composed out of the two things this target can do,
// with the response saying plainly what neither of them does.

type revokeHarness struct {
	allowances []db.Allowance
	converged  []db.SystemConvergence
	dispatched []addonop.Request
	rotateErr  error
	createErr  error
	// rotateOutcome is the ADD-ON answering. Distinct from rotateErr, which is
	// the backend refusing before anything was sent — the two produce different
	// sentences and the difference was for a while invisible.
	rotateOutcome addons.Outcome
	rotateReason  error
}

func stubRevocation(t *testing.T) *revokeHarness {
	t.Helper()
	h := &revokeHarness{}
	create, record, dispatch, resolve, unfinished := dbCreateAllowance, dbRecordSystemConvergence, svcDispatchOperation, svcResolveEntitlementsFor, dbHasUnfinishedConvergence
	t.Cleanup(func() {
		dbCreateAllowance, dbRecordSystemConvergence, svcDispatchOperation, svcResolveEntitlementsFor = create, record, dispatch, resolve
		dbHasUnfinishedConvergence = unfinished
	})
	// What the outbox would say: a convergence queued earlier is still waiting.
	dbHasUnfinishedConvergence = func(context.Context, string, string) (bool, error) {
		return len(h.converged) > 0, nil
	}

	dbCreateAllowance = func(_ context.Context, a db.Allowance) (db.Allowance, error) {
		if h.createErr != nil {
			return db.Allowance{}, h.createErr
		}
		a.ID = "allow_1"
		h.allowances = append(h.allowances, a)
		return a, nil
	}
	dbRecordSystemConvergence = func(_ context.Context, c db.SystemConvergence) (string, string, error) {
		h.converged = append(h.converged, c)
		return "plan_1", "outbox_1", nil
	}
	svcDispatchOperation = func(_ context.Context, req addonop.Request) (addonop.Result, error) {
		h.dispatched = append(h.dispatched, req)
		if h.rotateErr != nil {
			return addonop.Result{}, h.rotateErr
		}
		outcome := h.rotateOutcome
		if outcome == "" {
			outcome = addons.OutcomeSucceeded
		}
		return addonop.Result{OperationID: "op_1", Outcome: outcome, Err: h.rotateReason}, nil
	}
	svcResolveEntitlementsFor = func(context.Context, string, string) (map[string]json.RawMessage, error) {
		return map[string]json.RawMessage{"enabled": json.RawMessage(`false`)}, nil
	}
	return h
}

func revoke(body string) *httptest.ResponseRecorder {
	rr := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/v1/targets/truenas/users/u1/revoke-access", strings.NewReader(body))
	r.SetPathValue("target", "truenas")
	r.SetPathValue("id", "u1")
	handleRevokeTargetAccess(rr, r)
	return rr
}

// Both halves, from one action. Either alone is a revocation that does not
// revoke: the allowance leaves a held credential working on reconnect, and the
// rotation alone leaves the account able to authenticate with a new one.
func TestARevocationProducesBothHalves(t *testing.T) {
	h := stubRevocation(t)

	rr := revoke(`{"reason":"under investigation","confirmed":true}`)
	if rr.Code != http.StatusAccepted {
		t.Fatalf("want 202, got %d (%s)", rr.Code, rr.Body.String())
	}

	if len(h.allowances) != 1 {
		t.Fatalf("want the disabling allowance, got %d", len(h.allowances))
	}
	a := h.allowances[0]
	if a.Direction != db.AllowanceDeny || a.Field != "enabled" {
		t.Errorf("the suspension must be a denial on the lifecycle field: %+v", a)
	}
	if a.ReviewDate == nil {
		// An indefinite suspension with no review date is one nobody ever looks
		// at again.
		t.Error("an unbounded suspension must carry a review date")
	}
	if a.Reason != "under investigation" {
		t.Errorf("reason = %q", a.Reason)
	}

	if len(h.dispatched) != 1 || h.dispatched[0].Operation != "password.rotate" {
		t.Fatalf("want the credential rotated, got %+v", h.dispatched)
	}
	if len(h.converged) != 1 {
		t.Fatalf("the suspension must be queued to the target, got %d", len(h.converged))
	}
}

// The copy is the point of 6.17. An operator reading "revoked" and assuming the
// session ended is the failure this endpoint exists to prevent, and the sentence
// lives in the backend because it is a statement about what the system did.
func TestTheResponseSaysEstablishedSessionsEndOnReconnect(t *testing.T) {
	stubRevocation(t)

	rr := revoke(`{"reason":"offboarding","confirmed":true}`)
	body := rr.Body.String()
	if !strings.Contains(body, "reconnect") {
		t.Errorf("the response must say when established sessions end: %s", body)
	}
	for _, forbidden := range []string{"immediately", "disconnected", "terminated"} {
		if strings.Contains(strings.ToLower(body), forbidden) {
			t.Errorf("the response implies session termination this target cannot perform (%q): %s", forbidden, body)
		}
	}
	// And it must not report the target as already changed: the convergence is
	// queued and the drain has not run.
	if !strings.Contains(body, `"queued":true`) {
		t.Errorf("the suspension is queued, not applied: %s", body)
	}
}

// Unconfirmed, it does nothing — and the refusal is where the operator reads
// what this does and does not do.
func TestAnUnconfirmedRevocationDoesNothingAndExplainsItself(t *testing.T) {
	h := stubRevocation(t)

	rr := revoke(`{"reason":"offboarding"}`)
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("want 422, got %d (%s)", rr.Code, rr.Body.String())
	}
	if len(h.allowances) != 0 || len(h.dispatched) != 0 {
		t.Error("nothing may happen without the confirmation")
	}
	if !strings.Contains(rr.Body.String(), "reconnect") {
		t.Errorf("the confirmation must state what it cannot do: %s", rr.Body.String())
	}
}

// A failed rotation leaves the access already withdrawn and names the half that
// is outstanding. Reporting the whole thing as failed would invite a retry that
// rotates twice.
func TestAFailedRotationLeavesTheSuspensionStandingAndSaysWhatIsLeft(t *testing.T) {
	h := stubRevocation(t)
	h.rotateErr = errors.New("the target could not be reached")

	rr := revoke(`{"reason":"offboarding","confirmed":true}`)
	if rr.Code != http.StatusAccepted {
		t.Fatalf("want 202, got %d (%s)", rr.Code, rr.Body.String())
	}
	if len(h.allowances) != 1 {
		t.Error("the suspension must survive a failed rotation")
	}
	body := rr.Body.String()
	if !strings.Contains(body, "partially_revoked") || !strings.Contains(body, "password.rotate") {
		t.Errorf("the outstanding half must be named: %s", body)
	}
	if !strings.Contains(body, `"rotated":false`) {
		t.Errorf("a rotation that did not happen must not read as one that did: %s", body)
	}
}

// A revocation that failed after the deny committed must be finishable by
// retrying it. The retry meets the same hold already in force; refusing that
// would leave the lock unqueued and the credential unrotated for good.
func TestARetryAfterAFailedConvergenceFinishesTheRevocation(t *testing.T) {
	h := stubRevocation(t)
	create := dbCreateAllowance
	record := dbRecordSystemConvergence

	// First attempt: the deny commits, then queuing the convergence fails.
	dbRecordSystemConvergence = func(context.Context, db.SystemConvergence) (string, string, error) {
		return "", "", errors.New("outbox unavailable")
	}
	rr := revoke(`{"reason":"offboarding","confirmed":true}`)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("first attempt: want 500, got %d (%s)", rr.Code, rr.Body.String())
	}
	if len(h.dispatched) != 0 {
		t.Fatal("a convergence that was not queued must not be followed by a rotation")
	}

	// Retry: the same deny is now in force; the convergence works.
	dbRecordSystemConvergence = record
	dbCreateAllowance = func(ctx context.Context, a db.Allowance) (db.Allowance, error) {
		got, _ := create(ctx, a)
		return got, db.ErrAllowanceAlreadyInForce
	}
	rr = revoke(`{"reason":"offboarding","confirmed":true}`)
	if rr.Code != http.StatusAccepted {
		t.Fatalf("retry: want 202, got %d (%s)", rr.Code, rr.Body.String())
	}
	if len(h.converged) != 1 {
		t.Errorf("the retry must queue the convergence, got %d", len(h.converged))
	}
	if len(h.dispatched) != 1 || h.dispatched[0].Operation != "password.rotate" {
		t.Errorf("the retry must rotate the credential, got %+v", h.dispatched)
	}
	if !strings.Contains(rr.Body.String(), `"allowance_id":"allow_1"`) {
		t.Errorf("the response must name the allowance already in force: %s", rr.Body.String())
	}
}

// Retrying carries on past a hold that is permanent, and only past that: a hold
// with an expiry is somebody's time-boxed pause, and "revoke" does not ride on
// it (nor rotate the credential behind it).
func TestARetryOnlyCarriesOnPastAPermanentHold(t *testing.T) {
	h := stubRevocation(t)
	lapses := time.Now().Add(24 * time.Hour)
	dbCreateAllowance = func(_ context.Context, a db.Allowance) (db.Allowance, error) {
		a.ID, a.ExpiresAt = "allow_1", &lapses
		return a, db.ErrAllowanceAlreadyInForce
	}

	rr := revoke(`{"reason":"offboarding","confirmed":true}`)
	if rr.Code != http.StatusConflict {
		t.Fatalf("an expiring hold must answer 409, got %d (%s)", rr.Code, rr.Body.String())
	}
	if len(h.converged) != 0 || len(h.dispatched) != 0 {
		t.Errorf("nothing may be queued or rotated behind a hold that will lapse: %+v %+v", h.converged, h.dispatched)
	}
}

// Retrying twice must not queue the lock twice: the second finds the first
// still waiting. The rotation is the half a retry exists to finish, so it runs
// every time.
func TestADoubleRetryQueuesOneConvergence(t *testing.T) {
	h := stubRevocation(t)
	dbCreateAllowance = func(_ context.Context, a db.Allowance) (db.Allowance, error) {
		a.ID = "allow_1" // permanent: ExpiresAt stays nil
		return a, db.ErrAllowanceAlreadyInForce
	}

	for i := 1; i <= 2; i++ {
		if rr := revoke(`{"reason":"offboarding","confirmed":true}`); rr.Code != http.StatusAccepted {
			t.Fatalf("retry %d: want 202, got %d (%s)", i, rr.Code, rr.Body.String())
		}
	}
	if len(h.converged) != 1 {
		t.Errorf("want one convergence for two retries, got %d", len(h.converged))
	}
	if len(h.dispatched) != 2 {
		t.Errorf("each retry must still rotate, got %d rotations", len(h.dispatched))
	}
}

// The order matters: the durable half first. Rotating first and failing to
// record the suspension leaves a member with a credential they cannot use and no
// record of why.
func TestTheSuspensionIsRecordedBeforeTheCredentialIsRotated(t *testing.T) {
	h := stubRevocation(t)
	h.createErr = errors.New("the allowance could not be written")

	rr := revoke(`{"reason":"offboarding","confirmed":true}`)
	if rr.Code == http.StatusAccepted {
		t.Fatalf("a revocation that recorded nothing must not report success: %s", rr.Body.String())
	}
	if len(h.dispatched) != 0 {
		t.Error("nothing may be rotated for a suspension that was never recorded")
	}
}

// 9.19 — a revocation says it drains on its own, and a grant says it waits.
// Neither should require an operator to infer it from the drain rule.
func TestARevocationSaysItDrainsWithoutAnOperator(t *testing.T) {
	stubRevocation(t)

	rr := revoke(`{"reason":"offboarding","confirmed":true}`)
	body := rr.Body.String()

	if !strings.Contains(body, "drains on its own") {
		t.Errorf("a revocation must say it does not wait: %s", body)
	}
	if strings.Contains(body, "resumes the drain") {
		// That is the grant sentence. Shown here it would send an operator
		// looking for a button that should not exist.
		t.Errorf("a revocation must not tell an operator to resume anything: %s", body)
	}
}

// The sentence a human reads must agree with the fields a machine reads.
//
// Found on the dev deployment: a rotation the add-on REFUSED produced
// `rotated: false` beside "the credential has been replaced". The fallback copy
// was the success copy, and it applied to every case where the add-on answered
// rather than the dispatch failing — which is most of them.
func TestARefusedRotationNeverSaysTheCredentialWasReplaced(t *testing.T) {
	cases := []struct {
		name    string
		outcome addons.Outcome
		reason  error
		want    string
	}{
		{"rejected", addons.OutcomeRejected, errors.New("the bound account no longer exists on the target"), "no longer exists"},
		{"unreached", addons.OutcomeUnreached, nil, "could not be reached"},
		{"indeterminate", addons.OutcomeIndeterminate, nil, "did not confirm"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := stubRevocation(t)
			h.rotateOutcome, h.rotateReason = c.outcome, c.reason

			rr := revoke(`{"reason":"offboarding","confirmed":true}`)
			body := rr.Body.String()
			if rr.Code != http.StatusAccepted {
				t.Fatalf("want 202, got %d (%s)", rr.Code, body)
			}
			if strings.Contains(body, "credential has been replaced") {
				t.Errorf("a rotation that did not happen must not be described as one that did: %s", body)
			}
			if !strings.Contains(body, c.want) {
				t.Errorf("want the copy to say %q: %s", c.want, body)
			}
			// And the half that DID happen is still stated first — the failure
			// must not read as "the revocation failed".
			if !strings.Contains(body, "New connections are refused now") {
				t.Errorf("the suspension is real and must be stated: %s", body)
			}
		})
	}
}

// An unconfirmed rotation must not tell an operator to try again. A second
// rotation on an account that did rotate locks the member out of it.
func TestAnUnconfirmedRotationDoesNotAskForARetry(t *testing.T) {
	h := stubRevocation(t)
	h.rotateOutcome = addons.OutcomeIndeterminate

	body := revoke(`{"reason":"offboarding","confirmed":true}`).Body.String()
	if strings.Contains(body, "Try the rotation again") {
		t.Errorf("an unconfirmed rotation is the one case where retrying is wrong: %s", body)
	}
}

// §17 — the queued lock declares that it only withdraws, and the disclosure
// depends on it.
//
// Without the declaration the background runner cannot claim the row: it drains
// `revoke` rows and add-on withdrawals that say so, and an undeclared `apply`
// waits for an operator. The response tells the operator the opposite, so the
// flag and the sentence have to be checked together — separately, each looks
// right.
func TestTheQueuedLockDeclaresItselfAWithdrawal(t *testing.T) {
	h := stubRevocation(t)
	rr := revoke(`{"reason":"offboarding","confirmed":true}`)
	if rr.Code != http.StatusAccepted {
		t.Fatalf("want 202, got %d: %s", rr.Code, rr.Body.String())
	}
	if len(h.converged) != 1 {
		t.Fatalf("want one convergence, got %d", len(h.converged))
	}
	if !h.converged[0].WithdrawsOnly {
		t.Error("the lock must declare itself a withdrawal, or nothing claims it and the disclosure below is false")
	}
	if !strings.Contains(rr.Body.String(), "drains on its own") {
		t.Error("the disclosure is the promise the declaration keeps; one without the other is the bug")
	}
}
