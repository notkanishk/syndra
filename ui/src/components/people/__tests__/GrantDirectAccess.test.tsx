// @vitest-environment jsdom
import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { GIVE_ROLE_LABEL, GrantDirectAccess } from "@/components/people/GrantDirectAccess";
import { ApiError } from "@/lib/api-client";
import { waitsBeforeLine } from "@/components/states/useWaitsBeforeLine";
import { actPlan, cancelling, queuedChange, withPlanClient } from "@/test-utils/plan";
import type { ActPlan } from "@/lib/usePlanConsent";

/**
 * The direct-grant endpoint always enqueues to the outbox for the
 * operator-triggered drain — it never applies inline from this dialog. A
 * result reported as "given" here would tell the operator Zitadel has the
 * grant when only Syndra's own ledger does.
 */

const state = vi.hoisted(() => ({
  down: false,
  mutateAsync: vi.fn(),
  grants: [] as unknown[],
  queued: [] as string[],
  queuedAt: undefined as string | undefined,
  terms: undefined as Array<{ name: string; start: string; end: string }> | undefined,
  // What the planner answers (GET /users/{id}/plan), and the reads it was asked.
  plan: undefined as unknown as ActPlan,
  planFails: false,
  planCalls: [] as string[],
}));

vi.mock("@/components/states/DegradedBanner", () => ({ useDegraded: () => state.down }));

vi.mock("@/lib/api-client", async (original) => ({
  ...(await original<typeof import("@/lib/api-client")>()),
  request: async (path: string) => {
    // The waits-before line asks whether Zitadel's queue drains on its own.
    if (path === "/governance/indicators") return { auto_retry: true };
    if (!path.includes("/plan?")) throw new Error(`unmocked request ${path}`);
    state.planCalls.push(path);
    if (state.planFails) throw new Error("plan unreadable");
    return state.plan;
  },
}));

vi.mock("@/lib/queries/useTerms", () => ({
  useTerms: () => ({ data: state.terms }),
}));

vi.mock("@/lib/queries/useProjects", () => ({
  useProjects: () => ({
    data: [{ project: { id: "p1", name: "Printing Lab" } }, { project: { id: "p2", name: "Empty Lab" } }],
  }),
}));

vi.mock("@/lib/queries/useRoles", () => ({
  useGlobalRoleCatalog: () => ({
    data: [
      {
        project_id: "p1",
        role_key: "operator",
        project_name: "Printing Lab",
        display_name: "Operator",
        description: "Runs the printers",
        assigned_user_count: 3,
        observed_user_count: 4,
      },
      {
        project_id: "p1",
        role_key: "supervisor",
        project_name: "Printing Lab",
        display_name: "Supervisor",
        assigned_user_count: 1,
        observed_user_count: 1,
        waits_for_send: true,
      },
    ],
  }),
}));

vi.mock("@/lib/queries/useUsers", () => ({
  useCreateGrant: () => ({ mutateAsync: state.mutateAsync, isPending: false }),
  useUserGrants: () => ({ data: state.grants }),
  useUserAccess: () => ({
    data: {
      projects: [
        {
          project_id: "p1",
          observed_role_keys: [],
          source_roles: state.queued.map((key) => ({ role_key: key, reasons: [{ kind: "direct", description: "", queued: true, queued_at: state.queuedAt }] })),
          derived_roles: [],
        },
      ],
    },
  }),
}));

beforeEach(() => {
  state.down = false;
  state.grants = [];
  state.queued = [];
  state.queuedAt = undefined;
  state.terms = undefined;
  state.plan = actPlan();
  state.planFails = false;
  state.planCalls = [];
  state.mutateAsync.mockReset();
});

function open() {
  return render(withPlanClient(<GrantDirectAccess userId="u1" userName="Charlotte Flair" open onClose={vi.fn()} />));
}

function choose(role: string) {
  const dialog = screen.getByRole("dialog");
  if (!within(dialog).queryByRole("radio", { name: new RegExp(role) })) {
    fireEvent.click(within(dialog).getByRole("button", { name: /Printing Lab/ }));
  }
  fireEvent.click(within(dialog).getByRole("radio", { name: new RegExp(role) }));
}

// The plan is read once a role is chosen; the press waits on it (A-21 §3).
async function planRead(role = "Operator") {
  await waitFor(() => expect(screen.getByRole("button", { name: `Give ${role}` })).toBeEnabled());
}

