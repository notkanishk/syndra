package db

import (
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"
)

// 8.5/8.6, 8.9/8.10 — the rules an allowance is written and read under.

func denial() Allowance {
	return Allowance{
		SubjectID: "u1", Target: "truenas", Field: "group", Value: "lab_makers",
		Direction: AllowanceDeny, ActorID: "op_1", Reason: "safety review",
	}
}

func at(d time.Duration) *time.Time { t := time.Now().Add(d); return &t }

// The bound that stops a temporary measure becoming permanent by inattention.
// Both forms are valid and neither is optional.
func TestADenialMustCarryAnExpiryOrAReviewDate(t *testing.T) {
	if err := denial().validate(); !errors.Is(err, ErrAllowanceUnbounded) {
		t.Fatalf("neither-bound must be refused, got %v", err)
	}

	withExpiry := denial()
	withExpiry.ExpiresAt = at(24 * time.Hour)
	if err := withExpiry.validate(); err != nil {
		t.Errorf("expiry-only must be accepted: %v", err)
	}

	withReview := denial()
	withReview.ReviewDate = at(30 * 24 * time.Hour)
	if err := withReview.validate(); err != nil {
		t.Errorf("review-date-only must be accepted: %v", err)
	}
}

// The error offers both valid forms and names the per-person permanent path.
// An operator sent to edit the mapping instead would change access for every
// holder of that role — a blast radius disguised as a policy fix.
func TestTheUnboundedRefusalOffersBothFormsAndTheRightPermanentPath(t *testing.T) {
	err := denial().validate()
	msg := err.Error()
	for _, want := range []string{"expiry", "review date", "revoke their role grant"} {
		if !strings.Contains(msg, want) {
			t.Errorf("the refusal must mention %q: %q", want, msg)
		}
	}
	if !strings.Contains(msg, "everyone holding that role") {
		t.Error("it must say why editing the mapping is the wrong lever")
	}
}

// 8.13/8.14 — the additive arm is refused, never silently accepted and ignored.
// A stored allowance that resolves to nothing is worse than one that was never
// accepted, because somebody will read it and believe it applies.
func TestAnAdditiveAllowanceIsRefusedRatherThanIgnored(t *testing.T) {
	additive := denial()
	additive.Direction = AllowanceAllow
	additive.ExpiresAt = at(time.Hour)
	if err := additive.validate(); !errors.Is(err, ErrAllowanceAdditiveUnsupported) {
		t.Fatalf("want ErrAllowanceAdditiveUnsupported, got %v", err)
	}

	unknown := denial()
	unknown.Direction = "maybe"
	unknown.ExpiresAt = at(time.Hour)
	if err := unknown.validate(); !errors.Is(err, ErrAllowanceInvalid) {
		t.Fatalf("an unknown direction must be refused as invalid, got %v", err)
	}
}

// Every field is required, and the refusal names the field rather than the
// value: a value here is an add-on's own vocabulary and the backend does not
// know what may be in it.
func TestAnAllowanceRecordsWhoDecidedAndWhy(t *testing.T) {
	for _, missing := range []string{"subject_id", "target", "field", "value", "actor_id", "reason"} {
		a := denial()
		a.ExpiresAt = at(time.Hour)
		switch missing {
		case "subject_id":
			a.SubjectID = ""
		case "target":
			a.Target = ""
		case "field":
			a.Field = ""
		case "value":
			a.Value = " "
		case "actor_id":
			a.ActorID = ""
		case "reason":
			a.Reason = ""
		}
		err := a.validate()
		if !errors.Is(err, ErrAllowanceInvalid) || !strings.Contains(err.Error(), missing) {
			t.Errorf("a missing %s must be refused by name, got %v", missing, err)
		}
	}
}

// Lapsed, lifted and in force are three states an operator asks about
// differently, and the row survives all of them.
func TestInForceDistinguishesLapsedFromLifted(t *testing.T) {
	now := time.Now()

	live := denial()
	live.ExpiresAt = at(time.Hour)
	if !live.InForce(now) {
		t.Error("an unexpired allowance is in force")
	}

	lapsed := denial()
	lapsed.ExpiresAt = at(-time.Hour)
	if lapsed.InForce(now) {
		t.Error("an expired allowance is not in force")
	}

	lifted := denial()
	lifted.ReviewDate = at(time.Hour)
	lifted.LiftedAt, lifted.LiftedBy = at(-time.Minute), "op_2"
	if lifted.InForce(now) {
		t.Error("a lifted allowance is not in force")
	}

	indefinite := denial()
	indefinite.ReviewDate = at(-time.Hour)
	if !indefinite.InForce(now) {
		t.Error("a passed review date must NOT lift the suspension — surfacing is a prompt, not a lapse")
	}
}

