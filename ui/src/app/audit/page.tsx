"use client";

import { useRouter, useSearchParams } from "next/navigation";
import { useMemo, useState } from "react";

import { EmptyState, ListStates, RowSkeleton } from "@/components/states";
import { Mono } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { FilterChip } from "@/components/ui/FilterChip";
import { Card, CardColumns } from "@/components/ui/Card";
import { Input } from "@/components/ui/Input";
import { LeavingRow, useLeavingRows } from "@/components/ui/LeavingRows";
import { PageHeader } from "@/components/ui/PageHeader";
import { PaginationRow, usePaging } from "@/components/ui/PaginationRow";
import { Select } from "@/components/ui/Select";
import { UserName } from "@/components/names";
import { EventSentence } from "@/components/audit/EventLine";
import { TraceCell } from "@/components/audit/TraceCell";
import { describeAction, machineName, targetKind, shortId } from "@/lib/audit-vocabulary";
import { useAuditPages, type AuditEntry } from "@/lib/queries/useAudit";
import { useNameResolver } from "@/lib/queries/useNameResolver";
import { useDebounce } from "@/lib/useDebounce";
import { formatShortDate, formatWhen } from "@/lib/format";

type Window = "7" | "30" | "all";

/**
 * S8 · Review › Audit. Who did what, when.
 *
 * A record you consult — which is exactly why expiring access, which is work
 * with a deadline, is a separate destination rather than a tab in here.
 *
 * Every line names a human or a NAMED machine, and reads as a sentence rather
 * than a verb key: "Approved request — Ike Nwosu, Laser Lab / operator" is
 * something an operator can scan; `request.approve` is something they have to
 * decode. Colour marks the exception only — a destructive verb takes the
 * danger tone on the word itself, and nothing else on the row is coloured.
 */
