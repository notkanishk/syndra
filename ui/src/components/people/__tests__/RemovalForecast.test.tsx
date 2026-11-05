// @vitest-environment jsdom
import { QueryClient, QueryClientProvider, useMutation } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { RoleReason } from "@/components/access/AccessSource";
import { RemovalDialog, type Removal } from "@/components/people/RemovalDialog";
import { FORECAST } from "@/lib/outcome";
import type { Indicators } from "@/lib/queries/useIndicators";

/**
 * A-20 P8: Remove forecasts in the first frame, honestly, and the rail's
 * count follows the answer and never leads it.
 */
const wire = vi.hoisted(() => ({
  release: null as null | (() => void),
  fail: false,
  waits: false,
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
      <RemovalDialog removal={removal} userId="u1" userName="Tomas Beck" onClose={() => {}} />
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  wire.release = null;
  wire.fail = false;
  wire.waits = false;
});

describe("Remove, direct: the first 100ms", () => {
  it("says 'Will be removed' while out, then the true word, and never the forecast after", async () => {
    open([direct]);
    fireEvent.click(screen.getByRole("button", { name: "Revoke access" }));
    expect(await screen.findByText(FORECAST.remove)).toBeInTheDocument();
    wire.release!();
    await screen.findByText(/Direct access to .* removed/);
    expect(screen.queryByText(FORECAST.remove)).toBeNull();
  });

  it("says 'Will wait for you to send' for a role that waits", async () => {
    wire.waits = true;
    open([direct]);
    fireEvent.click(screen.getByRole("button", { name: "Revoke access" }));
    expect(await screen.findByText(FORECAST.queued)).toBeInTheDocument();
    expect(screen.queryByText(FORECAST.remove)).toBeNull();
    wire.release!();
  });

  it("withdrawing a grant that still waits forecasts a cancel, and the rail drops by one only once answered", async () => {
    open([queuedDirect]);
    fireEvent.click(screen.getByRole("button", { name: "Withdraw the queued grant" }));
    expect(await screen.findByText(FORECAST.cancel)).toBeInTheDocument();
    // Ahead of the answer, nothing on the rail has moved.
    expect(railNow().pending_propagation).toBe(4);
    wire.release!();
    await waitFor(() => expect(railNow().pending_propagation).toBe(3));
  });

  it("an error is the honest error: no forecast left, the rail untouched", async () => {
    wire.fail = true;
    open([queuedDirect]);
    fireEvent.click(screen.getByRole("button", { name: "Withdraw the queued grant" }));
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
    fireEvent.click(screen.getByRole("button", { name: "Remove bundle" }));
    expect(await screen.findByText(FORECAST.remove)).toBeInTheDocument();
    wire.release!();
    await screen.findByText(/removed from Tomas Beck/);
  });

  it("says 'Will wait for you to send' when the role waits", async () => {
    wire.waits = true;
    open([bundle]);
    fireEvent.click(screen.getByRole("button", { name: "Remove bundle" }));
    expect(await screen.findByText(FORECAST.queued)).toBeInTheDocument();
    wire.release!();
  });
});
