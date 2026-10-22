"use client";

import Link from "next/link";

import { EmptyState, ListStates, RowSkeleton } from "@/components/states";
import { Mono } from "@/components/ui/Badge";
import { ReadingWord, targetReading, type Reading } from "@/components/targets/TargetReading";
import { ArrowLink } from "@/components/ui/ArrowLink";
import { Card, CardColumns } from "@/components/ui/Card";
import { Notice } from "@/components/ui/Notice";
import { PageHeader } from "@/components/ui/PageHeader";
import { Relative } from "@/components/ui/Time";
import { formatClock, versionLabel } from "@/lib/format";
import { targetLabel } from "@/lib/nav";
import {
  useMergeFindings,
  useTargetHealth,
  useTargetInventory,
  useTargets,
  type TargetSummary,
} from "@/lib/queries/useTargets";
import { UNKNOWN_PROJECT } from "@/components/names/Unresolved";

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
  // A to Z, as the board draws it: the roster arrives in registration order.
  const rows = [...(targets.data ?? [])].sort((a, b) =>
    targetLabel(a.target).localeCompare(targetLabel(b.target)),
  );

  return (
    <div className="flex flex-col gap-[18px]">
      <PageHeader
        title="Connected systems"
        lede={
          rows.length > 0
            ? "The systems outside Syndra it creates and manages accounts on, each joined by an add-on. Open a system to see who has an account there and anything waiting on you."
            : "The systems outside Syndra it creates and manages accounts on, each joined to Syndra by an add-on."
        }
        meta={
          rows.length > 0
            ? `${rows.length} ${rows.length === 1 ? "system" : "systems"} connected`
            : undefined
        }
      />

      {/* A fault is said once, above the card, and the row keeps its dot and
          word. Where to look is the sentence's job: two of these are faults on
          the Syndra server, not on the system. */}
      {rows.map((row) => (
        <FaultNotice key={row.target} row={row} />
      ))}

      <Card>
        <CardColumns>
          <span className="min-w-0 flex-1">
            System <span aria-hidden>↑</span>
          </span>
          <span className="w-[180px] shrink-0">Status</span>
          <span className="hidden w-[250px] shrink-0 desktop:block">Needs you</span>
          <span className="w-[110px] shrink-0 text-right">People</span>
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
 * The one Notice a faulted system earns: red for a key Syndra cannot read,
 * amber for a system that stopped answering or a breaker waiting to retry.
 * No answer yet is no fault and gets none — it clears by itself.
 */
function FaultNotice({ row }: { row: TargetSummary }) {
  const name = targetLabel(row.target);
  const reading = readingFor(row);
  if (row.status === "cannot_connect") {
    return (
      <Notice tone="danger" blocking title={`Syndra can't connect to ${name}.`}>
        {reading.sentence} Nothing is reaching {name}: no new accounts, and no revocations.{" "}
        {reading.look}
      </Notice>
    );
  }
  if (row.status === "offline") {
    return (
      <Notice tone="warn" live title={`${name} isn't answering.`}>
        Syndra&apos;s last 3 checks went unanswered, from {formatClock(row.health?.reachable_since)},
        and it keeps checking every 30 seconds. Look at {name}, not at Syndra. Changes for {name}{" "}
        wait until it answers, and nothing is lost.
      </Notice>
    );
  }
  if (row.status === "waiting_to_retry") {
    return (
      <Notice tone="warn" live title={`Syndra is waiting to retry ${name}.`}>
        {reading.sentence}
      </Notice>
    );
  }
  return null;
}

/**
 * Three facts, and they are deliberately not collapsed into one status word.
 *
 * `registered` is deployment configuration. `callable` means a capability
 * manifest has been read and understood — registration alone offers nothing.
 * A transport fault is a third thing again: the secret that signs requests
 * failed to load, which is a deployment error and not a target outage, and the
 * two send an operator to different machines.
 *
 * Product, People and Needs you are read from the system's own page reads —
 * health, inventory and findings — so the index and the page cannot disagree.
 * A system that has never answered is not asked.
 */
function TargetRow({ row, first }: { row: TargetSummary; first: boolean }) {
  const reading = readingFor(row);
  const name = targetLabel(row.target);
  const ask = row.callable ? row.target : undefined;
  const health = useTargetHealth(ask);
  const inventory = useTargetInventory(ask);
  const findings = useMergeFindings(ask);

  const answering = reading.tone === "healthy";
  const bound = inventory.data?.bound;
  const version = versionLabel(name, health.data?.product_version);
  const product = health.data?.product ? `${health.data.product} ${version}`.trim() : null;
  const href = `/system/targets/${row.target}`;

  // What is waiting, each linking to where it is handled on the system's page.
  const unmanaged = inventory.data?.unmanaged?.length ?? 0;
  const changed = findings.data?.length ?? 0;
  const decisions =
    (health.data?.log_anchor?.violation_reason ? 1 : 0) + (health.data?.binding_conflicts?.length ?? 0);
  const needs: Array<{ text: string; href: string }> = [];
  if (answering) {
    if (unmanaged > 0)
      needs.push({
        text: `${unmanaged} ${unmanaged === 1 ? "account" : "accounts"} Syndra doesn't manage`,
        href: `${href}#people`,
      });
    if (changed > 0)
      needs.push({
        text: `${changed} ${changed === 1 ? "value" : "values"} somebody changed there`,
        href: `${href}#waiting`,
      });
    if (decisions > 0)
      needs.push({
        text: `${decisions} ${decisions === 1 ? "thing needs" : "things need"} a decision`,
        href: `${href}#waiting`,
      });
  }
  const loading = Boolean(ask) && (health.isLoading || inventory.isLoading || findings.isLoading);
  const unread = Boolean(ask) && (health.isError || inventory.isError || findings.isError);

  return (
    <div
      className={`flex flex-col items-start gap-2 px-5 py-4 tablet:flex-row tablet:flex-wrap tablet:items-start tablet:gap-[18px] ${first ? "" : "row-divider"}`}
    >
      <span className="flex min-w-[min(220px,100%)] flex-1 flex-col gap-0.5">
        <Link href={href} className="text-[15.5px] font-semibold hover:underline">
          {name}
        </Link>
        <span className={`text-[13px] ${product ? "text-muted" : "text-faint"}`}>
          {product ?? "Not read yet"}
        </span>
        {/* The project it serves, read-only: it is set in one place, the
            system's own page. Absent is the row's Needs you. */}
        {row.project ? (
          <span className="text-[13px] text-muted">
            Zitadel project{" "}
            <Link href={`/projects/${row.project.id}`} className="text-ink hover:underline">
              {row.project.name || UNKNOWN_PROJECT}
            </Link>
          </span>
        ) : row.project === null ? (
          <span className="text-[13px] text-faint">No Zitadel project yet</span>
        ) : null}
        <Mono className="truncate text-faint">{row.target}</Mono>
      </span>

      {/* A dot and a word, the same idiom the target's own page uses — never
          colour alone — with the check pips when a check was missed, and the
          time it began faint under it. */}
      <span className="flex flex-col gap-[3px] text-[13.5px] tablet:w-[180px] tablet:shrink-0">
        <ReadingWord reading={reading} recent={row.health?.recent ?? []} />
        {reading.line && <span className="pl-4 text-[12.5px] text-faint">{reading.line}</span>}
        {row.health?.last_answered_at ? (
          <span className="pl-4 text-[12.5px] text-faint">
            Last answered <Relative iso={row.health.last_answered_at} />
          </span>
        ) : (
          // Until the API carries it: the time of the last check, for a system that is answering.
          !reading.line &&
          answering &&
          row.health?.checked_at && (
            <span className="pl-4 text-[12.5px] text-faint">
              Answered <Relative iso={row.health.checked_at} />
            </span>
          )
        )}
      </span>

      {/* Under the row until there is room for a column. */}
      <span className="order-last flex w-full flex-col items-start gap-1 text-[13.5px] leading-[1.45] desktop:order-none desktop:w-[250px] desktop:shrink-0 desktop:pt-px">
        {row.project === null && <ArrowLink href={href}>Choose its Zitadel project</ArrowLink>}
        {needs.map((need) => (
          <ArrowLink key={need.text} href={need.href}>
            {need.text}
          </ArrowLink>
        ))}
        {needs.length === 0 && row.project !== null && (
          <NeedsNote answering={answering} callable={row.callable} loading={loading} unread={unread} />
        )}
      </span>

      {/* A count, and only when Syndra has one. A system that has never
          answered shows a dash, never 0: that would be a claim about the
          system rather than about Syndra's ignorance of it. Once it stops
          answering, the last count stays and says so. */}
      <span className="flex flex-col gap-0.5 pt-px tablet:w-[110px] tablet:shrink-0 tablet:text-right">
        {bound === undefined ? (
          <span className="text-[15px] text-faint">—</span>
        ) : (
          <>
            <span className={`text-[15px] ${answering ? "" : "text-muted"}`}>
              {bound}
              <span className="text-[13px] text-muted tablet:hidden">
                {bound === 1 ? " person" : " people"}
              </span>
            </span>
            {!answering && <span className="text-[12.5px] text-faint">last known</span>}
          </>
        )}
      </span>

      {/* The one reading that is no fault and so has no Notice: its sentence
          stays under its row. */}
      {reading.detail && (
        <span className="order-last w-full max-w-[80ch] text-[13.5px] leading-[1.5] text-muted">
          {reading.detail}
        </span>
      )}
    </div>
  );
}

function NeedsNote({
  answering,
  callable,
  loading,
  unread,
}: {
  answering: boolean;
  callable: boolean;
  loading: boolean;
  unread: boolean;
}) {
  if (!callable) return <span className="text-faint">—</span>;
  if (!answering) return <span className="text-faint">Unknown until it answers</span>;
  if (loading) return <span className="text-faint">Reading…</span>;
  if (unread) return <span className="text-faint">Not read just now</span>;
  return (
    <span className="inline-flex items-center gap-2 text-muted">
      <span aria-hidden className="size-1.5 rounded-pill bg-healthy" />
      Nothing waiting
    </span>
  );
}

/**
 * The backend ranks what can be true at once (A-01): cannot connect, offline,
 * waiting to retry, checking, answering. One reading here, besides: an add-on
 * registered and not answered yet, which is where every add-on starts. Its
 * sentence stays under the row; the faults' go to a Notice above the card.
 */
function readingFor(row: TargetSummary): Reading & { detail?: string } {
  const name = targetLabel(row.target);
  const reading = targetReading({ status: row.status, health: row.health, name, transportError: row.transport_error });
  if (!row.callable && !(row.status === "cannot_connect" || row.status === "offline" || row.status === "waiting_to_retry")) {
    // Neutral, not amber. Registered-and-not-yet-answered resolves on its own
    // within a refresh interval, and colouring it would cost amber its meaning.
    return {
      tone: "neutral",
      word: "No answer yet",
      pips: false,
      detail: "The add-on is configured and has not answered yet. This usually clears by itself within a minute or two.",
    };
  }
  return reading;
}
