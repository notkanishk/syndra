import { readFileSync } from "node:fs";
import { resolve } from "node:path";

import type { EntitlementField } from "@/lib/fields";

/**
 * What the TrueNAS add-on declares. Read from the contract fixture
 * `addons/contract/v2/fixtures/manifest_truenas.json`, the one file the add-on's
 * Go test (`GET /capabilities`) keeps equal to `addons/truenas/capabilities.go`,
 * so there is no hand copy here to drift.
 *
 * Read at test time, not imported: an import made the file part of the
 * program `next build` type-checks, and the UI image builds from `ui/` alone,
 * where `addons/` does not exist (TS2307). Tests run from the repo, which has
 * it, the way the audit-vocabulary coverage test reads `backend/`.
 */
const manifest = JSON.parse(
  readFileSync(resolve(process.cwd(), "..", "addons", "contract", "v2", "fixtures", "manifest_truenas.json"), "utf8"),
) as { entitlement_schema: EntitlementField[] };

export const TRUENAS_SCHEMA = manifest.entitlement_schema;
