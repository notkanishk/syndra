-- Role metadata for Syndra-managed roles.
-- Roles created through Syndra are propagated to Zitadel and cached locally.
-- Demo catalog and Zitadel-only roles are NOT stored here; the global catalog
-- merges sources at query time.

CREATE TABLE IF NOT EXISTS roles (
    id                  UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    zitadel_project_id  TEXT        NOT NULL CHECK (btrim(zitadel_project_id) <> ''),
    role_key            TEXT        NOT NULL CHECK (btrim(role_key) <> ''),
    display_name        TEXT        NOT NULL CHECK (btrim(display_name) <> ''),
    description         TEXT        NOT NULL DEFAULT '',
    role_group          TEXT        NOT NULL DEFAULT '',
    cloned_from_project TEXT,
    cloned_from_role    TEXT,
    created_by          TEXT        NOT NULL DEFAULT 'system',
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (zitadel_project_id, role_key)
);

CREATE INDEX IF NOT EXISTS idx_roles_project ON roles(zitadel_project_id);
