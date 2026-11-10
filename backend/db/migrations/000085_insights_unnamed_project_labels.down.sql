-- Back to the id standing in as an unlisted project's label.
UPDATE insights_snapshots s
   SET topology = jsonb_set(s.topology, '{nodes}', (
         SELECT jsonb_agg(
                  CASE WHEN n->>'kind' = 'project' AND n->>'label' = '' AND n ? 'project_id'
                       THEN jsonb_set(n, '{label}', to_jsonb(n->>'project_id'))
                       ELSE n END
                  ORDER BY i)
           FROM jsonb_array_elements(s.topology->'nodes') WITH ORDINALITY AS t(n, i)))
 WHERE EXISTS (SELECT 1 FROM jsonb_array_elements(s.topology->'nodes') AS n
                WHERE n->>'kind' = 'project' AND n->>'label' = '');
