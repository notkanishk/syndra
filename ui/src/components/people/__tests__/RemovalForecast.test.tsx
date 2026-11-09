// @vitest-environment jsdom
import { QueryClient, QueryClientProvider, useMutation } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { RoleReason } from "@/components/access/AccessSource";
import { RemovalDialog, type Removal } from "@/components/people/RemovalDialog";
import { FORECAST } from "@/lib/outcome";
import type { Indicators } from "@/lib/queries/useIndicators";
import type { ActPlan } from "@/lib/usePlanConsent";
import { actPlan } from "@/test-utils/plan";

/**
 * A-20 P8: Remove forecasts in the first frame, honestly, and the rail's
 * count follows the answer and never leads it.
 */
const wire = vi.hoisted(() => ({
  release: null as null | (() => void),
  fail: false,
  waits: false,
  plan: undefined as unknown as ActPlan,
}));

vi.mock("@/lib/api-client", async (original) => ({
  ...(await original<typeof import("@/lib/api-client")>()),
  request: async (path: string) => {
    if (path.includes("/plan?")) return wire.plan;
    throw new Error(`unmocked request ${path}`);
  },
}));

vi.mock("next/link", () => ({ default: ({ children }: { children: React.ReactNode }) => <span>{children}</span> }));
vi.mock("@/lib/queries/useRoleWaits", () => ({ useRoleWaits: () => () => wire.waits }));
const hold = () =>
  new Promise<{ retained_roles: string[]; cascade: { enqueued: number } }>((resolve, reject) => {
    wire.release = () => (wire.fail ? reject(new Error("boom")) : resolve({ retained_roles: [], cascade: { enqueued: 0 } }));
  });
vi.mock("@/lib/queries/useRoleMembers", () => ({
  useRemoveDirectGrant: () => useMutation({ mutationFn: hold }),
}));
vi.mock("@/lib/queries/useBundles", () => ({
  useRemoveBundle: () => useMutation({ mutationFn: hold }),
}));

const direct: RoleReason = { kind: "direct" };
const queuedDirect: RoleReason = { kind: "direct", queued: true };
const bundle: RoleReason = { kind: "bundle", bundle_id: "b1", bundle_name: "Lab Tech" };

const rail: Indicators = {
  pending_requests: 2,
  expiring_grants: 0,
  pending_propagation: 4,
  drift: 3,
  unconfirmed_revocations: 0,
  holds_due: 0,
  revocations_escalated: false,
  zitadel_reachable: true,
};

let client: QueryClient;
const railNow = () => client.getQueryData<Indicators>(["governance", "indicators"])!;

function open(sources: RoleReason[]) {
  client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  client.setQueryData(["governance", "indicators"], rail);
  const removal: Removal = {
    projectId: "pLaser",
    projectName: "Laser Lab",
    roleKey: "trained",
    sources,
    grantId: "g_88",
    grantsResolved: true,
  } as Removal;
  return render(
    <QueryClientProvider client={client}>
      <RemovalDialog removal={removal} userId="u1" userName="Kevin Owens" onClose={() => {}} />
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  wire.release = null;
  wire.fail = false;
  wire.waits = false;
  wire.plan = actPlan();
});

// The press waits on the plan (A-21 §3).
const planRead = (label: string) =>
  waitFor(() => expect(screen.getByRole("button", { name: label })).toBeEnabled());

describe("Remove, direct: the first 100ms", () => {
  it("says 'Will be revoked' while out, then the true word, and never the forecast after", async () => {
    open([direct]);
    await planRead("Revoke this role");
    fireEvent.click(screen.getByRole("button", { name: "Revoke this role" }));
    expect(await screen.findByText(FORECAST.revoke)).toBeInTheDocument();
    wire.release!();
    await screen.findByText(/Direct access to .* revoked/);
    expect(screen.getByText("Revoked")).toBeInTheDocument();
    expect(screen.queryByText(FORECAST.revoke)).toBeNull();
  });

  it("says 'Will wait for you to send' for a role that waits", async () => {
    wire.waits = true;
    open([direct]);
    await planRead("Revoke this role");
    fireEvent.click(screen.getByRole("button", { name: "Revoke this role" }));
    expect(await screen.findByText(FORECAST.queued)).toBeInTheDocument();
    expect(screen.queryByText(FORECAST.revoke)).toBeNull();
    wire.release!();
  });

  it("cancelling a give that still waits forecasts a cancel, and the rail drops by one only once answered", async () => {
    wire.plan = actPlan({ delivery: "none" });
    open([queuedDirect]);
    await planRead("Cancel giving it");
    fireEvent.click(screen.getByRole("button", { name: "Cancel giving it" }));
    expect(await screen.findByText(FORECAST.cancel)).toBeInTheDocument();
    // Ahead of the answer, nothing on the rail has moved.
    expect(railNow().pending_propagation).toBe(4);
    wire.release!();
    await waitFor(() => expect(railNow().pending_propagation).toBe(3));
  });

  it("an error is the honest error: no forecast left, the rail untouched", async () => {
    wire.fail = true;
    wire.plan = actPlan({ delivery: "none" });
    open([queuedDirect]);
    await planRead("Cancel giving it");
    fireEvent.click(screen.getByRole("button", { name: "Cancel giving it" }));
    await screen.findByText(FORECAST.cancel);
    wire.release!();
    await waitFor(() => expect(screen.getByText(/Nothing was changed/i)).toBeInTheDocument());
    expect(screen.queryByText(FORECAST.cancel)).toBeNull();
    expect(railNow().pending_propagation).toBe(4);
  });
});

describe("Remove, a bundle: the first 100ms", () => {
  it("forecasts 'Will be removed', or the wait for a role that waits", async () => {
    open([bundle]);
    await planRead("Remove the bundle");
    fireEvent.click(screen.getByRole("button", { name: "Remove the bundle" }));
    expect(await screen.findByText(FORECAST.remove)).toBeInTheDocument();
    wire.release!();
    await screen.findByText(/removed from Kevin Owens/);
  });

  it("says 'Will wait for you to send' when the role waits", async () => {
    wire.waits = true;
    open([bundle]);
    await planRead("Remove the bundle");
    fireEvent.click(screen.getByRole("button", { name: "Remove the bundle" }));
    expect(await screen.findByText(FORECAST.queued)).toBeInTheDocument();
    wire.release!();
  });
});