export default function AuditPage() {
  const params = useSearchParams();
  const router = useRouter();
  // `?user=` scopes the whole log to one person's involvement — actor OR
  // target — and it does so server-side. That distinction matters: the text
  // filter below narrows the rows already loaded, while this narrows the
  // query, so a person's trail is complete rather than "complete within
  // whatever happened to be fetched".
  const scopedUser = params.get("user") ?? "";

  const [actor, setActor] = useState("");
  const [window, setWindow] = useState<Window>(scopedUser ? "all" : "7");
  const debounced = useDebounce(actor, 250).trim().toLowerCase();
  const resolver = useNameResolver();

  const entries = useAuditPages({ limit: 100, userId: scopedUser || undefined });

  const all = useMemo(
    () => (entries.data?.pages ?? []).flat(),
    [entries.data],
  );

  const rows = useMemo(() => {
    const cutoff =
      window === "all" ? 0 : Date.now() - Number(window) * 24 * 60 * 60 * 1000;
    return all.filter((entry) => {
      if (cutoff && new Date(entry.created_at).getTime() < cutoff) return false;
      if (!debounced) return true;
      const actorName = resolver.resolveUser(entry.actor_id).value?.display_name ?? "";
      const targetName = resolver.resolveUser(entry.target_id).value?.display_name ?? "";
      return [entry.actor_id, entry.target_id, entry.action, entry.resource_id, actorName, targetName]
        .join(" ")
        .toLowerCase()
        .includes(debounced);
    });
  }, [all, debounced, window, resolver]);

  // One row of paging for a list the server hands over a page at a time: it
  // knows how many it has, never how many there are (A-04 section 7.4).
  const paging = usePaging();
  const visible = rows.slice(0, paging.limit);
  // A date range or a search drops entries: they fade where they were and the gap settles after
  // (A-05 §2.10). A log is append-only, so nothing else ever leaves it.
  const leaving = useLeavingRows(rows, (entry) => entry.id, !entries.isLoading && !entries.error, {
    narrowedBy: `${window}|${debounced}|${scopedUser}`,
  });
  const drawn = leaving.rows.slice(0, paging.limit);
  const cutoff = window === "all" ? null : Date.now() - Number(window) * 24 * 60 * 60 * 1000;
  // A window needs nothing older than itself, so once the loaded tail is past it
  // there is no more to ask for.
  const oldest = all[all.length - 1]?.created_at;
  const passedWindow = cutoff !== null && oldest !== undefined && new Date(oldest).getTime() < cutoff;
  const hasMore = rows.length > paging.limit || (Boolean(entries.hasNextPage) && !passedWindow);
  const showMore = () => {
    paging.more();
    if (rows.length < paging.limit + paging.step && entries.hasNextPage && !passedWindow) entries.fetchNextPage();
  };
  const since = cutoff !== null ? formatShortDate(new Date(cutoff).toISOString()) : oldest ? formatShortDate(oldest) : null;

  return (
    <div className="flex flex-col gap-[18px]">
      <PageHeader
        title="Audit"
        lede={
          scopedUser
            ? "Everything this person did, and everything done to them, searched across the whole log rather than only what is loaded."
            : "What people did in Syndra, newest first. The filters narrow what is loaded."
        }
        meta={
          scopedUser
            ? undefined
            : `${all.length} loaded${entries.hasNextPage ? ", more further back" : " — the whole log"}`
        }
        actions={
          <>
            <Select
              value={window}
              onChange={(event) => setWindow(event.target.value as Window)}
              aria-label="Date range"
              className="w-[150px]"
            >
              <option value="7">Last 7 days</option>
              <option value="30">Last 30 days</option>
              <option value="all">Everything loaded</option>
            </Select>
            <Input
              value={actor}
              onChange={(event) => setActor(event.target.value)}
              placeholder="Search names or actions"
              aria-label="Search the loaded entries"
              className="w-[220px]"
            />
            <Button onClick={() => downloadCsv(rows, resolver)} disabled={rows.length === 0}>
              Export what is shown (CSV)
            </Button>
          </>
        }
      />

      {scopedUser && (
        // A scoped log must always show a way back out, or an operator who
        // arrived by link has no way to tell they are not looking at everything.
        <FilterChip
          onClear={() => router.replace("/audit", { scroll: false })}
          clearLabel="Show the whole audit log"
        >
          Showing only <UserName id={scopedUser} />
        </FilterChip>
      )}

      <p className="text-[13px] text-faint">
        Follow-on: a c_ handle links to what an edit set off, in Change history. R_ names an
        automatic rule, b_ a bundle.
      </p>

      <Card>
        <CardColumns>
          <span className="w-[110px]">When</span>
          <span className="w-[150px]">Who</span>
          <span className="flex-1">What they did</span>
          <span className="w-[80px] text-right">Follow-on</span>
        </CardColumns>

        <ListStates
          isLoading={entries.isLoading}
          error={entries.error}
          isEmpty={drawn.length === 0}
          onRetry={() => entries.refetch()}
          errorTitle="Couldn't load the audit log."
          skeleton={<RowSkeleton rows={6} avatar={false} label="Loading audit entries" />}
          empty={
            <EmptyState
              title={
                debounced || window !== "all"
                  ? "Nothing matches in the entries that are loaded."
                  : "Nothing recorded yet."
              }
              guidance={
                debounced || window !== "all"
                  ? entries.hasNextPage
                    ? "These filters search what is loaded. Show more to search further back."
                    : "The whole log is loaded — nothing in it matches."
                  : "Access given, access revoked, and rule changes are written here as they happen."
              }
              action={
                entries.hasNextPage
                  ? { label: `Show ${paging.step} more`, onClick: showMore }
                  : undefined
              }
            />
          }
        >
          {drawn.map((entry) => (
            <LeavingRow key={entry.id} {...leaving.props(entry.id)}>
            <div
              className="row-divider flex min-h-[60px] flex-col items-start gap-1.5 px-5 py-3 tablet:flex-row tablet:flex-wrap tablet:items-baseline tablet:gap-4"
            >
              <Mono className="text-faint tablet:w-[124px] tablet:shrink-0">
                {formatWhen(entry.created_at)}
              </Mono>
              <span className="w-full truncate text-[14.5px] font-semibold tablet:w-[150px] tablet:shrink-0">
                <UserName id={entry.actor_id} fallback={machineName(entry.actor_id)} />
              </span>
              <span className="w-full text-[14px] text-muted tablet:min-w-[240px] tablet:flex-1">
                <EventSentence entry={entry} />
              </span>
              <TraceCell entry={entry} className="tablet:w-[80px] tablet:shrink-0 tablet:text-right" />
            </div>
            </LeavingRow>
          ))}

          {/* The end of the log is stated, not implied by a button that stops
              appearing. "Nothing older" and "there is more but you have to ask"
              are different facts about a record consulted to answer a question,
              and only one of them means you can stop looking. */}
          <PaginationRow
            shown={visible.length}
            hasMore={hasMore}
            since={since}
            pending={entries.isFetchingNextPage}
            onMore={showMore}
            onBack={paging.back}
          />
        </ListStates>
      </Card>
    </div>
  );
}

/**
 * Export what is on screen, with the names resolved — a CSV full of UUIDs is a
 * file somebody has to come back and ask about.
 */
function downloadCsv(
  rows: AuditEntry[],
  resolver: ReturnType<typeof useNameResolver>,
): void {
  const header = ["when", "who", "what", "target", "resource"];
  const lines = rows.map((entry) => {
    const kind = targetKind(entry.action);
    const target =
      kind === "bundle"
        ? (resolver.resolveBundle(entry.target_id).value?.name ??
          `a retired bundle (${shortId(entry.target_id, "b")})`)
        : kind === "rule"
          ? `a rule (${shortId(entry.target_id, "R")})`
          : (resolver.resolveUser(entry.target_id).value?.display_name ?? entry.target_id);
    return [
      entry.created_at,
      resolver.resolveUser(entry.actor_id).value?.display_name ?? entry.actor_id,
      describeAction(entry.action).verb,
      target,
      entry.resource_id,
    ];
  });
  const csv = [header, ...lines]
    .map((cells) => cells.map((cell) => `"${String(cell ?? "").replace(/"/g, '""')}"`).join(","))
    .join("\n");

  const url = URL.createObjectURL(new Blob([csv], { type: "text/csv;charset=utf-8" }));
  const link = document.createElement("a");
  link.href = url;
  link.download = `syndra-audit-${new Date().toISOString().slice(0, 10)}.csv`;
  link.click();
  URL.revokeObjectURL(url);
}
