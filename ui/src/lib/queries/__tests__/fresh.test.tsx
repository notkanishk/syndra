// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { readdirSync, readFileSync } from "node:fs";
import { resolve } from "node:path";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { ApiError, request } from "@/lib/api-client";
import { cooldownOf, useCountdown, useReadAgain, withFresh } from "@/lib/queries/fresh";
import { useMyStorage, useMyStorageAgain } from "@/lib/queries/useMyStorage";

/**
 * Feature freshness, the UI half: "Read again" is the card's own GET with
 * `fresh=true`, and nothing else ever sends it. A poll that sent it would ask
 * the NAS every 30 seconds for everyone with a page open.
 */

const offline = { target: "truenas", name: "TrueNAS", entitled: true, reachable: false, health: "offline", credential: { set: false } };
let urls: string[] = [];
let answer: (url: string) => Response;
const json = (body: unknown, init?: ResponseInit) => new Response(JSON.stringify(body), { status: 200, ...init });

function wrapper() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const Wrap = ({ children }: { children: ReactNode }) => <QueryClientProvider client={client}>{children}</QueryClientProvider>;
  return { client, Wrap };
}

beforeEach(() => {
  urls = [];
  answer = () => json({ targets: [offline] });
  vi.stubGlobal("fetch", vi.fn(async (input: RequestInfo | URL) => {
    const url = String(input);
    urls.push(url);
    return answer(url);
  }));
});
afterEach(() => {
  vi.useRealTimers();
  vi.unstubAllGlobals();
});

describe("the path", () => {
  it("asks for a fresh read with the one parameter the backend reads, after any query already there", () => {
    expect(withFresh("/me/targets")).toBe("/me/targets?fresh=true");
    expect(withFresh("/targets/truenas/activity?subject=u1&since=2026-11-01")).toBe("/targets/truenas/activity?subject=u1&since=2026-11-01&fresh=true");
  });

  it("reads a 429 cooldown, and only that, as a copy to count down", () => {
    expect(cooldownOf(new ApiError(429, { error: "FRESH_READ_COOLDOWN", retry_after: 12 }))).toEqual({ reason: "cooldown", retry_after: 12 });
    expect(cooldownOf(new ApiError(429, { error: "RATE_LIMITED", message: "slow down" }))).toBeUndefined();
    expect(cooldownOf(new ApiError(503, { error: "TARGET_UNREACHABLE" }))).toBeUndefined();
    expect(cooldownOf(new Error("x"))).toBeUndefined();
  });
});

describe("what sends fresh", () => {
  it("never the page's own load or its 30 second poll", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    const { Wrap } = wrapper();
    renderHook(() => useMyStorage(), { wrapper: Wrap });
    await vi.advanceTimersByTimeAsync(0);
    await waitFor(() => expect(urls.length).toBe(1));
    await vi.advanceTimersByTimeAsync(31_000);
    await vi.advanceTimersByTimeAsync(31_000);
    await vi.advanceTimersByTimeAsync(31_000);
    expect(urls.length).toBeGreaterThanOrEqual(3);
    for (const url of urls) {
      expect(url).toBe("/api/proxy/me/targets");
      expect(url).not.toContain("fresh");
    }
  });

  it("one press sends fresh exactly once, and its answer replaces the card's cached one", async () => {
    const { client, Wrap } = wrapper();
    const mine = renderHook(() => useMyStorage(false), { wrapper: Wrap });
    const again = renderHook(() => useMyStorageAgain(), { wrapper: Wrap });
    answer = () => json({ targets: [{ ...offline, name: "Read again" }] });
    await act(async () => {
      await again.result.current.press();
    });
    expect(urls).toEqual(["/api/proxy/me/targets?fresh=true"]);
    expect(client.getQueryData(["me", "targets"])).toEqual({ targets: [{ ...offline, name: "Read again" }] });
    expect(mine.result.current.fetchStatus).toBe("idle");
  });

  it("keeps what the backend refused until the next press, and says it is pending meanwhile", async () => {
    const { Wrap } = wrapper();
    const { result } = renderHook(() => useReadAgain(["inventory"], "/targets/truenas/inventory"), { wrapper: Wrap });
    answer = () => json({ error: "FRESH_READ_COOLDOWN", target: "truenas", halted: true, retry_after: 9 }, { status: 429, headers: { "Retry-After": "9" } });
    await act(async () => {
      await expect(result.current.press()).rejects.toBeInstanceOf(ApiError);
    });
    expect(cooldownOf(result.current.error)).toEqual({ reason: "cooldown", retry_after: 9 });
    expect(result.current.pending).toBe(false);
    answer = () => json({ target: "truenas" });
    await act(async () => {
      await result.current.press();
    });
    expect(result.current.error).toBeNull();
  });
});

