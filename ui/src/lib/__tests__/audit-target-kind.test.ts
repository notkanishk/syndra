import { describe, expect, it } from "vitest";

import { targetKind } from "@/lib/audit-vocabulary";

/**
 * `bundle.role_added`/`role_removed` record the bundle's own id in
 * `target_id` (see db.CascadeAudit) rather than a person's — a raw uuid
 * handed to the person resolver rendered as "Unknown account <uuid>" on the
 * audit page. `targetKind` is what routes the id to the right resolver
 * instead.
 */
describe("targetKind", () => {
  it("names the bundle for role_added and role_removed", () => {
    expect(targetKind("bundle.role_added")).toBe("bundle");
    expect(targetKind("bundle.role_removed")).toBe("bundle");
  });

  it("still treats other bundle actions as naming a person", () => {
    expect(targetKind("bundle.assigned")).toBe("user");
    expect(targetKind("bundle.unassigned")).toBe("user");
    expect(targetKind("bundle.holder_moved")).toBe("user");
  });

  it("names a rule for the mapping_rule family", () => {
    expect(targetKind("mapping_rule.created")).toBe("rule");
    expect(targetKind("mapping_rule.updated")).toBe("rule");
    expect(targetKind("mapping_rule.deleted")).toBe("rule");
  });

  it("defaults to a person for everything else", () => {
    expect(targetKind("direct_grant.upserted")).toBe("user");
    expect(targetKind("access_request.approved")).toBe("user");
  });
});
