-- Local index of Zitadel user-grant aggregates.
-- Populated from user.grant.added webhook events; consulted by the translator
-- enrichment step to fill user.grant.changed (no projectId in payload) and
-- user.grant.removed (no roleKeys in payload). On a miss, the translator
-- falls back to a synchronous Zitadel ListUserGrants call.
CREATE TABLE IF NOT EXISTS zitadel_grants_index (
    grant_id     TEXT PRIMARY KEY,
    user_id      TEXT NOT NULL CHECK (btrim(user_id) <> ''),
    project_id   TEXT NOT NULL CHECK (btrim(project_id) <> ''),
    role_keys    TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[],
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_zitadel_grants_index_user_id
    ON zitadel_grants_index (user_id);

CREATE INDEX IF NOT EXISTS idx_zitadel_grants_index_project_id
    ON zitadel_grants_index (project_id);
