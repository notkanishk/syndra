// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { RemovalDialog, type Removal } from "@/components/people/RemovalDialog";
import type { RoleReason } from "@/components/access/AccessSource";
import { ApiError } from "@/lib/api-client";
import type { ActPlan } from "@/lib/usePlanConsent";
import { actPlan, cancelling, queuedChange } from "@/test-utils/plan";

const removeDirect = vi.hoisted(() => vi.fn());
const removeBundle = vi.hoisted(() => vi.fn());
// What the planner answers (GET /users/{id}/plan), and the reads it was asked.
const wire = vi.hoisted(() => ({ plan: undefined as unknown as ActPlan, calls: [] as string[] }));

vi.mock("@/lib/api-client", async (original) => ({
  ...(await original<typeof import("@/lib/api-client")>()),
  request: async (path: string) => {
    if (path.includes("/plan?")) {
      wire.calls.push(path);
      return wire.plan;
    }
    throw new Error(`unmocked request ${path}`);
  },
}));

vi.mock("next/link", () => ({
  default: ({ children }: { children: React.ReactNode }) => <span>{children}</span>,
}));
vi.mock("@/lib/queries/useRoleMembers", () => ({
  useRemoveDirectGrant: () => ({ mutateAsync: removeDirect, isPending: false }),
}));
vi.mock("@/lib/queries/useBundles", () => ({
  useRemoveBundle: () => ({ mutateAsync: removeBundle, isPending: false }),
}));

const direct: RoleReason = { kind: "direct" };
const bundle: RoleReason = { kind: "bundle", bundle_id: "b1", bundle_name: "Lab Tech" };
const mapping: RoleReason = { kind: "mapping", trigger_project: "3D Lab", trigger_role: "operator" };

beforeEach(() => {
  wire.plan = actPlan();
  wire.calls = [];
  removeDirect.mockReset();
  removeBundle.mockReset();
});

// The press waits on the plan (A-21 §3).
const planRead = (label = "Revoke access") =>
  waitFor(() => expect(screen.getByRole("button", { name: label })).toBeEnabled());

function open(removal: Partial<Removal> & { sources: RoleReason[] }) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <RemovalDialog
        removal={{
          projectId: "pLaser",
          projectName: "Laser Lab",
          roleKey: "trained",
          userId: "u_2f81",
          userName: "Tomas Beck",
          ...removal,
        }}
        userId="u_2f81"
        userName="Tomas Beck"
        onClose={() => {}}
      />
    </QueryClientProvider>,
  );
}

