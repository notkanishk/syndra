"use client";

import Link from "next/link";

import { EmptyState, ListStates, RowSkeleton } from "@/components/states";
import { Mono } from "@/components/ui/Badge";
import { ReadingWord, targetReading, type Reading } from "@/components/targets/TargetReading";
import { ArrowLink } from "@/components/ui/ArrowLink";
import { Card, CardColumns, CardRow } from "@/components/ui/Card";
import { PageHeader } from "@/components/ui/PageHeader";
import { Term } from "@/components/ui/Term";
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
        lede={
          <>
            The systems Syndra creates and manages accounts on — today,{" "}
            <Term name="truenas">TrueNAS</Term>. Each one is joined to Syndra by an add-on, a small
            program that connects Syndra to the system. Open a system to see whether it is
            answering, who has an account there, and anything waiting on you.
          </>
        }
        meta={
          rows.length > 0
            ? `${rows.length} ${rows.length === 1 ? "system" : "systems"} connected`
            : undefined
        }
      />

      <Card>
        <CardColumns>
          <span className="min-w-0 flex-1">System</span>
          <span className="w-[190px] shrink-0">Status</span>
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
  const name = targetLabel(row.target);

  return (
    <CardRow first={first} className="flex-wrap">
      <span className="min-w-[min(200px,100%)] flex-1">
        <Link
          href={`/system/targets/${row.target}`}
          className="text-[15.5px] font-semibold hover:underline"
        >
          {name}
        </Link>
        {/* The project it serves, read-only: it is set in one place, the
            system's own page. Absent is the row's Needs you. */}
        {row.project === null ? (
          <ArrowLink href={`/system/targets/${row.target}`}>Choose its Zitadel project</ArrowLink>
        ) : row.project ? (
          <span className="block truncate text-[13px] text-muted">
            Serves {row.project.name || "a project Syndra couldn't name"}
          </span>
        ) : (
          <Mono className="block truncate text-faint">{row.target}</Mono>
        )}
      </span>

      {/* A dot and a word, the same idiom the target's own page uses — never
          colour alone — with the check pips when a check was missed, and the
          time it began faint under it. */}
      <span className="flex w-[190px] shrink-0 flex-col gap-0.5 text-[13.5px]">
        <ReadingWord reading={reading} recent={row.health?.recent ?? []} />
        {reading.line && <span className="pl-4 text-[12.5px] text-faint">{reading.line}</span>}
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
      {reading.detail && <span className="w-full text-[13px] text-muted">{reading.detail}</span>}
    </CardRow>
  );
}

/**
 * The backend ranks what can be true at once (A-01): cannot connect, offline,
 * waiting to retry, checking, answering. One reading here, besides: an add-on
 * registered and not answered yet, which is where every add-on starts.
 */
function readingFor(row: TargetSummary): Reading & { detail?: string } {
  const name = targetLabel(row.target);
  const reading = targetReading({ status: row.status, health: row.health, name, transportError: row.transport_error });
  if (row.status === "cannot_connect") {
    return { ...reading, detail: `Syndra cannot read the key it signs requests with. ${reading.look}` };
  }
  if (row.status === "offline") return { ...reading, detail: reading.look };
  if (row.status === "waiting_to_retry") return { ...reading, detail: reading.sentence };
  if (!row.callable) {
    // Neutral, not amber. Registered-and-not-yet-answered resolves on its own
    // within a refresh interval, and colouring it would cost amber its meaning.
    return {
      tone: "neutral",
      word: "Not answered yet",
      pips: false,
      detail: "The add-on is configured and has not answered yet. This usually clears by itself within a minute or two.",
    };
  }
  return reading;
}
