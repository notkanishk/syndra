// @vitest-environment jsdom
import { cleanup, render, screen, within } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import ServiceAccountsPage from "@/app/system/service-accounts/page";
import type { ServiceAccount } from "@/lib/types";

const list = vi.hoisted(() => ({
  state: {} as { data?: ServiceAccount[]; isLoading: boolean; error: unknown },
}));

vi.mock("@/lib/queries/useServiceAccounts", async (original) => ({
  ...(await original<typeof import("@/lib/queries/useServiceAccounts")>()),
  useServiceAccounts: () => ({ ...list.state, refetch: () => {}, dataUpdatedAt: 0, isError: false }),
}));

function account(overrides: Partial<ServiceAccount> = {}): ServiceAccount {
  return {
    id: "svc1",
    name: "netbird",
    kind: "integration",
    zitadel_type: "machine",
    purpose: "",
    owner_id: "",
    role_count: 3,
    marked_by: "",
    marked_at: null,
    ...overrides,
  };
}

beforeEach(() => {
  list.state = { data: [], isLoading: false, error: null };
});
afterEach(cleanup);

describe("Service accounts list", () => {
  it("says what the account is under its name, and that the name opens its page", () => {
    list.state.data = [
      account(),
      account({ id: "kiosk", name: "Ingestion Station", kind: "device", zitadel_type: "human", purpose: "Intake kiosk" }),
    ];
    render(<ServiceAccountsPage />);

    expect(screen.getByRole("link", { name: "netbird" })).toHaveAttribute("href", "/system/service-accounts/svc1");
    expect(screen.getByText("Integration · machine key")).toBeInTheDocument();
    expect(screen.getByText("Device · signs in")).toBeInTheDocument();
    expect(screen.getByText("Intake kiosk")).toBeInTheDocument();
  });

  it("counts the accounts at the top, and shows each one's role count", () => {
    list.state.data = [account({ role_count: 3 }), account({ id: "b", name: "zed", role_count: 0 })];
    render(<ServiceAccountsPage />);

    expect(screen.getByTestId("card-count")).toHaveTextContent("2");
    const row = screen.getByRole("link", { name: "netbird" }).closest("[data-account-row]") as HTMLElement;
    expect(within(row).getByText("3")).toBeInTheDocument();
  });

  it("names the person who answers for it, linked, or says there is nobody", () => {
    list.state.data = [account({ owner_id: "u9" }), account({ id: "b", name: "zed" })];
    render(<ServiceAccountsPage />);

    const owned = screen.getByRole("link", { name: "netbird" }).closest("[data-account-row]") as HTMLElement;
    // The owner id is never the label: an unplaced person reads as the fallback, with the id
    // beside it to look up in Zitadel (A-07 §7), linked to their page.
    expect(within(owned).getByText("Unknown account")).toBeInTheDocument();
    expect(owned.querySelector('a[href="/users/u9"]')).not.toBeNull();

    const orphan = screen.getByRole("link", { name: "zed" }).closest("[data-account-row]") as HTMLElement;
    expect(within(orphan).getByText("No owner")).toBeInTheDocument();
  });

  it("is a list in each of its four states", () => {
    list.state = { data: undefined, isLoading: false, error: new Error("down") };
    const { rerender } = render(<ServiceAccountsPage />);
    expect(screen.getByText("The list of service accounts could not be read.")).toBeInTheDocument();

    list.state = { data: [], isLoading: false, error: null };
    rerender(<ServiceAccountsPage />);
    expect(screen.getByText("No service accounts.")).toBeInTheDocument();
    expect(screen.getByTestId("card-count")).toHaveTextContent("0");
  });
});
