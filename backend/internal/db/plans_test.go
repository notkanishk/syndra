package db

import (
	"context"
	"errors"
	"os"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"
)

const sampleUUID = "3f2504e0-4f89-11d3-9a0c-0305e82c3301"

func claimablePlan() (Plan, PlanCitation) {
	expires := time.Now().Add(time.Hour)
	return Plan{
			ID:        sampleUUID,
			Target:    "truenas",
			Surface:   "grants.bulk",
			CreatedBy: "operator-1",
			ExpiresAt: &expires,
		}, PlanCitation{
			PlanID:  sampleUUID,
			Target:  "truenas",
			Surface: "grants.bulk",
			Actor:   "operator-1",
		}
}

func claimSQL(t *testing.T) string {
	t.Helper()
	src, err := os.ReadFile("plans.go")
	if err != nil {
		t.Fatalf("read plans.go: %v", err)
	}
	m := regexp.MustCompile("(?s)const claim = `(.*?)`").FindStringSubmatch(string(src))
	if m == nil {
		t.Fatal("could not isolate the claim statement — the guard below would pass by finding nothing")
	}
	return m[1]
}

// 2.17, 2.18 — the conditional UPDATE is the authority and the pure explainer
// only says why it lost, so the two must refuse on the same set of conditions.
// A dimension the explainer checks but the predicate omits is a rule enforced
// in Go and not in the database, which is the arrangement every other guard in
// this change exists to avoid; a dimension the predicate checks but the
// explainer does not is a refusal nobody can act on.
func TestTheClaimPredicateAndTheExplainerRefuseTheSameThings(t *testing.T) {
	sql := claimSQL(t)

	if !strings.Contains(sql, "SET applied_at = NOW()") {
		t.Error("the claim must mark the plan applied — a claim that reads without writing authorises every later apply too")
	}

	base, cite := claimablePlan()
	if err := planRefusal(base, cite, time.Now()); err != nil {
		t.Fatalf("the baseline plan is not claimable (%v) — every case below would pass without proving anything", err)
	}

	cases := []struct {
		name      string
		mutate    func(*Plan)
		want      error
		predicate string
	}{
		{
			name:      "issued for another target",
			mutate:    func(p *Plan) { p.Target = "zitadel" },
			want:      ErrPlanNotCitableHere,
			predicate: "target = $",
		},
		{
			name:      "issued by another surface",
			mutate:    func(p *Plan) { p.Surface = "drift.triage" },
			want:      ErrPlanNotCitableHere,
			predicate: "surface = $",
		},
		{
			name:      "approved by another operator",
			mutate:    func(p *Plan) { p.CreatedBy = "operator-2" },
			want:      ErrPlanNotYours,
			predicate: "created_by = $",
		},
		{
			name:      "already applied",
			mutate:    func(p *Plan) { now := time.Now(); p.AppliedAt = &now },
			want:      ErrPlanAlreadyApplied,
			predicate: "applied_at IS NULL",
		},
		{
			name:      "past its lifetime",
			mutate:    func(p *Plan) { past := time.Now().Add(-time.Minute); p.ExpiresAt = &past },
			want:      ErrPlanExpired,
			predicate: "expires_at > NOW()",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			plan, cite := claimablePlan()
			tc.mutate(&plan)
			if err := planRefusal(plan, cite, time.Now()); !errors.Is(err, tc.want) {
				t.Errorf("planRefusal = %v, want %v", err, tc.want)
			}
			if !strings.Contains(sql, tc.predicate) {
				t.Errorf("the claim predicate does not contain %q, so the database would grant what the explainer calls a refusal", tc.predicate)
			}
		})
	}
}

// 2.18, 2.25 — a provisional plan carries no expiry, and the predicate must let
// a NULL through rather than treating "no deadline" as "deadline passed". An
// outage outlasting the ordinary lifetime must not discard approved intent.
func TestAProvisionalPlanIsNotExpiredByHavingNoDeadline(t *testing.T) {
	plan, cite := claimablePlan()
	plan.Provisional, plan.ExpiresAt = true, nil

	// A year on, and still claimable: the gate is the re-fingerprint, not a clock.
	if err := planRefusal(plan, cite, time.Now().Add(365*24*time.Hour)); err != nil {
		t.Errorf("a provisional plan was refused by the clock: %v", err)
	}
	if !strings.Contains(claimSQL(t), "expires_at IS NULL OR expires_at > NOW()") {
		t.Error("the predicate must admit a NULL expiry, or every provisional plan is unclaimable")
	}
}

