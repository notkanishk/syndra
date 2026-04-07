-- 6. Mutable direct grants with optional expiry
CREATE TABLE IF NOT EXISTS direct_role_grants (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id VARCHAR(255) NOT NULL,
    zitadel_project_id VARCHAR(255) NOT NULL,
    zitadel_role_key VARCHAR(255) NOT NULL,
    granted_by VARCHAR(255) NOT NULL,
    reason TEXT,
    expires_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (user_id, zitadel_project_id, zitadel_role_key)
);

CREATE INDEX idx_direct_role_grants_user ON direct_role_grants(user_id);
CREATE INDEX idx_direct_role_grants_expiry ON direct_role_grants(expires_at);

-- 7. Self-service permission requests
CREATE TABLE IF NOT EXISTS access_requests (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    requester_user_id VARCHAR(255) NOT NULL,
    zitadel_project_id VARCHAR(255) NOT NULL,
    zitadel_role_key VARCHAR(255) NOT NULL,
    justification TEXT NOT NULL,
    duration_days INT,
    status VARCHAR(32) NOT NULL DEFAULT 'pending',
    reviewer_user_id VARCHAR(255),
    review_note TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    resolved_at TIMESTAMP WITH TIME ZONE
);

CREATE INDEX idx_access_requests_status ON access_requests(status);
