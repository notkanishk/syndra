package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// -------------------------------------------------------------
// CLAIM PROFILES
// -------------------------------------------------------------

// ClaimProfileRow is the application-facing view of the claim_profiles table:
// one project's default token shape. Used by the directory layer to overlay
// claim-shaping metadata on top of live Zitadel project discovery, and by the
// claim-shaping handlers as the editable record.
type ClaimProfileRow struct {
	ProjectID       string
	ClaimName       string
	FormatType      string
	AttributeClaims map[string]string
	StaticClaims    map[string]any
}

// AppClaimOverrideRow is one application's departure from its project's
// default shape. Keyed by application id; the project id is carried so the
// data plane can resolve every override for a project in one query.
type AppClaimOverrideRow struct {
	ApplicationID   string
	ProjectID       string
	ClaimName       string
	FormatType      string
	AttributeClaims map[string]string
	StaticClaims    map[string]any
}

const claimProfileColumns = `zitadel_project_id, claim_name, format_type, attribute_claims, static_claims`

// ListClaimProfiles returns every claim_profiles row. Used to overlay
// application metadata (ClaimName, FormatType) on live Zitadel projects when
// the directory layer is running in live mode, and by the data plane to shape
// tokens.
func ListClaimProfiles(ctx context.Context) ([]ClaimProfileRow, error) {
	rows, err := PG.Query(ctx, `SELECT `+claimProfileColumns+` FROM claim_profiles`)
	if err != nil {
		return nil, fmt.Errorf("list claim profiles: %w", err)
	}
	defer rows.Close()

	var out []ClaimProfileRow
	for rows.Next() {
		r, err := scanClaimProfile(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate claim profiles: %w", err)
	}
	return out, nil
}

// GetClaimProfile returns one project's default shape. The bool is false when
// the operator has never edited it — callers fall back to the built-in
// default rather than treating absence as an error.
func GetClaimProfile(ctx context.Context, projectID string) (ClaimProfileRow, bool, error) {
	row := PG.QueryRow(ctx,
		`SELECT `+claimProfileColumns+` FROM claim_profiles WHERE zitadel_project_id = $1`, projectID)
	r, err := scanClaimProfile(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return ClaimProfileRow{}, false, nil
	}
	if err != nil {
		return ClaimProfileRow{}, false, err
	}
	return r, true, nil
}

// UpsertClaimProfile writes a project's default token shape.
func UpsertClaimProfile(ctx context.Context, r ClaimProfileRow) error {
	attrs, statics, err := marshalClaimMaps(r.AttributeClaims, r.StaticClaims)
	if err != nil {
		return err
	}
	_, err = PG.Exec(ctx, `
		INSERT INTO claim_profiles (zitadel_project_id, claim_name, format_type, attribute_claims, static_claims)
		VALUES ($1,$2,$3,$4,$5)
		ON CONFLICT (zitadel_project_id) DO UPDATE
		SET claim_name = EXCLUDED.claim_name,
		    format_type = EXCLUDED.format_type,
		    attribute_claims = EXCLUDED.attribute_claims,
		    static_claims = EXCLUDED.static_claims,
		    updated_at = CURRENT_TIMESTAMP`,
		r.ProjectID, r.ClaimName, r.FormatType, attrs, statics)
	if err != nil {
		return fmt.Errorf("upsert claim profile for project %s: %w", r.ProjectID, err)
	}
	return nil
}

// ListAppClaimOverrides returns every per-application override. The data
// plane loads the whole (small) set once per token rather than issuing a
// query per project.
func ListAppClaimOverrides(ctx context.Context) ([]AppClaimOverrideRow, error) {
	rows, err := PG.Query(ctx, `
		SELECT application_id, zitadel_project_id, claim_name, format_type, attribute_claims, static_claims
		FROM app_claim_overrides`)
	if err != nil {
		return nil, fmt.Errorf("list app claim overrides: %w", err)
	}
	defer rows.Close()

	var out []AppClaimOverrideRow
	for rows.Next() {
		var r AppClaimOverrideRow
		var attrs, statics []byte
		if err := rows.Scan(&r.ApplicationID, &r.ProjectID, &r.ClaimName, &r.FormatType, &attrs, &statics); err != nil {
			return nil, fmt.Errorf("scan app claim override row: %w", err)
		}
		if err := unmarshalClaimMaps(attrs, statics, &r.AttributeClaims, &r.StaticClaims); err != nil {
			return nil, fmt.Errorf("decode app claim override %s: %w", r.ApplicationID, err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate app claim overrides: %w", err)
	}
	return out, nil
}

// UpsertAppClaimOverride writes one application's override.
func UpsertAppClaimOverride(ctx context.Context, r AppClaimOverrideRow) error {
	attrs, statics, err := marshalClaimMaps(r.AttributeClaims, r.StaticClaims)
	if err != nil {
		return err
	}
	_, err = PG.Exec(ctx, `
		INSERT INTO app_claim_overrides
			(application_id, zitadel_project_id, claim_name, format_type, attribute_claims, static_claims)
		VALUES ($1,$2,$3,$4,$5,$6)
		ON CONFLICT (application_id) DO UPDATE
		SET zitadel_project_id = EXCLUDED.zitadel_project_id,
		    claim_name = EXCLUDED.claim_name,
		    format_type = EXCLUDED.format_type,
		    attribute_claims = EXCLUDED.attribute_claims,
		    static_claims = EXCLUDED.static_claims,
		    updated_at = CURRENT_TIMESTAMP`,
		r.ApplicationID, r.ProjectID, r.ClaimName, r.FormatType, attrs, statics)
	if err != nil {
		return fmt.Errorf("upsert app claim override for %s: %w", r.ApplicationID, err)
	}
	return nil
}

// DeleteAppClaimOverride drops an override so the application falls back to
// its project's default shape.
func DeleteAppClaimOverride(ctx context.Context, applicationID string) error {
	if _, err := PG.Exec(ctx, `DELETE FROM app_claim_overrides WHERE application_id = $1`, applicationID); err != nil {
		return fmt.Errorf("delete app claim override for %s: %w", applicationID, err)
	}
	return nil
}

// rowScanner is satisfied by both pgx.Row and pgx.Rows.
type rowScanner interface {
	Scan(dest ...any) error
}

func scanClaimProfile(s rowScanner) (ClaimProfileRow, error) {
	var r ClaimProfileRow
	var attrs, statics []byte
	if err := s.Scan(&r.ProjectID, &r.ClaimName, &r.FormatType, &attrs, &statics); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ClaimProfileRow{}, err
		}
		return ClaimProfileRow{}, fmt.Errorf("scan claim profile row: %w", err)
	}
	if err := unmarshalClaimMaps(attrs, statics, &r.AttributeClaims, &r.StaticClaims); err != nil {
		return ClaimProfileRow{}, fmt.Errorf("decode claim profile for project %s: %w", r.ProjectID, err)
	}
	return r, nil
}

func marshalClaimMaps(attrs map[string]string, statics map[string]any) ([]byte, []byte, error) {
	if attrs == nil {
		attrs = map[string]string{}
	}
	if statics == nil {
		statics = map[string]any{}
	}
	attrJSON, err := json.Marshal(attrs)
	if err != nil {
		return nil, nil, fmt.Errorf("encode attribute claims: %w", err)
	}
	staticJSON, err := json.Marshal(statics)
	if err != nil {
		return nil, nil, fmt.Errorf("encode static claims: %w", err)
	}
	return attrJSON, staticJSON, nil
}

func unmarshalClaimMaps(attrs, statics []byte, outAttrs *map[string]string, outStatics *map[string]any) error {
	*outAttrs = map[string]string{}
	*outStatics = map[string]any{}
	if len(attrs) > 0 {
		if err := json.Unmarshal(attrs, outAttrs); err != nil {
			return err
		}
	}
	if len(statics) > 0 {
		if err := json.Unmarshal(statics, outStatics); err != nil {
			return err
		}
	}
	return nil
}

// GetClaimFailureMode returns the configured degraded-mode behavior for a project's
// claim profile. Returns ("fail_closed", nil, nil) if the project has no claim
// profile configured — fail_closed is always the safe default.
func GetClaimFailureMode(ctx context.Context, projectID string) (string, map[string]interface{}, error) {
	query := `
		SELECT claim_failure_mode, minimal_safe_claims
		FROM claim_profiles
		WHERE zitadel_project_id = $1`

	var mode string
	var rawClaims []byte
	err := PG.QueryRow(ctx, query, projectID).Scan(&mode, &rawClaims)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// No claim profile configured for this project — fail_closed is the safe default
			return "fail_closed", nil, nil
		}
		// Real DB fault: surface it so the caller can log and operators can see it
		return "fail_closed", nil, fmt.Errorf("query claim failure mode for project %s: %w", projectID, err)
	}

	if rawClaims == nil {
		return mode, nil, nil
	}

	var claims map[string]interface{}
	if err := json.Unmarshal(rawClaims, &claims); err != nil {
		return "fail_closed", nil, fmt.Errorf("malformed minimal_safe_claims for project %s: %w", projectID, err)
	}
	return mode, claims, nil
}
