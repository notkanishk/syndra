// @vitest-environment jsdom
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

// Zitadel answering: what the dialogs say before the act is not under test here.
vi.mock("@/components/states/useWaitsBeforeLine", () => ({ useWaitsBeforeLine: () => undefined }));

import { ManageBundles, bundlesBeforeLine } from "@/components/people/ManageBundles";
import { ApiError } from "@/lib/api-client";
import type { ActPlan } from "@/lib/usePlanConsent";
import { seedAutoRetry } from "@/test-utils/drain";
import { actPlan, cancelling, planClient, queuedChange, withPlanClient } from "@/test-utils/plan";

/**
 * What the assign panel promises has to be what the assign gives.
 *
 * An assignment pins the latest PUBLISHED version of a bundle and nothing else.
 * This panel asked for the working copy, so it listed every unpublished edit as
 * a role the person was about to receive — the operator saw the roles they had
 * just added promised here and called "unpublished" on the bundle screen, and
 * the apply then granted neither of the sets they had been shown.
 */

const state = vi.hoisted(() => ({
  roleQueries: vi.fn(),
  assignAsync: vi.fn().mockResolvedValue(undefined),
  removeAsync: vi.fn().mockResolvedValue(undefined),
  // What the planner answers per bundle, and the reads it was asked.
  plans: {} as Record<string, ActPlan>,
  planCalls: [] as string[],
}));

vi.mock("@/lib/queries/useBundles", () => ({
  useBundles: () => ({
    data: [
      {
        id: "b1",
        name: "Lab Tech",
        latest_version: 2,
        unpublished_changes: 2,
        holder_count: 0,
      },
      { id: "b2", name: "Door Crew", latest_version: 1, unpublished_changes: 0, holder_count: 1 },
    ],
  }),
  useBundleRolesByBundle: (...args: unknown[]) => state.roleQueries(...args),
  useRemoveBundle: () => ({ mutateAsync: (...args: unknown[]) => state.removeAsync(...args), isPending: false }),
}));

// What a change does, and what the rules add, is the planner's to say (A-21 §3),
// never this panel's: one plan per staged bundle.
vi.mock("@/lib/api-client", async (original) => ({
  ...(await original<typeof import("@/lib/api-client")>()),
  request: async (path: string) => {
    if (!path.includes("/plan?")) throw new Error(`unmocked request ${path}`);
    state.planCalls.push(path);
    return state.plans[new URLSearchParams(path.split("?")[1]).get("bundle_id") ?? ""] ?? actPlan();
  },
}));

vi.mock("@/lib/queries/useUsers", () => ({
  useAssignBundle: () => ({ mutateAsync: state.assignAsync, isPending: false }),
  useUserAccess: () => ({ data: { projects: [] } }),
}));

vi.mock("@/components/names", () => ({
  RoleRef: ({ roleKey }: { roleKey: string }) => <span>{roleKey}</span>,
}));

beforeEach(() => {
  state.roleQueries = vi.fn().mockReturnValue({
    byId: {
      b1: [{ bundle_id: "b1", zitadel_project_id: "p1", zitadel_role_key: "published-role" }],
    },
    allLoaded: true,
  });
  state.assignAsync = vi.fn().mockResolvedValue(undefined);
  state.removeAsync = vi.fn().mockResolvedValue(undefined);
  state.plans = {};
  state.planCalls = [];
});

function open(assigned: Array<{ id: string; name: string }> = []) {
  return render(
    withPlanClient(<ManageBundles userId="u1" userName="Ada" assigned={assigned} open onClose={vi.fn()} />, seedAutoRetry(planClient(), true)),
  );
}

const tick = (name: RegExp) => fireEvent.click(screen.getByRole("checkbox", { name }));
// The press waits on every staged plan (A-21 §3).
const applyReady = (label = "Apply 1 change") =>
  waitFor(() => expect(screen.getByRole("button", { name: label })).toBeEnabled());

