"use client";

import { useSearchParams } from "next/navigation";
import { useEffect, useMemo, useState } from "react";
import { LeavingRow, useLeavingRows, type LeaveProps } from "@/lib/motion";
import { Unfold } from "@/lib/motion";
import { formatClock } from "@/lib/format";
import { useQueueCleared } from "@/lib/useQueueCleared";

import { EmptyState, ListStates, RowSkeleton } from "@/components/states";
import { ActionOutcome } from "@/components/ui/ActionOutcome";
import { humanizeKey } from "@/lib/format";
import { StatusDot } from "@/components/ui/Badge";
import { ArrowLink } from "@/components/ui/ArrowLink";
import { FoldChevron } from "@/components/ui/FoldChevron";
import { ReadFreshness, readStateOfQuery } from "@/components/ui/ReadFreshness";
import { Button } from "@/components/ui/Button";
import { Card, CardColumns, CardHeader, CardRow } from "@/components/ui/Card";
import { FilterBar } from "@/components/ui/FilterBar";
import { FilterChip } from "@/components/ui/FilterChip";
import { Input } from "@/components/ui/Input";
import { WaitsMark } from "@/components/ui/WaitsMark";
import { RetryStatus } from "@/components/ui/RetryStatus";
import { narrow, usePendingFilters } from "@/lib/pending-filter";
import { useNameResolver } from "@/lib/queries/useNameResolver";
import { PageHeader } from "@/components/ui/PageHeader";
import { RoleInPlace, RoleRef, UserName } from "@/components/names";
import { machineName } from "@/lib/audit-vocabulary";
import { targetLabel } from "@/lib/nav";
import { outcomeFromDrain } from "@/lib/drain-outcome";
import { outcomeFromError, statesNothingChanged, type ActionOutcome as Outcome } from "@/lib/outcome";
import { useGovernanceSummary } from "@/lib/queries/useGovernance";
import { useAutoRetry } from "@/lib/queries/useIndicators";
import { useBundles, type BundleRow } from "@/lib/queries/useBundles";
import { useMappingRules, type MappingRuleRow } from "@/lib/queries/useMappingRules";
import {
  useDrainPropagations,
  useGaveUpPropagations,
  usePendingPropagations,
  type PendingPropagationRow,
} from "@/lib/queries/usePropagation";
import { Relative } from "@/components/ui/Time";
import { isRefused, refusalCause, waitIsChosen, waitReasonLine } from "@/lib/wait-reason";

