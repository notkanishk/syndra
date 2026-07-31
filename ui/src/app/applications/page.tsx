"use client";

import Link from "next/link";

import { EmptyState, ListStates, RowSkeleton } from "@/components/states";
import { Mono } from "@/components/ui/Badge";
import { Card, CardColumns } from "@/components/ui/Card";
import { PageHeader } from "@/components/ui/PageHeader";
import { useApplications } from "@/lib/queries/useApplications";

/**
 * Apps — the index. An app is the thing that receives the token; its page is
 * where "my app isn't seeing the roles it expects" gets answered.
 */
export default function AppsPage() {
  const apps = useApplications();
  const rows = apps.data ?? [];

  return (
    <div className="flex flex-col gap-[18px]">
      <PageHeader
        title="Apps"
        meta={
          rows.length > 0
            ? `${rows.length} ${rows.length === 1 ? "application" : "applications"} reading MkAuth claims`
            : undefined
        }
      />

      <Card>
        <CardColumns>
          <span className="flex-1">App</span>
          <span className="w-[180px]">Reads project</span>
          <span className="w-[260px]">Claim</span>
          <span className="w-[90px] text-right">People</span>
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
              <span className="min-w-0 flex-1">
                <span className="block truncate text-[15.5px] font-semibold">
                  {entry.application.name}
                </span>
                <span className="block truncate text-[13px] text-faint">
                  {entry.application.consumer}
                </span>
              </span>
              <span className="w-[180px] truncate text-[14px] text-muted">
                {entry.application.project_id}
              </span>
              <span className="flex w-[260px] items-center gap-2 truncate">
                <Mono className="text-muted">{entry.application.claim_name}</Mono>
                <span className="rounded-pill bg-tint-2 px-2 py-0.5 text-[12px]">
                  {entry.application.format_type}
                </span>
              </span>
              <span className="w-[90px] text-right text-[15px]">{entry.assigned_user_count}</span>
            </Link>
          ))}
        </ListStates>
      </Card>
    </div>
  );
}
