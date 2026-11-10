UPDATE insights_snapshots SET role_people = '{}'::jsonb WHERE role_people IS NULL;
ALTER TABLE insights_snapshots ALTER COLUMN role_people SET NOT NULL;
