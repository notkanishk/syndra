"use client";

import { EmptyState, ListStates, RowSkeleton } from "@/components/states";
import { Badge, Mono } from "@/components/ui/Badge";
import { Card, CardHeader } from "@/components/ui/Card";
import { PageHeader } from "@/components/ui/PageHeader";
import { ProjectName, UserName } from "@/components/names";
import { useRecentCascades } from "@/lib/queries/useConfirmationMode";
import { Relative } from "@/components/ui/Time";

/**
 * S4 · Automation › Change history. What a bundle or rule change actually did
 * downstream — the record that turns "I edited Lab Tech" into "and these
 * eleven people gained a role because of it".
 */
export default function ChangeHistoryPage() {
  const cascades = useRecentCascades();
  const rows = cascades.data ?? [];

  return (
    <div className="flex flex-col gap-[18px]">
      <PageHeader
        title="Change history"
        meta="What each bundle or rule change did to real people's access."
      />

      <Card>
        <CardHeader title="Applied" count={rows.length} />
        <ListStates
          isLoading={cascades.isLoading}
          error={cascades.error}
          isEmpty={rows.length === 0}
          onRetry={() => cascades.refetch()}
          errorTitle="Couldn't load change history."
          skeleton={<RowSkeleton rows={4} label="Loading change history" />}
          empty={
            <EmptyState
              title="Nothing has cascaded yet."
              guidance="Editing a bundle or a rule writes its downstream effect here."
            />
          }
        >
          {rows.map((row) => (
            <div key={row.id} className="row-divider flex flex-wrap items-center gap-4 px-5 py-3.5">
              <Badge tone={row.op_type === "revoke" ? "danger" : "accent"}>
                {row.op_type === "revoke" ? "Removed" : "Added"}
              </Badge>
              <div className="min-w-[220px] flex-1">
                <div className="text-[15px] font-semibold">
                  <UserName id={row.user_id} />
                </div>
                <div className="truncate text-[13.5px] text-muted">
                  <ProjectName id={row.project_id} /> ·{" "}
                  {(row.role_keys ?? []).map((key) => (
                    <Mono key={key} className="mr-1.5">
                      {key}
                    </Mono>
                  ))}
                </div>
              </div>
              <div className="w-[170px] text-[13.5px] text-muted">
                because of {row.source === "rule" ? "an automatic rule" : `a ${row.source} change`}
              </div>
              <div className="w-[140px] text-[13px] text-faint">
                <Relative iso={row.completed_at ?? undefined} />
              </div>
            </div>
          ))}
        </ListStates>
      </Card>
    </div>
  );
}
