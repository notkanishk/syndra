// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { ServiceAccountPage } from "@/components/system/ServiceAccountPage";
import { ApiError } from "@/lib/api-client";
import type { ServiceAccount } from "@/lib/types";

const read = vi.hoisted(() => ({
  account: {} as { data?: ServiceAccount; isLoading: boolean; error: unknown },
  access: {} as { data?: unknown; isLoading: boolean; error: unknown },
}));

vi.mock("@/lib/queries/useServiceAccounts", async (original) => ({
  ...(await original<typeof import("@/lib/queries/useServiceAccounts")>()),
  useServiceAccount: () => ({ ...read.account, refetch: () => {} }),
  useSaveServiceAccount: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useUnmarkServiceAccount: () => ({ mutateAsync: vi.fn(), isPending: false }),
}));
vi.mock("@/lib/queries/useUsers", () => ({
  useUserAccess: () => ({ ...read.access, refetch: () => {} }),
  useUserGrants: () => ({ data: [{ id: "g1", project_id: "pLaser", role_key: "trained", granted_by: "u1", reason: "" }], isLoading: false, error: null }),
  useUsers: () => ({ data: [], isLoading: false }),
}));

function account(overrides: Partial<ServiceAccount> = {}): ServiceAccount {
  return {
    id: "svc1",
    name: "netbird",
    kind: "integration",
    zitadel_type: "machine",
    purpose: "Syncs the door list",
    owner_id: "",
    role_count: 1,
    marked_by: "",
    marked_at: null,
    ...overrides,
  };
}

const holds = {
  user: { id: "svc1", name: "netbird" },
  bundles: [],
  allowances: [],
  cleanup_hints: [],
  observation: {},
  projects: [
    {
      project_id: "pLaser",
      project_name: "Laser Lab",
      project_name_resolved: true,
      source_roles: [{ role_key: "trained", reasons: [{ kind: "direct", description: "" }] }],
      derived_roles: [],
      effective_role_keys: ["trained"],
      observed_role_keys: null,
    },
  ],
};

function renderPage() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <ServiceAccountPage id="svc1" />
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  read.account = { data: account(), isLoading: false, error: null };
  read.access = { data: holds, isLoading: false, error: null };
});
afterEach(cleanup);

describe("a service account's page", () => {
  it("heads with its name, what it is and how it signs in, and its purpose as the lede", () => {
    renderPage();
    expect(screen.getByRole("heading", { level: 1, name: "netbird" })).toBeInTheDocument();
    expect(screen.getByText("Machine key")).toBeInTheDocument();
    expect(screen.getAllByText("Syncs the door list").length).toBeGreaterThan(0);
  });

  it("is a card of settings: Purpose and Owner you can change, Kind that Zitadel sets for a machine account", () => {
    renderPage();
    const card = screen.getByText("This account").closest(".card") as HTMLElement;

    expect(within(card).getByRole("button", { name: "Change purpose" })).toBeInTheDocument();
    expect(within(card).getByText("No owner")).toBeInTheDocument();
    expect(within(card).getByRole("button", { name: "Change owner" })).toBeInTheDocument();
    expect(within(card).getByText("A machine account in Zitadel: a key, no sign-in")).toBeInTheDocument();
    expect(within(card).getByText("Set by Zitadel")).toBeInTheDocument();
    // Zitadel says what it is; nobody un-marks that.
    expect(within(card).queryByRole("button", { name: "Mark as a person" })).not.toBeInTheDocument();
  });

  it("offers Mark as a person on a marked human-type account, and says who marked it", () => {
    read.account.data = account({
      zitadel_type: "human",
      kind: "device",
      marked_by: "u1",
      marked_at: "2026-10-23T09:00:00Z",
    });
    renderPage();

    expect(screen.getByText("Signs in")).toBeInTheDocument();
    expect(screen.getByText(/^Marked by .*, 23 Oct/)).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Mark as a person" }));
    expect(screen.getByRole("heading", { name: "Mark netbird as a person" })).toBeInTheDocument();
  });

  it("keeps the Unmarked result on screen after the page's own read stops finding the account", async () => {
    read.account.data = account({ zitadel_type: "human", kind: "device", marked_by: "u1", marked_at: "2026-10-23T09:00:00Z" });
    const { rerender } = renderPage();
    fireEvent.click(screen.getByRole("button", { name: "Mark as a person" }));
    fireEvent.click(within(screen.getByRole("dialog")).getByRole("button", { name: "Mark as a person" }));
    // The mutation's invalidation refetches; the account is a person now, so the read 404s.
    read.account = { data: undefined, isLoading: false, error: new ApiError(404, { error: "NOT_FOUND", message: "not found" }) };
    rerender(
      <QueryClientProvider client={new QueryClient()}>
        <ServiceAccountPage id="svc1" />
      </QueryClientProvider>,
    );
    expect(await screen.findByText("Unmarked")).toBeInTheDocument();
    expect(screen.getByText("netbird is a person again.")).toBeInTheDocument();
  });

  it("opens Change purpose as its own dialog", () => {
    renderPage();
    fireEvent.click(screen.getByRole("button", { name: "Change purpose" }));
    expect(screen.getByRole("heading", { name: "Change purpose" })).toBeInTheDocument();
  });

  it("lists what it holds like a person's roles, with Revoke opening the usual removal", () => {
    renderPage();
    const card = screen.getByText("What it holds").closest(".card") as HTMLElement;

    expect(within(card).getByTestId("card-count")).toHaveTextContent("1");
    expect(within(card).getByRole("link", { name: "Laser Lab" })).toHaveAttribute("href", "/projects/pLaser");
    expect(within(card).getByText("Trained")).toBeInTheDocument();
    fireEvent.click(within(card).getByRole("button", { name: /^Revoke Trained in Laser Lab/ }));
    expect(screen.getByRole("dialog")).toHaveTextContent("Revoke direct access to Laser Lab / trained?");
  });

  it("never asks and is never welcomed: no Ask for access, no bundles", () => {
    renderPage();
    expect(screen.queryByText(/ask for access/i)).not.toBeInTheDocument();
    expect(screen.queryByText(/welcome/i)).not.toBeInTheDocument();
  });

  it("says plainly when the id belongs to a person", () => {
    read.account = { data: undefined, isLoading: false, error: new ApiError(404, { error: "NOT_FOUND", message: "not found" }) };
    renderPage();
    expect(screen.getByText("That isn't a service account.")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Open it on People" })).toHaveAttribute("href", "/users/svc1");
  });
});
