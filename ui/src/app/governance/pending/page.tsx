"use client";

import { useMemo, useState } from "react";

import { EmptyState, ListStates, RowSkeleton } from "@/components/states";
import { ActionOutcome } from "@/components/ui/ActionOutcome";
import { Mono } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Card, CardColumns } from "@/components/ui/Card";
import { PageHeader } from "@/components/ui/PageHeader";
import { ProjectName, UserName } from "@/components/names";
import { outcomeFromDrain } from "@/lib/drain-outcome";
import { outcomeFromError, type ActionOutcome as Outcome } from "@/lib/outcome";
import { useGovernanceSummary } from "@/lib/queries/useGovernance";
import {
  useDrainPropagations,
  usePendingPropagations,
  type PendingPropagationRow,
} from "@/lib/queries/usePropagation";
import { ClockTime, Relative } from "@/components/ui/Time";

/**
 * S3 · Automation › Pending changes.
 *
 * Queued identity-provider writes awaiting confirmation. The queue is the
 * safety property, not a defect: a write sitting here is a write that has not
 * happened yet and can still be examined.
 *
 * Grouped by cascade, never flat by timestamp. A half-applied cascade is the
 * thing that creates unexplained access, so the writes one event produced stay
 * visibly together — they confirm together or not at all.
 */
export default function PendingChangesPage() {
  const pending = usePendingPropagations();
  const summary = useGovernanceSummary();
  const drain = useDrainPropagations();

  const rows = useMemo(() => pending.data ?? [], [pending.data]);
  const reachable = summary.data?.pending_propagation.zitadel_reachable ?? true;

  // The drain reports under the button that ran it, and stays there. It used
  // to be a toast, which meant the account of a pass that requeued eight
  // writes was gone in four seconds — on the one screen whose entire subject
  // is what is still outstanding.
  const [outcome, setOutcome] = useState<Outcome | null>(null);

  const groups = useMemo(() => groupByCascade(rows), [rows]);

  return (
    <div className="flex flex-col gap-[18px]">
      <PageHeader
        title="Pending changes"
        meta="Writes Syndra has decided on and the identity provider hasn't received yet."
        actions={
          <Button
            variant="accent"
            disabled={!reachable || rows.length === 0}
            isPending={drain.isPending}
            onClick={async () => {
              setOutcome(null);
              try {
                setOutcome(outcomeFromDrain(await drain.mutateAsync()));
              } catch (error) {
                setOutcome(outcomeFromError(error));
              }
            }}
          >
            Confirm all {rows.length}
          </Button>
        }
      />

      {outcome && <ActionOutcome outcome={outcome} />}

      {/*
        The reason is a visible strip, not a tooltip on a greyed button. Hover
        does not exist on touch and does not survive a screenshot sent to a
        colleague — and the point of this copy is that nothing is broken.
      */}
      {!reachable && (
        <div className="warn-note flex items-start gap-3.5 px-5 py-4">
          <span
            aria-hidden
            className="mt-px flex h-5 w-5 flex-none items-center justify-center rounded-pill bg-warn-soft text-[12px] font-bold text-warn-text"
          >
            !
          </span>
          <div>
            <div className="text-[15px] font-semibold text-warn-text">
              Identity provider unreachable as of <ClockTime />.
            </div>
            <p className="mt-1 max-w-[80ch] text-[14px] leading-[1.55] text-muted">
              Confirming is disabled, not failing silently. Writes stay queued and in order;
              nothing is lost and nothing has reached a machine.
            </p>
          </div>
        </div>
      )}

      <Card>
        <CardColumns>
          <span className="w-[150px]">Who</span>
          <span className="flex-1">Write</span>
          <span className="w-[160px]">Caused by</span>
          <span className="w-[78px] text-right">Queued</span>
        </CardColumns>

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
              guidance="Every decision Syndra has made has reached the identity provider."
              resolved
            />
          }
        >
          {groups.map((group) => (
            <div key={group.key}>
              {group.rows.map((row) => (
                <div
                  key={row.id}
                  className="row-divider flex flex-wrap items-center gap-[18px] px-5 py-3"
                >
                  <span className="w-[150px] shrink-0 truncate text-[14.5px] font-semibold">
                    <UserName id={row.user_id} />
                  </span>

                  <span className="min-w-[220px] flex-1 truncate text-[14px]">
                    <span className="text-muted">{verb(row.op_type)}</span>{" "}
                    <ProjectName id={row.project_id} /> /{" "}
                    {(row.role_keys ?? []).map((key) => (
                      <Mono key={key} className="mr-1.5">
                        {key}
                      </Mono>
                    ))}
                  </span>

                  <span className="w-[160px] shrink-0 truncate text-[13px]">
                    <Mono className="text-accent-text">{shortId(row.source_ref, "R")}</Mono>{" "}
                    <Mono className="text-faint">{shortId(row.cascade_id, "c")}</Mono>
                  </span>

                  <span className="w-[78px] shrink-0 text-right text-[13px] text-faint">
                    <Relative iso={row.created_at} />
                  </span>

                  {row.status === "failed" && (
                    <div className="w-full text-[13px] text-danger-text">
                      Failed{row.last_error ? ` — ${row.last_error}` : ""}
                    </div>
                  )}
                </div>
              ))}

              {group.rows.length > 1 && (
                <p className="row-divider bg-surface-0 px-5 py-2.5 text-[13.5px] leading-[1.5] text-muted">
                  These {group.rows.length} writes share cascade{" "}
                  <Mono className="text-faint">{shortId(group.key, "c")}</Mono> — one event
                  produced all of them. They confirm together or not at all.
                </p>
              )}
            </div>
          ))}
        </ListStates>
      </Card>
    </div>
  );
}

interface CascadeGroup {
  key: string;
  rows: PendingPropagationRow[];
}

/**
 * Rows arrive ordered by cascade then time. Rows predating the cascade_id
 * column group under their own id, so nothing is dropped from the queue just
 * because it was queued before the schema knew how to group it.
 */
function groupByCascade(rows: PendingPropagationRow[]): CascadeGroup[] {
  const order: string[] = [];
  const byKey = new Map<string, PendingPropagationRow[]>();
  for (const row of rows) {
    const key = row.cascade_id || row.id;
    if (!byKey.has(key)) {
      byKey.set(key, []);
      order.push(key);
    }
    byKey.get(key)!.push(row);
  }
  return order.map((key) => ({ key, rows: byKey.get(key)! }));
}

function verb(opType: string): string {
  if (opType === "revoke") return "remove";
  if (opType === "replace") return "replace";
  return "grant";
}

/** "c_8841" from a uuid — short enough to compare across three screens by eye. */
function shortId(id: string | undefined, prefix: string): string {
  if (!id) return "—";
  return `${prefix}_${id.replace(/-/g, "").slice(0, 4)}`;
}
