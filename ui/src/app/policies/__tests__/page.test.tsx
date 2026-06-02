// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import PoliciesView from "@/app/policies/page";
import { NameResolverProvider } from "@/lib/queries/useNameResolver";
import { UUID_REGEX, makeProxyFetch } from "@/test-utils/proxyFetch";

vi.mock("@/lib/toast", () => ({
  toastSuccess: vi.fn(),
  toastError: vi.fn(),
  toastInfo: vi.fn(),
}));

const SOURCE_PROJECT = "11111111-1111-4111-8111-111111111111";
const TARGET_PROJECT = "22222222-2222-4222-8222-222222222222";

let proxy: ReturnType<typeof makeProxyFetch>;

beforeEach(() => {
  proxy = makeProxyFetch();
  global.fetch = proxy.fetchImpl;

  proxy.register("GET", /\/api\/proxy\/rules\/mapping(\?|$)/, () => [
    {
      id: "rule-1",
      source_project: SOURCE_PROJECT,
      source_role: "mentor",
      target_project: TARGET_PROJECT,
      target_role: "trainee",
      created_at: new Date().toISOString(),
    },
  ]);

  proxy.register("GET", /\/api\/proxy\/catalog/, () => ({
    projects: [
      {
        id: SOURCE_PROJECT,
        name: "Lab Ops",
        roles: [{ key: "mentor", label: "Mentor" }],
      },
      {
        id: TARGET_PROJECT,
        name: "Workshop",
        roles: [{ key: "trainee", label: "Trainee" }],
      },
    ],
  }));

  proxy.register("POST", /\/api\/proxy\/lookup/, ({ body }) => {
    const projectIds = (body as { project_ids?: string[] } | undefined)?.project_ids ?? [];
    const roleKeys =
      (body as { role_keys?: Array<{ project_id: string; role_key: string }> } | undefined)
        ?.role_keys ?? [];
    const projects: Record<string, { name: string }> = {};
    if (projectIds.includes(SOURCE_PROJECT)) projects[SOURCE_PROJECT] = { name: "Lab Ops" };
    if (projectIds.includes(TARGET_PROJECT)) projects[TARGET_PROJECT] = { name: "Workshop" };
    const roles: Record<string, { display_name: string }> = {};
    for (const rk of roleKeys) {
      if (rk.project_id === SOURCE_PROJECT && rk.role_key === "mentor") {
        roles[`${rk.project_id}:${rk.role_key}`] = { display_name: "Mentor" };
      }
      if (rk.project_id === TARGET_PROJECT && rk.role_key === "trainee") {
        roles[`${rk.project_id}:${rk.role_key}`] = { display_name: "Trainee" };
      }
    }
    return { users: {}, projects, roles, bundles: {} };
  });
});

afterEach(() => {
  vi.restoreAllMocks();
});

function renderPolicies() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, staleTime: 0, gcTime: 0 } },
  });
  return render(
    <QueryClientProvider client={client}>
      <NameResolverProvider>
        <PoliciesView />
      </NameResolverProvider>
    </QueryClientProvider>,
  );
}

describe("PoliciesView (Stage 3)", () => {
  it("renders source and target project/role names from the lookup batch", async () => {
    renderPolicies();
    await screen.findAllByText(/Lab Ops/);
    await screen.findAllByText(/Workshop/);
    await screen.findAllByText(/Mentor/);
    await screen.findAllByText(/Trainee/);
  });

  it("opens the create-rule modal from the toolbar", async () => {
    renderPolicies();
    const trigger = await screen.findByRole("button", { name: /^\+ New rule$/ });
    fireEvent.click(trigger);
    expect(
      await screen.findByRole("dialog", { name: /Create mapping rule/i }),
    ).toBeInTheDocument();
  });

  it("never renders raw Zitadel UUIDs in the rule list once lookups resolve", async () => {
    const { container } = renderPolicies();
    await screen.findAllByText(/Lab Ops/);
    await waitFor(() => {
      expect(proxy.calls.some((c) => c.url.includes("/lookup"))).toBe(true);
    });
    const text = container.textContent ?? "";
    expect(UUID_REGEX.test(text)).toBe(false);
  });
});
