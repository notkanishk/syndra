// @vitest-environment jsdom
import { readdirSync, readFileSync } from "node:fs";
import { join } from "node:path";

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderHook, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

vi.mock("@/lib/api-client", () => ({ request: vi.fn().mockResolvedValue({}) }));

import { invalidateAfter, keysFor, type WriteKind } from "@/lib/queries/invalidation";
import { useDecideRequest } from "@/lib/queries/useRequests";
import { useDrainPropagations } from "@/lib/queries/usePropagation";

const QUERIES_DIR = join(import.meta.dirname, "..");

/** Every kind, so adding one without classifying it here fails the build. */
const KINDS: Record<WriteKind, { rail: boolean }> = {
  access: { rail: true },
  bundle: { rail: false },
  bundleRename: { rail: false },
  request: { rail: true },
  drift: { rail: true },
  role: { rail: false },
  serviceAccount: { rail: true },
  expiryNote: { rail: true },
  claimShape: { rail: false },
  signingKey: { rail: false },
  target: { rail: true },
  storageCredential: { rail: false },
  terms: { rail: false },
  welcomeClaims: { rail: false },
  retriesSeen: { rail: true },
  // A look-again moves what was observed, never the rail.
  observed: { rail: false },
  upstream: { rail: true },
  // Every read asks again, the rail's included.
  reconnected: { rail: false },
};

const has = (keys: readonly unknown[][], key: readonly unknown[]) =>
  keys.some((k) => JSON.stringify(k) === JSON.stringify(key));

describe("the write-to-keys map", () => {
  it("every kind that can move a rail count invalidates the governance summary, which holds the rail", () => {
    for (const [kind, { rail }] of Object.entries(KINDS) as [WriteKind, { rail: boolean }][]) {
      const keys = keysFor(kind, { target: "t", appId: "a" }) as unknown[][];
      expect(has(keys, ["governance"]), `${kind} and the rail`).toBe(rail);
    }
  });

  it("a look-again reaches that person's access view, nobody else's", () => {
    const keys = keysFor("observed", { user: "u1" }) as unknown[][];
    expect(keys).toEqual([["users", "access", "u1"]]);
  });

  it("a Zitadel-screen write reaches the readers of Zitadel's records and the rail", () => {
    const keys = keysFor("upstream") as unknown[][];
    for (const k of [["upstream"], ["drift"], ["name-catalog"], ["users"], ["roles"], ["governance"], ["propagations"]]) {
      expect(has(keys, k), JSON.stringify(k)).toBe(true);
    }
  });

  it("reconnecting drops every query, the rail's too", () => {
    const qc = new QueryClient();
    qc.setQueryData(["governance", "indicators"], 1);
    qc.setQueryData(["anything", "else"], 2);
    invalidateAfter(qc, "reconnected");
    expect(qc.getQueryState(["governance", "indicators"])?.isInvalidated).toBe(true);
    expect(qc.getQueryState(["anything", "else"])?.isInvalidated).toBe(true);
  });

  it("an access change reaches people, bundles, roles, pending, drift's neighbours and the rail", () => {
    const keys = keysFor("access") as unknown[][];
    for (const k of [["users"], ["bundles"], ["roles"], ["propagations"], ["governance"], ["review"], ["targets"], ["requests"]]) {
      expect(has(keys, k), JSON.stringify(k)).toBe(true);
    }
  });

  // Audit S7: a drift revoke queues a removal and an adopt can queue what a
  // rule gives; the signing key and a target's own state now write audit rows.
  it("drift reaches Pending changes, and the key and target writes reach the audit log", () => {
    expect(has(keysFor("drift") as unknown[][], ["propagations"])).toBe(true);
    expect(has(keysFor("signingKey") as unknown[][], ["audit"])).toBe(true);
    expect(has(keysFor("target", { target: "t" }) as unknown[][], ["audit"])).toBe(true);
  });

  it("scopes and extras are added, never replace the kind's keys", () => {
    const keys = keysFor("claimShape", { appId: "a1", extra: [["mine"]] }) as unknown[][];
    expect(has(keys, ["claim-shape", "app", "a1"])).toBe(true);
    expect(has(keys, ["applications"])).toBe(true);
    expect(has(keys, ["mine"])).toBe(true);
  });

  it("invalidateAfter drops exactly the mapped keys", () => {
    const qc = new QueryClient();
    const spy = vi.spyOn(qc, "invalidateQueries");
    invalidateAfter(qc, "terms", { extra: [["terms"]] });
    expect(spy.mock.calls.map((c) => c[0]?.queryKey)).toEqual([["terms"], ["insights"], ["terms"]]);
  });
});

