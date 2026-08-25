// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderHook, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { useCreateHold } from "@/lib/queries/useHolds";
import { usersQueryKeys } from "@/lib/queries/useUsers";

vi.mock("@/lib/api-client", () => ({
  request: vi.fn(async () => ({ id: "a1", subject_id: "u1", target: "truenas" })),
}));

/**
 * §17 — the key an invalidation issues has to match the key a query uses.
 *
 * Authoring a hold invalidated `["users", subject_id]`. The access view is
 * keyed `["users", "access", id]`, and prefix matching compares position by
 * position — so the two never met and the Withheld band did not appear on the
 * page that had just authored the hold. Lifting one used a bare `["users"]` and
 * did refresh, which is what made it read as a convention rather than a bug.
 *
 * Asserted against the real cache rather than against a literal, because a
 * literal repeated in a test is the same mistake written twice.
 */
describe("authoring a hold refreshes what it changed", () => {
  it("invalidates the access view the Withheld band is rendered from", async () => {
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    client.setQueryData(usersQueryKeys.access("u1"), { access: [] });
    client.setQueryData(["users", "u1", "allowances"], { allowances: [] });

    const { result } = renderHook(() => useCreateHold(), {
      wrapper: ({ children }) => (
        <QueryClientProvider client={client}>{children}</QueryClientProvider>
      ),
    });

    result.current.mutate({
      subjectId: "u1",
      target: "truenas",
      field: "enabled",
      value: "true",
      reason: "safety review",
      reviewDate: "2026-12-01T00:00:00Z",
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(client.getQueryState(usersQueryKeys.access("u1"))?.isInvalidated).toBe(true);
    // And the hold list, which IS keyed under the subject — the one thing the
    // old key happened to match.
    expect(client.getQueryState(["users", "u1", "allowances"])?.isInvalidated).toBe(true);
  });
});
