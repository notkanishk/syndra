// @vitest-environment jsdom
import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { GrantDirectAccess } from "@/components/people/GrantDirectAccess";

/**
 * The direct-grant endpoint always enqueues to the outbox for the
 * operator-triggered drain — it never applies inline from this dialog. A
 * result reported as "applied" here would tell the operator Zitadel has the
 * grant when only Syndra's own ledger does.
 */

const state = vi.hoisted(() => ({ mutateAsync: vi.fn(), grants: [] as unknown[] }));

vi.mock("@/lib/queries/useProjects", () => ({
  useProjects: () => ({ data: [{ project: { id: "p1", name: "Printing Lab" } }] }),
}));

vi.mock("@/lib/queries/useRoles", () => ({
  useGlobalRoleCatalog: () => ({
    data: [
      {
        project_id: "p1",
        role_key: "operator",
        project_name: "Printing Lab",
        display_name: "Operator",
        assigned_user_count: 3,
      },
    ],
  }),
}));

vi.mock("@/lib/queries/useUsers", () => ({
  useCreateGrant: () => ({ mutateAsync: state.mutateAsync, isPending: false }),
  useUserGrants: () => ({ data: state.grants }),
  useUserAccess: () => ({ data: { projects: [] } }),
}));

function open() {
  return render(
    <GrantDirectAccess userId="u1" userName="Ada Lovelace" open onClose={vi.fn()} />,
  );
}

async function grant() {
  fireEvent.change(screen.getByLabelText("Project"), { target: { value: "p1" } });
  fireEvent.change(screen.getByLabelText("Role"), { target: { value: "operator" } });
  fireEvent.change(screen.getByPlaceholderText(/Capstone build/), { target: { value: "test" } });
  fireEvent.click(screen.getByRole("button", { name: "Grant access" }));
  // "Close" only replaces "Grant access"/"Cancel" once an outcome (either kind)
  // is set — a wait condition that holds regardless of which message the
  // outcome carries.
  await screen.findByRole("button", { name: "Close" });
}

describe("GrantDirectAccess", () => {
  it("says a role already held with no end changes nothing, and will not send it", () => {
    state.grants = [{ id: "g1", project_id: "p1", role_key: "operator", expires_at: null }];
    open();
    fireEvent.change(screen.getByLabelText("Project"), { target: { value: "p1" } });
    fireEvent.change(screen.getByLabelText("Role"), { target: { value: "operator" } });
    expect(screen.getByText(/already has this, with no end/)).toBeTruthy();
    expect((screen.getByRole("button", { name: "Grant access" }) as HTMLButtonElement).disabled).toBe(true);
    expect(screen.getByText("They already have this.")).toBeTruthy();
    state.grants = [];
  });

  it("reports queued, not applied, when the grant is still in the outbox", async () => {
    state.mutateAsync.mockResolvedValue({ outbox_id: "o1", status: "pending" });
    open();
    await grant();

    expect(screen.getByRole("link", { name: /Waiting for you to send/ })).toHaveAttribute("href", "/governance/pending");
    expect(screen.queryByText("Applied")).not.toBeInTheDocument();
    expect(screen.getByText(/nothing has reached zitadel yet/i)).toBeInTheDocument();
    // Future tense while queued — "now holds" here would contradict the
    // "nothing has reached Zitadel yet" line right beside it.
    expect(screen.getByText(/Ada Lovelace will hold/)).toBeInTheDocument();
  });

  it("reports applied when the response says the grant is not pending", async () => {
    state.mutateAsync.mockResolvedValue({ outbox_id: "o1", status: "applied" });
    open();
    await grant();

    expect(screen.getByText("Applied")).toBeInTheDocument();
    expect(screen.getByText(/Ada Lovelace now holds/)).toBeInTheDocument();
  });

  it("names the project and role, never the bare id", () => {
    open();
    expect(screen.getByText("Printing Lab")).toBeInTheDocument();
    expect(screen.queryByText("p1")).not.toBeInTheDocument();
  });
});
