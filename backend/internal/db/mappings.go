package db

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// Role-to-target mappings: what a Zitadel role means on a target (design §6).
//
// The resolver has no other source for the role-derived half of an entitlement
// set. `role_key='maker'` means nothing to TrueNAS until somebody says it means
// `group=lab_makers`, and that sentence changes what every holder of that role
// can reach — so it carries the same versioning, rollback and audit a bundle
// edit carries, and is emphatically not deployment configuration.

var (
	// ErrMappingExists refuses a second binding for one (target, project, role,
	// field). Two rows binding one role's `group` to different values is not a
	// richer mapping — it is a resolver returning whichever the database
	// ordered first, and a subject whose access depends on that ordering.
	ErrMappingExists = errors.New("db: a mapping already binds that role's field on this target")
	// ErrMappingNotFound is a mapping cited that does not exist.
	ErrMappingNotFound = errors.New("db: no such mapping")
	// ErrMappingInvalid is a structurally impossible mapping, refused before
	// any database contact.
	ErrMappingInvalid = errors.New("db: invalid mapping")
)

// RoleMapping is one binding.
type RoleMapping struct {
	ID        string `json:"id"`
	Target    string `json:"target"`
	ProjectID string `json:"project_id"`
	RoleKey   string `json:"role_key"`
	Field     string `json:"field"`
	Value     string `json:"value"`
	CreatedBy string `json:"created_by"`
	UpdatedBy string `json:"updated_by,omitempty"`
}

func (m RoleMapping) validate() error {
	for _, f := range []struct{ name, value string }{
		{"target", m.Target}, {"project_id", m.ProjectID},
		{"role_key", m.RoleKey}, {"field", m.Field}, {"value", m.Value},
	} {
		if strings.TrimSpace(f.value) == "" {
			// The field name, never the value. A mapping value is an add-on's
			// own vocabulary and the backend does not know what may be in it.
			return fmt.Errorf("%w: %s is required", ErrMappingInvalid, f.name)
		}
	}
	return nil
}

// CreateRoleMapping writes one binding.
//
// Structural validation only: that the field exists in the add-on's declared
// schema, that the role exists, and that the value resolves on the target are
// checks the service layer makes, because two of the three need the add-on.
// What is enforced here is what the database is the authority on — uniqueness,
// and that the target is registered.
func CreateRoleMapping(ctx context.Context, m RoleMapping) (RoleMapping, error) {
	if err := m.validate(); err != nil {
		return RoleMapping{}, err
	}
	const q = `
		INSERT INTO target_role_mappings (target, project_id, role_key, field, value, created_by, updated_by)
		VALUES ($1, $2, $3, $4, $5, $6, $6)
		ON CONFLICT (target, project_id, role_key, field) DO NOTHING
		RETURNING id`
	err := PG.QueryRow(ctx, q, m.Target, m.ProjectID, m.RoleKey, m.Field, m.Value, m.CreatedBy).Scan(&m.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		// Absorbed rather than raised: a unique violation aborts the caller's
		// transaction, and this refusal is one a caller is expected to handle.
		return RoleMapping{}, fmt.Errorf("%w: %s %s/%s.%s", ErrMappingExists, m.Target, m.ProjectID, m.RoleKey, m.Field)
	}
	if err != nil {
		return RoleMapping{}, fmt.Errorf("create role mapping: %w", err)
	}
	m.UpdatedBy = m.CreatedBy
	return m, nil
}

// UpdateRoleMappingValue changes what a binding resolves to.
//
// Only the value moves. Re-pointing a mapping at a different role or field is
// not an edit — it is deleting one binding and creating another, with a
// different cohort on each side — and letting one statement do it would hide a
// change of blast radius inside a change of value.
func UpdateRoleMappingValue(ctx context.Context, id, value, actor string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%w: value is required", ErrMappingInvalid)
	}
	if !looksLikeUUID(id) {
		return fmt.Errorf("%w: %s", ErrMappingNotFound, id)
	}
	const q = `UPDATE target_role_mappings SET value = $2, updated_by = $3, updated_at = NOW() WHERE id = $1`
	tag, err := PG.Exec(ctx, q, id, value, actor)
	if err != nil {
		return fmt.Errorf("update role mapping: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("%w: %s", ErrMappingNotFound, id)
	}
	return nil
}

