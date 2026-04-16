-- Migration 000010: Shadow Password Vault
-- Infrastructure-only secondary credentials for LLDAP/Samba authentication.
-- These credentials are isolated from primary Zitadel/OIDC identity flows.

CREATE TABLE IF NOT EXISTS shadow_credentials (
    id              UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id         TEXT        NOT NULL UNIQUE CHECK (btrim(user_id) <> ''),
    credential_hash TEXT        NOT NULL CHECK (btrim(credential_hash) <> ''),
    algorithm       TEXT        NOT NULL DEFAULT 'argon2id'
                                    CHECK (algorithm IN ('argon2id')),
    salt_params     TEXT        NOT NULL CHECK (btrim(salt_params) <> ''),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    rotated_at      TIMESTAMPTZ,
    expires_at      TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_shadow_credentials_user ON shadow_credentials(user_id);

CREATE TABLE IF NOT EXISTS shadow_credential_audit (
    id         UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id    TEXT        NOT NULL CHECK (btrim(user_id) <> ''),
    action     TEXT        NOT NULL CHECK (action IN ('set', 'rotated', 'cleared', 'failed_validation')),
    actor_id   TEXT        NOT NULL CHECK (btrim(actor_id) <> ''),
    ip_address TEXT        NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_shadow_credential_audit_user ON shadow_credential_audit(user_id);
CREATE INDEX IF NOT EXISTS idx_shadow_credential_audit_created ON shadow_credential_audit(created_at);
