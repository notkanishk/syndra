package db

import (
	"context"
	"testing"
)

// A demo id as an audit row's actor counts as residue against a real Zitadel
// and not on the stand-in, where operators sign in as the demo identities; a
// demo id as a target counts either way.
func TestCountDemoResidue_ActorsOnlyWhenAsked_Live(t *testing.T) {
	ctx := liveDB(t)
	demoUser, realUser, project := "dr-demo-user", "dr-real-user", "dr-demo-project"
	clean := func() {
		_, _ = PG.Exec(context.Background(), `DELETE FROM audit_logs WHERE actor_zitadel_user_id LIKE 'dr-%' OR target_zitadel_user_id LIKE 'dr-%'`)
	}
	clean()
	t.Cleanup(clean)

	if _, err := PG.Exec(ctx, `
		INSERT INTO audit_logs (actor_zitadel_user_id, target_zitadel_user_id, action) VALUES
			($1, $2, 'grant.created'),  -- a demo identity acting on a real person
			($2, $1, 'grant.created')   -- a real person acting on a demo person`,
		demoUser, realUser); err != nil {
		t.Fatal(err)
	}

	for _, c := range []struct {
		actors bool
		want   int
	}{{true, 2}, {false, 1}} {
		got, err := CountDemoResidue(ctx, []string{project}, []string{demoUser}, c.actors)
		if err != nil || got != c.want {
			t.Errorf("countActors=%v: got %d, %v; want %d", c.actors, got, err, c.want)
		}
	}
}