// DeleteRoleMapping removes a binding.
//
// It is a grant-affecting change like any other: every holder of that role
// loses whatever it conferred, which is why the surface plans it first.
func DeleteRoleMapping(ctx context.Context, id string) error {
	if !looksLikeUUID(id) {
		return fmt.Errorf("%w: %s", ErrMappingNotFound, id)
	}
	tag, err := PG.Exec(ctx, `DELETE FROM target_role_mappings WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete role mapping: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("%w: %s", ErrMappingNotFound, id)
	}
	return nil
}

// GetRoleMapping reads one binding.
func GetRoleMapping(ctx context.Context, id string) (RoleMapping, error) {
	if !looksLikeUUID(id) {
		return RoleMapping{}, fmt.Errorf("%w: %s", ErrMappingNotFound, id)
	}
	const q = `
		SELECT id, target, project_id, role_key, field, value, created_by, updated_by
		  FROM target_role_mappings WHERE id = $1`
	var m RoleMapping
	err := PG.QueryRow(ctx, q, id).Scan(&m.ID, &m.Target, &m.ProjectID, &m.RoleKey, &m.Field, &m.Value, &m.CreatedBy, &m.UpdatedBy)
	if errors.Is(err, pgx.ErrNoRows) {
		return RoleMapping{}, fmt.Errorf("%w: %s", ErrMappingNotFound, id)
	}
	if err != nil {
		return RoleMapping{}, fmt.Errorf("read role mapping: %w", err)
	}
	return m, nil
}

// ListRoleMappings returns a target's whole binding set, or every target's when
// target is empty.
func ListRoleMappings(ctx context.Context, target string) ([]RoleMapping, error) {
	const q = `
		SELECT id, target, project_id, role_key, field, value, created_by, updated_by
		  FROM target_role_mappings
		 WHERE ($1 = '' OR target = $1)
		 ORDER BY target, project_id, role_key, field`
	rows, err := PG.Query(ctx, q, target)
	if err != nil {
		return nil, fmt.Errorf("list role mappings: %w", err)
	}
	defer rows.Close()
	return scanRoleMappings(rows)
}

// MappingsForRoles returns every binding reached by any of the given roles.
//
// One query for the whole role set rather than one per role: a subject holding
// twelve roles is ordinary, and the resolver runs on every grant change.
func MappingsForRoles(ctx context.Context, target string, roles []RoleRef) ([]RoleMapping, error) {
	if len(roles) == 0 {
		return nil, nil
	}
	projects := make([]string, len(roles))
	keys := make([]string, len(roles))
	for i, r := range roles {
		projects[i], keys[i] = r.ProjectID, r.RoleKey
	}
	// Paired by position, so (pA, roleB) does not match a mapping for
	// (pB, roleA) — a cross join over two IN lists would confer, on a project
	// the subject holds nothing in, whatever a role of the same name means
	// somewhere else.
	const q = `
		SELECT m.id, m.target, m.project_id, m.role_key, m.field, m.value, m.created_by, m.updated_by
		  FROM target_role_mappings m
		  JOIN unnest($2::text[], $3::text[]) AS held(project_id, role_key)
		    ON held.project_id = m.project_id AND held.role_key = m.role_key
		 WHERE m.target = $1
		 ORDER BY m.project_id, m.role_key, m.field`
	rows, err := PG.Query(ctx, q, target, projects, keys)
	if err != nil {
		return nil, fmt.Errorf("read mappings for roles: %w", err)
	}
	defer rows.Close()
	return scanRoleMappings(rows)
}

// TargetsMappedToRole names the targets a role reaches, which is what the
// lifecycle trigger consults on every grant change.
func TargetsMappedToRole(ctx context.Context, projectID, roleKey string) ([]string, error) {
	const q = `
		SELECT DISTINCT m.target
		  FROM target_role_mappings m
		  JOIN targets t ON t.target = m.target AND t.state = 'active'
		 WHERE m.project_id = $1 AND m.role_key = $2
		 ORDER BY m.target`
	rows, err := PG.Query(ctx, q, projectID, roleKey)
	if err != nil {
		return nil, fmt.Errorf("read targets mapped to %s/%s: %w", projectID, roleKey, err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, fmt.Errorf("scan mapped target: %w", err)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// MappingHolders lists the subjects a mapping currently reaches — the cohort a
// mapping edit or delete would move.
//
// Read from the effective-access projection rather than from direct grants
// alone, because a role held through a bundle or derived by a rule is held just
// as much as one granted by hand, and a blast-radius count that missed those
// would understate the change on exactly the mappings that matter most.
// The rule arm is the one this comment promised and the query did not have.
// Direct grants carry an expiry; a lapsed one is not held, and counting it
// overstated the cohort in the other direction.
//
// Rules are followed ONE hop. A rule chain would need a recursive walk, and the
// resolver does not follow one either — so a deeper count would report a
// blast radius the product cannot actually produce.
func MappingHolders(ctx context.Context, projectID, roleKey string) ([]string, error) {
	const q = `
		WITH held AS (
			SELECT user_id, zitadel_project_id AS project_id, zitadel_role_key AS role_key
			  FROM direct_role_grants
			 WHERE expires_at IS NULL OR expires_at > NOW()
			UNION
			SELECT ba.user_id, bvr.zitadel_project_id, bvr.zitadel_role_key
			  FROM user_bundle_assignments ba
			  JOIN bundle_version_roles bvr ON bvr.version_id = ba.version_id
		)
		SELECT DISTINCT user_id FROM (
			SELECT user_id FROM held WHERE project_id = $1 AND role_key = $2
			UNION
			SELECT h.user_id
			  FROM held h
			  JOIN mapping_rules r
			    ON r.source_zitadel_project_id = h.project_id
			   AND r.source_zitadel_role_key = h.role_key
			 WHERE r.target_zitadel_project_id = $1 AND r.target_zitadel_role_key = $2
		) holders
		ORDER BY user_id`
	rows, err := PG.Query(ctx, q, projectID, roleKey)
	if err != nil {
		return nil, fmt.Errorf("read mapping holders: %w", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var u string
		if err := rows.Scan(&u); err != nil {
			return nil, fmt.Errorf("scan mapping holder: %w", err)
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

func scanRoleMappings(rows pgx.Rows) ([]RoleMapping, error) {
	var out []RoleMapping
	for rows.Next() {
		var m RoleMapping
		if err := rows.Scan(&m.ID, &m.Target, &m.ProjectID, &m.RoleKey, &m.Field, &m.Value, &m.CreatedBy, &m.UpdatedBy); err != nil {
			return nil, fmt.Errorf("scan role mapping: %w", err)
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// PublishMappingVersion snapshots a target's whole binding set and returns the
// version number.
//
// Versioned per target rather than per row: a mapping edit is only meaningful
// against the set it sits in, and a per-row version would let a rollback
// restore one binding into a set the rest of which has moved on. The version
// number is allocated inside the transaction, so two concurrent publishes
// cannot both claim the same one.
func PublishMappingVersion(ctx context.Context, target, note, actor string) (int, error) {
	var version int
	err := InTx(ctx, func(tx pgx.Tx) error {
		// Serialised per target before the number is read. `MAX(version)+1` is
		// a read and a write with a gap in the middle, and two publishes of one
		// target that overlapped in that gap would both compute the same next
		// version — the thing the comment above says cannot happen. Migration
		// 000026's snapshot trigger takes this same lock for this same reason;
		// this is that pattern, reused rather than reinvented.
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, "mapping_version:"+target); err != nil {
			return fmt.Errorf("lock mapping versions for %s: %w", target, err)
		}
		const insertVersion = `
			INSERT INTO target_mapping_versions (target, version, note, published_by)
			SELECT $1, COALESCE(MAX(version), 0) + 1, $2, $3
			  FROM target_mapping_versions WHERE target = $1
			RETURNING id, version`
		var versionID string
		if err := tx.QueryRow(ctx, insertVersion, target, note, actor).Scan(&versionID, &version); err != nil {
			return fmt.Errorf("insert mapping version: %w", err)
		}
		const copyEntries = `
			INSERT INTO target_mapping_version_entries (version_id, project_id, role_key, field, value)
			SELECT $1, project_id, role_key, field, value
			  FROM target_role_mappings WHERE target = $2`
		if _, err := tx.Exec(ctx, copyEntries, versionID, target); err != nil {
			return fmt.Errorf("copy mapping entries: %w", err)
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return version, nil
}

// RollbackMappingVersion restores a target's working copy to a published
// version, in one transaction.
//
// Replace rather than merge. A rollback restores a SET, and merging would leave
// behind every binding added since — which is the half of the change an
// operator is usually rolling back.
func RollbackMappingVersion(ctx context.Context, target string, version int, actor string) error {
	return InTx(ctx, func(tx pgx.Tx) error {
		// The same lock a publish takes, and on the same key. A rollback
		// DELETEs the whole working set and re-inserts it; a publish reads that
		// set to snapshot it. Overlapping, one target could be published
		// mid-rollback and the resulting version would be a set that never
		// existed.
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, "mapping_version:"+target); err != nil {
			return fmt.Errorf("lock mapping versions for %s: %w", target, err)
		}
		var versionID string
		err := tx.QueryRow(ctx,
			`SELECT id FROM target_mapping_versions WHERE target = $1 AND version = $2`,
			target, version).Scan(&versionID)
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("%w: %s v%d", ErrMappingNotFound, target, version)
		}
		if err != nil {
			return fmt.Errorf("read mapping version: %w", err)
		}
		if _, err := tx.Exec(ctx, `DELETE FROM target_role_mappings WHERE target = $1`, target); err != nil {
			return fmt.Errorf("clear working mappings: %w", err)
		}
		const restore = `
			INSERT INTO target_role_mappings (target, project_id, role_key, field, value, created_by, updated_by)
			SELECT $1, project_id, role_key, field, value, $2, $2
			  FROM target_mapping_version_entries WHERE version_id = $3`
		if _, err := tx.Exec(ctx, restore, target, actor, versionID); err != nil {
			return fmt.Errorf("restore mapping version: %w", err)
		}
		return nil
	})
}

// A published version, as the history surface reads it (change `addon-platform`
// 9.7/9.8; design §24).
//
// Who published it and why travel with it, and they are not decoration: a
// rollback target with no reason attached is a guess, and the operator choosing
// one is deciding whose judgement to restore. The note is the only record of
// that judgement, which is why publishing takes one.
type MappingVersion struct {
	Version     int                   `json:"version"`
	Note        string                `json:"note"`
	PublishedBy string                `json:"published_by"`
	PublishedAt time.Time             `json:"published_at"`
	Entries     []MappingVersionEntry `json:"entries"`
}

// MappingVersionEntry is one binding inside a version. Deliberately not a
// RoleMapping: a snapshot row has no id and no target of its own, and giving it
// those fields would invite a caller to treat history as something editable.
type MappingVersionEntry struct {
	ProjectID string `json:"project_id"`
	RoleKey   string `json:"role_key"`
	Field     string `json:"field"`
	Value     string `json:"value"`
}

// MappingHistory is a target's published versions, newest first, with whether
// the working copy still matches the newest one.
//
// The second half is the part a version list alone cannot say. Publishing
// snapshots the working set; every edit afterwards moves the working set and
// not the snapshot. So "current version 4" is true and misleading on its own —
// what is live may be version 4 plus three edits nobody has published — and an
// operator rolling back to 4 from that state would be undoing work they cannot
// see listed anywhere.
type MappingHistory struct {
	Target string `json:"target"`
	// CurrentVersion is the newest PUBLISHED version, or 0 if none. What the
	// surface tints.
	CurrentVersion int `json:"current_version"`
	// Unpublished says the working copy differs from that newest version — or
	// that there is no version at all and every binding is unpublished.
	Unpublished bool             `json:"unpublished"`
	Versions    []MappingVersion `json:"versions"`
}

// ListMappingHistory reads a target's version history and compares the newest
// version against what is live.
func ListMappingHistory(ctx context.Context, target string) (MappingHistory, error) {
	if strings.TrimSpace(target) == "" {
		return MappingHistory{}, fmt.Errorf("list mapping history: no target")
	}
	history := MappingHistory{Target: target, Versions: []MappingVersion{}}

	const q = `
		SELECT v.version, v.note, v.published_by, v.published_at,
		       COALESCE(e.project_id, ''), COALESCE(e.role_key, ''),
		       COALESCE(e.field, ''), COALESCE(e.value, '')
		  FROM target_mapping_versions v
		  -- LEFT, because a version published against an empty working set is a
		  -- real event and must appear in the history. An inner join would drop
		  -- it, and the gap in the version numbers would be the only trace.
		  LEFT JOIN target_mapping_version_entries e ON e.version_id = v.id
		 WHERE v.target = $1
		 ORDER BY v.version DESC, e.project_id, e.role_key, e.field`
	rows, err := PG.Query(ctx, q, target)
	if err != nil {
		return MappingHistory{}, fmt.Errorf("list mapping versions for %s: %w", target, err)
	}
	defer rows.Close()

	byVersion := map[int]int{} // version number -> index in history.Versions
	for rows.Next() {
		var v MappingVersion
		var e MappingVersionEntry
		if err := rows.Scan(&v.Version, &v.Note, &v.PublishedBy, &v.PublishedAt,
			&e.ProjectID, &e.RoleKey, &e.Field, &e.Value); err != nil {
			return MappingHistory{}, fmt.Errorf("scan mapping version: %w", err)
		}
		idx, seen := byVersion[v.Version]
		if !seen {
			v.Entries = []MappingVersionEntry{}
			history.Versions = append(history.Versions, v)
			idx = len(history.Versions) - 1
			byVersion[v.Version] = idx
		}
		if e.ProjectID != "" {
			history.Versions[idx].Entries = append(history.Versions[idx].Entries, e)
		}
	}
	if err := rows.Err(); err != nil {
		return MappingHistory{}, fmt.Errorf("read mapping versions for %s: %w", target, err)
	}

	live, err := ListRoleMappings(ctx, target)
	if err != nil {
		return MappingHistory{}, err
	}
	if len(history.Versions) == 0 {
		history.Unpublished = len(live) > 0
		return history, nil
	}
	history.CurrentVersion = history.Versions[0].Version
	history.Unpublished = !sameBindings(live, history.Versions[0].Entries)
	return history, nil
}

// sameBindings compares a working set against a snapshot, order-independently.
//
// By CONTENT rather than by count: two sets of the same size differing in one
// value is exactly the edit an operator would want flagged as unpublished, and a
// length check would call them equal.
func sameBindings(live []RoleMapping, snapshot []MappingVersionEntry) bool {
	if len(live) != len(snapshot) {
		return false
	}
	key := func(project, role, field, value string) string {
		return project + "\x00" + role + "\x00" + field + "\x00" + value
	}
	have := make(map[string]int, len(live))
	for _, m := range live {
		have[key(m.ProjectID, m.RoleKey, m.Field, m.Value)]++
	}
	for _, e := range snapshot {
		k := key(e.ProjectID, e.RoleKey, e.Field, e.Value)
		if have[k] == 0 {
			return false
		}
		have[k]--
	}
	return true
}
