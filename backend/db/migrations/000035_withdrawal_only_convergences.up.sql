-- A convergence that can only take access away (§17; design §7).
--
-- The background revocation drain claims `op_type = 'revoke'`, which is a
-- Zitadel shape. An add-on revocation is an `op_type = 'apply'` row carrying a
-- resolved desired set, and the claim's own comment said why it was excluded:
-- an apply is level-triggered, so nothing on the row said which direction it
-- moved in, and a runner that dispatched one might confer access with no
-- operator in the loop.
--
-- The consequence was worse than the exclusion looked. A target revocation
-- queued a lock nothing would ever claim, told the operator in as many words
-- that it drains on its own, and appeared on no surface — retained access,
-- invisible, with a sentence promising the opposite. Three correct-looking
-- parts composing into the exact failure the revocation path exists to prevent.
--
-- So the direction stops being something the runner has to infer. The writer
-- knows: a revocation resolves a set with the lifecycle field denied, and it
-- can only subtract. It says so on the row, and the claim reads it.
--
-- FALSE by default, and that is the safe direction: a row that forgets to
-- declare itself is one the background runner will not touch, which leaves it
-- exactly where every other convergence already waits — for an operator.
ALTER TABLE propagation_outbox
    ADD COLUMN IF NOT EXISTS withdraws_only BOOLEAN NOT NULL DEFAULT FALSE;

-- Only ever true on an entitlement apply. A `revoke` row is already a
-- withdrawal by its op_type and an `add` is already not one; letting the flag
-- appear on either would create a second, disagreeing way to ask the same
-- question — and the two answers would be checked in different places.
ALTER TABLE propagation_outbox
    DROP CONSTRAINT IF EXISTS propagation_outbox_withdraws_only_check;
ALTER TABLE propagation_outbox
    ADD CONSTRAINT propagation_outbox_withdraws_only_check
    CHECK (NOT withdraws_only OR op_type = 'apply');

-- The claim's predicate, indexed the way it reads: pending withdrawals for one
-- target, in intent order. Partial, because the rows that satisfy it are a
-- small minority of the table and the runner asks this question every tick.
CREATE INDEX IF NOT EXISTS idx_outbox_pending_withdrawals
    ON propagation_outbox (target, intent_seq)
    WHERE withdraws_only AND status IN ('pending', 'in_flight');
