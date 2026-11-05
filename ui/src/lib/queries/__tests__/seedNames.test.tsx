// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import { Suspense } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import ProjectDetailPage from "@/app/projects/[id]/page";
import { BundlePage } from "@/components/bundles/BundlePage";
import { PageCrumbProvider, usePageCrumb } from "@/lib/page-crumb";
import type { BundleRow } from "@/lib/queries/useBundles";
import type { ProjectSummaryRow } from "@/lib/queries/useProjects";

/**
 * A-19 P1, the rest of it: a project's or bundle's page opened from its list
 * (or from a name that links to it) paints the name on the first frame. The
 * page's own read is all that waits.
 */

const catalog = vi.hoisted(() => ({
  projects: new Map<string, { name: string }>(),
  bundles: new Map<string, { name: string }>(),
}));

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: () => {}, replace: () => {} }),
  useSearchParams: () => new URLSearchParams(""),
}));
vi.mock("@/lib/ui-view", () => ({ useIsAdvanced: () => false }));
vi.mock("@/lib/queries/useNameResolver", () => ({
  useNameResolver: () => ({
    resolveProject: (id: string) => ({ value: catalog.projects.get(id), resolved: true }),
    resolveBundle: (id: string) => ({ value: catalog.bundles.get(id), resolved: true }),
  }),
}));
vi.mock("@/components/projects/ProjectClaimDefault", () => ({ ProjectClaimDefault: () => null }));
vi.mock("@/components/roles/CreateRoleDialog", () => ({ CreateRoleDialog: () => null }));
// Both lists are still loading: the page has nothing of its own yet.
vi.mock("@/lib/queries/useProjects", () => ({
  useProjects: () => ({ data: undefined, isLoading: true, error: null, refetch: () => {} }),
}));
vi.mock("@/lib/queries/useApplications", () => ({
  useApplications: () => ({ data: [], isLoading: false, error: null }),
}));
vi.mock("@/lib/queries/useRoles", () => ({
  useGlobalRoleCatalog: () => ({ data: [], isLoading: false, error: null, refetch: () => {} }),
}));
vi.mock("@/lib/queries/useBundles", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/lib/queries/useBundles")>()),
  useBundles: () => ({ data: undefined, isLoading: true, error: null, refetch: () => {} }),
  useBundleRoles: () => ({ data: undefined, isLoading: true, error: null, refetch: () => {} }),
  useBundleDraft: () => ({ data: undefined, isLoading: true, error: null }),
  useBundleIncluders: () => ({ data: undefined, isLoading: true, error: null }),
}));
vi.mock("@/lib/queries/useBundleVersions", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/lib/queries/useBundleVersions")>()),
  useBundleVersions: () => ({ data: undefined, isLoading: true, error: null }),
  useBundleHolders: () => ({ data: undefined, isLoading: true, error: null }),
  useVersionsByBundle: () => ({}),
}));

// React's `use` reads a settled thenable synchronously; a bare Promise would suspend.
const params = { status: "fulfilled", value: { id: "p-1" }, then() {} } as unknown as Promise<{ id: string }>;

function Crumb() {
  return <p data-testid="crumb">{usePageCrumb().crumb ?? "none"}</p>;
}

function shell(ui: React.ReactNode, seed?: (client: QueryClient) => void) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  seed?.(client);
  return render(
    <QueryClientProvider client={client}>
      <PageCrumbProvider>
        <Crumb />
        <Suspense fallback={null}>{ui}</Suspense>
      </PageCrumbProvider>
    </QueryClientProvider>,
  );
}

const projectRow = { project: { id: "p-1", name: "Laser Lab", kind: "x", description: "", roles: [] } } as unknown as ProjectSummaryRow;
const bundleRow = { id: "b-1", name: "Wood shop starter" } as BundleRow;

beforeEach(() => {
  catalog.projects.clear();
  catalog.bundles.clear();
});

describe("a project's page paints its name first", () => {
  it("from the Projects list cache, with the breadcrumb, and no skeleton", () => {
    shell(<ProjectDetailPage params={params} />, (c) => c.setQueryData(["projects"], [projectRow]));
    expect(screen.getByRole("heading", { level: 1 })).toHaveTextContent("Laser Lab");
    expect(screen.getByTestId("crumb")).toHaveTextContent("Laser Lab");
    expect(document.querySelector("h1 .skeleton-bar")).toBeNull();
    // What the row could not know is not invented: no count, no actions.
    expect(screen.queryByRole("button", { name: "New role" })).not.toBeInTheDocument();
  });

  it("from the name catalog when no list was open", () => {
    catalog.projects.set("p-1", { name: "Laser Lab" });
    shell(<ProjectDetailPage params={params} />);
    expect(screen.getByRole("heading", { level: 1 })).toHaveTextContent("Laser Lab");
  });

  it("with nothing cached, is today's skeleton and never the id", () => {
    shell(<ProjectDetailPage params={params} />);
    expect(screen.getByRole("heading", { level: 1 })).not.toHaveTextContent("p-1");
    expect(document.querySelector("h1 .skeleton-bar")).not.toBeNull();
    expect(screen.getByTestId("crumb")).toHaveTextContent("none");
  });
});

describe("a bundle's page paints its name first", () => {
  it("from the Bundles list cache, with the breadcrumb", () => {
    shell(<BundlePage bundleId="b-1" />, (c) => c.setQueryData(["bundles"], [bundleRow]));
    expect(screen.getByRole("heading", { level: 1 })).toHaveTextContent("Wood shop starter");
    expect(screen.getByTestId("crumb")).toHaveTextContent("Wood shop starter");
  });

  it("from the name catalog when no list was open", () => {
    catalog.bundles.set("b-1", { name: "Wood shop starter" });
    shell(<BundlePage bundleId="b-1" />);
    expect(screen.getByRole("heading", { level: 1 })).toHaveTextContent("Wood shop starter");
  });

  it("with nothing cached, keeps the generic title", () => {
    shell(<BundlePage bundleId="b-1" />);
    expect(screen.getByRole("heading", { level: 1 })).toHaveTextContent("Bundle");
    expect(screen.getByTestId("crumb")).toHaveTextContent("none");
  });
});
