// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it } from "vitest";

import { NameResolverProvider } from "@/lib/queries/useNameResolver";
import { makeProxyFetch } from "@/test-utils/proxyFetch";

import { RecentCascades } from "../RecentCascades";

const USER_ID = "11111111-1111-4111-8111-111111111111";
const PROJECT_ID = "22222222-2222-4222-8222-222222222222";

let proxy: ReturnType<typeof makeProxyFetch>;

beforeEach(() => {
  proxy = makeProxyFetch();
  global.fetch = proxy.fetchImpl;
  proxy.register("GET", /\/api\/proxy\/bundles(\?|$)/, () => [
    { id: "bundle-1", name: "Onboarding Bundle", description: "", roles: [], created_at: "" },
  ]);
});

afterEach(() => {
  proxy.fetchImpl.mockClear?.();
});

function renderCascades() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, staleTime: 0, gcTime: 0 } },
  });
  return render(
    <QueryClientProvider client={client}>
      <NameResolverProvider>
        <RecentCascades />
      </NameResolverProvider>
    </QueryClientProvider>,
  );
}

describe("RecentCascades", () => {
  it("renders cascade rows from a stubbed fetch", async () => {
    proxy.register("GET", /\/api\/proxy\/propagations\/cascades(\?|$)/, () => ({
      cascades: [
        {
          id: "c-1",
          op_type: "add",
          user_id: USER_ID,
          project_id: PROJECT_ID,
          role_keys: ["mentor"],
          source: "rule",
          source_ref: "rule-1",
          status: "applied",
          completed_at: new Date().toISOString(),
        },
      ],
    }));

    renderCascades();

    await screen.findByText(/Recent cascades \(1\)/);
    expect(screen.getByText(/mentor/)).toBeInTheDocument();
    expect(screen.getByText(/Mapping rule/)).toBeInTheDocument();
    // Rule source has no bundle-name resolver — the raw source_ref is shown.
    expect(screen.getByText(/rule-1/)).toBeInTheDocument();
  });

  it("resolves a bundle source_ref to its name", async () => {
    proxy.register("GET", /\/api\/proxy\/propagations\/cascades(\?|$)/, () => ({
      cascades: [
        {
          id: "c-2",
          op_type: "add",
          user_id: USER_ID,
          project_id: PROJECT_ID,
          role_keys: ["mentor"],
          source: "bundle",
          source_ref: "bundle-1",
          status: "applied",
          completed_at: new Date().toISOString(),
        },
      ],
    }));

    renderCascades();

    await screen.findByText(/Recent cascades \(1\)/);
    expect(await screen.findByText(/Onboarding Bundle/)).toBeInTheDocument();
  });

  it("renders the empty state when no cascades have applied yet", async () => {
    proxy.register("GET", /\/api\/proxy\/propagations\/cascades(\?|$)/, () => ({ cascades: [] }));

    renderCascades();

    await waitFor(() => {
      expect(screen.getByText(/No cascades yet/)).toBeInTheDocument();
    });
  });
});
