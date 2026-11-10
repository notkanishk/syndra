// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import Link from "next/link";

import { INTENT_MS, IntentPrefetchHost } from "@/components/shell/IntentPrefetchHost";

const prefetch = vi.fn();
const user = vi.fn();
const shape = vi.fn();
vi.mock("next/navigation", () => ({ useRouter: () => ({ prefetch }) }));
vi.mock("@/lib/queries/useUsers", () => ({ prefetchUserAccess: (...a: unknown[]) => user(...a) }));
vi.mock("@/lib/queries/useClaimShape", () => ({ prefetchAppClaimShape: (...a: unknown[]) => shape(...a) }));

function setConnection(value: object | undefined) {
  Object.defineProperty(navigator, "connection", { value, configurable: true });
}

beforeEach(() => {
  vi.useFakeTimers();
  render(
    <QueryClientProvider client={new QueryClient()}>
      <IntentPrefetchHost />
      <Link href="/users/u1">Cody</Link>
      <Link href="/applications/a1">Wiki</Link>
      <a href="/roles">Roles</a>
      <a href="https://example.com/users/x">Away</a>
      <a href="//evil.example/users/x">Protocol relative</a>
    </QueryClientProvider>,
  );
});
afterEach(() => {
  cleanup();
  vi.clearAllMocks();
  vi.useRealTimers();
  setConnection(undefined);
});

const rest = () => act(() => void vi.advanceTimersByTime(INTENT_MS));

describe("intent prefetch (P9)", () => {
  it("warms the route and the person's read once the pointer rests", () => {
    fireEvent.pointerOver(screen.getByText("Cody"));
    expect(prefetch).not.toHaveBeenCalled(); // not yet: it might only be passing over
    rest();
    expect(prefetch).toHaveBeenCalledWith("/users/u1");
    expect(user).toHaveBeenCalledTimes(1);
    expect(user.mock.calls[0][1]).toBe("u1");
  });

  it("fires once per address, however often it is hovered or focused", () => {
    const link = screen.getByText("Cody");
    for (let i = 0; i < 3; i += 1) {
      fireEvent.pointerOver(link);
      rest();
      fireEvent.focusIn(link);
      rest();
    }
    expect(prefetch).toHaveBeenCalledTimes(1);
    expect(user).toHaveBeenCalledTimes(1);
  });

  it("focus is intent too, and an app warms its shape", () => {
    fireEvent.focusIn(screen.getByText("Wiki"));
    rest();
    expect(shape).toHaveBeenCalledTimes(1);
    expect(shape.mock.calls[0][1]).toBe("a1");
    expect(user).not.toHaveBeenCalled();
  });

  it("a pointer that only passes over warms nothing", () => {
    const link = screen.getByText("Cody");
    fireEvent.pointerOver(link);
    act(() => void vi.advanceTimersByTime(INTENT_MS - 30));
    fireEvent.pointerOut(link);
    rest();
    expect(prefetch).not.toHaveBeenCalled();
  });

  it("a list link warms its route only; other origins warm nothing", () => {
    fireEvent.pointerOver(screen.getByText("Roles"));
    rest();
    expect(prefetch).toHaveBeenCalledWith("/roles");
    expect(user).not.toHaveBeenCalled();
    fireEvent.pointerOver(screen.getByText("Away"));
    fireEvent.pointerOver(screen.getByText("Protocol relative"));
    rest();
    expect(prefetch).toHaveBeenCalledTimes(1);
  });

  it("does nothing with Data Saver on", () => {
    setConnection({ saveData: true });
    fireEvent.pointerOver(screen.getByText("Cody"));
    rest();
    expect(prefetch).not.toHaveBeenCalled();
    expect(user).not.toHaveBeenCalled();
  });

  it("does nothing on a slow connection", () => {
    setConnection({ effectiveType: "2g" });
    fireEvent.focusIn(screen.getByText("Cody"));
    rest();
    expect(prefetch).not.toHaveBeenCalled();
  });
});
