// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { NameResolverProvider, useNameResolver } from "@/lib/queries/useNameResolver";
import { makeProxyFetch, respondWith } from "@/test-utils/proxyFetch";

const USER_ID = "u1";
const PROJECT_ID = "p1";
const ROLE_KEY = "mentor";
const BUNDLE_ID = "b1";

// Fixture stand-ins for GET /catalog and GET /bundles. `catalog` is a `let` so
// the invalidation test can mutate it between fetches.
let catalog: {
  users: Array<{ id: string; name: string; email: string }>;
  projects: Array<{ id: string; name: string; roles: Array<{ key: string; label: string }> }>;
  applications: unknown[];
};

let proxy: ReturnType<typeof makeProxyFetch>;
let bundlesStatus: number;

beforeEach(() => {
  catalog = {
    users: [{ id: USER_ID, name: "Jane Doe", email: "jane@x.edu" }],
    projects: [{ id: PROJECT_ID, name: "Lab Ops", roles: [{ key: ROLE_KEY, label: "Mentor" }] }],
    applications: [],
  };
  bundlesStatus = 200;

  proxy = makeProxyFetch();
  global.fetch = proxy.fetchImpl;

  proxy.register("GET", /\/api\/proxy\/catalog(\?|$)/, () => catalog);
  proxy.register("GET", /\/api\/proxy\/bundles(\?|$)/, () =>
    bundlesStatus === 200
      ? [{ id: BUNDLE_ID, name: "Starter Bundle", description: "", roles: [], created_at: "" }]
      : respondWith(bundlesStatus, { error: "Forbidden" }),
  );
});

afterEach(() => {
  vi.restoreAllMocks();
});

// Probe surfaces each resolver result as LOADING / MISS / <value>.
function Probe() {
  const r = useNameResolver();
  const u = r.resolveUser(USER_ID);
  const uMiss = r.resolveUser("nope");
  const p = r.resolveProject(PROJECT_ID);
  const role = r.resolveRole(PROJECT_ID, ROLE_KEY);
  const b = r.resolveBundle(BUNDLE_ID);
  const show = <T extends { display_name?: string; name?: string }>(res: {
    value: T | undefined;
    resolved: boolean;
  }) => (!res.resolved ? "LOADING" : (res.value?.display_name ?? res.value?.name ?? "MISS"));
  return (
    <div>
      <span data-testid="user">{show(u)}</span>
      <span data-testid="user-unknown">{show(uMiss)}</span>
      <span data-testid="project">{show(p)}</span>
      <span data-testid="role">{show(role)}</span>
      <span data-testid="bundle">{show(b)}</span>
    </div>
  );
}

function renderProbe() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0, staleTime: 0 } },
  });
  const utils = render(
    <QueryClientProvider client={client}>
      <NameResolverProvider>
        <Probe />
      </NameResolverProvider>
    </QueryClientProvider>,
  );
  return { client, ...utils };
}

describe("useNameResolver (full-catalog)", () => {
  it("reports resolved=false while the catalog query is loading", () => {
    renderProbe();
    // First synchronous paint: catalog query is still pending.
    expect(screen.getByTestId("user").textContent).toBe("LOADING");
  });

  it("resolves a known user, project, and nested role after the catalog loads", async () => {
    renderProbe();
    await waitFor(() => expect(screen.getByTestId("user").textContent).toBe("Jane Doe"));
    expect(screen.getByTestId("project").textContent).toBe("Lab Ops");
    expect(screen.getByTestId("role").textContent).toBe("Mentor");
  });

  it("returns resolved=true with no value for an unknown id after load", async () => {
    renderProbe();
    await waitFor(() => expect(screen.getByTestId("user").textContent).toBe("Jane Doe"));
    expect(screen.getByTestId("user-unknown").textContent).toBe("MISS");
  });

  it("resolves bundles from GET /bundles when allowed", async () => {
    renderProbe();
    await waitFor(() => expect(screen.getByTestId("bundle").textContent).toBe("Starter Bundle"));
  });

  it("isolates a 403 on GET /bundles: catalog resolves, bundle falls back", async () => {
    bundlesStatus = 403; // member context — proxy forbids GET /bundles
    renderProbe();
    // Catalog-backed resolvers still work...
    await waitFor(() => expect(screen.getByTestId("user").textContent).toBe("Jane Doe"));
    expect(screen.getByTestId("project").textContent).toBe("Lab Ops");
    expect(screen.getByTestId("role").textContent).toBe("Mentor");
    // ...while the bundle query failed and yields a fallback, not a stuck skeleton.
    await waitFor(() => expect(screen.getByTestId("bundle").textContent).toBe("MISS"));
  });

  it("refetches the catalog on invalidation so newly-present ids resolve", async () => {
    const { client } = renderProbe();
    await waitFor(() => expect(screen.getByTestId("user").textContent).toBe("Jane Doe"));
    expect(screen.getByTestId("user-unknown").textContent).toBe("MISS");

    // Simulate a user-create: the catalog now includes the previously-unknown id.
    catalog.users.push({ id: "nope", name: "New Person", email: "new@x.edu" });
    await client.invalidateQueries({ queryKey: ["name-catalog"] });

    await waitFor(() => expect(screen.getByTestId("user-unknown").textContent).toBe("New Person"));
  });
});