async function grant(role = "Operator") {
  choose(role);
  await planRead(role);
  fireEvent.change(screen.getByPlaceholderText(/Capstone build/), { target: { value: "test" } });
  fireEvent.click(screen.getByRole("button", { name: `Give ${role}` }));
  // "Close" only replaces "Give {Role}"/"Cancel" once an outcome (either kind)
  // is set — a wait condition that holds regardless of which message the
  // outcome carries.
  await screen.findByRole("button", { name: "Close" });
}

describe("GrantDirectAccess · choosing the role", () => {
  it("is the role picker in single choice, with the operator's notes", () => {
    open();
    expect(screen.queryByRole("combobox")).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: /Printing Lab/ }));
    const row = screen.getByRole("radio", { name: /Operator/ }).closest("label")!;
    expect(within(row).getByText("operator")).toBeInTheDocument();
    expect(within(row).getByText("4 people")).toBeInTheDocument();
    expect(within(row).getByText("Runs the printers")).toBeInTheDocument();
    expect(screen.getByRole("img", { name: "Waits for you" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /Empty Lab/ })).toHaveTextContent("Nothing yet");
  });

  it("says a role already held with no end changes nothing, and will not send it", () => {
    state.grants = [{ id: "g1", project_id: "p1", role_key: "operator", expires_at: null }];
    open();
    fireEvent.click(screen.getByRole("button", { name: /Printing Lab/ }));
    expect(screen.getByText("Has this")).toBeInTheDocument();
    expect(screen.getByRole("radio", { name: /Operator/ })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Give a role" })).toBeDisabled();
  });

  it("a role given and not yet sent says so, with the way to send it", () => {
    state.queued = ["operator"];
    open();
    fireEvent.click(screen.getByRole("button", { name: /Printing Lab/ }));
    expect(screen.getByText("Given")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /Waiting for you to send/ })).toHaveAttribute("href", expect.stringMatching(/^\/governance\/pending\?role=/));
    expect(screen.getByRole("radio", { name: /Operator/ })).toBeDisabled();
  });

  it("says when it was given, from the time the read carries, beside the way to send it", () => {
    state.queued = ["operator"];
    state.queuedAt = new Date(Date.now() - 2 * 3_600_000).toISOString();
    open();
    fireEvent.click(screen.getByRole("button", { name: /Printing Lab/ }));
    expect(screen.getByText("Given 2h ago")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /Waiting for you to send/ })).toBeInTheDocument();
  });

  it("a role given with an end but not yet sent is on its way, not held until a date", () => {
    // Found on the dev box: an approved request's grant has an end and waits in the
    // outbox; the row said "Has this until 31 Dec · Extend" for access not yet in Zitadel.
    state.grants = [{ id: "g1", project_id: "p1", role_key: "operator", expires_at: "2026-12-18T12:00:00Z" }];
    state.queued = ["operator"];
    state.queuedAt = new Date(Date.now() - 60_000).toISOString();
    open();
    fireEvent.click(screen.getByRole("button", { name: /Printing Lab/ }));
    expect(screen.getByText(/^Given/)).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /Waiting for you to send/ })).toBeInTheDocument();
    expect(screen.queryByText(/Has this until/)).toBeNull();
    expect(screen.queryByRole("link", { name: /Extend/ })).toBeNull();
  });

  it("a role held with an end is listed, not pickable, and offers Extend", () => {
    state.grants = [{ id: "g1", project_id: "p1", role_key: "operator", expires_at: "2026-12-18T12:00:00Z" }];
    open();
    fireEvent.click(screen.getByRole("button", { name: /Printing Lab/ }));
    expect(screen.getByText("Has this until 18 Dec")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /Extend/ })).toBeInTheDocument();
    expect(screen.getByRole("radio", { name: /Operator/ })).toBeDisabled();
  });

  it("says before the press that a waiting role waits, and a plain one doesn't", () => {
    open();
    choose("Operator");
    expect(screen.getByText("Charlotte can use it as soon as you give it.")).toBeInTheDocument();
    choose("Supervisor");
    expect(
      screen.getByText("Supervisor always waits for you, so Charlotte gets it once you send it."),
    ).toBeInTheDocument();
  });

  it("names the project and role, never the bare id", () => {
    open();
    expect(screen.getByText("Printing Lab")).toBeInTheDocument();
    expect(screen.queryByText("p1")).not.toBeInTheDocument();
  });
});

