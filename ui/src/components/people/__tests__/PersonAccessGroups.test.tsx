// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { PersonAccess } from "@/components/people/PersonAccess";

/**
 * route-users-id, default frame: access under one folding card per project, a
 * read line that says once when Zitadel was read, a chip per project at its
 * right end, and the verb that takes each row away in the row itself.
 */

const state = vi.hoisted(() => ({
  projects: [] as Array<Record<string, unknown>>,
  bundles: [] as Array<Record<string, unknown>>,
  advanced: false,
  zitadel: [] as Array<{ id: string; projectId: string; roleKeys: string[] }>,
  observedAt: "2026-10-02T09:54:00Z" as string | undefined,
}));

vi.mock("@/lib/ui-view", () => ({
  useIsAdvanced: () => state.advanced,
  useUiView: () => ({ revealInAdvanced: vi.fn() }),
}));

vi.mock("@/lib/queries/useUpstream", () => ({
  useUpstreamUserGrants: () => ({
    data: { items: state.zitadel, total: state.zitadel.length, observedAt: state.observedAt },
    isLoading: false,
    error: null,
  }),
}));

vi.mock("@/lib/queries/useUsers", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/lib/queries/useUsers")>()),
  useUserAccess: () => ({
    data: {
      user: {
        id: "u1",
        name: "Priya Nair",
        email: "priya@example.edu",
        title: "Research assistant",
        team: "Design",
        status: "active",
        avatar: "PN",
      },
      bundles: state.bundles,
      projects: state.projects,
      allowances: [],
      cleanup_hints: [],
    },
    isLoading: false,
    error: null,
    refetch: () => {},
  }),
  useUserGrants: () => ({
    data: [{ id: "g1", project_id: "p-laser", role_key: "operator", granted_by: "u9" }],
    isLoading: false,
    error: null,
  }),
}));

vi.mock("@/lib/queries/useAudit", () => ({
  useAuditEntries: () => ({ data: [], isLoading: false, error: null, refetch: () => {} }),
}));
vi.mock("@/lib/queries/useRequests", () => ({
  useRequestsAdmin: () => ({ data: [], isLoading: false, error: null, refetch: () => {} }),
  useDecideRequest: () => ({ isPending: false, mutateAsync: vi.fn() }),
}));

function role(key: string, reasons: Array<Record<string, unknown>>) {
  return { role_key: key, is_source: true, reasons };
}

function twoProjects() {
  return [
    {
      project_id: "p-laser",
      project_name: "Laser Lab",
      project_name_resolved: true,
      source_roles: [
        role("operator", [{ kind: "direct" }]),
        role("supervisor", [{ kind: "direct", queued: true }]),
      ],
      derived_roles: [role("door", [{ kind: "mapping", trigger_project: "Laser Lab", trigger_role: "operator" }])],
      effective_role_keys: ["operator", "supervisor", "door"],
      observed_role_keys: ["operator", "supervisor", "door"],
    },
    {
      project_id: "p-wood",
      project_name: "Wood shop",
      project_name_resolved: true,
      source_roles: [role("member", [{ kind: "bundle", bundle_id: "b1", bundle_name: "Lab Tech", description: "" }])],
      derived_roles: [],
      effective_role_keys: ["member"],
      observed_role_keys: ["member", "stray"],
    },
  ];
}

function renderPerson(isOperator = true) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <PersonAccess userId="u1" isOperator={isOperator} />
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  state.projects = twoProjects();
  state.bundles = [];
  state.advanced = false;
  state.observedAt = "2026-10-02T09:54:00Z";
  state.zitadel = [
    { id: "zg1", projectId: "p-laser", roleKeys: ["operator", "supervisor", "door"] },
    { id: "zg2", projectId: "p-wood", roleKeys: ["member", "stray"] },
  ];
});

describe("the header", () => {
  it("names the person, then email, role and team, with no raw id in Basic", () => {
    renderPerson();
    expect(screen.getByRole("heading", { name: "Priya Nair" })).toBeInTheDocument();
    const text = document.body.textContent ?? "";
    expect(text).toMatch(/priya@example\.edu.*Research assistant.*Design/);
    expect(text).not.toMatch(/\bu1\b/);
  });
});

