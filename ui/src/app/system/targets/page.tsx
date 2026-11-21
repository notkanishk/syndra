"use client";

import Link from "next/link";

import { EmptyState, ListStates, RowSkeleton } from "@/components/states";
import { StatusDot } from "@/components/ui/Badge";
import { ReadingWord, targetReading, type Reading } from "@/components/targets/TargetReading";
import { ArrowLink } from "@/components/ui/ArrowLink";
import { Card, CardColumns, CardRow, NUMBER_COL, type Columns } from "@/components/ui/Card";
import { Notice } from "@/components/ui/Notice";
import { PageHeader } from "@/components/ui/PageHeader";
import { ReadFreshness, readStateOfQuery } from "@/components/ui/ReadFreshness";
import { Relative } from "@/components/ui/Time";
import { versionLabel } from "@/lib/format";
import { dataOf, ofRead, readOfQuery } from "@/lib/queries/read";
import {
  useMergeFindings,
  useTargetHealth,
  useTargetInventory,
  useTargets,
  type TargetSummary,
} from "@/lib/queries/useTargets";
import { projectLabel } from "@/components/names/Unresolved";
import { sortByName } from "@/lib/sort";

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
/**
 * The columns, the headings' and every row's: the system and its status, shares held to 130 each,
 * then its people (`NUMBER_COL`) and the wide seat (Needs you): 130 + 130 + 110 + 300 and the gaps
 * is 718, one line from a 1080 window. The heading's System had no size and the row's held 220, so
 * the row's seat dropped while the heading's did not and STATUS sat 330px off its cell (staging,
 * 19 Nov); D-1's 240 status seat then came to 858 and dropped the seat from 1114 to 1203. The board
 * (route-systems) draws Status 180 and a 250 seat, which D-1's seats (140, 200, 300) do not have:
 * at 1280 each share is 238, past the board's 180.
 */
const SYSTEM_COLS: Columns = [
  "min-w-[min(130px,100%)] flex-1",
  "min-w-[min(130px,100%)] flex-1",
  NUMBER_COL,
];

