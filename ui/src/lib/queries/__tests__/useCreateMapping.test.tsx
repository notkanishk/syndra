// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { useCreateMapping } from "@/lib/queries/useMappings";

const sent: Array<{ path: string; body: Record<string, unknown> }> = [];

vi.mock("@/lib/api-client", () => ({
  request: vi.fn(async (path: string, init?: { body?: Record<string, unknown> }) => {
    sent.push({ path, body: init?.body ?? {} });
    return { mapping: { id: "m1", target: "truenas" }, queued_convergences: 3 };
  }),
  ApiError: class extends Error {},
}));

/**
 * Creating a mapping stopped being a bare write.
 *
 * It cites an approval whenever the role has holders — entitlements are derived
 * from mappings, so the row alone changes what every holder is entitled to —
 * and it answers with the row plus how many people it queued. The client stayed
 * on the old contract: no citation, and a response declared as a bare mapping.
 *
 * Nothing called it, so nothing broke. But a form built on it would have met a
 * deterministic refusal on any held role, and read the wrong shape back on a
 * role nobody holds.
 */

function harness() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return renderHook(() => useCreateMapping(), {
    wrapper: ({ children }) => (
      <QueryClientProvider client={client}>{children}</QueryClientProvider>
    ),
  });
}

const mapping = {
  target: "truenas",
  projectId: "pLab",
  roleKey: "maker",
  field: "group",
  value: "lab_makers",
};

beforeEach(() => {
  sent.length = 0;
});

describe("creating a mapping", () => {
  it("cites the approval the rehearsal issued", async () => {
    const { result } = harness();
    result.current.mutate({ ...mapping, planId: "plan_61a8" });

    await waitFor(() => expect(sent.length).toBe(1));
    expect(sent[0].body).toMatchObject({
      target: "truenas",
      project_id: "pLab",
      role_key: "maker",
      field: "group",
      value: "lab_makers",
      plan_id: "plan_61a8",
    });
  });

  // A mapping on a role nobody holds is a definition, and there is nothing to
  // review. The key is omitted rather than sent empty: an empty citation is a
  // citation the backend has to decide how to read.
  it("sends no citation at all when there is none", async () => {
    const { result } = harness();
    result.current.mutate(mapping);

    await waitFor(() => expect(sent.length).toBe(1));
    expect(sent[0].body).not.toHaveProperty("plan_id");
  });

  it("reads the row and the count out of the response", async () => {
    const { result } = harness();
    result.current.mutate({ ...mapping, planId: "plan_61a8" });

    await waitFor(() => expect(result.current.data).toBeDefined());
    expect(result.current.data?.mapping.id).toBe("m1");
    // Queued, never applied — the drain is what moves them.
    expect(result.current.data?.queued_convergences).toBe(3);
  });
});
