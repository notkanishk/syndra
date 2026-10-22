// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { request } from "@/lib/api-client";
import {
  useSaveServiceAccount,
  useServiceAccount,
  useServiceAccounts,
  useUnmarkServiceAccount,
} from "@/lib/queries/useServiceAccounts";
import { usersQueryKeys } from "@/lib/queries/useUsers";

vi.mock("@/lib/api-client", () => ({ request: vi.fn() }));

const account = {
  id: "u1",
  name: "netbird",
  kind: "integration",
  zitadel_type: "machine",
  purpose: "",
  owner_id: "",
  role_count: 2,
  marked_by: "",
  marked_at: null,
};

function setup() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const wrapper = ({ children }: { children: React.ReactNode }) => (
    <QueryClientProvider client={client}>{children}</QueryClientProvider>
  );
  return { client, wrapper };
}

beforeEach(() => vi.mocked(request).mockReset());

describe("service accounts", () => {
  it("reads the list as the accounts array, and one by id", async () => {
    vi.mocked(request).mockResolvedValueOnce({ accounts: [account] }).mockResolvedValueOnce(account);
    const { wrapper } = setup();
    const list = renderHook(() => useServiceAccounts(), { wrapper });
    await waitFor(() => expect(list.result.current.data).toEqual([account]));
    expect(request).toHaveBeenCalledWith("/service-accounts");

    const one = renderHook(() => useServiceAccount("u1"), { wrapper });
    await waitFor(() => expect(one.result.current.data).toEqual(account));
    expect(request).toHaveBeenLastCalledWith("/service-accounts/u1");
  });

  it("marking or changing is one PUT with kind, purpose and owner; the lists it moves things between refresh", async () => {
    vi.mocked(request).mockResolvedValue(account);
    const { client, wrapper } = setup();
    client.setQueryData(["service-accounts", "list"], { accounts: [] });
    client.setQueryData(usersQueryKeys.list(""), []);
    client.setQueryData(["governance", "indicators"], {});

    const { result } = renderHook(() => useSaveServiceAccount("u1"), { wrapper });
    result.current.mutate({ kind: "device", purpose: "Door kiosk", owner_id: "u9" });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(request).toHaveBeenCalledWith("/service-accounts/u1", {
      method: "PUT",
      body: { kind: "device", purpose: "Door kiosk", owner_id: "u9" },
    });
    expect(client.getQueryState(["service-accounts", "list"])?.isInvalidated).toBe(true);
    // The account leaves (or rejoins) People and the people counts.
    expect(client.getQueryState(usersQueryKeys.list(""))?.isInvalidated).toBe(true);
    expect(client.getQueryState(["governance", "indicators"])?.isInvalidated).toBe(true);
  });

  it("Mark as a person is a DELETE and refreshes the same things", async () => {
    vi.mocked(request).mockResolvedValue({ status: "ok" });
    const { client, wrapper } = setup();
    client.setQueryData(["service-accounts", "one", "u1"], account);
    client.setQueryData(usersQueryKeys.list(""), []);

    const { result } = renderHook(() => useUnmarkServiceAccount("u1"), { wrapper });
    result.current.mutate();
    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(request).toHaveBeenCalledWith("/service-accounts/u1", { method: "DELETE" });
    expect(client.getQueryState(["service-accounts", "one", "u1"])?.isInvalidated).toBe(true);
    expect(client.getQueryState(usersQueryKeys.list(""))?.isInvalidated).toBe(true);
  });
});
