// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { AdminDashboard } from "@/components/dashboard/AdminDashboard";
import { NameResolverProvider } from "@/lib/queries/useNameResolver";
import { UUID_REGEX, makeProxyFetch } from "@/test-utils/proxyFetch";

const ACTOR_ID = "11111111-1111-4111-8111-111111111111";
const TARGET_ID = "22222222-2222-4222-8222-222222222222";
const PROJECT_ID = "33333333-3333-4333-8333-333333333333";

let proxy: ReturnType<typeof makeProxyFetch>;

beforeEach(() => {
  proxy = makeProxyFetch();
  global.fetch = proxy.fetchImpl;
  proxy.register("GET", /\/api\/proxy\/governance\/summary/, () => ({
    pending_requests: [{ id: "r1" }],
    expiring_grants: [],
    cleanup_hints: [],
  }));
  proxy.register("GET", /\/api\/proxy\/audit/, () => [
    {
      id: "a1",
      actor_id: ACTOR_ID,
      target_id: TARGET_ID,
      action: "grant.created",
      resource_id: "grant:g1",
      created_at: new Date().toISOString(),
    },
  ]);
  proxy.register("GET", /\/api\/proxy\/projects/, () => [
    { project: { id: PROJECT_ID, name: "Lab Ops", kind: "managed", description: "", roles: [] }, member_count: 5, bundle_count: 1, rule_in_count: 0, rule_out_count: 0, active_role_keys: [], sample_members: [] },
  ]);
  proxy.register("GET", /\/api\/proxy\/bundles/, () => [{ id: "b1", name: "Mentor Pack", description: "" }]);
  proxy.register("GET", /\/api\/proxy\/intents/, () => []);
  proxy.register("POST", /\/api\/proxy\/lookup/, ({ body }) => {
    const ids = (body as { user_ids?: string[] } | undefined)?.user_ids ?? [];
    const users: Record<string, { display_name: string; email: string }> = {};
    if (ids.includes(ACTOR_ID)) users[ACTOR_ID] = { display_name: "Alice Rivera", email: "alice@ex.org" };
    if (ids.includes(TARGET_ID)) users[TARGET_ID] = { display_name: "Sam Patel", email: "sam@ex.org" };
    return { users, projects: {}, roles: {}, bundles: {} };
  });
});

afterEach(() => {
  vi.restoreAllMocks();
});

function renderDashboard() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, staleTime: 0, gcTime: 0 } },
  });
  return render(
    <QueryClientProvider client={client}>
      <NameResolverProvider>
        <AdminDashboard adminName="Alice" />
      </NameResolverProvider>
    </QueryClientProvider>,
  );
}

describe("AdminDashboard", () => {
  it("renders the operator console hero with the admin name", async () => {
    renderDashboard();
    expect(await screen.findByText(/Welcome back, Alice/)).toBeInTheDocument();
    expect(screen.getByText(/Operator Console/i)).toBeInTheDocument();
  });

  it("caps the stat grid at 4 columns at xl", () => {
    const { container } = renderDashboard();
    const grid = container.querySelector(".xl\\:grid-cols-4");
    expect(grid).not.toBeNull();
  });

  it("resolves activity actor & target into names, never raw UUIDs", async () => {
    const { container } = renderDashboard();
    // The audit row resolves both ids — wait for the lookup batch.
    await waitFor(() => {
      const lookupCalls = proxy.calls.filter((c) => c.url.includes("/api/proxy/lookup"));
      expect(lookupCalls.length).toBeGreaterThanOrEqual(1);
    });
    await screen.findByText("Alice Rivera");
    await screen.findByText("Sam Patel");
    // No raw UUID should appear in the rendered DOM.
    const text = container.textContent ?? "";
    expect(UUID_REGEX.test(text)).toBe(false);
  });
});
