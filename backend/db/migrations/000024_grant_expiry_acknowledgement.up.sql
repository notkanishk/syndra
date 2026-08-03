-- C4 — an operator can record "I have seen this and I am deliberately letting it lapse".
--
-- The reopen rule is "when the grant changes", and it is implemented by STORING WHAT WAS
-- ACKNOWLEDGED rather than by invalidating anything. An acknowledgement names the expiry date it
-- was made against; it applies only while the grant still carries that date. Validity is a
-- comparison at read time — no trigger, no sweep, no second copy of the truth to keep coherent,
-- and nothing to go stale silently.
--
-- What that gets right: an operator agrees to let a specific role lapse on a specific day. Move
-- the day and they have agreed to something that no longer exists, so the row returns and asks
-- again. `direct_role_grants` is keyed UNIQUE on (user, project, role) and upserts in place, so a
-- re-grant keeps the same id and changes expires_at — exactly the case this catches.
--
-- ON DELETE CASCADE, unlike the deliberate absence of a foreign key on audit_logs.cascade_id
-- (000023) and onboarding_triggers.bundle_id (000021). Those are history and must outlive their
-- subject. This is not history: it is an annotation on a live row, meaningless once the grant is
-- gone, and the grant being gone is the normal end of its life (the expiry sweep deletes it). The
-- history of who acknowledged what lives in audit_logs, where it belongs.
CREATE TABLE IF NOT EXISTS grant_expiry_acknowledgements (
    -- One acknowledgement per grant. Re-acknowledging after a change REPLACES the stale row
    -- rather than stacking: this table holds the current annotation, not the record of every
    -- decision ever taken about it.
    grant_id UUID PRIMARY KEY REFERENCES direct_role_grants(id) ON DELETE CASCADE,

    -- The date that was acknowledged. NOT NULL: a grant with no expiry never reaches this screen,
    -- so an acknowledgement of one would be an agreement about nothing.
    acknowledged_expires_at TIMESTAMP WITH TIME ZONE NOT NULL,

    acknowledged_by VARCHAR(255) NOT NULL,
    acknowledged_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,

    -- Optional, and the reason this is worth a dialog rather than a single click: the next
    -- operator reading the queue is the audience.
    note TEXT
);

-- The read is "for these expiring grants, which are acknowledged" — a join on the primary key,
-- which needs no further index. This one serves the operator question "what have I acknowledged",
-- which is the other direction.
CREATE INDEX IF NOT EXISTS idx_grant_expiry_ack_by
    ON grant_expiry_acknowledgements(acknowledged_by, acknowledged_at DESC);
