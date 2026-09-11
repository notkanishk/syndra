"use client";

import Link from "next/link";
import { Term } from "@/components/ui/Term";
import { useMemo, useState } from "react";

import { EmptyState, ListStates, RowSkeleton } from "@/components/states";
import { ActionOutcome } from "@/components/ui/ActionOutcome";
import { Mono } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Card, CardColumns } from "@/components/ui/Card";
import { PageHeader } from "@/components/ui/PageHeader";
import { ProjectName, UserName } from "@/components/names";
import { shortId } from "@/lib/audit-vocabulary";
import { outcomeFromDrain } from "@/lib/drain-outcome";
import { outcomeFromError, type ActionOutcome as Outcome } from "@/lib/outcome";
import { useGovernanceSummary } from "@/lib/queries/useGovernance";
import { useBundles, type BundleRow } from "@/lib/queries/useBundles";
import { useMappingRules, type MappingRuleRow } from "@/lib/queries/useMappingRules";
import {
  useDrainPropagations,
  usePendingPropagations,
  type PendingPropagationRow,
} from "@/lib/queries/usePropagation";
import { ClockTime, Relative } from "@/components/ui/Time";

/**
 * S3 · Automation › Pending changes.
 *
 * Changes waiting to be sent to Zitadel or a connected system. The queue is the
 * safety property, not a defect: a change sitting here is a change that has not
 * happened yet and can still be examined.
 *
 * Grouped by cascade, never flat by timestamp. A half-applied cascade is the
 * thing that creates unexplained access, so the changes one event produced stay
 * visibly together — they are sent together or not at all.
 */
export default function PendingChangesPage() {
  const pending = usePendingPropagations();
  const summary = useGovernanceSummary();
  const drain = useDrainPropagations();
  // Named, not just numbered: a bundle or rule is the thing an operator
  // recognizes. Both lists are small and already cached elsewhere in the
  // product, so loading them here to label a handle costs nothing a reload
  // of Bundles or Access wouldn't have paid anyway.
  const bundles = useBundles();
  const rules = useMappingRules();

  const rows = useMemo(() => pending.data ?? [], [pending.data]);
  const reachable = summary.data?.pending_propagation.zitadel_reachable ?? true;

  // The send reports under the button that ran it, and stays there. It used
  // to be a toast, which meant the account of a pass that requeued eight
  // changes was gone in four seconds — on the one screen whose entire subject
  // is what is still outstanding.
  const [outcome, setOutcome] = useState<Outcome | null>(null);

  const groups = useMemo(() => groupByCascade(rows), [rows]);

  return (
    <div className="flex flex-col gap-[18px]">
      <PageHeader
        title="Pending changes"
        lede={
          <>
            Changes Syndra has decided on that have not reached <Term name="zitadel">Zitadel</Term>{" "}
            or a connected system yet. They wait in the <Term name="outbox">outbox</Term>; nothing
            here takes effect until you send it.
          </>
        }
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
            Send {rows.length} {rows.length === 1 ? "change" : "changes"}
          </Button>
        }
      />

      <p className="max-w-[80ch] text-[14px] leading-[1.55] text-muted">
        Sending delivers every change below to the system it is for, in order, and each
        person&rsquo;s access changes as theirs arrives. Revocations also send on their own,
        every few minutes; everything else waits for you.
      </p>

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
              Zitadel is not answering, as of <ClockTime />.
            </div>
            <p className="mt-1 max-w-[80ch] text-[14px] leading-[1.55] text-muted">
              Sending is paused. Nothing is lost: the changes wait in order, and nothing has
              reached any system. Send them once Zitadel is back.
            </p>
          </div>
        </div>
      )}

      <Card>
        <CardColumns>
          <span className="w-[150px]">Who</span>
          <span className="flex-1">Change</span>
          <span className="w-[160px]">Caused by</span>
          <span className="w-[78px] text-right">Waiting</span>
        </CardColumns>

        <ListStates
          isLoading={pending.isLoading}
          error={pending.error}
          isEmpty={rows.length === 0}
          onRetry={() => pending.refetch()}
          errorTitle="Couldn't load pending changes."
          skeleton={<RowSkeleton rows={4} label="Loading pending changes" />}
          empty={
            <EmptyState
              title="Nothing is waiting to be sent."
              guidance="Every change Syndra has decided on has reached the system it was for."
              resolved
            />
          }
        >
          {groups.map((group) => (
            <div key={group.key}>
              {group.rows.map((row) => (
                <div
                  key={row.id}
                  className="row-divider flex min-h-[60px] flex-col items-start gap-1.5 px-5 py-3 tablet:flex-row tablet:flex-wrap tablet:items-center tablet:gap-[18px]"
                >
                  <span className="w-full truncate text-[14.5px] font-semibold tablet:w-[150px] tablet:shrink-0">
                    <UserName id={row.user_id} />
                  </span>

                  <span className="w-full text-[14px] tablet:min-w-[220px] tablet:flex-1 tablet:truncate">
                    <span className="text-muted">{verb(row.op_type)}</span>{" "}
                    <ProjectName id={row.project_id} /> /{" "}
                    {(row.role_keys ?? []).map((key) => (
                      <Mono key={key} className="mr-1.5">
                        {key}
                      </Mono>
                    ))}
                  </span>

                  <span className="w-full text-[13px] tablet:w-[160px] tablet:shrink-0">
                    <CausedBy row={row} bundles={bundles.data} rules={rules.data} />
                  </span>

                  <span className="text-[13px] text-faint tablet:w-[78px] tablet:shrink-0 tablet:text-right">
                    <Relative iso={row.created_at} />
                  </span>

                  {row.status === "failed" && (
                    <div className="w-full text-[13px] text-danger-text">
                      Failed, and Syndra will try again
                      {row.last_error ? `: ${row.last_error}` : "."}
                    </div>
                  )}
                </div>
              ))}

              {group.rows.length > 1 && (
                <p className="row-divider bg-surface-0 px-5 py-2.5 text-[13.5px] leading-[1.5] text-muted">
                  These {group.rows.length} changes come from one edit (
                  <Mono className="text-faint">{shortId(group.key, "c")}</Mono>). They are
                  sent together or not at all.
                </p>
              )}
            </div>
          ))}
        </ListStates>
        <p className="px-5 py-3 text-[13px] text-faint">
          Handles: R_ is an automatic rule, b_ a bundle, c_ one edit&rsquo;s set of changes.
        </p>
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

