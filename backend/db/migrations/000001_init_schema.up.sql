CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- 1. Bundles (Aggregations of Raw Zitadel Roles)
CREATE TABLE IF NOT EXISTS bundles (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) UNIQUE NOT NULL,
    description TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS bundle_roles (
    bundle_id UUID NOT NULL REFERENCES bundles(id) ON DELETE CASCADE,
    zitadel_project_id VARCHAR(255) NOT NULL,
    zitadel_role_key VARCHAR(255) NOT NULL,
    PRIMARY KEY (bundle_id, zitadel_project_id, zitadel_role_key)
);

-- 2. Logic Mapping Rules (Automated Hierarchies)
CREATE TABLE IF NOT EXISTS mapping_rules (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    source_zitadel_project_id VARCHAR(255) NOT NULL,
    source_zitadel_role_key VARCHAR(255) NOT NULL,
    target_zitadel_project_id VARCHAR(255) NOT NULL,
    target_zitadel_role_key VARCHAR(255) NOT NULL,
    version INT NOT NULL DEFAULT 1,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Ensuring no pure duplicates of the same exact rule logic exist
CREATE UNIQUE INDEX idx_mapping_rules_logic ON mapping_rules(
    source_zitadel_project_id, source_zitadel_role_key, target_zitadel_project_id, target_zitadel_role_key
);

-- 3. Claim Profiles (Application Token Formatting)
CREATE TABLE IF NOT EXISTS claim_profiles (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    zitadel_project_id VARCHAR(255) UNIQUE NOT NULL,
    claim_name VARCHAR(128) NOT NULL,
    format_type VARCHAR(64) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- 4. Audit & Governance (The Log)
CREATE TABLE IF NOT EXISTS audit_logs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    actor_zitadel_user_id VARCHAR(255) NOT NULL,
    target_zitadel_user_id VARCHAR(255) NOT NULL,
    action VARCHAR(128) NOT NULL,
    resource_id VARCHAR(255),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);
