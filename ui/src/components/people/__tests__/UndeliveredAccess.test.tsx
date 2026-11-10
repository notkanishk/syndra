// @vitest-environment jsdom
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

// Zitadel answering: what the dialogs say before the act is not under test here.
vi.mock("@/components/states/useWaitsBeforeLine", () => ({ useWaitsBeforeLine: () => undefined }));

import { RemovalDialog, type Removal } from "@/components/people/RemovalDialog";
import type { ActPlan } from "@/lib/usePlanConsent";
import { actPlan, withPlanClient } from "@/test-utils/plan";

/**
 * Recorded is not delivered, and the screens must not say otherwise.
 *
 * Assigning a bundle in manual mode writes the assignment and queues the
 * grants; nothing reaches Zitadel until somebody confirms Pending changes. For
 * the whole of that window the person page rendered those roles exactly like
 * roles delivered a month ago, and the removal dialog offered to take away
 * access that had never been given — "THEY WILL LOSE ... no other source gives
 * it" about a grant sitting in the outbox.
 *
 * The operator who hit it read the page as confirmation the work was done.
 */

const removed = vi.hoisted(() => ({
  calls: 0,
  enqueued: 0,
  // What the planner answers: a give that never went says it only cancels (A-21 §3).
  plan: undefined as unknown as ActPlan,
}));

vi.mock("@/lib/api-client", async (original) => ({
  ...(await original<typeof import("@/lib/api-client")>()),
  request: async (path: string) => {
    if (path.includes("/plan?")) return removed.plan;
    throw new Error(`unmocked request ${path}`);
  },
}));

vi.mock("@/lib/queries/useBundles", () => ({
  useRemoveBundle: () => ({
    mutateAsync: vi.fn(async () => {
      removed.calls += 1;
      return { cascade: { enqueued: removed.enqueued } };
    }),
    isPending: false,
  }),
}));
vi.mock("@/lib/queries/useRoleMembers", () => ({
  useRemoveDirectGrant: () => ({ mutateAsync: vi.fn(), isPending: false }),
}));

beforeEach(() => {
  removed.plan = actPlan();
});

/** The dialog, with the plan the backend would give for what it shows. */
function show(removal: Removal, neverSent: boolean) {
  removed.plan = actPlan(neverSent ? { delivery: "none" } : {});
  return render(withPlanClient(<RemovalDialog removal={removal} userId="nikki" userName="Nikki Cross" onClose={vi.fn()} />));
}

const bundleSource = (queued: boolean) => ({
  projectId: "p-audio",
  projectName: "Audio-Dash",
  roleKey: "community",
  sources: [
    { kind: "bundle", bundle_id: "b1", bundle_name: "Ops Admin", description: "", queued },
  ],
});

describe("removing a bundle whose grants were never sent", () => {
  it("does not claim they lose access they never received", async () => {
    show(bundleSource(true), true);

    await waitFor(() => expect(screen.getByRole("button", { name: "Cancel giving it" })).toBeEnabled());
    expect(screen.getByText(/Nothing to take back/i)).toBeInTheDocument();
    expect(screen.getByText(/recorded, never sent/i)).toBeInTheDocument();
    // The two sentences that were false, verbatim.
    expect(screen.queryByText(/They will lose/i)).toBeNull();
    expect(screen.queryByText(/no other source gives it/i)).toBeNull();
  });

  it("says nothing of its own about the waiting grants: the step is the one place that is said", async () => {
    show(bundleSource(true), true);

    expect(screen.queryByText(/Pending changes will be empty afterwards/i)).toBeNull();
    expect(screen.queryByText(/none of them had been sent/i)).toBeNull();
  });

  it("still says what a delivered bundle takes away", async () => {
    show(bundleSource(false), false);

    await waitFor(() => expect(screen.getByRole("button", { name: "Remove the bundle" })).toBeEnabled());
    expect(screen.getByText(/They will lose/i)).toBeInTheDocument();
    expect(screen.getByText(/no other source gives it/i)).toBeInTheDocument();
    expect(screen.queryByText(/Nothing to take back/i)).toBeNull();
  });
});

describe("revoking a direct grant that was never sent", () => {
  const directSource = (queued: boolean) => ({
    projectId: "p-audio",
    projectName: "Audio-Dash",
    roleKey: "community",
    grantId: "g1",
    grantsResolved: true,
    sources: [{ kind: "direct", description: "", queued }],
  });

  it("says there is nothing at the door to close, and the button is the cancel", async () => {
    show(directSource(true), true);

    expect(await screen.findByRole("button", { name: "Cancel giving it" })).toBeInTheDocument();
    expect(screen.getByText(/never had it, so nothing is sent to Zitadel/)).toBeInTheDocument();
    expect(screen.queryByText(/will lose this role/i)).toBeNull();
  });

  it("keeps the real consequence when the grant has been delivered", async () => {
    show(directSource(false), false);

    expect(screen.getByText(/will lose this role/i)).toBeInTheDocument();
    expect(await screen.findByRole("button", { name: "Revoke this role" })).toBeInTheDocument();
  });
});

describe("the action, once it has run", () => {
  it("names the cancel rather than a removal while nothing has been sent", async () => {
    show(bundleSource(true), true);

    const act = await screen.findByRole("button", { name: /Cancel giving it/ });
    // Red is a promise about consequence, and this one takes nothing away.
    expect(act.className).not.toMatch(/danger/);
    expect(screen.queryByRole("button", { name: "Remove the bundle" })).toBeNull();
  });

  it("cannot be pressed a second time", async () => {
    removed.calls = 0;
    removed.enqueued = 0;
    show(bundleSource(true), true);

    await waitFor(() => expect(screen.getByRole("button", { name: /Cancel giving it/ })).toBeEnabled());
    fireEvent.click(screen.getByRole("button", { name: /Cancel giving it/ }));
    await waitFor(() => expect(screen.getByRole("button", { name: "Close" })).toBeInTheDocument());

    // The dialog stays open to report what happened, and a destructive
    // confirm left armed inside it queues another cascade on the next click —
    // which is what a person does when the dialog does not close.
    expect(screen.queryByRole("button", { name: /Cancel giving it/ })).toBeNull();
    expect(screen.getByRole("button", { name: "Close" })).toBeInTheDocument();
    expect(removed.calls).toBe(1);
  });

  it("reports the queue it actually left behind", async () => {
    removed.calls = 0;
    removed.enqueued = 0;
    show(bundleSource(true), true);

    await waitFor(() => expect(screen.getByRole("button", { name: /Cancel giving it/ })).toBeEnabled());
    fireEvent.click(screen.getByRole("button", { name: /Cancel giving it/ }));
    await waitFor(() => expect(screen.getByText("Cancelled")).toBeInTheDocument());
    expect(
      screen.getByText("The Ops Admin bundle won't be given to Nikki. It never reached Zitadel."),
    ).toBeInTheDocument();
    // The sentence that contradicted the dialog one centimetre above it.
    expect(screen.queryByText(/are revoked once you send/i)).toBeNull();
  });

  it("still says what a delivered removal queued", async () => {
    removed.calls = 0;
    removed.enqueued = 3;
    show(bundleSource(false), false);

    await waitFor(() => expect(screen.getByRole("button", { name: "Remove the bundle" })).toBeEnabled());
    fireEvent.click(screen.getByRole("button", { name: "Remove the bundle" }));
    // No wait reason: the server delivered all three (services.Deliver), so it
    // says they went, never "once you send" (final verification N1).
    await waitFor(() => expect(screen.getByText(/3 changes went to Zitadel/)).toBeInTheDocument());
  });
});
