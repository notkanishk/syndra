// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import UsersView from "@/app/users/page";
import { NameResolverProvider } from "@/lib/queries/useNameResolver";
import { UUID_REGEX, makeProxyFetch } from "@/test-utils/proxyFetch";

vi.mock("@/lib/toast", () => ({
  toastSuccess: vi.fn(),
  toastError: vi.fn(),
}));

const USER_ID = "44444444-4444-4444-8444-444444444444";
const PROJECT_ID = "55555555-5555-4555-8555-555555555555";
const GRANTED_BY_ID = "66666666-6666-4666-8666-666666666666";

let proxy: ReturnType<typeof makeProxyFetch>;

beforeEach(() => {
  proxy = makeProxyFetch();
  global.fetch = proxy.fetchImpl;

  proxy.register("GET", /\/api\/proxy\/users(\?|$)/, () => [
    {
      user: {
        id: USER_ID,
        name: "Sam Patel",
        email: "sam@ex.org",
        title: "Student Maker",
        team: "Members",
        status: "active",
        avatar: "SP",
      },
      bundle_count: 1,
      effective_role_count: 3,
      key_projects: ["Lab Ops"],
    },
  ]);

  proxy.register("GET", /\/api\/proxy\/bundles(\?|$)/, () => [
    { id: "b1", name: "Mentor Pack", description: "Hands-on training" },
  ]);

  proxy.register("GET", /\/api\/proxy\/bundles\/b1\/roles/, () => [
    { bundle_id: "b1", zitadel_project_id: PROJECT_ID, zitadel_role_key: "mentor" },
  ]);

  proxy.register("GET", /\/api\/proxy\/projects(\?|$)/, () => [
    {
      project: {
        id: PROJECT_ID,
        name: "Lab Ops",
        kind: "managed",
        description: "",
        roles: [{ key: "mentor", label: "Mentor" }],
      },
      member_count: 5,
      bundle_count: 1,
      rule_in_count: 0,
      rule_out_count: 0,
      active_role_keys: [],
      sample_members: [],
    },
  ]);

  proxy.register("GET", new RegExp(`/api/proxy/users/${USER_ID}/access`), () => ({
    user: {
      id: USER_ID,
      name: "Sam Patel",
      email: "sam@ex.org",
      title: "Student Maker",
      team: "Members",
      status: "active",
      avatar: "SP",
    },
    bundles: [],
    projects: [
      {
        project_id: PROJECT_ID,
        project_name: "Lab Ops",
        source_roles: [
          { role_key: "mentor", reasons: [{ kind: "direct", description: "Direct grant by admin" }] },
        ],
        derived_roles: [
          { role_key: "trainee", reasons: [{ kind: "bundle", description: "From Mentor Pack" }] },
        ],
        effective_role_keys: ["mentor", "trainee"],
      },
    ],
    cleanup_hints: [],
  }));

  proxy.register("GET", new RegExp(`/api/proxy/users/${USER_ID}/grants`), () => [
    {
      id: "g1",
      project_id: PROJECT_ID,
      role_key: "mentor",
      granted_by: GRANTED_BY_ID,
      reason: "Mentoring spring cohort",
      expires_at: null,
    },
  ]);

  // Name resolution reads the full catalog: users + projects + nested roles.
  proxy.register("GET", /\/api\/proxy\/catalog(\?|$)/, () => ({
    users: [
      { id: USER_ID, name: "Sam Patel", email: "sam@ex.org" },
      { id: GRANTED_BY_ID, name: "Alice Rivera", email: "alice@ex.org" },
    ],
    projects: [
      {
        id: PROJECT_ID,
        name: "Lab Ops",
        kind: "managed",
        description: "",
        roles: [
          { key: "mentor", label: "Mentor", description: "" },
          { key: "trainee", label: "Trainee", description: "" },
        ],
      },
    ],
    applications: [],
  }));
});

afterEach(() => {
  vi.restoreAllMocks();
});

function renderUsers() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, staleTime: 0, gcTime: 0 } },
  });
  return render(
    <QueryClientProvider client={client}>
      <NameResolverProvider>
        <UsersView />
      </NameResolverProvider>
    </QueryClientProvider>,
  );
}

describe("UsersView (Stage 2)", () => {
  it("renders project filter pill labels using project names from key_projects", async () => {
    renderUsers();
    const pill = await screen.findByRole("button", { name: "Lab Ops" });
    expect(pill).toBeInTheDocument();
    expect(pill).toHaveAttribute("aria-pressed", "false");
    fireEvent.click(pill);
    expect(pill).toHaveAttribute("aria-pressed", "true");
  });

  it("renders both Source and Derived columns of the lineage tree", async () => {
    renderUsers();
    expect(await screen.findByText(/Source · directly granted/i)).toBeInTheDocument();
    expect(await screen.findByText(/Derived · via bundles & rules/i)).toBeInTheDocument();
  });

  it("opens a ConfirmModal when an admin clicks Assign on an unassigned bundle", async () => {
    renderUsers();
    const assignBtn = await screen.findByRole("button", { name: /^Assign$/ });
    fireEvent.click(assignBtn);
    expect(await screen.findByRole("dialog", { name: /Assign "Mentor Pack"/ })).toBeInTheDocument();
  });

  it("renders the assign confirmation copy with resolved role display names", async () => {
    renderUsers();
    const assignBtn = await screen.findByRole("button", { name: /^Assign$/ });
    fireEvent.click(assignBtn);
    const dialog = await screen.findByRole("dialog", { name: /Assign "Mentor Pack"/ });
    // The confirm description must include the resolved role display name
    // ("Mentor"), never the raw project_id:role_key composite.
    await waitFor(() => {
      expect(dialog.textContent ?? "").toContain("Mentor");
    });
    const text = dialog.textContent ?? "";
    expect(text).not.toMatch(/[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}:mentor/);
  });

  it("never renders raw Zitadel UUIDs once lookups resolve", async () => {
    const { container } = renderUsers();
    await waitFor(() => {
      const catalogCalls = proxy.calls.filter((c) => c.url.includes("/api/proxy/catalog"));
      expect(catalogCalls.length).toBeGreaterThanOrEqual(1);
    });
    // Wait for at least one resolved name to land before asserting.
    await screen.findAllByText("Sam Patel");
    const text = container.textContent ?? "";
    expect(UUID_REGEX.test(text)).toBe(false);
  });
});
