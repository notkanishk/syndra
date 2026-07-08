// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import AuditView from "@/app/audit/page";
import { NameResolverProvider } from "@/lib/queries/useNameResolver";
import { UUID_REGEX, makeProxyFetch } from "@/test-utils/proxyFetch";

const ACTOR_ID = "77777777-7777-4777-8777-777777777777";
const TARGET_ID = "88888888-8888-4888-8888-888888888888";
const GRANT_USER_ID = "99999999-9999-4999-8999-999999999999";
const GRANT_PROJECT_ID = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa";

let proxy: ReturnType<typeof makeProxyFetch>;

beforeEach(() => {
  proxy = makeProxyFetch();
  global.fetch = proxy.fetchImpl;

  let auditCalls = 0;
  proxy.register("GET", /\/api\/proxy\/audit/, ({ url }) => {
    auditCalls++;
    const u = new URL(url, "http://t");
    const limit = Number(u.searchParams.get("limit") ?? "50");
    return Array.from({ length: limit }).map((_, i) => ({
      id: `a${i}-${auditCalls}`,
      actor_id: ACTOR_ID,
      target_id: TARGET_ID,
      action: "grant.created",
      resource_id: "grant:g1",
      created_at: new Date(Date.now() - i * 60_000).toISOString(),
    }));
  });

  proxy.register("GET", /\/api\/proxy\/governance\/summary/, () => ({
    pending_requests: [{ id: "r1" }],
    expiring_grants: [
      {
        id: "wg1",
        user_id: GRANT_USER_ID,
        project_id: GRANT_PROJECT_ID,
        role_key: "mentor",
        expires_at: new Date(Date.now() + 2 * 24 * 60 * 60 * 1000).toISOString(),
      },
    ],
    cleanup_hints: ["Remove stale role grant for X"],
  }));

  // Name resolution reads the full catalog: users + projects + nested roles.
  proxy.register("GET", /\/api\/proxy\/catalog(\?|$)/, () => ({
    users: [
      { id: ACTOR_ID, name: "Alice Rivera", email: "alice@ex.org" },
      { id: TARGET_ID, name: "Sam Patel", email: "sam@ex.org" },
      { id: GRANT_USER_ID, name: "Maya Chen", email: "maya@ex.org" },
    ],
    projects: [
      { id: GRANT_PROJECT_ID, name: "Lab Ops", kind: "managed", description: "", roles: [{ key: "mentor", label: "Mentor", description: "" }] },
    ],
    applications: [],
  }));
});

afterEach(() => {
  vi.restoreAllMocks();
});

function renderAudit() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, staleTime: 0, gcTime: 0 } },
  });
  return render(
    <QueryClientProvider client={client}>
      <NameResolverProvider>
        <AuditView />
      </NameResolverProvider>
    </QueryClientProvider>,
  );
}

describe("AuditView (Stage 2)", () => {
  it("renders activity rows with resolved actor + target names", async () => {
    renderAudit();
    await screen.findAllByText("Alice Rivera");
    await screen.findAllByText("Sam Patel");
  });

  it("renders the actor select with resolved names instead of raw UUIDs", async () => {
    renderAudit();
    const select = await screen.findByLabelText(/Filter by actor/i);
    // Wait for resolution to land — once it does, the option text should be
    // the resolved display name, not a UUID.
    await waitFor(() => {
      const options = Array.from(select.querySelectorAll("option")).map((o) => o.textContent ?? "");
      const resolved = options.find((t) => t === "Alice Rivera");
      expect(resolved).toBe("Alice Rivera");
    });
    const optionsText = Array.from(select.querySelectorAll("option")).map((o) => o.textContent ?? "");
    for (const text of optionsText) {
      expect(UUID_REGEX.test(text)).toBe(false);
    }
  });

  it("renders the watchlist row with name-resolved user → project : role format", async () => {
    renderAudit();
    await screen.findByText("Maya Chen");
    await screen.findAllByText("Lab Ops");
    await screen.findByText("Mentor");
  });

  it("triggers a second audit fetch when 'Load more' is clicked", async () => {
    renderAudit();
    // Wait for initial fetch
    await waitFor(() => {
      expect(proxy.calls.filter((c) => c.url.includes("/api/proxy/audit")).length).toBeGreaterThanOrEqual(1);
    });
    const before = proxy.calls.filter((c) => c.url.includes("/api/proxy/audit")).length;
    const loadMore = await screen.findByRole("button", { name: /Load more/i });
    fireEvent.click(loadMore);
    await waitFor(() => {
      const after = proxy.calls.filter((c) => c.url.includes("/api/proxy/audit")).length;
      expect(after).toBeGreaterThan(before);
    });
  });

  it("never renders raw Zitadel UUIDs in the visible activity feed once names resolve", async () => {
    const { container } = renderAudit();
    await screen.findAllByText("Alice Rivera");
    // The Drawer closed-state contains no resource UUID; only when an admin
    // opens it via click does the raw resource_id surface (deliberate
    // forensic affordance). The default body must be UUID-clean.
    const text = container.textContent ?? "";
    expect(UUID_REGEX.test(text)).toBe(false);
  });
});
