-- The operations pages list each log newest first with a LIMIT, and the 90-day
-- prune of webhook_events deletes by age. Without an index on created_at both
-- read the whole table.
CREATE INDEX IF NOT EXISTS idx_webhook_events_created_at ON webhook_events (created_at DESC);
CREATE INDEX IF NOT EXISTS idx_onboarding_triggers_created_at ON onboarding_triggers (created_at DESC);
