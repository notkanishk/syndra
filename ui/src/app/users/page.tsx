"use client";

import Link from "next/link";
import { useMemo, useState } from "react";

import { ListStates, EmptyState, RowSkeleton } from "@/components/states";
import { Avatar } from "@/components/ui/Avatar";
import { Card, CardColumns } from "@/components/ui/Card";
import { Input } from "@/components/ui/Input";
import { PageHeader } from "@/components/ui/PageHeader";
import { Select } from "@/components/ui/Select";
import { useUsers, type UserListEntry } from "@/lib/queries/useUsers";
import { useDebounce } from "@/lib/useDebounce";
import { daysUntil } from "@/lib/format";

/** Explicit pagination, never infinite scroll: a queue you can reach the end of. */
const PAGE = 50;

/**
 * People — the index.
 *
 * The last column is the reason this list exists rather than being a plain
 * directory: it carries the one thing about this person that might need you.
 * Everything else on the row is context for recognising the right human
 * quickly. Take that column away and this is a phone book.
 */
export default function PeoplePage() {
  const [query, setQuery] = useState("");
  const [project, setProject] = useState("");
  const [limit, setLimit] = useState(PAGE);
  const debounced = useDebounce(query, 250);
  const users = useUsers(debounced);

  const all = useMemo(() => users.data ?? [], [users.data]);

  // Project filter options come from the rows themselves — every project
  // anybody actually holds a role in, which is the only set worth filtering by.
  const projects = useMemo(
    () => Array.from(new Set(all.flatMap((entry) => entry.key_projects))).sort(),
    [all],
  );

  const rows = project ? all.filter((entry) => entry.key_projects.includes(project)) : all;
  const visible = rows.slice(0, limit);
  const expiringSoon = all.filter((entry) => entry.expiring_count > 0).length;

  return (
    <div className="flex flex-col gap-[18px]">
      <PageHeader
        title="People"
        meta={
          all.length > 0
            ? `${all.length} ${all.length === 1 ? "account" : "accounts"}${
                expiringSoon > 0
                  ? ` · ${expiringSoon} with access expiring inside 30 days`
                  : ""
              }`
            : undefined
        }
        actions={
          <>
            <Input
              value={query}
              onChange={(event) => {
                setQuery(event.target.value);
                setLimit(PAGE);
              }}
              // Role keys are searchable here because "who has `trained` in the
              // laser lab" gets typed on this page before anyone thinks to go
              // to Roles. The placeholder has to say so or nobody tries it.
              placeholder="Search name, email or role key…"
              aria-label="Search people by name, email or role key"
              className="min-w-[260px]"
            />
            <Select
              value={project}
              onChange={(event) => {
                setProject(event.target.value);
                setLimit(PAGE);
              }}
              aria-label="Filter by project"
              className="w-[180px]"
            >
              <option value="">Any project</option>
              {projects.map((name) => (
                <option key={name} value={name}>
                  {name}
                </option>
              ))}
            </Select>
          </>
        }
      />

      <Card>
        <CardColumns>
          <span className="w-[250px]">Person</span>
          <span className="w-[150px]">Team</span>
          <span className="w-[220px]">Bundles</span>
          <span className="flex-1">Access</span>
          <span className="w-[150px] text-right">Needs attention</span>
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
              title={
                debounced || project ? "Nobody matches that." : "There's nobody here yet."
              }
              guidance={
                debounced || project
                  ? "Try a shorter search, part of an email address, or a role key."
                  : "People appear here once they exist in the identity provider."
              }
              action={
                debounced || project
                  ? {
                      label: "Clear the search",
                      onClick: () => {
                        setQuery("");
                        setProject("");
                      },
                    }
                  : undefined
              }
            />
          }
        >
          {visible.map((entry) => (
            <PersonRow key={entry.user.id} entry={entry} />
          ))}

          {rows.length > visible.length && (
            <div className="row-divider flex items-center gap-4 px-5 py-3.5">
              <span className="text-[13.5px] text-faint">
                {rows.length - visible.length} more
              </span>
              <button
                type="button"
                onClick={() => setLimit((current) => current + PAGE)}
                className="rounded-pill border border-line-strong px-4 py-1.5 text-[13.5px] font-semibold transition-colors hover:bg-[var(--hover)]"
              >
                Load next {Math.min(PAGE, rows.length - visible.length)}
              </button>
            </div>
          )}
        </ListStates>
      </Card>
    </div>
  );
}

