package extcall

import (
	"reflect"
	"sort"
	"testing"
	"time"
)

// Parity: the Zitadel outbox and a storage password are two configurations of
// one model. The same tried-count gives the same shape of state, the same
// audit keys, and a next try from the same backoff — only the numbers differ.
func TestRetryModel_ParityAcrossSystems(t *testing.T) {
	now := time.Date(2026, 11, 5, 8, 30, 0, 0, time.UTC)
	for name, p := range map[string]Policy{"outbox": OutboxRetry, "addon resend": AddonResend} {
		next := p.NextTryAfter(now, 1)
		if wait := next.Sub(now); wait < p.Base/2 || wait > p.Base {
			t.Errorf("%s: first wait %v is not Backoff(p, 0) in [%v, %v]", name, wait, p.Base/2, p.Base)
		}
		s := p.Retrying(1, &next)
		if s.State != RetryRetrying || s.Try != 2 || s.Of != p.Attempts || s.NextTry != &next {
			t.Errorf("%s: after one try = %+v, want retrying, try 2 of %d", name, s, p.Attempts)
		}
		if last := p.Retrying(p.Attempts, nil); last.State != RetryWaiting || last.Try != p.Attempts || last.NextTry != nil {
			t.Errorf("%s: after the last try = %+v, want waiting with no next try", name, last)
		}
		keys := func(m map[string]any) []string {
			out := make([]string, 0, len(m))
			for k := range m {
				out = append(out, k)
			}
			sort.Strings(out)
			return out
		}
		if got := keys(p.AuditMark(2)); !reflect.DeepEqual(got, []string{"of", "retried_by", "try"}) {
			t.Errorf("%s: audit mark keys %v", name, got)
		}
		if p.AuditMark(2)["retried_by"] != "automatic" || p.AuditMark(2)["of"] != p.Attempts {
			t.Errorf("%s: audit mark %v", name, p.AuditMark(2))
		}
	}
	// The rules themselves, as the owner set them.
	if AddonResend.Attempts != 2 {
		t.Error("a member's password is sent once more, never more (A-13 §4): the backend keeps no copy")
	}
	if OutboxRetry.Attempts != 6 || OutboxRetry.Base != time.Minute || OutboxRetry.Max != 30*time.Minute {
		t.Errorf("outbox rule changed: %+v", OutboxRetry)
	}
}
