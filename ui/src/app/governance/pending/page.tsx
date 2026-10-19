"use client";

import Link from "next/link";
import { Term } from "@/components/ui/Term";
import { useEffect, useMemo, useState } from "react";
import { LeavingRow, useLeavingRows, type LeaveProps } from "@/components/ui/LeavingRows";
import { Unfold } from "@/components/ui/Unfold";
import { formatClock } from "@/lib/format";
import { useQueueCleared } from "@/lib/useQueueCleared";

import { EmptyState, ListStates, RowSkeleton } from "@/components/states";
import { ActionOutcome } from "@/components/ui/ActionOutcome";
import { humanizeKey } from "@/lib/format";
import { Mono } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Card, CardColumns } from "@/components/ui/Card";
import { PageHeader } from "@/components/ui/PageHeader";
import { ProjectName, UserName } from "@/components/names";
import { machineName, shortId } from "@/lib/audit-vocabulary";
import { targetLabel } from "@/lib/nav";
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
import { Notice } from "@/components/ui/Notice";
import { ClockTime, Relative } from "@/components/ui/Time";
import { waitIsChosen, waitReasonLine } from "@/lib/wait-reason";

/**
 * S3 · Automation › Pending changes.
 *
 * Changes waiting for you to send to Zitadel or a connected system. The queue is the
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

  // S5: a row that goes on a later read (cancelled by someone, sent by someone)
  // fades where it was and closes its gap after, and not under the pointer.
  const leaving = useLeavingRows(rows, (row) => row.id, !pending.isLoading && !pending.error);
  const groups = groupByCascade(leaving.rows);
  // S13: a queue that empties while you watch says so once.
  const { cleared, beginSend, failedSend } = useQueueCleared(
    rows,
    !pending.isLoading && !pending.error,
  );
  const [folded, setFolded] = useState(false);
  if (!cleared && folded) setFolded(false);
  const sentCount = cleared ? cleared.items.length : rows.length;

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
          // With nothing left to send it fades out, then goes.
          cleared && folded ? null : (
            <span className={cleared ? "tint-out inline-flex" : "inline-flex"} inert={cleared ? true : undefined}>
              <Button
                variant="accent"
                disabled={!reachable || rows.length === 0}
                isPending={drain.isPending}
                onClick={async () => {
                  setOutcome(null);
                  beginSend(rows.length);
                  try {
                    setOutcome(outcomeFromDrain(await drain.mutateAsync()));
                  } catch (error) {
                    failedSend();
                    setOutcome(outcomeFromError(error));
                  }
                }}
              >
                Send {sentCount} {sentCount === 1 ? "change" : "changes"}
              </Button>
            </span>
          )
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
        <Notice tone="warn" live title="Zitadel is not answering.">
          As of <ClockTime />. Sending waits until Zitadel answers. Nothing is lost: the changes wait in
          order, and nothing has reached any system. Send them once Zitadel is back.
        </Notice>
      )}

      <Card>
        <CardColumns>
          <span className="w-[150px]">Who</span>
          <span className="flex-1">Change</span>
          <span className="w-[160px]">Caused by</span>
          <span className="w-[200px]">Why it&rsquo;s waiting</span>
          <span className="w-[78px] text-right">Waiting</span>
        </CardColumns>

        {cleared ? (
          <>
            <ClearedGroups
              groups={groupByCascade(cleared.items)}
              bundles={bundles.data}
              rules={rules.data}
              onFolded={() => setFolded(true)}
            />
            {folded && <EverythingSent mine={cleared.mine} />}
          </>
        ) : (
          <div {...leaving.listProps}>
            <ListStates
              isLoading={pending.isLoading}
              error={pending.error}
              isEmpty={rows.length === 0}
              onRetry={() => pending.refetch()}
              errorTitle="Couldn't load pending changes."
              skeleton={<RowSkeleton rows={4} label="Loading pending changes" />}
              empty={
                <EmptyState
                  title="Nothing is waiting for you to send."
                  guidance="Every change Syndra has decided on has reached the system it was for."
                  resolved
                />
              }
            >
              <PendingGroups groups={groups} bundles={bundles.data} rules={rules.data} leave={leaving.props} />
            </ListStates>
          </div>
        )}
        <p className="px-5 py-3 text-[13px] text-faint">
          Handles: R_ is an automatic rule, b_ a bundle, c_ one edit&rsquo;s set of changes.
        </p>
      </Card>
    </div>
  );
}

/** The queue's rows, grouped by the edit that produced them. */
function PendingGroups({
  groups,
  bundles,
  rules,
  leave,
}: {
  groups: CascadeGroup[];
  bundles?: BundleRow[];
  rules?: MappingRuleRow[];
  /** Each row's seat in a read that is taking it away (S5). */
  leave: (id: string) => LeaveProps;
}) {
  // A row on its way out is not one of "these N changes".
  const waiting = (group: CascadeGroup) => group.rows.filter((row) => !leave(row.id).leaving).length;
  return (
    <>
      {groups.map((group) => (
        <div key={group.key}>
          {group.rows.map((row) => (
            <LeavingRow key={row.id} {...leave(row.id)}>
            <div
              className="row-divider flex min-h-[60px] flex-col items-start gap-1.5 px-5 py-3 tablet:flex-row tablet:flex-wrap tablet:items-center tablet:gap-[18px]"
            >
              <span className="w-full truncate text-[14.5px] font-semibold tablet:w-[150px] tablet:shrink-0">
                <UserName id={row.user_id} />
              </span>

              <span className="w-full text-[14px] tablet:min-w-[220px] tablet:flex-1">
                {row.op_type === "apply" ? (
                  <SystemChange row={row} />
                ) : (
                  <>
                    <span className="text-muted">{verb(row.op_type)}</span>{" "}
                    <ProjectName id={row.project_id} /> /{" "}
                    {(row.role_keys ?? []).map((key) => (
                      <Mono key={key} className="mr-1.5">
                        {key}
                      </Mono>
                    ))}
                  </>
                )}
              </span>

              <span className="w-full text-[13px] tablet:w-[160px] tablet:shrink-0">
                <CausedBy row={row} bundles={bundles} rules={rules} />
              </span>

              <WhyWaiting row={row} />

              <span className="text-[13px] text-faint tablet:w-[78px] tablet:shrink-0 tablet:text-right">
                <Relative iso={row.created_at} />
              </span>

              {row.status === "failed" && (
                <div className="w-full text-[13px] text-danger-text [overflow-wrap:anywhere]">
                  Failed, and Syndra will try again
                  {row.last_error ? `: ${row.last_error}` : "."}
                </div>
              )}
            </div>
            </LeavingRow>
          ))}

          {waiting(group) > 1 && (
            <p className="row-divider bg-surface-0 px-5 py-2.5 text-[13.5px] leading-[1.5] text-muted">
              These {waiting(group)} changes come from one edit (
              <Mono className="text-faint">{shortId(group.key, "c")}</Mono>). They are
              sent together or not at all.
            </p>
          )}
        </div>
      ))}
    </>
  );
}