describe("GrantDirectAccess · End of term", () => {
  beforeEach(() => vi.useFakeTimers({ toFake: ["Date"], now: new Date(2026, 8, 1, 12) }));
  afterEach(() => vi.useRealTimers());

  it("is the end of the term holding today, from the term setting", () => {
    state.terms = [{ name: "Monsoon", start: "2026-08-03", end: "2026-11-27" }];
    open();
    fireEvent.click(screen.getByRole("radio", { name: "End of term" }));
    expect(screen.getByText("27 Nov")).toBeInTheDocument();
  });

  it("falls back to 18 December when the setting can't be read", () => {
    open();
    fireEvent.click(screen.getByRole("radio", { name: "End of term" }));
    expect(screen.getByText("18 Dec")).toBeInTheDocument();
  });
});

describe("GrantDirectAccess · its result", () => {
  it("a role that always waits is Given, amber, with why and what that means for them", async () => {
    state.mutateAsync.mockResolvedValue({ outbox_id: "o1", status: "pending", wait_reason: "role_waits" });
    open();
    await grant("Supervisor");

    expect(screen.getByText("Given")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /Waiting for you to send/ })).toHaveAttribute("href", "/governance/pending?person=u1");
    expect(screen.getByText("Supervisor always waits for you, so Charlotte can't use it yet.")).toBeInTheDocument();
    expect(screen.queryByText("Applied")).not.toBeInTheDocument();
  });

  it("a send that stopped is queued with the stop named, and goes when sent", async () => {
    state.mutateAsync.mockResolvedValue({ outbox_id: "o1", status: "pending", wait_reason: "halted:zitadel_offline" });
    open();
    await grant();

    expect(screen.getByText("Given")).toBeInTheDocument();
    expect(
      screen.getByText(/^Zitadel didn't answer at \d\d:\d\d, so it goes when you send it\.$/),
    ).toBeInTheDocument();
  });

  // Final verification N1: Zitadel refused it outright. It used to read "It
  // goes when you send it", with a Send link Send could never honour.
  it("a role Zitadel refused says so, with why and the true step, and no Send link", async () => {
    state.mutateAsync.mockResolvedValue({ outbox_id: "o1", status: "failed", wait_reason: "refused:not_found" });
    open();
    await grant();

    expect(screen.getByText("Refused by Zitadel")).toBeInTheDocument();
    expect(screen.getByText(/Zitadel refused this: it no longer has this person, project or role/)).toBeInTheDocument();
    expect(screen.getByText(/give it again once the reason is fixed/)).toBeInTheDocument();
    expect(screen.queryByText(/goes when you send it/)).not.toBeInTheDocument();
    expect(screen.queryByRole("link", { name: /Waiting for you to send/ })).not.toBeInTheDocument();
    expect(screen.queryByText(/Nothing was changed/)).not.toBeInTheDocument();
    // Given in Syndra: the sheet is its result, and nothing gives twice.
    expect(screen.queryByRole("button", { name: "Give Operator" })).not.toBeInTheDocument();
  });

  it("reports Given, in their first name, when nothing waits", async () => {
    state.mutateAsync.mockResolvedValue({ outbox_id: "o1", status: "applied" });
    open();
    await grant();

    expect(screen.getByText("Given")).toBeInTheDocument();
    expect(screen.getByText("Charlotte can use Operator on their own now.")).toBeInTheDocument();
    expect(screen.queryByText("Applied")).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Give Operator" })).not.toBeInTheDocument();
  });

  // Audit C2: the person giving one role sees, before the click, every role an
  // automatic rule also gives with it, and the result says they went with it.
  it("names what automatic rules also give, before the press and in its result", async () => {
    state.plan = actPlan({ rules: ["Also gives Bench in Workshop, by the automatic rule from Operator."] });
    state.mutateAsync.mockResolvedValue({ outbox_id: "o1", status: "applied", rule_outbox_ids: ["o2"] });
    open();
    choose("Operator");
    expect(await screen.findByText("Also gives Bench in Workshop, by the automatic rule from Operator.")).toBeInTheDocument();
    await planRead();
    fireEvent.click(screen.getByRole("button", { name: "Give Operator" }));
    expect(await screen.findByText("Charlotte can use Operator on their own now.")).toBeInTheDocument();
    expect(screen.getByText(/Also gives Bench in Workshop/)).toBeInTheDocument();
  });

  // One consent rule with Approve (usePlanConsent): a plan that can't be read
  // holds the press, with the reason and a Try again, never a press with a warning.
  it("holds the press while the plan is read, and when it can't be, and Try again reads it again", async () => {
    state.planFails = true;
    open();
    choose("Operator");
    expect(screen.getByRole("button", { name: "Give Operator" })).toBeDisabled();
    expect(screen.getByText("Checking what giving it does.")).toBeInTheDocument();
    expect(await screen.findByText("Held until Syndra can read what giving it does.")).toBeInTheDocument();
    expect(
      screen.getByText(
        "Syndra couldn't read what giving it does, so it can't show what else this would give or cancel. Try again to read it.",
      ),
    ).toBeInTheDocument();
    const reads = state.planCalls.length;
    state.planFails = false;
    fireEvent.click(screen.getByRole("button", { name: "Try again" }));
    await planRead();
    expect(state.planCalls.length).toBeGreaterThan(reads);
    expect(state.mutateAsync).not.toHaveBeenCalled();
  });

  it("asks the planner for this person, project and role, and reads no rule preview", async () => {
    open();
    choose("Operator");
    await planRead();
    expect(state.planCalls).toEqual(["/users/u1/plan?op=give&project_id=p1&role_key=operator"]);
  });

  it("says nothing about rules when no rule starts from the role", async () => {
    open();
    choose("Operator");
    await planRead();
    expect(screen.queryByText(/Also gives/)).not.toBeInTheDocument();
  });

  it("a refusal is Not given, and keeps the form", async () => {
    state.mutateAsync.mockRejectedValue(new ApiError(409, { error: "ALREADY_HELD", message: "They already have this." }));
    open();
    choose("Operator");
    await planRead();
    fireEvent.click(screen.getByRole("button", { name: "Give Operator" }));
    expect(await screen.findByText("Not given")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Give Operator" })).toBeEnabled();
  });
});

