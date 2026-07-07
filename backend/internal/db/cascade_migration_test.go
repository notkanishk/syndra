package db

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// TestCascadeAndEnqueue_WriteNoDirectRoleGrants is a source-coherence guard for the
// load-bearing design pivot (global-constraints.md): cascades must NEVER write a
// direct_role_grants row. direct_role_grants has UNIQUE(user,project,role) and the direct
// enqueue path upserts it with ON CONFLICT ... source=EXCLUDED.source — a cascade insert there
// would silently clobber (or be clobbered by) an operator's source='direct' row, breaking the
// OtherSourceCovers coverage check relied on elsewhere. Bundle/rule intent instead lives in
// user_bundle_assignments / bundle_roles / mapping_rules; attribution rides on the outbox row's
// source/source_ref. This test reads cascade.go and fails if any of the atomic *AndEnqueue
// functions ever gain a direct_role_grants write.
func TestCascadeAndEnqueue_WriteNoDirectRoleGrants(t *testing.T) {
	src, err := os.ReadFile("cascade.go")
	if err != nil {
		t.Fatalf("read cascade.go: %v", err)
	}
	body := string(src)
	// Match actual SQL writes, not doc-comment prose (which legitimately explains the pivot) —
	// a substring check on the whole "body" would also swallow the next function's leading
	// doc comment (funcBody cuts at the next `^func` line, not before its comment).
	writePattern := regexp.MustCompile(`(?i)(INSERT INTO|UPDATE|DELETE FROM)\s+direct_role_grants`)
	for _, fn := range []string{
		"AssignBundleAndEnqueue", "AddRoleToBundleAndEnqueue", "CreateMappingRuleAndEnqueue",
		"RemoveBundleFromUserAndEnqueue", "RemoveRoleFromBundleAndEnqueue", "UpdateMappingRuleAndEnqueue",
	} {
		fb := funcBody(t, body, fn)
		if writePattern.MatchString(fb) {
			t.Errorf("%s must NOT write direct_role_grants — cascade intent lives in the bundle/rule tables (design pivot); body:\n%s", fn, fb)
		}
	}
}

// TestEnqueueCascadeRows_WritesSourceAndSourceRef is a coherence guard: the cascade outbox
// insert must carry source/source_ref (migration 000017), or cascade rows would be
// indistinguishable from operator direct grants in the Pending worklist / recent-cascades UI.
func TestEnqueueCascadeRows_WritesSourceAndSourceRef(t *testing.T) {
	src, err := os.ReadFile("cascade.go")
	if err != nil {
		t.Fatalf("read cascade.go: %v", err)
	}
	fb := funcBody(t, string(src), "enqueueCascadeRows")
	for _, want := range []string{"source", "source_ref"} {
		if !strings.Contains(fb, want) {
			t.Errorf("enqueueCascadeRows must write outbox column %q; body:\n%s", want, fb)
		}
	}
}
