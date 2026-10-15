-- Why a convergence was queued, in the operator's words (truenas-reconcile,
-- found 15 Oct 2026). An add-on row has no project and no roles, so Pending
-- changes had nothing to say about it. Optional: rows queued before this, and
-- Zitadel rows, which say what they do through their project and roles.
ALTER TABLE propagation_outbox ADD COLUMN IF NOT EXISTS reason text;
