-- Roles that wait for you (reconcile N14 option B, N15).
--
-- The pause moves from bundles and rules onto roles: a role marked here
-- waits in Pending changes whenever anyone would gain or lose it, however it
-- happens; every other role goes straight to Zitadel. The old per-bundle and
-- per-rule confirmation_mode columns stay until the operator switches over
-- (config key confirmation.model = 'roles'), then are dropped by a later
-- migration.
CREATE TABLE role_waits (
    project_id VARCHAR(255) NOT NULL,
    role_key   TEXT         NOT NULL,
    marked_by  VARCHAR(255) NOT NULL,
    marked_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    PRIMARY KEY (project_id, role_key)
);

-- Why a row is waiting, so Pending changes and the step left can say it:
-- role_waits (the role always waits), halted:<reason> (meant to go, and the
-- send stopped: zitadel_offline, drain_in_progress), manual (a bundle or rule
-- set to wait, before the switch over). NULL for rows written before this.
ALTER TABLE propagation_outbox
    ADD COLUMN wait_reason TEXT
    CHECK (wait_reason IS NULL OR wait_reason IN ('role_waits', 'manual') OR wait_reason LIKE 'halted:%');