export default function ConnectedSystemsPage() {
  const targets = useTargets();
  // A to Z, as the board draws it: the roster arrives in registration order.
  const rows = sortByName(targets.data ?? [], (t) => t.name, (t) => t.target
  );

  return (
    <div className="flex flex-col gap-4">
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

      {targets.data && <ReadFreshness state={readStateOfQuery(targets)} subject="This list" />}

      {/* A fault is said once, above the card, and the row keeps its dot and
          word. Where to look is the sentence's job: two of these are faults on
          the Syndra server, not on the system. */}
      {rows.map((row) => (
        <FaultNotice key={row.target} row={row} />
      ))}

      <Card>
        <ListStates
          head={
            <CardColumns cols={SYSTEM_COLS} seat="wide" seatLabel="Needs you">
              <span>
                System <span aria-hidden>↑</span>
              </span>
              <span>Status</span>
              <span>People</span>
            </CardColumns>
          }
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
                "Connecting one is done on the Syndra server; the person who runs it has the steps. " +
                "Until then Syndra manages sign-in only."
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
 * The one Notice a faulted system earns (design §13.1): the add-on down, or
 * its app down, named apart; red when the fault is Syndra's own (a secret it
 * cannot load). Not checked yet is no fault and gets none: it clears by itself.
 */
function FaultNotice({ row }: { row: TargetSummary }) {
  const name = row.name;
  const reading = readingFor(row);
  if (row.status !== "addon_unreachable" && row.status !== "app_unreachable") return null;
  return (
    <Notice
      tone={reading.tone === "danger" ? "danger" : "warn"}
      live
      title={row.status === "addon_unreachable" ? `${name}'s add-on isn't answering.` : `${name} isn't answering.`}
      next="Nothing is lost: Syndra sends what waits once it answers."
    >
      {reading.sentence}
    </Notice>
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
 *
 * Product, People and Needs you are read from the system's own page reads —
 * health, inventory and findings — so the index and the page cannot disagree.
 * A system that has never answered is not asked.
 */
function TargetRow({ row, first }: { row: TargetSummary; first: boolean }) {
  const reading = readingFor(row);
  const name = row.name;
  const ask = row.callable ? row.target : undefined;
  const health = useTargetHealth(ask);
  const inventory = useTargetInventory(ask);
  const findings = useMergeFindings(ask);

  const answering = reading.tone === "healthy";
  const read = readOfQuery(inventory);
  // A count only from a read (design §13.6): a read with nothing to give is "Not read", never 0.
  const bound = ofRead(read, (d) => d.bound);
  const unread = read.state === "none";
  // The product and its version, whatever the operator named the system.
  const said = health.data ? dataOf(health.data.addon) : undefined;
  const product = said?.product ? `${said.product} ${versionLabel(said.product, said.product_version)}`.trim() : null;
  const href = `/system/targets/${row.target}`;

  // What is waiting, each linking to where it is handled on the system's page.
  const unmanaged = ofRead(read, (d) => d.unmanaged?.length) ?? 0;
  const changed = findings.data?.length ?? 0;
  const decisions = health.data?.log_anchor?.violation_reason ? 1 : 0;
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
  const readFailed = Boolean(ask) && (health.isError || inventory.isError || findings.isError);

  return (
    <CardRow
      first={first}
      cols={SYSTEM_COLS}
      seat="wide"
      top
      // The one reading that is no fault and so has no Notice: its sentence is the row's detail line.
      detail={reading.detail}
      action={
        // What needs you is the row's way on (D-1 §4): its seat, under the "Needs you" heading. A link per
        // thing, or the one line saying nothing does.
        <span className="flex flex-col items-end gap-1 text-right text-small leading-[1.45]">
          {row.project === null && <ArrowLink href={href}>Choose its Zitadel project</ArrowLink>}
          {needs.map((need) => (
            <ArrowLink key={need.text} href={need.href}>
              {need.text}
            </ArrowLink>
          ))}
          {needs.length === 0 && row.project !== null && (
            <NeedsNote answering={answering} callable={row.callable} loading={loading} readFailed={readFailed} />
          )}
        </span>
      }
    >
      <span className="flex flex-col gap-0.5">
        <Link href={href} className="text-body font-semibold link-hover">
          {name}
        </Link>
        <span className={`text-small ${product ? "text-muted" : "text-faint"}`}>
          {product ?? "Not read yet"}
        </span>
        {/* The project it serves, read-only: it is set in one place, the
            system's own page. Absent is the row's Needs you. */}
        {row.project ? (
          <span className="text-small text-muted">
            Zitadel project{" "}
            <Link href={`/projects/${row.project.id}`} className="text-ink link-hover">
              {projectLabel(row.project.name)}
            </Link>
          </span>
        ) : row.project === null ? (
          <span className="text-small text-faint">No Zitadel project yet</span>
        ) : null}
      </span>

      {/* A dot and a word, the same idiom the target's own page uses — never
          colour alone — with the check pips when a check was missed, and the
          time it began faint under it. */}
      <span className="flex flex-col gap-[3px] text-small">
        <ReadingWord reading={reading} recent={row.health?.recent ?? []} />
        {reading.line && <span className="pl-(--dot-indent) text-fine text-faint">{reading.line}</span>}
        {row.health?.last_answered_at ? (
          <span className="pl-(--dot-indent) text-fine text-faint">
            Last answered <Relative iso={row.health.last_answered_at} />
          </span>
        ) : (
          // Until the API carries it: the time of the last check, for a system that is answering.
          !reading.line &&
          answering &&
          row.health?.checked_at && (
            <span className="pl-(--dot-indent) text-fine text-faint">
              Answered <Relative iso={row.health.checked_at} />
            </span>
          )
        )}
      </span>

      {/* A count, and only from a read, never 0 in its place: that would be a
          claim about the system rather than about Syndra's ignorance of it. A
          read with nothing to give says "Not read"; a system not read yet says
          "Not yet". */}
      <span data-figure className="flex flex-col gap-0.5 pt-px">
        {unread ? (
          <span className="text-body text-faint">Not read</span>
        ) : bound === undefined ? (
          <span className="text-body text-faint">Not yet</span>
        ) : (
          <span className={`text-body ${answering ? "" : "text-muted"}`}>
            {bound}
            <span className="text-small text-muted tablet:hidden">
              {bound === 1 ? " person" : " people"}
            </span>
          </span>
        )}
      </span>

    </CardRow>
  );
}

function NeedsNote({
  answering,
  callable,
  loading,
  readFailed,
}: {
  answering: boolean;
  callable: boolean;
  loading: boolean;
  readFailed: boolean;
}) {
  if (!callable) return <span className="text-faint">Not yet</span>;
  if (!answering) return <span className="text-faint">Unknown until it answers</span>;
  if (loading) return <span className="text-faint">Reading…</span>;
  if (readFailed) return <span className="text-faint">Not read just now</span>;
  return (
    <span className="inline-flex items-center gap-2 text-muted">
      <StatusDot tone="healthy" />
      Nothing waiting
    </span>
  );
}

/**
 * The monitor's state (design §13.2): the add-on down, its app down, checking,
 * answering. One reading here, besides: an add-on
 * registered and not answered yet, which is where every add-on starts. Its
 * sentence stays under the row; the faults' go to a Notice above the card.
 */
function readingFor(row: TargetSummary): Reading & { detail?: string } {
  const name = row.name;
  const reading = targetReading({ status: row.status, health: row.health, name });
  if (!row.callable && row.status !== "addon_unreachable" && row.status !== "app_unreachable") {
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
