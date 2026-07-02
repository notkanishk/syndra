"use client";

import { ProjectName } from "@/components/names/ProjectName";
import { UserName } from "@/components/names/UserName";
import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Card, CardHeader, CardTitle } from "@/components/ui/Card";
import { EmptyState } from "@/components/ui/EmptyState";
import { Eyebrow } from "@/components/ui/Eyebrow";
import { Skeleton } from "@/components/ui/Skeleton";
import { useGovernanceSummary } from "@/lib/queries/useGovernance";
import { useDrainPropagations, usePendingPropagations } from "@/lib/queries/usePropagation";

import { DrainResultBanner } from "./DrainResultBanner";

const OP_LABELS: Record<string, string> = {
  add: "Grant",
  replace: "Replace",
  revoke: "Revoke",
};

/**
 * Operator worklist + drain control for the Zitadel propagation outbox. Lists
 * every buffered grant mutation still awaiting Zitadel and exposes the explicit
 * "Resume now" drain. Reachability (from the governance summary) gates the drain
 * the same way the dashboard callout does.
 */
export function PendingPropagationsClient() {
  const pending = usePendingPropagations();
  const governance = useGovernanceSummary();
  const drain = useDrainPropagations();

  const rows = pending.data ?? [];
  const reachable = governance.data?.pending_propagation?.zitadel_reachable ?? true;

  return (
    <div className="p-8 space-y-6 animate-fade-in-up relative z-10">
      <header className="flex items-start justify-between gap-4">
        <div className="flex flex-col gap-2">
          <Eyebrow>Operations</Eyebrow>
          <h1 className="text-3xl font-semibold text-on-surface mt-1 font-display">
            Pending propagations
          </h1>
          <p className="text-sm text-on-surface-variant max-w-2xl">
            MkAuth-mediated grant changes are recorded durably first, then propagated to Zitadel on
            your explicit confirmation. Resume to flush the buffer.
          </p>
        </div>
        <Button
          onClick={() => drain.mutate()}
          disabled={!reachable || rows.length === 0}
          isPending={drain.isPending}
        >
          Resume now
        </Button>
      </header>

      {!reachable && (
        <div
          role="status"
          className="rounded-card border border-warning/40 bg-[color-mix(in_srgb,var(--warning)_15%,transparent)] px-4 py-3 text-sm text-on-surface"
        >
          Zitadel is offline — changes stay buffered until it is reachable again.
        </div>
      )}

      {drain.isError && (
        <div
          role="alert"
          className="rounded-card border border-error/40 bg-[color-mix(in_srgb,var(--error)_15%,transparent)] px-4 py-3 text-sm text-on-surface"
        >
          {drain.error instanceof Error ? drain.error.message : "Drain failed"}
        </div>
      )}

      {drain.isSuccess && <DrainResultBanner result={drain.data} />}

      <Card variant="glass">
        <CardHeader>
          <CardTitle>Awaiting Zitadel ({rows.length})</CardTitle>
        </CardHeader>
        {pending.isLoading ? (
          <div className="space-y-3" aria-busy="true">
            {Array.from({ length: 3 }).map((_, i) => (
              <Skeleton key={i} className="h-16 w-full" />
            ))}
          </div>
        ) : rows.length === 0 ? (
          <EmptyState
            title="Nothing pending"
            description="Every MkAuth-mediated grant change has been propagated to Zitadel."
          />
        ) : (
          <ul className="space-y-3">
            {rows.map((row) => (
              <li
                key={row.id}
                className="rounded-card border border-outline-variant bg-surface-container-low/40 p-4"
              >
                <div className="flex flex-wrap items-center gap-2">
                  <Badge variant="secondary" className="text-[10px]">
                    {OP_LABELS[row.op_type] ?? row.op_type}
                  </Badge>
                  <Badge variant={row.status === "in_flight" ? "default" : "outline"} className="text-[10px]">
                    {row.status}
                  </Badge>
                  <span className="text-sm text-on-surface">
                    <UserName id={row.user_id} />
                  </span>
                  <span className="text-sm text-on-surface-variant">
                    on <ProjectName id={row.project_id} />
                  </span>
                  <span className="text-sm text-on-surface-variant">
                    · {row.role_keys.join(", ")}
                  </span>
                  {row.attempts > 0 && (
                    <span className="ml-auto text-xs text-warning">retried {row.attempts}×</span>
                  )}
                </div>
              </li>
            ))}
          </ul>
        )}
      </Card>
    </div>
  );
}
