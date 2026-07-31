"use client";

import { useState } from "react";

import { EmptyState, ListStates, RowSkeleton } from "@/components/states";
import { Mono } from "@/components/ui/Badge";
import { Card, CardHeader } from "@/components/ui/Card";
import { Input } from "@/components/ui/Input";
import { PageHeader } from "@/components/ui/PageHeader";
import { UserName } from "@/components/names";
import { useAuditEntries } from "@/lib/queries/useAudit";
import { useDebounce } from "@/lib/useDebounce";
import { Relative } from "@/components/ui/Time";

/**
 * S8 · Review › Audit. Who did what, when. A historical record you consult —
 * which is exactly why expiring access, which is work with a deadline, is a
 * separate destination rather than a tab in here.
 */
export default function AuditPage() {
  const [actor, setActor] = useState("");
  const debounced = useDebounce(actor, 250).trim().toLowerCase();
  // The endpoint takes a limit and nothing else, so the filter narrows the
  // window that was fetched rather than the query. The header says which,
  // because a filter that silently searches only part of the log is how
  // somebody concludes an action never happened.
  const entries = useAuditEntries({ limit: 200 });

  const all = entries.data ?? [];
  const rows = debounced
    ? all.filter((entry) =>
        [entry.actor_id, entry.target_id, entry.action, entry.resource_id]
          .join(" ")
          .toLowerCase()
          .includes(debounced),
      )
    : all;

  return (
    <div className="flex flex-col gap-[18px]">
      <PageHeader
        title="Audit"
        meta="Every mutation MkAuth made, and who asked for it. Showing the most recent 200."
        actions={
          <Input
            value={actor}
            onChange={(event) => setActor(event.target.value)}
            placeholder="Filter these 200 entries"
            aria-label="Filter the loaded audit entries"
            className="w-[280px]"
          />
        }
      />

      <Card>
        <CardHeader title="Recorded" count={rows.length} />
        <ListStates
          isLoading={entries.isLoading}
          error={entries.error}
          isEmpty={rows.length === 0}
          onRetry={() => entries.refetch()}
          errorTitle="Couldn't load the audit log."
          skeleton={<RowSkeleton rows={6} avatar={false} label="Loading audit entries" />}
          empty={
            <EmptyState
              title={debounced ? "Nothing matches in the last 200 entries." : "Nothing recorded yet."}
              guidance={
                debounced
                  ? "Older entries are not loaded — this filter searches what is on screen."
                  : "Grants, revokes and policy changes are written here as they happen."
              }
            />
          }
        >
          {rows.map((entry) => (
            <div key={entry.id} className="row-divider flex flex-wrap items-center gap-4 px-5 py-3">
              <div className="w-[190px] shrink-0 truncate text-[14.5px] font-semibold">
                <UserName id={entry.actor_id} />
              </div>
              <Mono className="w-[220px] shrink-0 truncate text-muted">{entry.action}</Mono>
              <div className="min-w-0 flex-1 truncate text-[14px] text-muted">
                <UserName id={entry.target_id} />
              </div>
              <Mono className="w-[220px] shrink-0 truncate text-faint">{entry.resource_id}</Mono>
              <div className="w-[110px] shrink-0 text-right text-[13px] text-faint">
                <Relative iso={entry.created_at} />
              </div>
            </div>
          ))}
        </ListStates>
      </Card>
    </div>
  );
}
