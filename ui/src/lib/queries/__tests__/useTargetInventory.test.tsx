// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { ApiError } from "@/lib/api-client";
import { useTargetInventory, useTargetInventoryAgain } from "@/lib/queries/useTargets";

/**
 * A pass that stopped answers 503 with the inventory itself (handlers/inventory.go),
 * so the card can say why. The reader has to take that body as data; a 429's cooldown
 * and every other failure still throw.
 */
let reply: () => Response;
const halted = {
  target: "truenas",
  bound: 0,
  unmanaged: [],
  current: false,
  halted: true,
  reason: "read_refused",
  detail: "Syndra's records of who has which account on TrueNAS can't be read. Check the add-on's storage, then try again.",
};

function wrapper() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const Wrap = ({ children }: { children: ReactNode }) => <QueryClientProvider client={client}>{children}</QueryClientProvider>;
  return { client, Wrap };
}

beforeEach(() => {
  vi.stubGlobal("fetch", vi.fn(async () => reply()));
});
afterEach(() => vi.unstubAllGlobals());

describe("the account list's reader", () => {
  it("keeps the 503 body, so the add-on's sentence reaches the card", async () => {
    reply = () => new Response(JSON.stringify(halted), { status: 503 });
    const { Wrap } = wrapper();
    const { result } = renderHook(() => useTargetInventory("truenas"), { wrapper: Wrap });
    await waitFor(() => expect(result.current.data).toBeDefined());
    expect(result.current.error).toBeNull();
    expect(result.current.data?.halted).toBe(true);
    expect(result.current.data?.detail).toBe(halted.detail);
  });

  it("still fails on any other 503, which is an error and never an empty list", async () => {
    reply = () => new Response(JSON.stringify({ error: "TARGET_UNREACHABLE", message: "TrueNAS did not answer" }), { status: 503 });
    const { Wrap } = wrapper();
    const { result } = renderHook(() => useTargetInventory("truenas"), { wrapper: Wrap });
    await waitFor(() => expect(result.current.error).toBeInstanceOf(ApiError));
    expect((result.current.error as ApiError).code).toBe("TARGET_UNREACHABLE");
    expect(result.current.data).toBeUndefined();
  });

  // The handler's reason for the 503 is `halted`; an inventory-shaped body without it, and a JSON null, are errors.
  it.each([
    ["an inventory-shaped body that is not halted", JSON.stringify({ target: "truenas", bound: 3, unmanaged: [] })],
    ["a JSON null", "null"],
  ])("does not read %s as a halted inventory", async (_name, text) => {
    reply = () => new Response(text, { status: 503 });
    const { Wrap } = wrapper();
    const { result } = renderHook(() => useTargetInventory("truenas"), { wrapper: Wrap });
    await waitFor(() => expect(result.current.error).toBeInstanceOf(ApiError));
    expect(result.current.data).toBeUndefined();
  });

  it("still fails on a 500", async () => {
    reply = () => new Response(JSON.stringify({ error: "INVENTORY_ERROR" }), { status: 500 });
    const { Wrap } = wrapper();
    const { result } = renderHook(() => useTargetInventory("truenas"), { wrapper: Wrap });
    await waitFor(() => expect(result.current.error).toBeInstanceOf(ApiError));
    expect(result.current.data).toBeUndefined();
  });

  it("Read again keeps the 503 body too, and a 429 cooldown is still an error to count down", async () => {
    const { Wrap, client } = wrapper();
    const { result } = renderHook(() => useTargetInventoryAgain("truenas"), { wrapper: Wrap });
    reply = () => new Response(JSON.stringify(halted), { status: 503 });
    await act(async () => void (await result.current.press()));
    expect((client.getQueryData(["targets", "truenas", "inventory"]) as typeof halted).detail).toBe(halted.detail);

    reply = () => new Response(JSON.stringify({ error: "FRESH_READ_COOLDOWN", retry_after: 20 }), { status: 429 });
    await act(async () => void (await result.current.press().catch(() => {})));
    await waitFor(() => expect(result.current.cooldown).toMatchObject({ reason: "cooldown", retry_after: 20 }));
  });
});
