// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { useStandingBindingConflicts } from "@/lib/queries/useTargets";

/**
 * 7t-6: a page that lists stopped changes reads each system's standing findings once, however many
 * of its rows are binding conflicts there, and says a conflict stands only for that person on that
 * system and only while its finding is open (the read lists open findings alone).
 */
const fetched: string[] = [];

beforeEach(() => {
  fetched.length = 0;
  vi.stubGlobal(
    "fetch",
    vi.fn(async (url: string) => {
      fetched.push(String(url));
      const findings = String(url).includes("/targets/truenas/")
        ? [
            { id: "f1", target: "truenas", subject_id: "u1", outcome: "binding_conflict" },
            { id: "f2", target: "truenas", subject_id: "u2", outcome: "conflict" },
          ]
        : [];
      return new Response(JSON.stringify({ findings }), { status: 200, headers: { "Content-Type": "application/json" } });
    }),
  );
});
afterEach(() => vi.unstubAllGlobals());

describe("whose binding conflict still stands", () => {
  it("reads each system once and answers per person and system", async () => {
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const Wrap = ({ children }: { children: ReactNode }) => <QueryClientProvider client={client}>{children}</QueryClientProvider>;
    const { result } = renderHook(() => useStandingBindingConflicts(["truenas", "truenas", "unifi", "truenas", ""]), {
      wrapper: Wrap,
    });
    expect(result.current.loading("truenas")).toBe(true);
    await waitFor(() => expect(result.current.loading("truenas")).toBe(false));
    await waitFor(() => expect(result.current.loading("unifi")).toBe(false));
    expect(fetched.filter((u) => u.includes("merge-findings"))).toHaveLength(2);
    expect(result.current.stands("truenas", "u1")).toBe(true);
    expect(result.current.stands("truenas", "u2")).toBe(false); // another kind of finding
    expect(result.current.stands("unifi", "u1")).toBe(false); // another system
    expect(result.current.stands("zitadel", "u1")).toBe(false); // a system not asked about
    expect(result.current.loading("zitadel")).toBe(false);
  });
});
