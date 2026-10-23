// @vitest-environment jsdom
import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import RolesThatWaitPage from "@/app/changes/roles-that-wait/page";
import type { CatalogRole } from "@/lib/queries/useRoles";
import type { RolesThatWait } from "@/lib/queries/useRolesThatWait";

const state = vi.hoisted(() => ({
  waits: null as unknown as RolesThatWait,
  marked: [] as { ids: string[]; waits: boolean }[],
  switched: 0,
}));

function role(project_id: string, role_key: string, display_name: string, people: number, waits = false): CatalogRole {
  return {
    project_id, project_name: "Laser Lab", role_key, display_name, description: "",
    bundle_count: 0, rule_count: 0, assigned_user_count: people, is_unused: false, source: "zitadel",
    waits_for_send: waits,
  } as CatalogRole;
}

const catalog = [
  role("p1", "laser.supervisor", "Laser cutter · Supervisor", 4),
  role("p1", "laser.operator", "Laser cutter · Operator", 9),
  role("p1", "laser.door", "Laser bay · Door", 22),
];

vi.mock("@/lib/queries/useRoles", async (orig) => ({
  ...(await orig<typeof import("@/lib/queries/useRoles")>()),
  useGlobalRoleCatalog: () => ({ data: catalog, isLoading: false, error: null }),
}));
vi.mock("@/lib/queries/usePropagation", async (orig) => ({
  ...(await orig<typeof import("@/lib/queries/usePropagation")>()),
  usePendingPropagations: () => ({
    data: [{ id: "o1", wait_reason: "role_waits" }, { id: "o2", wait_reason: "halted:zitadel_offline" }],
  }),
}));
vi.mock("next/navigation", () => ({ useSearchParams: () => new URLSearchParams() }));
vi.mock("@/components/names/UserName", () => ({ UserName: () => <>Kanishk Singh</> }));
vi.mock("@/lib/queries/useRolesThatWait", async (orig) => ({
  ...(await orig<typeof import("@/lib/queries/useRolesThatWait")>()),
  useRolesThatWait: () => ({ data: state.waits, isLoading: false, error: null, refetch: vi.fn() }),
  useSetRolesWait: () => ({
    isPending: false,
    mutateAsync: async (v: { ids: string[]; waits: boolean }) => {
      state.marked.push(v);
    },
  }),
  useSwitchToRolesThatWait: () => ({
    isPending: false,
    mutateAsync: async () => {
      state.switched++;
    },
  }),
}));

beforeEach(() => {
  state.marked = [];
  state.switched = 0;
});

describe("Roles that wait for you", () => {
  describe("the one-time move, before the switch over", () => {
    beforeEach(() => {
      state.waits = {
        roles: [],
        switched_over: false,
        suggestions: [
          { project_id: "p1", role_key: "laser.supervisor", via: [{ kind: "bundle", id: "b1", name: "Staff" }] },
          { project_id: "p1", role_key: "laser.door", via: [{ kind: "rule", id: "r1" }] },
        ],
      };
    });

    it("says what waited, and keeps every suggestion chosen to start", () => {
      render(<RolesThatWaitPage />);
      expect(screen.getByText("Bundles and rules no longer have their own modes")).toBeInTheDocument();
      expect(screen.getByText(/Until today, 1 bundle and 1 rule waited for you/)).toBeInTheDocument();
      expect(screen.getByText("Given by Staff, which waited")).toBeInTheDocument();
      expect(screen.getByRole("checkbox", { name: "Laser cutter · Supervisor" })).toBeChecked();
      expect(screen.getByRole("button", { name: "Mark 2 and finish" })).toBeInTheDocument();
    });

    it("marks what's chosen, then switches over", async () => {
      render(<RolesThatWaitPage />);
      fireEvent.click(screen.getByRole("checkbox", { name: "Laser bay · Door" }));
      fireEvent.click(screen.getByRole("button", { name: "Mark 1 and finish" }));
      await waitFor(() => expect(state.switched).toBe(1));
      expect(state.marked).toEqual([{ ids: ["p1:laser.supervisor"], waits: true }]);
    });

    it("can finish with none marked", async () => {
      render(<RolesThatWaitPage />);
      fireEvent.click(screen.getByRole("button", { name: "Mark none" }));
      await waitFor(() => expect(state.switched).toBe(1));
      expect(state.marked).toEqual([{ ids: [], waits: true }]);
    });
  });

  describe("after the switch over", () => {
    beforeEach(() => {
      state.waits = {
        roles: [{ project_id: "p1", role_key: "laser.operator", marked_by: "u1", marked_at: "2026-10-02T09:00:00Z" }],
        switched_over: true,
        suggestions: [],
      };
    });

    it("lists the roles that wait, gathered by project, with who marked them", () => {
      render(<RolesThatWaitPage />);
      expect(screen.getByText("Laser Lab · 1")).toBeInTheDocument();
      expect(screen.getByText("Laser cutter · Operator")).toBeInTheDocument();
      expect(screen.getByText(/Kanishk Singh, 2 Oct/)).toBeInTheDocument();
      // Only changes waiting because a role waits count here; a stopped send is another story.
      expect(screen.getByText("1 change to these roles")).toBeInTheDocument();
    });

    it("does not point at the drift sound (A-07 §1: it is about drift, and lives in Account)", () => {
      render(<RolesThatWaitPage />);
      expect(screen.queryByText(/sound/i)).toBeNull();
    });

    it("stops a role waiting with its row's verb", async () => {
      render(<RolesThatWaitPage />);
      fireEvent.click(screen.getByRole("button", { name: "Stop waiting" }));
      await waitFor(() => expect(state.marked).toEqual([{ ids: ["p1:laser.operator"], waits: false }]));
    });

    it("says what marking means before it marks", async () => {
      render(<RolesThatWaitPage />);
      fireEvent.click(screen.getByRole("button", { name: "Add roles" }));
      const dialog = await screen.findByRole("dialog");
      expect(within(dialog).getByText("Choose the roles that should always wait for you.", { selector: "div" })).toBeInTheDocument();
      fireEvent.click(within(dialog).getByRole("checkbox", { name: /Laser cutter · Supervisor/ }));
      expect(
        within(dialog).getByText(
          "From now on, Laser cutter · Supervisor waits for you. 4 people have it today, and nothing changes for them now.",
        ),
      ).toBeInTheDocument();
      fireEvent.click(within(dialog).getByRole("button", { name: "Mark 1 role" }));
      await waitFor(() => expect(state.marked).toEqual([{ ids: ["p1:laser.supervisor"], waits: true }]));
    });
  });
});
