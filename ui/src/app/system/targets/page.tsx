"use client";

import Link from "next/link";

import { EmptyState, ListStates, RowSkeleton } from "@/components/states";
import { Card, CardColumns, CardRow } from "@/components/ui/Card";
import { PageHeader } from "@/components/ui/PageHeader";
import { targetLabel } from "@/lib/nav";
import { useTargets, type TargetSummary } from "@/lib/queries/useTargets";

/**
 * Connected systems — the index of the target plane.
 *
 * It exists so that a deployment which has registered NO add-on still has
 * somewhere that says so. Before this page, such a deployment showed nothing
 * about add-ons anywhere: no row, no page, no sentence. The reading an operator
 * takes from that is "the feature is not here", which is a different and wrong
 * answer to the one they asked.
 *
 * The roster is deployment configuration — what was registered, never what is
 * reachable and never what this operator can see. Reachability is a separate
 * fact and is reported per row, because "registered" and "answering" fail
 * independently and an operator's next move differs completely between them.
 */
export default function ConnectedSystemsPage() {
  const targets = useTargets();
  const rows = targets.data ?? [];

  return (
    <div className="flex flex-col gap-[18px]">
      <PageHeader
        title="Connected systems"
        meta={
          rows.length > 0
            ? `${rows.length} ${rows.length === 1 ? "add-on" : "add-ons"} registered`
            : undefined
        }
      />

      <Card>
        <CardColumns>
          <span className="min-w-0 flex-1">System</span>
          <span className="w-[150px] shrink-0">Reachable</span>
          <span className="w-[110px] shrink-0 text-right">Operations</span>
        </CardColumns>

        <ListStates
          isLoading={targets.isLoading}
          error={targets.error}
          isEmpty={rows.length === 0}
          onRetry={() => targets.refetch()}
          errorTitle="Couldn't load the add-on roster."
          skeleton={<RowSkeleton rows={2} avatar={false} label="Loading connected systems" />}
          empty={
            <EmptyState
              title="No system is connected."
              guidance={
                "Syndra manages accounts on a target — a NAS, a door controller — through an " +
                "add-on running in its own container. One appears here after the deployment " +
                "names it in ADDON_TARGETS and starts its container; see DEPLOY.md, " +
                "“Bringing up the TrueNAS add-on”. Until then Syndra governs identity only, " +
                "and nothing outside the identity provider is being reconciled."
              }
            />
          }
        >
          {rows.map((row, index) => (
            <TargetRow key={row.target} row={row} first={index === 0} />
          ))}
        </ListStates>
      </Card>
    </div>
  );
}

/**
 * Three facts, and they are deliberately not collapsed into one status word.
 *
 * `registered` is deployment configuration. `callable` means a capability
 * manifest has been read and understood — registration alone offers nothing.
 * A transport fault is a third thing again: the secret that signs requests
 * failed to load, which is a deployment error and not a target outage, and the
 * two send an operator to different machines.
 */
function TargetRow({ row, first }: { row: TargetSummary; first: boolean }) {
  const transportBroken = row.transport_status === "error";

  return (
    <CardRow first={first} className="flex-wrap">
      <span className="min-w-0 flex-1">
        <Link
          href={`/system/targets/${row.target}`}
          className="text-[15.5px] font-semibold hover:underline"
        >
          {targetLabel(row.target)}
        </Link>
        <span className="block truncate text-[13px] text-faint">{row.target}</span>
      </span>

      <span className="w-[150px] shrink-0 text-[13.5px]">
        {transportBroken ? (
          <span className="text-danger-text">transport failed</span>
        ) : row.circuit_open ? (
          <span className="text-warn-text">calls suspended</span>
        ) : row.callable ? (
          <span className="text-healthy">answering</span>
        ) : (
          <span className="text-warn-text">no manifest yet</span>
        )}
      </span>

      {/* A count, and only when there is a manifest to count. Rendering 0 for a
          target that has never answered would read as "it can do nothing",
          which is a claim about the target rather than about Syndra's ignorance
          of it. */}
      <span className="w-[110px] shrink-0 text-right text-[15px]">
        {row.callable ? row.operations.length : <span className="text-faint">—</span>}
      </span>
    </CardRow>
  );
}
