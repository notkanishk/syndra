package db

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"syndra/internal/models"
)

// ---------------------------------------------------------------------------
// Shadow Password Vault
// ---------------------------------------------------------------------------

// RetiredBridgeTarget is what a pre-cutover enrolment names.
//
// Those rows describe a credential set against the LLDAP bridge, which had no
// target because it was not one — it was the single directory every member
// shared. Naming it explicitly is what lets "they enrolled before the change"
// stay a different sentence from "they have never enrolled" now that enrolment
// is per target (§23).
const RetiredBridgeTarget = "retired_bridge"

// RecordCredentialSet notes that a member has set a credential on a target,
// and when (change `addon-platform` group 11).
//
// It takes no credential and there is nowhere to put one. The member's password
// is forwarded to the target by the operation that received it and is kept
// nowhere: no API in this system accepts a hash, so the only thing a stored one
// could ever do is leak. What survives is the metadata the member's own view
// renders and the answer to "have they enrolled".
//
// Per target, because the view that renders it is. Keyed on the person alone,
// enrolling on the NAS reported "set, last changed…" for every other target and
// cleared the re-enrolment notice on all of them at once.
func RecordCredentialSet(ctx context.Context, userID, target string) (string, error) {
	if strings.TrimSpace(target) == "" {
		// Refused rather than defaulted. A row with no target is the state this
		// column exists to remove, and the only rows entitled to a stand-in are
		// the pre-cutover ones the migration named.
		return "", fmt.Errorf("record credential for %s: an enrolment must name the target it is on", userID)
	}
	var id string
	err := querier(ctx).QueryRow(ctx, `
		INSERT INTO shadow_credentials (user_id, target)
		VALUES ($1, $2)
		ON CONFLICT (user_id, target) DO UPDATE SET
			updated_at              = NOW(),
			rotated_at              = NOW(),
			-- Setting one through the new path is what clears the mark: the
			-- member has now enrolled against the system that exists.
			enrolled_before_cutover = FALSE
		RETURNING id`, userID, target).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("record credential for %s on %s: %w", userID, target, err)
	}
	return id, nil
}

// DeleteShadowCredential removes a user's enrolment records, on every target.
//
// Every one, deliberately. This is the operator action "clear this person's
// credential record", reached from a surface that names a person and no target,
// and clearing one target while leaving another would leave the operator
// believing they had done the thing the button says.
func DeleteShadowCredential(ctx context.Context, userID string) error {
	tag, err := querier(ctx).Exec(ctx, `DELETE FROM shadow_credentials WHERE user_id = $1`, userID)
	if err != nil {
		return fmt.Errorf("delete shadow credential (user=%s): %w", userID, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("shadow credential not found for user %s: %w", userID, pgx.ErrNoRows)
	}
	return nil
}

// HasShadowCredential answers whether a member has enrolled on a target, and
// when.
//
// There is nothing else it could answer: the table holds no credential, and
// this SELECT names every column that survives.
//
// The pre-cutover row is the second candidate and never the first. A member who
// enrolled against the retired bridge and has since set a password on THIS
// target has both rows, and the one that describes a working credential has to
// win — otherwise the page tells somebody to re-enrol after they already have.
//
// An empty target asks the older question, "have they enrolled anywhere", which
// is what the per-person vault route has always asked and is still the right
// question there: it names a person and no system.
func HasShadowCredential(ctx context.Context, userID, target string) (models.ShadowCredentialStatus, error) {
	var s models.ShadowCredentialStatus
	var createdAt, updatedAt time.Time
	var rotatedAt, expiresAt *time.Time
	var beforeCutover bool
	err := querier(ctx).QueryRow(ctx, `
		SELECT created_at, updated_at, rotated_at, expires_at, enrolled_before_cutover
		FROM shadow_credentials
		 WHERE user_id = $1
		   AND ($2 = '' OR target IN ($2, '`+RetiredBridgeTarget+`'))
		 ORDER BY (target = $2) DESC, updated_at DESC
		 LIMIT 1`, userID, target).
		Scan(&createdAt, &updatedAt, &rotatedAt, &expiresAt, &beforeCutover)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.ShadowCredentialStatus{HasCredential: false}, nil
		}
		return s, fmt.Errorf("check shadow credential (user=%s): %w", userID, err)
	}
	return models.ShadowCredentialStatus{
		// A pre-cutover row is NOT a credential the member can use. The hash it
		// described is gone and the system it was for does not exist, so
		// reporting it as set would tell somebody they had enrolled when the
		// next connection attempt will fail (task 11.9).
		HasCredential:    !beforeCutover,
		NeedsReEnrolment: beforeCutover,
		CreatedAt:        &createdAt,
		UpdatedAt:        &updatedAt,
		RotatedAt:        rotatedAt,
		ExpiresAt:        expiresAt,
	}, nil
}

// InsertShadowCredentialAudit records a credential lifecycle event.
func InsertShadowCredentialAudit(ctx context.Context, userID, action, actorID, ipAddress string) error {
	_, err := querier(ctx).Exec(ctx, `
		INSERT INTO shadow_credential_audit (user_id, action, actor_id, ip_address)
		VALUES ($1, $2, $3, $4)`,
		userID, action, actorID, ipAddress)
	if err != nil {
		return fmt.Errorf("insert shadow credential audit (user=%s action=%s): %w", userID, action, err)
	}
	return nil
}

// GetShadowCredentialAudit returns the audit trail for a user's shadow credential.
func GetShadowCredentialAudit(ctx context.Context, userID string) ([]models.ShadowCredentialAudit, error) {
	rows, err := querier(ctx).Query(ctx, `
		SELECT id, user_id, action, actor_id, ip_address, created_at
		FROM shadow_credential_audit
		WHERE user_id = $1
		ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("query shadow credential audit (user=%s): %w", userID, err)
	}
	defer rows.Close()

	var entries []models.ShadowCredentialAudit
	for rows.Next() {
		var e models.ShadowCredentialAudit
		if err := rows.Scan(&e.ID, &e.UserID, &e.Action, &e.ActorID, &e.IPAddress, &e.CreatedAt); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, nil
}
