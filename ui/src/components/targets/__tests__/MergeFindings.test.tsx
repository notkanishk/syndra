// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, within } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { MergeFindings } from "@/components/targets/MergeFindings";
import type { MergeFinding, ResolveFindingInput } from "@/lib/queries/useTargets";

// `reconciliation-as-merge` — the surface for the differences a sweep refused
// to resolve.
//
// What each row has to say is WHAT IT USED TO BE. That is the question an
// operator asks first, it is what no surface could answer before the merge base
// existed, and a row that omits it is a row somebody has to go and read the
// target's own history for — which for most targets does not exist.

const state = {
  findings: [] as MergeFinding[],
  resolved: [] as ResolveFindingInput[],
  error: null as unknown,
  decideOnSave: false,
};

vi.mock("@/lib/queries/useTargets", async () => {
  const actual = await vi.importActual<typeof import("@/lib/queries/useTargets")>(
    "@/lib/queries/useTargets",
  );
  return {
    ...actual,
    useMergeFindings: () => ({
      data: state.findings,
      isLoading: false,
      error: null,
      refetch: vi.fn(),
    }),
    useResolveMergeFinding: () => ({
      mutate: (input: ResolveFindingInput, opts?: { onSuccess?: () => void }) => {
        state.resolved.push(input);
        // The refetch that follows a save: the finding is now decided, and waits for the system to match.
        if (state.decideOnSave) state.findings = state.findings.map((f) => ({ ...f, decision: input.resolution, decided_by: "op-1" }));
        opts?.onSuccess?.();
      },
      isPending: false,
      error: state.error,
    }),
  };
});

vi.mock("@/components/names", () => ({
  UserName: ({ id }: { id: string }) => <span>{id}</span>,
}));

function renderFindings() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <MergeFindings target="truenas" />
    </QueryClientProvider>,
  );
}

function finding(over: Partial<MergeFinding>): MergeFinding {
  return {
    id: "f1",
    target: "truenas",
    subject_id: "sub-1",
    field: "enabled",
    outcome: "theirs_only",
    base: true,
    ours: true,
    theirs: false,
    detected_at: "2026-08-18T12:00:00Z",
    last_seen_at: "2026-08-19T12:00:00Z",
    ...over,
  };
}