describe("who may ask the system itself", () => {
  // The structural half of "no poll sends fresh": the parameter is written in
  // one file, and only `useReadAgain` calls what writes it.
  it("is one hook: nothing else writes the parameter or calls withFresh", () => {
    const src = resolve(__dirname, "../../..");
    const writers: string[] = [];
    const walk = (dir: string) => {
      for (const entry of readdirSync(dir, { withFileTypes: true })) {
        const path = resolve(dir, entry.name);
        if (entry.isDirectory()) {
          if (entry.name !== "__tests__") walk(path);
        } else if (/\.tsx?$/.test(entry.name) && !path.endsWith("queries/fresh.ts") && /fresh=true|withFresh\(/.test(readFileSync(path, "utf8").replace(/\/\*[\s\S]*?\*\/|^\s*\/\/.*$/gm, ""))) {
          writers.push(path.replace(`${src}/`, ""));
        }
      }
    };
    walk(src);
    expect(writers).toEqual([]);
  });
});

describe("the countdown", () => {
  it("counts whole seconds down to 0 and stops there", () => {
    vi.useFakeTimers();
    const { result } = renderHook(() => useCountdown(3));
    expect(result.current).toBe(3);
    act(() => void vi.advanceTimersByTime(1000));
    expect(result.current).toBe(2);
    act(() => void vi.advanceTimersByTime(2000));
    expect(result.current).toBe(0);
    act(() => void vi.advanceTimersByTime(5000));
    expect(result.current).toBe(0);
  });

  it("is 0 when there is nothing to wait for, and starts over on a new wait", () => {
    vi.useFakeTimers();
    const { result, rerender } = renderHook(({ s }: { s: number | undefined }) => useCountdown(s), { initialProps: { s: undefined as number | undefined } });
    expect(result.current).toBe(0);
    rerender({ s: 20 });
    expect(result.current).toBe(20);
  });
});

describe("the app's retry", () => {
  it("retries a blip once, and never a read the cooldown refused: it would only be refused again", async () => {
    const { makeQueryClient } = await import("@/lib/query-client");
    const retry = makeQueryClient().getDefaultOptions().queries?.retry as (count: number, error: Error) => boolean;
    expect(retry(0, new Error("blip"))).toBe(true);
    expect(retry(1, new Error("blip"))).toBe(false);
    expect(retry(0, new ApiError(429, { error: "FRESH_READ_COOLDOWN", retry_after: 5 }))).toBe(false);
  });
});

describe("a poll in flight when Read again is pressed", () => {
  it("cannot land after the press and put the copy back", async () => {
    const { client, Wrap } = wrapper();
    const again = renderHook(() => useReadAgain<{ v: string }>(["card"], "/card"), { wrapper: Wrap });
    let landPoll!: (r: Response) => void;
    vi.stubGlobal("fetch", vi.fn(async (input: RequestInfo | URL) => {
      urls.push(String(input));
      return String(input).includes("fresh=true") ? json({ v: "fresh" }) : new Promise<Response>((r) => (landPoll = r));
    }));
    void client.fetchQuery({ queryKey: ["card"], queryFn: () => request<{ v: string }>("/card") }).catch(() => {});
    await waitFor(() => expect(urls).toEqual(["/api/proxy/card"]));
    await act(async () => {
      await again.result.current.press();
    });
    landPoll(json({ v: "copy" }));
    await act(async () => void (await new Promise((r) => setTimeout(r, 10))));
    expect(client.getQueryData(["card"])).toEqual({ v: "fresh" });
  });
});

describe("the countdown counts from when the answer arrived", () => {
  it("starts over for a second answer with the same wait, and does not start over on coming back", () => {
    vi.useFakeTimers();
    vi.setSystemTime(100_000);
    const { result, rerender, unmount } = renderHook(({ since }: { since: number }) => useCountdown(10, since), { initialProps: { since: 100_000 } });
    act(() => void vi.advanceTimersByTime(6000));
    expect(result.current).toBe(4);
    rerender({ since: 106_000 });
    expect(result.current).toBe(10);
    act(() => void vi.advanceTimersByTime(3000));
    unmount();
    // Back on the page: the same answer, three seconds old, is not drawn as ten.
    expect(renderHook(() => useCountdown(10, 106_000)).result.current).toBe(7);
  });

  it("is held by useReadAgain from the press the refusal came back on", async () => {
    vi.useFakeTimers({ toFake: ["Date"] });
    vi.setSystemTime(50_000);
    const { Wrap } = wrapper();
    const { result } = renderHook(() => useReadAgain(["inv"], "/inv"), { wrapper: Wrap });
    answer = () => json({ error: "FRESH_READ_COOLDOWN", retry_after: 9 }, { status: 429 });
    await act(async () => {
      await expect(result.current.press()).rejects.toBeInstanceOf(ApiError);
    });
    expect(result.current.errorAt).toBe(50_000);
  });
});
