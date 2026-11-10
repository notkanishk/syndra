-- A map stored before one-path W2d labelled a project the directory doesn't
-- list with its own id. The map now reads an empty label as "no name", its one
-- signal (the UI's "Unknown project"), so a stored day must say the same:
-- blank those labels, keeping every node in its place. Idempotent; a day with
-- none is left untouched (production had none on 10 Nov 2026).
UPDATE insights_snapshots s
   SET topology = jsonb_set(s.topology, '{nodes}', (
         SELECT jsonb_agg(
                  CASE WHEN n->>'kind' = 'project' AND n->>'label' = n->>'project_id'
                       THEN jsonb_set(n, '{label}', '""'::jsonb)
                       ELSE n END
                  ORDER BY i)
           FROM jsonb_array_elements(s.topology->'nodes') WITH ORDINALITY AS t(n, i)))
 WHERE EXISTS (SELECT 1 FROM jsonb_array_elements(s.topology->'nodes') AS n
                WHERE n->>'kind' = 'project' AND n->>'label' = n->>'project_id');
