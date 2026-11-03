-- claims-delivery-integrity S2: how old a last-known-good answer may be when
-- Redis is out, per project, beside claim_failure_mode. NULL is the data
-- plane's default (6h). "Serve what we knew an hour ago" is a policy choice
-- about revocations, so it is the project's to make.
ALTER TABLE claim_profiles
    ADD COLUMN IF NOT EXISTS claim_lkg_max_age_seconds INTEGER
        CONSTRAINT ck_claim_profiles_lkg_max_age
            CHECK (claim_lkg_max_age_seconds IS NULL OR claim_lkg_max_age_seconds > 0);
