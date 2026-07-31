"use client";

import { toast } from "sonner";

import { AccessSource } from "@/components/access/AccessSource";
import { EmptyState, ListStates, RowSkeleton } from "@/components/states";
import { Avatar } from "@/components/ui/Avatar";
import { Mono } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Card, CardHeader } from "@/components/ui/Card";
import { PageHeader } from "@/components/ui/PageHeader";
import { ProjectName, UserName } from "@/components/names";
import { useGovernanceSummary } from "@/lib/queries/useGovernance";
import { useCreateGrant } from "@/lib/queries/useUsers";
import { daysUntil, formatShortDate } from "@/lib/format";

/**
 * S7 · Review › Expiring access. Its own route, not an audit tab: audit is a
 * historical record you consult, this is time-boxed work you act on before a
 * deadline, and a badge should point at a destination rather than a tab.
 *
 * There is exactly ONE action here: extend. Doing nothing is not an action and
 * is not drawn as one — no "let it lapse", no dismiss, no secondary control
 * that looks like it submits something. The row simply states the outcome of
 * inaction, and the sweep does the rest.
 */
export default function ExpiringAccessPage() {
  const summary = useGovernanceSummary();
  const grants = [...(summary.data?.expiring_grants ?? [])].sort(sortBySoonest);

  return (
    <div className="flex flex-col gap-[18px]">
      <PageHeader
        title="Expiring access"
        meta="Direct grants approaching their expiry, soonest first."
      />

      <Card>
        <CardHeader
          title="Runs out soon"
          count={grants.length}
          tone="warn"
          note="Extending is the only action — doing nothing lets it lapse."
        />
        <ListStates
          isLoading={summary.isLoading}
          error={summary.error}
          isEmpty={grants.length === 0}
          onRetry={() => summary.refetch()}
          errorTitle="Couldn't load expiring access."
          skeleton={<RowSkeleton rows={4} label="Loading expiring access" />}
          empty={
            <EmptyState
              title="Nothing expires in the next two weeks."
              guidance="Direct grants appear here two weeks before their expiry date."
            />
          }
        >
          {grants.map((grant) => (
            <ExpiringRow key={grant.id} grant={grant} />
          ))}
        </ListStates>
      </Card>
    </div>
  );
}

function ExpiringRow({
  grant,
}: {
  grant: {
    id: string;
    user_id: string;
    project_id: string;
    role_key: string;
    expires_at?: string | null;
  };
}) {
  // Extending re-submits the grant with a later date: POST upserts on
  // (user, project, role) and overwrites expires_at, so this renews in place
  // rather than creating a duplicate.
  const extend = useCreateGrant(grant.user_id);
  const remaining = daysUntil(grant.expires_at);

  return (
    <div className="row-divider flex flex-wrap items-center gap-[18px] px-5 py-3.5">
      <Avatar name={undefined} />
      <div className="w-[170px] shrink-0 truncate text-[15px] font-semibold">
        <UserName id={grant.user_id} />
      </div>
      <div className="w-[250px] shrink-0 truncate text-[14.5px] text-ink/80">
        <ProjectName id={grant.project_id} /> / <Mono>{grant.role_key}</Mono>
      </div>
      <div className="flex min-w-0 flex-1 items-center gap-3">
        <AccessSource kind="direct" />
        <span className="truncate text-[14px] text-muted">
          No action — expires {formatShortDate(grant.expires_at)}
        </span>
      </div>
      <div className="w-[80px] shrink-0 text-[13.5px] font-semibold text-warn-text">
        {remaining === null ? "—" : remaining <= 0 ? "today" : `${remaining} days`}
      </div>
      <Button
        size="sm"
        isPending={extend.isPending}
        onClick={async () => {
          try {
            await extend.mutateAsync({
              project_id: grant.project_id,
              role_key: grant.role_key,
              reason: "Extended from Expiring access",
              duration_days: 90,
            });
            toast.success("Extended by 90 days.");
          } catch (error) {
            toast.error(error instanceof Error ? error.message : "The extension didn't go through.");
          }
        }}
      >
        Extend
      </Button>
    </div>
  );
}

function sortBySoonest(
  a: { expires_at?: string | null },
  b: { expires_at?: string | null },
): number {
  if (!a.expires_at) return 1;
  if (!b.expires_at) return -1;
  return a.expires_at < b.expires_at ? -1 : 1;
}
