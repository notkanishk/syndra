"use client";

import Link from "next/link";

import { EmptyState, ListStates, RowSkeleton } from "@/components/states";
import { Mono, STATUS_TONE, StatusDot, type StatusTone } from "@/components/ui/Badge";
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
        lede="The systems Syndra creates and manages accounts on — today, TrueNAS, the network storage server. Each one is joined to Syndra by an add-on, a small program that connects Syndra to the system. Open a system to see whether it is answering, who has an account there, and anything waiting on you."
        meta={
          rows.length > 0
            ? `${rows.length} ${rows.length === 1 ? "system" : "systems"} connected`
            : undefined
        }
      />

      <Card>
        <CardColumns>
          <span className="min-w-0 flex-1">System</span>
          <span className="w-[150px] shrink-0">Status</span>
          <span className="w-[110px] shrink-0 text-right">Things it can do</span>
        </CardColumns>

        <ListStates
          isLoading={targets.isLoading}
          error={targets.error}
          isEmpty={rows.length === 0}
          onRetry={() => targets.refetch()}
          errorTitle="The list of connected systems could not be read."
          skeleton={<RowSkeleton rows={2} avatar={false} label="Loading connected systems" />}
          empty={
            <EmptyState
              title="No system is connected."
              guidance={
                "Syndra manages accounts on a system outside itself — network storage, a door " +
                "controller — through an add-on running in its own container. For whoever runs " +
                "the Syndra server: one appears here after the deployment names it in " +
                "ADDON_TARGETS and starts its container; see DEPLOY.md, “Bringing up the " +
                "TrueNAS add-on”. Until then Syndra manages sign-in only, and no outside " +
                "system is being kept in step."
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
  const reading = readingFor(row);

  return (
    <CardRow first={first} className="flex-wrap">
      <span className="min-w-0 flex-1">
        <Link
          href={`/system/targets/${row.target}`}
          className="text-[15.5px] font-semibold hover:underline"
        >
          {targetLabel(row.target)}
        </Link>
        <Mono className="block truncate text-faint">{row.target}</Mono>
      </span>

      {/* A dot and a word, the same idiom the target's own health card uses —
          never colour alone. Green and amber are one word to an operator who
          cannot tell them apart, and one word in a greyscale screenshot. */}
      <span className="flex w-[150px] shrink-0 items-center gap-2 text-[13.5px]">
        <StatusDot tone={reading.tone} />
        <span className={STATUS_TONE[reading.tone].label}>{reading.label}</span>
      </span>

      {/* A count, and only when there is a manifest to count. Rendering 0 for a
          target that has never answered would read as "it can do nothing",
          which is a claim about the target rather than about Syndra's ignorance
          of it. */}
      <span className="w-[110px] shrink-0 text-right text-[15px]">
        {row.callable ? row.operations.length : <span className="text-faint">—</span>}
      </span>

      {/* Which machine to look at. The status word alone sends a reader to the
          wrong one: two of these are faults on the Syndra server. */}
      {reading.detail && (
        <span className="w-full text-[13px] text-muted">{reading.detail}</span>
      )}
    </CardRow>
  );
}

/**
 * Registration is deployment configuration, a manifest having been read is a
 * runtime fact, and a transport secret that will not load is a fault on
 * Syndra's side. Three failures, in the order an operator has to rule them out:
 * a broken transport explains everything below it, and a suspended breaker
 * explains a target that would otherwise answer.
 */
function readingFor(row: TargetSummary): { tone: StatusTone; label: string; detail?: string } {
  const name = targetLabel(row.target);
  if (row.transport_status === "error")
    return {
      tone: "danger",
      label: "Cannot connect",
      detail: `Syndra cannot read the key it signs requests with. Look at the Syndra server, not at ${name}.`,
    };
  if (row.circuit_open)
    return {
      tone: "warn",
      label: "Paused after failures",
      detail: `Syndra stopped calling ${name} after repeated failures and will try again by itself. This does not mean ${name} is down — look at the Syndra server.`,
    };
  if (row.callable) return { tone: "healthy", label: "Answering" };
  // Neutral, not amber. Registered-and-not-yet-answered is where every add-on
  // starts, and it resolves on its own within a refresh interval — see the tone
  // itself for why colouring it would cost amber its meaning elsewhere.
  return {
    tone: "neutral",
    label: "Not answered yet",
    detail: `The add-on is configured and has not answered yet. This usually clears by itself within a minute or two.`,
  };
}