function PersonRow({ entry }: { entry: UserListEntry }) {
  // A departed account still belongs in the list — it is often exactly who you
  // came looking for — but it reads at reduced contrast so a live person is
  // never mistaken for one who left.
  const departed = isDeparted(entry.user.status);

  return (
    <Link
      href={`/users/${entry.user.id}`}
      className={`row-divider flex items-center gap-[18px] px-5 py-3.5 transition-colors hover:bg-[var(--hover)] ${
        departed ? "opacity-60" : ""
      }`}
    >
      <span className="flex w-[250px] min-w-0 items-center gap-3">
        <Avatar name={entry.user.name} />
        <span className="min-w-0">
          <span className="block truncate text-[15px] font-semibold">{entry.user.name}</span>
          <span className="block truncate text-[12.5px] text-faint">
            {entry.user.title || entry.user.email}
          </span>
        </span>
      </span>

      <span className="w-[150px] truncate text-[14px] text-muted">{entry.user.team || "—"}</span>

      <span className="flex w-[220px] flex-wrap gap-1.5">
        {entry.bundle_names?.length ? (
          entry.bundle_names.map((name) => (
            <span key={name} className="rounded-pill bg-tint-2 px-2.5 py-1 text-[12.5px]">
              {name}
            </span>
          ))
        ) : (
          <span className="text-[13px] text-faint">none</span>
        )}
      </span>

      <span className="min-w-0 flex-1 truncate text-[14px] text-muted">
        {describeAccess(entry)}
      </span>

      <span className="w-[150px] text-right">
        <NeedsAttention entry={entry} />
      </span>
    </Link>
  );
}

/**
 * One line, in the semantic colour it belongs to. Deliberately singular: if
 * three things need you the row names the most urgent, because a cell holding
 * three coloured phrases is a cell nobody reads.
 */
function NeedsAttention({ entry }: { entry: UserListEntry }) {
  if (entry.unexplained_count > 0) {
    return (
      <span className="text-[13.5px] font-semibold text-danger-text">
        {entry.unexplained_count} unexplained
      </span>
    );
  }
  if (entry.expiring_count > 0) {
    const days = daysUntil(entry.soonest_expiry);
    return (
      <span className="text-[13.5px] font-semibold text-warn-text">
        {entry.expiring_count} expires{" "}
        {days === null ? "soon" : days <= 0 ? "today" : `in ${days} day${days === 1 ? "" : "s"}`}
      </span>
    );
  }
  if (entry.open_request_count > 0) {
    return (
      <span className="text-[13.5px] font-semibold text-accent-text">
        {entry.open_request_count} open request{entry.open_request_count === 1 ? "" : "s"}
      </span>
    );
  }
  return <span className="text-[13.5px] text-faint">—</span>;
}

function describeAccess(entry: UserListEntry): string {
  if (entry.effective_role_count === 0) return "No roles yet";
  const roles = `${entry.effective_role_count} ${entry.effective_role_count === 1 ? "role" : "roles"}`;
  if (!entry.project_count) return roles;
  return `${roles} across ${entry.project_count} ${entry.project_count === 1 ? "project" : "projects"}`;
}

function isDeparted(status: string | undefined): boolean {
  const value = (status ?? "").toLowerCase();
  return value === "departed" || value === "inactive" || value === "alumni";
}
