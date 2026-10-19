// @vitest-environment jsdom
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { RetiredRulesFold } from "@/components/rules/RetiredRules";

const state = vi.hoisted(() => ({ retired: [] as unknown[] }));

vi.mock("@/lib/queries/useMappingRules", () => ({
  useRetiredMappingRules: () => ({ data: state.retired, isLoading: false, error: null, refetch: vi.fn() }),
}));
vi.mock("@/components/names", () => ({
  ProjectName: ({ id }: { id: string }) => <span>{id}</span>,
  UserName: ({ id }: { id: string }) => <>{id === "u0" ? "Kanishk Singh" : id}</>,
  RoleRef: () => null,
  UserAvatar: () => null,
}));

afterEach(cleanup);

describe("the retired rules fold", () => {
  it("is closed until asked, then reads each rule as its sentence with when and by whom", () => {
    state.retired = [
      {
        id: "r1",
        source_project: "3D Lab",
        source_role: "operator",
        target_project: "Laser Lab",
        target_role: "trained",
        retired_at: "2020-03-04T09:00:00Z",
        retired_by: "u0",
      },
    ];
    render(<RetiredRulesFold />);
    expect(screen.queryByText("operator")).toBeNull();

    fireEvent.click(screen.getByRole("button", { name: /Retired rules/ }));
    expect(screen.getByText("operator")).toBeInTheDocument();
    expect(screen.getByText("trained")).toBeInTheDocument();
    expect(screen.getByText(/^Retired 4 Mar 2020/)).toBeInTheDocument();
    expect(screen.getByText("by Kanishk Singh")).toBeInTheDocument();
    // Read-only: there is nothing to edit or retire.
    expect(screen.queryByRole("button", { name: /Edit|Retire rule/ })).toBeNull();
  });

  it("is not there at all when nothing has been retired", () => {
    state.retired = [];
    const { container } = render(<RetiredRulesFold />);
    expect(container).toBeEmptyDOMElement();
  });
});
