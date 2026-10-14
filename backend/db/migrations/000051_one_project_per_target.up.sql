-- Each target serves exactly one Zitadel project (truenas-reconcile D3, owner
-- 8 Oct 2026). A mapping is then a role and a group: the project is the
-- target's, not each row's.
--
-- Mapping rows KEEP their project_id. Fifteen resolution sites join on
-- (project_id, role_key), and none of them has to change. What changes is that
-- the database now refuses a row whose project is not its target's, by a
-- composite key, so no path can write one.

ALTER TABLE targets ADD COLUMN IF NOT EXISTS project_id VARCHAR(255);

-- A project serves one system.
ALTER TABLE targets ADD CONSTRAINT targets_project_id_key UNIQUE (project_id);
-- The referent of the composite key below. `target` is already the primary key,
-- so this is unique trivially; it exists so the pair can be referenced.
ALTER TABLE targets ADD CONSTRAINT targets_target_project_key UNIQUE (target, project_id);

-- Inferred, never guessed (owner, 14 Oct). A target whose mappings all name
-- one project gets that project. One whose mappings span several stops the
-- migration: picking one would silently drop every other project's roles from
-- what the target grants.
DO $$
DECLARE
    spans  TEXT;
    shared TEXT;
BEGIN
    SELECT string_agg(target || ' (' || projects || ')', '; ' ORDER BY target)
      INTO spans
      FROM (SELECT target, string_agg(DISTINCT project_id, ', ' ORDER BY project_id) AS projects
              FROM target_role_mappings
             GROUP BY target
            HAVING count(DISTINCT project_id) > 1) s;
    IF spans IS NOT NULL THEN
        RAISE EXCEPTION 'targets whose mappings span several Zitadel projects: %', spans
            USING HINT = 'Each target serves one project now. Delete the mappings that belong to the other projects, then run the migration again.';
    END IF;

    SELECT string_agg(project_id || ' (' || targets || ')', '; ' ORDER BY project_id)
      INTO shared
      FROM (SELECT project_id, string_agg(DISTINCT target, ', ' ORDER BY target) AS targets
              FROM target_role_mappings
             GROUP BY project_id
            HAVING count(DISTINCT target) > 1) s;
    IF shared IS NOT NULL THEN
        RAISE EXCEPTION 'projects mapped on several targets: %', shared
            USING HINT = 'A project serves one system now. Delete one target''s mappings for that project, then run the migration again.';
    END IF;
END $$;

UPDATE targets t
   SET project_id = m.project_id
  FROM (SELECT DISTINCT target, project_id FROM target_role_mappings) m
 WHERE t.target = m.target;

-- The rule itself. ON UPDATE RESTRICT is what makes "change the project while
-- mappings exist" a refusal (409 MAPPINGS_EXIST) instead of a silent re-scope
-- in which every mapping's role matches nothing.
ALTER TABLE target_role_mappings
    ADD CONSTRAINT target_role_mappings_target_project_fkey
    FOREIGN KEY (target, project_id) REFERENCES targets (target, project_id)
    ON UPDATE RESTRICT ON DELETE RESTRICT;
