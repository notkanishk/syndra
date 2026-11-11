import manifest from "../../../../addons/contract/v2/fixtures/manifest_truenas.json";

import type { EntitlementField } from "@/lib/fields";

/**
 * What the TrueNAS add-on declares. Read from the contract fixture
 * `addons/contract/v2/fixtures/manifest_truenas.json`, the one file the add-on's
 * Go test (`GET /capabilities`) keeps equal to `addons/truenas/capabilities.go`,
 * so there is no hand copy here to drift.
 */
export const TRUENAS_SCHEMA = manifest.entitlement_schema as EntitlementField[];
