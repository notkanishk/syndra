// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it } from "vitest";

import { NameResolverProvider } from "@/lib/queries/useNameResolver";
import { makeProxyFetch } from "@/test-utils/proxyFetch";

import { PendingPropagationsClient } from "../PendingPropagationsClient";

const USER_ID = "11111111-1111-4111-8111-111111111111";
const PROJECT_ID = "22222222-2222-4222-8222-222222222222";

let proxy: ReturnType<typeof makeProxyFetch>;

beforeEach(() => {
  proxy = makeProxyFetch();
  global.fetch = proxy.fetchImpl;
  proxy.register("POST", /\/api\/proxy\/lookup/, () => ({
    users: {}, projects: {}, roles: {}, bundles: { "bundle-1": { name: "Onboarding Bundle" } },
  }));
});

afterEach(() => {
  proxy.fetchImpl.mockClear?.();
});

function renderPending() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, staleTime: 0, gcTime: 0 } },
  });
  return render(
    <QueryClientProvider client={client}>
      <NameResolverProvider>
        <PendingPropagationsClient />
      </NameResolverProvider>
    </QueryClientProvider>,
  );
}

describe("PendingPropagationsClient", () => {
  it("shows the source and resolved bundle name for a cascade-originated row", async () => {
    proxy.register("GET", /\/api\/proxy\/propagations(\?|$)/, () => ({
      pending: [
        {
          id: "p-1",
          op_type: "add",
          user_id: USER_ID,
          project_id: PROJECT_ID,
          role_keys: ["mentor"],
          source: "bundle",
          source_ref: "bundle-1",
          status: "pending",
          attempts: 0,
          created_at: new Date().toISOString(),
        },
      ],
    }));

    renderPending();

    await screen.findByText(/Awaiting Zitadel \(1\)/);
    expect(screen.getByText("Bundle")).toBeInTheDocument();
    expect(await screen.findByText(/Onboarding Bundle/)).toBeInTheDocument();
  });

  it("shows the raw source_ref for a rule-originated row (no rule-name resolver)", async () => {
    proxy.register("GET", /\/api\/proxy\/propagations(\?|$)/, () => ({
      pending: [
        {
          id: "p-2",
          op_type: "revoke",
          user_id: USER_ID,
          project_id: PROJECT_ID,
          role_keys: ["mentor"],
          source: "rule",
          source_ref: "rule-9",
          status: "pending",
          attempts: 0,
          created_at: new Date().toISOString(),
        },
      ],
    }));

    renderPending();

    await screen.findByText(/Awaiting Zitadel \(1\)/);
    expect(screen.getByText("Mapping rule")).toBeInTheDocument();
    expect(screen.getByText(/rule-9/)).toBeInTheDocument();
  });

  it("omits the attribution span for a direct grant with no source_ref", async () => {
    proxy.register("GET", /\/api\/proxy\/propagations(\?|$)/, () => ({
      pending: [
        {
          id: "p-3",
          op_type: "add",
          user_id: USER_ID,
          project_id: PROJECT_ID,
          role_keys: ["mentor"],
          source: "direct",
          status: "pending",
          attempts: 0,
          created_at: new Date().toISOString(),
        },
      ],
    }));

    renderPending();

    await screen.findByText(/Awaiting Zitadel \(1\)/);
    expect(screen.getByText("Direct")).toBeInTheDocument();
    expect(screen.queryByText(/via/)).not.toBeInTheDocument();
  });
});
