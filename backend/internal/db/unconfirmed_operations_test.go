package db

import (
	"os"
	"strings"
	"testing"
)

// A-13 §5: passwords never reach an operator. Every read that feeds Waiting on
// you, its count and Home excludes them, and so does the operator's answer.
func TestPasswordsNeverReachTheOperator(t *testing.T) {
	want := map[string]bool{"password.set": true, "password.rotate": true}
	if len(SelfSettledOperations) != len(want) {
		t.Fatalf("SelfSettledOperations = %v", SelfSettledOperations)
	}
	for _, op := range SelfSettledOperations {
		if !want[op] {
			t.Errorf("unexpected self-settled operation %q", op)
		}
	}
	src, err := os.ReadFile("addon_operations.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, fn := range []string{"ListUnresolvedAddonOperations", "CountUnresolvedAddonOperations", "ResolveAddonOperation"} {
		body := funcBody(t, string(src), fn)
		if !strings.Contains(body, "operation <> ALL(") || !strings.Contains(body, "SelfSettledOperations") {
			t.Errorf("%s must exclude the operations that settle themselves", fn)
		}
	}
}

// The operator's answer has its own path, and that path can never write
// `succeeded`: an answer goes in `resolution`, and `status` moves at most from
// a stale `dispatching` to `indeterminate`.
func TestAnAnswerNeverRewritesTheDispatchAsSuccess(t *testing.T) {
	src, err := os.ReadFile("addon_operations.go")
	if err != nil {
		t.Fatal(err)
	}
	body := funcBody(t, string(src), "ResolveAddonOperation")
	if strings.Contains(body, "succeeded") {
		t.Error("ResolveAddonOperation must never write succeeded")
	}
	if !strings.Contains(body, "SET status = 'indeterminate'") || !strings.Contains(body, "resolution IS NULL") {
		t.Error("the answer moves status only to indeterminate, and only once")
	}
	if !strings.Contains(body, "claimed_at IS NOT NULL") {
		t.Error("a record never sent is not a question anyone answers")
	}
	if !strings.Contains(body, "INSERT INTO audit_logs") {
		t.Error("an answer leaves an audit row in the same transaction")
	}
	// And the dispatch path still refuses anything but `dispatching`.
	settle := funcBody(t, string(src), "SettleAddonOperation")
	if !strings.Contains(settle, "WHERE id = $1 AND status = 'dispatching'") {
		t.Error("the dispatch settle must still move only from dispatching")
	}
}

// The resend is once: the insert names a first send of the same call, and the
// limiter does not count it.
func TestAResendIsBoundToItsFirstSendAndUncounted(t *testing.T) {
	src, err := os.ReadFile("addon_operations.go")
	if err != nil {
		t.Fatal(err)
	}
	insert := funcBody(t, string(src), "insertAddonOperation")
	for _, guard := range []string{"first.attempt = 1", "first.target = $1", "first.operation = $2", "first.subject_id = $4"} {
		if !strings.Contains(insert, guard) {
			t.Errorf("the resend insert is missing %q", guard)
		}
	}
	if !strings.Contains(insert, "extcall.AddonResend.AuditMark(2)") {
		t.Error("the resend's audit row carries the one automatic-retry mark (retried_by automatic, try 2 of 2)")
	}
	count := funcBody(t, string(src), "CountRecentAddonOperations")
	if !strings.Contains(count, "AND retry_of IS NULL") {
		t.Error("the automatic resend must not count against the rate limit")
	}
}
