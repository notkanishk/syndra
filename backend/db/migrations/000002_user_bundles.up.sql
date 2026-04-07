-- 5. User-Bundle Association (Role Bundles assigned to users)
CREATE TABLE IF NOT EXISTS user_bundle_assignments (
    user_id VARCHAR(255) NOT NULL,
    bundle_id UUID NOT NULL REFERENCES bundles(id) ON DELETE CASCADE,
    assigned_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, bundle_id)
);

CREATE INDEX idx_user_bundles_user ON user_bundle_assignments(user_id);
