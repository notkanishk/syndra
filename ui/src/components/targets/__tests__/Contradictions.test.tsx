// @vitest-environment jsdom
import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { MergeFindings } from "@/components/targets/MergeFindings";
import { ApiError } from "@/lib/api-client";
import type { MergeFinding } from "@/lib/queries/useTargets";

/**
 * The two places the code contradicted the drawing (designs B1, B2).
 *
 * B1 is the one that matters. A finding takes ONE decision, because the answers
 * are opposites: keeping Syndra's value and taking the target's cannot both be
 * queued without one releasing an account the other is re-provisioning. So the
 * second operator to press is refused — and the useful thing a refusal can give
 * them is what was chosen and why, not the fact that they lost.
 */

const state = {
  findings: [] as MergeFinding[],
  error: null as unknown,
};

vi.mock("@/lib/queries/useTargets", async () => {
  const actual = await vi.importActual<typeof import("@/lib/queries/useTargets")>(
    "@/lib/queries/useTargets",
  );
  return {
    ...actual,
    useMergeFindings: () => ({ data: state.findings, isLoading: false, error: null, refetch: vi.fn() }),
    useResolveMergeFinding: () => ({ mutate: vi.fn(), isPending: false, error: state.error }),
  };
});

vi.mock("@/components/names", () => ({
  UserName: ({ id, fallback }: { id: string; fallback?: string }) => <span>{id || fallback}</span>,
}));

function conflict(): MergeFinding {
  return {
    id: "f_2210",
    target: "truenas",
    subject_id: "u_moller",
    field: "uid",
    outcome: "conflict",
    base: 3104,
    ours: 3131,
    theirs: 3120,
    detected_at: "2026-08-27T09:00:00Z",
    last_seen_at: "2026-08-27T09:00:00Z",
    adoptable: true,
  };
}

function alreadyDecided(): ApiError {
  return new ApiError(409, {
    error: "ALREADY_DECIDED",
    message: "f_2210 already decided as take_theirs by u_iyer",
    details: {
      decision: "take_theirs",
      decided_by: "u_iyer",
      decision_reason: "renumbered by hand during the migration; her files are all under 3120",
      decided_at: "2026-08-27T09:04:00Z",
    },
  });
}

describe("a decision somebody else already took", () => {
  it("quotes their reason in full, which is who the mandatory reason was for", () => {
    state.findings = [conflict()];
    state.error = alreadyDecided();
    render(<MergeFindings target="truenas" />);

    // The form has to be open for the refusal to have a home.
    fireEvent.click(screen.getByRole("button", { name: "Decide" }));

    // Split across elements, so read the rendered text rather than one node.
    const text = document.body.textContent ?? "";
    expect(text).toContain("renumbered by hand during the migration");
    expect(text).toMatch(/Using TrueNAS.s value/);
  });

  // The API's own sentence is raw material: a UUID beside a snake_case
  // resolution is not copy, and it was what an operator used to be handed.
  it("does not render the API's sentence", () => {
    state.findings = [conflict()];
    state.error = alreadyDecided();
    render(<MergeFindings target="truenas" />);
    fireEvent.click(screen.getByRole("button", { name: "Decide" }));

    expect(screen.queryByText(/already decided as take_theirs/)).toBeNull();
    expect(screen.queryByText(/f_2210 already decided/)).toBeNull();
  });

  it("says the finding is still standing, not gone", () => {
    state.findings = [conflict()];
    state.error = alreadyDecided();
    render(<MergeFindings target="truenas" />);
    fireEvent.click(screen.getByRole("button", { name: "Decide" }));

    expect(document.body.textContent).toContain("has not gone anywhere");
  });
});
