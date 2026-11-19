// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { PersonAccess } from "@/components/people/PersonAccess";

const view = vi.hoisted(() => ({ advanced: false, extra: {} as Record<string, unknown> }));

vi.mock("@/lib/ui-view", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/lib/ui-view")>()),
  useIsAdvanced: () => view.advanced,
}));

vi.mock("@/lib/queries/useUsers", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/lib/queries/useUsers")>()),
  useUserAccess: () => ({
    data: {
      user: {
        id: "312458847001600001",
        name: "Charlotte Flair",
        email: "charlotte@example.edu",
        title: "Student Staff",
        team: "Fabrication",
        status: "active",
        avatar: "AL",
      },
      bundles: [],
      projects: [],
      cleanup_hints: [],
      ...view.extra,
    },
    isLoading: false,
    error: null,
    refetch: () => {},
  }),
  useUserGrants: () => ({ data: [], isLoading: false, error: null }),
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
      <PersonAccess userId="312458847001600001" isOperator />
    </QueryClientProvider>,
  );
}

// The Basic header is the person's name, email, role and team (route-users-id). The 18-digit
// subject id sat after them, so a screenshot of the page carried an opaque number.
describe("a person's header", () => {
  it("shows no raw id in Basic", () => {
    view.advanced = false;
    renderPerson();
    expect(screen.getByRole("heading", { name: "Charlotte Flair" })).toBeInTheDocument();
    expect(screen.getByText(/charlotte@example.edu/)).toBeInTheDocument();
    expect(screen.queryByText("312458847001600001")).not.toBeInTheDocument();
  });

  it("keeps the id reachable in Advanced", () => {
    view.advanced = true;
    renderPerson();
    expect(screen.getByText("312458847001600001")).toBeInTheDocument();
  });

  // A-24 §9: the line that knows sits where the lede was, and with no read of Zitadel it says so.
  it("opens on the line that knows, not the lede every person had", () => {
    renderPerson();
    expect(screen.getByText("Checking what Zitadel shows.")).toBeInTheDocument();
    expect(screen.queryByText(/Everything this person can use/)).toBeNull();
    // A-24 frame 18: a head line with its dot. While checking the dot's seat is kept, unseen, so the words
    // never shift when the ladder's dot lands.
    const dot = screen.getByText("Checking what Zitadel shows.").closest("[data-dot-line]")?.querySelector("[data-dot]");
    expect(dot).toHaveClass("invisible");
  });

  it("draws the line's dot amber when it names an end (A-24 frame 18)", () => {
    const until = new Date(Date.now() + 3 * 86_400_000).toISOString();
    view.extra = {
      observation: { read_at: new Date().toISOString(), current: true, truncated: false },
      observed_role_count: 1,
      projects: [
        { project_id: "pL", project_name: "Laser cutting", source_roles: [], derived_roles: [], effective_role_keys: ["op"], observed_role_keys: ["op"], standings: { op: "held" }, holdings: { op: { kind: "until", until } } },
      ],
    };
    try {
      renderPerson();
      const line = screen.getByText(/ends in 3 days\.$/).closest("[data-dot-line]");
      expect(line?.querySelector("[data-dot]")).toHaveClass("bg-warn");
      expect(line?.querySelector("[data-dot]")).not.toHaveClass("invisible");
    } finally {
      view.extra = {};
    }
  });
});
