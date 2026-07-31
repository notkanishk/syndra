"use client";

import Link from "next/link";
import { useMemo } from "react";

import { EmptyState, ListStates, RowSkeleton } from "@/components/states";
import { Mono } from "@/components/ui/Badge";
import { Card, CardColumns } from "@/components/ui/Card";
import { PageHeader } from "@/components/ui/PageHeader";
import { ProjectName } from "@/components/names";
import { useApplications, type ApplicationView } from "@/lib/queries/useApplications";

/**
 * Apps — the index. An app is the thing that receives the token; its page is
 * where "my app isn't seeing the roles it expects" gets answered.
 *
 * Kind is shown because an OIDC client, a SAML SP and a plain API consumer fail
 * in different ways, and knowing which one you are looking at changes the first
 * thing you check.
 */
export default function AppsPage() {
  const apps = useApplications();
  const rows = useMemo(() => apps.data ?? [], [apps.data]);

  // The common failure, surfaced on the index rather than making somebody open
  // every app to find it: two apps reading the same project through different
  // formats. Legal, sometimes deliberate, and the single most common cause of
  // "this app isn't seeing the roles it expects".
  const mixedProjects = useMemo(() => {
    const formats = new Map<string, Set<string>>();
    for (const entry of rows) {
      const key = entry.application.project_id;
      if (!formats.has(key)) formats.set(key, new Set());
      formats.get(key)!.add(entry.application.format_type);
    }
    const mixed = new Set<string>();
    formats.forEach((set, projectId) => {
      if (set.size > 1) mixed.add(projectId);
    });
    return mixed;
  }, [rows]);

  return (
    <div className="flex flex-col gap-[18px]">
      <PageHeader
        title="Apps"
        meta={
          rows.length > 0
            ? `${rows.length} ${rows.length === 1 ? "app receives" : "apps receive"} a token and read roles out of it`
            : undefined
        }
      />

      {mixedProjects.size > 0 && (
        <div className="warn-note flex items-start gap-3.5 px-[18px] py-3.5">
          <span
            aria-hidden
            className="mt-px flex h-5 w-5 flex-none items-center justify-center rounded-pill bg-warn-soft text-[12px] font-bold text-warn-text"
          >
            !
          </span>
          <p className="max-w-[92ch] text-[14px] leading-[1.55] text-muted">
            <strong className="font-semibold text-ink">
              {mixedProjects.size === 1 ? "One project is" : `${mixedProjects.size} projects are`}{" "}
              read in more than one format.
            </strong>{" "}
            That is legal and often deliberate, but it is the single most common cause of
            &ldquo;this app isn&rsquo;t seeing the roles it expects&rdquo; — so the index surfaces
            it rather than making somebody open every app to find it.
          </p>
        </div>
      )}

      <Card>
        <CardColumns>
          <span className="w-[170px]">App</span>
          <span className="w-[80px]">Kind</span>
          <span className="flex-1">Reads roles from</span>
          <span className="w-[110px] text-right">People</span>
          <span className="w-[96px] text-right">Format</span>
        </CardColumns>

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
              guidance="Add an OIDC client, API or SAML app in the identity provider and it appears here."
              action={{ label: "Open the identity provider", href: "/zitadel" }}
            />
          }
        >
          {rows.map((entry) => (
            <Link
              key={entry.application.id}
              href={`/applications/${entry.application.id}`}
              className="row-divider flex items-center gap-[18px] px-5 py-3.5 transition-colors hover:bg-[var(--hover)]"
            >
              <span className="w-[170px] min-w-0">
                <span className="block truncate text-[15.5px] font-semibold">
                  {entry.application.name}
                </span>
                <span className="block truncate text-[13px] text-faint">
                  <Mono>{entry.application.claim_name}</Mono>
                </span>
              </span>

              <span className="w-[80px] shrink-0 text-[13px] font-semibold uppercase tracking-[0.06em] text-muted">
                {kindOf(entry)}
              </span>

              <span className="min-w-0 flex-1 truncate text-[14px] text-muted">
                <ProjectName id={entry.application.project_id} />
              </span>

              <span className="w-[110px] text-right text-[15px]">
                {entry.assigned_user_count}
              </span>

              <span
                className={`w-[96px] text-right text-[13px] ${
                  mixedProjects.has(entry.application.project_id)
                    ? "font-semibold text-warn-text"
                    : "text-muted"
                }`}
              >
                <Mono>{entry.application.format_type}</Mono>
              </span>
            </Link>
          ))}
        </ListStates>
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
