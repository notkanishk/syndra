import { describe, expect, it } from "vitest";

import { traceFor } from "@/lib/audit-vocabulary";
import type { AuditEntry } from "@/lib/queries/useAudit";

const CASCADE = "1f2e3d4c-5b6a-4798-8a9b-0c1d2e3f4a5b";
const RULE = "9a8b7c6d-5e4f-4321-8765-0f1e2d3c4b5a";

function entry(over: Partial<AuditEntry> = {}): AuditEntry {
  return {
    id: "a-1",
    actor_id: "u-1",
    target_id: "-",
    action: "mapping_rule.updated",
    resource_id: RULE,
    created_at: "2026-08-01T10:00:00Z",
    ...over,
  };
}

describe("traceFor", () => {
  // The defect this replaced: the column rendered the *rule* id with a `c_` prefix and linked to
  // an unfiltered change history. Both halves have to be true now — the id is the cascade's, and
  // the link goes to that cascade.
  it("names the cascade and links to it, once the backend records one", () => {
    const trace = traceFor(entry({ cascade_id: CASCADE }));
    expect(trace).toEqual({
      kind: "cascade",
      label: "c_1f2e",
      href: `/operations/cascades?cascade=${CASCADE}`,
    });
  });

  it("does not borrow the resource id when a cascade id is present", () => {
    const trace = traceFor(entry({ cascade_id: CASCADE }));
    // Would be R_9a8b if it fell through to the object branch.
    expect(trace.kind === "cascade" && trace.label).toBe("c_1f2e");
  });

  // Rows written before the column existed. They still know which rule the change was about;
  // they cannot know what it did downstream, so they say the first and not the second.
  it("falls back to the object, labelled as the object, with no link", () => {
    expect(traceFor(entry())).toEqual({
      kind: "object",
      label: "R_9a8b",
      title: "Rule",
    });
  });

  it("distinguishes a bundle from a rule", () => {
    const trace = traceFor(entry({ action: "bundle.unassigned", resource_id: RULE }));
    expect(trace.kind === "object" && trace.label).toBe("b_9a8b");
  });

  // `bundle.role_added` records `project/role`, not an id. Four characters of that is a label
  // that looks like a handle and refers to nothing.
  it("says nothing when the resource is not an identifier", () => {
    expect(traceFor(entry({ action: "bundle.role_added", resource_id: "laser-lab/trained" })))
      .toEqual({ kind: "none" });
  });

  // A direct grant's removal commits through the same atomic function a cascade does, but its
  // writes carry source='direct' and Change history excludes those — so the backend leaves its
  // audit row unstamped, and this column must not invent a trace from the resource either. Its
  // resource is `project/role`, which is what makes that automatic.
  it("leaves a direct grant's removal untraced", () => {
    expect(
      traceFor(entry({ action: "direct_grant.removed", resource_id: "pLaser/trained" })),
    ).toEqual({ kind: "none" });
  });

  it("says nothing for actions that never cascade", () => {
    expect(traceFor(entry({ action: "access_request.created", resource_id: RULE }))).toEqual({
      kind: "none",
    });
  });

  // A published version records `<uuid>@v3`. The version is the object; the id is still an id.
  it("reads the id out of a versioned resource", () => {
    const trace = traceFor(
      entry({ action: "bundle.version_published", resource_id: `${RULE}@v3` }),
    );
    expect(trace.kind === "object" && trace.label).toBe("b_9a8b");
  });

  it("says nothing when there is no resource at all", () => {
    expect(traceFor(entry({ resource_id: "" }))).toEqual({ kind: "none" });
  });
});
