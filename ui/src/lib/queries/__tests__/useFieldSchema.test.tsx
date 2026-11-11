// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { useFieldSchema, useFieldSchemaState, useSchemaSource } from "@/lib/queries/useFieldSchema";

const asked: string[] = [];
const fail = { targets: false };
const schema = [{ name: "active", role: "account" }, { name: "policy", role: "membership", label: "Access policy" }];

vi.mock("@/lib/api-client", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/lib/api-client")>()),
  request: async (path: string) => {
    asked.push(path);
    if (fail.targets) throw new Error("roster down");
    if (path === "/targets") {
      return { targets: [{ target: "doors", kind: "access", entitlement_schema: schema }, { target: "fresh" }, { target: "empty", entitlement_schema: [] }] };
    }
    return { targets: [{ target: "doors", kind: "access", entitlement_schema: schema }] };
  },
}));

function wrapper({ children }: { children: React.ReactNode }) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return <QueryClientProvider client={client}>{children}</QueryClientProvider>;
}

beforeEach(() => {
  asked.length = 0;
  fail.targets = false;
});

describe("a system's fields come from the response the page loaded", () => {
  it("reads the accepted schema from the roster", async () => {
    const { result } = renderHook(() => useFieldSchema("doors"), { wrapper });
    await waitFor(() => expect(result.current).toEqual(schema));
  });

  it("is undefined, not a guess, for a system whose manifest has not been accepted", async () => {
    const { result } = renderHook(() => ({ fresh: useFieldSchema("fresh"), empty: useFieldSchema("empty"), unknown: useFieldSchema("nope") }), { wrapper });
    await waitFor(() => expect(asked).toContain("/targets"));
    await new Promise((r) => setTimeout(r, 10));
    expect(result.current).toEqual({ fresh: undefined, empty: undefined, unknown: undefined });
  });

  it("asks a member's own list and never the roster their proxy refuses", async () => {
    const { result } = renderHook(() => useSchemaSource(false), { wrapper });
    await waitFor(() => expect(result.current("doors")).toEqual(schema));
    expect(asked).toEqual(["/me/targets"]);
  });

  it("asks the roster and not the member list for an operator", async () => {
    const { result } = renderHook(() => useSchemaSource(true), { wrapper });
    await waitFor(() => expect(result.current("doors")).toEqual(schema));
    expect(asked).toEqual(["/targets"]);
  });
});

// Three facts, one shape: loading, failed, and read-without-a-schema are never the same answer.
describe("the state of the read a schema comes from", () => {
  it("is loading first, then the schema, with no error", async () => {
    const { result } = renderHook(() => useFieldSchemaState("doors"), { wrapper });
    expect(result.current).toMatchObject({ schema: undefined, loading: true, error: null });
    await waitFor(() => expect(result.current.schema).toEqual(schema));
    expect(result.current).toMatchObject({ loading: false, error: null });
  });

  it("is read-without-a-schema (not loading, no error) for a system with no accepted manifest", async () => {
    const { result } = renderHook(() => useFieldSchemaState("fresh"), { wrapper });
    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current).toMatchObject({ schema: undefined, error: null });
  });

  it("carries the failure, not an empty schema, when the roster read failed", async () => {
    fail.targets = true;
    const { result } = renderHook(() => ({ one: useFieldSchemaState("doors"), many: useSchemaSource(true) }), { wrapper });
    await waitFor(() => expect(result.current.one.error).toBeTruthy());
    expect(result.current.one).toMatchObject({ schema: undefined, loading: false });
    expect(result.current.many.error).toBeTruthy();
    expect(typeof result.current.many.refetch).toBe("function");
  });
});
