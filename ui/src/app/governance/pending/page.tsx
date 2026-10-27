"use client";

import Link from "next/link";
import { useEffect, useMemo, useState } from "react";
import { LeavingRow, useLeavingRows, type LeaveProps } from "@/components/ui/LeavingRows";
import { Unfold } from "@/components/ui/Unfold";
import { formatClock } from "@/lib/format";
import { useQueueCleared } from "@/lib/useQueueCleared";

import { EmptyState, ListStates, RowSkeleton } from "@/components/states";
import { ActionOutcome } from "@/components/ui/ActionOutcome";
import { humanizeKey } from "@/lib/format";
import { Mono, StatusDot } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Card, CardColumns } from "@/components/ui/Card";
import { FilterBar } from "@/components/ui/FilterBar";
import { Input } from "@/components/ui/Input";
import { WaitsMark } from "@/components/ui/WaitsMark";
import { useNameResolver } from "@/lib/queries/useNameResolver";
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
import { Relative } from "@/components/ui/Time";
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

  // Search narrows what is shown, never what Send sends: the note beside it says so.
  const resolver = useNameResolver();
  const [query, setQuery] = useState("");
  const needle = query.trim().toLowerCase();
  const matched = useMemo(
    () => (needle ? rows.filter((row) => matches(row, needle, resolver)) : rows),
    [rows, needle, resolver],
  );
  // S5: a row that goes on a later read (cancelled by someone, sent by someone)
  // fades where it was and closes its gap after, and not under the pointer. A row
  // a search drops goes the same way, but its gap does not wait for the pointer
  // (A-05 §2.10), and the rows that still match are the same rows.
  const leaving = useLeavingRows(matched, (row) => row.id, !pending.isLoading && !pending.error, {
    narrowedBy: needle,
  });
  const shown = leaving.rows;
  const groups = groupByCascade(shown);
  // S13: a queue that empties while you watch says so once.
  const { cleared, beginSend, failedSend } = useQueueCleared(
    rows,
    !pending.isLoading && !pending.error,
  );
  const [folded, setFolded] = useState(false);
  if (!cleared && folded) setFolded(false);
  const sentCount = cleared ? cleared.items.length : rows.length;
  const known = !pending.isLoading && !pending.error;

  return (
    <div className="flex flex-col gap-[18px]">
      <PageHeader
        title="Pending changes"
        lede="Changes that have not reached Zitadel or a connected system yet. They wait for you: sending delivers all of them, in order, and each person's access changes as theirs arrives. Revocations also send on their own every few minutes."
        actions={
          // With nothing left to send it fades out, then goes.
          cleared && folded ? null : (
            <span className={cleared ? "tint-out inline-flex" : "inline-flex"} inert={cleared ? true : undefined}>
              <Button
                variant="accent"
                disabled={!reachable || !known || rows.length === 0}
                // Zitadel not answering is the shell's one Notice, above the
                // title. The reason a greyed button gives is visible text
                // beside it, never a tooltip: hover does not exist on touch.
                reason={!reachable ? "Sending waits until Zitadel answers. Nothing is lost." : undefined}
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
                {known ? `Send ${sentCount} ${sentCount === 1 ? "change" : "changes"}` : "Send changes"}
              </Button>
            </span>
          )
        }
      />

      {outcome && <ActionOutcome outcome={outcome} />}

      {known && rows.length > 0 && !cleared && (
        <div className="flex flex-col gap-3.5">
          <FilterBar
            search={
              <Input
                value={query}
                onChange={(event) => setQuery(event.target.value)}
                placeholder="Search person or role…"
                aria-label="Search pending changes by person or role"
              />
            }
          />
          <span className="text-[13.5px] text-muted">
            {needle
              ? `${shown.length} of the ${rows.length} changes match. Sending sends all ${rows.length}, in order.`
              : "They are sent in this order."}
          </span>
        </div>
      )}

      <Card>
        <CardColumns>
          <span className="w-[130px]">Who</span>
          <span className="flex-1">Change</span>
          <span className="w-[130px]">Caused by</span>
          <span className="w-[110px] text-right">Waiting</span>
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
              isEmpty={shown.length === 0}
              onRetry={() => pending.refetch()}
              errorTitle="Couldn't load pending changes."
              skeleton={<RowSkeleton rows={4} label="Loading pending changes" />}
              empty={
                rows.length === 0 ? (
                  <EmptyState
                    title="Nothing is waiting for you to send."
                    guidance="Every change Syndra has decided on has reached the system it was for."
                    resolved
                  />
                ) : (
                  <EmptyState
                    title={`No pending change matches \u201c${query.trim()}\u201d.`}
                    guidance="Search by a person's name or a role."
                  />
                )
              }
            >
              <PendingGroups
                groups={groups}
                whole={groupByCascade(rows)}
                bundles={bundles.data}
                rules={rules.data}
                leave={leaving.props}
              />
            </ListStates>
          </div>
        )}
      </Card>
    </div>
  );
}

