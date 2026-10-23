// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { Home } from "@/components/home/Home";
import type { SessionUser } from "@/lib/session";
import { setMediaQuery } from "@/test-utils/media";

/**
 * A-07 §31–33 (23 Oct): service accounts nobody answers for are one Home block
 * in When you're ready, and the new people without a bundle are newest joined
 * first, each with their own Give a bundle that opens People's bulk flow for
 * that one person.
 */
const state = vi.hoisted(() => ({
  accounts: [] as Array<{ id: string; name: string; owner_id: string }>,
  missed: [] as Array<{ user_id: string; name: string; email: string; joined?: string }>,
  dialog: null as null | { op: string; userIds: string[] },
  expiring: [] as unknown[],
  extended: { outbox_id: "o1", status: "done" } as { outbox_id: string; status: string },
}));

vi.mock("@/lib/queries/useGovernance", () => ({
  useGovernanceSummary: () => ({
    data: {
      pending_requests: [],
      expiring_grants: state.expiring,
      cleanup_hints: [],
      pending_propagation: { count: 0, zitadel_reachable: true },
      drift: { count: 0, top: [] },
      unreconciled_targets: [],
      merge_findings: 0,
    },
    isLoading: false,
    error: null,
    refetch: vi.fn(),
  }),
}));
vi.mock("@/lib/queries/useRequests", () => {
  const none: never[] = [];
  return {
    useRequestsAdmin: () => ({ data: none, isLoading: false, error: null, refetch: vi.fn() }),
    useDecideRequest: () => ({ mutate: vi.fn(), isPending: false }),
  };
});
vi.mock("@/lib/queries/usePropagation", () => ({
  useDrainPropagations: () => ({ mutateAsync: vi.fn(), isPending: false }),
}));
vi.mock("@/lib/queries/useUsers", () => ({
  useCreateGrant: () => ({ mutateAsync: async () => state.extended, isPending: false }),
  useUserAccess: () => ({ data: undefined }),
}));
vi.mock("@/lib/queries/useTargets", () => ({ useTargets: () => ({ data: [], isLoading: false }) }));
vi.mock("@/lib/queries/useOperations", () => ({
  useMissedOnboarding: () => ({
    data: { welcome_bundle_configured: true, missed: state.missed },
    isLoading: false,
    error: null,
    refetch: vi.fn(),
  }),
}));
vi.mock("@/lib/queries/useServiceAccounts", () => ({
  useServiceAccounts: () => ({ data: state.accounts, isLoading: false, error: null }),
  serviceAccountHref: (id: string) => `/system/service-accounts/${id}`,
}));
vi.mock("@/components/people/BulkDialog", () => ({
  BulkDialog: (props: { op: string; userIds: string[]; onClose: () => void }) => {
    state.dialog = { op: props.op, userIds: props.userIds };
    return <button onClick={props.onClose}>close the dialog</button>;
  },
}));
vi.mock("@/lib/ui-view", () => ({ useIsAdvanced: () => false }));
vi.mock("@/components/home/Makerspace", () => ({ Makerspace: () => <div /> }));
vi.mock("@/components/names", async () => {
  const actual = await vi.importActual<typeof import("@/components/names")>("@/components/names");
  return {
    ...actual,
    UserName: ({ id }: { id: string | null | undefined }) => <span>{id}</span>,
    UserAvatar: () => <span />,
    RoleRef: () => <span>a role</span>,
  };
});

const session = { id: "u1", name: "Ada", email: "ada@example.org" } as unknown as SessionUser;

function renderHome() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <Home session={session} />
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  state.accounts = [];
  state.missed = [];
  state.dialog = null;
  state.expiring = [];
  state.extended = { outbox_id: "o1", status: "done" };
  setMediaQuery("(max-width: 44.99rem)", false);
});

