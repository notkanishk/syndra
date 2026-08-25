package db

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// 2.51/2.52's other half: the escalation the drain's terminal rows escalate TO.
//
// The drain's own tests already assert that a spent row goes terminal with its
// error, that lock contention yields without spinning, and that an unreachable
// target costs a probe. What none of them could assert is that anybody ever
// sees it — the runner is a background loop, and a correct terminal row nobody
// reads is retained access with a tidy audit trail.

func TestASpentRevocationEscalatesImmediatelyAndAQueuedOneOnTime(t *testing.T) {
	const threshold = 24 * time.Hour

	cases := []struct {
		name    string
		summary UnconfirmedRevocationSummary
		want    bool
		because string
	}{
		{
			name:    "a fresh queue is not a finding",
			summary: UnconfirmedRevocationSummary{Queued: 3, OldestAge: 5 * time.Minute},
			want:    false,
			because: "escalating on depth alone would make the badge permanent and therefore ignored",
		},
		{
			name:    "one spent row, however new",
			summary: UnconfirmedRevocationSummary{Spent: 1, OldestAge: time.Minute},
			want:    true,
			because: "nothing will dispatch it again, so waiting produces nothing and its age is beside the point",
		},
		{
			name:    "a queue that has stopped draining",
			summary: UnconfirmedRevocationSummary{Queued: 1, OldestAge: 25 * time.Hour},
			want:    true,
			because: "a revocation that has not landed in a day is not draining, it is stuck behind something",
		},
		{
			name:    "exactly at the threshold",
			summary: UnconfirmedRevocationSummary{Queued: 1, OldestAge: threshold},
			want:    true,
			because: "the boundary belongs to the finding: being wrong towards escalation costs a glance",
		},
		{
			name:    "nothing at all",
			summary: UnconfirmedRevocationSummary{},
			want:    false,
		},
		{
			name:    "an old age with nothing behind it",
			summary: UnconfirmedRevocationSummary{OldestAge: 100 * time.Hour},
			want:    false,
			because: "the age of an empty set is not evidence of anything",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.summary.Escalated(threshold); got != tc.want {
				t.Fatalf("Escalated = %t, want %t (%s)", got, tc.want, tc.because)
			}
		})
	}
}

// The listing and the count must describe the same population, or the badge and
// the page it links to disagree — which is the failure mode of every indicator
// that computes its own number.
func TestTheListingAndTheCountShareOnePredicate(t *testing.T) {
	src := readSource(t, "unconfirmed_revocations.go")

	if strings.Count(src, "unconfirmedRevocationPredicate") < 3 {
		// Declared once and used by both, rather than written twice.
		t.Fatal("the listing and the count must share one predicate")
	}
	predicate := between(t, src, "const unconfirmedRevocationPredicate = ", "// ListUnconfirmedRevocations")

	// A withdrawal, and only a withdrawal. Without this it is an outbox listing
	// wearing a revocation's name.
	if !strings.Contains(predicate, "op_type IN ('revoke', 'replace')") {
		t.Error("the predicate must restrict to withdrawals; a replace withdraws whatever its new set omits")
	}
	// And only the rows that have NOT reached the target. `applied` is a
	// revocation that landed, and counting it would make the number never fall.
	for _, terminal := range []string{"applied", "superseded", "abandoned"} {
		if strings.Contains(predicate, "'"+terminal+"'") {
			t.Errorf("the predicate includes %q, which is a revocation that is no longer unconfirmed", terminal)
		}
	}
	if !strings.Contains(predicate, "'failed'") {
		// The spent rows ARE the finding. Excluding them would leave the surface
		// showing only the healthy queue.
		t.Error("the predicate must include the spent rows, which are the whole point of the surface")
	}
}

// The two populations are counted apart. Merged, a healthy queue of five-minute
// old rows hides a revocation that failed permanently three days ago.
func TestTheCountKeepsTheTwoPopulationsApart(t *testing.T) {
	src := readSource(t, "unconfirmed_revocations.go")
	count := between(t, src, "func CountUnconfirmedRevocations", "return s, nil")

	if !strings.Contains(count, `FILTER (WHERE p.status <> 'failed')`) {
		t.Error("the queued count must exclude the spent rows")
	}
	if !strings.Contains(count, `FILTER (WHERE p.status = 'failed')`) {
		t.Error("the spent count must be its own column")
	}
	if strings.Count(count, "FROM propagation_outbox") != 1 {
		// One query rather than three: three would be three moments, and a
		// count of spent rows taken after the oldest age can disagree with it.
		t.Error("the summary must come from one query, or its parts describe different moments")
	}
}

// §23 — the field name and the unit have to agree.
//
// Found on the deployment, not by a test: the surface reported an age of
// 18,000,000,000 for a row eighteen seconds old. `Age` is a `time.Duration`,
// which marshals as NANOSECONDS, so a JSON field called `age_seconds` was off
// by a factor of a billion — and it read as a plausible number, which is the
// kind of wrong nobody notices. No client had read it yet, so nothing was
// broken and everything was ready to be.
//
// The Duration stays, because `Escalated` compares against one and that
// comparison is right. It just does not go on the wire under a name that
// promises seconds.
func TestTheAgeOnTheWireIsInTheUnitItsNameClaims(t *testing.T) {
	row := UnconfirmedRevocation{Age: 90 * time.Second, AgeSeconds: 90}
	encoded, err := json.Marshal(row)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	if got := decoded["age_seconds"]; got != float64(90) {
		t.Errorf("age_seconds must be seconds, got %v", got)
	}

	summary := UnconfirmedRevocationSummary{Queued: 1, OldestAge: 2 * time.Hour, OldestAgeSeconds: 7200}
	encoded, err = json.Marshal(summary)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	if got := decoded["oldest_age_seconds"]; got != float64(7200) {
		t.Errorf("oldest_age_seconds must be seconds, got %v", got)
	}
	// And the Go-side comparison still works on the Duration, which is the one
	// place the unit is not a matter of naming.
	if !summary.Escalated(time.Hour) {
		t.Error("a two-hour-old queued row must still escalate against a one-hour threshold")
	}
}
