-- 000026_addon_platform_target_dimension.up.sql
-- Add-on platform, group 1: the target dimension (change `addon-platform`,
-- design §3 and §15). Schema only. Every existing row keeps its meaning; the
-- tables that carried "the thing Syndra propagates to" simply stop assuming
-- there is exactly one of them.
--
-- Ordering matters: `targets` must exist before anything can reference it, and
-- the outbox rename must happen before that table is reshaped under its new
-- name.

-- 1.1 --------------------------------------------------------------------
-- The registry. A foreign key, not a CHECK: a CHECK would make registering a
-- later add-on a schema migration, so configuration and schema would have to
-- move together and a config-only deployment could write rows the database
-- refuses (design §3).
--
-- `state` is how a target retires. Unregistering is disabling, NEVER deleting:
-- propagation and drift history keeps pointing here and must keep resolving.
-- "The drain must not dispatch work for an unregistered target" is therefore a
-- state check the drain performs, not a property this foreign key provides.
CREATE TABLE IF NOT EXISTS targets (
    target        TEXT PRIMARY KEY,
    state         TEXT NOT NULL DEFAULT 'active' CHECK (state IN ('active', 'disabled')),
    registered_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO targets (target, state) VALUES ('zitadel', 'active')
    ON CONFLICT (target) DO NOTHING;

-- 1.2 --------------------------------------------------------------------
-- `pending_zitadel_propagations` becomes false the moment a second target
-- exists. Rename it here rather than leaving a trap for the next reader.
-- Indexes and constraints carry the old name too; Postgres does not rename
-- them with the table, so nothing below is cosmetic.
ALTER TABLE IF EXISTS pending_zitadel_propagations RENAME TO propagation_outbox;

ALTER INDEX IF EXISTS idx_pending_zitadel_propagations_status
    RENAME TO idx_propagation_outbox_status;
ALTER INDEX IF EXISTS idx_pending_zitadel_propagations_source
    RENAME TO idx_propagation_outbox_source;
ALTER INDEX IF EXISTS idx_pending_zitadel_propagations_cascade
    RENAME TO idx_propagation_outbox_cascade;
ALTER INDEX IF EXISTS pending_zitadel_propagations_pkey
    RENAME TO propagation_outbox_pkey;
ALTER INDEX IF EXISTS pending_zitadel_propagations_idempotency_key_key
    RENAME TO propagation_outbox_idempotency_key_key;

-- CHECK constraints are dropped and re-added rather than renamed, because two
-- of the three are being widened anyway and a mixed rename/redefine reads worse
-- than three symmetric statements.
ALTER TABLE propagation_outbox
    DROP CONSTRAINT IF EXISTS pending_zitadel_propagations_source_check;
ALTER TABLE propagation_outbox
    ADD CONSTRAINT propagation_outbox_source_check
    CHECK (source IN ('direct', 'bundle', 'rule', 'external_backfill', 'lifecycle_cascade'));

-- `superseded` is installed now though nothing writes it until the drain learns
-- version rejection (design §15). Same reasoning migration 000015 used for the
-- 5-value source enum: a status the schema forbids is a runtime error waiting
-- for a second ALTER, and this migration is where the outbox is already open.
ALTER TABLE propagation_outbox
    DROP CONSTRAINT IF EXISTS pending_zitadel_propagations_status_check;
ALTER TABLE propagation_outbox
    ADD CONSTRAINT propagation_outbox_status_check
    CHECK (status IN ('pending', 'in_flight', 'applied', 'failed', 'superseded'));

-- 1.3 --------------------------------------------------------------------
-- `apply` is the add-on op_type: a level-triggered convergence onto a resolved
-- desired set, which has no add/revoke/replace distinction to make.
ALTER TABLE propagation_outbox
    DROP CONSTRAINT IF EXISTS pending_zitadel_propagations_op_type_check;
ALTER TABLE propagation_outbox
    ADD CONSTRAINT propagation_outbox_op_type_check
    CHECK (op_type IN ('add', 'revoke', 'replace', 'apply'));

-- The Zitadel-shaped columns relax so an add-on row can exist at all: a TrueNAS
-- entitlement apply has no project, no role keys, and no grant id. Its intent
-- lives in `desired_state_snapshots` instead (design §3).
ALTER TABLE propagation_outbox
    ALTER COLUMN project_id DROP NOT NULL,
    ALTER COLUMN role_keys  DROP NOT NULL;

ALTER TABLE propagation_outbox
    ADD COLUMN IF NOT EXISTS target TEXT NOT NULL DEFAULT 'zitadel' REFERENCES targets(target);

-- Relaxing the NOT NULLs is what lets an add-on row exist; this CHECK is what
-- stops that becoming a licence to write a half-formed Zitadel row. The columns
-- stay mandatory for the target that actually has them.
ALTER TABLE propagation_outbox
    ADD CONSTRAINT propagation_outbox_zitadel_shape_check
    CHECK (target <> 'zitadel' OR (project_id IS NOT NULL AND role_keys IS NOT NULL));

CREATE INDEX IF NOT EXISTS idx_propagation_outbox_target_status
    ON propagation_outbox(target, status, created_at);

-- 1.4 --------------------------------------------------------------------
-- An outbox row saying "converge this subject" is under-specified: the drain
-- runs later, the resolver recomputes, and what lands may not be what anyone
-- approved. Each entitlement change records the desired set it approved, and
-- the row references that (design §15).
--
-- Rows are immutable audit records. The trigger below says so in the one place
-- that cannot be forgotten by a future writer.
CREATE TABLE IF NOT EXISTS desired_state_snapshots (
    id         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    subject_id VARCHAR(255) NOT NULL,
    target     TEXT NOT NULL REFERENCES targets(target),
    -- Allocated by the trigger below, never supplied by the writer. The default
    -- exists so an INSERT can legally omit it; a supplied value is replaced.
    version    BIGINT NOT NULL DEFAULT 0 CHECK (version > 0),
    state_json JSONB NOT NULL,
    created_by VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (subject_id, target, version)
);

CREATE OR REPLACE FUNCTION reject_desired_state_snapshot_mutation() RETURNS trigger
    LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'desired_state_snapshots rows are immutable audit records (attempted %)', TG_OP;
END;
$$;

DROP TRIGGER IF EXISTS desired_state_snapshots_immutable ON desired_state_snapshots;
CREATE TRIGGER desired_state_snapshots_immutable
    BEFORE UPDATE OR DELETE ON desired_state_snapshots
    FOR EACH ROW EXECUTE FUNCTION reject_desired_state_snapshot_mutation();

-- The UNIQUE above forbids two rows sharing a version. It does NOT make versions
-- monotonic: it permits version 2 followed by version 1 for one (subject,
-- target), and a stale-version check compares against "the last version
-- applied", which a backwards insert makes a lie — on the mechanism whose whole
-- job is stopping a queued grant landing after a newer revoke.
--
-- So the version is ALLOCATED here, not validated here. Validating it would
-- leave every writer to read MAX, propose MAX+1, and retry on the unique
-- violation when it lost the race — the same loop, written again, in each of
-- them, and wrong in whichever one forgets. Allocation under a pair-scoped
-- advisory lock means the loop exists nowhere: a concurrent writer for the same
-- (subject, target) blocks until the first commits and then reads the version it
-- wrote. The next version for a pair is not a fact a writer can know, so it is
-- not a value a writer supplies.
--
-- The lock is transaction-scoped, so it releases on commit or rollback with
-- nothing to leak. hashtext can collide, which costs two unrelated pairs a
-- little needless serialization and costs correctness nothing.
--
-- ponytail: a transaction inserting snapshots for many subjects takes one lock
-- per pair, so bulk writers must insert in a deterministic subject order or two
-- overlapping bulk writes can deadlock. Ordering is free at the call site; if
-- that ever stops being true, take one lock for the whole target instead.
--
-- UNIQUE stays as the backstop that does not depend on this trigger existing.
CREATE OR REPLACE FUNCTION enforce_desired_state_snapshot_version() RETURNS trigger
    LANGUAGE plpgsql AS $$
DECLARE
    last_version BIGINT;
BEGIN
    PERFORM pg_advisory_xact_lock(hashtext(NEW.subject_id || ':' || NEW.target));

    SELECT COALESCE(MAX(version), 0) INTO last_version
      FROM desired_state_snapshots
     WHERE subject_id = NEW.subject_id AND target = NEW.target;

    NEW.version := last_version + 1;
    RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS desired_state_snapshots_version_monotonic ON desired_state_snapshots;
CREATE TRIGGER desired_state_snapshots_version_monotonic
    BEFORE INSERT ON desired_state_snapshots
    FOR EACH ROW EXECUTE FUNCTION enforce_desired_state_snapshot_version();

-- 1.5 --------------------------------------------------------------------
-- Plan storage. One approval, one durable object: the per-subject row holds
-- both the snapshot that was approved and the fingerprint of the state it was
-- approved against, and the outbox references THAT row rather than the snapshot
-- directly. An outbox row pointing at a snapshot while a plan separately held
-- the fingerprints would be two records of one decision, free to disagree
-- (design §8).
--
-- `surface` is which rehearsal issued the plan. Without it, a plan issued by
-- drift triage could be cited on the bulk-grant apply endpoint.
--
-- `state_read_at` is when the target read behind the fingerprints was taken.
-- It is not derivable from created_at: a provisional plan is computed against a
-- last-known read that may be days older than the plan itself, and the operator
-- surface has to say how old.
CREATE TABLE IF NOT EXISTS plans (
    id            UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    target        TEXT NOT NULL REFERENCES targets(target),
    surface       TEXT NOT NULL,
    created_by    VARCHAR(255) NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at    TIMESTAMPTZ,
    provisional   BOOLEAN NOT NULL DEFAULT FALSE,
    state_read_at TIMESTAMPTZ,
    -- A confirmed plan MUST carry a lifetime; a provisional one MUST NOT.
    -- The ordinary lifetime bounds how long a *verified* plan may sit unexecuted,
    -- and applying it to a provisional plan would silently discard an approved
    -- change whenever an outage outlasted it. A provisional plan is gated by
    -- re-fingerprinting on the target's return, not by a clock, and must be
    -- labelled with the age of the read it was computed against (design §8).
    CONSTRAINT plans_lifetime_check CHECK (
        (provisional AND expires_at IS NULL AND state_read_at IS NOT NULL)
        OR (NOT provisional AND expires_at IS NOT NULL)
    )
);

CREATE TABLE IF NOT EXISTS plan_subjects (
    id           UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    plan_id      UUID NOT NULL REFERENCES plans(id),
    subject_id   VARCHAR(255) NOT NULL,
    -- NULL for Zitadel plans, whose intent is the outbox row's own columns.
    snapshot_id  UUID REFERENCES desired_state_snapshots(id),
    fingerprint  TEXT NOT NULL,
    outcome_json JSONB NOT NULL,
    UNIQUE (plan_id, subject_id)
);

ALTER TABLE propagation_outbox
    ADD COLUMN IF NOT EXISTS plan_subject_id UUID REFERENCES plan_subjects(id);

-- 1.6 --------------------------------------------------------------------
-- No foreign key above carries ON DELETE CASCADE, deliberately. Plan expiry
-- MUST NOT reach a snapshot: snapshots are audit records that outlive the plan
-- that produced them. With NO ACTION, deleting a plan whose subject rows an
-- outbox row still cites is refused, and a snapshot can never be dragged along
-- behind a plan. Expiry is a timestamp comparison at citation time, not a
-- delete. The immutability trigger on desired_state_snapshots is the second
-- lock on the same door.
--
-- No column on either table can hold a declared secret: plans persist intent —
-- who, on whom, and against what state — and `secret_params` values ride the
-- apply request and are discarded with it (design §5).

-- 1.7 --------------------------------------------------------------------
-- Drift gains the same dimension, and one thing more. The pending-dedupe unique
-- index on (user_id, project_id, drift_type, role_keys) would collide across
-- targets: two targets drifting on one user would silently suppress one of them
-- (design §3).
ALTER TABLE drift_items
    ADD COLUMN IF NOT EXISTS target TEXT NOT NULL DEFAULT 'zitadel' REFERENCES targets(target);

ALTER TABLE drift_items
    ALTER COLUMN project_id DROP NOT NULL,
    ALTER COLUMN role_keys  DROP NOT NULL;

ALTER TABLE drift_items
    ADD CONSTRAINT drift_items_zitadel_shape_check
    CHECK (target <> 'zitadel' OR (project_id IS NOT NULL AND role_keys IS NOT NULL));

-- `zitadel_only` names the target inside the value, which is exactly the shape
-- this change is removing. On a TrueNAS drift row it would be a false
-- statement. The value becomes `target_only` — "present on the target,
-- unexplained by Syndra" — with the target named by the column beside it.
-- Same move, same order-of-operations, as the MkAuth -> Syndra rename in
-- 000025: the constraint comes off before the UPDATE, because the old
-- constraint forbids the new value and the new one forbids the old rows.
ALTER TABLE drift_items DROP CONSTRAINT IF EXISTS drift_items_drift_type_check;

UPDATE drift_items SET drift_type = 'target_only' WHERE drift_type = 'zitadel_only';

ALTER TABLE drift_items
    ADD CONSTRAINT drift_items_drift_type_check
    CHECK (drift_type IN ('target_only', 'syndra_only'));

-- NULLS NOT DISTINCT (Postgres 15) so an add-on row, whose project_id and
-- role_keys are NULL, still dedupes. Under the default NULLS DISTINCT every
-- re-detection would insert a fresh row and the triage queue would flood on the
-- first target that is not Zitadel.
DROP INDEX IF EXISTS idx_drift_items_pending_unique;
CREATE UNIQUE INDEX idx_drift_items_pending_unique
    ON drift_items (target, user_id, project_id, drift_type, role_keys) NULLS NOT DISTINCT
    WHERE status = 'pending_triage';

-- An exclusion is "this grant is legitimately external, stop flagging it". It
-- is a statement about one target, so two targets holding the same
-- (user, project, role) must be excludable independently.
ALTER TABLE external_grant_exclusions
    ADD COLUMN IF NOT EXISTS target TEXT NOT NULL DEFAULT 'zitadel' REFERENCES targets(target);

ALTER TABLE external_grant_exclusions
    DROP CONSTRAINT IF EXISTS external_grant_exclusions_pkey;
ALTER TABLE external_grant_exclusions
    ADD PRIMARY KEY (target, user_id, project_id, role_key);

-- 1.9 --------------------------------------------------------------------
-- `direct_role_grants` deliberately gains NO target column. Direct grants are
-- intents against Zitadel user_grants; add-on entitlements come from mappings
-- and allowances, which have their own tables. A column no code path can
-- populate is a column someone will later assume means something (design §3).
