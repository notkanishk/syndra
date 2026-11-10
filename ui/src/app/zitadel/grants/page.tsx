"use client";

import { useMemo, useState } from "react";

import { EmptyState, ListStates, RowSkeleton } from "@/components/states";
import { UpstreamShell } from "@/components/upstream/UpstreamShell";
import { Mono } from "@/components/ui/Badge";
import { CopyableValue } from "@/components/ui/CopyableValue";
import { TechnicalDetail } from "@/components/ui/TechnicalDetail";
import { Card, CardColumns, CardHeader } from "@/components/ui/Card";
import { Input } from "@/components/ui/Input";
import { FilterBar } from "@/components/ui/FilterBar";
import { PaginationRow, usePaging } from "@/components/ui/PaginationRow";
import { ReadFreshness } from "@/components/ui/ReadFreshness";
import { ProjectName, UserName } from "@/components/names";
import { useUpstreamGrants } from "@/lib/queries/useUpstream";
import { useDebounce } from "@/lib/useDebounce";
import { sortByName, sortNames } from "@/lib/sort";
import { useShownNames } from "@/components/names/shownNames";

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
  const paging = usePaging();
  const limit = paging.limit;
  const search = useDebounce(query, 200).trim().toLowerCase();

  const names = useShownNames();
  // A to Z by the person shown, then the project, before the page cuts the list.
  const rows = useMemo(() => {
    const all = sortByName(
      grants.data?.items ?? [],
      (g) => `${names.user(g.userId)} \u0000${names.project(g.projectId)}`,
    );
    if (!search) return all;
    return all.filter((grant) =>
      [names.user(grant.userId), names.project(grant.projectId), grant.userId, grant.projectId, grant.roleKeys.join(" ")]
        .join(" ")
        .toLowerCase()
        .includes(search),
    );
  }, [grants.data, search, names]);

  const visible = rows.slice(0, limit);

  return (
    <UpstreamShell
      title="Roles held"
      lede="Every role Zitadel has given to anyone, whether Syndra gave it or not."
      syndraHref="/governance/drift?tab=reconciliation"
      syndraLabel="See where the two sides disagree"
    >
      {/* The same row every other list narrows itself from, with the read line 10 under it. */}
      <FilterBar
        read={
          grants.data && (
            <ReadFreshness
              subject="This list"
              state={{ readAt: grants.data.observedAt, truncated: grants.data.truncated }}
              onRefresh={() => grants.refetch()}
              refreshing={grants.isFetching}
            />
          )
        }
        search={
          <Input
            value={query}
            onChange={(event) => {
              setQuery(event.target.value);
              paging.back();
            }}
            placeholder="Name, project or role key"
            aria-label="Filter roles held by person, project or role key"
          />
        }
      />

      <Card>
        <CardHeader
          title="Every role Zitadel has given"
          count={grants.isLoading ? undefined : (grants.data?.total ?? rows.length)}
        />

        <ListStates
          head={
            <CardColumns>
              <span className="w-[230px]">Person</span>
              <span className="w-[200px]">Project</span>
              <span className="flex-1">Roles</span>
              <span className="w-[190px] text-right">ID in Zitadel</span>
            </CardColumns>
          }
          isLoading={grants.isLoading}
          error={grants.error}
          isEmpty={rows.length === 0}
          onRetry={() => grants.refetch()}
          // `fine` is said only when Syndra answered and Zitadel did not; ErrorState drops it for
          // "not set up" (a Syndra-side gap) and for a network fault.
          errorTitle="Couldn't read roles held from Zitadel."
          fine="Syndra itself is fine."
          skeleton={<RowSkeleton rows={8} avatar={false} label="Reading roles held" />}
          empty={
            <EmptyState
              title={search ? "Nothing matches that." : "Nobody holds any roles in Zitadel."}
              guidance={
                search
                  ? "Try a person, a project or a role key, such as trained."
                  : "Zitadel has given no roles to anyone."
              }
            />
          }
        >
          {visible.map((grant) => (
            <div key={grant.id} className="row-divider flex flex-wrap items-center gap-x-4 gap-y-1 px-5 py-2.5">
              <span className="w-full text-[14px] font-semibold [overflow-wrap:anywhere] tablet:w-[230px] tablet:shrink-0">
                <UserName id={grant.userId} />
              </span>
              <span className="w-full text-[14px] text-muted [overflow-wrap:anywhere] tablet:w-[200px] tablet:shrink-0">
                <ProjectName id={grant.projectId} />
              </span>
              <span className="min-w-0 flex-1 basis-full tablet:basis-0">
                {sortNames(grant.roleKeys).map((key) => (
                  <Mono key={key} className="mr-2 text-muted [overflow-wrap:anywhere]">
                    {key}
                  </Mono>
                ))}
              </span>
              <span className="basis-full">
                <TechnicalDetail placeKey={`grant:${grant.id}`}>
                  <CopyableValue value={grant.id} label="Grant id" />
                </TechnicalDetail>
              </span>
            </div>
          ))}

          <PaginationRow
            shown={visible.length}
            total={rows.length}
            onMore={paging.more}
            onAll={() => paging.all(rows.length)}
            onBack={paging.back}
          />
        </ListStates>
      </Card>
    </UpstreamShell>
  );
}
