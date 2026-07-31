"use client";

import { toast } from "sonner";

import { EmptyState, ListStates, RowSkeleton } from "@/components/states";
import { Badge, Mono } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Card, CardHeader } from "@/components/ui/Card";
import { PageHeader } from "@/components/ui/PageHeader";
import { ProjectName, UserName } from "@/components/names";
import { useGovernanceSummary } from "@/lib/queries/useGovernance";
import { useDrainPropagations, usePendingPropagations } from "@/lib/queries/usePropagation";
import { Relative } from "@/components/ui/Time";

/**
 * S3 · Automation › Pending changes.
 *
 * Queued identity-provider writes awaiting confirmation. The queue is the
 * safety property, not a defect: a write sitting here is a write that has not
 * happened yet and can still be examined.
 */
export default function PendingChangesPage() {
  const pending = usePendingPropagations();
  const summary = useGovernanceSummary();
  const drain = useDrainPropagations();

  const rows = pending.data ?? [];
  const reachable = summary.data?.pending_propagation.zitadel_reachable ?? true;

  return (
    <div className="flex flex-col gap-[18px]">
      <PageHeader
        title="Pending changes"
        meta="Writes MkAuth has decided on and the identity provider hasn't received yet."
        actions={
          <Button
            variant="accent"
            disabled={!reachable || rows.length === 0}
            isPending={drain.isPending}
            reason={
              !reachable
                ? "Disabled — the identity provider is unreachable. Writes stay queued; nothing is lost."
                : undefined
            }
            onClick={async () => {
              try {
                const result = await drain.mutateAsync();
                toast.success(`${result?.applied ?? 0} applied, ${result?.failed ?? 0} failed.`);
              } catch (error) {
                toast.error(error instanceof Error ? error.message : "The drain didn't run.");
              }
            }}
          >
            Resume now
          </Button>
        }
      />

      <Card>
        <CardHeader title="Queued" count={rows.length} />
        <ListStates
          isLoading={pending.isLoading}
          error={pending.error}
          isEmpty={rows.length === 0}
          onRetry={() => pending.refetch()}
          errorTitle="Couldn't load the queue."
          skeleton={<RowSkeleton rows={4} label="Loading queued writes" />}
          empty={
            <EmptyState
              title="Nothing is waiting."
              guidance="Every decision MkAuth has made has reached the identity provider."
            />
          }
        >
          {rows.map((row) => (
            <div key={row.id} className="row-divider flex flex-wrap items-center gap-4 px-5 py-3.5">
              <Badge tone={row.op_type === "revoke" ? "danger" : "accent"}>
                {row.op_type === "revoke" ? "Remove" : row.op_type === "replace" ? "Replace" : "Add"}
              </Badge>
              <div className="min-w-[220px] flex-1">
                <div className="text-[15px] font-semibold">
                  <UserName id={row.user_id} />
                </div>
                <div className="truncate text-[13.5px] text-muted">
                  <ProjectName id={row.project_id} /> ·{" "}
                  {(row.role_keys ?? []).map((key) => (
                    <Mono key={key} className="mr-1.5">
                      {key}
                    </Mono>
                  ))}
                </div>
              </div>
              <div className="w-[140px] text-[13px] text-faint">
                queued <Relative iso={row.created_at} />
              </div>
              <Badge tone={row.status === "failed" ? "danger" : "neutral"}>{row.status}</Badge>
              {row.last_error && (
                <div className="w-full text-[13px] text-danger-text">{row.last_error}</div>
              )}
            </div>
          ))}
        </ListStates>
      </Card>
    </div>
  );
}
