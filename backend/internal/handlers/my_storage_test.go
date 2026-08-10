package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"syndra/internal/addons"
	"syndra/internal/auth"
	"syndra/internal/db"
	"syndra/internal/models"
	"syndra/internal/services"
	"syndra/internal/services/addonop"
)

// 2.47's remaining half, and 10.7 — the member-to-backend credential leg.
//
// This is the one request in the system whose BODY is a secret. Everything else
// this change protects is about what the backend writes down; this is about
// what it says out loud on the way past.

const memberSecret = "Correct-Horse-Battery-9!"

type storageHarness struct {
	dispatched []addonop.Request
	recorded   []string
	entitled   bool
	bound      bool
	err        error
	outcome    addons.Outcome
	// unreachable is the target being switched off while the add-on in front of
	// it answers perfectly — the state that decides whether this member is
	// offered a form at all.
	unreachable bool
}

func stubMyStorage(t *testing.T, h *storageHarness) {
	t.Helper()
	resolve, binding, cred, dispatch, record, callable, health :=
		svcResolveEntitlementSet, dbGetTargetBinding, dbHasShadowCredential,
		svcDispatchOperation, svcRecordCredentialSet, addonsCallable, addonsHealth
	t.Cleanup(func() {
		svcResolveEntitlementSet, dbGetTargetBinding, dbHasShadowCredential,
			svcDispatchOperation, svcRecordCredentialSet, addonsCallable, addonsHealth =
			resolve, binding, cred, dispatch, record, callable, health
	})

	svcResolveEntitlementSet = func(context.Context, string, string) (services.EntitlementSet, error) {
		return services.EntitlementSet{
			Fields:    map[string][]string{"group": {"lab_makers"}},
			Lifecycle: services.LifecycleState{Enabled: h.entitled, SMBEnabled: h.entitled},
		}, nil
	}
	dbGetTargetBinding = func(context.Context, string, string) (db.TargetBinding, bool, error) {
		if !h.bound {
			return db.TargetBinding{}, false, nil
		}
		return db.TargetBinding{Target: "truenas", SubjectID: "u1", Username: "ada"}, true, nil
	}
	dbHasShadowCredential = func(context.Context, string) (models.ShadowCredentialStatus, error) {
		return models.ShadowCredentialStatus{}, nil
	}
	svcDispatchOperation = func(_ context.Context, req addonop.Request) (addonop.Result, error) {
		h.dispatched = append(h.dispatched, req)
		if h.err != nil {
			return addonop.Result{}, h.err
		}
		outcome := h.outcome
		if outcome == "" {
			outcome = addons.OutcomeSucceeded
		}
		return addonop.Result{OperationID: "op_1", Outcome: outcome}, nil
	}
	svcRecordCredentialSet = func(_ context.Context, uid, _, _ string) error {
		h.recorded = append(h.recorded, uid)
		return nil
	}
	addonsCallable = func(string) bool { return true }
	addonsHealth = func(context.Context, string) addons.TargetHealth {
		return addons.TargetHealth{Reachable: !h.unreachable}
	}
}

func setCredential(t *testing.T, body string) (*httptest.ResponseRecorder, string) {
	t.Helper()
	var logs bytes.Buffer
	previous := log.Writer()
	log.SetOutput(&logs)
	t.Cleanup(func() { log.SetOutput(previous) })

	r := httptest.NewRequest(http.MethodPost, "/api/v1/me/targets/truenas/credential", strings.NewReader(body))
	r.SetPathValue("target", "truenas")
	// The authenticated person, as `withUserAuth` would have stashed them. The
	// handler takes the subject from here and never from the body, which is the
	// property `password.set` with somebody else's id would otherwise defeat.
	r = r.WithContext(withPrincipal(r.Context(), &auth.Principal{Subject: "u1"}))
	rr := httptest.NewRecorder()
	handleSetMyCredential(rr, r)
	return rr, logs.String()
}