/** The queue's rows, grouped by the edit that produced them. */
function PendingGroups({
  groups,
  whole,
  bundles,
  rules,
  leave,
}: {
  groups: CascadeGroup[];
  /** The same batches as the read has them, before a search narrowed them: a band speaks for the whole edit, not for what is shown. */
  whole?: CascadeGroup[];
  bundles?: BundleRow[];
  rules?: MappingRuleRow[];
  /** Each row's seat in a read that is taking it away (S5). */
  leave: (id: string) => LeaveProps;
}) {
  // A batch folds to its band (the board's chevron); a lone change has nothing to fold.
  const [folded, setFolded] = useState<ReadonlySet<string>>(new Set());
  const toggle = (key: string) =>
    setFolded((prev) => {
      const next = new Set(prev);
      if (!next.delete(key)) next.add(key);
      return next;
    });
  // A row on its way out is not one of "these N changes".
  const waiting = (group: CascadeGroup) => group.rows.filter((row) => !leave(row.id).leaving).length;
  return (
    <>
      {groups.map((group) => {
        const read = whole?.find((entry) => entry.key === group.key);
        const all = read ?? group;
        // What the read still has of this edit; a batch that has left it is counting down what is going.
        const count = read ? read.rows.length : waiting(all);
        const shown = waiting(group);
        const open = count < 2 || !folded.has(group.key);
        // One reason for the whole batch is said once, in its band, not on every row.
        const shared = count > 1 ? sharedWait(all.rows) : undefined;
        return (
          <div key={group.key}>
            {count > 1 && (
              <div className="row-divider flex items-center gap-3 bg-surface-0 px-5 py-2.5">
                <button
                  type="button"
                  aria-expanded={open}
                  aria-label={`${open ? "Fold" : "Open"} these ${count} changes`}
                  onClick={() => toggle(group.key)}
                  className="group -my-2 -ml-2 flex size-11 flex-none items-center justify-center"
                >
                  {/* The 28px round chevron is the look; the 44px button around it is the touch floor. */}
                  <span
                    aria-hidden
                    className={`flex size-7 items-center justify-center rounded-pill border border-line-strong text-[13px] text-muted motion-turn group-hover:bg-[var(--hover)] ${
                      open ? "rotate-90" : ""
                    }`}
                  >
                    ›
                  </span>
                </button>
                <p className="min-w-0 flex-1 text-[13.5px] leading-[1.5] text-muted">
                  These {count} changes come from one edit (
                  <Mono className="text-faint">{shortId(group.key, "c")}</Mono>).{" "}
                  {shared ? `${shared}. ` : ""}They are sent together or not at all.
                  {shown < count && ` ${shown} of ${count} shown.`}
                </p>
              </div>
            )}
            <Unfold open={open}>
              {group.rows.map((row) => (
                <LeavingRow key={row.id} {...leave(row.id)}>
                  <div className="row-divider flex min-h-[60px] flex-col items-start gap-1.5 px-5 py-3 tablet:flex-row tablet:items-center tablet:gap-[18px]">
                    <span className="w-full truncate text-[14.5px] font-semibold tablet:w-[130px] tablet:shrink-0">
                      <UserName id={row.user_id} />
                    </span>

                    <span className="flex w-full min-w-0 flex-col gap-[3px] tablet:flex-1">
                      <span className="text-[14px]">
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
                            {row.wait_reason === "role_waits" && <WaitsMark className="ml-1.5" />}
                          </>
                        )}
                      </span>
                      <RowStatus row={row} skipWait={shared} />
                    </span>

                    <span className="w-full text-[13.5px] tablet:w-[130px] tablet:shrink-0">
                      <CausedBy row={row} bundles={bundles} rules={rules} />
                    </span>

                    <span className="text-[13px] text-faint tablet:w-[110px] tablet:shrink-0 tablet:text-right">
                      <Relative iso={row.created_at} />
                    </span>
                  </div>
                </LeavingRow>
              ))}
            </Unfold>
          </div>
        );
      })}
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

