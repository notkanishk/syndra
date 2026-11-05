import type { QueryClient, QueryKey } from "@tanstack/react-query";

/**
 * The ONE place a write learns which reads it made stale.
 *
 * Every mutation hook calls `invalidateAfter(qc, kind, scope)` and nothing
 * else; no hook names a query key to invalidate. Before this each hook listed
 * its own, and they disagreed: approving a request missed People and Pending
 * changes, draining missed People, and two hooks for the same kind of change
 * covered different sets. `invalidation.test.ts` fails a mutation that does
 * not call this and a raw `invalidateQueries` outside this file.
 *
 * The rail's counts are `["governance", "indicators"]`, so any write that can
 * move a count includes `["governance"]` (see RAIL_KINDS in the test).
 */
export type WriteKind =
  /** Who holds what changed: a grant, a hold, a revoke, a decision, a publish, a rule, a mapping. */
  | "access"
  /** A bundle's own definition: create, roles, includes, welcome, discard. Reaches nobody. */
  | "bundle"
  /** A bundle's name: the bundle reads and the name resolver's copy. */
  | "bundleRename"
  /** A request was filed or withdrawn. */
  | "request"
  /** A drift finding was triaged. */
  | "drift"
  /** Roles: create, or set which wait. */
  | "role"
  /** A service-account mark. */
  | "serviceAccount"
  /** An expiry was acknowledged or the acknowledgement cleared. */
  | "expiryNote"
  /** A claim shape, its profile, or a debug session. */
  | "claimShape"
  /** The signing key rotated. */
  | "signingKey"
  /** A target's own state: settings, lifecycle, project, findings, bindings. */
  | "target"
  /** A member's own storage credential. */
  | "storageCredential"
  /** Term dates. */
  | "terms"
  /** The welcome-claims switch. */
  | "welcomeClaims";

export interface WriteScope {
  target?: string;
  appId?: string;
  /** Keys only the calling hook knows (its own list or a local constant). */
  extra?: readonly QueryKey[];
}

const GOVERNANCE: QueryKey = ["governance"];

const ACCESS: readonly QueryKey[] = [
  ["users"],
  ["bundles"],
  ["roles"],
  ["review"],
  ["me", "targets"],
  ["requests"],
  ["propagations"],
  ["targets"],
  // Every access change is in the log, and Home's "New people without a bundle"
  // reads operations.
  ["audit"],
  ["operations"],
  GOVERNANCE,
];

/** Pure: the keys a write of this kind makes stale. Exported for the test. */
export function keysFor(kind: WriteKind, scope: WriteScope = {}): QueryKey[] {
  const { target, appId } = scope;
  const keys: QueryKey[] = (() => {
    switch (kind) {
      case "access":
        return [...ACCESS];
      case "bundle":
        return [["bundles"]];
      case "bundleRename":
        return [["bundles"], ["name-bundles"]];
      case "request":
        return [["requests"], GOVERNANCE];
      case "drift":
        // A revoke queues a removal, and an adopt can queue what a rule gives.
        return [["drift"], ["users"], ["propagations"], GOVERNANCE];
      case "role":
        return [["roles"], ["roles-that-wait"], ["projects"], ["name-catalog"]];
      case "serviceAccount":
        return [["service-accounts"], ["users"], GOVERNANCE];
      case "expiryNote":
        return [["review", "expiring-grants"], GOVERNANCE];
      case "claimShape":
        return [appId ? ["claim-shape", "app", appId] : ["claim-shape"], ["claim-debug"], ["applications"]];
      case "signingKey":
        return [["zitadel", "rotation"], ["claims"], ["audit"]];
      case "target":
        return [target ? ["targets", target] : ["targets"], ["audit"], GOVERNANCE];
      case "storageCredential":
        return [["me", "targets"]];
      case "terms":
        return [["terms"], ["insights"]];
      case "welcomeClaims":
        return [];
    }
  })();
  return [...keys, ...(scope.extra ?? [])];
}

export function invalidateAfter(qc: QueryClient, kind: WriteKind, scope: WriteScope = {}) {
  for (const queryKey of keysFor(kind, scope)) void qc.invalidateQueries({ queryKey });
}
