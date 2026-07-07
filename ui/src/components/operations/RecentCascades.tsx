"use client";

import { BundleName } from "@/components/names/BundleName";
import { ProjectName } from "@/components/names/ProjectName";
import { UserName } from "@/components/names/UserName";
import { Badge } from "@/components/ui/Badge";
import { Card, CardHeader, CardTitle } from "@/components/ui/Card";
import { EmptyState } from "@/components/ui/EmptyState";
import { Skeleton } from "@/components/ui/Skeleton";
import { useRecentCascades } from "@/lib/queries/useConfirmationMode";

const OP_LABELS: Record<string, string> = {
  add: "Grant",
  replace: "Replace",
  revoke: "Revoke",
};

const SOURCE_LABELS: Record<string, string> = {
  bundle: "Bundle",
  rule: "Mapping rule",
  lifecycle_cascade: "Lifecycle",
};

function relativeAge(timestamp?: string): string {
  if (!timestamp) return "—";
  const ms = Date.now() - new Date(timestamp).getTime();
  if (Number.isNaN(ms) || ms < 0) return "—";
  const seconds = Math.floor(ms / 1000);
  if (seconds < 60) return `${seconds}s ago`;
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes}m ago`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.floor(hours / 24);
  return `${days}d ago`;
}

/**
 * Recent automated cascade feed (Task 22). Lists applied bundle/rule/lifecycle
 * outbox projections — the operator's evidence that "auto" confirmation mode
 * never means invisible: every fired cascade traces back to its originating
 * bundle/rule via `source`/`source_ref`.
 */
export function RecentCascades() {
  const cascades = useRecentCascades();
  const rows = cascades.data ?? [];

  return (
    <Card variant="glass">
      <CardHeader>
        <CardTitle>Recent cascades ({rows.length})</CardTitle>
      </CardHeader>
      {cascades.isLoading ? (
        <div className="space-y-3" aria-busy="true">
          {Array.from({ length: 3 }).map((_, i) => (
            <Skeleton key={i} className="h-14 w-full" />
          ))}
        </div>
      ) : rows.length === 0 ? (
        <EmptyState
          title="No cascades yet"
          description="Bundle, mapping-rule, and lifecycle projections that reached Zitadel will show up here."
        />
      ) : (
        <ul className="space-y-2">
          {rows.map((row) => (
            <li
              key={row.id}
              className="rounded-card border border-outline-variant bg-surface-container-low/40 p-3"
            >
              <div className="flex flex-wrap items-center gap-2">
                <Badge variant="outline" className="text-[10px]">
                  {SOURCE_LABELS[row.source] ?? row.source}
                </Badge>
                <Badge variant="secondary" className="text-[10px]">
                  {OP_LABELS[row.op_type] ?? row.op_type}
                </Badge>
                <span className="text-sm text-on-surface">
                  <UserName id={row.user_id} />
                </span>
                <span className="text-sm text-on-surface-variant">
                  on <ProjectName id={row.project_id} />
                </span>
                <span className="text-sm text-on-surface-variant">· {row.role_keys.join(", ")}</span>
                {row.source_ref && (
                  <span className="text-xs text-on-surface-variant">
                    via{" "}
                    {row.source === "bundle" ? (
                      <BundleName id={row.source_ref} fallback={row.source_ref} />
                    ) : (
                      row.source_ref
                    )}
                  </span>
                )}
                <span className="ml-auto text-[11px] text-on-surface-variant whitespace-nowrap">
                  {relativeAge(row.completed_at)}
                </span>
              </div>
            </li>
          ))}
        </ul>
      )}
    </Card>
  );
}
