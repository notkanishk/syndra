"use client";

import { useState } from "react";

import { UserName } from "@/components/names";
import { Badge } from "@/components/ui/Badge";
import { Eyebrow } from "@/components/ui/Eyebrow";
import { useBundleImpact } from "@/lib/queries/useBundles";

interface BundleImpactAccordionProps {
  bundleId: string;
  /** Maximum number of users to render before collapsing into "+N more". */
  sampleLimit?: number;
}

/**
 * Self-contained collapsible card showing the user impact of a bundle.
 * Behavior contract — must remain stable for the bundles page test suite:
 *   1. The accordion is collapsed by default.
 *   2. The `useBundleImpact` query MUST NOT fire until the operator opens it
 *      (the bundle's user list can be expensive on large rosters; deferring
 *      avoids paying the cost on every page render).
 *   3. When opened, the affected role count is shown alongside the first
 *      `sampleLimit` users (default 10) plus "+N more" overflow.
 */
export default function BundleImpactAccordion({
  bundleId,
  sampleLimit = 10,
}: BundleImpactAccordionProps) {
  const [open, setOpen] = useState(false);
  // Conditional `bundleId` argument is the documented way to defer the query
  // (the hook checks for null/undefined and stays disabled).
  const impactQuery = useBundleImpact(open ? bundleId : null);

  const users = impactQuery.data?.users ?? [];
  const visible = users.slice(0, sampleLimit);
  const overflow = Math.max(0, users.length - visible.length);

  return (
    <div className="rounded-card border border-outline-variant">
      <button
        type="button"
        onClick={() => setOpen((prev) => !prev)}
        aria-expanded={open}
        className="flex w-full items-center justify-between gap-3 px-3 py-2 text-left transition-colors hover:bg-surface-container focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary-container rounded-card"
      >
        <div className="flex items-center gap-2">
          <Eyebrow tone="primary">Impact preview</Eyebrow>
          {impactQuery.data && (
            <span className="text-xs text-on-surface-variant">
              {users.length} user{users.length === 1 ? "" : "s"} · {impactQuery.data.role_count}{" "}
              role{impactQuery.data.role_count === 1 ? "" : "s"}
            </span>
          )}
        </div>
        <span aria-hidden="true" className="text-on-surface-variant">
          {open ? "▾" : "▸"}
        </span>
      </button>

      {open && (
        <div className="border-t border-outline-variant px-3 py-3 animate-fade-in-up">
          {impactQuery.isLoading ? (
            <p className="text-xs text-on-surface-variant italic">Loading impact…</p>
          ) : users.length === 0 ? (
            <p className="text-xs text-on-surface-variant italic">
              No users would be affected by this bundle yet.
            </p>
          ) : (
            <div className="flex flex-wrap items-center gap-2">
              {visible.map((user) => (
                <Badge key={user.id} variant="secondary">
                  <UserName id={user.id} fallback={user.name} />
                </Badge>
              ))}
              {overflow > 0 && <Badge variant="outline">+{overflow} more</Badge>}
            </div>
          )}
        </div>
      )}
    </div>
  );
}