describe("differences waiting on a decision", () => {
  it("leads with what the value used to be", () => {
    state.findings = [finding({})];
    state.resolved = [];
    state.error = null;
    renderFindings();

    // The base, in the sentence, not behind a disclosure.
    expect(screen.getByText(/was true when Syndra last saw it/i)).toBeTruthy();
    expect(screen.getByText(/somebody changed it on TrueNAS/i)).toBeTruthy();
  });

  // The three kinds want different actions, so they must not render as one
  // "out of step" row with one button.
  it("says both moved when both moved", () => {
    state.findings = [finding({ outcome: "conflict", ours: true, theirs: false, base: "unknown" })];
    renderFindings();

    expect(screen.getByText(/both moved, differently/i)).toBeTruthy();
  });

  it("offers provisioning or unbinding for an account that is gone", () => {
    state.findings = [finding({ outcome: "deleted_upstream", field: undefined })];
    state.resolved = [];
    renderFindings();

    expect(screen.getByText(/no longer on TrueNAS/i)).toBeTruthy();
    fireEvent.click(screen.getByText("Decide"));
    expect(screen.getByText("Create it again")).toBeTruthy();
    expect(screen.getByText("Stop managing it")).toBeTruthy();
  });

  // A decision with no reason is a decision nobody can be asked about later,
  // and for an adopted value it becomes that person's policy.
  it("will not send a decision with no reason", () => {
    state.findings = [finding({})];
    state.resolved = [];
    renderFindings();

    fireEvent.click(screen.getByText("Decide"));
    const keep = screen.getByText("Keep Syndra's value") as HTMLButtonElement;
    expect(keep.disabled).toBe(true);

    fireEvent.change(screen.getByLabelText("Why"), {
      target: { value: "the suspension was a mistake" },
    });
    fireEvent.click(screen.getByText("Keep Syndra's value"));
    expect(state.resolved).toHaveLength(1);
    expect(state.resolved[0].resolution).toBe("keep_ours");
    expect(state.resolved[0].reason).toBe("the suspension was a mistake");
  });

  // An adopted suspension carries a bound in time. The schema refuses a denial
  // without one, and this surface must not be the way around it.
  it("adopts the target's value and leaves the review date to the backend", () => {
    state.findings = [finding({ adoptable: true })];
    state.resolved = [];
    renderFindings();

    fireEvent.click(screen.getByText("Decide"));
    fireEvent.change(screen.getByLabelText("Why"), { target: { value: "incident review" } });
    fireEvent.click(screen.getByText("Use TrueNAS's value"));

    expect(state.resolved[0].resolution).toBe("take_theirs");
    // Never an instant worked out here (the backend refuses one): absent, so the backend sets it.
    expect(state.resolved[0].review_date).toBeUndefined();
  });

  // A value with no per-person home gets no button at all. Offering one and
  // failing afterwards is worse than not offering it: the operator believes
  // they decided, and nothing happened.
  it("does not offer an adoption that cannot be expressed", () => {
    state.findings = [
      finding({
        field: "group",
        theirs: ["electronics"],
        adoptable: false,
        why_not:
          "group comes from this target's role mappings, which have no per-person form. Editing one changes it for every holder of that role.",
        policy: [
          { mapping_id: "m1", project_id: "p1", role_key: "lab_tech", value: "lab_makers", holders: 3 },
        ],
      }),
    ];
    state.error = null;
    renderFindings();

    fireEvent.click(screen.getByText("Decide"));
    expect(screen.queryByText("Use TrueNAS's value")).toBeNull();
    // And the alternative, with the mapping and how far editing it reaches.
    expect(screen.getByText(/every holder of that role/i)).toBeTruthy();
    expect(screen.getByText(/Syndra gives that role to 3 people/i)).toBeTruthy();
  });

  // A suspension the target made IS expressible — as a deny allowance — so the
  // button is there.
  it("offers adoption when the value has a per-subject home", () => {
    state.findings = [finding({ adoptable: true })];
    state.error = null;
    renderFindings();

    fireEvent.click(screen.getByText("Decide"));
    expect(screen.getByText("Use TrueNAS's value")).toBeTruthy();
  });

  // A decided finding is still standing: the convergence is queued and the
  // difference is still on the target. The row says what was chosen and what it
  // is waiting for, rather than offering the decision again — or claiming it is
  // over.
  it("shows a decided finding as waiting rather than offering it again", () => {
    state.findings = [
      finding({ decision: "keep_ours", decided_by: "op-1", decided_at: "2026-08-19T09:00:00Z" }),
    ];
    state.error = null;
    renderFindings();

    expect(screen.getByText(/waiting for TrueNAS to match/i)).toBeTruthy();
    expect(screen.queryByText("Decide")).toBeNull();
  });
});

// An unbind settles immediately when it works, so one still standing is a
// repair rather than a wait. Hiding its control is what turned a recoverable
// half-write into a wedge: the add-on had let go, the backend row remained, and
// every retry came back refused.
it("offers to finish an unbind that did not complete", () => {
  state.findings = [
    finding({
      outcome: "deleted_upstream",
      field: undefined,
      decision: "unbound",
      decided_by: "op-charlotte",
    }),
  ];
  state.resolved = [];
  state.error = null;
  renderFindings();

  expect(screen.getByText(/may already have let go of this account/i)).toBeTruthy();
  fireEvent.click(screen.getByText("Finish: stop managing it"));
  expect(state.resolved).toHaveLength(1);
  expect(state.resolved[0].resolution).toBe("unbound");
});

// How the fields' read stands: known, still being read, failed, or read with no accepted schema.
const schemaRead = vi.hoisted(() => ({ mode: "known" as "known" | "loading" | "failed" | "unread", refetch: vi.fn() }));
vi.mock("@/lib/queries/useFieldSchema", async () => {
  const m = (await import("@/lib/__fixtures__/field-schema-mock")).fieldSchemaMock;
  return {
    ...m,
    useFieldSchemaState: () => {
      const base = { loading: false, error: null as unknown, refetch: schemaRead.refetch };
      switch (schemaRead.mode) {
        case "known":
          return m.useFieldSchemaState();
        case "loading":
          return { ...base, schema: undefined, loading: true };
        case "failed":
          return { ...base, schema: undefined, error: new Error("roster down") };
        case "unread":
          return { ...base, schema: undefined };
      }
    },
  };
});