/** Q2 (A-17): an empty queue still says "Send 0 changes". The owner's answer is a change to this one function. */
const sendLabel = (known: boolean, n: number) => (known && n > 0 ? `Send ${n} ${n === 1 ? "change" : "changes"}` : "Send changes");

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
  const auto = useAutoRetry();
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
  // Seeded from `?q=`, so "Waiting for you to send →" on a person's page opens
  // this list narrowed to them (A-10 §3).
  const params = useSearchParams();
  const [query, setQuery] = useState(() => params?.get("q") ?? "");
  const needle = query.trim().toLowerCase();
  // "Waiting for you to send →" lands here narrowed to a person or a role
  // (A-19 P3, A-20 P16): one model, one in-force chip each, cleared by its own ✕.
  const { inForce, key: filterKey, active: filtered, clear } = usePendingFilters(params);
  // `?gave_up=1` (Home's "See them →"): only what Syndra stopped sending.
  const gaveUp = useGaveUpPropagations();
  const [onlyStopped, setOnlyStoppedState] = useState(() => params?.get("gave_up") === "1");
  const setOnlyStopped = (on: boolean) => {
    setOnlyStoppedState(on);
    const url = new URL(window.location.href);
    if (on) url.searchParams.set("gave_up", "1");
    else url.searchParams.delete("gave_up");
    window.history.replaceState(window.history.state, "", url);
  };
  const { person, role } = inForce;
  const matched = useMemo(
    () => narrow(rows, inForce).filter((row) => !needle || matches(row, needle, resolver)),
    // inForce is rebuilt each render; filterKey is its identity.
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [rows, filterKey, needle, resolver],
  );
  // S5: a row that goes on a later read (cancelled by someone, sent by someone)
  // fades where it was and closes its gap after, and not under the pointer. A row
  // a search drops goes the same way, but its gap does not wait for the pointer
  // (A-05 §2.10), and the rows that still match are the same rows.
  const leaving = useLeavingRows(matched, (row) => row.id, !pending.isLoading && !pending.error, {
    narrowedBy: needle + filterKey,
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
    <div className="flex flex-col gap-4">
      <PageHeader
        title="Pending changes"
        lede={`Changes that have not reached Zitadel or a connected system yet. Sending delivers all of them, in order, now.${auto === null ? "" : auto ? " Revocations, and changes Zitadel didn't take when they were sent, also go again on their own; a role that waits for you never does." : " This Syndra server sends nothing again on its own, so each waits for Send."}`}
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
                    setOutcome(outcomeFromDrain(await drain.mutateAsync(), auto));
                  } catch (error) {
                    failedSend();
                    setOutcome(outcomeFromError(error));
                  }
                }}
              >
                {sendLabel(known, sentCount)}
              </Button>
            </span>
          )
        }
      />

      {outcome && (
        <ActionOutcome
          outcome={outcome}
          // "Sent" only when every change went: a pass that left some to be
          // sent again is not one (W2).
          word={statesNothingChanged(outcome.kind) ? "Not sent" : outcome.kind === "queued" ? "Not all sent" : "Sent"}
        />
      )}

      {known && rows.length > 0 && !cleared && (
        <FilterBar
          search={
            <Input
              value={query}
              onChange={(event) => setQuery(event.target.value)}
              placeholder="Search person or role…"
              aria-label="Search pending changes by person or role"
            />
          }
          read={
            <div className="flex flex-wrap items-center gap-x-3 gap-y-1">
              <ReadFreshness state={readStateOfQuery(pending)} subject="The queue" />
              {person && (
                <FilterChip onClear={() => clear("person")} clearLabel="Show everyone's changes">
                  <UserName id={person} link={false} />
                </FilterChip>
              )}
              {role && (
                <FilterChip onClear={() => clear("role")} clearLabel="Show every role's changes">
                  <RoleRef projectId={role.projectId} roleKey={role.roleKey} />
                </FilterChip>
              )}
              {(needle || filtered) && (
                <span className="text-[13.5px] text-muted">
                  {filtered ? (
                    <>
                      Showing{" "}
                      {person ? (
                        <>
                          <UserName id={person} link={false} />&rsquo;s
                        </>
                      ) : (
                        "the"
                      )}{" "}
                      {shown.length === 1 ? "1 change" : `${shown.length} changes`}
                      {role && (
                        <>
                          {" "}
                          for <RoleRef projectId={role.projectId} roleKey={role.roleKey} />
                        </>
                      )}
                      . Sending sends all {rows.length}, in order.
                    </>
                  ) : (
                    `${shown.length} of the ${rows.length} changes match. Sending sends all ${rows.length}, in order.`
                  )}
                </span>
              )}
            </div>
          }
        />
      )}

      {onlyStopped && (
        <FilterBar
          read={
            <FilterChip onClear={() => setOnlyStopped(false)} clearLabel="Show the whole queue">
              Changes Syndra stopped sending
            </FilterChip>
          }
        />
      )}
      {onlyStopped && <StoppedSending rows={gaveUp.data ?? []} loading={gaveUp.isLoading} />}

      <Card className={onlyStopped ? "hidden" : undefined}>
        {(cleared ? !folded : known && shown.length > 0) && (
          <CardColumns>
            <span className="w-[130px]">Who</span>
            <span className="flex-1">Change</span>
            <span className="w-[130px]">Caused by</span>
            <span className="w-[110px] text-right">Waiting</span>
          </CardColumns>
        )}

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
                    title={
                      needle
                        ? `No pending change matches \u201c${query.trim()}\u201d.`
                        : person
                          ? "Nothing is waiting for you to send for this person."
                          : "Nothing is waiting for you to send for this role."
                    }
                    guidance={
                      needle
                        ? "Search by a person's name or a role."
                        : person
                          ? "Their changes have reached the system they were for."
                          : "Its changes have reached the system they were for."
                    }
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

      {!onlyStopped && (gaveUp.data?.length ?? 0) > 0 && <StoppedSending rows={gaveUp.data ?? []} loading={false} />}
    </div>
  );
}

/**
 * What Syndra stopped sending that this person has not marked seen on Home
 * (re-audit R11): out of tries, or refused by the system. Each row says why
 * and the one true step. Its record is gone, so a revocation is not revoked
 * from the person's page: still held in the system, it is raised in Drift by
 * the next check and revoked there. A give is given again from their page.
 */
