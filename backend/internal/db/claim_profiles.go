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

// ClaimProfileRow is the application-facing view of the claim_profiles table.
// Used by the directory layer to overlay claim-shaping metadata on top of live
// Zitadel project discovery.
type ClaimProfileRow struct {
	ProjectID  string
	ClaimName  string
	FormatType string
}

// ListClaimProfiles returns every claim_profiles row. Used to overlay
// application metadata (ClaimName, FormatType) on live Zitadel projects when
// the directory layer is running in live mode.
func ListClaimProfiles(ctx context.Context) ([]ClaimProfileRow, error) {
	rows, err := PG.Query(ctx, `SELECT zitadel_project_id, claim_name, format_type FROM claim_profiles`)
	if err != nil {
		return nil, fmt.Errorf("list claim profiles: %w", err)
	}
	defer rows.Close()

	var out []ClaimProfileRow
	for rows.Next() {
		var r ClaimProfileRow
		if err := rows.Scan(&r.ProjectID, &r.ClaimName, &r.FormatType); err != nil {
			return nil, fmt.Errorf("scan claim profile row: %w", err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate claim profiles: %w", err)
	}
	return out, nil
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