describe("ManageBundles", () => {
  it("asks for the published version, not the working copy", () => {
    open();

    expect(state.roleQueries).toHaveBeenCalledWith(["b1", "b2"], { published: true });
  });

  // The role list is the planner's to say now; the row counts the roles of the
  // published version, the set an assignment actually pins.
  it("counts the roles the assignment will actually grant: the published version's", () => {
    open();
    expect(screen.getByText(/· 1 role$/)).toBeInTheDocument();
    expect(screen.getByText(/· 0 roles$/)).toBeInTheDocument();
  });

  // The panel is right and the bundle screen shows a different set, so an
  // operator who has just added those roles would read their absence here as
  // this panel being stale rather than as the truth about what a pin means.
  it("says the unpublished edits are not part of the assignment", () => {
    open();
    fireEvent.click(screen.getByRole("checkbox", { name: /Lab Tech/ }));

    expect(screen.getByText(/2 unpublished changes/)).toBeInTheDocument();
    expect(screen.getByText(/an assignment gives v2/i)).toBeInTheDocument();
  });

  // The pre-apply hint used to promise "Queues for confirmation" every time
  // something was staged, which the apply outcome does not always keep: a
  // change that reaches nobody's effective access is reported as `applied`,
  // not queued. The hint must hold in both cases.
  it("hints at what the button does without promising a queue that may not happen", async () => {
    open();
    fireEvent.click(screen.getByRole("checkbox", { name: /Lab Tech/ }));

    // The planner's own line: it goes now, or it waits, as the plan says.
    expect(await screen.findByText("This goes to Zitadel when you apply.")).toBeInTheDocument();
    expect(screen.queryByText(/queues for confirmation/i)).not.toBeInTheDocument();
  });

  it("reports Recorded, not queued, when the assignment reaches nobody yet", async () => {
    state.plans = { b1: actPlan({ delivery: "none" }) };
    state.assignAsync = vi.fn().mockResolvedValue({ cascade: { enqueued: 0 } });
    open();
    fireEvent.click(screen.getByRole("checkbox", { name: /Lab Tech/ }));
    await applyReady();
    fireEvent.click(screen.getByRole("button", { name: "Apply 1 change" }));

    // The act's own word, never the "Applied" fallback.
    expect(await screen.findByText("Recorded")).toBeInTheDocument();
    expect(screen.queryByText("Applied")).not.toBeInTheDocument();
    expect(screen.queryByRole("link", { name: /Waiting for you to send/ })).not.toBeInTheDocument();
    expect(screen.getByText(/nothing is waiting for you to send/i)).toBeInTheDocument();
  });

  // Final verification N1: a cascade Zitadel refused is said as a refusal, with
  // its cause and step; it never waits, and Send is not the step.
  it("says Zitadel refused this, with why and what to do, when it did", async () => {
    state.assignAsync = vi.fn().mockResolvedValue({ cascade: { enqueued: 1, wait_reason: "refused:not_allowed" } });
    open();
    fireEvent.click(screen.getByRole("checkbox", { name: /Lab Tech/ }));
    await applyReady();
    fireEvent.click(screen.getByRole("button", { name: "Apply 1 change" }));

    expect(await screen.findByText("Refused by Zitadel")).toBeInTheDocument();
    expect(screen.getByText(/Zitadel refused this: Syndra's service account isn't allowed/)).toBeInTheDocument();
    expect(screen.getByText(/Ask whoever runs Zitadel/)).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: /Waiting for you to send/ })).not.toBeInTheDocument();
    expect(screen.queryByText(/Nothing was changed/)).not.toBeInTheDocument();
  });

  it("says a cascade the server sent went, never that it waits", async () => {
    state.assignAsync = vi.fn().mockResolvedValue({ cascade: { enqueued: 2 } });
    open();
    fireEvent.click(screen.getByRole("checkbox", { name: /Lab Tech/ }));
    await applyReady();
    fireEvent.click(screen.getByRole("button", { name: "Apply 1 change" }));

    expect(await screen.findByText(/Sent to Zitadel: 2 changes/)).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: /Waiting for you to send/ })).not.toBeInTheDocument();
  });

  it("reports the change waiting, and where to send it, when it enqueues a cascade", async () => {
    state.assignAsync = vi.fn().mockResolvedValue({ cascade: { enqueued: 2, waiting: 2, wait_reason: "role_waits" } });
    open();
    fireEvent.click(screen.getByRole("checkbox", { name: /Lab Tech/ }));
    await applyReady();
    fireEvent.click(screen.getByRole("button", { name: "Apply 1 change" }));

    expect(await screen.findByRole("link", { name: /Waiting for you to send/ })).toHaveAttribute(
      "href",
      "/governance/pending",
    );
    expect(screen.getByText(/2 changes wait under pending changes/i)).toBeInTheDocument();
  });

  // The what-if list and "through an automatic rule" lines are gone: the
  // planner's words for the change, rules followed, are the one source.
  it("says the planner's words for a staged change, rules included, under the bundle's name", async () => {
    state.plans = {
      b1: actPlan({
        detail: "Ada gets Lab Tech.",
        said: ["Also gives Bench in Workshop, by the automatic rule from Operator."],
      }),
    };
    open();
    tick(/Lab Tech/);

    expect(await screen.findByText("Ada gets Lab Tech. Also gives Bench in Workshop, by the automatic rule from Operator.")).toBeInTheDocument();
    expect(state.planCalls).toEqual(["/users/u1/plan?op=assign_bundle&bundle_id=b1"]);
    expect(screen.queryByText(/would give/)).not.toBeInTheDocument();
    expect(screen.queryByText(/through an automatic rule/)).not.toBeInTheDocument();
    expect(screen.queryByText("already held, no change")).not.toBeInTheDocument();
  });

  it("removing asks the planner for remove_bundle", async () => {
    open([{ id: "b2", name: "Door Crew" }]);
    tick(/Door Crew/);
    await applyReady();
    expect(state.planCalls).toEqual(["/users/u1/plan?op=remove_bundle&bundle_id=b2"]);
  });

  it("holds Apply while the plan is read, and when it can't be", async () => {
    open();
    tick(/Lab Tech/);
    expect(screen.getByRole("button", { name: "Apply 1 change" })).toBeDisabled();
    expect(screen.getAllByText("Checking what this does.").length).toBeGreaterThan(0);
    await applyReady();
  });

  it("holds Apply, with the planner's reason, when the planner refuses the change", async () => {
    state.plans = { b1: actPlan({ effect: "no_change", refusal: "held", detail: "Ada already has Lab Tech." }) };
    open();
    tick(/Lab Tech/);
    expect((await screen.findAllByText("Ada already has Lab Tech.")).length).toBeGreaterThan(0);
    expect(screen.getByRole("button", { name: "Apply 1 change" })).toBeDisabled();
  });

  it("an empty bundle keeps its own line instead of the planner's", async () => {
    state.roleQueries = vi.fn().mockReturnValue({ byId: { b1: [] }, allLoaded: true });
    state.plans = { b1: actPlan({ detail: "Ada gets Lab Tech." }) };
    open();
    tick(/Lab Tech/);
    expect(await screen.findByText("v2 of this bundle carries no roles, so this gives nothing.")).toBeInTheDocument();
    expect(screen.queryByText("Ada gets Lab Tech.")).not.toBeInTheDocument();
  });

  it("says two changes' before-line from their plans: some go now, some wait", async () => {
    state.plans = { b1: actPlan({ delivery: "straight_away" }), b2: actPlan({ delivery: "waits_for_you" }) };
    open([{ id: "b2", name: "Door Crew" }]);
    tick(/Lab Tech/);
    tick(/Door Crew/);
    expect(await screen.findByText("Apply sends 1 change to Zitadel now. 1 waits for you to send.")).toBeInTheDocument();
  });

  describe("what it cancels", () => {
    const giveWaiting = queuedChange({ id: "00000000-0000-0000-0000-0000000000c1", op_type: "add", role_keys: ["published-role"] });
    const removalWaiting = queuedChange({ id: "00000000-0000-0000-0000-0000000000c2" });

    it("foretells the cancel before the press, once, from the plan", async () => {
      state.plans = { b1: cancelling([removalWaiting]) };
      open();
      tick(/Lab Tech/);
      expect(
        await screen.findByText("Applying cancels the removal Sam Lee queued 2h ago. Nothing is cancelled until you choose on the next step."),
      ).toBeInTheDocument();
      expect(state.assignAsync).not.toHaveBeenCalled();
    });

    it("one change: the step opens at once from the plan, no write first, and the write carries that bundle's plan", async () => {
      state.plans = { b1: cancelling([removalWaiting], { fingerprint: "fp-b1" }) };
      state.assignAsync = vi.fn().mockResolvedValue({ cascade: { enqueued: 0 } });
      open();
      tick(/Lab Tech/);
      await applyReady();
      fireEvent.click(screen.getByRole("button", { name: "Apply 1 change" }));

      expect(await screen.findByRole("heading", { name: "This cancels a removal that's waiting" })).toBeInTheDocument();
      expect(state.assignAsync).not.toHaveBeenCalled();
      fireEvent.click(screen.getByRole("button", { name: "Cancel it and apply 1 change" }));

      expect(await screen.findByText("Recorded")).toBeInTheDocument();
      expect(state.assignAsync).toHaveBeenCalledTimes(1);
      expect(state.assignAsync).toHaveBeenCalledWith({
        bundleId: "b1",
        seen: { fingerprint: "fp-b1", cancel_ids: [removalWaiting.id] },
      });
    });

    it("several changes: the cancels of all of them are gathered into ONE ask before the first write", async () => {
      state.plans = {
        b1: cancelling([giveWaiting], { fingerprint: "fp-b1" }),
        b2: cancelling([removalWaiting], { fingerprint: "fp-b2" }),
      };
      state.assignAsync = vi.fn().mockResolvedValue({ cascade: { enqueued: 0 } });
      state.removeAsync = vi.fn().mockResolvedValue({ cascade: { enqueued: 0 } });
      open([{ id: "b2", name: "Door Crew" }]);
      tick(/Lab Tech/);
      tick(/Door Crew/);
      await applyReady("Apply 2 changes");
      fireEvent.click(screen.getByRole("button", { name: "Apply 2 changes" }));

      expect(await screen.findByRole("heading", { name: "This cancels 2 changes that are waiting" })).toBeInTheDocument();
      expect(screen.getAllByRole("dialog")).toHaveLength(1);
      expect(state.assignAsync).not.toHaveBeenCalled();
      expect(state.removeAsync).not.toHaveBeenCalled();
      fireEvent.click(screen.getByRole("button", { name: "Cancel all 2 and apply 2 changes" }));

      await waitFor(() => expect(state.removeAsync).toHaveBeenCalledTimes(1));
      expect(state.assignAsync).toHaveBeenCalledWith({ bundleId: "b1", seen: { fingerprint: "fp-b1", cancel_ids: [giveWaiting.id] } });
      expect(state.removeAsync).toHaveBeenCalledWith({ bundleId: "b2", seen: { fingerprint: "fp-b2", cancel_ids: [removalWaiting.id] } });
    });

    it("the same waiting change named by two plans is asked about once", async () => {
      state.plans = { b1: cancelling([removalWaiting]), b2: cancelling([removalWaiting]) };
      open([{ id: "b2", name: "Door Crew" }]);
      tick(/Lab Tech/);
      tick(/Door Crew/);
      await applyReady("Apply 2 changes");
      fireEvent.click(screen.getByRole("button", { name: "Apply 2 changes" }));
      expect(await screen.findByRole("heading", { name: "This cancels a removal that's waiting" })).toBeInTheDocument();
      expect(screen.getByRole("button", { name: "Cancel it and apply 2 changes" })).toBeInTheDocument();
    });

    it("Keep it sends nothing, and the bundles stay staged", async () => {
      state.plans = { b1: cancelling([removalWaiting]) };
      open();
      tick(/Lab Tech/);
      await applyReady();
      fireEvent.click(screen.getByRole("button", { name: "Apply 1 change" }));
      fireEvent.click(await screen.findByRole("button", { name: "Keep it" }));
      await waitFor(() => expect(screen.getByRole("button", { name: "Apply 1 change" })).toBeVisible());
      expect(state.assignAsync).not.toHaveBeenCalled();
    });

    it("cancelling nothing pushes no step", async () => {
      state.assignAsync = vi.fn().mockResolvedValue({ cascade: { enqueued: 1 } });
      open();
      tick(/Lab Tech/);
      await applyReady();
      fireEvent.click(screen.getByRole("button", { name: "Apply 1 change" }));
      expect(await screen.findByText(/Sent to Zitadel: one change/)).toBeInTheDocument();
      expect(screen.queryByRole("heading", { name: /This cancels/ })).not.toBeInTheDocument();
      expect(state.assignAsync).toHaveBeenCalledWith({ bundleId: "b1", seen: { fingerprint: "fp-1", cancel_ids: [] } });
    });
  });

  it("when their access moved: nothing was changed, the notice says so, and the plans are read again", async () => {
    state.assignAsync = vi.fn().mockRejectedValue(new ApiError(409, { error: "PLAN_STALE", message: "Moved." }));
    open();
    tick(/Lab Tech/);
    await applyReady();
    const reads = state.planCalls.length;
    fireEvent.click(screen.getByRole("button", { name: "Apply 1 change" }));

    expect(await screen.findByText("Nothing was changed. Ada's access moved while this was open.")).toBeInTheDocument();
    expect(screen.getByText("Read the note under each bundle, then press Apply again.")).toBeInTheDocument();
    await waitFor(() => expect(state.planCalls.length).toBeGreaterThan(reads));
    // Not a result: still staged, still the form.
    expect(screen.queryByText("Recorded")).not.toBeInTheDocument();
  });
});

