"use client";

import { useState } from "react";

import { FoldHeading } from "@/components/bundles/FoldHeading";
import { RetiredStatus } from "@/components/bundles/RetiredBundle";
import { ProjectName } from "@/components/names";
import { EmptyState, ListStates, RowSkeleton } from "@/components/states";
import { Mono } from "@/components/ui/Badge";
import { Card } from "@/components/ui/Card";
import { Unfold } from "@/components/ui/Unfold";
import { useRetiredMappingRules } from "@/lib/queries/useMappingRules";

/**
 * The end of the Automatic rules list: a quiet fold of the rules that stopped being given. Closed,
 * it reads nothing; opened, each is its sentence with when it was retired and by whom. Read-only,
 * and nothing counts people: a retired rule gives the second role to nobody.
 */
export function RetiredRulesFold() {
  const [open, setOpen] = useState(false);
  const retired = useRetiredMappingRules();
  // Nothing retired, or not read yet: the list ends where it ends, with no empty fold to open.
  const n = retired.data?.length ?? 0;
  if (n === 0 && !retired.error) return null;
  return (
    <Card>
      <FoldHeading open={open} onClick={() => setOpen(!open)} title="Retired rules" count={n ? String(n) : ""} picked={false} />
      <Unfold open={open}>
        <RetiredRuleRows />
      </Unfold>
    </Card>
  );
}

function RetiredRuleRows() {
  const retired = useRetiredMappingRules();
  const rows = retired.data ?? [];
  return (
    <ListStates
      isLoading={retired.isLoading}
      error={retired.error}
      isEmpty={rows.length === 0}
      onRetry={() => retired.refetch()}
      errorTitle="Couldn't load the retired rules."
      skeleton={<RowSkeleton rows={2} avatar={false} label="Loading retired rules" />}
      empty={<EmptyState title="Nothing has been retired." guidance="A rule you retire is listed here, and stays in the history." />}
    >
      {rows.map((rule) => (
        <div key={rule.id} className="row-divider flex flex-col gap-1.5 px-5 py-3.5">
          <span className="text-[14.5px] text-muted">
            <ProjectName id={rule.source_project} /> / <Mono>{rule.source_role}</Mono>
            <span aria-hidden className="mx-2.5 text-faint">
              ⇒
            </span>
            <span className="sr-only">then </span>
            <ProjectName id={rule.target_project} /> / <Mono>{rule.target_role}</Mono>
          </span>
          <span className="text-[13px] text-faint">
            <RetiredStatus at={rule.retired_at} by={rule.retired_by} />
          </span>
        </div>
      ))}
    </ListStates>
  );
}