// 2.18 — an expiry is a deadline, not a suggestion: the boundary instant is
// past, not still valid. Cheap to get backwards, and the wrong side of it is a
// plan citable forever at exactly its own deadline.
func TestExpiryIsExclusiveAtTheInstant(t *testing.T) {
	plan, cite := claimablePlan()
	at := time.Now()
	plan.ExpiresAt = &at

	if err := planRefusal(plan, cite, at); !errors.Is(err, ErrPlanExpired) {
		t.Errorf("a plan at its own expiry instant = %v, want expired", err)
	}
	if err := planRefusal(plan, cite, at.Add(-time.Nanosecond)); err != nil {
		t.Errorf("a plan a nanosecond before its expiry = %v, want claimable", err)
	}
}

// 2.18 — identity is reported before state. Telling an operator that a plan
// they never approved has expired sends them to re-plan somebody else's work.
func TestARefusalNamesTheIdentityMismatchAheadOfTheState(t *testing.T) {
	plan, cite := claimablePlan()
	past := time.Now().Add(-time.Hour)
	plan.ExpiresAt = &past     // expired
	plan.CreatedBy = "someone" // and never theirs

	if err := planRefusal(plan, cite, time.Now()); !errors.Is(err, ErrPlanNotYours) {
		t.Errorf("err = %v, want the identity mismatch reported first", err)
	}
}

// 2.19 — a citation the backend cannot even parse is answered, not forwarded.
// Postgres rejects a malformed uuid with a type error, and inside the caller's
// transaction that error aborts every statement after it: an apply citing
// rubbish would take its own bookkeeping down instead of being told no.
//
// The nil transaction is the proof. If the refusal reached the database at all,
// this test would panic rather than fail.
func TestAMalformedPlanIdIsRefusedBeforeTheDatabase(t *testing.T) {
	for _, id := range []string{"", "hunter2", "'; DROP TABLE plans; --", sampleUUID + "x", strings.ReplaceAll(sampleUUID, "-", "")} {
		_, _, err := ClaimPlanTx(context.Background(), nil, PlanCitation{PlanID: id, Target: "truenas", Surface: "grants.bulk", Actor: "op"})
		if !errors.Is(err, ErrPlanNotFound) {
			t.Errorf("ClaimPlanTx(%q) = %v, want ErrPlanNotFound", id, err)
		}
	}
}

func TestLooksLikeUUIDAcceptsOnlyTheShapeTheTablesAllocate(t *testing.T) {
	valid := []string{sampleUUID, strings.ToUpper(sampleUUID)}
	for _, s := range valid {
		if !looksLikeUUID(s) {
			t.Errorf("looksLikeUUID(%q) = false, want true", s)
		}
	}
	invalid := []string{
		"",
		sampleUUID[:35],
		sampleUUID + "0",
		strings.Replace(sampleUUID, "-", "0", 1), // dash in the wrong place
		strings.Replace(sampleUUID, "3f25", "3g25", 1),
		"3f2504e0_4f89_11d3_9a0c_0305e82c3301",
	}
	for _, s := range invalid {
		if looksLikeUUID(s) {
			t.Errorf("looksLikeUUID(%q) = true, want false", s)
		}
	}
}

// 2.17 — what a plan must refuse to become. Each of these would produce a row
// that an apply accepts and that means nothing.
func TestAPlanRefusesToPersistWhatCannotBeApproved(t *testing.T) {
	good := func() NewPlan {
		return NewPlan{
			Target:    "truenas",
			Surface:   "grants.bulk",
			CreatedBy: "operator-1",
			Lifetime:  15 * time.Minute,
			Subjects: []NewPlanSubject{
				{SubjectID: "u1", Fingerprint: "sha256:aaa", Outcome: PlanOutcome{Effect: "apply"}},
			},
		}
	}
	if err := good().validate(); err != nil {
		t.Fatalf("the baseline plan is invalid (%v) — every case below would pass vacuously", err)
	}

	cases := []struct {
		name   string
		mutate func(*NewPlan)
	}{
		{"no target", func(p *NewPlan) { p.Target = " " }},
		{"no surface", func(p *NewPlan) { p.Surface = "" }},
		{"no author", func(p *NewPlan) { p.CreatedBy = "" }},
		{"no subjects", func(p *NewPlan) { p.Subjects = nil }},
		{"a subject with no id", func(p *NewPlan) { p.Subjects[0].SubjectID = " " }},
		{"a subject with no fingerprint", func(p *NewPlan) { p.Subjects[0].Fingerprint = "" }},
		{"a subject cited twice", func(p *NewPlan) { p.Subjects = append(p.Subjects, p.Subjects[0]) }},
		{"a malformed snapshot reference", func(p *NewPlan) { p.Subjects[0].SnapshotID = "snapshot-7" }},
		{"confirmed with no lifetime", func(p *NewPlan) { p.Lifetime = 0 }},
		{"confirmed with a negative lifetime", func(p *NewPlan) { p.Lifetime = -time.Minute }},
		{"provisional with no state read", func(p *NewPlan) { p.Provisional, p.Lifetime = true, 0 }},
		{"provisional carrying a lifetime", func(p *NewPlan) { p.Provisional, p.StateReadAt = true, time.Now() }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := good()
			tc.mutate(&p)
			if err := p.validate(); !errors.Is(err, ErrInvalidPlan) {
				t.Fatalf("validate() = %v, want ErrInvalidPlan", err)
			}
		})
	}

	// And the shape that is legitimate must stay legitimate, or the refusals
	// above are just a plan store nobody can write to.
	provisional := good()
	provisional.Provisional, provisional.Lifetime, provisional.StateReadAt = true, 0, time.Now().Add(-48*time.Hour)
	if err := provisional.validate(); err != nil {
		t.Errorf("a provisional plan against a two-day-old read was refused: %v", err)
	}
	withSnapshot := good()
	withSnapshot.Subjects[0].SnapshotID = sampleUUID
	if err := withSnapshot.validate(); err != nil {
		t.Errorf("a plan citing a snapshot was refused: %v", err)
	}
}