describe("bundlesBeforeLine", () => {
  const now = actPlan({ delivery: "straight_away" });
  const wait = actPlan({ delivery: "waits_for_you" });
  const none = actPlan({ delivery: "none" });

  it("says a single change by itself", () => {
    expect(bundlesBeforeLine([now])).toBe("This goes to Zitadel when you apply.");
    expect(bundlesBeforeLine([wait])).toBe("This waits for you to send.");
  });

  it("counts several: some now, some waiting", () => {
    expect(bundlesBeforeLine([now, now, wait])).toBe("Apply sends 2 changes to Zitadel now. 1 waits for you to send.");
    expect(bundlesBeforeLine([now, wait, wait])).toBe("Apply sends 1 change to Zitadel now. 2 wait for you to send.");
  });

  it("says every change waits when they all do", () => {
    expect(bundlesBeforeLine([wait, wait])).toBe("Every change waits for you to send.");
  });

  it("says a cancel-only change sends nothing", () => {
    expect(bundlesBeforeLine([none])).toBe("For 1, this only cancels a change that's waiting; nothing is sent.");
    expect(bundlesBeforeLine([now, none, none])).toBe(
      "Apply sends 1 change to Zitadel now. For 2, this only cancels changes that are waiting; nothing is sent.",
    );
  });

  it("leaves out a change the planner refuses, and says nothing is sent when none is left", () => {
    expect(bundlesBeforeLine([now, actPlan({ effect: "blocked" })])).toBe("This goes to Zitadel when you apply.");
    expect(bundlesBeforeLine([actPlan({ effect: "no_change" })])).toBe("Nothing here is sent to Zitadel.");
    expect(bundlesBeforeLine([undefined])).toBe("Nothing here is sent to Zitadel.");
  });
});
