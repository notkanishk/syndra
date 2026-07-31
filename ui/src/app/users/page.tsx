"use client";

import Link from "next/link";
import { useState } from "react";

import { ListStates, EmptyState, RowSkeleton } from "@/components/states";
import { Avatar } from "@/components/ui/Avatar";
import { Card, CardColumns } from "@/components/ui/Card";
import { Input } from "@/components/ui/Input";
import { PageHeader } from "@/components/ui/PageHeader";
import { useUsers } from "@/lib/queries/useUsers";
import { useDebounce } from "@/lib/useDebounce";

/**
 * People — the index. Its whole job is to get an operator to one person's
 * page, which is where the real work happens.
 */
export default function PeoplePage() {
  const [query, setQuery] = useState("");
  const debounced = useDebounce(query, 250);
  const users = useUsers(debounced);

  const rows = users.data ?? [];

  return (
    <div className="flex flex-col gap-[18px]">
      <PageHeader
        title="People"
        meta={
          rows.length > 0
            ? `${rows.length} ${rows.length === 1 ? "person" : "people"}`
            : undefined
        }
        actions={
          <Input
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            placeholder="Search by name, email or team"
            aria-label="Search people"
            className="w-[320px]"
          />
        }
      />

      <Card>
        <CardColumns>
          <span className="flex-1">Person</span>
          <span className="w-[190px]">Team</span>
          <span className="w-[120px] text-right">Bundles</span>
          <span className="w-[120px] text-right">Roles</span>
        </CardColumns>

        <ListStates
          isLoading={users.isLoading}
          error={users.error}
          isEmpty={rows.length === 0}
          onRetry={() => users.refetch()}
          errorTitle="Couldn't load people."
          skeleton={<RowSkeleton rows={6} label="Loading people" />}
          empty={
            <EmptyState
              title={debounced ? "Nobody matches that." : "There's nobody here yet."}
              guidance={
                debounced
                  ? "Try a shorter search, or part of an email address."
                  : "People appear here once they exist in the identity provider."
              }
            />
          }
        >
          {rows.map((entry) => (
            <Link
              key={entry.user.id}
              href={`/users/${entry.user.id}`}
              className="row-divider flex items-center gap-[18px] px-5 py-3.5 transition-colors hover:bg-[var(--hover)]"
            >
              <Avatar name={entry.user.name} />
              <div className="min-w-0 flex-1">
                <div className="truncate text-[15px] font-semibold">{entry.user.name}</div>
                <div className="truncate text-[13.5px] text-faint">{entry.user.email}</div>
              </div>
              <div className="w-[190px] truncate text-[14px] text-muted">
                {entry.user.title || entry.user.team || "—"}
              </div>
              <div className="w-[120px] text-right text-[15px]">{entry.bundle_count}</div>
              <div className="w-[120px] text-right text-[15px]">{entry.effective_role_count}</div>
            </Link>
          ))}
        </ListStates>
      </Card>
    </div>
  );
}
