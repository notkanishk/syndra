"use client";

import Link from "next/link";
import { Fragment, useMemo } from "react";

import { EmptyState, ListStates, RowSkeleton } from "@/components/states";
import { Mono, StatusDot } from "@/components/ui/Badge";
import { ReadFreshness, type ReadState } from "@/components/ui/ReadFreshness";
import { Card, CardColumns } from "@/components/ui/Card";
import { Notice } from "@/components/ui/Notice";
import { PageHeader } from "@/components/ui/PageHeader";
import { ProjectName } from "@/components/names";
import { SortHead, orderBy, useSort } from "@/components/ui/SortHead";
import { holdersLine, holdersToneClass } from "@/lib/holders";
import { useApplications, type ApplicationView } from "@/lib/queries/useApplications";
import { useClaimTemplates, type ClaimTemplate } from "@/lib/queries/useClaimShape";

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
  const [sort, setSort, view] = useSort<AppSort>("app", { list: "applications", keys: ["app", "people"] });
  const all = useMemo(() => apps.data ?? [], [apps.data]);
  const rows = useMemo(() => orderBy(all, view.sort, APP_SORT), [all, view.sort]);
  const basis = all[0]?.observation;
  const readState: ReadState = {
    readAt: basis?.read_at,
    current: basis ? basis.current : undefined,
    truncated: basis?.truncated,
  };

  const templates = useClaimTemplates().data ?? [];
  // A mismatch is said once, above the table: the app's page says the fix (A-14 §7).
  const unreached = all.filter((entry) => entry.claims?.reaches === false).map((entry) => entry.application.name);

  return (
    <div className="flex flex-col gap-4">
      <PageHeader
        title="Apps"
        lede="An app is something people sign in to, like NetBird or GitLab. Each is told a person's roles from one project, in its own shape."
        meta={
          rows.length > 0
            ? `${rows.length} ${rows.length === 1 ? "app" : "apps"}`
            : undefined
        }
      />

      {/* One basis for the whole page — every row carries an identical copy,
          so the freshness is stated once here rather than once per row. The
          People column's "unexplained" and "not in Zitadel yet" are only
          honest beside it. */}
      {/* Not until a read has landed: no read is not a stale one. */}
      {apps.data && (
        <ReadFreshness state={readState} subject="What Zitadel confirmed" className="px-[2px]" />
      )}
      {unreached.length > 0 && (
        <Notice tone="warn" title={`${unreached.join(", ")} ${unreached.length === 1 ? "doesn't" : "don't"} get ${unreached.length === 1 ? "its" : "their"} claims.`}>
          {unreached.length === 1 ? "Its page says" : "Each page says"} which Zitadel setting to change.
        </Notice>
      )}

      <Card stale={readState.current === false}>
        <CardColumns>
          <span className="w-[300px] min-w-[170px]">
            <SortHead label="App" name="app" sort={sort} onSort={setSort} />
          </span>
          <span className="min-w-[140px] flex-1">Reads roles from</span>
          <span className="w-[220px] shrink-0 text-right">Claims</span>
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
          errorTitle="Couldn't load apps."
          skeleton={<RowSkeleton rows={4} avatar={false} label="Loading apps" />}
          empty={
            <EmptyState
              title="No apps registered."
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
            const subLine = [kindOf(entry), shapeLine(entry, all, templates)].filter(Boolean);
            return (
            <Link
              key={entry.application.id}
              href={`/applications/${entry.application.id}`}
              className="row-divider flex min-h-[60px] flex-col items-start gap-1.5 px-5 py-3.5 hover-tint tablet:flex-row tablet:items-center tablet:gap-[18px]"
            >
              <span className="w-full min-w-0 tablet:w-[300px] tablet:min-w-[170px] tablet:shrink-0">
                <span className="block text-[15.5px] font-semibold [text-wrap:pretty]">
                  {entry.application.name}
                </span>
                <span className="block text-[13px] text-faint [text-wrap:pretty]">
                  {subLine.map((part, i) => (
                    <Fragment key={i}>
                      {i > 0 && " · "}
                      {part}
                    </Fragment>
                  ))}
                </span>
              </span>

              <span className="w-full text-[14px] text-muted tablet:min-w-[140px] tablet:flex-1">
                <ProjectName id={entry.application.project_id} />
              </span>

              <span className="text-[13px] tablet:w-[220px] tablet:shrink-0 tablet:text-right">
                <ClaimsCell entry={entry} />
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
 * is the word an operator debugging a token already has in their head. OIDC is
 * the usual, so it is said by saying nothing (A-17 §2.9).
 */
function kindOf(entry: ApplicationView): string | null {
  const raw = (entry.application.consumer || "").toLowerCase();
  if (raw.includes("saml")) return "SAML";
  if (raw.includes("oidc") || raw.includes("oauth")) return null;
  if (raw.includes("api")) return "API";
  return raw ? entry.application.consumer : null;
}

const WHERE: Record<string, string> = {
  access_token: "access token",
  userinfo: "ID token, userinfo",
};

/**
 * The Claims column (A-14 §7, A-17 §2.9): the app's real key and "and N more",
 * on one quiet line. Where they land is said only when it is not the usual
 * (access token and userinfo); amber when they don't reach.
 */
function ClaimsCell({ entry }: { entry: ApplicationView }) {
  const c = entry.claims;
  if (/truenas/i.test(entry.application.name) && !c?.keys.length) return <span className="text-faint">Access goes through the add-on</span>;
  if (!c) return <span className="text-faint">—</span>;
  if (c.keys.length === 0) return <span className="text-faint">Nothing of its own</span>;
  const where = c.lands === "both" ? null : (WHERE[c.lands] ?? c.lands);
  return (
    <span className="flex flex-col tablet:items-end">
      <span>
        <Mono>{c.keys[0]}</Mono>
        {c.reaches !== false && (
          <span className="text-faint">
            {c.keys.length > 1 ? ` and ${c.keys.length - 1} more` : ""}
            {where ? ` · ${where}` : ""}
          </span>
        )}
      </span>
      {c.reaches === false && (
        <span className="inline-flex items-center gap-1.5 font-semibold text-warn-text">
          <StatusDot tone="warn" />
          Doesn&rsquo;t reach it
        </span>
      )}
    </span>
  );
}

/** The sub-line's shape: "NetBird template, changed", "Code's default", "copied from NetBird". */
function shapeLine(entry: ApplicationView, all: ApplicationView[], templates: ClaimTemplate[]): React.ReactNode {
  const c = entry.claims;
  if (/truenas/i.test(entry.application.name) && (!c || c.source !== "app")) return "no template: TrueNAS can't sign in with Zitadel";
  if (!c) return null;
  if (c.source !== "app") {
    return (
      <>
        <ProjectName id={entry.application.project_id} />
        &rsquo;s default
      </>
    );
  }
  if (c.copied_from_application_id) {
    const from = all.find((a) => a.application.id === c.copied_from_application_id)?.application.name;
    return from ? `copied from ${from}` : "copied from another app";
  }
  const t = templates.find((x) => x.id === c.template_id);
  if (!t) return "its own shape";
  const bare = t.id === "custom" || t.id === "sign-in-only";
  return `${t.name}${bare ? "" : " template"}${c.template_changed ? ", changed" : ""}`;
}
