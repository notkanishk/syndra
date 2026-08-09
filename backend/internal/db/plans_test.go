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
		{
			name:      "computed for a different request",
			mutate:    func(p *Plan) { p.RequestFingerprint = "0f" + strings.Repeat("a", 62) },
			want:      ErrPlanRequestMismatch,
			predicate: "request_fingerprint = $",
		},
	}

	// The list above is closed, not illustrative. Counting the predicate's own
	// conjuncts is what makes it so: a dimension added to the claim without a
	// case here is a refusal nobody can act on, and a case with no conjunct is a
	// rule enforced in Go while the database grants it. Either way the arrangement
	// this whole change exists to avoid comes back, quietly, in one direction.
	conjuncts := strings.Count(sql, "AND ")
	if conjuncts != len(cases) {
		t.Errorf("the claim carries %d conditions beyond its id but %d are explained; every dimension of a citation needs both",
			conjuncts, len(cases))
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
				{SubjectID: "u1", Fingerprint: "sha256:aaa", Outcome: PlanOutcome{Effect: PlanEffectApply}},
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

// 2.21, 2.22 — no field of a plan outcome accepts a value the backend did not
// choose. A closed struct of free strings was not that guarantee: a submitted
// password IS a string, so `Detail` and `Consequence` were a route into
// `outcome_json` however carefully their first writer avoided it, and no
// character class separates a password from a role name.
//
// So the test is a sentinel, applied to every field in turn by reflection. A
// field added later without a check fails here rather than quietly becoming the
// next route — including a `string` one, which is the case this guard used to
// wave through.
func TestNoFieldOfAPlanOutcomeAcceptsAValueTheBackendDidNotChoose(t *testing.T) {
	const sentinel = "correct-horse-battery-staple"

	valid := PlanOutcome{Effect: PlanEffectApply, GrantIDs: []string{sampleUUID}}
	if err := valid.validate(); err != nil {
		t.Fatalf("the baseline outcome is invalid (%v) — every probe below would pass without proving anything", err)
	}

	typ := reflect.TypeOf(valid)
	for i := range typ.NumField() {
		f := typ.Field(i)
		probe := valid
		field := reflect.ValueOf(&probe).Elem().Field(i)

		switch {
		case f.Type.Kind() == reflect.String:
			field.SetString(sentinel)
		case f.Type.Kind() == reflect.Slice && f.Type.Elem().Kind() == reflect.String:
			field.Set(reflect.ValueOf([]string{sentinel}))
		default:
			t.Errorf("PlanOutcome.%s is a %s: this guard cannot probe it, and JSONB will store whatever it holds",
				f.Name, f.Type.Kind())
			continue
		}

		if err := probe.validate(); !errors.Is(err, ErrInvalidPlan) {
			t.Errorf("PlanOutcome.%s accepted a submitted value (%v) — a secret written there would be marshalled into outcome_json",
				f.Name, err)
		}
		if err := probe.validate(); err != nil && strings.Contains(err.Error(), sentinel) {
			t.Errorf("the refusal for PlanOutcome.%s echoed the value it refused: %v", f.Name, err)
		}
	}
}

// 2.21 — and the refusal happens before any database contact. `PG` is nil in
// this package's tests, so a CreatePlan that validated after opening its
// transaction would panic here rather than fail: the assertion is the ordering,
// not just the error.
func TestCreatePlanRefusesASubmittedValueBeforeItTouchesTheDatabase(t *testing.T) {
	const sentinel = "correct-horse-battery-staple"

	base := func() NewPlan {
		return NewPlan{
			Target: "truenas", Surface: "grants.bulk", CreatedBy: "operator-1", Lifetime: time.Minute,
			Subjects: []NewPlanSubject{{SubjectID: "u1", Fingerprint: "sha256:aaa", Outcome: PlanOutcome{Effect: PlanEffectApply}}},
		}
	}
	smuggle := map[string]func(*NewPlan){
		"as the effect":     func(p *NewPlan) { p.Subjects[0].Outcome.Effect = sentinel },
		"as a grant id":     func(p *NewPlan) { p.Subjects[0].Outcome.GrantIDs = []string{sentinel} },
		"as a snapshot ref": func(p *NewPlan) { p.Subjects[0].SnapshotID = sentinel },
	}
	for name, mutate := range smuggle {
		t.Run(name, func(t *testing.T) {
			p := base()
			mutate(&p)
			_, err := CreatePlan(context.Background(), p)
			if !errors.Is(err, ErrInvalidPlan) {
				t.Fatalf("CreatePlan = %v, want ErrInvalidPlan", err)
			}
			if strings.Contains(err.Error(), sentinel) {
				t.Errorf("the refusal echoed the value it refused: %v", err)
			}
		})
	}
}

// 2.21 — the sentences a rehearsal shows are rendered, not recorded. Persisting
// them would put backend-composed free text on a durable row for no benefit the
// snapshot does not already give, and free text is the shape a secret fits.
func TestAPlanRecordsTheDecisionAndNotItsRendering(t *testing.T) {
	typ := reflect.TypeOf(PlanOutcome{})
	for _, prose := range []string{"Detail", "Consequence", "Name", "Email", "Message", "Summary"} {
		if _, ok := typ.FieldByName(prose); ok {
			t.Errorf("PlanOutcome.%s is a rendering: it belongs to the read that displays a plan, not to the row that authorises one", prose)
		}
	}
}

// 2.17 — the plan's effect vocabulary and the rehearsal's are one vocabulary.
// They cannot be one constant: internal/services imports this package.
func TestPlanEffectsMatchTheRehearsalVocabulary(t *testing.T) {
	src, err := os.ReadFile("../services/bulk.go")
	if err != nil {
		t.Fatalf("read bulk.go: %v", err)
	}
	for _, effect := range []string{PlanEffectApply, PlanEffectNoChange, PlanEffectBlocked} {
		if !validPlanEffect(effect) {
			t.Errorf("%q is declared as a plan effect and refused as one", effect)
		}
		if !regexp.MustCompile(`Effect\w+\s*=\s*"` + effect + `"`).MatchString(string(src)) {
			t.Errorf("a plan may record effect %q, which the rehearsal never produces", effect)
		}
	}
	// A plan states what will happen. What became of it belongs to the outbox
	// row: recording `applied` on an approval would make the approval claim an
	// outcome it cannot know.
	for _, after := range []string{"applied", "failed", "queued", "succeeded"} {
		if validPlanEffect(after) {
			t.Errorf("%q is what became of a plan, not what it approved", after)
		}
	}
}

// 2.17 — the vocabulary is closed at compile time, not merely small at the
// moment it was written. An exported `var PlanEffects = []string{...}` was
// neither: a slice is a mutable package variable, so any package could append
// to it before CreatePlan ran and validation would then admit whatever had been
// added — and the refusal message, spelled from the same slice, would name it
// as legitimate.
func TestTheEffectVocabularyCannotBeWidenedByAnotherPackage(t *testing.T) {
	src, err := os.ReadFile("plans.go")
	if err != nil {
		t.Fatalf("read plans.go: %v", err)
	}
	// Package-level `var Name = []T{...}` / `map[...]`. The error sentinels are
	// vars too, but they are of an interface type with no contents to widen.
	mutable := regexp.MustCompile(`(?m)^var\s+(\w+)\s*=\s*(\[\]|map\[)`)
	for _, m := range mutable.FindAllStringSubmatch(string(src), -1) {
		t.Errorf("package-level %s is a mutable collection: any package can rewrite it before a validation reads it, "+
			"which is not what a closed vocabulary means", m[1])
	}
}

// 2.21 — a uuid is a syntax, not a provenance. The shape check cannot tell a
// grant this database allocated from a value that merely looks like one, so the
// rule that matters is tested where it lives: against the rows the lookup
// actually returned.
func TestAPlanMayOnlyCiteGrantsThisDatabaseAllocatedToThatSubject(t *testing.T) {
	const (
		mine       = "3f2504e0-4f89-11d3-9a0c-0305e82c3301"
		theirs     = "3f2504e0-4f89-11d3-9a0c-0305e82c3302"
		fabricated = "3f2504e0-4f89-11d3-9a0c-0305e82c3303"
	)
	owner := map[string]string{mine: "u1", theirs: "u2"}
	subject := func(ids ...string) []NewPlanSubject {
		return []NewPlanSubject{{SubjectID: "u1", Fingerprint: "sha256:aaa", Outcome: PlanOutcome{Effect: PlanEffectApply, GrantIDs: ids}}}
	}

	if err := matchGrantOwners(subject(mine), owner); err != nil {
		t.Fatalf("a subject citing their own grant was refused: %v", err)
	}
	if err := matchGrantOwners(subject(), owner); err != nil {
		t.Fatalf("a subject citing no grant was refused: %v", err)
	}

	// A uuid-shaped value that names nothing. This is the sentinel the shape
	// check waved through: well-formed, and a reference to no grant at all.
	//
	// The message is asserted, not just the sentinel: "no such grant" and "not
	// this person's grant" are different findings, and reporting the second for
	// the first sends an operator looking for a conflict that does not exist.
	fabricatedErr := matchGrantOwners(subject(fabricated), owner)
	if !errors.Is(fabricatedErr, ErrInvalidPlan) {
		t.Errorf("a fabricated identifier was accepted: %v", fabricatedErr)
	} else if !strings.Contains(fabricatedErr.Error(), "did not allocate") {
		t.Errorf("a fabricated identifier was refused for the wrong reason: %v", fabricatedErr)
	}
	// Someone else's grant, on this subject's row. The apply acts on the rows
	// the plan names, so this is an instruction to mutate a person the operator
	// was never shown.
	err := matchGrantOwners(subject(theirs), owner)
	if !errors.Is(err, ErrInvalidPlan) {
		t.Errorf("a subject citing another person's grant was accepted: %v", err)
	} else if !strings.Contains(err.Error(), "different person") {
		t.Errorf("another person's grant was refused for the wrong reason: %v", err)
	}
	// And the refusal does not disclose whose it was.
	for _, leak := range []string{"u2", theirs} {
		if err != nil && strings.Contains(err.Error(), leak) {
			t.Errorf("the refusal disclosed the other subject's grant: %v", err)
		}
	}
	// One good citation does not license a bad one beside it.
	if err := matchGrantOwners(subject(mine, fabricated), owner); !errors.Is(err, ErrInvalidPlan) {
		t.Errorf("a fabricated identifier passed when cited alongside a real one: %v", err)
	}
}

// 2.21 — an uppercase citation is the same identifier, and must not be refused
// as a fabricated one. Postgres compares uuids after parsing and returns the
// lowercase form, so the SQL lookup finds the row and the Go map misses it: the
// two halves of the check would disagree about a legitimate grant.
//
// Normalising the value rather than the comparison is the part that matters —
// the row is written from the same normalised copy, so every later reader
// compares against what the database returns.
func TestAnUppercaseCitationNamesTheSameGrant(t *testing.T) {
	const lower = "3f2504e0-4f89-11d3-9a0c-0305e82c3301"
	upper := strings.ToUpper(lower)

	subjects := canonicalSubjects([]NewPlanSubject{{
		SubjectID:   "u1",
		Fingerprint: "sha256:aaa",
		SnapshotID:  upper,
		Outcome:     PlanOutcome{Effect: PlanEffectApply, GrantIDs: []string{upper}},
	}})

	if got := subjects[0].Outcome.GrantIDs[0]; got != lower {
		t.Errorf("grant id stored as %q, want the form the database returns", got)
	}
	if got := subjects[0].SnapshotID; got != lower {
		t.Errorf("snapshot id stored as %q, want the form the database returns", got)
	}
	// And so the judgement agrees with the lookup that found the row.
	if err := matchGrantOwners(subjects, map[string]string{lower: "u1"}); err != nil {
		t.Errorf("an uppercase citation of a real grant was refused: %v", err)
	}
	if err := matchSnapshotSubjects(subjects, "truenas", map[string]snapshotRef{lower: {subject: "u1", target: "truenas"}}); err != nil {
		t.Errorf("an uppercase citation of a real snapshot was refused: %v", err)
	}

	// The caller's slice is untouched: normalisation is a copy, or a rehearsal
	// would find its own outcome rewritten under it.
	original := []NewPlanSubject{{SubjectID: "u1", Outcome: PlanOutcome{GrantIDs: []string{upper}}}}
	_ = canonicalSubjects(original)
	if original[0].Outcome.GrantIDs[0] != upper {
		t.Error("canonicalSubjects mutated the caller's rows")
	}
}

// 2.21 — a snapshot must be this subject's desired state for this target. The
// foreign key proves only that the row exists, and existence was never the
// property: one approval, one durable object means the snapshot and the
// fingerprint verifying it describe the same person on the same target.
func TestAPlanMayOnlyCiteASnapshotTakenForThatSubjectAndTarget(t *testing.T) {
	const (
		mine       = "3f2504e0-4f89-11d3-9a0c-0305e82c3301"
		theirs     = "3f2504e0-4f89-11d3-9a0c-0305e82c3302"
		otherTgt   = "3f2504e0-4f89-11d3-9a0c-0305e82c3303"
		fabricated = "3f2504e0-4f89-11d3-9a0c-0305e82c3304"
	)
	taken := map[string]snapshotRef{
		mine:     {subject: "u1", target: "truenas"},
		theirs:   {subject: "u2", target: "truenas"},
		otherTgt: {subject: "u1", target: "zitadel"},
	}
	subject := func(snapshot string) []NewPlanSubject {
		return []NewPlanSubject{{SubjectID: "u1", Fingerprint: "sha256:aaa", SnapshotID: snapshot, Outcome: PlanOutcome{Effect: PlanEffectApply}}}
	}

	if err := matchSnapshotSubjects(subject(mine), "truenas", taken); err != nil {
		t.Fatalf("a subject citing their own snapshot for this target was refused: %v", err)
	}
	// Zitadel plans cite none, and that is not a refusal.
	if err := matchSnapshotSubjects(subject(""), "truenas", taken); err != nil {
		t.Fatalf("a subject citing no snapshot was refused: %v", err)
	}

	for _, tc := range []struct{ name, snapshot, want string }{
		{"a snapshot that was never recorded", fabricated, "did not record"},
		{"another person's desired state", theirs, "different person"},
		{"the same person on another target", otherTgt, "different target"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := matchSnapshotSubjects(subject(tc.snapshot), "truenas", taken)
			if !errors.Is(err, ErrInvalidPlan) {
				t.Fatalf("accepted %s: %v", tc.name, err)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("refused for the wrong reason: %v", err)
			}
			for _, leak := range []string{"u2", "zitadel", tc.snapshot} {
				if strings.Contains(err.Error(), leak) {
					t.Errorf("the refusal disclosed %q: %v", leak, err)
				}
			}
		})
	}
}