/**
 * What set this change off — a bundle's name or a rule's mapping, never the
 * bare handle on its own. The handle stays alongside, muted, for tracing back
 * to Bundles, Access or Change history; it just never stands in for the name.
 */
function CausedBy({
  row,
  bundles,
  rules,
}: {
  row: PendingPropagationRow;
  bundles?: BundleRow[];
  rules?: MappingRuleRow[];
}) {
  const isBundle = row.source === "bundle";
  const bundle = isBundle ? bundles?.find((b) => b.id === row.source_ref) : undefined;
  const rule = !isBundle ? rules?.find((r) => r.id === row.source_ref) : undefined;

  const name = bundle
    ? bundle.name
    : rule
      ? `${rule.source_role} → ${rule.target_role}`
      : isBundle
        ? "Bundle"
        : "Automatic rule";

  return (
    <span className="flex flex-wrap items-baseline gap-x-1.5">
      <span className="truncate">{name}</span>
      <Mono className="text-faint">{shortId(row.source_ref, isBundle ? "b" : "R")}</Mono>
      {row.cascade_id ? (
        <Link
          href={`/operations/cascades?cascade=${encodeURIComponent(row.cascade_id)}`}
          aria-label="See what this change set off, in Change history"
          className="motion-tint hover:text-accent-text"
        >
          <Mono className="text-faint">{shortId(row.cascade_id, "c")}</Mono>
        </Link>
      ) : (
        <Mono className="text-faint">{shortId(row.cascade_id, "c")}</Mono>
      )}
    </span>
  );
}

function verb(opType: string): string {
  if (opType === "revoke") return "revoke";
  if (opType === "replace") return "replace";
  return "give";
}