// A-07 §9: a person's page stays on while Zitadel is down, and says before the
// operator acts that the change will wait.
describe("GrantDirectAccess · while Zitadel is not answering", () => {
  it("says before the grant that it will wait, instead of promising it works at once", async () => {
    state.down = true;
    open();
    choose("Operator");
    await planRead();
    const dialog = screen.getByRole("dialog");
    expect(
      within(dialog).getByText(waitsBeforeLine("give")),
    ).toBeInTheDocument();
    expect(within(dialog).queryByText(/as soon as you give it/)).not.toBeInTheDocument();
    expect(within(dialog).getByRole("button", { name: "Give Operator" })).toBeEnabled();
  });
});

// A-17 Q7: the owner's rename is one line, so the dialog title reads the constant.
describe("GrantDirectAccess · wording and the end date", () => {
  it("titles itself with the one exported label", () => {
    open();
    expect(screen.getByRole("heading", { name: GIVE_ROLE_LABEL })).toBeInTheDocument();
  });

  it("says the end date once: the control, and one footer note (no caption above it)", () => {
    open();
    choose("Operator");
    const dialog = screen.getByRole("dialog");
    expect(within(dialog).getAllByText(/Expiring access/)).toHaveLength(1);
    expect(within(dialog).queryByText(/It ends on/)).not.toBeInTheDocument();
    fireEvent.click(within(dialog).getByRole("radio", { name: "Never" }));
    expect(within(dialog).getByText("It stays until someone removes it.")).toBeInTheDocument();
    expect(within(dialog).getByText("No end date")).toBeInTheDocument();
  });

  it("carries the chosen role on the button, and the date line sits under Ends, not in the footer note", () => {
    open();
    const dialog = screen.getByRole("dialog");
    expect(within(dialog).getByRole("button", { name: "Give a role" })).toBeDisabled();
    choose("Operator");
    expect(within(dialog).getByRole("button", { name: "Give Operator" })).toBeInTheDocument();
    const hint = within(dialog).getByText(/The sweep removes it on that date/);
    expect(hint.tagName).toBe("P");
    expect(hint.closest("[data-modal-footer], footer")).toBeNull();
  });
});

