-- One honest picture of the access graph per local day, kept from now on (Insights
-- "As of a date"). The audit log cannot rebuild the whole graph for a past date,
-- so a date before the first row here is reported as unknown, never guessed.
CREATE TABLE insights_snapshots (
    day         DATE PRIMARY KEY,
    taken_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    topology    JSONB NOT NULL,  -- models.TopologyGraph
    role_people JSONB NOT NULL   -- {"project_id:role_key": people holding it}
);
