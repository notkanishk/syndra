-- 000071_app_claim_shapes
--
-- A shape belongs to one application (A-14, ADR-003). Every save is an
-- immutable version; "go back" saves an old version again as a new one. The
-- latest version of each app is what the data plane serves; an app with no
-- row uses its project's default (claim_profiles), else the built-in `roles`.
--
-- profile is internal/claims.Profile as JSON: ordered claim rows, the role
-- words and inclusions, roles-added-later, and where the claims land.
-- client_id is stored with each version so the data plane maps a sign-in's
-- application.client_id to its shape without asking Zitadel.
CREATE TABLE IF NOT EXISTS app_claim_shape_versions (
    application_id    TEXT        NOT NULL,
    version           INTEGER     NOT NULL CHECK (version > 0),
    project_id        TEXT        NOT NULL,
    client_id         TEXT        NOT NULL DEFAULT '',
    profile           JSONB       NOT NULL,
    template_id       TEXT        NOT NULL DEFAULT '',
    template_version  INTEGER     NOT NULL DEFAULT 0,
    copied_from_application_id TEXT,
    copied_from_version        INTEGER,
    restored_from     INTEGER,
    note              TEXT        NOT NULL DEFAULT '',
    saved_by          TEXT        NOT NULL,
    saved_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (application_id, version)
);

CREATE INDEX IF NOT EXISTS idx_app_claim_shape_versions_project
    ON app_claim_shape_versions (project_id, application_id, version DESC);

-- The seed. Applications live in Zitadel, not here, so the rows can't be
-- written in SQL: the API seeds lazily (services.SeedClaimShapes, at start
-- and hourly) every app Zitadel created before this cutoff that has no shape,
-- with a Custom shape reproducing Actions v1 exactly, as version 1 "As
-- Actions v1 sent it". Apps created after the cutoff start on their project's
-- default. The cutoff is this migration's own time, per environment.
CREATE TABLE IF NOT EXISTS app_claim_shape_seed (
    only_row BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK (only_row),
    cutoff   TIMESTAMPTZ NOT NULL DEFAULT now()
);
INSERT INTO app_claim_shape_seed (only_row) VALUES (TRUE) ON CONFLICT DO NOTHING;
