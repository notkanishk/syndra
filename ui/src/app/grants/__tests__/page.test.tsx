// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import GrantsClient from "@/components/grants/GrantsClient";
import { NameResolverProvider } from "@/lib/queries/useNameResolver";
import { UUID_REGEX, makeProxyFetch } from "@/test-utils/proxyFetch";

vi.mock("@/lib/toast", () => ({
  toastSuccess: vi.fn(),
  toastError: vi.fn(),
  toastInfo: vi.fn(),
}));

const USER_ID = "11111111-1111-4111-8111-111111111111";
const PROJECT_ID = "22222222-2222-4222-8222-222222222222";

let proxy: ReturnType<typeof makeProxyFetch>;

beforeEach(() => {
  proxy = makeProxyFetch();
  global.fetch = proxy.fetchImpl;

  // One Zitadel grant with two roles, one of which is missing on the
  // MkAuth side — produces a drift entry below.
  proxy.register("GET", /\/api\/proxy\/zitadel\/grants/, () => ({
    items: [
      { id: "g-1", userId: USER_ID, projectId: PROJECT_ID, roleKeys: ["viewer", "editor"] },
    ],
    total: 1,
    limit: 500,
    offset: 0,
  }));

  // Reconciliation diff: one drift entry — MkAuth has [viewer], Zitadel has
  // [viewer, editor]. So `editor` is "Only in Zitadel" relative to MkAuth.
  proxy.register("GET", /\/api\/proxy\/reconciliation\/grants/, () => ({
    only_in_mkauth: [],
    only_in_zitadel: [],
    drift: [
      {
        user_id: USER_ID,
        project_id: PROJECT_ID,
        mkauth_roles: ["viewer"],
        zitadel_roles: ["editor", "viewer"],
        only_in_mkauth: [],
        only_in_zitadel: ["editor"],
        grant_id: "g-1",
      },
    ],
    generated_at: new Date().toISOString(),
    truncated: false,
  }));

  proxy.register("GET", /\/api\/proxy\/rules\/mapping/, () => []);

  // Name resolution reads the full catalog: users + projects + nested roles.
  proxy.register("GET", /\/api\/proxy\/catalog(\?|$)/, () => ({
    users: [{ id: USER_ID, name: "Sam Patel", email: "sam@ex.org" }],
    projects: [
      {
        id: PROJECT_ID,
        name: "Lab Ops",
        kind: "managed",
        description: "",
        roles: [
          { key: "viewer", label: "Viewer", description: "" },
          { key: "editor", label: "Editor", description: "" },
        ],
      },
    ],
    applications: [],
  }));
});

afterEach(() => {
  vi.restoreAllMocks();
});

function renderGrants() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, staleTime: 0, gcTime: 0 } },
  });
  return render(
    <QueryClientProvider client={client}>
      <NameResolverProvider>
        <GrantsClient />
      </NameResolverProvider>
    </QueryClientProvider>,
  );
}

describe("GrantsClient (Stage 4)", () => {
  it("All grants tab tags zitadel-only roles distinctly from mkauth roles", async () => {
    const { container } = renderGrants();

    // Wait for data + name resolution to settle.
    await screen.findAllByText(/Sam Patel/);

    // Two rows: one viewer (mkauth — both sides agree on viewer) and one
    // editor (zitadel-only — drift entry's only_in_zitadel).
    const labels = screen.getAllByText(/MkAuth \+ Zitadel|Zitadel only/);
    expect(labels.length).toBeGreaterThanOrEqual(2);

    // Source-pill copy must include both labels.
    expect(screen.getAllByText("MkAuth + Zitadel").length).toBeGreaterThan(0);
    expect(screen.getAllByText("Zitadel only").length).toBeGreaterThan(0);

    expect(UUID_REGEX.test(container.textContent ?? "")).toBe(false);
  });

  it("Reconciliation tab shows the drift count and opens the Drawer on row click", async () => {
    renderGrants();
    await screen.findAllByText(/Sam Patel/);

    fireEvent.click(screen.getByRole("tab", { name: /Reconciliation/i }));

    // Drift summary card with count = 1 — "Role mismatch" appears in both the
    // count card and the section header, so assert via findAllByText.
    const mismatchLabels = await screen.findAllByText(/Role mismatch/);
    expect(mismatchLabels.length).toBeGreaterThanOrEqual(2);

    // Click the drift row (multiple "View ▸" buttons may exist; first is the
    // single drift entry seeded above).
    const viewButtons = await screen.findAllByRole("button", { name: /View ▸/ });
    fireEvent.click(viewButtons[0]);

    // Drawer should render the JsonView with both sides
    await screen.findByRole("dialog");
    await screen.findAllByText(/MkAuth-side/i);
    await screen.findAllByText(/Zitadel-side/i);
  });
});
