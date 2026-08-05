// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderHook, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

vi.mock("@/lib/api-client", () => ({ request: vi.fn().mockResolvedValue({}) }));

import { useCreateGrant } from "@/lib/queries/useUsers";
import { useApplyBulk } from "@/lib/queries/useBulkGrants";

/**
 * Which caches a grant write has to drop.
 *
 * `POST /users/{id}/grants` upserts on (user, project, role), so it is BOTH "grant access" and
 * "extend access" — Review › Expiring access uses it for the second. That screen is built entirely
 * from expiry dates, so an extension that does not invalidate it leaves the row on screen with its
 * old date, and if the row had been acknowledged, still showing an acknowledgement the new date has
 * already voided. The console would be contradicting the backend about who keeps access.
 */
function harness() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  const invalidated: unknown[] = [];
  const original = client.invalidateQueries.bind(client);
  client.invalidateQueries = ((filters?: { queryKey?: unknown }) => {
    if (filters?.queryKey) invalidated.push(filters.queryKey);
    return original(filters as Parameters<typeof original>[0]);
  }) as typeof client.invalidateQueries;

  const wrapper = ({ children }: { children: React.ReactNode }) => (
    <QueryClientProvider client={client}>{children}</QueryClientProvider>
  );
  return { wrapper, invalidated };
}

function keyRoots(invalidated: unknown[]): string[] {
  return invalidated.map((key) => (Array.isArray(key) ? String(key[0]) : String(key)));
}

describe("grant writes invalidate the expiring-access queue", () => {
  it("useCreateGrant drops the review queue as well as the person", async () => {
    const { wrapper, invalidated } = harness();
    const { result } = renderHook(() => useCreateGrant("u1"), { wrapper });

    await result.current.mutateAsync({
      project_id: "pLaser",
      role_key: "trained",
      reason: "Extended from Expiring access",
      duration_days: 90,
    });

    await waitFor(() => expect(invalidated.length).toBeGreaterThan(0));
    const roots = keyRoots(invalidated);
    expect(roots).toContain("review");
    // Home counts what is expiring, so it is wrong by one until this lands too.
    expect(roots).toContain("governance");
    // And still the caches it always dropped.
    expect(roots).toContain("users");
  });

  // Same staleness one caller over: bulk `extend` rewrites the same dates, and this hook's key
  // list had every root except the one the screen it is launched from reads.
  it("useApplyBulk drops the review queue too", async () => {
    const { wrapper, invalidated } = harness();
    const { result } = renderHook(() => useApplyBulk(), { wrapper });

    await result.current.mutateAsync({
      op: "extend",
      user_ids: ["u1"],
      reason: "Extended from Expiring access",
      duration_days: 90,
      grant_ids: ["g1"],
    });

    await waitFor(() => expect(invalidated.length).toBeGreaterThan(0));
    expect(keyRoots(invalidated)).toContain("review");
  });
});
