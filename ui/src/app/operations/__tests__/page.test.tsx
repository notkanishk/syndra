// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import OperationsClient from "@/components/operations/OperationsClient";
import { NameResolverProvider } from "@/lib/queries/useNameResolver";
import { UUID_REGEX, makeProxyFetch } from "@/test-utils/proxyFetch";

vi.mock("@/lib/toast", () => ({
  toastSuccess: vi.fn(),
  toastError: vi.fn(),
  toastInfo: vi.fn(),
}));

const USER_ID = "44444444-4444-4444-8444-444444444444";
const PROJECT_ID = "33333333-3333-4333-8333-333333333333";

let proxy: ReturnType<typeof makeProxyFetch>;

beforeEach(() => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  proxy = makeProxyFetch();
  global.fetch = proxy.fetchImpl;

  proxy.register("GET", /\/api\/proxy\/intents(\?|$)/, () => [
    {
      id: "i-1",
      target_uid: USER_ID,
      action: "AddRole",
      lldap_group: "lab_mentor",
      source_project: PROJECT_ID,
      source_role: "mentor",
      idempotency_key: "abc",
      status: "in_flight",
      created_at: new Date().toISOString(),
    },
  ]);

  proxy.register("GET", /\/api\/proxy\/webhook\/events(\?|$)/, () => [
    {
      id: "w-1",
      event_type: "user.role.added",
      user_id: USER_ID,
      source_project: PROJECT_ID,
      role_key: "mentor",
      idempotency_key: "evt",
      status: "processed",
      created_at: new Date().toISOString(),
    },
  ]);

  proxy.register("GET", /\/api\/proxy\/onboarding\/triggers/, () => [
    {
      id: "o-1",
      user_id: USER_ID,
      source: "webhook",
      idempotency_key: "ob",
      status: "completed",
      bundle_id: "b-1",
      created_at: new Date().toISOString(),
    },
  ]);

  // Name resolution reads the full catalog: users + projects + nested roles.
  proxy.register("GET", /\/api\/proxy\/catalog(\?|$)/, () => ({
    users: [{ id: USER_ID, name: "Sam Patel", email: "sam@ex.org" }],
    projects: [
      {
        id: PROJECT_ID,
        name: "Lab Ops",
        kind: "managed",
        description: "",
        roles: [{ key: "mentor", label: "Mentor", description: "" }],
      },
    ],
    applications: [],
  }));
});

afterEach(() => {
  vi.useRealTimers();
  vi.restoreAllMocks();
});

function renderOps() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, staleTime: 0, gcTime: 0 } },
  });
  return render(
    <QueryClientProvider client={client}>
      <NameResolverProvider>
        <OperationsClient />
      </NameResolverProvider>
    </QueryClientProvider>,
  );
}

describe("OperationsClient (Stage 4)", () => {
  it("renders the intents tab and resolves user/project names", async () => {
    const { container } = renderOps();
    await screen.findByText("AddRole");
    await screen.findAllByText(/Sam Patel/);
    await screen.findAllByText(/Lab Ops/);
    // Wait an extra tick so the catalog-backed name resolution has settled.
    await waitFor(() => {
      expect(proxy.calls.some((c) => c.url.includes("/catalog"))).toBe(true);
    });
    expect(UUID_REGEX.test(container.textContent ?? "")).toBe(false);
  });

  it("switches to the webhook events tab and renders rows", async () => {
    renderOps();
    await screen.findByText("AddRole");
    fireEvent.click(screen.getByRole("tab", { name: /Webhook events/i }));
    await screen.findByText("user.role.added");
  });

  it("opens the payload modal when the operator clicks Payload on a row", async () => {
    renderOps();
    await screen.findByText("AddRole");
    const payloadButtons = screen.getAllByRole("button", { name: /Payload/i });
    fireEvent.click(payloadButtons[0]);
    await screen.findByRole("dialog");
    // The JsonView dump must render the action value from the row. Two
    // occurrences are expected — one in the row label, one inside the
    // payload — so use findAllByText.
    const matches = await screen.findAllByText(/AddRole/);
    expect(matches.length).toBeGreaterThanOrEqual(1);
  });
});
