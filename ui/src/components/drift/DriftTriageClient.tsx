"use client";

import { useRef, useState } from "react";

import { ProjectName } from "@/components/names/ProjectName";
import { UserName } from "@/components/names/UserName";
import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Card, CardHeader, CardTitle } from "@/components/ui/Card";
import { EmptyState } from "@/components/ui/EmptyState";
import { Eyebrow } from "@/components/ui/Eyebrow";
import { Input } from "@/components/ui/Input";
import { Select } from "@/components/ui/Select";
import { Skeleton } from "@/components/ui/Skeleton";
import { request } from "@/lib/api-client";
import { type DriftFilter, useDriftItems, useReconcileNow } from "@/lib/queries/useDrift";

import { DriftRowActions } from "./DriftRowActions";

const DRIFT_TYPE_LABELS: Record<string, string> = {
  zitadel_only: "Only in Zitadel",
  mkauth_only: "Only in MkAuth",
};

interface BulkResult {
  attributed: number;
  failed: number;
}

/**
 * Operator triage worklist for out-of-band drift (B2). RED, undismissible —
 * unlike the amber pending-propagation worklist, every row requires an
 * explicit Attribute / Revoke / Mark-external. Filters (user/project/source)
 * drive the list query; "Reconcile now" forces an immediate sweep; bulk
 * attribute lets the operator clear a backlog with one call.
 */
export function DriftTriageClient() {
  const [filter, setFilter] = useState<DriftFilter>({});
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [bulkResult, setBulkResult] = useState<BulkResult | null>(null);
  const [bulkPending, setBulkPending] = useState(false);

  const drift = useDriftItems(filter);
  const reconcile = useReconcileNow();

  const rows = drift.data ?? [];

  // Track which row ids were already on screen so freshly-arrived drift gets
  // a one-shot motion-safe highlight; existing rows never re-animate on
  // unrelated re-renders (e.g. filter changes, refetch of the same set).
  // The first successful load has no "previous" set, so nothing highlights
  // yet — only rows appearing after that count as new. Guarded on
  // `!drift.isLoading` so the empty `rows` during the initial fetch doesn't
  // get mistaken for "previously saw zero rows".
  const seenIds = useRef<Set<string> | null>(null);
  const newIds =
    seenIds.current === null
      ? new Set<string>()
      : new Set(rows.map((r) => r.id).filter((id) => !seenIds.current!.has(id)));
  if (!drift.isLoading) {
    seenIds.current = new Set(rows.map((r) => r.id));
  }

  function toggleSelected(id: string) {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  }

  async function runBulkAttribute() {
    if (selected.size === 0) return;
    setBulkPending(true);
    setBulkResult(null);
    try {
      const result = await request<BulkResult>("/governance/drift/bulk-attribute", {
        method: "POST",
        body: { ids: Array.from(selected), source: "external_backfill" },
      });
      setBulkResult(result);
      setSelected(new Set());
      drift.refetch();
    } finally {
      setBulkPending(false);
    }
  }

  return (
    <div className="p-8 space-y-6 animate-fade-in-up relative z-10">
      <header className="flex items-start justify-between gap-4">
        <div className="flex flex-col gap-2">
          <Eyebrow tone="primary">Governance · Drift</Eyebrow>
          <h1 className="text-3xl font-semibold text-on-surface mt-1 font-display">
            Drift triage
          </h1>
          <p className="text-sm text-on-surface-variant max-w-2xl">
            Out-of-band changes between MkAuth and Zitadel need explicit resolution. Every row must
            be attributed, revoked, or marked external — drift is never auto-resolved.
          </p>
        </div>
        <Button onClick={() => reconcile.mutate()} isPending={reconcile.isPending} variant="destructive">
          Reconcile now
        </Button>
      </header>

      {reconcile.isError && (
        <div role="alert" className="rounded-card border border-error/40 bg-[color-mix(in_srgb,var(--error)_15%,transparent)] px-4 py-3 text-sm text-on-surface">
          {reconcile.error instanceof Error ? reconcile.error.message : "Reconcile failed"}
        </div>
      )}

      <div className="flex flex-wrap gap-3">
        <Input
          value={filter.user_id ?? ""}
          onChange={(e) => setFilter((f) => ({ ...f, user_id: e.target.value || undefined }))}
          placeholder="Filter by user_id"
          aria-label="Filter by user"
          className="w-auto"
        />
        <Input
          value={filter.project_id ?? ""}
          onChange={(e) => setFilter((f) => ({ ...f, project_id: e.target.value || undefined }))}
          placeholder="Filter by project_id"
          aria-label="Filter by project"
          className="w-auto"
        />
        <Select
          value={filter.source ?? ""}
          onChange={(e) => setFilter((f) => ({ ...f, source: e.target.value || undefined }))}
          aria-label="Filter by detection source"
          className="w-auto"
        >
          <option value="">All sources</option>
          <option value="webhook">Webhook</option>
          <option value="reconciliation_sweep">Reconciliation sweep</option>
        </Select>
      </div>

      {selected.size > 0 && (
        <div className="flex items-center gap-3 rounded-card border border-error/40 bg-[color-mix(in_srgb,var(--error)_10%,transparent)] px-4 py-3">
          <span className="text-sm text-on-surface">{selected.size} selected</span>
          <Button size="sm" onClick={runBulkAttribute} isPending={bulkPending}>
            Bulk attribute (external backfill)
          </Button>
        </div>
      )}

      {bulkResult && (
        <div role="status" className="rounded-card border border-outline-variant bg-surface-container-low/40 px-4 py-3 text-sm text-on-surface">
          Bulk attribute: {bulkResult.attributed} attributed, {bulkResult.failed} failed.
        </div>
      )}

      <Card variant="glass" className="border-error/30">
        <CardHeader>
          <CardTitle className="text-error">Drift items ({rows.length})</CardTitle>
        </CardHeader>
        {drift.isLoading ? (
          <div className="space-y-3" aria-busy="true">
            {Array.from({ length: 3 }).map((_, i) => (
              <Skeleton key={i} className="h-16 w-full" />
            ))}
          </div>
        ) : rows.length === 0 ? (
          <EmptyState title="No drift detected" description="MkAuth and Zitadel are in sync." />
        ) : (
          <ul className="space-y-3">
            {rows.map((item) => (
              <li
                key={item.id}
                data-new={newIds.has(item.id) || undefined}
                className={`rounded-card border border-error/30 bg-surface-container-low/40 p-4 ${
                  newIds.has(item.id) ? "motion-safe:animate-row-highlight" : ""
                }`}
              >
                <div className="flex flex-wrap items-center gap-2">
                  <input
                    type="checkbox"
                    aria-label={`Select drift item ${item.id}`}
                    checked={selected.has(item.id)}
                    onChange={() => toggleSelected(item.id)}
                    className="h-4 w-4"
                  />
                  <Badge variant="destructive" className="text-[10px]">
                    {DRIFT_TYPE_LABELS[item.drift_type] ?? item.drift_type}
                  </Badge>
                  <Badge variant="outline" className="text-[10px]">
                    {item.detection_source}
                  </Badge>
                  <span className="text-sm text-on-surface">
                    <UserName id={item.user_id} />
                  </span>
                  <span className="text-sm text-on-surface-variant">
                    on <ProjectName id={item.project_id} />
                  </span>
                  <span className="text-sm text-on-surface-variant">
                    · {item.role_keys.join(", ")}
                  </span>
                </div>
                <DriftRowActions item={item} />
              </li>
            ))}
          </ul>
        )}
      </Card>
    </div>
  );
}
