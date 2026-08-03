import { describe, expect, it } from "vitest";

import { draftChangeCount, type BundleDraft } from "@/lib/queries/useBundleVersions";

/**
 * The function that took Review › Bundles down in production.
 *
 * It read `draft?.added.length ?? 0`. Optional chaining guards the PARENT: when `draft` was absent
 * the whole chain short-circuited and `?? 0` did its job, so the code looked defensive. When
 * `draft` was present and `added` was `null` — which the backend sent for every bundle whose
 * working copy matched its published version — `.length` threw before `?? 0` could run, and the
 * page rendered its error boundary.
 *
 * It had no test at all: both suites that touch it mock it out (`draftChangeCount: () => 0`), which
 * is why the whole path was green. These call the real thing.
 */
function draft(over: Partial<BundleDraft> = {}): BundleDraft {
  return {
    bundle_id: "b1",
    latest_version: 2,
    next_version: 3,
    added: [],
    removed: [],
    holder_count: 4,
    ...over,
  };
}

const role = { bundle_id: "b1", zitadel_project_id: "pLaser", zitadel_role_key: "trained" };

describe("draftChangeCount", () => {
  it("counts additions and removals together", () => {
    expect(draftChangeCount(draft({ added: [role], removed: [role] }))).toBe(2);
  });

  it("is zero for a draft with nothing unpublished", () => {
    expect(draftChangeCount(draft())).toBe(0);
  });

  it("is zero when there is no draft yet", () => {
    expect(draftChangeCount(undefined)).toBe(0);
  });

  // The outage. A nil Go slice arrives as null, and the type said it could not.
  it("survives null arrays rather than throwing", () => {
    const nulled = draft() as unknown as Record<string, unknown>;
    nulled.added = null;
    nulled.removed = null;
    expect(draftChangeCount(nulled as unknown as BundleDraft)).toBe(0);
  });

  // The asymmetric case, which is what an unpublished bundle actually looks like: everything is an
  // addition and `removed` is null. Reading the count must not depend on which one is populated.
  it("counts one side when the other is null", () => {
    const half = draft({ added: [role] }) as unknown as Record<string, unknown>;
    half.removed = null;
    expect(draftChangeCount(half as unknown as BundleDraft)).toBe(1);
  });
});