// 3.8: the target changed a person's limit. Taking its value saves it as THEIR OWN limit, which
// is policy for that person, so it asks for a reason and for the day it ends or is reviewed.
describe("taking the target's value for a limit", () => {
  const limitFinding = () =>
    finding({ field: "home_quota", outcome: "theirs_only", base: ["0", "5368709120"], ours: ["5368709120"], theirs: ["21474836480"], adoptable: true });

  function open() {
    state.findings = [limitFinding()];
    state.resolved = [];
    state.error = null;
    renderFindings();
    fireEvent.click(screen.getByText("Decide"));
    fireEvent.change(screen.getByLabelText("Why"), { target: { value: "Approved at the lab meeting" } });
    fireEvent.click(screen.getByText("Use TrueNAS's value"));
  }

  it("says plainly that it saves the value as this person's own limit, and sends nothing yet", () => {
    open();
    expect(screen.getByText(/saves TrueNAS's value, 20 GB, as this person's own home folder limit/)).toBeTruthy();
    expect(screen.getByText(/takes the place of what their roles give until it ends or you review it/)).toBeTruthy();
    expect(state.resolved).toHaveLength(0);
  });

  it("will not save without a day", () => {
    open();
    expect((screen.getByText("Save it as their own limit").closest("button") as HTMLButtonElement).disabled).toBe(true);
  });

  it("sends the reason and an end date", () => {
    open();
    fireEvent.change(screen.getByLabelText("Ends on"), { target: { value: "2027-03-01" } });
    fireEvent.click(screen.getByText("Save it as their own limit"));
    expect(state.resolved).toHaveLength(1);
    expect(state.resolved[0]).toMatchObject({ resolution: "take_theirs", reason: "Approved at the lab meeting" });
    expect(state.resolved[0].expires_at).toMatch(/^2027-03-01/);
    expect(state.resolved[0].review_date).toBeUndefined();
  });

  it("sends a review date instead when that is what was chosen", () => {
    open();
    fireEvent.click(screen.getByRole("radio", { name: "Review on" }));
    fireEvent.change(screen.getByLabelText("Review on"), { target: { value: "2027-03-01" } });
    fireEvent.click(screen.getByText("Save it as their own limit"));
    expect(state.resolved[0].review_date).toMatch(/^2027-03-01/);
    expect(state.resolved[0].expires_at).toBeUndefined();
  });

  it("shows the result where it was decided, in the outcome pattern, once it is saved", () => {
    state.decideOnSave = true;
    open();
    fireEvent.change(screen.getByLabelText("Ends on"), { target: { value: "2027-03-01" } });
    fireEvent.click(screen.getByText("Save it as their own limit"));
    state.decideOnSave = false;
    expect(screen.getByText("Saved")).toBeTruthy();
    expect(screen.getByText(/Their own home folder limit is now 20 GB\. It ends on 1 Mar/)).toBeTruthy();
    expect(screen.getByText(/waiting for TrueNAS to match/)).toBeTruthy();
  });

  it("keeps the one-press path for a switched-off account, which is not a limit", () => {
    state.findings = [finding({ adoptable: true })];
    state.resolved = [];
    renderFindings();
    fireEvent.click(screen.getByText("Decide"));
    fireEvent.change(screen.getByLabelText("Why"), { target: { value: "incident review" } });
    fireEvent.click(screen.getByText("Use TrueNAS's value"));
    expect(state.resolved[0].resolution).toBe("take_theirs");
    expect(screen.queryByText("Save it as their own limit")).toBeNull();
  });
});

// A limit finding's values read in the field's own words, by its ROLE (never its name).
describe("a limit's values read by role", () => {
  beforeEach(() => {
    schemaRead.mode = "known";
  });

  it("words base, ours and theirs in the field's unit, and 'no limit' for its unlimited", () => {
    state.findings = [finding({ field: "home_quota", outcome: "conflict", base: ["0"], ours: ["107374182400"], theirs: ["21474836480"] })];
    renderFindings();
    expect(screen.getByText(/was no limit when Syndra last saw it\. Syndra now wants 100 GB and TrueNAS has 20 GB/)).toBeTruthy();
    expect(screen.queryByText(/107374182400/)).toBeNull();
  });

  it("leaves a value of any other role as the target said it", () => {
    state.findings = [finding({ field: "group", outcome: "theirs_only", base: "lab_makers", theirs: "lab_staff" })];
    renderFindings();
    expect(screen.getByText(/was lab_makers when Syndra last saw it, and is lab_staff now/)).toBeTruthy();
  });

  it("follows the field's declared unlimited, not zero", () => {
    state.findings = [finding({ field: "home_quota", outcome: "theirs_only", base: ["5368709120"], theirs: ["-1"] })];
    renderFindings();
    // The fixture's unlimited is "0": "-1" is a number the field does not call unlimited.
    expect(screen.getByText(/and is -1 bytes now/)).toBeTruthy();
  });
});