function harness() {
  const client = new QueryClient({ defaultOptions: { mutations: { retry: false } } });
  const roots: string[] = [];
  const original = client.invalidateQueries.bind(client);
  client.invalidateQueries = ((f?: { queryKey?: unknown }) => {
    if (Array.isArray(f?.queryKey)) roots.push(String(f.queryKey[0]));
    return original(f as Parameters<typeof original>[0]);
  }) as typeof client.invalidateQueries;
  const wrapper = ({ children }: { children: React.ReactNode }) => (
    <QueryClientProvider client={client}>{children}</QueryClientProvider>
  );
  return { wrapper, roots };
}

// The two the audit named: approving a grant missed People and Pending changes,
// and a drain missed People.
describe("the hooks the audit found short", () => {
  it("approving a request reaches people and pending changes", async () => {
    const { wrapper, roots } = harness();
    const { result } = renderHook(() => useDecideRequest(), { wrapper });
    await result.current.mutateAsync({ id: "r1", decision: "approved" } as never);
    await waitFor(() => expect(roots).toContain("propagations"));
    expect(roots).toEqual(expect.arrayContaining(["users", "governance", "requests"]));
  });

  it("a drain reaches people", async () => {
    const { wrapper, roots } = harness();
    const { result } = renderHook(() => useDrainPropagations(), { wrapper });
    await result.current.mutateAsync();
    await waitFor(() => expect(roots).toContain("users"));
    expect(roots).toContain("governance");
  });
});

// ---- source guards ------------------------------------------------------

/** Dry runs and lookups: they write nothing, so there is nothing to invalidate. */
const READ_ONLY = new Set([
  "useRehearsePublish",
  "useRehearseMoveHolders",
  "useRehearseBulk",
  "useRehearseEntitlements",
  "useDraftFrom",
  "useValidateMappingRule",
  "useLookUpSignIns",
  // Writes its answer straight into the cache with setQueryData.
  "useSaveWelcomeClaims",
]);

const SRC = join(QUERIES_DIR, "..", "..");

/** Every non-test source file under src/. */
function srcFiles(dir: string): string[] {
  return readdirSync(dir, { withFileTypes: true }).flatMap((e) => {
    const p = join(dir, e.name);
    if (e.isDirectory()) return e.name === "__tests__" ? [] : srcFiles(p);
    return /\.tsx?$/.test(e.name) && !/\.test\.tsx?$/.test(e.name) ? [p] : [];
  });
}

function hookFiles(): string[] {
  return readdirSync(QUERIES_DIR)
    .filter((n) => /^use.*\.tsx?$/.test(n))
    .map((n) => join(QUERIES_DIR, n));
}

/** A top-level function (or `export const use… =`) per block. */
function blocks(src: string): string[] {
  return src.split(/\n(?=(?:export )?(?:async )?function |export const )/);
}

export function mutationsMissingTheMap(files: Record<string, string>): string[] {
  const missing: string[] = [];
  for (const [file, src] of Object.entries(files)) {
    for (const block of blocks(src)) {
      if (!/useMutation\(/.test(block)) continue;
      const name = block.match(/(?:function|const) (\w+)/)?.[1] ?? "?";
      if (READ_ONLY.has(name)) continue;
      if (!/\binvalidateAfter\(/.test(block)) missing.push(`${file}: ${name}`);
    }
  }
  return missing;
}

describe("every mutation hook goes through the map", () => {
  const files = Object.fromEntries(hookFiles().map((f) => [f, readFileSync(f, "utf8")]));

  it("calls invalidateAfter unless it is a dry run", () => {
    expect(mutationsMissingTheMap(files)).toEqual([]);
  });

  it("the guard fails a mutation that does not (mutation check)", () => {
    const broken = {
      "useX.ts": `export function useX() {\n  return useMutation({ mutationFn: () => 1, onSuccess: () => qc.invalidateQueries({ queryKey: ["users"] }) });\n}`,
    };
    expect(mutationsMissingTheMap(broken)).toEqual(["useX.ts: useX"]);
  });

  it("nothing in src names a key to invalidate: only invalidation.ts does", () => {
    const offenders = srcFiles(SRC)
      .filter((f) => !f.endsWith("lib/queries/invalidation.ts"))
      .filter((f) => /\.invalidateQueries\(/.test(readFileSync(f, "utf8")))
      .map((f) => f.slice(SRC.length));
    expect(offenders).toEqual([]);
  });
});
