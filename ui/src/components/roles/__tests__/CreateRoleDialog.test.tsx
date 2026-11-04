// @vitest-environment jsdom
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { CreateRoleDialog } from "@/components/roles/CreateRoleDialog";

const create = vi.fn().mockResolvedValue({ id: "role_1" });

vi.mock("@/lib/queries/useNameResolver", () => ({
  useNameResolver: () => ({
    resolveUser: () => ({ value: undefined, resolved: true }),
    resolveProject: (id: string) =>
      id === "p-laser"
        ? { value: { name: "Laser Cutter" }, resolved: true }
        : { value: undefined, resolved: true },
    resolveRole: () => ({ value: undefined, resolved: true }),
    resolveBundle: () => ({ value: undefined, resolved: true }),
  }),
}));

vi.mock("@/lib/queries/useProjects", () => ({
  useProjects: () => ({
    data: [{ project: { id: "p-laser", name: "Laser Cutter" } }],
    isLoading: false,
    error: null,
  }),
}));

vi.mock("@/lib/queries/useRoles", () => ({
  useGlobalRoleCatalog: () => ({ data: [], isLoading: false, error: null }),
  useCreateRole: () => ({ mutateAsync: create, isPending: false }),
}));

describe("the new-role dialog pinned to a project", () => {
  it("names the pinned project rather than showing its raw id", () => {
    render(<CreateRoleDialog pinnedProjectId="p-laser" onClose={() => {}} />);
    expect(screen.getByText("Laser Cutter")).toBeInTheDocument();
    expect(screen.queryByText("p-laser")).toBeNull();
  });

  it("names the project in the created-role message, not its raw id", async () => {
    render(<CreateRoleDialog pinnedProjectId="p-laser" onClose={() => {}} />);
    fireEvent.change(screen.getByLabelText("Role key"), { target: { value: "trained" } });
    fireEvent.change(screen.getByLabelText("Display name"), {
      target: { value: "Laser trained" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Create role" }));

    await waitFor(() => expect(create).toHaveBeenCalled());
    expect(await screen.findByText(/Laser trained is in Laser Cutter/)).toBeInTheDocument();
    expect(screen.queryByText(/p-laser/)).toBeNull();
    // Said once, in the footer note; the result does not repeat it.
    expect(screen.getByText("Nobody has it yet.")).toBeInTheDocument();
    expect(screen.queryByText(/goes straight to Zitadel/)).toBeNull();
    expect(screen.getByRole("button", { name: "Close" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Cancel" })).toBeNull();
  });
});
