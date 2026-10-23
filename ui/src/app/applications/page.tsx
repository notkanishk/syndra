"use client";

import Link from "next/link";
import { useMemo } from "react";

import { EmptyState, ListStates, RowSkeleton } from "@/components/states";
import { Mono } from "@/components/ui/Badge";
import { ReadFreshness, type ReadState } from "@/components/ui/ReadFreshness";
import { Card, CardColumns } from "@/components/ui/Card";
import { PageHeader } from "@/components/ui/PageHeader";
import { ProjectName } from "@/components/names";
import { SortHead, orderBy, useSort } from "@/components/ui/SortHead";
import { holdersLine, holdersToneClass } from "@/lib/holders";
import { useApplications, type ApplicationView } from "@/lib/queries/useApplications";
import { Notice } from "@/components/ui/Notice";

type AppSort = "app" | "people";

const APP_SORT: Record<AppSort, (entry: ApplicationView) => string | number> = {
  app: (entry) => entry.application.name,
  // The number the row shows: what Zitadel holds, else what Syndra recorded.
  people: (entry) => entry.observed_user_count ?? entry.assigned_user_count,
};

/**
 * Apps — the index. An app is the thing that receives the token; its page is
 * where "my app isn't seeing the roles it expects" gets answered.
 *
 * The kind rides under the name, before the claim key: an OIDC client, a SAML
 * SP and a plain API consumer fail in different ways, and knowing which one you
 * are looking at changes the first thing you check. It is not a column of its
 * own (board, 8b) — Format and People sit last in their 110px seats.
 */
export default function AppsPage() {
  const apps = useApplications();
  const [sort, setSort, view] = useSort<AppSort>("app");
  const all = useMemo(() => apps.data ?? [], [apps.data]);
  const rows = useMemo(() => orderBy(all, view.sort, APP_SORT), [all, view.sort]);
  const basis = all[0]?.observation;
  const readState: ReadState = {
    readAt: basis?.read_at,
    current: basis ? basis.current : undefined,
    truncated: basis?.truncated,
  };

  // The common failure, surfaced on the index rather than making somebody open
  // every app to find it: two apps reading the same project through different
  // formats. Legal, sometimes deliberate, and the single most common cause of
  // "this app isn't seeing the roles it expects".
  const mixedProjects = useMemo(() => {
    const formats = new Map<string, Set<string>>();
    for (const entry of all) {
      const key = entry.application.project_id;
      if (!formats.has(key)) formats.set(key, new Set());
      formats.get(key)!.add(entry.application.format_type);
    }
    const mixed = new Set<string>();
    formats.forEach((set, projectId) => {
      if (set.size > 1) mixed.add(projectId);
    });
    return mixed;
  }, [all]);

  return (
    <div className="flex flex-col gap-[18px]">
      <PageHeader
        title="Apps"
        lede="An app is something people sign in to — the booking site, the badge reader. Each one is told a person's roles from one project. Open an app when it is not seeing the roles it should."
        meta={
          rows.length > 0
            ? `${rows.length} ${rows.length === 1 ? "app" : "apps"}`
            : undefined
        }
      />

      {mixedProjects.size > 0 && (
        <Notice
          tone="warn"
          title={`${mixedProjects.size === 1 ? "One project is" : `${mixedProjects.size} projects are`} read in more than one format.`}
        >
          That is legal and often deliberate, but it is the single most common cause of
          &ldquo;this app isn&rsquo;t seeing the roles it expects&rdquo; — so the index surfaces it
          rather than making somebody open every app to find it.
        </Notice>
      )}

      {/* One basis for the whole page — every row carries an identical copy,
          so the freshness is stated once here rather than once per row. The
          People column's "unexplained" and "not in Zitadel yet" are only
          honest beside it. */}
      <ReadFreshness state={readState} subject="What Zitadel confirmed" className="px-[2px]" />

      <Card>
        <CardColumns>
          <span className="w-[250px] min-w-[170px]">
            <SortHead label="App" name="app" sort={sort} onSort={setSort} />
          </span>
          <span className="min-w-[140px] flex-1">Reads roles from</span>
          <span className="w-[110px] shrink-0 text-right">Format</span>
          <span className="w-[110px] shrink-0 text-right">
            <SortHead label="People" name="people" sort={sort} onSort={setSort} />
          </span>
        </CardColumns>

        <div className={view.className}>
        <ListStates
          isLoading={apps.isLoading}
          error={apps.error}
          isEmpty={rows.length === 0}
          onRetry={() => apps.refetch()}
          errorTitle="Couldn't load applications."
          skeleton={<RowSkeleton rows={4} avatar={false} label="Loading applications" />}
          empty={
            <EmptyState
              title="No applications registered."
              guidance="Add an app in Zitadel and it appears here."
              action={{ label: "Open Zitadel", href: "/zitadel" }}
            />
          }
        >
          {rows.map((entry) => {
            const people = holdersLine(
              entry.assigned_user_count,
              entry.confirmed_user_count,
              entry.observed_user_count,
            );
            return (
            <Link
              key={entry.application.id}
              href={`/applications/${entry.application.id}`}
              className="row-divider flex min-h-[60px] flex-col items-start gap-1.5 px-5 py-3.5 motion-tint hover:bg-[var(--hover)] tablet:flex-row tablet:items-center tablet:gap-[18px]"
            >
              <span className="w-full min-w-0 tablet:w-[250px] tablet:min-w-[170px] tablet:shrink-0">
                <span className="block text-[15.5px] font-semibold [text-wrap:pretty]">
                  {entry.application.name}
                </span>
                <span className="block text-[13px] text-faint [text-wrap:pretty]">
                  {kindOf(entry)} · <Mono>{entry.application.claim_name}</Mono>
                </span>
              </span>

              <span className="w-full text-[14px] text-muted tablet:min-w-[140px] tablet:flex-1">
                <ProjectName id={entry.application.project_id} />
              </span>

              <span
                className={`text-[13px] tablet:w-[110px] tablet:shrink-0 tablet:text-right ${
                  mixedProjects.has(entry.application.project_id)
                    ? "font-semibold text-warn-text"
                    : "text-muted"
                }`}
              >
                <Mono>{entry.application.format_type}</Mono>
                {mixedProjects.has(entry.application.project_id) && (
                  <span className="block text-[12.5px] font-normal">
                    mixed in <ProjectName id={entry.application.project_id} />
                  </span>
                )}
              </span>

              <span className="text-[15px] tablet:w-[110px] tablet:shrink-0 tablet:text-right">
                {people.headline}
                <span className="text-[13px] text-faint tablet:hidden">
                  {people.headline === "1" ? " person" : " people"}
                </span>
                {people.note && (
                  <span className={`block text-[12.5px] ${holdersToneClass[people.tone]}`}>
                    {people.note}
                  </span>
                )}
              </span>
            </Link>
            );
          })}
        </ListStates>
        </div>
      </Card>
    </div>
  );
}

/**
 * The backend calls it `consumer`; on screen it is the protocol, because that
 * is the word an operator debugging a token already has in their head.
 */
function kindOf(entry: ApplicationView): string {
  const raw = (entry.application.consumer || "").toLowerCase();
  if (raw.includes("saml")) return "SAML";
  if (raw.includes("oidc") || raw.includes("oauth")) return "OIDC";
  if (raw.includes("api")) return "API";
  return raw ? entry.application.consumer : "—";
}