// The whole point of a review date: it surfaces the decision without making
// the decision. Lapsing on a date nobody acted on would restore access by
// inattention, which is the failure the review date exists to prevent running
// backwards.
func TestAPassedReviewDateSurfacesWithoutLifting(t *testing.T) {
	now := time.Now()

	due := denial()
	due.ReviewDate = at(-time.Hour)
	if !due.ReviewDue(now) || !due.InForce(now) {
		t.Fatalf("a passed review date must surface AND stay in force: due=%t inForce=%t", due.ReviewDue(now), due.InForce(now))
	}

	future := denial()
	future.ReviewDate = at(time.Hour)
	if future.ReviewDue(now) {
		t.Error("a review date in the future is not due")
	}

	lifted := denial()
	lifted.ReviewDate = at(-time.Hour)
	lifted.LiftedAt, lifted.LiftedBy = at(-time.Minute), "op_2"
	if lifted.ReviewDue(now) {
		t.Error("a lifted allowance is not waiting for a decision")
	}
}

// Lapsing is recorded, never a delete, and the record and the audit row are one
// write. An allowance is a decision somebody took; removing the row erases the
// only evidence the suspension ever happened.
func TestLapsingIsRecordedAndAuditedInOneStatement(t *testing.T) {
	body := funcBody(t, readDBSource(t, "allowances.go"), "ResolveLapsedAllowance")

	if strings.Contains(body, "DELETE FROM allowances") {
		t.Fatal("a lapsed allowance must be recorded, not deleted")
	}
	if !regexp.MustCompile(`(?s)WITH lapsed AS \(\s*UPDATE allowances.*?\)\s*INSERT INTO audit_logs`).MatchString(body) {
		t.Fatal("the record and its audit row must be one write, or a lapse can be recorded by nothing")
	}
	// The actor is a clock, and the row says so rather than naming whichever
	// sweep happened to run.
	if !strings.Contains(body, "lifted_by = 'expiry_sweep'") {
		t.Error("the sweep must record itself as the actor")
	}
	// Conditional on the row still being lapsed and still in force, so a
	// renewal landing in the window takes the whole thing with it.
	for _, cond := range []string{"lifted_at IS NULL", "expires_at <= NOW()"} {
		if !strings.Contains(body, cond) {
			t.Errorf("the write must be conditional on %q", cond)
		}
	}
}

// The resolver's read compares the expiry in the predicate, so the answer is
// the database's own clock — the same clock the sweep resolves by, and the only
// way the two cannot disagree about the instant a suspension ends.
func TestTheInForceReadUsesTheDatabaseClock(t *testing.T) {
	body := funcBody(t, readDBSource(t, "allowances.go"), "AllowancesInForce")
	for _, cond := range []string{"lifted_at IS NULL", "expires_at IS NULL OR expires_at > NOW()"} {
		if !strings.Contains(body, cond) {
			t.Errorf("the read must narrow on %q", cond)
		}
	}
	if !strings.Contains(body, "subject_id = $1 AND target = $2") {
		t.Error("an allowance is a statement about one subject on one target")
	}
}

// A passed review date surfaces only while the suspension still applies:
// listing one that has already lapsed would ask an operator to decide about
// something that ended on its own.
func TestTheReviewQueueExcludesWhatHasAlreadyEnded(t *testing.T) {
	body := funcBody(t, readDBSource(t, "allowances.go"), "AllowancesDueForReview")
	for _, cond := range []string{"lifted_at IS NULL", "review_date <= NOW()", "expires_at IS NULL OR expires_at > NOW()"} {
		if !strings.Contains(body, cond) {
			t.Errorf("the review queue must narrow on %q", cond)
		}
	}
	if !strings.Contains(body, "direction = 'deny'") {
		t.Error("only a denial has a review date to be due")
	}
}
