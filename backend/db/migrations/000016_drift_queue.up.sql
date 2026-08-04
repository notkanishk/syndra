-- 000016_drift_queue.up.sql
-- Wave 2 · Part 4 sub-phase 2 (B2/C6): the drift triage queue for out-of-band
-- Zitadel grants Syndra has no intent record for, plus the operator's
-- "this is legitimately external, stop flagging it" exclusion list.
-- No drift item resolves automatically (design §8): every row needs explicit
-- Attribute / Revoke / Mark-external triage.

CREATE TABLE IF NOT EXISTS drift_items (
    id                      UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id                 VARCHAR(255) NOT NULL,
    project_id              VARCHAR(255) NOT NULL,
    role_keys               TEXT[] NOT NULL,
    zitadel_grant_id        TEXT,
    detected_at             TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    detection_source        TEXT NOT NULL CHECK (detection_source IN ('webhook', 'reconciliation_sweep')),
    drift_type              TEXT NOT NULL CHECK (drift_type IN ('zitadel_only', 'mkauth_only')),
    status                  TEXT NOT NULL DEFAULT 'pending_triage'
                                CHECK (status IN ('pending_triage', 'attributed', 'revoked', 'marked_external')),
    resolved_at             TIMESTAMPTZ,
    resolved_by             VARCHAR(255),
    resolution_payload_json JSONB
);

CREATE INDEX IF NOT EXISTS idx_drift_items_status ON drift_items(status, detected_at);

-- Dedupe identical PENDING detections so a noisy sweep / flapping grant cannot
-- flood the triage queue. Keyed at ROLE granularity (role_keys included) because
-- the sweep + webhook emit one single-role row per drifting role: dropping
-- role_keys from the key would silently discard the 2nd+ role on a (user,project)
-- pair. Resolved rows leave the partial index, so the same triple can re-drift.
CREATE UNIQUE INDEX IF NOT EXISTS idx_drift_items_pending_unique
    ON drift_items(user_id, project_id, drift_type, role_keys)
    WHERE status = 'pending_triage';

CREATE TABLE IF NOT EXISTS external_grant_exclusions (
    user_id     VARCHAR(255) NOT NULL,
    project_id  VARCHAR(255) NOT NULL,
    role_key    VARCHAR(255) NOT NULL,
    marked_by   VARCHAR(255) NOT NULL,
    marked_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    reason      TEXT,
    PRIMARY KEY (user_id, project_id, role_key)
);
