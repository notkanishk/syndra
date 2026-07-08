// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { Mock } from "vitest";

import Users from "@/components/zitadel/Users";
import { makeProxyFetch } from "@/test-utils/proxyFetch";

vi.mock("@/lib/queries/usePropagation", () => ({
  usePendingPropagations: vi.fn(),
}));
import { usePendingPropagations } from "@/lib/queries/usePropagation";

const U1 = "u1";
const P1 = "p1";

let proxy: ReturnType<typeof makeProxyFetch>;

beforeEach(() => {
  proxy = makeProxyFetch();
  global.fetch = proxy.fetchImpl;

  proxy.register("GET", /\/api\/proxy\/zitadel\/users(\?|$)/, () => ({
    items: [{ id: U1, userName: "sam", displayName: "Sam Patel", email: "sam@ex.org", state: "active" }],
    total: 1,
    limit: 500,
    offset: 0,
  }));
  proxy.register("GET", /\/api\/proxy\/zitadel\/projects(\?|$)/, () => ({
    items: [{ id: P1, name: "Lab Ops", state: "active" }],
    total: 1,
    limit: 500,
    offset: 0,
  }));
  proxy.register("GET", /\/api\/proxy\/zitadel\/users\/u1\/grants/, () => ({
    items: [
      { id: "g1", userId: U1, projectId: P1, roleKeys: ["roleA"] },
      { id: "g2", userId: U1, projectId: P1, roleKeys: ["roleB"] },
    ],
    total: 2,
    limit: 500,
    offset: 0,
  }));

  // One pending add covers (u1, p1, roleA) only.
  (usePendingPropagations as Mock).mockReturnValue({
    data: [
      {
        id: "p-1",
        op_type: "add",
        user_id: U1,
        project_id: P1,
        role_keys: ["roleA"],
        source: "manual",
        status: "pending",
        attempts: 0,
        created_at: "",
      },
    ],
  });
});

afterEach(() => {
  vi.restoreAllMocks();
});

function renderUsers() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, staleTime: 0, gcTime: 0 } },
  });
  return render(
    <QueryClientProvider client={client}>
      <Users />
    </QueryClientProvider>,
  );
}

describe("Users pending 'Awaiting Zitadel' tag (§5.2)", () => {
  it("tags a grant awaiting Zitadel and leaves an unrelated grant untagged", async () => {
    renderUsers();
    // Select the user to load its grants.
    const userSelect = await screen.findByRole("combobox");
    fireEvent.change(userSelect, { target: { value: U1 } });

    // Grants render (project name resolved from the projects list).
    await screen.findByText("Lab Ops");
    await screen.findByText("roleA");
    await screen.findByText("roleB");

    // Exactly one tag — on the roleA grant, not the roleB one.
    const tags = await screen.findAllByText(/Awaiting Zitadel/);
    expect(tags).toHaveLength(1);
  });
});
