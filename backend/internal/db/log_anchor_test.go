package db

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// 2.28/2.29 — the anchor catches what a hash chain cannot catch about itself.
//
// A chain verifies its own contents. Cut the first thousand records off, re-chain
// from what is left, and every remaining link still verifies — so the only thing
// that notices is an outside observer who remembers where the head was.

func TestATruncatedLogIsDetectedEvenThoughTheChainStillVerifies(t *testing.T) {
	cases := []struct {
		name        string
		prevHead    string
		prevRecords int64
		head        string
		records     int64
		want        string
		because     string
	}{
		{
			name:     "records removed and the remainder re-chained",
			prevHead: "aaa", prevRecords: 1200, head: "zzz", records: 200,
			want:    AnchorRecordsDecreased,
			because: "the re-chained remainder verifies perfectly; only the count remembers the records that are gone",
		},
		{
			name:     "rewritten in place",
			prevHead: "aaa", prevRecords: 1200, head: "bbb", records: 1200,
			want:    AnchorHeadRewritten,
			because: "the same number of records now hash to something else",
		},
		{
			name:     "written past without being chained",
			prevHead: "aaa", prevRecords: 1200, head: "aaa", records: 1400,
			want:    AnchorHeadRewritten,
			because: "the head is a digest over the chain, so more records with the same head cannot happen honestly",
		},
		{
			name:     "ordinary growth",
			prevHead: "aaa", prevRecords: 1200, head: "bbb", records: 1400,
			want: AnchorExtended,
		},
		{
			name:     "an add-on that performed no writes",
			prevHead: "aaa", prevRecords: 1200, head: "aaa", records: 1200,
			want:    AnchorUnchanged,
			because: "an idle add-on reports the same pair, and calling that tampering would cry wolf on every quiet day",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ClassifyLogHead(tc.prevHead, tc.prevRecords, tc.head, tc.records)
			if got != tc.want {
				t.Fatalf("ClassifyLogHead = %q, want %q (%s)", got, tc.want, tc.because)
			}
			if AnchorViolation(got) != (tc.want == AnchorRecordsDecreased || tc.want == AnchorHeadRewritten) {
				t.Errorf("AnchorViolation disagrees with the verdict %q", got)
			}
		})
	}
}

// The anchor must not advance past a violation. Advancing would adopt the
// tampered state as the new baseline and report every subsequent read as
// healthy — which is the one thing an anchor must never do, because the whole
// mechanism is a memory of what was true before.
func TestTheAnchorDoesNotAdvancePastAViolation(t *testing.T) {
	src := readSource(t, "log_anchor.go")

	advance := between(t, src, "UPDATE addon_log_anchors\n\t\t\t\t   SET head =", "WHERE target = $1")
	if strings.Contains(advance, "violation") {
		t.Error("the advancing update touches the violation columns")
	}
	if !strings.Contains(src, `} else if prevReason == "" {`) {
		t.Error("the anchor advances without checking whether the target is already carrying a finding, so a tampered chain catches up the next time it happens to extend")
	}

	flag := between(t, src, "SET violation_reason = $2", "WHERE target = $1")
	for _, col := range []string{"violation_head", "violation_records", "violation_at"} {
		if !strings.Contains(flag, col) {
			t.Errorf("the finding does not record %s, so nobody can see what was reported", col)
		}
	}
	// Word-boundary matched: `violation_records` ends in `records` and would
	// satisfy a substring check, which is how a guard passes while the thing it
	// guards is broken.
	for _, col := range []string{"head", "records", "anchored_at"} {
		if regexp.MustCompile(`(^|[^_a-z])` + col + ` *=`).MatchString(flag) {
			t.Errorf("the violation write moves the anchor's %s", col)
		}
	}
}

// An add-on that reports no head cannot be anchored at all, and recording an
// empty one would make every later empty head compare equal and read as healthy
// for ever.
func TestAnEmptyHeadIsRefusedRatherThanAnchored(t *testing.T) {
	src := readSource(t, "log_anchor.go")
	if !strings.Contains(src, "the target reported no chain head") {
		t.Error("an empty head must be refused, not stored")
	}
}

// The vocabulary is closed in both places, and the two must agree: the CHECK
// constraint is what a surface reads back, and a Go constant it does not permit
// is a write that fails at runtime.
func TestTheViolationVocabularyIsOneVocabulary(t *testing.T) {
	raw, err := os.ReadFile("../../db/migrations/000033_addon_log_anchors.up.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	sql := string(raw)
	for _, reason := range []string{AnchorRecordsDecreased, AnchorHeadRewritten} {
		if !strings.Contains(sql, "'"+reason+"'") {
			t.Errorf("the CHECK does not permit %q, so writing it fails at runtime", reason)
		}
	}
	// And the healthy verdicts must NOT be in it: they are never written to the
	// column, and permitting them would let a healthy reading be recorded as a
	// finding.
	for _, ok := range []string{AnchorExtended, AnchorUnchanged, AnchorFirstSighting} {
		if strings.Contains(sql, "'"+ok+"'") {
			t.Errorf("the CHECK permits %q, which is not a finding", ok)
		}
	}
	// The four violation columns are one fact and must move together.
	if !strings.Contains(sql, "addon_log_anchors_violation_is_whole") {
		t.Error("nothing stops a reason with no timestamp, which describes a moment that did not happen")
	}
}