// A-21 §3: the plan is read before the press, so the sheet foretells what
// giving it cancels and the press shows it at once: no write is sent to find out.
describe("GrantDirectAccess · what the plan says before the press", () => {
  const removal = queuedChange();

  it("foretells the cancel in the footer, before anything is chosen on the next step", async () => {
    state.plan = cancelling([removal]);
    open();
    choose("Operator");
    expect(
      await screen.findByText(
        /^Giving it cancels removing .+ from .+, which was waiting for you to send\. Nothing is cancelled until you choose on the next step\.$/,
      ),
    ).toBeInTheDocument();
    expect(state.mutateAsync).not.toHaveBeenCalled();
  });

  it("counts several waiting changes in the one sentence", async () => {
    state.plan = cancelling([removal, queuedChange({ id: "00000000-0000-0000-0000-0000000000a2" })]);
    open();
    choose("Operator");
    expect(await screen.findByText(/Giving it cancels 2 changes that are waiting\./)).toBeInTheDocument();
  });

  it("pushes the step at once from the plan, sends no write first, and then carries the plan", async () => {
    state.plan = cancelling([removal], { fingerprint: "fp-9" });
    state.mutateAsync.mockResolvedValue({ outbox_id: "o1", status: "applied" });
    open();
    choose("Operator");
    await planRead();
    fireEvent.click(screen.getByRole("button", { name: "Give Operator" }));

    const step = await screen.findByRole("heading", { name: /^Cancel removing .+\?$/ });
    expect(step).toBeInTheDocument();
    // Nothing was written to learn this: the step came from the plan.
    expect(state.mutateAsync).not.toHaveBeenCalled();

    fireEvent.click(screen.getByRole("button", { name: "Cancel removing it and give Operator" }));
    await screen.findByRole("button", { name: "Close" });
    expect(state.mutateAsync).toHaveBeenCalledTimes(1);
    expect(state.mutateAsync.mock.calls[0][0]).toMatchObject({
      project_id: "p1",
      role_key: "operator",
      fingerprint: "fp-9",
      cancel_ids: [removal.id],
    });
  });

  it("Keep it waiting sends nothing and leaves the form as it was", async () => {
    state.plan = cancelling([removal]);
    open();
    choose("Operator");
    await planRead();
    fireEvent.click(screen.getByRole("button", { name: "Give Operator" }));
    fireEvent.click(await screen.findByRole("button", { name: "Keep it waiting" }));
    expect(state.mutateAsync).not.toHaveBeenCalled();
    await waitFor(() => expect(screen.getByRole("button", { name: "Give Operator" })).toBeVisible());
  });

  it("gains no step when the plan cancels nothing, and the write still carries the fingerprint", async () => {
    state.plan = actPlan({ fingerprint: "fp-3" });
    state.mutateAsync.mockResolvedValue({ outbox_id: "o1", status: "applied" });
    open();
    await grant();
    expect(screen.queryByText(/This cancels/)).not.toBeInTheDocument();
    expect(screen.queryByText(/Giving it cancels/)).not.toBeInTheDocument();
    expect(state.mutateAsync.mock.calls[0][0]).toMatchObject({ fingerprint: "fp-3", cancel_ids: [] });
  });

  it("when their access moved: nothing was changed, the plan is read again, and the note says so", async () => {
    state.plan = actPlan({ fingerprint: "fp-1" });
    state.mutateAsync.mockRejectedValueOnce(new ApiError(409, { error: "PLAN_STALE", message: "Moved." }));
    open();
    choose("Operator");
    await planRead();
    const reads = state.planCalls.length;
    // What the next read finds.
    state.plan = cancelling([removal], { fingerprint: "fp-2" });
    fireEvent.click(screen.getByRole("button", { name: "Give Operator" }));

    expect(await screen.findByText("Nothing was changed. Charlotte's access moved while this was open.")).toBeInTheDocument();
    expect(screen.getByText("Read the note under the form, then press Give Operator again.")).toBeInTheDocument();
    await waitFor(() => expect(state.planCalls.length).toBeGreaterThan(reads));
    expect(await screen.findByText(/Giving it cancels removing .+ from .+, which was waiting for you to send\./)).toBeInTheDocument();
    // The form is still here, not a result.
    expect(screen.queryByText("Not given")).not.toBeInTheDocument();
  });

  it("when the planner refuses because they already have it: held, with the reason and nothing to press", async () => {
    state.plan = actPlan({ effect: "no_change", refusal: "held", detail: "Charlotte already has Operator." });
    open();
    choose("Operator");
    expect(await screen.findByText("Nothing to give.")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Give Operator" })).toBeDisabled();
    expect(screen.getByText("Charlotte already has Operator.")).toBeInTheDocument();
  });
});