describe("source-specific removal", () => {
  // The residual-outcome sentence is the difference between a safe click and
  // an outage on the laser cutter.
  it("says they will LOSE the role when nothing else gives it to them", () => {
    open({ sources: [direct], grantId: "g_88" });
    expect(screen.getByText(/will lose this role/i)).toBeInTheDocument();
    expect(screen.queryByText(/will still hold/i)).toBeNull();
  });

  it("says they will STILL HOLD it when another source survives", () => {
    open({ sources: [direct, bundle], grantId: "g_88" });
    // Two sources → the menu names one removal per source, never a guess.
    fireEvent.click(screen.getByText("Revoke direct access"));

    expect(screen.getByText(/will still hold this role/i)).toBeInTheDocument();
    expect(screen.getByText(/via Lab Tech/i)).toBeInTheDocument();
  });

  it("blocks the confirm with a visible reason when there is no grant to remove", () => {
    // Reachable by design so the copy can be reviewed; the button carries its
    // reason in text rather than only in a title attribute.
    open({ sources: [direct], grantId: undefined });
    const confirm = screen.getByRole("button", { name: "Revoke access" });
    expect(confirm).toBeDisabled();
    expect(screen.getByText(/no direct access to revoke/i)).toBeInTheDocument();
  });

  it("offers no removal at all for an automatic source", () => {
    open({ sources: [mapping] });

    expect(screen.getByText("This one isn't yours to remove.")).toBeInTheDocument();
    // No destructive colour anywhere, because nothing is being destroyed.
    expect(screen.queryByRole("button", { name: /remove/i })).toBeNull();
    // It names the input role — the rule's input is the only per-person route,
    // stated both in the rule sentence and in the instruction beneath it.
    expect(screen.getAllByText(/3D Lab \/ operator/).length).toBeGreaterThan(0);
  });

  it("names the bundle and what is lost with it", () => {
    open({ sources: [bundle] });

    expect(screen.getByText(/Remove the Lab Tech bundle from Tomas Beck\?/)).toBeInTheDocument();
    expect(screen.getByText("They will lose")).toBeInTheDocument();
    expect(
      screen.getByText(/Every other role this bundle carries is removed too/i),
    ).toBeInTheDocument();
  });

  it("lists one entry per source when a role is held more than one way", () => {
    open({ sources: [direct, bundle, mapping], grantId: "g_88" });

    expect(screen.getByText(/holds Laser Lab \/ trained 3 ways/i)).toBeInTheDocument();
    expect(screen.getByText("Revoke direct access")).toBeInTheDocument();
    expect(screen.getByText("Remove bundle assignment")).toBeInTheDocument();
    expect(screen.getByText("Open the rule")).toBeInTheDocument();
  });

  it("removes the grant the row is about, not some other one, carrying the plan it was shown", async () => {
    wire.plan = actPlan({ fingerprint: "fp-7" });
    open({ sources: [direct], grantId: "g_88" });
    await planRead();

    expect(wire.calls).toEqual(["/users/u_2f81/plan?op=remove&grant_id=g_88"]);
    fireEvent.click(screen.getByRole("button", { name: "Revoke access" }));
    expect(removeDirect).toHaveBeenCalledWith({
      userId: "u_2f81",
      grantId: "g_88",
      seen: { fingerprint: "fp-7", cancel_ids: [] },
    });
  });

  it("holds the revoke while the plan is read, with the words for it", async () => {
    open({ sources: [direct], grantId: "g_88" });
    expect(screen.getByRole("button", { name: "Revoke access" })).toBeDisabled();
    expect(screen.getByText("Checking what revoking it does.")).toBeInTheDocument();
    await planRead();
  });
});

describe("initial focus", () => {
  // The dialog's focus trap focuses the first focusable element on open. A
  // destructive confirm rendered first got that focus for free, so Enter —
  // pressed on reflex to close a dialog that just opened — revoked access.
  it("lands on Cancel, never on the destructive confirm", () => {
    open({ sources: [direct], grantId: "g_88" });

    expect(screen.getByRole("button", { name: "Cancel" })).toHaveFocus();
  });

  it("lands on Cancel for a bundle removal too", () => {
    open({ sources: [bundle] });

    expect(screen.getByRole("button", { name: "Cancel" })).toHaveFocus();
  });
});