const STAYING: (id: string) => LeaveProps = () => ({ leaving: false, held: false, onGone: () => {} });

/** What was sent, fading first and then folding shut (S13); `onFolded` says the gap has closed. */
function ClearedGroups({
  groups,
  bundles,
  rules,
  onFolded,
}: {
  groups: CascadeGroup[];
  bundles?: BundleRow[];
  rules?: MappingRuleRow[];
  onFolded: () => void;
}) {
  // Mounted open, then closed: the first frame is the list as it was.
  const [open, setOpen] = useState(true);
  useEffect(() => setOpen(false), []);
  return (
    <Unfold open={open} onClosed={onFolded}>
      <PendingGroups groups={groups} bundles={bundles} rules={rules} leave={STAYING} />
    </Unfold>
  );
}

/**
 * The queue has emptied (S13). One line rises 6px into the list's place; its dot
 * blooms once only when this operator's send did it. A queue someone else sent
 * (or that was cancelled away) says the line without the bloom and without a
 * count or a time it cannot know.
 */
function EverythingSent({ mine }: { mine: { count: number; at: string } | null }) {
  return (
    <div role="status" className="lift-in flex flex-col justify-center gap-2.5 px-6 py-8">
      <div className="flex items-center gap-2.5">
        <span aria-hidden className={`h-2 w-2 flex-none rounded-pill bg-healthy ${mine ? "bloom" : ""}`} />
        <div className="type-empty-title">Everything has reached Zitadel</div>
      </div>
      <p className="max-w-[60ch] text-[14px] text-faint">
        {mine
          ? `${mine.count} ${mine.count === 1 ? "change" : "changes"} sent at ${formatClock(mine.at)} · nothing is waiting for you`
          : "Nothing is waiting for you to send"}
      </p>
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
/**
 * A connected system's change has no project or roles to name. It says which
 * system, and why it was queued; rows from before the reason was kept say what
 * every such row does.
 */
function SystemChange({ row }: { row: PendingPropagationRow }) {
  return (
    <>
      <span className="text-muted">{targetLabel(row.target ?? "")} account:</span>{" "}
      {row.reason || "brought in line with their roles."}
    </>
  );
}

/** Empty when the row carries no reason; warn only for a send that stopped. */
function WhyWaiting({ row }: { row: PendingPropagationRow }) {
  const role = (row.role_keys ?? []).map(humanizeKey).join(", ") || "It";
  const line = waitReasonLine(row.wait_reason, role, row.created_at);
  return (
    <span
      className={`w-full text-[13px] tablet:w-[200px] tablet:shrink-0 ${
        waitIsChosen(row.wait_reason) ? "text-muted" : "text-warn-text"
      }`}
    >
      {line}
    </span>
  );
}

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

  const isRule = row.source === "rule";
  // A connected system's change is queued by whoever made it, or by the check
  // that found it out of step. "Given by hand" was wrong for both.
  if (row.op_type === "apply" && row.initiated_by) {
    return (
      <span className="truncate">
        {row.initiated_by === "system:reconcile" ? (
          "Check for differences"
        ) : row.initiated_by.startsWith("system:") ? (
          machineName(row.initiated_by)
        ) : (
          <UserName id={row.initiated_by} />
        )}
      </span>
    );
  }
  // Named by what caused it, never by a handle alone. A direct grant has no
  // handle: somebody chose it by hand on the person's page.
  const name = bundle
    ? bundle.name
    : rule
      ? `${rule.source_role} → ${rule.target_role}`
      : isBundle
        ? "Bundle"
        : isRule
          ? "Automatic rule"
          : row.source === "direct"
            ? "Given by hand"
            : humanizeKey(row.source);

  return (
    <span className="flex flex-wrap items-baseline gap-x-1.5">
      <span className="truncate">{name}</span>
      {(isBundle || isRule) && row.source_ref && (
        <Mono className="text-faint">{shortId(row.source_ref, isBundle ? "b" : "R")}</Mono>
      )}
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
