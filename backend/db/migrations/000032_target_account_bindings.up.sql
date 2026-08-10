-- 000032_target_account_bindings.up.sql
-- Add-on platform, group 1 (1.18/1.19): which account on a target belongs to
-- which subject, recorded by the backend.
--
-- The add-on already keeps this in its local store, and that store is where the
-- apply resolves a subject to an account. This table is not a second authority
-- over that decision — it is the backend's own record of what the add-on
-- reported, and it exists because three things need it and none of them can ask
-- the add-on:
--
--   * The unmanaged inventory. A target holds accounts Syndra never provisioned
--     — `root`, service accounts, whatever an admin made by hand. Telling them
--     apart from the ones Syndra DID provision is what stops the first sweep
--     after deployment burying the triage queue in findings that are not
--     findings, and "which of these do we manage" is a question about Syndra's
--     history, not about the target's present.
--   * The member's own view. It shows the account name they connect with, and
--     resolving that through a full state read on every page load would put a
--     rate-limited WebSocket behind an ordinary page.
--   * Recovery. Design §11 says the derivation is a recovery path and the
--     recorded binding is authoritative; if the add-on's local store is lost,
--     this is the record that says which account was whose.
--
-- Deliberately NOT a source of truth the apply consults. Two stores deciding
-- which account belongs to a subject is two answers to the one question where
-- being wrong hands somebody else's home directory to a member.

CREATE TABLE IF NOT EXISTS target_account_bindings (
    target     TEXT         NOT NULL REFERENCES targets(target),
    subject_id VARCHAR(255) NOT NULL,
    -- The account's name on the target, as the add-on reported it. Mutable: a
    -- rename out of band moves it, and the uid below is what recognises the
    -- account across that.
    username   TEXT         NOT NULL,
    -- The target's stable identity for the account. Nullable because not every
    -- target has one — this table is not TrueNAS-shaped — and because a binding
    -- recorded before uids were reported carries none.
    account_uid BIGINT,
    -- Who decided this. An adoption hands an existing account to a subject, and
    -- if it is wrong it hands them somebody else's data, so the actor survives
    -- separately from "the apply created it".
    bound_by   VARCHAR(255) NOT NULL,
    bound_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    -- When the backend last saw this binding confirmed by the target. Not the
    -- same as bound_at: a binding recorded a year ago and confirmed this morning
    -- is a different fact from one nothing has confirmed since.
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    PRIMARY KEY (target, subject_id)
);

-- One account, one subject. Without this, two subjects could be recorded
-- against one account and both would be told it is theirs — and the second one
-- to converge would take the first one's groups away.
CREATE UNIQUE INDEX IF NOT EXISTS idx_target_account_bindings_username
    ON target_account_bindings(target, username);

-- The same rule on the stable identity, where the target has one. A rename
-- moves `username` and leaves this alone, which is exactly why it is the
-- stronger of the two.
CREATE UNIQUE INDEX IF NOT EXISTS idx_target_account_bindings_uid
    ON target_account_bindings(target, account_uid)
    WHERE account_uid IS NOT NULL;