// 2.21 — and the lookup runs before anything is written, on the plan's own
// transaction: a plan whose citations cannot be verified must leave no row.
func TestGrantProvenanceIsVerifiedBeforeThePlanIsWritten(t *testing.T) {
	src, err := os.ReadFile("plans.go")
	if err != nil {
		t.Fatalf("read plans.go: %v", err)
	}
	body := string(src)
	insert := strings.Index(body, "INSERT INTO plans")
	if insert < 0 {
		t.Fatal("could not locate the plan INSERT")
	}

	for _, tc := range []struct{ call, query string }{
		{"verifyGrantProvenance(ctx, tx,", `SELECT id, user_id FROM direct_role_grants WHERE id = ANY\(\$1`},
		{"verifySnapshotProvenance(ctx, tx,", `SELECT id, subject_id, target FROM desired_state_snapshots WHERE id = ANY\(\$1`},
	} {
		at := strings.Index(body, tc.call)
		if at < 0 {
			t.Errorf("could not locate %s", tc.call)
			continue
		}
		if at > insert {
			t.Errorf("%s must run before the plan is written, or an unverifiable citation leaves a row behind", tc.call)
		}
		// The read must exist and be scoped to the ids the plan cites. A lookup
		// that returns nothing makes every citation unknown, which refuses
		// every plan; one that ignores its argument makes the judgement
		// meaningless in whichever direction its rows happen to fall.
		if !regexp.MustCompile(tc.query).MatchString(body) {
			t.Errorf("the lookup behind %s must read the cited rows, keyed on the cited identifiers", tc.call)
		}
	}

	// Canonicalisation has to happen before either lookup, not inside one:
	// the same normalised rows are what get written.
	canon := strings.Index(body, "subjects := canonicalSubjects(")
	first := strings.Index(body, "verifyGrantProvenance(ctx, tx,")
	if canon < 0 || canon > first {
		t.Error("identifiers must be canonicalised before they are compared against what the database returns")
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
		// CREATE TABLE plus every later ADD COLUMN: a column added by a
		// subsequent migration is as declared as one in the original body, and
		// reading only the original would make every future addition look like
		// a write against a column that is not there.
		body := createTableBody(t, up, tc.table) + "\n" + addedColumns(t, tc.table)
		for _, col := range strings.Split(m[1], ",") {
			col = strings.TrimSpace(col)
			if !regexp.MustCompile(`(?m)^\s*` + regexp.QuoteMeta(col) + `\s`).MatchString(body) {
				t.Errorf("%s writes column %q, which no migration declares", tc.table, col)
			}
		}
	}

	// The apply's authority is a column, so it has to exist.
	if !regexp.MustCompile(`(?m)^\s*applied_at\s+TIMESTAMPTZ`).MatchString(createTableBody(t, up, "plans")) {
		t.Error("plans must carry applied_at: without it a reviewed diff can be cited and enqueued repeatedly, and the fingerprint check will not notice while the first apply's rows are still queued")
	}
}

// addedColumns returns one line per ADD COLUMN against `table` across every up
// migration, in the shape createTableBody produces, so the two can be searched
// as one declaration.
func addedColumns(t *testing.T, table string) string {
	t.Helper()
	re := regexp.MustCompile(`(?is)ALTER TABLE\s+` + regexp.QuoteMeta(table) +
		`\s+ADD COLUMN(?:\s+IF NOT EXISTS)?\s+(\w+)\s+([^;]*);`)
	var out []string
	for _, m := range re.FindAllStringSubmatch(allUpMigrationsSQL(t), -1) {
		out = append(out, "    "+m[1]+" "+strings.TrimSpace(m[2]))
	}
	return strings.Join(out, "\n")
}
