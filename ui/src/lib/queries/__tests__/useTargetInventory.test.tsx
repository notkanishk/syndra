// @vitest-environment jsdom
import { QueryClient, QueryClientProvider, type UseMutationResult, type UseQueryResult } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { ApiError } from "@/lib/api-client";
import { readOf, type Answered } from "@/lib/queries/read";
import { useDormantAccounts } from "@/lib/queries/useDormant";
import { useRehearseEntitlements } from "@/lib/queries/useEntitlements";
import { useMyStorage, usePersonTargets } from "@/lib/queries/useMyStorage";
import { useTargetActivity } from "@/lib/queries/useTargetActivity";
import {
  useMappableValues,
  useTargetHealth,
  useTargetHomes,
  useTargetInventory,
  useTargetInventoryAgain,
  useTargetReach,
  useTargetSettings,
} from "@/lib/queries/useTargets";
import { useTargetSystemHealth } from "@/lib/queries/useTargetSystemHealth";

/**
 * Design §13.6's guard: every add-on read reaches a component as a `Read`, and a
 * read with nothing to give (503 TARGET_NOT_READ, or a composite route's
 * `read: none`) is `none` with its why, never data, never an error. A hook that
 * goes back to a raw `request` fails here.
 */
let reply: () => Response;
const notRead = () =>
  new Response(
    JSON.stringify({
      error: "TARGET_NOT_READ",
      message: "Syndra can't reach TrueNAS's add-on. Check that its container is running. Syndra keeps trying.",
      details: { reason: "addon_unreachable", cause: "no_answer" },
    }),
    { status: 503, headers: { "Retry-After": "30" } },
  );
const noneRead = { state: "none", reason: "addon_unreachable", cause: "no_answer", retry_after: 30 };

function wrapper() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const Wrap = ({ children }: { children: ReactNode }) => <QueryClientProvider client={client}>{children}</QueryClientProvider>;
  return { client, Wrap };
}

beforeEach(() => {
  vi.stubGlobal("fetch", vi.fn(async () => reply()));
});
afterEach(() => vi.unstubAllGlobals());

const expectNone = (read: Answered<unknown> | undefined) => {
  expect(read?.state).toBe("none");
  if (read?.state !== "none") return;
  expect(read.why.reason).toBe("addon_unreachable");
  expect(read.why.cause).toBe("no_answer");
  expect(read.why.retryAfter).toBe(30);
  expect("data" in read).toBe(false);
};

describe("every add-on read hook answers a Read", () => {
  const queries: Array<[string, () => UseQueryResult<Answered<unknown>>]> = [
    ["inventory", () => useTargetInventory("truenas")],
    ["reach", () => useTargetReach("truenas")],
    ["homes", () => useTargetHomes("truenas")],
    ["settings", () => useTargetSettings("truenas")],
    ["mappable values", () => useMappableValues("truenas", "groups")],
    ["system health", () => useTargetSystemHealth("truenas")],
    ["activity", () => useTargetActivity("truenas", "u-cena")],
    ["dormant accounts", () => useDormantAccounts("truenas")],
  ];
  it.each(queries)("%s: a 503 TARGET_NOT_READ is none, with its why and no data", async (_name, hook) => {
    reply = notRead;
    const { Wrap } = wrapper();
    const { result } = renderHook(hook, { wrapper: Wrap });
    await waitFor(() => expect(result.current.data).toBeDefined());
    expect(result.current.error).toBeNull();
    expectNone(result.current.data);
  });

  it("a plan rehearsed against a system with nothing to read is none, never an empty plan", async () => {
    reply = notRead;
    const { Wrap } = wrapper();
    const { result } = renderHook(() => useRehearseEntitlements("truenas"), { wrapper: Wrap });
    let plan: Answered<unknown> | undefined;
    await act(async () => {
      plan = await (result.current as UseMutationResult<Answered<unknown>, unknown, { subjectIds: string[] }>).mutateAsync({ subjectIds: ["u-cena"] });
    });
    expectNone(plan);
  });

  it("the health route keeps Syndra's own records beside an add-on part that is none", async () => {
    reply = () =>
      new Response(JSON.stringify({ target: "truenas", read: noneRead, health: { state: "addon_unreachable", cause: "no_answer", recent: [] }, status: "addon_unreachable" }), { status: 200 });
    const { Wrap } = wrapper();
    const { result } = renderHook(() => useTargetHealth("truenas"), { wrapper: Wrap });
    await waitFor(() => expect(result.current.data).toBeDefined());
    expect(result.current.data?.status).toBe("addon_unreachable");
    expectNone(result.current.data?.addon);
  });

  it.each([
    ["a member's own systems", () => useMyStorage()],
    ["a person's systems", () => usePersonTargets("u-cena")],
  ])("%s: each system's add-on part is a Read, and none carries no storage", async (_name, hook) => {
    reply = () =>
      new Response(
        JSON.stringify({ targets: [{ target: "truenas", name: "TrueNAS", entitled: true, credential: { set: false }, health: "addon_unreachable", read: noneRead }] }),
        { status: 200 },
      );
    const { Wrap } = wrapper();
    const { result } = renderHook(hook, { wrapper: Wrap });
    await waitFor(() => expect(result.current.data).toBeDefined());
    const [view] = result.current.data as Array<{ read?: Answered<unknown>; storage?: unknown }>;
    expectNone(view.read);
    expect(view.storage).toBeUndefined();
  });

  it("any other failure is Syndra's own and stays an error", async () => {
    reply = () => new Response(JSON.stringify({ error: "INVENTORY_ERROR", message: "Syndra's server had a problem." }), { status: 500 });
    const { Wrap } = wrapper();
    const { result } = renderHook(() => useTargetInventory("truenas"), { wrapper: Wrap });
    await waitFor(() => expect(result.current.error).toBeInstanceOf(ApiError));
    expect(result.current.data).toBeUndefined();
  });

  it("Read again's answer replaces the card's read, a none included", async () => {
    reply = () => new Response(JSON.stringify({ target: "truenas", bound: 2, read: { state: "now", taken_at: "2026-11-21T10:00:00Z" } }), { status: 200 });
    const { Wrap } = wrapper();
    const { result } = renderHook(() => ({ card: useTargetInventory("truenas"), again: useTargetInventoryAgain("truenas") }), { wrapper: Wrap });
    await waitFor(() => expect(result.current.card.data?.state).toBe("now"));
    reply = notRead;
    await act(async () => {
      await result.current.again.press();
    });
    expectNone(result.current.card.data);
    expect((fetch as unknown as ReturnType<typeof vi.fn>).mock.calls.at(-1)?.[0]).toContain("fresh=true");
  });
});

describe("readOf: one date", () => {
  it.each([
    ["no taken_at", { state: "now" as const }],
    ["Go's zero time", { state: "now" as const, taken_at: "0001-01-01T00:00:00Z" }],
    ["an unparseable time", { state: "kept" as const, taken_at: "yesterday" }],
  ])("a reading with %s is none, never a date", (_name, wire) => {
    expect(readOf(wire, { bound: 0 }).state).toBe("none");
  });

  it("a kept copy keeps its reason and wait", () => {
    const read = readOf({ state: "kept", taken_at: "2026-11-21T10:00:00Z", reason: "cooldown", retry_after: 12 }, { bound: 3 });
    expect(read).toMatchObject({ state: "kept", data: { bound: 3 }, why: { reason: "cooldown", retryAfter: 12 } });
  });
});
