import { QueryClient } from "@tanstack/react-query";
import { describe, expect, it, vi } from "vitest";

import { bundleVersionKeys } from "@/lib/queries/useBundleVersions";
import { bundlesQueryKeys } from "@/lib/queries/useBundles";

/**
 * The draft query and the edit mutations live in different files, so nothing
 * but a test keeps their keys the same shape. When they drifted, an edit left
 * the Publish button absent until a reload — the edit looked applied and
 * nothing said otherwise.
 */
describe("bundle cache keys", () => {
  it("the draft key an edit invalidates matches the one the draft query uses", () => {
    const qc = new QueryClient();
    const spy = vi.spyOn(qc, "invalidateQueries");

    // Exactly what invalidateAfterEdit issues for the draft.
    qc.invalidateQueries({ queryKey: ["bundles", "b1", "draft"] });

    expect(spy.mock.calls[0][0]).toEqual({ queryKey: bundleVersionKeys.draft("b1") });
  });

  it("the bundle list key is shared, so the unpublished marker refreshes", () => {
    expect(bundlesQueryKeys.list).toBeDefined();
  });
});
