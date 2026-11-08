// @vitest-environment jsdom
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("@/components/states/useWaitsBeforeLine", () => ({ useWaitsBeforeLine: () => undefined }));

import { RemovalDialog, type Removal } from "@/components/people/RemovalDialog";
import type { ActPlan } from "@/lib/usePlanConsent";
import { actPlan, planClient, queuedChange, withPlanClient } from "@/test-utils/plan";

/**
 * Browser pass, 8 Nov: cancelling Anushka's waiting give of Navidrome /
 * Community. The title said "Revoke direct access", the dismiss beside "Cancel
 * the waiting give" was another "Cancel", and after the press the act's own
 * refetch (the give gone, the planner now reading a held role) re-derived the
 * body: a red "will lose this role" above the green "Cancelled", and the lede
 * switched to "Given directly to". The act's refetch is simulated here by the
 * plan read changing under the dialog the moment the act runs.
 */

const world = vi.hoisted(() => ({ plan: undefined as unknown as ActPlan, client: undefined as unknown as import("@tanstack/react-query").QueryClient }));

vi.mock("@/lib/api-client", async (original) => ({
  ...(await original<typeof import("@/lib/api-client")>()),
  request: async (path: string) => {
    if (path.includes("/plan?")) return world.plan;
    throw new Error(`unmocked request ${path}`);
  },
}));

// The act, and the refetch it causes: after it the give is gone and revoking
// would be a real revoke, which is what the planner then answers.
const afterTheAct = async () => {
  world.plan = actPlan({ delivery: "straight_away" });
  await world.client.invalidateQueries();
  return { cascade: { enqueued: 0 } };
};
vi.mock("@/lib/queries/useRoleMembers", () => ({
  useRemoveDirectGrant: () => ({ mutateAsync: afterTheAct, isPending: false }),
}));
vi.mock("@/lib/queries/useBundles", () => ({
  useRemoveBundle: () => ({ mutateAsync: afterTheAct, isPending: false }),
}));

const give = queuedChange({ op_type: "add", initiated_by_name: "Sam Lee" });

beforeEach(() => {
  world.plan = actPlan({ delivery: "none", cancels: [give], cancel_ids: [give.id] });
  world.client = planClient();
});

const show = (removal: Removal) =>
  render(withPlanClient(<RemovalDialog removal={removal} userId="anushka" userName="Anushka Sinha" onClose={vi.fn()} />, world.client));

/** The act, once the plan is read and the button armed. */
async function ready(name: string) {
  await waitFor(() => expect(screen.getByRole("button", { name })).toBeEnabled());
  return screen.getByRole("button", { name });
}

const direct: Removal = {
  projectId: "p-navidrome",
  projectName: "Navidrome",
  roleKey: "community",
  grantId: "g1",
  grantsResolved: true,
  sources: [{ kind: "direct", description: "", queued: true }],
};

describe("cancelling a direct give that is still waiting", () => {
  it("is titled as the act, and its dismiss keeps it rather than cancelling", async () => {
    show(direct);

    await ready("Cancel the waiting give");
    expect(screen.getByRole("heading", { name: /^Cancel the waiting give of .* in Navidrome\?$/ })).toBeInTheDocument();
    expect(screen.queryByText(/Revoke direct access/)).toBeNull();
    expect(screen.getByRole("button", { name: "Keep it" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Cancel" })).toBeNull();
  });

  it("after the press, says the result under what it said before, never a warning from the refetch", async () => {
    show(direct);

    fireEvent.click(await ready("Cancel the waiting give"));
    expect(await screen.findByText(/that was waiting is cancelled/)).toBeInTheDocument();
    // Let the refetched plan land and render.
    await waitFor(() => expect(world.client.isFetching()).toBe(0));

    expect(screen.queryByText(/will lose this role/i)).toBeNull();
    expect(screen.queryByText(/Their access ends once the change reaches Zitadel/)).toBeNull();
    expect(screen.queryByText(/Given directly to/)).toBeNull();
    expect(screen.getByText(/still waiting for you to send/)).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: /^Cancel the waiting give of/ })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Close" })).toBeInTheDocument();
  });

  it("a revoke of a delivered give keeps its own title and a plain Cancel", async () => {
    world.plan = actPlan({ delivery: "straight_away" });
    show({ ...direct, sources: [{ kind: "direct", description: "", queued: false }] });

    expect(await screen.findByRole("button", { name: "Revoke access" })).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: /^Revoke direct access to Navidrome \/ .*\?$/ })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Cancel" })).toBeInTheDocument();
  });
});

describe("cancelling a bundle that is still waiting", () => {
  const bundle: Removal = {
    projectId: "p-navidrome",
    projectName: "Navidrome",
    roleKey: "community",
    sources: [{ kind: "bundle", bundle_id: "b1", bundle_name: "Community – Basic", description: "", queued: true }],
  };

  it("keeps it on dismiss, and the refetch after the press does not turn the body into a loss", async () => {
    show(bundle);

    fireEvent.click(await ready("Cancel the waiting change"));
    expect(screen.queryByRole("button", { name: "Cancel" })).toBeNull();
    expect(await screen.findByText(/bundle that was waiting is cancelled/)).toBeInTheDocument();
    await waitFor(() => expect(world.client.isFetching()).toBe(0));

    expect(screen.queryByText(/They will lose/i)).toBeNull();
    expect(screen.getByText(/Nothing to take back/i)).toBeInTheDocument();
  });

  it("offers Keep it beside the cancel before the press", async () => {
    show(bundle);
    await ready("Cancel the waiting change");
    expect(screen.getByRole("button", { name: "Keep it" })).toBeInTheDocument();
  });
});