describe("access grouped by project", () => {
  it("draws one card per project, its name a link to the project", () => {
    renderPerson();
    expect(screen.getByRole("link", { name: "Laser Lab" })).toHaveAttribute("href", "/projects/p-laser");
    expect(screen.getByRole("link", { name: "Wood shop" })).toHaveAttribute("href", "/projects/p-wood");
  });

  it("folds a project's rows under its heading and opens them again", () => {
    renderPerson();
    expect(screen.getByText("Member")).toBeInTheDocument();

    const fold = screen.getByRole("button", { name: "Fold Wood shop" });
    expect(fold).toHaveAttribute("aria-expanded", "true");
    fireEvent.click(fold);
    expect(screen.queryByText("Member")).toBeNull();
    // The other project is untouched.
    expect(screen.getByText("Operator")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Open Wood shop" }));
    expect(screen.getByText("Member")).toBeInTheDocument();
  });

  it("says when Zitadel was read once, not on every project", () => {
    renderPerson();
    expect(screen.getAllByText(/What Zitadel confirmed/)).toHaveLength(1);
    expect(screen.queryByText(/Read from Zitadel/)).toBeNull();
  });
});

describe("the project chips", () => {
  it("name each project with its count, and mark the one with something unexplained", () => {
    renderPerson();
    const nav = screen.getByRole("navigation", { name: "Projects on this page" });
    const laser = within(nav).getByRole("link", { name: /Laser Lab/ });
    const wood = within(nav).getByRole("link", { name: /Wood shop/ });
    expect(laser).toHaveTextContent("3");
    expect(wood).toHaveTextContent("1");
    expect(wood.querySelector(".bg-warn")).not.toBeNull();
    expect(laser.querySelector(".bg-warn")).toBeNull();
  });

  it("open a folded project on the way to it", () => {
    renderPerson();
    fireEvent.click(screen.getByRole("button", { name: "Fold Wood shop" }));
    expect(screen.queryByText("Member")).toBeNull();

    fireEvent.click(
      within(screen.getByRole("navigation", { name: "Projects on this page" })).getByRole("link", { name: /Wood shop/ }),
    );
    expect(screen.getByText("Member")).toBeInTheDocument();
  });

  it("are not drawn for a single project", () => {
    state.projects = twoProjects().slice(0, 1);
    renderPerson();
    expect(screen.queryByRole("navigation", { name: "Projects on this page" })).toBeNull();
  });
});

describe("the verb on each row", () => {
  it("is Revoke for direct access, Withdraw for what is still waiting, Remove for the rest", () => {
    renderPerson();
    expect(screen.getByRole("button", { name: /Revoke Operator/ })).toHaveTextContent("Revoke");
    expect(screen.getByRole("button", { name: /Withdraw Supervisor/ })).toHaveTextContent("Withdraw");
    expect(screen.getByRole("button", { name: /Remove Door/ })).toHaveTextContent("Remove");
    expect(screen.getByRole("button", { name: /Remove Member/ })).toHaveTextContent("Remove");
  });

  it("opens the same removal dialog the overflow used to", () => {
    renderPerson();
    fireEvent.click(screen.getByRole("button", { name: /Revoke Operator/ }));
    expect(screen.getByRole("heading", { name: /Revoke direct access to Laser Lab \/ operator/ })).toBeInTheDocument();
  });

  it("is not offered to a member reading their own page", () => {
    renderPerson(false);
    expect(screen.queryByRole("button", { name: /^(Revoke|Withdraw|Remove)/ })).toBeNull();
  });
});

describe("a person who holds nothing", () => {
  it("says what would put access here", () => {
    state.projects = [];
    renderPerson();
    expect(screen.getByText("Priya Nair holds no access yet")).toBeInTheDocument();
    expect(screen.getByText(/Access appears here once/)).toBeInTheDocument();
    // The strip above it keeps its seat.
    expect(screen.getByText("None assigned")).toBeInTheDocument();
  });

  it("speaks to a member in the second person", () => {
    state.projects = [];
    renderPerson(false);
    expect(screen.getByText("You hold no access yet")).toBeInTheDocument();
  });
});
