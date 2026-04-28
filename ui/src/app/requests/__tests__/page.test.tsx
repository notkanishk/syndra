// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import AdminRequestsView from "@/components/requests/AdminRequestsView";
import { NameResolverProvider } from "@/lib/queries/useNameResolver";
import { UUID_REGEX, makeProxyFetch } from "@/test-utils/proxyFetch";

vi.mock("@/lib/toast", () => ({
  toastSuccess: vi.fn(),
  toastError: vi.fn(),
  toastInfo: vi.fn(),
}));

const REQUESTER_ID = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa";
const REVIEWER_ID = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb";
const PROJECT_ID = "cccccccc-cccc-4ccc-8ccc-cccccccccccc";
const STALE_CREATED_AT = new Date(Date.now() - 1000 * 60 * 60 * 26).toISOString();

let proxy: ReturnType<typeof makeProxyFetch>;

beforeEach(() => {
  proxy = makeProxyFetch();
  global.fetch = proxy.fetchImpl;

  proxy.register("GET", /\/api\/proxy\/catalog/, () => ({
    users: [
      { id: REQUESTER_ID, name: "Sam Patel" },
      { id: REVIEWER_ID, name: "Alice Rivera" },
    ],
    projects: [
      {
        id: PROJECT_ID,
        name: "Lab Ops",
        roles: [{ key: "mentor", label: "Mentor" }],
      },
    ],
  }));

  proxy.register("GET", /\/api\/proxy\/requests(\?|$)/, ({ url }) => {
    const filter = new URL(url, "http://localhost").searchParams.get("status");
    if (filter && filter !== "pending") return [];
    return [
      {
        id: "r1",
        requester_id: REQUESTER_ID,
        project_id: PROJECT_ID,
        role_key: "mentor",
        justification: "Mentoring spring cohort",
        duration_days: 14,
        status: "pending",
        created_at: STALE_CREATED_AT,
      },
    ];
  });

  proxy.register("POST", /\/api\/proxy\/lookup/, ({ body }) => {
    const userIds = (body as { user_ids?: string[] } | undefined)?.user_ids ?? [];
    const projectIds = (body as { project_ids?: string[] } | undefined)?.project_ids ?? [];
    const roleKeys =
      (body as { role_keys?: Array<{ project_id: string; role_key: string }> } | undefined)
        ?.role_keys ?? [];
    const users: Record<string, { display_name: string; email: string }> = {};
    if (userIds.includes(REQUESTER_ID))
      users[REQUESTER_ID] = { display_name: "Sam Patel", email: "sam@ex.org" };
    if (userIds.includes(REVIEWER_ID))
      users[REVIEWER_ID] = { display_name: "Alice Rivera", email: "alice@ex.org" };
    const projects: Record<string, { name: string }> = {};
    if (projectIds.includes(PROJECT_ID)) projects[PROJECT_ID] = { name: "Lab Ops" };
    const roles: Record<string, { display_name: string }> = {};
    for (const rk of roleKeys) {
      if (rk.project_id === PROJECT_ID && rk.role_key === "mentor") {
        roles[`${rk.project_id}:${rk.role_key}`] = { display_name: "Mentor" };
      }
    }
    return { users, projects, roles, bundles: {} };
  });
});

afterEach(() => {
  vi.restoreAllMocks();
});

function renderAdmin() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, staleTime: 0, gcTime: 0 } },
  });
  return render(
    <QueryClientProvider client={client}>
      <NameResolverProvider>
        <AdminRequestsView />
      </NameResolverProvider>
    </QueryClientProvider>,
  );
}

describe("AdminRequestsView (Stage 3)", () => {
  it("renders the request row with resolved user/project/role names", async () => {
    renderAdmin();
    // Resolved display name for the requester replaces the raw UUID.
    await screen.findAllByText(/Sam Patel/);
    // Project name and role name resolve via the lookup batch.
    expect(await screen.findAllByText(/Lab Ops/)).not.toHaveLength(0);
    expect(await screen.findAllByText(/Mentor/)).not.toHaveLength(0);
  });

  it("flags requests pending more than 24 hours with a stale badge", async () => {
    renderAdmin();
    await screen.findByText(/Pending >24h/);
  });

  it("opens a ConfirmModal before approving a request", async () => {
    renderAdmin();
    const approveBtn = await screen.findByRole("button", { name: /^Approve$/ });
    fireEvent.click(approveBtn);
    expect(
      await screen.findByRole("dialog", { name: /Approve this access request/ }),
    ).toBeInTheDocument();
  });

  it("never renders raw Zitadel UUIDs once lookups resolve", async () => {
    const { container } = renderAdmin();
    await waitFor(() => {
      const lookupCalls = proxy.calls.filter((c) => c.url.includes("/api/proxy/lookup"));
      expect(lookupCalls.length).toBeGreaterThanOrEqual(1);
    });
    await screen.findAllByText(/Sam Patel/);
    const text = container.textContent ?? "";
    expect(UUID_REGEX.test(text)).toBe(false);
  });
});
