package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// ErrProjectRequired is a write that would give access on a target before the
// target has its Zitadel project. A project is needed to GIVE access, never to
// take it away: removals stay possible whatever the target's state.
var ErrProjectRequired = errors.New("this system has no Zitadel project yet")

// ErrProjectTaken is a project already serving another target.
type ErrProjectTaken struct{ Target string }

func (e ErrProjectTaken) Error() string {
	return fmt.Sprintf("that project already serves %s", e.Target)
}

// ErrMappingsExist is a project change on a target that still has mappings.
// Re-scoping them silently would leave every mapped role matching nothing.
type ErrMappingsExist struct{ Count int }

func (e ErrMappingsExist) Error() string {
	return fmt.Sprintf("this system has %d mapping(s); remove them before changing its project", e.Count)
}

// TargetProject returns a target's Zitadel project, "" when none is set.
func TargetProject(ctx context.Context, target string) (string, error) {
	var project *string
	err := querier(ctx).QueryRow(ctx, `SELECT project_id FROM targets WHERE target = $1`, target).Scan(&project)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", fmt.Errorf("no target %q", target)
	}
	if err != nil {
		return "", fmt.Errorf("read project of %s: %w", target, err)
	}
	if project == nil {
		return "", nil
	}
	return *project, nil
}

// TargetProjects returns every target's project, keyed by target; targets with
// none are absent.
func TargetProjects(ctx context.Context) (map[string]string, error) {
	rows, err := querier(ctx).Query(ctx, `SELECT target, project_id FROM targets WHERE project_id IS NOT NULL`)
	if err != nil {
		return nil, fmt.Errorf("read target projects: %w", err)
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var target, project string
		if err := rows.Scan(&target, &project); err != nil {
			return nil, err
		}
		out[target] = project
	}
	return out, rows.Err()
}

// RequireTargetProject refuses a giving write on a target with no project. The
// built-in Zitadel target is the source of projects, not a system serving one,
// and is never asked for one.
func RequireTargetProject(ctx context.Context, target string) error {
	if target == TargetZitadel {
		return nil
	}
	project, err := TargetProject(ctx, target)
	if err != nil {
		return err
	}
	if project == "" {
		return ErrProjectRequired
	}
	return nil
}

// SetTargetProject sets the Zitadel project a target serves. A Syndra-side
// write: it changes nobody's access by itself.
//
// Both refusals come from the database, not a read before the write: the
// unique key on targets.project_id answers "taken", and the composite key from
// target_role_mappings (ON UPDATE RESTRICT) answers "mappings exist". A check
// written here would be a second definition of rules the schema already holds.
func SetTargetProject(ctx context.Context, target, projectID string) error {
	tag, err := querier(ctx).Exec(ctx, `UPDATE targets SET project_id = $2 WHERE target = $1`, target, projectID)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch {
		case pgErr.Code == "23505" && pgErr.ConstraintName == "targets_project_id_key":
			var holder string
			if qerr := querier(ctx).QueryRow(ctx, `SELECT target FROM targets WHERE project_id = $1`, projectID).Scan(&holder); qerr != nil {
				holder = "another system"
			}
			return ErrProjectTaken{Target: holder}
		case pgErr.Code == "23503" && pgErr.ConstraintName == "target_role_mappings_target_project_fkey":
			var n int
			_ = querier(ctx).QueryRow(ctx, `SELECT count(*) FROM target_role_mappings WHERE target = $1`, target).Scan(&n)
			return ErrMappingsExist{Count: n}
		}
	}
	if err != nil {
		return fmt.Errorf("set project of %s: %w", target, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("no target %q", target)
	}
	return nil
}

// TargetServingProject returns the target a Zitadel project serves, if any. A
// project serves at most one target (targets_project_id_key).
func TargetServingProject(ctx context.Context, projectID string) (string, bool, error) {
	var target string
	err := querier(ctx).QueryRow(ctx, `SELECT target FROM targets WHERE project_id = $1 AND state = 'active'`, projectID).Scan(&target)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("read target serving %s: %w", projectID, err)
	}
	return target, true, nil
}

// PendingApplyExists reports whether a convergence for this subject on this
// target is already queued and not yet sent. The reconcile pass asks before
// queueing one, so a person waiting on an operator gets one row in Pending
// changes rather than a new one every six hours.
func PendingApplyExists(ctx context.Context, target, userID string) (bool, error) {
	const q = `SELECT EXISTS(
		SELECT 1 FROM propagation_outbox
		 WHERE op_type = 'apply' AND target = $1 AND user_id = $2
		   AND status IN ('pending', 'in_flight'))`
	var exists bool
	if err := querier(ctx).QueryRow(ctx, q, target, userID).Scan(&exists); err != nil {
		return false, fmt.Errorf("pending apply exists (%s %s): %w", target, userID, err)
	}
	return exists, nil
}
