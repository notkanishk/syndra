import { readFileSync } from "node:fs";
import { resolve } from "node:path";

import { describe, expect, it } from "vitest";

import { BULK_MAX_USERS } from "@/lib/queries/useBulkGrants";

/**
 * One number, written twice, in two languages.
 *
 * `BULK_MAX_USERS` is what four selection bars promise an operator before they
 * tap; `services.BulkMaxUsers` is what three handlers refuse on afterwards.
 * Nothing connected them. Drop the Go constant to 250 and the bars keep
 * promising 500, stop refusing at the right point, and operators are back to
 * meeting the 4xx after the tap — which is the exact failure the ceiling work
 * exists to prevent, restored silently by an edit in another language.
 *
 * A comment in the expiry queue's test used to claim this check existed. It
 * did not. This is that check.
 */
const BACKEND_CONSTANT = resolve(__dirname, "../../../../../backend/internal/services/bulk.go");

describe("the bulk ceiling is one number", () => {
  it("matches the constant the handlers actually refuse on", () => {
    let source: string;
    try {
      source = readFileSync(BACKEND_CONSTANT, "utf8");
    } catch {
      // Loud rather than skipped. A guard that quietly passes when it cannot
      // find what it is guarding is the thing this test was written to
      // replace. If `ui/` is ever built without the backend tree beside it,
      // this needs a generated constant rather than a silent pass.
      throw new Error(
        `Could not read ${BACKEND_CONSTANT}. The frontend ceiling is only verifiable against the backend source; a generated constant is needed if these trees are separated.`,
      );
    }

    const declared = /^const BulkMaxUsers = (\d+)$/m.exec(source);
    expect(declared, "services.BulkMaxUsers must be a plain literal this can read").not.toBeNull();
    expect(Number(declared![1]), "BULK_MAX_USERS must equal services.BulkMaxUsers").toBe(
      BULK_MAX_USERS,
    );
  });
});
