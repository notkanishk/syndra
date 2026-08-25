package db

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// A source guard for the one thing this package cannot test any other way.
//
// `internal/db` has no live-DB harness, so a query that is valid SQL and wrong
// about NULLs fails on an operator's screen rather than in CI. This one did:
// the anchor comparison read `violation_reason` into a plain string, which is
// NULL on every target that has never been tampered with — so the read failed
// every time, the anchor was written once at first sighting and never compared
// against anything again, and a mutation log could have been truncated at any
// point afterwards with nothing noticing.
//
// The rule, stated so it survives the next edit: a nullable column is either
// COALESCEd in the query or scanned into a pointer. Never into a bare value.

// nullableAnchorText is the addon_log_anchors columns that are TEXT and
// nullable. Both carry a finding, so both are absent on a healthy row.
var nullableAnchorText = []string{"violation_reason", "violation_head"}

func TestEveryNullableAnchorColumnIsReadSafely(t *testing.T) {
	src, err := os.ReadFile("log_anchor.go")
	if err != nil {
		t.Fatalf("read source: %v", err)
	}

	// Each SELECT ... FROM addon_log_anchors in the file, with its select list.
	selects := regexp.MustCompile(`(?s)SELECT(.*?)FROM\s+addon_log_anchors`).FindAllStringSubmatch(string(src), -1)
	if len(selects) == 0 {
		t.Fatal("no SELECT against addon_log_anchors found; this guard has stopped guarding anything")
	}

	for _, match := range selects {
		list := match[1]
		for _, col := range nullableAnchorText {
			if !strings.Contains(list, col) {
				continue
			}
			coalesced := regexp.MustCompile(`COALESCE\(\s*` + col + `\s*,`).MatchString(list)
			// The pointer idiom is the other legal one, and it is legible in
			// the same file: `var reason, vHead *string`. A select list that
			// neither coalesces nor sits beside such a declaration is the bug.
			pointerScan := regexp.MustCompile(`var [a-zA-Z, ]*\*string`).MatchString(string(src))
			if !coalesced && !pointerScan {
				t.Errorf("a SELECT reads %s without COALESCE and without a pointer scan:\n%s", col, list)
			}
		}
	}
}

// And the specific query that broke, held to the stricter of the two idioms.
//
// Named separately because the general guard above passes as long as SOME
// pointer scan exists in the file, and this read has none of its own — it scans
// into `prevReason`, a plain string, which is what made a healthy anchor
// unreadable.
func TestTheAnchorComparisonCoalescesItsFinding(t *testing.T) {
	src, err := os.ReadFile("log_anchor.go")
	if err != nil {
		t.Fatalf("read source: %v", err)
	}
	comparison := regexp.MustCompile(`(?s)SELECT([^;]*?)FROM\s+addon_log_anchors\s+WHERE target = \$1\s+FOR UPDATE`).
		FindStringSubmatch(string(src))
	if comparison == nil {
		t.Fatal("the locked anchor read is gone; if it moved, move this guard with it")
	}
	if !strings.Contains(comparison[1], "COALESCE(violation_reason, '')") {
		t.Errorf("the locked read must COALESCE violation_reason — it scans into a bare string:\n%s", comparison[1])
	}
}
