// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { BulkDialog } from "@/components/people/BulkDialog";
import type { BulkPlan } from "@/lib/queries/useBulkGrants";

const api = vi.hoisted(() => ({
  rehearsed: [] as unknown[],
  applied: [] as unknown[],
  plan: null as BulkPlan | null,
}));

vi.mock("@/lib/queries/useBulkGrants", async () => {
  const actual = await vi.importActual<typeof import("@/lib/queries/useBulkGrants")>(
    "@/lib/queries/useBulkGrants",
  );
  return {
    ...actual,
    useRehearseBulk: () => ({
      isPending: false,
      mutateAsync: async (input: unknown) => {
        api.rehearsed.push(input);
        return api.plan;
      },
    }),
    useApplyBulk: () => ({
      isPending: false,
      mutateAsync: async (input: unknown) => {
        api.applied.push(input);
        return { ...api.plan!, plan_id: undefined, applied: true };
      },
    }),
  };
});

vi.mock("@/lib/queries/useProjects", () => ({
  useProjects: () => ({
    data: [
      { project: { id: "pLaser", name: "Laser Lab", roles: [{ key: "trained", label: "Trained" }] } },
    ],
  }),
}));

vi.mock("@/lib/queries/useBundles", () => ({
  useBundles: () => ({ data: [{ id: "b1", name: "Safety" }] }),
}));


function plan(overrides: Partial<BulkPlan> = {}): BulkPlan {
  return {
    op: "assign_role",
    applied: false,
    outcomes: [
      {
        user_id: "u1",
        name: "Ada Lovelace",
        email: "ada@example.edu",
        effect: "apply",
        detail: "Gains Member in Laser Lab.",
        delivery: "straight_away",
        waits: [],
        goes: ["Member in Laser Lab"],
      },
      {
        user_id: "u2",
        name: "Leo Brooks",
        email: "leo@example.edu",
        effect: "blocked",
        detail: "Account is departed. Remove it from the selection to continue.",
      },
      {
        user_id: "u3",
        name: "Sam Patel",
        email: "sam@example.edu",
        effect: "no_change",
        detail: "Holds no direct grant here.",
        consequence: "Keeps the role another way, through a bundle or an automatic rule. Remove that instead.",
      },
    ],
    // A rehearsal now returns the approval it became; the apply cites it, and
    // the button stays disabled without one.
    plan_id: "plan_1",
    summary: { total: 3, apply: 1, no_change: 1, blocked: 1, failed: 0, succeeded: 0, queued: 0 },
    ...overrides,
  };
}

function open(op: Parameters<typeof BulkDialog>[0]["op"] = "assign_role", onClose: (applied?: boolean) => void = () => {}) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <BulkDialog op={op} userIds={["u1", "u2", "u3"]} scope="in Laser Lab" onClose={onClose} />
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  api.rehearsed = [];
  api.applied = [];
  api.plan = plan();
});

