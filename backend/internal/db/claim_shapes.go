package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"syndra/internal/claims"
)

// ClaimShapeVersion is one immutable save of an application's shape
// (app_claim_shape_versions). The latest version is the one that applies.
type ClaimShapeVersion struct {
	ApplicationID     string         `json:"application_id"`
	Version           int            `json:"version"`
	ProjectID         string         `json:"project_id"`
	ClientID          string         `json:"-"`
	Profile           claims.Profile `json:"profile"`
	TemplateID        string         `json:"template_id"`
	TemplateVersion   int            `json:"template_version"`
	CopiedFromApp     string         `json:"copied_from_application_id,omitempty"`
	CopiedFromVersion int            `json:"copied_from_version,omitempty"`
	RestoredFrom      int            `json:"restored_from,omitempty"`
	Note              string         `json:"note"`
	SavedBy           string         `json:"saved_by"`
	SavedAt           time.Time      `json:"saved_at"`
}

const claimShapeColumns = `application_id, version, project_id, client_id, profile, template_id, template_version,
	COALESCE(copied_from_application_id, ''), COALESCE(copied_from_version, 0), COALESCE(restored_from, 0),
	note, saved_by, saved_at`

func scanClaimShape(s rowScanner) (ClaimShapeVersion, error) {
	var v ClaimShapeVersion
	var profile []byte
	if err := s.Scan(&v.ApplicationID, &v.Version, &v.ProjectID, &v.ClientID, &profile, &v.TemplateID,
		&v.TemplateVersion, &v.CopiedFromApp, &v.CopiedFromVersion, &v.RestoredFrom, &v.Note, &v.SavedBy, &v.SavedAt); err != nil {
		return ClaimShapeVersion{}, err
	}
	if err := json.Unmarshal(profile, &v.Profile); err != nil {
		return ClaimShapeVersion{}, fmt.Errorf("decode claim shape %s v%d: %w", v.ApplicationID, v.Version, err)
	}
	return v, nil
}

func collectClaimShapes(rows pgx.Rows, err error) ([]ClaimShapeVersion, error) {
	if err != nil {
		return nil, fmt.Errorf("list claim shapes: %w", err)
	}
	defer rows.Close()
	out := []ClaimShapeVersion{}
	for rows.Next() {
		v, err := scanClaimShape(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// LatestClaimShapes is the latest version of every app shape on a project: the
// data plane's whole per-project read, cached in Redis.
func LatestClaimShapes(ctx context.Context, projectID string) ([]ClaimShapeVersion, error) {
	rows, err := querier(ctx).Query(ctx, `
		SELECT DISTINCT ON (application_id) `+claimShapeColumns+`
		FROM app_claim_shape_versions WHERE project_id = $1
		ORDER BY application_id, version DESC`, projectID)
	return collectClaimShapes(rows, err)
}

// ClaimShapeHistory is every version of one app's shape, newest first.
func ClaimShapeHistory(ctx context.Context, applicationID string) ([]ClaimShapeVersion, error) {
	rows, err := querier(ctx).Query(ctx, `
		SELECT `+claimShapeColumns+` FROM app_claim_shape_versions
		WHERE application_id = $1 ORDER BY version DESC`, applicationID)
	return collectClaimShapes(rows, err)
}

// ClaimShapeAt is one version of one app's shape; version 0 means the latest.
func ClaimShapeAt(ctx context.Context, applicationID string, version int) (ClaimShapeVersion, bool, error) {
	v, err := scanClaimShape(querier(ctx).QueryRow(ctx, `
		SELECT `+claimShapeColumns+` FROM app_claim_shape_versions
		WHERE application_id = $1 AND ($2::int = 0 OR version = $2::int)
		ORDER BY version DESC LIMIT 1`, applicationID, version))
	if errors.Is(err, pgx.ErrNoRows) {
		return ClaimShapeVersion{}, false, nil
	}
	if err != nil {
		return ClaimShapeVersion{}, false, fmt.Errorf("claim shape %s v%d: %w", applicationID, version, err)
	}
	return v, true, nil
}

// ErrClaimShapeRace is a save that lost to another save of the same app.
var ErrClaimShapeRace = errors.New("another save of this app's claims landed first")

// InsertClaimShapeVersion saves the next version of an app's shape. With
// onlyIfNone it writes version 1 only when the app has no shape yet (the seed)
// and reports false when it already had one.
func InsertClaimShapeVersion(ctx context.Context, v ClaimShapeVersion, onlyIfNone bool) (ClaimShapeVersion, bool, error) {
	profile, err := json.Marshal(v.Profile)
	if err != nil {
		return ClaimShapeVersion{}, false, fmt.Errorf("encode claim shape: %w", err)
	}
	guard := ""
	if onlyIfNone {
		guard = `WHERE NOT EXISTS (SELECT 1 FROM app_claim_shape_versions WHERE application_id = $1::text)`
	}
	out, err := scanClaimShape(querier(ctx).QueryRow(ctx, `
		INSERT INTO app_claim_shape_versions
			(application_id, version, project_id, client_id, profile, template_id, template_version,
			 copied_from_application_id, copied_from_version, restored_from, note, saved_by)
		SELECT $1::text, COALESCE((SELECT MAX(version) FROM app_claim_shape_versions WHERE application_id = $1::text), 0) + 1,
		       $2::text, $3::text, $4::jsonb, $5::text, $6::int, NULLIF($7::text, ''), NULLIF($8::int, 0), NULLIF($9::int, 0), $10::text, $11::text
		`+guard+`
		ON CONFLICT (application_id, version) DO NOTHING
		RETURNING `+claimShapeColumns,
		v.ApplicationID, v.ProjectID, v.ClientID, profile, v.TemplateID, v.TemplateVersion,
		v.CopiedFromApp, v.CopiedFromVersion, v.RestoredFrom, v.Note, v.SavedBy))
	if errors.Is(err, pgx.ErrNoRows) {
		if onlyIfNone {
			return ClaimShapeVersion{}, false, nil
		}
		return ClaimShapeVersion{}, false, ErrClaimShapeRace
	}
	if err != nil {
		return ClaimShapeVersion{}, false, fmt.Errorf("save claim shape for %s: %w", v.ApplicationID, err)
	}
	return out, true, nil
}

// ClaimShapeSeedCutoff is when migration 000071 ran: apps Zitadel created
// before it existed under Actions v1 and are seeded with its shape.
func ClaimShapeSeedCutoff(ctx context.Context) (time.Time, error) {
	var t time.Time
	if err := querier(ctx).QueryRow(ctx, `SELECT cutoff FROM app_claim_shape_seed`).Scan(&t); err != nil {
		return time.Time{}, fmt.Errorf("claim shape seed cutoff: %w", err)
	}
	return t, nil
}
