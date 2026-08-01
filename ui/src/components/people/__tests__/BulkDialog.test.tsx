// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
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
        return { ...api.plan!, applied: true };
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

vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

function plan(overrides: Partial<BulkPlan> = {}): BulkPlan {
  return {
    op: "assign_role",
    applied: false,
    outcomes: [
      { user_id: "u1", name: "Ada Lovelace", email: "ada@x.edu", effect: "apply", detail: "Gains this role." },
      {
        user_id: "u2",
        name: "Leo Brooks",
        email: "leo@x.edu",
        effect: "blocked",
        detail: "Account is departed — remove it from the selection to continue.",
      },
      {
        user_id: "u3",
        name: "Sam Patel",
        email: "sam@x.edu",
        effect: "no_change",
        detail: "Holds no direct grant here.",
        consequence: "Keeps the role via the Safety bundle — remove that source instead.",
      },
    ],
    summary: { total: 3, apply: 1, no_change: 1, blocked: 1, failed: 0, succeeded: 0 },
    ...overrides,
  };
}

function open(op: Parameters<typeof BulkDialog>[0]["op"] = "assign_role") {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <BulkDialog op={op} userIds={["u1", "u2", "u3"]} scope="in Laser Lab" onClose={() => {}} />
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
    expect(screen.getByRole("button", { name: "Rehearse" })).toBeInTheDocument();
    expect(api.applied).toHaveLength(0);
  });

  it("requires a reason before it will even rehearse", () => {
    open();
    // Every row of a bulk write lands in the audit log; an unexplained one is
    // a change nobody can account for later.
    expect(screen.getByRole("button", { name: "Rehearse" })).toBeDisabled();
  });

  async function rehearse() {
    fireEvent.change(screen.getByLabelText("Project"), { target: { value: "pLaser" } });
    fireEvent.change(screen.getByLabelText("Role"), { target: { value: "trained" } });
    fireEvent.change(screen.getByLabelText("Reason"), { target: { value: "New cohort" } });
    fireEvent.click(screen.getByRole("button", { name: "Rehearse" }));
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
    expect(screen.getByText(/Keeps the role via the Safety bundle/)).toBeInTheDocument();
  });

  it("counts only the rows that will actually change on the confirm button", async () => {
    open();
    await rehearse();
    // 3 selected, 1 actionable. Offering "Apply to 3" would be a lie.
    expect(screen.getByRole("button", { name: "Apply to 1 person" })).toBeInTheDocument();
    expect(screen.getByText(/1 already in that state · 1 refused/)).toBeInTheDocument();
  });

  it("applies the same request it rehearsed", async () => {
    open();
    await rehearse();
    fireEvent.click(screen.getByRole("button", { name: "Apply to 1 person" }));
    await waitFor(() => expect(api.applied).toHaveLength(1));
    // The plan an operator approved and the write it authorises must describe
    // the same operation.
    expect(api.applied[0]).toEqual(api.rehearsed[0]);
  });

  it("refuses to apply when the rehearsal found nothing to do", async () => {
    api.plan = plan({ summary: { total: 3, apply: 0, no_change: 3, blocked: 0, failed: 0, succeeded: 0 } });
    open();
    await rehearse();
    const button = screen.getByRole("button", { name: "Nothing to apply" });
    expect(button).toBeDisabled();
  });

  it("lets an operator go back and change the target without losing the dialog", async () => {
    open();
    await rehearse();
    fireEvent.click(screen.getByRole("button", { name: "Back" }));
    expect(screen.getByRole("button", { name: "Rehearse" })).toBeInTheDocument();
    expect(screen.getByLabelText("Reason")).toHaveValue("New cohort");
  });
});
