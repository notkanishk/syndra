-- Migration 005: Production security boundary additions
-- Adds explicit claim failure modes to claim_profiles and an idempotent onboarding
-- trigger table for backend-owned welcome-bundle assignment.

-- 1. Explicit claim-injection degraded behavior per project
--    fail_closed (default): emit no custom claims on cache miss/error
--    minimal_safe: emit a configured minimal claim set on cache miss/error
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'claim_profiles' AND column_name = 'claim_failure_mode'
    ) THEN
        ALTER TABLE claim_profiles
            ADD COLUMN claim_failure_mode TEXT NOT NULL DEFAULT 'fail_closed'
                CONSTRAINT ck_claim_profiles_failure_mode CHECK (claim_failure_mode IN ('fail_closed', 'minimal_safe')),
            ADD COLUMN minimal_safe_claims JSONB;
    END IF;
END $$;

-- 2. Onboarding triggers — idempotent record of welcome-bundle assignment events
CREATE TABLE IF NOT EXISTS onboarding_triggers (
    id              UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id         TEXT        NOT NULL CHECK (btrim(user_id) <> ''),
    source          TEXT        NOT NULL CHECK (source IN ('webhook', 'manual')),
    idempotency_key TEXT        NOT NULL UNIQUE,
    status          TEXT        NOT NULL DEFAULT 'pending'
                                    CONSTRAINT ck_onboarding_triggers_status
                                    CHECK (status IN ('pending', 'completed', 'failed')),
    bundle_id       UUID        REFERENCES bundles(id),
    error_message   TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at    TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_onboarding_triggers_user ON onboarding_triggers(user_id);
CREATE INDEX IF NOT EXISTS idx_onboarding_triggers_status ON onboarding_triggers(status);
