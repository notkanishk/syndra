-- 000074_claim_findings
--
-- claims-delivery-integrity S3/S4 (assessment §4b, §4c): one row per
-- condition the detectors find, never one per event. kind is what was found
-- (CLAIMS_NOT_CALLED, CLAIMS_NOT_SERVED, SIGNIN_EVENTS_MISSING, CONFIG_DRIFT,
-- CANNOT_CHECK); subject is where (an app's client id, or a configuration
-- check such as 'R1:endpoint'). A row keeps when it was first and last seen,
-- how often, and one sample. It settles on its own after enough clean looks
-- in a row (three sign-ins, or one configuration read): resolved_at is set
-- and clean_streak says how many. A failure after that starts it again.
CREATE TABLE IF NOT EXISTS claim_findings (
    kind         TEXT        NOT NULL,
    subject      TEXT        NOT NULL,
    first_seen   TIMESTAMPTZ NOT NULL,
    last_seen    TIMESTAMPTZ NOT NULL,
    count        BIGINT      NOT NULL CHECK (count > 0),
    sample       JSONB       NOT NULL DEFAULT '{}',
    clean_streak INTEGER     NOT NULL DEFAULT 0 CHECK (clean_streak >= 0),
    resolved_at  TIMESTAMPTZ,
    PRIMARY KEY (kind, subject)
);

CREATE INDEX IF NOT EXISTS claim_findings_open_idx ON claim_findings (kind) WHERE resolved_at IS NULL;
