// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

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
      mutate: (input: ResolveFindingInput) => state.resolved.push(input),
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
    expect(screen.getByText(/somebody changed it on the target/i)).toBeTruthy();
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

    expect(screen.getByText(/no longer on the target/i)).toBeTruthy();
    fireEvent.click(screen.getByText("Decide"));
    expect(screen.getByText("Provision it again")).toBeTruthy();
    expect(screen.getByText("Stop managing it")).toBeTruthy();
  });

  // A decision with no reason is a decision nobody can be asked about later,
  // and for an adopted value it becomes that person's policy.
  it("will not send a decision with no reason", () => {
    state.findings = [finding({})];
    state.resolved = [];
    renderFindings();

    fireEvent.click(screen.getByText("Decide"));
    const keep = screen.getByText("Keep Syndra's") as HTMLButtonElement;
    expect(keep.disabled).toBe(true);

    fireEvent.change(screen.getByLabelText("Why"), {
      target: { value: "the suspension was a mistake" },
    });
    fireEvent.click(screen.getByText("Keep Syndra's"));
    expect(state.resolved).toHaveLength(1);
    expect(state.resolved[0].resolution).toBe("keep_ours");
    expect(state.resolved[0].reason).toBe("the suspension was a mistake");
  });

  // An adopted suspension carries a bound in time. The schema refuses a denial
  // without one, and this surface must not be the way around it.
  it("adopts the target's value with a review date", () => {
    state.findings = [finding({ adoptable: true })];
    state.resolved = [];
    renderFindings();

    fireEvent.click(screen.getByText("Decide"));
    fireEvent.change(screen.getByLabelText("Why"), { target: { value: "incident review" } });
    fireEvent.click(screen.getByText("Take the target's"));

    expect(state.resolved[0].resolution).toBe("take_theirs");
    expect(state.resolved[0].review_date).toBeTruthy();
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
    expect(screen.queryByText("Take the target's")).toBeNull();
    // And the alternative, with the mapping and how far editing it reaches.
    expect(screen.getByText(/every holder of that role/i)).toBeTruthy();
    expect(screen.getByText(/3 people hold that role/i)).toBeTruthy();
  });

  // A suspension the target made IS expressible — as a deny allowance — so the
  // button is there.
  it("offers adoption when the value has a per-subject home", () => {
    state.findings = [finding({ adoptable: true })];
    state.error = null;
    renderFindings();

    fireEvent.click(screen.getByText("Decide"));
    expect(screen.getByText("Take the target's")).toBeTruthy();
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

    expect(screen.getByText(/waiting for the target to agree/i)).toBeTruthy();
    expect(screen.queryByText("Decide")).toBeNull();
  });
});
