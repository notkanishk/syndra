package db

import "context"

// CountDemoResidue counts operator rows that reference a demo fixture.
//
// The seeder writes into eight tables and every row it creates points at a demo
// project id or a demo user id. Counting those references answers the question
// SYNDRA_SEED_DEMO cannot: not "did this process seed?" but "is seeded data
// still being served?". Turning the flag off and restarting changes the first
// answer and leaves the second one exactly as it was — which is how a live
// deployment ends up showing three demo bundles and four demo rules with no
// signal anywhere that they are fixtures.
//
// Both id sets come from internal/demo, so adding a fixture project or user to
// the catalog widens this check automatically rather than silently narrowing it.
//
// The count is a signal, not a manifest: it is deliberately a single scalar so
// the caller can render one honest sentence. Operators who want the breakdown
// run scripts/reset-data.sh, which prints per-table counts before deleting
// anything.
func CountDemoResidue(ctx context.Context, projectIDs, userIDs []string) (int, error) {
	// One round trip. Every branch is an index-free scan of a table that holds
	// hundreds of rows at most, and this runs on a 60s UI poll.
	const q = `
		SELECT
			(SELECT count(*) FROM bundle_roles WHERE zitadel_project_id = ANY($1))
		  + (SELECT count(*) FROM mapping_rules
		       WHERE source_zitadel_project_id = ANY($1)
		          OR target_zitadel_project_id = ANY($1))
		  + (SELECT count(*) FROM claim_profiles WHERE zitadel_project_id = ANY($1))
		  + (SELECT count(*) FROM direct_role_grants
		       WHERE zitadel_project_id = ANY($1) OR user_id = ANY($2))
		  + (SELECT count(*) FROM user_bundle_assignments WHERE user_id = ANY($2))
		  + (SELECT count(*) FROM access_requests
		       WHERE requester_user_id = ANY($2) OR zitadel_project_id = ANY($1))
		  + (SELECT count(*) FROM audit_logs
		       WHERE target_zitadel_user_id = ANY($2) OR actor_zitadel_user_id = ANY($2))
	`

	var total int
	if err := PG.QueryRow(ctx, q, projectIDs, userIDs).Scan(&total); err != nil {
		return 0, err
	}
	return total, nil
}
