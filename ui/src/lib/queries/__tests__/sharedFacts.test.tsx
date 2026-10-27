// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";

vi.mock("@/lib/api-client", () => ({ request: vi.fn(async () => ({})) }));

import { useUpdateBundle } from "@/lib/queries/useBundles";
import { useLiftHold } from "@/lib/queries/useHolds";
import { useCreateRole } from "@/lib/queries/useRoles";
import { useAssignBundle } from "@/lib/queries/useUsers";

/**
 * One fact, one answer: a write refreshes every cache that shows its result.
 * A cache left out keeps showing the old answer on another screen, which is
 * two screens disagreeing about the same thing.
 */
async function invalidatedBy<T>(use: () => { mutateAsync: (v: T) => Promise<unknown> }, variables: T) {
  const client = new QueryClient();
  const spy = vi.spyOn(client, "invalidateQueries");
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={client}>{children}</QueryClientProvider>
  );
  const { result } = renderHook(use, { wrapper });
  await result.current.mutateAsync(variables);
  await waitFor(() => expect(spy).toHaveBeenCalled());
  return spy.mock.calls.map(([filters]) => JSON.stringify(filters?.queryKey));
}

describe("what a write refreshes", () => {
  it("assigning a bundle refreshes the bundles, which count its holders", async () => {
    const keys = await invalidatedBy(() => useAssignBundle("u1"), "b1");
    expect(keys).toContain(JSON.stringify(["bundles"]));
  });

  it("creating a role refreshes the project cards and the names screens resolve", async () => {
    const keys = await invalidatedBy(() => useCreateRole(), {
      project_id: "p1",
      role_key: "trained",
      display_name: "Trained",
    } as never);
    expect(keys).toContain(JSON.stringify(["projects"]));
    expect(keys).toContain(JSON.stringify(["name-catalog"]));
  });

  it("renaming a bundle refreshes the resolver's own copy of its name", async () => {
    const keys = await invalidatedBy(() => useUpdateBundle(), { id: "b1", name: "Lab Tech" } as never);
    expect(keys).toContain(JSON.stringify(["name-bundles"]));
  });

  it("lifting a hold refreshes the whole governance prefix, the rail's indicators included", async () => {
    const keys = await invalidatedBy(() => useLiftHold(), "h1");
    expect(keys).toContain(JSON.stringify(["governance"]));
  });
});
