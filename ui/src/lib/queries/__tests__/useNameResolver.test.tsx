// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, render, screen, waitFor } from "@testing-library/react";
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
let lookupCalls: string[][];
let lookupUsers: Record<string, { display_name: string; email: string }>;

beforeEach(() => {
  catalog = {
    users: [{ id: USER_ID, name: "Jane Doe", email: "jane@example.edu" }],
    projects: [{ id: PROJECT_ID, name: "Lab Ops", roles: [{ key: ROLE_KEY, label: "Mentor" }] }],
    applications: [],
  };
  bundlesStatus = 200;

  proxy = makeProxyFetch();
  global.fetch = proxy.fetchImpl;

  lookupCalls = [];
  lookupUsers = {};

  proxy.register("GET", /\/api\/proxy\/catalog(\?|$)/, () => catalog);
  // POST /lookup is the resolver's second chance at an id the catalog doesn't
  // carry — a deleted account, one created seconds ago, a machine principal.
  proxy.register("POST", /\/api\/proxy\/lookup(\?|$)/, ({ body }) => {
    const ids = (body as { user_ids?: string[] } | undefined)?.user_ids ?? [];
    lookupCalls.push(ids);
    const users: Record<string, { display_name: string; email: string }> = {};
    for (const id of ids) {
      if (lookupUsers[id]) users[id] = lookupUsers[id];
    }
    return { users, projects: {}, roles: {}, bundles: {} };
  });
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

  it("settles on MISS only after the backend lookup also fails to place the id", async () => {
    renderProbe();
    await waitFor(() => expect(screen.getByTestId("user").textContent).toBe("Jane Doe"));
    // The catalog missed, so the resolver asks the backend — which is where
    // deleted accounts and machine principals still resolve from. Only when
    // that also comes back empty is the id genuinely unknown.
    await waitFor(() => expect(lookupCalls.length).toBeGreaterThan(0));
    expect(lookupCalls[0]).toContain("nope");
    await waitFor(() => expect(screen.getByTestId("user-unknown").textContent).toBe("MISS"));
  });

  it("resolves a catalog miss from the backend lookup", async () => {
    // The case that motivated the fallback: an id the user list doesn't carry
    // but the directory can still name. Without this it renders as a raw id.
    lookupUsers = { nope: { display_name: "Deleted Person", email: "gone@example.edu" } };
    renderProbe();
    await waitFor(() => expect(screen.getByTestId("user-unknown").textContent).toBe("Deleted Person"));
  });

  it("works through more misses than fit in one batch", async () => {
    // The backend caps a lookup at 256 ids. A queue that kept settled ids would
    // re-slice the same first 256 forever, so everything past the ceiling would
    // render as a raw identifier and never recover.
    const ids = Array.from({ length: 300 }, (_, index) => `miss-${String(index).padStart(3, "0")}`);
    for (const id of ids) lookupUsers[id] = { display_name: `Person ${id}`, email: "" };

    function ManyProbe() {
      const r = useNameResolver();
      return (
        <div>
          {ids.map((id) => (
            <span key={id} data-testid={id}>
              {r.resolveUser(id).value?.display_name ?? ""}
            </span>
          ))}
        </div>
      );
    }

    render(
      <QueryClientProvider
        client={new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 } } })}
      >
        <NameResolverProvider>
          <ManyProbe />
        </NameResolverProvider>
      </QueryClientProvider>,
    );

    // The last id — well past the batch ceiling — must still resolve.
    await waitFor(
      () => expect(screen.getByTestId("miss-299").textContent).toBe("Person miss-299"),
      { timeout: 4000 },
    );
    // ...and the first batch's answers must survive the second batch landing.
    expect(screen.getByTestId("miss-000").textContent).toBe("Person miss-000");
    expect(lookupCalls.length).toBeGreaterThan(1);
    expect(lookupCalls.every((batch) => batch.length <= 256)).toBe(true);
  });

  it("asks about an unresolvable id once, not on every render", async () => {
    renderProbe();
    await waitFor(() => expect(lookupCalls.length).toBeGreaterThan(0));
    await waitFor(() => expect(screen.getByTestId("user-unknown").textContent).toBe("MISS"));
    const after = lookupCalls.length;
    // A miss the backend can't place must stop being re-requested, or a single
    // deleted account would re-fire a lookup for the life of the session.
    await new Promise((resolve) => setTimeout(resolve, 50));
    expect(lookupCalls.length).toBe(after);
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
    await waitFor(() => expect(screen.getByTestId("user-unknown").textContent).toBe("MISS"));

    // Simulate a user-create: the catalog now includes the previously-unknown id.
    catalog.users.push({ id: "nope", name: "New Person", email: "new@example.edu" });
    await client.invalidateQueries({ queryKey: ["name-catalog"] });

    await waitFor(() => expect(screen.getByTestId("user-unknown").textContent).toBe("New Person"));
  });
});

describe("NameResolverProvider — the unauthenticated gate", () => {
  /** An id the catalog fixture does not carry, so resolving it provokes a lookup. */
  const MISSING_ID = "u-not-in-catalog";

  function Consumer({ id }: { id: string }) {
    const { resolveUser } = useNameResolver();
    const user = resolveUser(id);
    return <span data-testid="name">{user.value?.display_name ?? (user.resolved ? "—" : "…")}</span>;
  }

  function mount(enabled: boolean, id = MISSING_ID) {
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    return render(
      <QueryClientProvider client={client}>
        <NameResolverProvider enabled={enabled}>
          <Consumer id={id} />
        </NameResolverProvider>
      </QueryClientProvider>,
    );
  }

  // The miss lookup is queued from a microtask and fired a render later, so
  // asserting "no request" straight after render proves nothing — it just races
  // the queue. Flush far enough that a request would have landed, then assert.
  async function settle() {
    await act(async () => {
      for (let i = 0; i < 3; i += 1) await new Promise((resolve) => setTimeout(resolve, 0));
    });
  }

  // The control for the test below. If this ever stops firing, `settle()` has
  // become too short and the silence it observes stops meaning anything.
  it("(control) the same wait catches every request an enabled provider makes", async () => {
    mount(true);
    await settle();
    const seen = proxy.calls.map((call) => `${call.method} ${call.url.split("?")[0]}`);
    expect(seen).toContain("GET /api/proxy/catalog");
    expect(seen).toContain("GET /api/proxy/bundles");
    // Triggered by a descendant merely calling resolveUser on an unknown id.
    expect(seen).toContain("POST /api/proxy/lookup");
  });

  it("issues no request at all when there is nobody to resolve names for", async () => {
    mount(false);
    await settle();
    expect(proxy.calls).toEqual([]);
  });

  // A disabled query is pending-but-idle, so `resolved` must read true and the
  // caller must render its fallback rather than a skeleton that never resolves.
  it("resolves to a fallback rather than an eternal skeleton", async () => {
    mount(false);
    await settle();
    expect(screen.getByTestId("name")).toHaveTextContent("—");
  });

  it("still resolves names for a signed-in visitor", async () => {
    mount(true, USER_ID);
    await waitFor(() => expect(screen.getByTestId("name")).toHaveTextContent("Jane Doe"));
  });
});
