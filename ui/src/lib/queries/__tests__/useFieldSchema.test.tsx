// @vitest-environment jsdom
import { renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { TRUENAS_SCHEMA } from "@/lib/__fixtures__/truenas-schema";
import { useFieldSchemaState, useSchemaSource, useSystemNames } from "@/lib/queries/useFieldSchema";

/**
 * The one place a page learns how the read its fields come from went, and
 * what each system is called. Review finding 3: the roster refetches every
 * 30s, and a failed refresh keeps the rows it had. Counting that as a failed
 * read blanked whatever a page drew from the rows (MergeFindings unmounted
 * the decision form, with the reason the operator had typed).
 */
const roster = vi.hoisted(() => ({
  data: undefined as unknown[] | undefined,
  isLoading: false,
  error: null as unknown,
  refetch: vi.fn(),
}));
vi.mock("@/lib/queries/useTargets", () => ({ useTargets: () => roster }));
vi.mock("@/lib/queries/useMyStorage", () => ({ useMyStorage: () => roster }));

const ROW = { target: "truenas", name: "Big NAS", entitlement_schema: TRUENAS_SCHEMA };

beforeEach(() => {
  roster.data = undefined;
  roster.isLoading = false;
  roster.error = null;
});

describe("a failed read is failed only when there are no rows", () => {
  it("keeps the schema and reports no error when a background refresh fails", () => {
    roster.data = [ROW];
    roster.error = new Error("refresh failed");
    const { result } = renderHook(() => useFieldSchemaState("truenas"));
    expect(result.current.schema).toBe(TRUENAS_SCHEMA);
    expect(result.current.error).toBeNull();
  });

  it("reports the error when the first read failed", () => {
    roster.error = new Error("down");
    const { result } = renderHook(() => useFieldSchemaState("truenas"));
    expect(result.current.schema).toBeUndefined();
    expect(result.current.error).toBe(roster.error);
  });

  it("does the same for a many-systems source", () => {
    roster.data = [ROW];
    roster.error = new Error("refresh failed");
    const { result } = renderHook(() => useSchemaSource(true));
    expect(result.current.error).toBeNull();
    expect(result.current("truenas")).toBe(TRUENAS_SCHEMA);
  });
});

describe("what a system is called", () => {
  it("is the roster's name, one source, and the id title-cased before the rows land", () => {
    roster.data = [ROW];
    expect(renderHook(() => useSystemNames()).result.current("truenas")).toBe("Big NAS");
    expect(renderHook(() => useSystemNames()).result.current("door")).toBe("Door");
    roster.data = undefined;
    expect(renderHook(() => useSystemNames()).result.current("truenas")).toBe("Truenas");
  });
});

// Review finding 6: a member page with no hold to word does not ask for
// /me/targets on the hold card's behalf (that read calls each add-on live).
describe("a source that is not needed asks nothing", () => {
  it("passes enabled through to the one read it would make", async () => {
    const storage = await import("@/lib/queries/useMyStorage");
    const targets = await import("@/lib/queries/useTargets");
    const mine = vi.spyOn(storage, "useMyStorage");
    const ops = vi.spyOn(targets, "useTargets");
    renderHook(() => useSchemaSource(false, false));
    expect(mine).toHaveBeenLastCalledWith(false);
    expect(ops).toHaveBeenLastCalledWith(false);
    renderHook(() => useSchemaSource(false, true));
    expect(mine).toHaveBeenLastCalledWith(true);
    expect(ops).toHaveBeenLastCalledWith(false);
  });
});
