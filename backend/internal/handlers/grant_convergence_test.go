package handlers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"syndra/internal/addons"
	"syndra/internal/db"
)

func TestADirectGrantConvergesItsProjectsSystem(t *testing.T) {
	enqueue, converge := dbEnqueueDirectGrantPropagation, svcConvergeProjectChange
	t.Cleanup(func() { dbEnqueueDirectGrantPropagation, svcConvergeProjectChange = enqueue, converge })
	var asked []string
	svcConvergeProjectChange = func(_ context.Context, actor, user, project string) error {
		asked = append(asked, actor+"|"+user+"|"+project)
		return nil
	}

	dbEnqueueDirectGrantPropagation = func(context.Context, db.EnqueueParams) (db.EnqueueResult, error) {
		return db.EnqueueResult{OutboxID: "o1"}, nil
	}
	if _, err := grantDirect(context.Background(), db.EnqueueParams{UserID: "u1", ProjectID: "pNet", GrantedBy: "op_1"}); err != nil {
		t.Fatal(err)
	}
	if len(asked) != 1 || asked[0] != "op_1|u1|pNet" {
		t.Fatalf("a committed grant must converge its project's system, got %v", asked)
	}

	// A grant that did not commit converges nothing.
	asked = nil
	dbEnqueueDirectGrantPropagation = func(context.Context, db.EnqueueParams) (db.EnqueueResult, error) {
		return db.EnqueueResult{}, errors.New("refused")
	}
	if _, err := grantDirect(context.Background(), db.EnqueueParams{UserID: "u1", ProjectID: "pNet"}); err == nil || len(asked) != 0 {
		t.Fatalf("a failed grant must not converge, got err=%v asked=%v", err, asked)
	}
}

// A hold takes effect at the next send, not at the next reconcile up to six
// hours later. So does lifting it.
func TestAHoldAndItsLiftConvergeThatPerson(t *testing.T) {
	stubMappingDeps(t, []addons.EntitlementField{{Name: "group", Type: "string[]"}})
	create, lift, converge := dbCreateAllowance, dbLiftAllowance, svcConvergeSubjectOn
	t.Cleanup(func() { dbCreateAllowance, dbLiftAllowance, svcConvergeSubjectOn = create, lift, converge })
	var asked []string
	svcConvergeSubjectOn = func(_ context.Context, _, user, target string) error {
		asked = append(asked, user+"@"+target)
		return nil
	}
	dbCreateAllowance = func(_ context.Context, a db.Allowance) (db.Allowance, error) { a.ID = "a1"; return a, nil }
	dbLiftAllowance = func(context.Context, string, string) (string, string, error) { return "u1", "truenas", nil }

	rr := httptest.NewRecorder()
	handleCreateAllowance(rr, httptest.NewRequest(http.MethodPost, "/api/v1/allowances", strings.NewReader(
		`{"subject_id":"u1","target":"truenas","field":"group","value":"lab_makers","direction":"deny","reason":"safety review"}`)))
	if rr.Code != http.StatusCreated {
		t.Fatalf("want 201, got %d (%s)", rr.Code, rr.Body.String())
	}
	r := httptest.NewRequest(http.MethodPost, "/api/v1/allowances/a1/lift", nil)
	r.SetPathValue("id", "a1")
	rr = httptest.NewRecorder()
	handleLiftAllowance(rr, r)
	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", rr.Code, rr.Body.String())
	}
	if len(asked) != 2 || asked[0] != "u1@truenas" || asked[1] != "u1@truenas" {
		t.Fatalf("create and lift must each converge u1 on truenas, got %v", asked)
	}
}

// A person Zitadel deactivates is converged on every system they have an
// account on, so the account is switched off at the next send.
func TestADeactivatedPersonIsConvergedWhereTheyHaveAnAccount(t *testing.T) {
	reg, binding, converge, invalidate := addonsRegistered, dbGetTargetBinding, svcConvergeSubjectOn, cacheInvalidateUser
	t.Cleanup(func() {
		addonsRegistered, dbGetTargetBinding, svcConvergeSubjectOn, cacheInvalidateUser = reg, binding, converge, invalidate
	})
	cacheInvalidateUser = func(context.Context, string) error { return nil }
	addonsRegistered = func() []addons.Registration { return []addons.Registration{{Target: "truenas"}, {Target: "door"}} }
	dbGetTargetBinding = func(_ context.Context, target, _ string) (db.TargetBinding, bool, error) {
		return db.TargetBinding{}, target == "truenas", nil
	}
	var asked []string
	svcConvergeSubjectOn = func(_ context.Context, actor, user, target string) error {
		asked = append(asked, actor+"|"+user+"@"+target)
		return nil
	}
	if err := processUserDeactivated(context.Background(), WebhookPayload{EventType: "user_deactivated", UserID: "u1"}); err != nil {
		t.Fatal(err)
	}
	if len(asked) != 1 || asked[0] != "zitadel:user_deactivated|u1@truenas" {
		t.Fatalf("want one convergence on truenas only, got %v", asked)
	}
}

// Only a field the add-on marks per-person can be set for one person.
func TestOnlyPerPersonFieldsCanBeSetForOnePerson(t *testing.T) {
	stubMappingDeps(t, []addons.EntitlementField{
		{Name: "group", Type: "string[]"},
		{Name: "home_quota", Type: "int", PerPerson: true},
	})
	create := dbCreateAllowance
	t.Cleanup(func() { dbCreateAllowance = create })
	dbCreateAllowance = func(_ context.Context, a db.Allowance) (db.Allowance, error) { a.ID = "a1"; return a, nil }

	post := func(field, value string) int {
		rr := httptest.NewRecorder()
		handleCreateAllowance(rr, httptest.NewRequest(http.MethodPost, "/api/v1/allowances", strings.NewReader(
			`{"subject_id":"u1","target":"truenas","field":"`+field+`","value":"`+value+`","direction":"set","reason":"thesis data","review_date":"2027-01-01T00:00:00Z"}`)))
		return rr.Code
	}
	if code := post("group", "Staff"); code != http.StatusBadRequest {
		t.Fatalf("setting a group for one person must be refused, got %d", code)
	}
	if code := post("home_quota", "53687091200"); code != http.StatusCreated {
		t.Fatalf("a per-person limit must be accepted, got %d", code)
	}
}

// A home folder limit is a whole number of bytes; anything else is refused
// before the add-on is asked.
func TestALimitMustBeAWholeNumber(t *testing.T) {
	h := stubMappingDeps(t, []addons.EntitlementField{{Name: "home_quota", Type: "int", Label: "Home folder limit", Single: true}})
	rr := postMapping(`{"target":"truenas","project_id":"p","role_key":"maker","field":"home_quota","value":"lots"}`)
	if rr.Code != http.StatusBadRequest || !strings.Contains(rr.Body.String(), "Home folder limit takes a whole number") {
		t.Fatalf("want a refusal naming the field, got %d %s", rr.Code, rr.Body.String())
	}
	if len(h.resolved) != 0 {
		t.Fatal("the add-on must not be asked about a value that is not a number")
	}
}