// 2.21, 2.22 — the plan store holds no secret, and not because its writers are
// careful. `outcome_json` is JSONB and would take anything, so the guarantee
// has to be that no caller holds a route to it: every field the plan types
// expose is a string the backend decided, and none is a map, an interface, or
// anything else a submitted parameter set could be assigned to.
//
// The realistic mistake is not malice. It is an apply that already holds
// `params` in hand adding one field so the plan can "show what will be sent".
func TestPlanTypesHoldNoFieldASubmittedValueCouldReach(t *testing.T) {
	outcome := reflect.TypeOf(PlanOutcome{})
	for _, typ := range []reflect.Type{outcome, reflect.TypeOf(NewPlanSubject{}), reflect.TypeOf(PlanSubject{})} {
		for i := range typ.NumField() {
			f := typ.Field(i)
			if f.Type == outcome {
				continue // checked on its own account
			}
			switch f.Type.Kind() {
			case reflect.String, reflect.Bool:
			case reflect.Pointer:
				if f.Type.Elem().Kind() != reflect.String {
					t.Errorf("%s.%s is a pointer to %s", typ.Name(), f.Name, f.Type.Elem().Kind())
				}
			case reflect.Slice:
				if f.Type.Elem().Kind() != reflect.String {
					t.Errorf("%s.%s is a slice of %s", typ.Name(), f.Name, f.Type.Elem().Kind())
				}
			default:
				t.Errorf("%s.%s is a %s — an open-ended field on a durable plan row is where a declared secret arrives, "+
					"and the column it lands in is JSONB, which will accept it", typ.Name(), f.Name, f.Type.Kind())
			}
		}
	}
}

// 2.17 — the columns the store writes are the columns the migration declares.
// A write naming a column that is not there fails at runtime, on an operator's
// screen, after the rehearsal they were reading has already been computed.
func TestPlanWritesMatchTheMigratedColumns(t *testing.T) {
	up, _ := addonMigrationSQL(t)
	src, err := os.ReadFile("plans.go")
	if err != nil {
		t.Fatalf("read plans.go: %v", err)
	}

	for _, tc := range []struct{ table, insert string }{
		{"plans", `INSERT INTO plans \(([^)]*)\)`},
		{"plan_subjects", `INSERT INTO plan_subjects \(([^)]*)\)`},
	} {
		m := regexp.MustCompile(tc.insert).FindStringSubmatch(string(src))
		if m == nil {
			t.Fatalf("could not isolate the %s INSERT", tc.table)
		}
		body := createTableBody(t, up, tc.table)
		for _, col := range strings.Split(m[1], ",") {
			col = strings.TrimSpace(col)
			if !regexp.MustCompile(`(?m)^\s*` + regexp.QuoteMeta(col) + `\s`).MatchString(body) {
				t.Errorf("%s writes column %q, which the migration does not declare", tc.table, col)
			}
		}
	}

	// The apply's authority is a column, so it has to exist.
	if !regexp.MustCompile(`(?m)^\s*applied_at\s+TIMESTAMPTZ`).MatchString(createTableBody(t, up, "plans")) {
		t.Error("plans must carry applied_at: without it a reviewed diff can be cited and enqueued repeatedly, and the fingerprint check will not notice while the first apply's rows are still queued")
	}
}
