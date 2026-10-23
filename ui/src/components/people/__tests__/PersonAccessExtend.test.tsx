// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { PersonAccess } from "@/components/people/PersonAccess";

/**
 * A-07 §19: a row on a person's page that ends within 14 days offers Extend, the same
 * as Expiring and Home. Other rows don't. Giving access is no longer the way to an end date.
 */

const state = vi.hoisted(() => ({
  endsInDays: null as number | null,
  isOperator: true,
  extend: vi.fn(),
}));

vi.mock("@/lib/ui-view", () => ({
  useIsAdvanced: () => false,
  useUiView: () => ({ revealInAdvanced: vi.fn() }),
}));

vi.mock("@/lib/queries/useUpstream", () => ({
  useUpstreamUserGrants: () => ({
    data: { items: [{ id: "zg1", projectId: "p-audio", roleKeys: ["community"] }], total: 1 },
    isLoading: false,
    error: null,
  }),
}));

vi.mock("@/lib/queries/useUsers", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/lib/queries/useUsers")>()),
  useUserAccess: () => ({
    data: {
      user: { id: "u1", name: "Shikha Yadav", email: "s@example.edu", status: "active", avatar: "SY" },
      bundles: [],
      projects: [
        {
          project_id: "p-audio",
          project_name: "Audio-Dash",
          source_roles: [
            {
              project_id: "p-audio",
              project_name: "Audio-Dash",
              role_key: "community",
              is_source: true,
              reasons: [{ kind: "direct", description: "" }],
            },
          ],
          derived_roles: [],
          effective_role_keys: ["community"],
        },
      ],
      allowances: [],
      cleanup_hints: [],
    },
    isLoading: false,
    error: null,
    refetch: () => {},
  }),
  useUserGrants: () => ({
    data: [
      {
        id: "g1",
        project_id: "p-audio",
        role_key: "community",
        granted_by: "priya",
        expires_at:
          state.endsInDays === null ? null : new Date(Date.now() + state.endsInDays * 86_400_000).toISOString(),
      },
    ],
    isLoading: false,
    error: null,
  }),
  useCreateGrant: () => ({ mutateAsync: state.extend, isPending: false }),
}));

vi.mock("@/lib/queries/useAudit", () => ({
  useAuditEntries: () => ({ data: [], isLoading: false, error: null, refetch: () => {} }),
}));
vi.mock("@/lib/queries/useRequests", () => ({
  useRequestsAdmin: () => ({ data: [], isLoading: false, error: null, refetch: () => {} }),
  useDecideRequest: () => ({ isPending: false, mutateAsync: vi.fn() }),
}));

function renderPerson() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <PersonAccess userId="u1" isOperator={state.isOperator} />
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  state.endsInDays = null;
  state.isOperator = true;
  state.extend = vi.fn().mockResolvedValue({ outbox_id: "ob1", status: "applied" });
});

describe("a person's row that ends soon", () => {
  it("offers Extend when it ends within 14 days", () => {
    state.endsInDays = 5;
    renderPerson();
    expect(screen.getByText(/^Ends /)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /^Extend/ })).toBeInTheDocument();
  });

  it("offers Extend on the last day itself", () => {
    state.endsInDays = 14;
    renderPerson();
    expect(screen.getByRole("button", { name: /^Extend/ })).toBeInTheDocument();
  });

  it("does not offer it for a row that ends later, or never", () => {
    state.endsInDays = 40;
    const { unmount } = renderPerson();
    expect(screen.queryByRole("button", { name: /^Extend/ })).toBeNull();
    unmount();

    state.endsInDays = null;
    renderPerson();
    expect(screen.queryByRole("button", { name: /^Extend/ })).toBeNull();
  });

  it("is an operator's act: a member reading their own page is not offered it", () => {
    state.endsInDays = 5;
    state.isOperator = false;
    renderPerson();
    expect(screen.queryByRole("button", { name: /^Extend/ })).toBeNull();
  });

  it("extends the same way Expiring does, and says what happened in the row's seat", async () => {
    state.endsInDays = 5;
    renderPerson();
    fireEvent.click(screen.getByRole("button", { name: /^Extend/ }));
    await waitFor(() =>
      expect(state.extend).toHaveBeenCalledWith({
        project_id: "p-audio",
        role_key: "community",
        reason: "Extended from the person's page",
        duration_days: 90,
        extend: true,
      }),
    );
    expect(await screen.findByText("Extended by 90 days")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /^Extend/ })).toBeNull();
  });

  it("says it will wait when the change queues", async () => {
    state.endsInDays = 5;
    state.extend = vi.fn().mockResolvedValue({ outbox_id: "ob1", status: "pending" });
    renderPerson();
    fireEvent.click(screen.getByRole("button", { name: /^Extend/ }));
    expect(await screen.findByText("Will extend by 90 days once sent")).toBeInTheDocument();
  });
});