function waitLine(row: PendingPropagationRow): string | undefined {
  const role = (row.role_keys ?? []).map(humanizeKey).join(", ") || "It";
  return waitReasonLine(row.wait_reason, role, row.created_at);
}

/** The one reason every row of a batch gives, if they all give the same. */
function sharedWait(rows: PendingPropagationRow[]): string | undefined {
  const first = waitLine(rows[0]);
  return first && rows.every((row) => waitLine(row) === first) ? first : undefined;
}

/**
 * What state the change is in, under it: why it waits (warn only for a send
 * that stopped, not for a wait the operator chose), a failure and what the
 * system said, and that a revocation also goes on its own. `skipWait` is the
 * batch's shared reason, already said in its band.
 */
function RowStatus({ row, skipWait }: { row: PendingPropagationRow; skipWait?: string }) {
  const line = waitLine(row);
  return (
    <>
      {line &&
        line !== skipWait &&
        (waitIsChosen(row.wait_reason) ? (
          <span className="text-[12.5px] text-muted">{line}</span>
        ) : (
          <span className="inline-flex items-center gap-2 text-[13px]">
            <StatusDot tone="warn" />
            <span className="text-warn-text">{line}</span>
          </span>
        ))}
      {row.status === "failed" && (
        <span className="inline-flex items-center gap-2 text-[13px] text-danger-text">
          <StatusDot tone="danger" />
          <span className="[overflow-wrap:anywhere]">
            Failed, and Syndra will try again
            {row.last_error ? `: ${row.last_error}` : "."}
          </span>
        </span>
      )}
      {row.op_type === "revoke" && (
        <span className="text-[12.5px] text-faint">
          A revocation: it also sends on its own within a few minutes.
        </span>
      )}
    </>
  );
}

/** Search by person or role: the name as the operator reads it, never the id. */
function matches(
  row: PendingPropagationRow,
  needle: string,
  resolver: ReturnType<typeof useNameResolver>,
): boolean {
  const person = resolver.resolveUser(row.user_id).value;
  const project = resolver.resolveProject(row.project_id).value;
  return [
    person?.display_name,
    person?.email,
    project?.name,
    ...(row.role_keys ?? []).flatMap((key) => [key, humanizeKey(key)]),
  ].some((text) => text?.toLowerCase().includes(needle));
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
    <span className="flex flex-col gap-0.5">
      <span>{name}</span>
      {(isBundle || isRule) && row.source_ref && (
        <Mono className="text-faint">{shortId(row.source_ref, isBundle ? "b" : "R")}</Mono>
      )}
      {/* A change made by hand has no edit behind it: no handle, not a dash. */}
      {row.cascade_id && (
        <Link
          href={`/operations/cascades?cascade=${encodeURIComponent(row.cascade_id)}`}
          aria-label="See what this change set off, in Change history"
          className="link-hover"
        >
          <Mono className="text-faint">{shortId(row.cascade_id, "c")}</Mono>
        </Link>
      )}
    </span>
  );
}

function verb(opType: string): string {
  if (opType === "revoke") return "revoke";
  if (opType === "replace") return "replace";
  return "give";
}