describe("Service accounts to look at", () => {
  it("stays out of Home when every service account has an owner", async () => {
    state.accounts = [{ id: "s1", name: "kiosk", owner_id: "u9" }];
    renderHome();
    await screen.findByText(/last checked/);
    expect(screen.queryByText("Service accounts to look at")).toBeNull();
  });

  it("lists an account nobody answers for, linked, with a warn dot and No owner", async () => {
    state.accounts = [
      { id: "s1", name: "kiosk", owner_id: "u9" },
      { id: "s2", name: "door panel", owner_id: "" },
    ];
    renderHome();

    expect(await screen.findByText("Service accounts to look at")).toBeTruthy();
    expect(screen.getByRole("link", { name: "Open Service accounts →" })).toHaveAttribute("href", "/system/service-accounts");
    expect(screen.getByRole("link", { name: "door panel" })).toHaveAttribute(
      "href",
      "/system/service-accounts/s2",
    );
    expect(screen.queryByText("kiosk")).toBeNull();
    expect(screen.getByText("No owner")).toBeTruthy();
    expect(document.querySelector('[data-testid="card-count"]')?.textContent).toBe("1");
  });

  it("counts as one block in the headline", async () => {
    state.accounts = ["a", "b", "c"].map((id) => ({ id, name: id, owner_id: "" }));
    renderHome();
    expect(await screen.findByText("One thing needs you.")).toBeTruthy();
  });

  it("holds five rows, then links the rest to Service accounts", async () => {
    state.accounts = Array.from({ length: 8 }, (_, i) => ({ id: `s${i}`, name: `acct${i}`, owner_id: "" }));
    renderHome();
    await screen.findByText("Service accounts to look at");
    expect(screen.getAllByText("No owner")).toHaveLength(5);
    expect(screen.getByRole("link", { name: "and 3 more" })).toHaveAttribute(
      "href",
      "/system/service-accounts",
    );
  });
});

describe("Extend on Home says what the grant endpoint answered", () => {
  const grant = {
    id: "g1",
    user_id: "u-ex",
    project_id: "pLaser",
    role_key: "trained",
    expires_at: "2026-11-01T00:00:00Z",
  };

  it("says extended when it went through", async () => {
    state.expiring = [grant];
    renderHome();
    fireEvent.click(await screen.findByRole("button", { name: "Extend" }));
    expect(await screen.findByText("Extended by 90 days.")).toBeTruthy();
  });

  it("says it waits to be sent when the endpoint answers pending", async () => {
    state.expiring = [grant];
    state.extended = { outbox_id: "o1", status: "pending" };
    renderHome();
    fireEvent.click(await screen.findByRole("button", { name: "Extend" }));
    expect(await screen.findByText("Will extend by 90 days once sent.")).toBeTruthy();
    expect(screen.queryByText("Extended by 90 days.")).toBeNull();
  });
});

describe("New people without a bundle", () => {
  it("lists the newest joined first and a person with no join time last", async () => {
    state.missed = [
      { user_id: "u-old", name: "Old", email: "o@example.edu", joined: "2026-09-01T00:00:00Z" },
      { user_id: "u-none", name: "None", email: "n@example.edu" },
      { user_id: "u-new", name: "New", email: "w@example.edu", joined: "2026-10-20T00:00:00Z" },
    ];
    renderHome();
    await screen.findByText("New people without a bundle");
    const names = screen.getAllByRole("link", { name: /^u-/ }).map((node) => node.textContent);
    expect(names).toEqual(["u-new", "u-old", "u-none"]);
  });

  it("gives each row its own Give a bundle, opening the bulk flow for that one person", async () => {
    state.missed = [
      { user_id: "u-a", name: "A", email: "a@example.edu", joined: "2026-10-20T00:00:00Z" },
      { user_id: "u-b", name: "B", email: "b@example.edu", joined: "2026-10-10T00:00:00Z" },
    ];
    renderHome();
    await screen.findByText("New people without a bundle");

    const buttons = screen.getAllByRole("button", { name: "Give a bundle" });
    expect(buttons).toHaveLength(2);
    fireEvent.click(buttons[1]);

    expect(state.dialog).toEqual({ op: "assign_bundle", userIds: ["u-b"] });

    fireEvent.click(screen.getByText("close the dialog"));
    expect(screen.queryByText("close the dialog")).toBeNull();
  });
});