function StoppedSending({ rows, loading }: { rows: PendingPropagationRow[]; loading: boolean }) {
  return (
    <Card id="stopped-sending">
      <CardHeader title="Syndra stopped sending" count={rows.length} />
      {loading ? (
        <RowSkeleton rows={2} label="Loading what Syndra stopped sending" />
      ) : rows.length === 0 ? (
        <EmptyState
          resolved
          title="Nothing here."
          guidance="Every change Syndra stopped sending has been marked seen, or there were none this week."
        />
      ) : (
        rows.map((row) => {
          const system = targetLabel(row.target ?? "zitadel");
          const outOfTries = (row.last_error ?? "").startsWith("out of retries");
          const revocation = row.op_type === "revoke";
          const zitadel = (row.target ?? "zitadel") === "zitadel";
          return (
            <CardRow key={row.id}>
              <span className="w-full text-[14.5px] font-semibold tablet:w-[130px] tablet:shrink-0">
                <UserName id={row.user_id} />
              </span>
              <span className="flex min-w-0 flex-1 flex-col gap-[3px]">
                <span className="text-[14px]">
                  {row.op_type === "apply" ? (
                    <SystemChange row={row} />
                  ) : (
                    <>
                      <span className="text-muted">{verb(row.op_type)}</span>{" "}
                      {(row.role_keys ?? []).map((key) => (
                        <RoleRef key={key} projectId={row.project_id} roleKey={key} className="mr-3" />
                      ))}
                    </>
                  )}
                </span>
                <span className="inline-flex items-center gap-2 text-[13px]">
                  <StatusDot tone="danger" />
                  <span className="text-danger-text">
                    {outOfTries
                      ? `${system} didn't answer after ${row.attempts} ${row.attempts === 1 ? "try" : "tries"}, so Syndra stopped sending it.`
                      : `${system} refused it${isRefused(row.wait_reason) ? `: ${refusalCause(row.wait_reason ?? "")}` : ""}, so Syndra stopped sending it.`}
                  </span>
                </span>
                <span className="text-[12.5px] text-faint">
                  {revocation && zitadel
                    ? `They may still hold it in ${system}. It shows in Drift after Syndra's next check; revoke it there.`
                    : revocation
                      ? `They may still hold it in ${system}. Check their account on ${system}'s page.`
                      : "Give it again from their page once the reason is fixed."}{" "}
                  Then mark it seen on Home.
                </span>
              </span>
              <span className="tablet:w-[140px] tablet:shrink-0 tablet:text-right">
                {revocation && zitadel ? (
                  <ArrowLink href="/governance/drift">Drift</ArrowLink>
                ) : revocation ? (
                  <ArrowLink href={`/system/targets/${encodeURIComponent(row.target ?? "")}`}>{`Open ${system}`}</ArrowLink>
                ) : (
                  <ArrowLink href={`/users/${encodeURIComponent(row.user_id)}`}>Their page</ArrowLink>
                )}
              </span>
            </CardRow>
          );
        })
      )}
    </Card>
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
                  className="group -my-2 -ml-2 flex size-[var(--tap)] flex-none items-center justify-center"
                >
                  <FoldChevron open={open} />
                </button>
                <p className="min-w-0 flex-1 text-[13.5px] leading-[1.5] text-muted">
                  These {count} changes come from one edit. {shared ? `${shared}. ` : ""}They are sent together or not at all.
                  {shown < count && ` ${shown} of ${count} shown.`}
                </p>
                {all.rows[0]?.cascade_id && <SetOff id={all.rows[0].cascade_id} />}
              </div>
            )}
            <Unfold open={open}>
              {group.rows.map((row) => (
                <LeavingRow key={row.id} {...leave(row.id)}>
                  <div className="row-divider flex min-h-[60px] flex-col items-start gap-1.5 px-5 py-3 tablet:flex-row tablet:items-center tablet:gap-[18px]">
                    <span className="w-full text-[14.5px] font-semibold tablet:w-[130px] tablet:shrink-0">
                      <UserName id={row.user_id} />
                    </span>

                    <span className="flex w-full min-w-0 flex-col gap-[3px] tablet:flex-1">
                      <span className="text-[14px]">
                        {row.op_type === "apply" ? (
                          <SystemChange row={row} />
                        ) : (
                          <>
                            <span className="text-muted">{verb(row.op_type)}</span>{" "}
                            {(row.role_keys ?? []).map((key) => (
                              <RoleRef key={key} projectId={row.project_id} roleKey={key} className="mr-3" />
                            ))}
                            {row.wait_reason === "role_waits" && <WaitsMark className="ml-1.5" />}
                          </>
                        )}
                      </span>
                      <RowStatus row={row} skipWait={shared} />
                      {/* The operator's words, kept with the change (task 1.8a). A
                          connected system's row says its reason as its change. */}
                      {row.op_type !== "apply" && row.reason && (
                        <span className="text-[13px] text-ink">&ldquo;{row.reason}&rdquo;</span>
                      )}
                    </span>

                    <span className="w-full text-[13.5px] tablet:w-[130px] tablet:shrink-0">
                      <CausedBy row={row} bundles={bundles} rules={rules} setOff={count < 2} />
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
 * The queue has emptied (S13). One line says so; a queue someone else sent (or
 * that was cancelled away) says it without a count or a time it cannot know.
 */
function EverythingSent({ mine }: { mine: { count: number; at: string } | null }) {
  return (
    <EmptyState
      resolved
      lift
      bloom={mine !== null}
      title="Everything has reached Zitadel."
      action={{ label: "Change history", href: "/operations/cascades" }}
      guidance={
        mine
          ? `${mine.count} ${mine.count === 1 ? "change" : "changes"} sent at ${formatClock(mine.at)}. Nothing is waiting for you.`
          : "Nothing is waiting for you to send."
      }
    />
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
 * What state the change is in, under it: a change Zitadel didn't take says
 * when Syndra sends it again and which try (RetryStatus, the one retry model);
 * otherwise why it waits (warn only for a send that stopped, not for a wait the
 * operator chose), and that a revocation also goes on its own. `skipWait` is
 * the batch's shared reason, already said in its band.
 *
 * No failed line: this list holds only pending and in-flight rows, and a
 * failed row is terminal, so nothing here could say "Syndra will try again"
 * about one truthfully (S4).
 */
function RowStatus({ row, skipWait }: { row: PendingPropagationRow; skipWait?: string }) {
  const auto = useAutoRetry();
  if (row.retry) return <RetryStatus retry={row.retry} system={targetLabel(row.target ?? "zitadel")} />;
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
      {row.op_type === "revoke" && !waitIsChosen(row.wait_reason) && auto && (
        <span className="text-[12.5px] text-faint">
          A revocation: it also sends on its own within a minute.
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

/** "What this set off": the one way to the edit's Change history, never its handle. */
function SetOff({ id }: { id: string }) {
  return <ArrowLink href={`/operations/cascades?cascade=${encodeURIComponent(id)}`}>What this set off</ArrowLink>;
}

/** What set this change off, by name. `setOff` puts the link to Change history on this row (a batch says it once, in its band). */
function CausedBy({
  row,
  bundles,
  rules,
  setOff,
}: {
  row: PendingPropagationRow;
  bundles?: BundleRow[];
  rules?: MappingRuleRow[];
  setOff: boolean;
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
  // handle: somebody chose it by hand on the person's page. A rule has no name:
  // the role that set it off is its name (A-21 §Q3), also when a bundle's
  // change set the rule off, since the row is the rule's (task 1.6). That role
  // is the one kept on the row (the hop before it in a chain, as it was when
  // written, task 1.6a); a row from before it was kept reads the rule's own.
  const from = row.rule_from ?? (rule && { project_id: rule.source_project, role_key: rule.source_role });
  const name = bundle ? (
    bundle.name
  ) : rule && from ? (
    <>
      Rule from <RoleInPlace projectId={from.project_id} roleKey={from.role_key} />
    </>
  ) : isBundle
        ? "Bundle"
        : isRule
          ? "Automatic rule"
          : row.source === "direct"
            ? "Given by hand"
            : humanizeKey(row.source);

  return (
    <span className="flex flex-col gap-0.5">
      <span>{name}</span>
      {/* A change made by hand has no edit behind it: no link, not a dash. */}
      {setOff && row.cascade_id && <SetOff id={row.cascade_id} />}
    </span>
  );
}

function verb(opType: string): string {
  if (opType === "revoke") return "revoke";
  if (opType === "replace") return "replace";
  return "give";
}
