-- 000072_claim_delivery
--
-- Telemetry of every Actions v2 call (claims-delivery-integrity S1, assessment
-- §4a). Zitadel issues the token whatever Syndra answers, so an answer without
-- Syndra's claims is silent unless it is counted. Counts only: never one row
-- per call. The backend keeps counters in memory and folds them in once a
-- minute; a row is one 5-minute bucket of one (project, client, function,
-- signing-key slot, outcome), with a fixed latency histogram.
--
-- buckets is the histogram: calls at or under 1, 2, 5, 10, 25, 50, 100, 250,
-- 500, 1000 and 3000 ms, then +Inf. Always 12 long.
--
-- An absent dimension is '-', never NULL, so the primary key holds.

CREATE OR REPLACE FUNCTION claim_hist_add(a BIGINT[], b BIGINT[]) RETURNS BIGINT[]
    LANGUAGE sql IMMUTABLE PARALLEL SAFE AS $$
    SELECT array_agg(COALESCE(x, 0) + COALESCE(y, 0) ORDER BY i)
    FROM unnest(a, b) WITH ORDINALITY AS t(x, y, i)
$$;

CREATE AGGREGATE claim_hist_sum(BIGINT[]) (
    SFUNC = claim_hist_add,
    STYPE = BIGINT[],
    INITCOND = '{0,0,0,0,0,0,0,0,0,0,0,0}'
);

CREATE TABLE IF NOT EXISTS claim_delivery_bucket (
    bucket     TIMESTAMPTZ NOT NULL,
    project_id TEXT        NOT NULL,
    client_id  TEXT        NOT NULL,
    function   TEXT        NOT NULL,
    key_slot   TEXT        NOT NULL,
    outcome    TEXT        NOT NULL,
    n          BIGINT      NOT NULL CHECK (n >= 0),
    buckets    BIGINT[]    NOT NULL CHECK (cardinality(buckets) = 12),
    PRIMARY KEY (bucket, project_id, client_id, function, key_slot, outcome)
);

-- Rows older than 48h roll up here nightly (on the usage-prune scheduler) and
-- are kept 90 days.
CREATE TABLE IF NOT EXISTS claim_delivery_hour (
    hour       TIMESTAMPTZ NOT NULL,
    project_id TEXT        NOT NULL,
    client_id  TEXT        NOT NULL,
    function   TEXT        NOT NULL,
    key_slot   TEXT        NOT NULL,
    outcome    TEXT        NOT NULL,
    n          BIGINT      NOT NULL CHECK (n >= 0),
    buckets    BIGINT[]    NOT NULL CHECK (cardinality(buckets) = 12),
    PRIMARY KEY (hour, project_id, client_id, function, key_slot, outcome)
);

-- When each app last reached Syndra through each function, and when it was
-- last served. client_known: the app has a shape of its own (false means the
-- project's default answered, assessment A5).
CREATE TABLE IF NOT EXISTS claim_delivery_last (
    client_id      TEXT        NOT NULL,
    function       TEXT        NOT NULL,
    last_at        TIMESTAMPTZ NOT NULL,
    last_outcome   TEXT        NOT NULL,
    last_served_at TIMESTAMPTZ,
    client_known   BOOLEAN     NOT NULL DEFAULT false,
    PRIMARY KEY (client_id, function)
);

-- Each payload shape Zitadel has sent: its key paths, hashed. The first body of
-- each shape is kept, redacted (no person's data), as a contract-test fixture.
-- A shape first seen after the earliest one for its function is the finding
-- "Zitadel's payload changed shape".
CREATE TABLE IF NOT EXISTS claim_payload_fingerprints (
    fp         TEXT        PRIMARY KEY,
    function   TEXT        NOT NULL,
    paths      TEXT[]      NOT NULL,
    example    JSONB,
    first_seen TIMESTAMPTZ NOT NULL DEFAULT now()
);
