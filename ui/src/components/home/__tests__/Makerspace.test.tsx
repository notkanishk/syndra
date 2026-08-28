// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { Makerspace } from "@/components/home/Makerspace";
import type { UserListEntry } from "@/lib/queries/useUsers";

const state = vi.hoisted(() => ({
  users: [] as UserListEntry[],
  roles: [] as Array<Record<string, unknown>>,
  bundles: [] as Array<Record<string, unknown>>,
  audit: [] as Array<Record<string, unknown>>,
  governance: {
    expiring_grants: [] as unknown[],
    pending_propagation: { count: 0, zitadel_reachable: true },
    drift: { count: 0 },
  },
}));

vi.mock("@/lib/queries/useUsers", () => ({ useUsers: () => ({ data: state.users }) }));
vi.mock("@/lib/queries/useRoles", () => ({ useGlobalRoleCatalog: () => ({ data: state.roles }) }));
vi.mock("@/lib/queries/useBundles", () => ({ useBundles: () => ({ data: state.bundles }) }));
vi.mock("@/lib/queries/useAudit", () => ({
  useAuditEntries: () => ({ data: state.audit, isLoading: false, error: null, refetch: () => {} }),
}));
vi.mock("@/lib/queries/useGovernance", () => ({
  useGovernanceSummary: () => ({ data: state.governance }),
}));
vi.mock("@/components/names", () => ({
  UserName: ({ id, fallback }: { id: string; fallback?: React.ReactNode }) => (
    <span>{fallback ?? id}</span>
  ),
}));

function person(overrides: Partial<UserListEntry> = {}): UserListEntry {
  return {
    user: { id: "u1", name: "Ada", email: "", title: "", team: "", status: "active", avatar: "A" },
    bundle_count: 0,
    bundle_names: [],
    effective_role_count: 3,
    project_count: 1,
    key_projects: ["Laser Lab"],
    key_project_ids: ["pLaser"],
    expiring_count: 0,
    open_request_count: 0,
    unexplained_count: 0,
    ...overrides,
  } as UserListEntry;
}

function renderMakerspace() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <Makerspace />
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  state.users = [person()];
  state.roles = [];
  state.bundles = [];
  state.audit = [];
  state.governance = {
    expiring_grants: [],
    pending_propagation: { count: 0, zitadel_reachable: true },
    drift: { count: 0 },
  };
});

describe("Home › The makerspace", () => {
  it("renders even when nothing needs the operator", () => {
    // The reason this zone exists: an empty queue used to leave a blank page,
    // so the operator went hunting through the nav instead.
    renderMakerspace();
    expect(screen.getByText("The makerspace")).toBeInTheDocument();
    expect(screen.getByText("Zitadel")).toBeInTheDocument();
  });

  it("surfaces both ends of a person's life here as links, not just counts", () => {
    state.users = [
      person({ effective_role_count: 0 }),
      person({
        user: { ...person().user, id: "u2", status: "departed" },
        effective_role_count: 4,
      }),
    ];
    renderMakerspace();

    const noAccess = screen.getByRole("link", { name: /no access at all/ });
    expect(noAccess).toHaveAttribute("href", "/users?attention=no-access");
    const departed = screen.getByRole("link", { name: /still holds? roles/ });
    expect(departed).toHaveAttribute("href", "/users?attention=departed");
  });

  it("states a clear gap in words rather than showing a zero", () => {
    renderMakerspace();
    expect(screen.getByText("Everybody here has something.")).toBeInTheDocument();
    expect(screen.getByText("Nobody who left still holds anything.")).toBeInTheDocument();
  });

  it("takes its semantic colour only when the machine is actually unhappy", () => {
    state.governance = {
      expiring_grants: [],
      pending_propagation: { count: 0, zitadel_reachable: false },
      drift: { count: 2 },
    };
    renderMakerspace();
    expect(screen.getByText("Unreachable").className).toContain("text-danger-text");
    expect(screen.getByText("2").className).toContain("text-danger-text");

    // A cell with nothing to report stays quiet. The healthy signal is a dot
    // beside the note, never the value itself — four 26px lime numerals would
    // make "nothing is wrong" the loudest thing on the page, and this is the
    // one state that earns its meaning by being the calmest.
    const calm = screen.getAllByText("0");
    expect(calm.length).toBeGreaterThan(0);
    for (const cell of calm) {
      expect(cell.className).toContain("text-ink");
      expect(cell.className).not.toContain("healthy");
      // Healthy is never an action, so it must never borrow the accent either.
      expect(cell.className).not.toContain("accent");
    }

    // It is still said — quietly, and only where nothing is wrong.
    const dots = document.querySelectorAll(".bg-healthy");
    expect(dots.length, "a calm cell marks itself with a healthy dot").toBeGreaterThan(0);
    expect(
      screen.getByText("Changes wait here — nothing is lost").querySelector(".bg-healthy"),
      "an unhealthy cell gets no healthy dot",
    ).toBeNull();
  });

  it("makes every role in the shape list a link to the people holding it", () => {
    state.roles = [
      {
        project_id: "pLaser",
        project_name: "Laser Lab",
        role_key: "trained",
        display_label: "Trained",
        assigned_user_count: 12,
        is_unused: false,
      },
      {
        project_id: "pWood",
        project_name: "Wood Shop",
        role_key: "dormant",
        display_label: "Dormant",
        assigned_user_count: 0,
        is_unused: true,
      },
    ];
    renderMakerspace();

    const link = screen.getByRole("link", { name: /Trained/ });
    expect(link).toHaveAttribute("href", "/users?project=pLaser&role=trained");
    // A role nobody holds is catalogue debt, not a headcount entry.
    expect(screen.queryByRole("link", { name: /Dormant/ })).not.toBeInTheDocument();
    expect(screen.getByRole("link", { name: "1 role nobody holds" })).toBeInTheDocument();
  });

  it("says so plainly when the catalogue has no dead entries", () => {
    state.roles = [
      {
        project_id: "pLaser",
        project_name: "Laser Lab",
        role_key: "trained",
        display_label: "Trained",
        assigned_user_count: 12,
        is_unused: false,
      },
    ];
    state.bundles = [{ id: "b1", name: "Safety", holder_count: 4 }];
    renderMakerspace();
    expect(screen.getByText("Every role and bundle is in use.")).toBeInTheDocument();
  });
});