describe("a removal that would cancel a give still waiting (the plan says so before the press)", () => {
  const waiting = queuedChange({
    id: "00000000-0000-0000-0000-000000000009",
    op_type: "add",
    user_id: "u_2f81",
    project_id: "pLaser",
    role_keys: ["trained"],
    source: "bundle",
    initiated_by: "op-2",
    initiated_by_name: "Sam Iyer",
    said: "joined the course",
  });

  it("foretells the cancel, then turns the sheet to the step at once; Keep it sends nothing and returns here", async () => {
    wire.plan = cancelling([waiting]);
    open({ sources: [bundle] });
    await planRead("Remove bundle");
    expect(screen.getByText("Removing it cancels the give Sam Iyer queued 2h ago.")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Remove bundle" }));
    expect(await screen.findByRole("heading", { name: "This cancels a give that's waiting" })).toBeInTheDocument();
    // One sheet, never two; and no write was sent to find this out.
    expect(screen.getAllByRole("dialog")).toHaveLength(1);
    expect(removeBundle).not.toHaveBeenCalled();
    expect(screen.getByRole("button", { name: "Cancel it and remove" })).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Keep it" }));
    await waitFor(() => expect(screen.getByRole("button", { name: "Remove bundle" })).toBeInTheDocument());
    expect(removeBundle).not.toHaveBeenCalled();
    expect(screen.getByText("Nothing changed. Sam's give is still waiting.")).toBeInTheDocument();
  });

  it("going ahead sends one removal that carries the fingerprint and the ids it showed", async () => {
    wire.plan = cancelling([waiting], { fingerprint: "fp-4" });
    removeBundle.mockResolvedValueOnce({ cascade: { enqueued: 0 } });
    open({ sources: [bundle] });
    await planRead("Remove bundle");

    fireEvent.click(screen.getByRole("button", { name: "Remove bundle" }));
    fireEvent.click(await screen.findByRole("button", { name: "Cancel it and remove" }));
    await waitFor(() => expect(removeBundle).toHaveBeenCalledTimes(1));
    expect(removeBundle).toHaveBeenCalledWith({
      bundleId: "b1",
      seen: { fingerprint: "fp-4", cancel_ids: [waiting.id] },
    });
    expect(await screen.findByText(/The give Sam queued 2h ago is cancelled, so nothing goes to Zitadel/)).toBeInTheDocument();
  });

  it("a revoke whose plan cancels something pushes the step at press: Cancel it and revoke", async () => {
    wire.plan = cancelling([queuedChange({ id: waiting.id, user_id: "u_2f81", project_id: "pLaser" })]);
    removeDirect.mockResolvedValueOnce({});
    open({ sources: [direct], grantId: "g_88" });
    await planRead();
    expect(screen.getByText("Revoking it cancels the removal Sam Lee queued 2h ago.")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Revoke access" }));
    expect(await screen.findByRole("heading", { name: "This cancels a removal that's waiting" })).toBeInTheDocument();
    expect(removeDirect).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole("button", { name: "Cancel it and revoke" }));
    await waitFor(() => expect(removeDirect).toHaveBeenCalledTimes(1));
    expect(removeDirect.mock.calls[0][0].seen).toEqual({ fingerprint: "fp-1", cancel_ids: [waiting.id] });
  });

  it("a revoke whose plan cancels nothing pushes no step", async () => {
    removeDirect.mockResolvedValueOnce({});
    open({ sources: [direct], grantId: "g_88" });
    await planRead();
    fireEvent.click(screen.getByRole("button", { name: "Revoke access" }));
    await waitFor(() => expect(removeDirect).toHaveBeenCalledTimes(1));
    expect(screen.queryByRole("heading", { name: /This cancels/ })).not.toBeInTheDocument();
  });
});

describe("a give that never reached Zitadel: the dialog is the cancel", () => {
  const waitingGive = queuedChange({
    id: "00000000-0000-0000-0000-00000000000b",
    op_type: "add",
    user_id: "u_2f81",
    project_id: "pLaser",
    role_keys: ["trained"],
    source: "direct",
    initiated_by_name: "Sam Iyer",
    queued_at: "2026-10-12T08:00:00Z",
  });
  const cancelOnly = () => cancelling([waitingGive], { delivery: "none", fingerprint: "fp-c" });

  it("says so plainly, with who gave it and when, and names no revoke", async () => {
    wire.plan = cancelOnly();
    open({ sources: [{ ...direct, queued: true }], grantId: "g_88" });
    await planRead("Cancel the waiting give");

    expect(screen.getByText("Given to Tomas on 12 Oct by Sam Iyer, and still waiting for you to send.")).toBeInTheDocument();
    expect(screen.getByText("Nothing to take back.")).toBeInTheDocument();
    expect(
      screen.getByText("It hasn’t reached Zitadel, so Tomas never had it. This cancels that give and sends nothing."),
    ).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Revoke access" })).toBeNull();
    expect(screen.queryByRole("button", { name: "Withdraw the queued grant" })).toBeNull();
  });

  it("leaves out the by-part when the plan's cancel doesn't name who queued it", async () => {
    wire.plan = cancelling([{ ...waitingGive, initiated_by_name: undefined }], { delivery: "none" });
    open({ sources: [{ ...direct, queued: true }], grantId: "g_88" });
    await planRead("Cancel the waiting give");
    expect(screen.getByText("Given to Tomas on 12 Oct, and still waiting for you to send.")).toBeInTheDocument();
  });

  it("the press is the cancel itself: no step, the word Cancelled, the cancel-only sentence, nothing for you to do", async () => {
    wire.plan = cancelOnly();
    removeDirect.mockResolvedValueOnce({});
    open({ sources: [{ ...direct, queued: true }], grantId: "g_88" });
    await planRead("Cancel the waiting give");
    fireEvent.click(screen.getByRole("button", { name: "Cancel the waiting give" }));

    expect(await screen.findByText("Cancelled")).toBeInTheDocument();
    expect(screen.getByText(/^The give of .+ in Laser Lab that was waiting is cancelled\. Tomas never had it, so nothing goes to Zitadel\.$/)).toBeInTheDocument();
    expect(screen.getByText("Nothing for you to do.")).toBeInTheDocument();
    // The dialog was already the cancel: no step was pushed over it.
    expect(screen.queryByRole("heading", { name: /This cancels/ })).not.toBeInTheDocument();
    expect(removeDirect).toHaveBeenCalledWith({
      userId: "u_2f81",
      grantId: "g_88",
      seen: { fingerprint: "fp-c", cancel_ids: [waitingGive.id] },
    });
    expect(screen.queryByRole("button", { name: "Cancel the waiting give" })).toBeNull();
  });

  it("a bundle that was never sent: Cancel the waiting change, and the bundle's cancel-only sentence", async () => {
    wire.plan = cancelling([{ ...waitingGive, source: "bundle" }], { delivery: "none", fingerprint: "fp-b" });
    removeBundle.mockResolvedValueOnce({ cascade: { enqueued: 0 } });
    open({ sources: [{ ...bundle, queued: true }] });
    await planRead("Cancel the waiting change");
    expect(screen.queryByRole("button", { name: "Withdraw the queued change" })).toBeNull();
    expect(screen.queryByRole("button", { name: "Remove bundle" })).toBeNull();

    fireEvent.click(screen.getByRole("button", { name: "Cancel the waiting change" }));
    expect(await screen.findByText("Cancelled")).toBeInTheDocument();
    expect(
      screen.getByText("The Lab Tech bundle that was waiting is cancelled. Nothing it carried had reached Tomas, so nothing goes to Zitadel."),
    ).toBeInTheDocument();
    expect(screen.getByText("Nothing for you to do.")).toBeInTheDocument();
    expect(screen.queryByRole("heading", { name: /This cancels/ })).not.toBeInTheDocument();
    expect(removeBundle).toHaveBeenCalledWith({
      bundleId: "b1",
      seen: { fingerprint: "fp-b", cancel_ids: [waitingGive.id] },
    });
  });
});

describe("when their access moved while the dialog was open", () => {
  const stale = () => new ApiError(409, { error: "PLAN_STALE", message: "Moved." });

  it("revoke: nothing was changed, the notice names the button's own label, and the plan is read again", async () => {
    removeDirect.mockRejectedValueOnce(stale());
    open({ sources: [direct], grantId: "g_88" });
    await planRead();
    const reads = wire.calls.length;
    wire.plan = actPlan({ fingerprint: "fp-new" });
    fireEvent.click(screen.getByRole("button", { name: "Revoke access" }));

    expect(await screen.findByText("Nothing was changed. Tomas's access moved while this was open.")).toBeInTheDocument();
    expect(screen.getByText("Read the note under the form, then press Revoke access again.")).toBeInTheDocument();
    await waitFor(() => expect(wire.calls.length).toBeGreaterThan(reads));
    // Still the form, with its button: not a result.
    expect(screen.getByRole("button", { name: "Revoke access" })).toBeInTheDocument();
  });

  it("the cancel-only dialog names its own label in the notice", async () => {
    wire.plan = cancelling([queuedChange({ op_type: "add" })], { delivery: "none" });
    removeDirect.mockRejectedValueOnce(stale());
    open({ sources: [{ ...direct, queued: true }], grantId: "g_88" });
    await planRead("Cancel the waiting give");
    fireEvent.click(screen.getByRole("button", { name: "Cancel the waiting give" }));
    expect(await screen.findByText("Read the note under the form, then press Cancel the waiting give again.")).toBeInTheDocument();
  });

  it("bundle removal: the same notice, with Remove bundle", async () => {
    removeBundle.mockRejectedValueOnce(stale());
    open({ sources: [bundle] });
    await planRead("Remove bundle");
    fireEvent.click(screen.getByRole("button", { name: "Remove bundle" }));
    expect(await screen.findByText("Read the note under the form, then press Remove bundle again.")).toBeInTheDocument();
  });
});
