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
    expiring_grants: [] as Array<{ user_id: string }>,
    unreconciled_targets: [] as Array<{ target: string; since: string }>,
  },
  targets: [{ target: "truenas" }] as Array<{ target: string }>,
}));

const loading = vi.hoisted(() => ({ users: false, roles: false, bundles: false }));

vi.mock("@/lib/queries/useUsers", () => ({
  useUsers: () => ({ data: state.users, isLoading: loading.users }),
}));
vi.mock("@/lib/queries/useRoles", () => ({
  useGlobalRoleCatalog: () => ({ data: state.roles, isLoading: loading.roles }),
}));
vi.mock("@/lib/queries/useBundles", () => ({
  useBundles: () => ({ data: state.bundles, isLoading: loading.bundles }),
}));
vi.mock("@/lib/queries/useAudit", () => ({
  useAuditEntries: () => ({ data: state.audit, isLoading: false, error: null, refetch: () => {} }),
}));
vi.mock("@/lib/queries/useGovernance", () => ({
  useGovernanceSummary: () => ({ data: state.governance }),
}));
vi.mock("@/lib/queries/useTargets", () => ({
  useTargets: () => ({ data: state.targets, isLoading: false }),
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
    // An observed count only ever arrives with the basis it was read on.
    observation: { read_at: "2026-09-18T10:00:00Z", current: true, truncated: false },
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
  state.governance = { expiring_grants: [], unreconciled_targets: [] };
  state.targets = [{ target: "truenas" }];
  loading.users = false;
  loading.roles = false;
  loading.bundles = false;
});

describe("Home › The makerspace", () => {
  it("renders even when nothing needs the operator", () => {
    // The reason this zone exists: an empty queue used to leave a blank page,
    // so the operator went hunting through the nav instead.
    renderMakerspace();
    expect(screen.getByText("The makerspace")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Connected systems" })).toBeInTheDocument();
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

  it("has the board's three tiles, each the link to the list it counts", () => {
    renderMakerspace();
    const hrefs = Object.fromEntries(
      ["Connected systems", "People", "Where access lives"].map((name) => [
        name,
        screen.getByRole("link", { name }).getAttribute("href"),
      ]),
    );
    expect(hrefs).toEqual({
      "Connected systems": "/system/targets",
      People: "/users",
      "Where access lives": "/projects",
    });
    // The four machine-health cells repeated the queue above them: pending changes,
    // drift and expiring access are blocks there, so they are not counted twice here.
    for (const gone of ["Zitadel", "Waiting for you to send", "Ending in the next 14 days"]) {
      expect(screen.queryByRole("link", { name: gone })).toBeNull();
    }
  });

  it("says how many systems were read, and which one was not", () => {
    state.targets = [{ target: "truenas" }, { target: "grafana" }, { target: "wiki" }];
    state.governance = {
      expiring_grants: [],
      unreconciled_targets: [{ target: "truenas", since: "2026-10-01T12:00:00Z" }],
    };
    renderMakerspace();
    const tile = screen.getByRole("link", { name: "Connected systems" });
    expect(tile).toHaveTextContent("2 of 3 read");
    expect(tile).toHaveTextContent(/TrueNAS has not been read since 1 Oct/);
    expect(tile.querySelector(".text-warn-text")).not.toBeNull();
  });

  it("agrees with an empty queue when every system was read", () => {
    state.targets = [{ target: "truenas" }, { target: "grafana" }, { target: "wiki" }];
    renderMakerspace();
    const tile = screen.getByRole("link", { name: "Connected systems" });
    expect(tile).toHaveTextContent("3 of 3 read");
    expect(tile.querySelector(".text-warn-text")).toBeNull();
  });

  it("counts people, and the people whose access ends soon once each", () => {
    state.users = [person(), person({ user: { ...person().user, id: "u2" } })];
    state.governance = {
      expiring_grants: [{ user_id: "u1" }, { user_id: "u1" }, { user_id: "u2" }],
      unreconciled_targets: [],
    };
    renderMakerspace();
    const tile = screen.getByRole("link", { name: "People" });
    expect(tile).toHaveTextContent("2");
    expect(tile).toHaveTextContent("2 whose access ends in the next 14 days");
  });

  it("shows no people count while the list is still being read", () => {
    loading.users = true;
    state.users = [];
    renderMakerspace();
    expect(screen.getByRole("link", { name: "People" })).not.toHaveTextContent(/^People0/);
    expect(screen.getByRole("link", { name: "People" })).toHaveTextContent("—");
  });

  it("counts roles across projects and bundles", () => {
    state.roles = [
      { project_id: "pLaser", project_name: "Laser Lab", role_key: "a", assigned_user_count: 1, is_unused: false },
      { project_id: "pLaser", project_name: "Laser Lab", role_key: "b", assigned_user_count: 1, is_unused: false },
      { project_id: "pWood", project_name: "Wood Shop", role_key: "c", assigned_user_count: 1, is_unused: false },
    ];
    state.bundles = [{ id: "b1", name: "Safety", holder_count: 4 }];
    renderMakerspace();
    const tile = screen.getByRole("link", { name: "Where access lives" });
    expect(tile).toHaveTextContent("3 roles");
    expect(tile).toHaveTextContent("Across 2 projects and 1 bundle");
  });

  it("makes every role in the shape list a link to its own page, which lists everyone the count counts", () => {
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
    expect(link).toHaveAttribute("href", "/projects/pLaser/roles/trained");
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

  it("defers to the observed count once one exists, same as the People filter", () => {
    // effective_role_count says access, but the observed count is what
    // hasAccess() (people-filters) treats as the answer once it exists — the
    // same predicate the Gaps tile is required to reuse rather than
    // re-implement.
    state.users = [
      person({ effective_role_count: 3, observed_role_count: 0 }),
      person({
        user: { ...person().user, id: "u2", status: "departed" },
        effective_role_count: 0,
        observed_role_count: 4,
      }),
    ];
    renderMakerspace();

    expect(screen.getByRole("link", { name: /no access at all/ })).toHaveTextContent("1");
    expect(screen.getByRole("link", { name: /still holds roles/ })).toHaveTextContent("1");
  });

  it("shows a neutral placeholder while people are still loading, never an affirmative empty state", () => {
    loading.users = true;
    state.users = [];
    renderMakerspace();

    expect(screen.getByLabelText("Loading gaps")).toBeInTheDocument();
    expect(screen.queryByText("Everybody here has something.")).not.toBeInTheDocument();
  });

  it("shows a neutral placeholder while the role catalogue is still loading, never an affirmative empty state", () => {
    loading.roles = true;
    state.roles = [];
    renderMakerspace();

    expect(screen.getAllByLabelText(/^Loading (gaps|where access lives)$/).length).toBeGreaterThan(0);
    expect(screen.queryByText("Nobody holds a role yet.")).not.toBeInTheDocument();
    expect(screen.queryByText("Every role and bundle is in use.")).not.toBeInTheDocument();
  });

  it("prints a bare clock for today and a dated one for anything older, in Lately", () => {
    const today = new Date();
    const lastWeek = new Date(today.getTime() - 7 * 86_400_000);
    state.audit = [
      { id: "a1", actor_id: "u1", action: "grant", target_id: "-", created_at: today.toISOString() },
      { id: "a2", actor_id: "u1", action: "grant", target_id: "-", created_at: lastWeek.toISOString() },
    ];
    renderMakerspace();

    // formatWhen: a bare clock for today, a dated one otherwise — Lately used
    // to always show a bare clock, which read as "today" for a week-old entry.
    const times = screen.getAllByText(/^\d{2}:\d{2}$|^\d{1,2} \w+ \d{2}:\d{2}$/);
    expect(times.some((el) => /^\d{1,2} \w+ \d{2}:\d{2}$/.test(el.textContent ?? ""))).toBe(true);
  });
});
