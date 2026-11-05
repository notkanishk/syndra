// @vitest-environment jsdom
import { QueryClient, QueryClientProvider, useMutation } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { GrantDirectAccess } from "@/components/people/GrantDirectAccess";
import { FORECAST } from "@/lib/outcome";
import type { Indicators } from "@/lib/queries/useIndicators";

/**
 * A-20 P8: Give forecasts in the first frame. A role that waits (the role
 * catalogue's `waits_for_send`, the same mark a plan row reads) forecasts
 * "Will wait for you to send"; any other forecasts "Will be given". Only the
 * true word follows, and the rail's count moves when the answer lands.
 */
const wire = vi.hoisted(() => ({
  settle: null as null | { resolve: (r: unknown) => void; reject: (e: unknown) => void },
}));

vi.mock("@/components/states/DegradedBanner", () => ({ useDegraded: () => false }));
vi.mock("@/lib/queries/useTerms", () => ({ useTerms: () => ({ data: undefined }) }));
// No automatic rule follows these roles (the preview has answered).
vi.mock("@/lib/queries/useRulePreview", () => ({
  useRulePreview: () => ({ data: [], isLoading: false, isError: false }),
}));
vi.mock("@/lib/queries/useProjects", () => ({
  useProjects: () => ({ data: [{ project: { id: "p1", name: "Printing Lab" } }] }),
}));
vi.mock("@/lib/queries/useRoles", () => ({
  useGlobalRoleCatalog: () => ({
    data: [
      { project_id: "p1", role_key: "operator", project_name: "Printing Lab", display_name: "Operator", observed_user_count: 1 },
      { project_id: "p1", role_key: "supervisor", project_name: "Printing Lab", display_name: "Supervisor", observed_user_count: 1, waits_for_send: true },
    ],
  }),
}));
vi.mock("@/lib/queries/useUsers", async (original) => ({
  ...(await original<typeof import("@/lib/queries/useUsers")>()),
  useCreateGrant: () =>
    useMutation({
      mutationFn: () => new Promise((resolve, reject) => (wire.settle = { resolve, reject })),
      onSuccess: async () => {
        const { settleIndicators } = await import("@/lib/queries/useIndicators");
        settleIndicators(client, { pending_propagation: 1 });
      },
    }),
  useUserGrants: () => ({ data: [] }),
  useUserAccess: () => ({ data: { projects: [] } }),
}));

const rail: Indicators = {
  pending_requests: 0,
  expiring_grants: 0,
  pending_propagation: 2,
  drift: 0,
  unconfirmed_revocations: 0,
  holds_due: 0,
  revocations_escalated: false,
  zitadel_reachable: true,
};
let client: QueryClient;
const pendingNow = () => client.getQueryData<Indicators>(["governance", "indicators"])!.pending_propagation;

beforeEach(() => {
  wire.settle = null;
  client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  client.setQueryData(["governance", "indicators"], rail);
});

function give(role: string) {
  render(
    <QueryClientProvider client={client}>
      <GrantDirectAccess userId="u1" userName="Ada Lovelace" open onClose={vi.fn()} />
    </QueryClientProvider>,
  );
  const dialog = screen.getByRole("dialog");
  fireEvent.click(within(dialog).getByRole("button", { name: /Printing Lab/ }));
  fireEvent.click(within(dialog).getByRole("radio", { name: new RegExp(role) }));
  fireEvent.change(screen.getByPlaceholderText(/Capstone build/), { target: { value: "test" } });
  fireEvent.click(screen.getByRole("button", { name: "Give role" }));
}

describe("Give: the first 100ms", () => {
  it("says 'Will be given' for an ordinary role, and nothing on the rail moves ahead of the answer", async () => {
    give("Operator");
    expect(await screen.findByText(FORECAST.give)).toBeInTheDocument();
    expect(pendingNow()).toBe(2);
    wire.settle!.resolve({ outbox_id: "o1", status: "pending", wait_reason: "manual" });
    await screen.findByRole("button", { name: "Close" });
    expect(screen.queryByText(FORECAST.give)).toBeNull();
    await waitFor(() => expect(pendingNow()).toBe(3));
  });

  it("says 'Will wait for you to send' for a role that waits, never 'Will be given'", async () => {
    give("Supervisor");
    expect(await screen.findByText(FORECAST.queued)).toBeInTheDocument();
    expect(screen.queryByText(FORECAST.give)).toBeNull();
    wire.settle!.resolve({ outbox_id: "o1", status: "pending", wait_reason: "role_waits" });
    await screen.findByRole("button", { name: "Close" });
  });

  it("an error is the honest error: the forecast goes, the rail does not move", async () => {
    give("Operator");
    await screen.findByText(FORECAST.give);
    wire.settle!.reject(new Error("boom"));
    await waitFor(() => expect(screen.getByText(/Nothing was changed/i)).toBeInTheDocument());
    expect(screen.queryByText(FORECAST.give)).toBeNull();
    expect(pendingNow()).toBe(2);
  });
});
