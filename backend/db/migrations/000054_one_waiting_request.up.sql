-- One waiting request per person and role (found on production, 15 Oct 2026).
--
-- A member could send the same request again and again from one dialog, and
-- the queue showed every copy. Approving each copy would have queued the
-- same grant once per copy, and written its end date over the access the
-- person already had.
--
-- A request is for a role someone lacks, one per role (owner, 15 Oct). So,
-- before the index: requests waiting for a role the person already has are
-- closed, then copies of the same request, the earliest of each set kept.
-- Both are declined by `system` with a note saying why ("withdrawn" means the
-- member took it back, and they didn't), and each closure is audited.
WITH held AS (
    UPDATE access_requests a
       SET status = 'rejected',
           reviewer_user_id = 'system',
           review_note = 'They already have this, so there was nothing to approve.',
           resolved_at = now()
     WHERE a.status = 'pending'
       AND (EXISTS (SELECT 1 FROM direct_role_grants g
                     WHERE g.user_id = a.requester_user_id
                       AND g.zitadel_project_id = a.zitadel_project_id
                       AND g.zitadel_role_key = a.zitadel_role_key)
            OR EXISTS (SELECT 1 FROM zitadel_grants_index z
                        WHERE z.user_id = a.requester_user_id
                          AND z.project_id = a.zitadel_project_id
                          AND a.zitadel_role_key = ANY (z.role_keys)))
    RETURNING a.id, a.requester_user_id
)
INSERT INTO audit_logs (actor_zitadel_user_id, target_zitadel_user_id, action, resource_id)
SELECT 'system', requester_user_id, 'access_request.closed_as_held', id::text FROM held;

WITH ranked AS (
    SELECT id, requester_user_id,
           row_number() OVER (
               PARTITION BY requester_user_id, zitadel_project_id, zitadel_role_key
               ORDER BY created_at, id
           ) AS copy
      FROM access_requests
     WHERE status = 'pending'
), closed AS (
    UPDATE access_requests a
       SET status = 'rejected',
           reviewer_user_id = 'system',
           review_note = 'A copy of an earlier request that is still waiting.',
           resolved_at = now()
      FROM ranked r
     WHERE a.id = r.id AND r.copy > 1
    RETURNING a.id, a.requester_user_id
)
INSERT INTO audit_logs (actor_zitadel_user_id, target_zitadel_user_id, action, resource_id)
SELECT 'system', requester_user_id, 'access_request.closed_as_copy', id::text FROM closed;

CREATE UNIQUE INDEX IF NOT EXISTS idx_access_requests_one_waiting
    ON access_requests (requester_user_id, zitadel_project_id, zitadel_role_key)
    WHERE status = 'pending';
