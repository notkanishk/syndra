package db

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// Roles that wait for you (reconcile N14 option B). A marked role waits in
// Pending changes whenever anyone would gain or lose it; every other role
// goes straight to Zitadel.

// ConfigKeyConfirmationModel says which rule decides delivery: "" (the old
// per-bundle and per-rule modes, with marked roles waiting as well) until the
// operator switches over, then "roles" (only marked roles wait).
const ConfigKeyConfirmationModel = "confirmation.model"

// ConfirmationModelRoles is the value once the operator has switched over.
const ConfirmationModelRoles = "roles"

// Why a row waits (propagation_outbox.wait_reason).
const (
	WaitReasonRoleWaits = "role_waits"
	WaitReasonManual    = "manual"
	waitReasonHalted    = "halted:"
)

// WaitReasonHalted is a row that was meant to go and whose send stopped.
func WaitReasonHalted(reason string) string { return waitReasonHalted + reason }

// RoleWait is one marked role.
type RoleWait struct {
	ProjectID string    `json:"project_id"`
	RoleKey   string    `json:"role_key"`
	MarkedBy  string    `json:"marked_by"`
	MarkedAt  time.Time `json:"marked_at"`
}

// ListRoleWaits returns every marked role, by project then key.
func ListRoleWaits(ctx context.Context) ([]RoleWait, error) {
	rows, err := querier(ctx).Query(ctx, `
		SELECT project_id, role_key, marked_by, marked_at
		FROM role_waits ORDER BY project_id, role_key`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (RoleWait, error) {
		var w RoleWait
		err := r.Scan(&w.ProjectID, &w.RoleKey, &w.MarkedBy, &w.MarkedAt)
		return w, err
	})
}

// MarkRoleWaits marks a role. Marking one already marked keeps who first
// marked it and when; created says whether this call marked it.
func MarkRoleWaits(ctx context.Context, projectID, roleKey, by string) (created bool, err error) {
	tag, err := querier(ctx).Exec(ctx, `
		INSERT INTO role_waits (project_id, role_key, marked_by) VALUES ($1, $2, $3)
		ON CONFLICT DO NOTHING`, projectID, roleKey, by)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

// UnmarkRoleWaits stops a role waiting. Changes already waiting stay waiting:
// they are the operator's to send, and stopping is a setting, not a send.
func UnmarkRoleWaits(ctx context.Context, projectID, roleKey string) (removed bool, err error) {
	tag, err := querier(ctx).Exec(ctx,
		`DELETE FROM role_waits WHERE project_id = $1 AND role_key = $2`, projectID, roleKey)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

// OutboxRowsWaitForRole says, for each outbox row id, whether any role it
// carries is marked to wait. Ids that aren't rows are absent.
func OutboxRowsWaitForRole(ctx context.Context, ids []string) (map[string]bool, error) {
	out := make(map[string]bool, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := querier(ctx).Query(ctx, `
		SELECT p.id::text, EXISTS (
		    SELECT 1 FROM role_waits w
		    WHERE w.project_id = p.project_id AND w.role_key = ANY(p.role_keys))
		FROM propagation_outbox p WHERE p.id::text = ANY($1)`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var waits bool
		if err := rows.Scan(&id, &waits); err != nil {
			return nil, err
		}
		out[id] = waits
	}
	return out, rows.Err()
}

// SetWaitReason records why rows still pending are waiting. Rows already
// sent, or no longer pending, are left alone.
func SetWaitReason(ctx context.Context, ids []string, reason string) error {
	if len(ids) == 0 {
		return nil
	}
	_, err := querier(ctx).Exec(ctx, `
		UPDATE propagation_outbox SET wait_reason = $2
		WHERE id::text = ANY($1) AND status = 'pending'`, ids, reason)
	return err
}

// ConfigSetting is a setting with who set it and when (N14: "Set to … by
// {name} on {date}").
type ConfigSetting struct {
	Value     string    `json:"value"`
	UpdatedBy string    `json:"updated_by"`
	UpdatedAt time.Time `json:"updated_at"`
}

// GetConfigSettingMeta returns the setting with its actor, or ok=false when
// it was never set.
func GetConfigSettingMeta(ctx context.Context, key string) (ConfigSetting, bool, error) {
	var c ConfigSetting
	var by *string
	err := querier(ctx).QueryRow(ctx,
		`SELECT value, updated_by, updated_at FROM config_settings WHERE key = $1`, key).
		Scan(&c.Value, &by, &c.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ConfigSetting{}, false, nil
	}
	if err != nil {
		return ConfigSetting{}, false, err
	}
	if by != nil {
		c.UpdatedBy = *by
	}
	return c, true, nil
}

// WaitSuggestion is a role that today waits only because a bundle or rule
// carrying it is set to wait: the one-time move suggests marking it, with why.
type WaitSuggestion struct {
	ProjectID string       `json:"project_id"`
	RoleKey   string       `json:"role_key"`
	Via       []WaitSource `json:"via"`
}

// WaitSource is the bundle or rule that makes a suggested role wait today.
type WaitSource struct {
	Kind string `json:"kind"` // bundle | rule
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
}

// ListWaitSuggestions returns every role given by a bundle (any published
// version) or rule set to wait, not already marked, by project then key.
func ListWaitSuggestions(ctx context.Context) ([]WaitSuggestion, error) {
	rows, err := querier(ctx).Query(ctx, `
		SELECT project_id, role_key, kind, id, name FROM (
		    SELECT DISTINCT bvr.zitadel_project_id AS project_id, bvr.zitadel_role_key AS role_key,
		           'bundle' AS kind, b.id::text AS id, b.name AS name
		    FROM bundles b
		    JOIN bundle_versions v ON v.bundle_id = b.id
		    JOIN bundle_version_roles bvr ON bvr.version_id = v.id
		    WHERE b.confirmation_mode = 'manual' AND b.retired_at IS NULL
		    UNION
		    SELECT r.target_zitadel_project_id, r.target_zitadel_role_key, 'rule', r.id::text, ''
		    FROM mapping_rules r WHERE r.confirmation_mode = 'manual' AND r.retired_at IS NULL
		) s
		WHERE NOT EXISTS (SELECT 1 FROM role_waits w
		                  WHERE w.project_id = s.project_id AND w.role_key = s.role_key)
		ORDER BY project_id, role_key, kind, name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []WaitSuggestion
	for rows.Next() {
		var p, k string
		var src WaitSource
		if err := rows.Scan(&p, &k, &src.Kind, &src.ID, &src.Name); err != nil {
			return nil, err
		}
		if n := len(out); n > 0 && out[n-1].ProjectID == p && out[n-1].RoleKey == k {
			out[n-1].Via = append(out[n-1].Via, src)
			continue
		}
		out = append(out, WaitSuggestion{ProjectID: p, RoleKey: k, Via: []WaitSource{src}})
	}
	return out, rows.Err()
}
