"use client";

import { useRouter, useSearchParams } from "next/navigation";
import { useMemo, useState } from "react";

import { EmptyState, ListStates, RowSkeleton } from "@/components/states";
import { Mono } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Card, CardColumns } from "@/components/ui/Card";
import { Input } from "@/components/ui/Input";
import { PageHeader } from "@/components/ui/PageHeader";
import { Select } from "@/components/ui/Select";
import { UserName } from "@/components/names";
import { TraceCell } from "@/components/audit/TraceCell";
import { describeAction, machineName } from "@/lib/audit-vocabulary";
import { useAuditPages, type AuditEntry } from "@/lib/queries/useAudit";
import { useNameResolver } from "@/lib/queries/useNameResolver";
import { useDebounce } from "@/lib/useDebounce";
import { formatShortDate } from "@/lib/format";

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

  return (
    <div className="flex flex-col gap-[18px]">
      <PageHeader
        title="Audit"
        meta={
          scopedUser
            ? "Everything this person did, and everything done to them. Filtered at the source, so nothing is missing from the window."
            : `Every mutation Syndra made, and who asked for it. ${all.length} loaded${
                entries.hasNextPage ? ", more further back" : " — that is the whole log"
              } · the filters below narrow what is loaded.`
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
              placeholder="Any actor"
              aria-label="Filter by actor"
              className="w-[220px]"
            />
            <Button onClick={() => downloadCsv(rows, resolver)} disabled={rows.length === 0}>
              Export CSV
            </Button>
          </>
        }
      />

      {scopedUser && (
        // A scoped log must always show a way back out, or an operator who
        // arrived by link has no way to tell they are not looking at everything.
        <div className="flex items-center gap-2.5 self-start rounded-pill bg-tint-2 py-1.5 pl-4 pr-2.5 text-[13.5px]">
          <span>
            Scoped to <UserName id={scopedUser} />
          </span>
          <button
            type="button"
            onClick={() => router.replace("/audit", { scroll: false })}
            aria-label="Show the whole audit log"
            className="rounded-pill px-2 py-0.5 font-semibold text-muted motion-tint hover:text-ink"
          >
            ✕
          </button>
        </div>
      )}

      <Card>
        <CardColumns>
          <span className="w-[110px]">When</span>
          <span className="w-[150px]">Who</span>
          <span className="flex-1">What they did</span>
          <span className="w-[80px] text-right">Trace</span>
        </CardColumns>

        <ListStates
          isLoading={entries.isLoading}
          error={entries.error}
          isEmpty={rows.length === 0}
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
                    ? "These filters search what is loaded. Load more to search further back."
                    : "The whole log is loaded — nothing in it matches."
                  : "Grants, revokes and policy changes are written here as they happen."
              }
              action={
                entries.hasNextPage
                  ? { label: "Load more", onClick: () => entries.fetchNextPage() }
                  : undefined
              }
            />
          }
        >
          {rows.map((entry) => (
            <div
              key={entry.id}
              className="row-divider flex flex-wrap items-baseline gap-4 px-5 py-3"
            >
              <Mono className="w-[110px] shrink-0 text-faint">
                {formatShortDate(entry.created_at)}
              </Mono>
              <span className="w-[150px] shrink-0 truncate text-[14.5px] font-semibold">
                <UserName id={entry.actor_id} fallback={machineName(entry.actor_id)} />
              </span>
              <span className="min-w-[240px] flex-1 text-[14px] text-muted">
                <Sentence entry={entry} />
              </span>
              <TraceCell entry={entry} className="w-[80px] shrink-0 text-right" />
            </div>
          ))}

          {/*
            The end of the log is stated, not implied by a button that stops
            appearing. "Nothing older" and "there is more but you have to ask"
            are different facts about a record you are consulting to answer a
            question, and only one of them means you can stop looking.
          */}
          <div className="row-divider flex items-center gap-4 px-5 py-3.5">
            {entries.hasNextPage ? (
              <>
                <Button
                  size="sm"
                  isPending={entries.isFetchingNextPage}
                  onClick={() => entries.fetchNextPage()}
                >
                  Load more
                </Button>
                <span className="text-[13px] text-faint">
                  {all.length} loaded — older entries are further back
                </span>
              </>
            ) : (
              <span className="text-[13px] text-faint">
                That is the whole log — {all.length}{" "}
                {all.length === 1 ? "entry" : "entries"}, nothing older.
              </span>
            )}
          </div>
        </ListStates>
      </Card>
    </div>
  );
}

/**
 * The verb, in words, with only the destructive one carrying colour. Unknown
 * actions fall back to the raw key rather than a guess — a log that invents a
 * description for something it doesn't recognise is worse than one that admits
 * it.
 */
function Sentence({ entry }: { entry: AuditEntry }) {
  const { verb, destructive } = describeAction(entry.action);
  const hasTarget = entry.target_id && entry.target_id !== "-" && entry.target_id !== "system";

  return (
    <>
      <span className={destructive ? "font-semibold text-danger-text" : undefined}>{verb}</span>
      {hasTarget ? (
        <>
          {" — "}
          <UserName id={entry.target_id} />
        </>
      ) : null}
    </>
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
  const lines = rows.map((entry) => [
    entry.created_at,
    resolver.resolveUser(entry.actor_id).value?.display_name ?? entry.actor_id,
    describeAction(entry.action).verb,
    resolver.resolveUser(entry.target_id).value?.display_name ?? entry.target_id,
    entry.resource_id,
  ]);
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