// The value reaches the add-on and appears in nothing this handler emits.
func TestTheSubmittedCredentialAppearsInNoResponseAndNoLogLine(t *testing.T) {
	h := &storageHarness{entitled: true, bound: true}
	stubMyStorage(t, h)

	rr, logs := setCredential(t, `{"password":"`+memberSecret+`"}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", rr.Code, rr.Body.String())
	}

	if len(h.dispatched) != 1 {
		t.Fatalf("want one dispatch, got %d", len(h.dispatched))
	}
	if got, _ := h.dispatched[0].Params["password"].(string); got != memberSecret {
		t.Fatal("the credential must reach the add-on unchanged, or this test proves nothing")
	}
	if strings.Contains(rr.Body.String(), memberSecret) {
		t.Errorf("the response echoes the credential: %s", rr.Body.String())
	}
	if strings.Contains(logs, memberSecret) {
		t.Errorf("a log line carries the credential: %s", logs)
	}
	// And the subject is the actor, never the request. A member-scoped
	// operation binds who it acts ON, not only who may call it.
	if h.dispatched[0].SubjectID != "u1" || h.dispatched[0].ActorID != "u1" {
		t.Errorf("subject and actor must both be the authenticated person: %+v", h.dispatched[0])
	}
}

// A refusal is where a request most often comes back at the caller.
func TestARefusalDoesNotEchoTheCredential(t *testing.T) {
	for _, tc := range []struct {
		name     string
		entitled bool
		bound    bool
	}{
		{"no role reaches the target", false, false},
		{"the account does not exist yet", true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := &storageHarness{entitled: tc.entitled, bound: tc.bound}
			stubMyStorage(t, h)

			rr, logs := setCredential(t, `{"password":"`+memberSecret+`"}`)
			if rr.Code != http.StatusConflict {
				t.Fatalf("want 409, got %d (%s)", rr.Code, rr.Body.String())
			}
			if len(h.dispatched) != 0 {
				t.Error("nothing may be dispatched at an account that cannot receive it")
			}
			if strings.Contains(rr.Body.String(), memberSecret) || strings.Contains(logs, memberSecret) {
				t.Error("the refusal carries the credential")
			}
		})
	}
}

// An unconfirmed outcome must not be reported as success. An operation carrying
// a secret is never auto-retried, so the member is told to check rather than
// told it worked.
func TestAnUnconfirmedCredentialSetIsNotReportedAsDone(t *testing.T) {
	h := &storageHarness{entitled: true, bound: true, outcome: addons.OutcomeIndeterminate}
	stubMyStorage(t, h)

	rr, _ := setCredential(t, `{"password":"`+memberSecret+`"}`)
	if rr.Code != http.StatusAccepted {
		t.Fatalf("want 202, got %d (%s)", rr.Code, rr.Body.String())
	}
	var out map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &out)
	if out["status"] == "set" {
		t.Error("an outcome the target did not confirm must not read as set")
	}
	if len(h.recorded) != 0 {
		t.Error("nothing may be recorded for a change nobody confirmed")
	}
}

// An add-on that answers is not a target that answers, and this view must not
// confuse the two.
//
// Found on the dev deployment: the add-on served its manifest happily while the
// NAS behind it was switched off, so the member page offered the credential form
// and the member learned otherwise after typing a password. The probe reads the
// add-on's health — which is a live call to the target — rather than the
// manifest it published about itself.
func TestAnAnsweringAddOnWithAnUnreachableTargetOffersNoForm(t *testing.T) {
	h := &storageHarness{entitled: true, bound: true, unreachable: true}
	stubMyStorage(t, h)

	r := httptest.NewRequest(http.MethodGet, "/api/v1/me/targets", nil)
	r = r.WithContext(withPrincipal(r.Context(), &auth.Principal{Subject: "u1"}))
	view, err := describeMyTarget(r, "truenas", "u1")
	if err != nil {
		t.Fatal(err)
	}
	if view.Reachable {
		t.Error("a member must not be offered a form whose submission cannot land")
	}
}

// And the refusal is the backend's, not the target's: a credential for an
// unreachable target is refused here, before the value is sent anywhere.
func TestACredentialForAnUnreachableTargetIsNotDispatched(t *testing.T) {
	h := &storageHarness{entitled: true, bound: true, unreachable: true}
	stubMyStorage(t, h)

	rr, logs := setCredential(t, `{"password":"`+memberSecret+`"}`)
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("want 503, got %d (%s)", rr.Code, rr.Body.String())
	}
	if len(h.dispatched) != 0 {
		t.Error("nothing may be sent at a target that cannot receive it")
	}
	if strings.Contains(rr.Body.String(), memberSecret) || strings.Contains(logs, memberSecret) {
		t.Error("the refusal carries the credential")
	}
}

// A password the backend would reject never leaves the process.
func TestAWeakCredentialIsRefusedBeforeItReachesTheTarget(t *testing.T) {
	h := &storageHarness{entitled: true, bound: true}
	stubMyStorage(t, h)

	rr, _ := setCredential(t, `{"password":"short"}`)
	if rr.Code != http.StatusBadRequest && rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("want a validation refusal, got %d (%s)", rr.Code, rr.Body.String())
	}
	if len(h.dispatched) != 0 {
		t.Error("a rejected password must not have already reached the target")
	}
}
