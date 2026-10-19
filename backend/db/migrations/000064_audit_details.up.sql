-- An audit entry may carry structured details. First user: a cancellation of an
-- unsent change (propagation.cancelled_unsent) keeps what was cancelled and
-- what cancelled it, because the outbox row it names is pruned after the
-- retention window and the audit log is the durable record.
ALTER TABLE audit_logs
    ADD COLUMN IF NOT EXISTS details JSONB;

-- The member's feed (GET /me/changes) reads one person's cancellations.
CREATE INDEX IF NOT EXISTS idx_audit_logs_cancelled_unsent_target
    ON audit_logs (target_zitadel_user_id, created_at DESC)
    WHERE action = 'propagation.cancelled_unsent';
