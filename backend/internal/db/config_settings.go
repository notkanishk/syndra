package db

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

const (
	confirmationModeAuto   = "auto"
	confirmationModeManual = "manual"

	ConfigKeyDefaultConfirmationMode = "global.default_rule_confirmation_mode"
)

// GetConfigSetting returns the value for key, or "" when the key is absent
// (absence is not an error — callers fall back to a compile-time default).
func GetConfigSetting(ctx context.Context, key string) (string, error) {
	var v string
	err := querier(ctx).QueryRow(ctx, `SELECT value FROM config_settings WHERE key = $1`, key).Scan(&v)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return v, nil
}

// SetConfigSetting upserts a config value.
func SetConfigSetting(ctx context.Context, key, value, updatedBy string) error {
	_, err := querier(ctx).Exec(ctx, `
		INSERT INTO config_settings (key, value, updated_by, updated_at)
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value,
			updated_by = EXCLUDED.updated_by, updated_at = NOW()`,
		key, value, updatedBy)
	return err
}

// NormalizeConfirmationMode returns a valid mode, defaulting unknown/empty to auto.
func NormalizeConfirmationMode(m string) string {
	if m == confirmationModeManual {
		return confirmationModeManual
	}
	return confirmationModeAuto
}
