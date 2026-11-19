// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { PersonAccess } from "@/components/people/PersonAccess";

/**
 * A person's role rows and project headers, as laid out (layout audit, staging 19 Nov 2026). jsdom
 * lays nothing out, so this reads the structure the layout comes from; `bun run layout:check`
 * measures it in a browser.
 */

vi.mock("@/lib/ui-view", () => ({
  useIsAdvanced: () => false,
  useUiView: () => ({ revealInAdvanced: vi.fn() }),
}));

vi.mock("@/lib/queries/useUpstream", () => ({
  useUpstreamUserGrants: () => ({
    data: { items: [{ id: "zg1", projectId: "p-laser", roleKeys: ["operator", "member"] }], total: 1, observedAt: "2026-10-02T09:54:00Z" },
    isLoading: false,
    error: null,
  }),
}));

vi.mock("@/lib/queries/useUsers", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/lib/queries/useUsers")>()),
  useUserAccess: () => ({
    data: {
      user: { id: "u1", name: "Cody Rhodes", email: "cody@example.edu", status: "active", avatar: "CR" },
      bundles: [],
      projects: [
        {
          project_id: "p-laser",
          project_name: "Laser Lab",
          project_name_resolved: true,
          source_roles: [
            { role_key: "operator", is_source: true, reasons: [{ kind: "direct" }] },
            {
              role_key: "member",
              is_source: true,
              reasons: [{ kind: "bundle", bundle_id: "b1", bundle_name: "Community - Basic", description: "" }],
            },
          ],
          derived_roles: [],
          effective_role_keys: ["operator", "member"],
          observed_role_keys: ["operator", "member"],
        },
      ],
      allowances: [],
      cleanup_hints: [],
    },
    isLoading: false,
    error: null,
    refetch: () => {},
  }),
  useUserGrants: () => ({ data: [{ id: "g1", project_id: "p-laser", role_key: "operator" }], isLoading: false, error: null }),
}));

vi.mock("@/lib/queries/useApplications", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/lib/queries/useApplications")>()),
  useApplications: () => ({
    data: [{ application: { id: "a1", name: "Laser booking", project_id: "p-laser" } }],
    isLoading: false,
    error: null,
  }),
}));

vi.mock("@/lib/queries/useSignIns", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/lib/queries/useSignIns")>()),
  usePersonSignIns: () => ({ data: { usage: [], receiving: true, receiving_since: "2026-10-20T00:00:00Z" }, isLoading: false }),
}));

vi.mock("@/lib/queries/useAudit", () => ({
  useAuditEntries: () => ({ data: [], isLoading: false, error: null, refetch: () => {} }),
}));
vi.mock("@/lib/queries/useRequests", () => ({
  useRequestsAdmin: () => ({ data: [], isLoading: false, error: null, refetch: () => {} }),
  useDecideRequest: () => ({ isPending: false, mutateAsync: vi.fn() }),
}));

function renderPerson() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <PersonAccess userId="u1" isOperator />
    </QueryClientProvider>,
  );
}

describe("a person's role row", () => {
  // 1080–1240: the source had no basis and got 36px, its "Via bundle" chip ran into the status and
  // "Community - Basic" wrapped a word a line over it; on a phone the name kept 230px.
  it("is three columns on the row's own width: four across from 878px, stacked below it, the source never starved", () => {
    renderPerson();
    const row = screen.getByText("Community - Basic").closest("[data-card-row]")!;
    expect(row).toHaveClass("@container");
    const [name, source, status] = Array.from(row.querySelectorAll(":scope > [data-col]"));
    expect(source).toContainElement(screen.getByText("Community - Basic"));
    // Four across: name 230, source at least 160, status 240 (D-1 §5 #22), seat 200, gaps: 878.
    expect(name).toHaveClass("w-full", "@min-[878px]:w-[230px]", "@min-[878px]:flex-none");
    expect(source).toHaveClass("@min-[878px]:min-w-[160px]", "@min-[878px]:flex-[1_1_0%]");
    expect(status).toHaveClass("@min-[878px]:w-(--seat-status)", "@min-[878px]:flex-none");
    // Stacked: the name a line, the source and the status half a line each (less a gap, so the seat
    // cannot squeeze in beside them), then the seat.
    for (const half of [source, status]) expect(half).toHaveClass("flex-[1_1_calc(50%_-_var(--row-gap))]");
    expect(row.querySelector(":scope > [data-action-seat]")).toBeTruthy();
  });
});

describe("a project card's header", () => {
  // On a phone the sign-in line wrapped onto a line of its own, led by its typed "·".
  it("separates its facts with MetaRow's dots, never a typed one that could begin a line", () => {
    renderPerson();
    const signIn = screen.getByText(/sign-in to its app/i);
    const header = signIn.closest("#proj-p-laser")!;
    expect(header.textContent).not.toContain("·");
    expect(signIn.parentElement!.querySelector("[aria-hidden].rounded-pill")).toBeTruthy();
  });
});
