-- 000034_retire_the_lldap_bridge.up.sql
-- Add-on platform, group 11: the LLDAP path is gone, and so is what it needed.
--
-- THIS IS THE POINT OF NO RETURN, and it is one migration rather than two on
-- purpose: the intent queue and the credential vault were the two halves of one
-- design — Syndra hashed a password, the sync service read the hash and pushed
-- it into LLDAP, and a queue of group memberships travelled beside it. Neither
-- half means anything without the other.
--
-- What replaces it does not need either. A target's changes live in the
-- propagation outbox beside Zitadel's, dispatched by an add-on that owns the
-- target; and a member's credential is forwarded to that target and kept
-- nowhere, because no API here accepts a hash and the only thing a stored one
-- could ever do is leak.
--
-- Every enrolled member must set a new credential. That is not a bug in this
-- migration, it is what removing a password store means: the hashes cannot be
-- converted into anything the new path can use — TrueNAS takes plaintext and
-- nothing else — so keeping them would keep the risk and buy nothing. The
-- member view renders the un-enrolled state (task 11.8/11.9), and the operator
-- communication goes out before this ships.

-- 11.5 ------------------------------------------------------------------
-- The queue itself. Dropped whole rather than emptied: a table nothing writes
-- to and nothing reads is a table somebody wires back up.
DROP TABLE IF EXISTS provisioning_intents;

-- 11.6 ------------------------------------------------------------------
-- The vault reduction. What survives is EXISTENCE and ROTATION METADATA — that
-- a member has set a credential, and when — because that is what the member's
-- own view renders and what an operator needs to answer "have they enrolled".
--
-- What goes is the only part that was ever a liability: the hash, the algorithm
-- that describes how to attack it, and the salt parameters that complete it.
ALTER TABLE shadow_credentials
    DROP COLUMN IF EXISTS credential_hash,
    DROP COLUMN IF EXISTS algorithm,
    DROP COLUMN IF EXISTS salt_params;

-- Every row that survives describes a credential set through the OLD path,
-- against a system that no longer exists. Marked rather than deleted: the row
-- is how the member view knows to say "you enrolled before the change and need
-- to set a new one" instead of "you have never set one", and those are
-- different sentences to somebody who remembers doing it.
ALTER TABLE shadow_credentials
    ADD COLUMN IF NOT EXISTS enrolled_before_cutover BOOLEAN NOT NULL DEFAULT FALSE;

UPDATE shadow_credentials SET enrolled_before_cutover = TRUE;

-- And the default flips, so a credential set after this migration is not
-- marked. Left at TRUE it would mark every future enrolment as pre-cutover,
-- which is the same class of mistake as a `target` column defaulting to
-- 'zitadel' — the schema answering on behalf of a statement that did not say.
ALTER TABLE shadow_credentials ALTER COLUMN enrolled_before_cutover SET DEFAULT FALSE;

-- The coherence guard this migration owes, expressed as a constraint rather
-- than as a comment: no column on this table may hold a credential. A future
-- migration adding one has to remove this first, which is a conversation.
--
-- It cannot check content — a TEXT column will take anything — so it checks the
-- one thing a schema can: that the columns whose names describe a stored secret
-- are not here. Postgres has no "reject a column name" constraint, so this is a
-- trigger on the table's own DDL... which Postgres also does not offer per
-- table. What remains is the source guard in `internal/db`, and this comment
-- naming the rule it enforces: shadow_credentials holds metadata, never a
-- credential.
COMMENT ON TABLE shadow_credentials IS
    'Existence and rotation metadata for a member''s target credential. Holds NO credential: the value is forwarded to the target and kept nowhere. See addon-platform group 11.';
