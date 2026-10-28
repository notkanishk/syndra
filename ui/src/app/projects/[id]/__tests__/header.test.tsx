// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { Suspense } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import ProjectDetailPage from "@/app/projects/[id]/page";

const state = vi.hoisted(() => ({ loading: true, failed: false, advanced: false }));

vi.mock("@/lib/ui-view", () => ({ useIsAdvanced: () => state.advanced }));
vi.mock("@/lib/page-crumb", () => ({ useCrumb: () => {} }));
vi.mock("@/lib/queries/useProjects", () => ({
  useProjects: () => ({
    data: state.loading || state.failed
      ? undefined
      : [{ project: { id: "p-9001", name: "Laser Lab" }, member_count: 3, confirmed_member_count: 3, observed_member_count: 3 }],
    isLoading: state.loading,
    error: state.failed ? new Error("down") : null,
    refetch: () => {},
  }),
}));
vi.mock("@/lib/queries/useApplications", () => ({
  useApplications: () => ({ data: [], isLoading: false, error: null }),
}));
vi.mock("@/lib/queries/useRoles", () => ({
  useGlobalRoleCatalog: () => ({ data: [], isLoading: false, error: null, refetch: () => {} }),
}));
vi.mock("@/components/roles/CreateRoleDialog", () => ({ CreateRoleDialog: () => null }));

// React's `use` reads a settled thenable synchronously; a bare Promise would suspend.
const params = { status: "fulfilled", value: { id: "p-9001" }, then() {} } as unknown as Promise<{ id: string }>;

async function renderPage() {
  render(
    <Suspense fallback={null}>
      <ProjectDetailPage params={params} />
    </Suspense>,
  );
  await screen.findByRole("heading", { level: 1 });
}

beforeEach(() => {
  state.loading = true;
  state.failed = false;
  state.advanced = false;
});

// While the list loaded the title was the project id and the meta line read "0 roles · no app
// reads this yet": an opaque string and a count of nothing, both on screen at once.
describe("a project's header", () => {
  it("is a skeleton while loading, never the id or a count", async () => {
    await renderPage();
    const title = screen.getByRole("heading", { level: 1 });
    expect(title).not.toHaveTextContent("p-9001");
    expect(screen.queryByText(/no app reads this yet/)).not.toBeInTheDocument();
    expect(document.querySelector("h1 .skeleton-bar")).not.toBeNull();
  });

  it("names the project and prints no id in Basic once loaded", async () => {
    state.loading = false;
    await renderPage();
    expect(screen.getByRole("heading", { level: 1 })).toHaveTextContent("Laser Lab");
    expect(screen.queryByText("p-9001")).not.toBeInTheDocument();
  });

  // A list that failed has said nothing about whether the project exists.
  it("is an error, not 'isn't listed', when the list failed to load", async () => {
    state.loading = false;
    state.failed = true;
    render(
      <Suspense fallback={null}>
        <ProjectDetailPage params={params} />
      </Suspense>,
    );
    expect(await screen.findByText("Couldn't load this project.")).toBeInTheDocument();
    expect(screen.queryByText(/isn't listed/)).not.toBeInTheDocument();
  });

  it("keeps the id in Advanced", async () => {
    state.loading = false;
    state.advanced = true;
    await renderPage();
    expect(screen.getByText("p-9001")).toBeInTheDocument();
  });
});