describe("BulkDialog", () => {
  it("cannot reach apply without rehearsing first", async () => {
    open();
    // The only affirmative control on the first step is Rehearse. There is no
    // path from a selection straight to a write, for any operation.
    expect(screen.queryByRole("button", { name: /^Apply/ })).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Preview the change" })).toBeInTheDocument();
    expect(api.applied).toHaveLength(0);
  });

  it("requires a reason before it will even rehearse", () => {
    open();
    // Every row of a bulk write lands in the audit log; an unexplained one is
    // a change nobody can account for later.
    expect(screen.getByRole("button", { name: "Preview the change" })).toBeDisabled();
  });

  // The role picker, never a native select: a project's roles sit in its fold.
  function chooseRole() {
    const dialog = screen.getByRole("dialog");
    if (!within(dialog).queryByRole("radio", { name: /Trained/ })) {
      fireEvent.click(within(dialog).getByRole("button", { name: /Laser Lab/ }));
    }
    fireEvent.click(within(dialog).getByRole("radio", { name: /Trained/ }));
  }

  async function rehearse() {
    chooseRole();
    fireEvent.change(screen.getByLabelText("Reason"), { target: { value: "New cohort" } });
    fireEvent.click(screen.getByRole("button", { name: "Preview the change" }));
    await screen.findByText("Ada Lovelace");
  }

  it("shows the server's plan verbatim, naming every person", async () => {
    open();
    await rehearse();

    // Names, not ids: a plan identified by id is a plan nobody can check.
    expect(screen.getByText("Ada Lovelace")).toBeInTheDocument();
    expect(screen.getByText("Leo Brooks")).toBeInTheDocument();
    expect(screen.getByText(/Account is departed/)).toBeInTheDocument();
    // The consequence — who keeps the role anyway — is the part a count hides.
    expect(screen.getByText(/Keeps the role another way, through a bundle or an automatic rule/)).toBeInTheDocument();
  });

  it("counts only the rows that will actually change on the confirm button", async () => {
    open();
    await rehearse();
    // 3 selected, 1 actionable. Offering "Apply to 3" would be a lie.
    expect(screen.getByRole("button", { name: "Apply to 1 person" })).toBeInTheDocument();
    // The folds carry the counts the retired note did (A-21 §2.5), and the
    // footer says whether it finishes now.
    expect(screen.getByRole("group", { name: "Straight away, 1 person" })).toBeInTheDocument();
    expect(screen.getByRole("group", { name: "Left as they are, 2 people" })).toBeInTheDocument();
    expect(screen.getByText("Every change goes to Zitadel when you apply.")).toBeInTheDocument();
    expect(screen.queryByText(/No change for 1/)).toBeNull();
  });

  it("applies the request it rehearsed, citing the approval that rehearsal became", async () => {
    open();
    await rehearse();
    fireEvent.click(screen.getByRole("button", { name: "Apply to 1 person" }));
    await waitFor(() => expect(api.applied).toHaveLength(1));
    // The body is the same operation, and it now names the approval. The
    // backend binds the two, so a body that drifted from the reviewed one is
    // refused rather than applied under an approval it does not belong to.
    //
    // `acknowledge_scope` is the one field that does not travel: it unlocks
    // issuing the approval, it does not change what the approval does, so it
    // is deliberately outside the binding and outside the apply.
    const { acknowledge_scope: _ack, ...rehearsed } = api.rehearsed[0] as Record<string, unknown>;
    expect(api.applied[0]).toEqual({ ...rehearsed, plan_id: "plan_1" });
  });

  // A-19 P10.2 (Q9): the caller ends Select only when the dialog closes on its result.
  it("tells its caller it closed on a result, so Select can end", async () => {
    const closed: Array<boolean | undefined> = [];
    open("assign_role", (applied) => closed.push(applied));
    await rehearse();
    fireEvent.click(screen.getByRole("button", { name: "Apply to 1 person" }));
    await waitFor(() => expect(api.applied).toHaveLength(1));
    fireEvent.click(await screen.findByRole("button", { name: "Close" }));
    expect(closed).toEqual([true]);
  });

  it("tells its caller it closed without applying when it is cancelled", () => {
    const closed: Array<boolean | undefined> = [];
    open("assign_role", (applied) => closed.push(applied));
    fireEvent.click(screen.getByRole("button", { name: "Cancel" }));
    expect(api.applied).toHaveLength(0);
    expect(closed).toEqual([false]);
  });

  it("cannot apply a rehearsal the backend did not record", async () => {
    api.plan = plan({ plan_id: undefined });
    open();
    await rehearse();
    // No approval, no apply. The alternative is an operator pressing a button
    // that can only fail.
    expect(screen.getByRole("button", { name: "Apply to 1 person" })).toBeDisabled();
  });

  it("refuses to apply when the rehearsal found nothing to do", async () => {
    api.plan = plan({
      summary: { total: 3, apply: 0, no_change: 3, blocked: 0, failed: 0, succeeded: 0, queued: 0 },
    });
    open();
    await rehearse();
    const button = screen.getByRole("button", { name: "Apply" });
    expect(button).toBeDisabled();
  });

  // RehearsalDialog's `notReadyReason` is rendered under the disabled Preview
  // button, but BulkDialog never passed one — `ready={false}` on its own greys
  // the only control on screen and says nothing about which of five gaps is
  // open. Each is checked with the others satisfied, so the visible reason
  // names the one gap actually left.
  describe("says which of the four gaps is open", () => {
    it("no role chosen (a role is picked with its project, so one gap covers both)", () => {
      open();
      fireEvent.change(screen.getByLabelText("Reason"), { target: { value: "New cohort" } });
      expect(screen.getByText("Choose a role.")).toBeInTheDocument();
    });

    it("no bundle chosen", () => {
      open("assign_bundle");
      fireEvent.change(screen.getByLabelText("Reason"), { target: { value: "New cohort" } });
      expect(screen.getByText("Choose a bundle.")).toBeInTheDocument();
    });

    it("extend duration is zero", () => {
      open("extend");
      fireEvent.change(screen.getByLabelText("Extend by (days)"), { target: { value: "0" } });
      fireEvent.change(screen.getByLabelText("Reason"), { target: { value: "New cohort" } });
      expect(screen.getByText(/Enter how many days to extend by/)).toBeInTheDocument();
    });

    it("reason left blank", () => {
      open();
      chooseRole();
      expect(screen.getByText(/Say why/)).toBeInTheDocument();
      expect(screen.getByText("Say why. Syndra keeps it with each person's change.")).toBeInTheDocument();
    });
  });

  it("lets an operator go back and change the target without losing the dialog", async () => {
    open();
    await rehearse();
    fireEvent.click(screen.getByRole("button", { name: "Back" }));
    expect(screen.getByRole("button", { name: "Preview the change" })).toBeInTheDocument();
    expect(screen.getByLabelText("Reason")).toHaveValue("New cohort");
  });

  // The summary line and Ada's own row are two sentences about the same
  // apply, computed from the same plan — this pins them to agreeing rather
  // than the summary claiming "applied" over a row still marked "queued".
  it("keeps the summary outcome and the per-row outcome saying the same thing, by name", async () => {
    open();
    await rehearse();
    api.plan = plan({
      outcomes: [
        {
          user_id: "u1",
          name: "Ada Lovelace",
          email: "ada@example.edu",
          effect: "queued",
          detail: "Waiting to be sent to Zitadel.",
        },
        api.plan!.outcomes![1],
        api.plan!.outcomes![2],
      ],
      summary: { total: 3, apply: 0, no_change: 1, blocked: 1, failed: 0, succeeded: 0, queued: 1 },
    });
    fireEvent.click(screen.getByRole("button", { name: "Apply to 1 person" }));
    await waitFor(() => expect(api.applied).toHaveLength(1));

    // Summary: queued outranks a clean apply, so the headline pill reads
    // Summary: queued outranks a clean apply, so the headline is the step
    // left, the link to Pending changes where the operator sends it from —
    // never "Applied" while a row is still queued.
    const summary = screen.getByRole("link", { name: /Waiting for you to send/ });
    expect(summary.getAttribute("href")).toBe("/governance/pending");
    // Ada's own row says the same words, so the two agree rather than one
    // saying "applied" over a row still marked queued.
    const sayings = screen.getAllByText("Waiting for you to send");
    expect(sayings.length).toBeGreaterThanOrEqual(2);
    expect(sayings.filter((el) => !el.closest("a"))).toHaveLength(1);
    expect(screen.queryByText("Applied")).not.toBeInTheDocument();
    // Named rather than by id.
    expect(screen.getByText("Ada Lovelace")).toBeInTheDocument();
    expect(screen.queryByText("u1")).not.toBeInTheDocument();
  });
});