// "Use {system}'s value" cannot tell a limit (a panel) from a switched-off account (one press) until
// the fields are known, so until then it is off, says why, and sends nothing.
describe("adopting before the fields are known", () => {
  afterEach(() => {
    schemaRead.mode = "known";
  });

  const adoptable = () => {
    state.findings = [finding({ adoptable: true })];
    state.resolved = [];
  };

  it("is disabled with its reason when no schema was accepted, and a click sends nothing", () => {
    adoptable();
    schemaRead.mode = "unread";
    renderFindings();
    fireEvent.click(screen.getByText("Decide"));
    fireEvent.change(screen.getByLabelText("Why"), { target: { value: "incident review" } });
    const button = screen.getByText("Use TrueNAS's value").closest("button") as HTMLButtonElement;
    expect(button.disabled).toBe(true);
    expect(screen.getByText(/has not read TrueNAS's fields yet/)).toBeTruthy();
    fireEvent.click(button);
    expect(state.resolved).toHaveLength(0);
  });

  it("offers no decision while the fields are being read", () => {
    adoptable();
    schemaRead.mode = "loading";
    renderFindings();
    expect(screen.queryByText("Decide")).toBeNull();
    expect(state.resolved).toHaveLength(0);
  });

  it("shows the failed read with Try again, and no decision", () => {
    adoptable();
    schemaRead.mode = "failed";
    renderFindings();
    expect(screen.queryByText("Decide")).toBeNull();
    fireEvent.click(screen.getByRole("button", { name: "Try again" }));
    expect(schemaRead.refetch).toHaveBeenCalled();
    // It was the system's fields that failed, not the findings, and it says so.
    expect(screen.getByText("TrueNAS's fields could not be read")).toBeTruthy();
    expect(screen.queryByText("The findings could not be read")).toBeNull();
  });
});

// 3.8: Review on does not end the limit; the sentence and the hint say so, from one component.
describe("the Ends on / Review on choice on a finding", () => {
  beforeEach(() => {
    schemaRead.mode = "known";
    state.findings = [finding({ field: "home_quota", outcome: "theirs_only", base: ["0"], theirs: ["21474836480"], adoptable: true })];
    state.resolved = [];
    state.error = null;
    renderFindings();
    fireEvent.click(screen.getByText("Decide"));
  });

  it("says why Save is off when the reason is empty, even after it was cleared", () => {
    fireEvent.change(screen.getByLabelText("Why"), { target: { value: "x" } });
    fireEvent.click(screen.getByText("Use TrueNAS's value"));
    fireEvent.change(screen.getByLabelText("Ends on"), { target: { value: "2027-03-01" } });
    fireEvent.change(screen.getByLabelText("Why"), { target: { value: "" } });
    const save = screen.getByText("Save it as their own limit").closest("button") as HTMLButtonElement;
    expect(save.disabled).toBe(true);
    expect(within(document.querySelector("[data-adopt-limit]") as HTMLElement).getByText("Say why. This becomes the record of the decision.")).toBeTruthy();
  });

  it("promises no end for a review date, in the hint and in the result", () => {
    state.decideOnSave = true;
    fireEvent.change(screen.getByLabelText("Why"), { target: { value: "x" } });
    fireEvent.click(screen.getByText("Use TrueNAS's value"));
    fireEvent.click(screen.getByRole("radio", { name: "Review on" }));
    expect(screen.getByText(/It stays until you keep it or end it\. On that day it lists under holds due for review/)).toBeTruthy();
    fireEvent.change(screen.getByLabelText("Review on"), { target: { value: "2027-03-01" } });
    fireEvent.click(screen.getByText("Save it as their own limit"));
    state.decideOnSave = false;
    expect(screen.getByText(/It stays until somebody keeps or ends it, and lists under holds due for review on 1 Mar/)).toBeTruthy();
    expect(screen.queryByText(/ending on 1 Mar/)).toBeNull();
  });
});
