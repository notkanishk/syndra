"use client";

import { useMemo, useState } from "react";

import { EmptyState, ListStates, RowSkeleton } from "@/components/states";
import { UpstreamShell } from "@/components/upstream/UpstreamShell";
import { Mono } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Card, CardColumns, CardHeader } from "@/components/ui/Card";
import { Input } from "@/components/ui/Input";
import { ProjectName, UserName } from "@/components/names";
import { useUpstreamGrants } from "@/lib/queries/useUpstream";
import { useDebounce } from "@/lib/useDebounce";

const PAGE = 100;

/**
 * Every grant the identity provider holds.
 *
 * Read-only on purpose: a grant is edited from the person it belongs to, where
 * the consequence is visible. This page exists to answer "is it all there" —
 * the whole inventory, in one place, with an honest note when the fetch had to
 * stop short of the end.
 */
export default function UpstreamGrantsPage() {
  const grants = useUpstreamGrants();
  const [query, setQuery] = useState("");
  const [limit, setLimit] = useState(PAGE);
  const search = useDebounce(query, 200).trim().toLowerCase();

  const rows = useMemo(() => {
    const all = grants.data?.items ?? [];
    if (!search) return all;
    return all.filter((grant) =>
      [grant.userId, grant.projectId, grant.roleKeys.join(" ")]
        .join(" ")
        .toLowerCase()
        .includes(search),
    );
  }, [grants.data, search]);

  const visible = rows.slice(0, limit);

  return (
    <UpstreamShell
      title="Grants"
      lede="The complete grant inventory the identity provider holds, Syndra-caused or not."
      syndraHref="/governance/drift?tab=reconciliation"
      syndraLabel="See where the two sides disagree"
    >
      {grants.data?.truncated && (
        <div className="warn-note px-5 py-3.5 text-[14px] text-warn-text">
          This listing stopped at the safety cap, so it is incomplete. Treat an absence here as
          &ldquo;not loaded&rdquo;, never as &ldquo;does not exist&rdquo;.
        </div>
      )}

      <Card>
        <CardHeader
          title="All grants upstream"
          count={grants.data?.total ?? rows.length}
          action={
            <Input
              value={query}
              onChange={(event) => {
                setQuery(event.target.value);
                setLimit(PAGE);
              }}
              placeholder="Filter by person, project or role"
              aria-label="Filter grants"
              className="w-[280px]"
            />
          }
        />

        <CardColumns>
          <span className="w-[230px]">Person</span>
          <span className="w-[200px]">Project</span>
          <span className="flex-1">Roles</span>
          <span className="w-[190px] text-right">Grant id</span>
        </CardColumns>

        <ListStates
          isLoading={grants.isLoading}
          error={grants.error}
          isEmpty={rows.length === 0}
          onRetry={() => grants.refetch()}
          errorTitle="Couldn't read grants from the identity provider. Syndra itself is fine."
          skeleton={<RowSkeleton rows={8} avatar={false} label="Reading grants" />}
          empty={
            <EmptyState
              title={search ? "Nothing matches that." : "No grants upstream."}
              guidance={
                search
                  ? "Try a project name or a role key."
                  : "The identity provider holds no grants at all."
              }
            />
          }
        >
          {visible.map((grant) => (
            <div key={grant.id} className="row-divider flex flex-wrap items-center gap-4 px-5 py-2.5">
              <span className="w-[230px] shrink-0 truncate text-[14px] font-semibold">
                <UserName id={grant.userId} fallback={<Mono>{grant.userId}</Mono>} />
              </span>
              <span className="w-[200px] shrink-0 truncate text-[14px] text-muted">
                <ProjectName id={grant.projectId} fallback={grant.projectId} />
              </span>
              <span className="min-w-0 flex-1 truncate">
                {grant.roleKeys.map((key) => (
                  <Mono key={key} className="mr-2 text-muted">
                    {key}
                  </Mono>
                ))}
              </span>
              <Mono className="w-[190px] shrink-0 truncate text-right text-faint">{grant.id}</Mono>
            </div>
          ))}

          {rows.length > visible.length && (
            <div className="row-divider flex items-center gap-4 px-5 py-3">
              <span className="text-[13.5px] text-faint">{rows.length - visible.length} more</span>
              {/* The fourth copy of this pill. Three were found and replaced
                  by `one-control-surface`; this one was outside the sweep and
                  had already lost the touch floor. */}
              <Button size="sm" onClick={() => setLimit((current) => current + PAGE)}>
                Load next {Math.min(PAGE, rows.length - visible.length)}
              </Button>
            </div>
          )}
        </ListStates>
      </Card>
    </UpstreamShell>
  );
}
