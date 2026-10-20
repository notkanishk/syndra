// @vitest-environment jsdom
import { render, screen, within } from "@testing-library/react";
import { Suspense } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import ProjectDetailPage from "@/app/projects/[id]/page";

const state = vi.hoisted(() => ({
  apps: [] as Array<{ application: { id: string; name: string; project_id: string } }>,
  roles: [] as Array<Record<string, unknown>>,
}));

vi.mock("@/lib/ui-view", () => ({ useIsAdvanced: () => false }));
vi.mock("@/lib/page-crumb", () => ({ useCrumb: () => {} }));
vi.mock("@/lib/queries/useProjects", () => ({
  useProjects: () => ({
    data: [{ project: { id: "p1", name: "Laser Lab" }, member_count: 3, confirmed_member_count: 3, observed_member_count: 3 }],
    isLoading: false,
    error: null,
  }),
}));
vi.mock("@/lib/queries/useApplications", () => ({
  useApplications: () => ({ data: state.apps, isLoading: false, error: null }),
}));
vi.mock("@/lib/queries/useRoles", () => ({
  useGlobalRoleCatalog: () => ({ data: state.roles, isLoading: false, error: null, refetch: () => {} }),
}));
vi.mock("@/components/roles/CreateRoleDialog", () => ({ CreateRoleDialog: () => null }));

const params = { status: "fulfilled", value: { id: "p1" }, then() {} } as unknown as Promise<{ id: string }>;

function role(key: string, over: Record<string, unknown> = {}) {
  return {
    project_id: "p1",
    role_key: key,
    display_name: key,
    description: "",
    group: "Machines",
    waits_for_send: false,
    assigned_user_count: 1,
    ...over,
  };
}

async function renderPage() {
  render(
    <Suspense fallback={null}>
      <ProjectDetailPage params={params} />
    </Suspense>,
  );
  await screen.findByRole("heading", { level: 1 });
}

beforeEach(() => {
  state.apps = [
    { application: { id: "a1", name: "Laser bookings", project_id: "p1" } },
    { application: { id: "a2", name: "Laser badge reader", project_id: "p1" } },
  ];
  state.roles = [
    role("laser.operator", {
      display_name: "Laser cutter · Operator",
      description: "Can start a cut without staff in the room.",
      waits_for_send: true,
    }),
    role("laser.member", { display_name: "Laser cutter · Member" }),
  ];
});

// The board (route-projects-id): serves are links to the apps, each role has its
// description under it, a role that always waits wears the mark, and the only
// action beside the lede is New role.
describe("a project's page, as drawn", () => {
  it("links each app it serves, and offers only New role", async () => {
    await renderPage();
    expect(screen.getByRole("link", { name: /Laser bookings/ })).toHaveAttribute("href", "/applications/a1");
    expect(screen.getByRole("link", { name: /Laser badge reader/ })).toHaveAttribute("href", "/applications/a2");
    expect(screen.queryByRole("link", { name: "Token format" })).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "New role" })).toBeInTheDocument();
  });

  it("says no app reads it when none does, with nothing to link", async () => {
    state.apps = [];
    await renderPage();
    expect(screen.getByText(/no app reads this yet/)).toBeInTheDocument();
  });

  it("writes the description under the role, in full and in the muted ink", async () => {
    await renderPage();
    const text = screen.getByText("Can start a cut without staff in the room.");
    expect(text.className).toMatch(/text-muted/);
    expect(text.className).not.toMatch(/truncate|line-clamp/);
  });

  it("marks only the role that waits, and says Straight away for the other", async () => {
    await renderPage();
    const waiting = screen.getByText("Laser cutter · Operator").closest("div[class*='row-divider']") as HTMLElement;
    const straight = screen.getByText("Laser cutter · Member").closest("div[class*='row-divider']") as HTMLElement;
    expect(within(waiting).getAllByRole("img", { name: "Waits for you" }).length).toBeGreaterThan(0);
    expect(within(waiting).getByText("Waits for you")).toBeInTheDocument();
    expect(within(straight).queryByRole("img", { name: "Waits for you" })).not.toBeInTheDocument();
    expect(within(straight).getByText("Straight away")).toBeInTheDocument();
  });

  it("explains When given under the card, with the way to where it is set", async () => {
    await renderPage();
    expect(screen.getByText(/When given says whether a change to a role goes straight to Zitadel/)).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /Set in Roles that wait/ })).toHaveAttribute("href", "/changes/roles-that-wait");
  });

  it("reads Role, Group, When given, People, and never cuts a group name", async () => {
    await renderPage();
    const head = document.querySelector("[class*='tracking-[0.09em]']") as HTMLElement;
    expect(Array.from(head.children).map((c) => c.textContent)).toEqual(["Role", "Group", "When given", "People"]);
    const group = screen.getAllByText("Machines")[0];
    expect(group.className).not.toMatch(/truncate/);
  });
});
