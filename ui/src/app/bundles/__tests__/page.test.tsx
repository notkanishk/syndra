// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import BundlesView from "@/app/bundles/page";
import { NameResolverProvider } from "@/lib/queries/useNameResolver";
import { UUID_REGEX, makeProxyFetch } from "@/test-utils/proxyFetch";

vi.mock("@/lib/toast", () => ({
  toastSuccess: vi.fn(),
  toastError: vi.fn(),
  toastInfo: vi.fn(),
}));

const PROJECT_ID = "33333333-3333-4333-8333-333333333333";
const USER_ID = "44444444-4444-4444-8444-444444444444";

let proxy: ReturnType<typeof makeProxyFetch>;

beforeEach(() => {
  proxy = makeProxyFetch();
  global.fetch = proxy.fetchImpl;

  proxy.register("GET", /\/api\/proxy\/bundles(\?|$)/, () => [
    { id: "b1", name: "Mentor Pack", description: "Hands-on training", created_at: new Date().toISOString() },
  ]);

  proxy.register("GET", /\/api\/proxy\/bundles\/b1\/roles/, () => [
    { bundle_id: "b1", zitadel_project_id: PROJECT_ID, zitadel_role_key: "mentor" },
  ]);

  proxy.register("GET", /\/api\/proxy\/bundles\/b1\/impact/, () => ({
    role_count: 1,
    users: [{ id: USER_ID, name: "Sam Patel" }],
  }));

  // Name resolution reads the full catalog: users + projects + nested roles.
  proxy.register("GET", /\/api\/proxy\/catalog/, () => ({
    users: [{ id: USER_ID, name: "Sam Patel", email: "sam@ex.org" }],
    projects: [
      {
        id: PROJECT_ID,
        name: "Lab Ops",
        roles: [{ key: "mentor", label: "Mentor" }],
      },
    ],
    applications: [],
  }));
});

afterEach(() => {
  vi.restoreAllMocks();
});

function renderBundles() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, staleTime: 0, gcTime: 0 } },
  });
  return render(
    <QueryClientProvider client={client}>
      <NameResolverProvider>
        <BundlesView />
      </NameResolverProvider>
    </QueryClientProvider>,
  );
}

describe("BundlesView (Stage 3)", () => {
  it("renders the bundle row and resolves role chips through Name components on expand", async () => {
    renderBundles();
    const bundleHeader = await screen.findByRole("button", { name: /Mentor Pack/ });
    fireEvent.click(bundleHeader);

    // The role chip should resolve to "Lab Ops · Mentor" — never the raw composite id.
    await screen.findAllByText(/Mentor/);
    await screen.findAllByText(/Lab Ops/);
  });

  it("only fetches impact data when the impact accordion is opened", async () => {
    renderBundles();
    const bundleHeader = await screen.findByRole("button", { name: /Mentor Pack/ });
    fireEvent.click(bundleHeader);

    // Roles always load on expand; impact must not.
    await waitFor(() => {
      expect(proxy.calls.some((c) => c.url.includes("/bundles/b1/roles"))).toBe(true);
    });
    expect(proxy.calls.some((c) => c.url.includes("/bundles/b1/impact"))).toBe(false);

    // Now open the impact accordion — exactly one impact call should fire.
    const impactToggle = await screen.findByRole("button", { name: /Impact preview/i });
    fireEvent.click(impactToggle);
    await waitFor(() => {
      expect(proxy.calls.some((c) => c.url.includes("/bundles/b1/impact"))).toBe(true);
    });
  });

  it("never renders raw Zitadel UUIDs once lookups resolve", async () => {
    const { container } = renderBundles();
    const bundleHeader = await screen.findByRole("button", { name: /Mentor Pack/ });
    fireEvent.click(bundleHeader);
    const impactToggle = await screen.findByRole("button", { name: /Impact preview/i });
    fireEvent.click(impactToggle);
    await screen.findAllByText(/Sam Patel/);
    await waitFor(() => {
      expect(proxy.calls.some((c) => c.url.includes("/catalog"))).toBe(true);
    });
    const text = container.textContent ?? "";
    expect(UUID_REGEX.test(text)).toBe(false);
  });
});

describe("BundlesView welcome-bundle toggle", () => {
  // Override the bundles list registered by the top-level beforeEach with one
  // that flags Mentor Pack as the welcome bundle. Use a fresh proxy so the
  // first-match-wins ordering inside makeProxyFetch is irrelevant.
  beforeEach(() => {
    proxy = makeProxyFetch();
    global.fetch = proxy.fetchImpl;
    proxy.register("GET", /\/api\/proxy\/bundles(\?|$)/, () => [
      {
        id: "b1",
        name: "Mentor Pack",
        description: "Hands-on training",
        is_welcome: true,
        created_at: new Date().toISOString(),
      },
    ]);
    proxy.register("PUT", /\/api\/proxy\/bundles\/[^/]+\/welcome/, () => ({ message: "Welcome bundle set" }));
  });

  it("renders the Welcome badge for the flagged bundle", async () => {
    renderBundles();
    expect(await screen.findByText("Welcome")).toBeInTheDocument();
  });

  it("disables the Set as welcome bundle button when already flagged", async () => {
    renderBundles();
    const header = await screen.findByRole("button", { name: /Mentor Pack/ });
    fireEvent.click(header);
    const btn = await screen.findByRole("button", { name: /Already welcome bundle/i });
    expect(btn).toBeDisabled();
  });
});
