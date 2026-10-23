-- Syndra's own record of a service account: the operator's mark, kind, purpose
-- and owner (A-07 §26-31). Zitadel stays the source of whether a user is a
-- machine; a row for a machine user only adds what Zitadel cannot say (what it
-- is for, who answers for it), and a row for a human-type user IS the mark.
-- No naming rule, no metadata key: the row is the one place.
CREATE TABLE service_accounts (
    user_id    TEXT PRIMARY KEY,
    kind       TEXT NOT NULL CHECK (kind IN ('integration', 'device', 'agent', 'shared_login')),
    purpose    TEXT NOT NULL DEFAULT '',
    owner_id   TEXT,
    marked_by  TEXT NOT NULL,
    marked_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
