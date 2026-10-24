"use client";

import Link from "next/link";
import { useState } from "react";

import { EmptyState, ListStates } from "@/components/states";
import { ServiceAccountTag, useAccountHref } from "@/components/system/ServiceAccountTag";
import { Crossfade } from "@/components/ui/Crossfade";
import { ActionOutcome } from "@/components/ui/ActionOutcome";
import { ArrowLink } from "@/components/ui/ArrowLink";
import { Button } from "@/components/ui/Button";
import { Card, CardRow } from "@/components/ui/Card";
import { LeavingRow, useLeavingRows } from "@/components/ui/LeavingRows";
import { ReadFreshness } from "@/components/ui/ReadFreshness";
import { Relative } from "@/components/ui/Time";
import { UserName } from "@/components/names";
import { targetLabel } from "@/lib/nav";
import { NOTHING_CHANGED, outcomeFromError, sentence, type ActionOutcome as Outcome } from "@/lib/outcome";
import { useHoldsDueForReview, useLiftHold, type Hold } from "@/lib/queries/useHolds";

/**
 * Holds whose review date has passed (9.25/9.26; design §25).
 *
 * Its own queue beside Expiring access, never a row inside it. Inaction means
 * the opposite thing in each: expiring access lapses if ignored, and a hold
 * STAYS IN FORCE. One list would sit "do nothing and access ends" next to "do
 * nothing and access stays blocked", and an operator working down it would be
 * applying one mental model to two opposite objects.
 *
 * Surfacing is a prompt, never a lapse. Nothing here expires by being listed —
 * ending a hold because nobody looked at it would restore access by inattention,
 * which is the failure a review date exists to prevent, running backwards.
 */
export function HoldsDueForReview() {
  const due = useHoldsDueForReview();
  const rows = due.data?.due_for_review ?? [];
  // S5: a hold that leaves the list on a later read (lifted, kept or ended elsewhere) fades where
  // it was and closes its gap after, and not under the pointer.
  const leaving = useLeavingRows(rows, (hold) => hold.id, !due.isLoading && !due.error);

  return (
    <div className="flex flex-col gap-3.5">
      {/* The page title already says "Holds due" and counts them, so the card has
          no heading of its own. The read line is for a list, not for its absence. */}
      {rows.length > 0 && !due.isLoading && !due.error && (
        <div className="px-5">
          <ReadFreshness
            state={{ readAt: new Date(due.dataUpdatedAt).toISOString(), current: true }}
            subject="This list"
          />
        </div>
      )}
      <Card {...leaving.listProps}>
        <ListStates
          isLoading={due.isLoading}
          error={due.error}
          isEmpty={rows.length === 0}
          onRetry={() => due.refetch()}
          errorTitle="Couldn't load holds due."
          empty={
            <EmptyState
              resolved
              title="No hold is due for review."
              guidance="A hold appears here on the review date the person who placed it chose. Until then it stays in force without asking."
            />
          }
        >
          <>
            {leaving.rows.map((hold, i) => (
              <LeavingRow key={hold.id} {...leaving.props(hold.id)}>
                {hold.direction === "set" ? (
                  <OwnValueRow hold={hold} first={i === 0} />
                ) : (
                  <HoldRow hold={hold} first={i === 0} />
                )}
              </LeavingRow>
            ))}
          </>
        </ListStates>
      </Card>
    </div>
  );
}

/** Whose hold it is. A service account says so where a person's title would be, and its name opens its own page (A-07 §29). */
function Subject({ id }: { id: string }) {
  const accountHref = useAccountHref();
  return (
    <span className="text-[14px]">
      <Link href={accountHref(id)} className="motion-tint hover:text-accent-text">
        <UserName id={id} />
      </Link>
      <ServiceAccountTag id={id} />
    </span>
  );
}

/**
 * What a person calls the thing a hold blocks. The value the resolver reads is
 * sent verbatim — "true" for a whole account — and is the wrong thing to put
 * in a sentence (see `HoldDialog`).
 */
function holdLabel(hold: Hold): string {
  if (hold.field === "enabled") return `their ${targetLabel(hold.target)} account`;
  if (hold.field === "share") return `the ${hold.value} share`;
  if (hold.field === "group") return `the ${hold.value} group`;
  return `${hold.field} ${hold.value}`;
}

/**
 * One person's own value due for review: a home folder limit in place of what
 * their roles give. Read-only here; it is kept or ended on the system's page,
 * its one write place (A-03 §3).
 */
function OwnValueRow({ hold, first }: { hold: Hold; first: boolean }) {
  const system = targetLabel(hold.target);
  const bytes = Number(hold.value);
  const limit = !bytes ? "no limit" : `${(bytes / 1024 ** 3).toFixed(bytes % 1024 ** 3 ? 1 : 0)} GB`;
  return (
    <CardRow first={first} className="flex-wrap">
      <Subject id={hold.subject_id} />
      <span className="text-[13.5px]">their own home folder limit, {limit}</span>
      <span className="text-[13px] text-faint">
        on {system} · set by <UserName id={hold.actor_id} /> <Relative iso={hold.created_at} />
      </span>
      <span className="flex-1" />
      <ArrowLink href={`/system/targets/${hold.target}`} className="whitespace-nowrap">
        Review it on {system}&apos;s page
      </ArrowLink>
      <p className="w-full text-[13.5px] text-muted">
        {hold.reason || <span className="text-faint">No reason recorded.</span>}
      </p>
      <p className="w-full text-[13px] text-faint">
        Review date passed <Relative iso={hold.review_date} />. It stays at {limit} until it is kept or ended.
      </p>
    </CardRow>
  );
}

function HoldRow({ hold, first }: { hold: Hold; first: boolean }) {
  const lift = useLiftHold();
  const [outcome, setOutcome] = useState<Outcome | null>(null);
  const label = holdLabel(hold);

  return (
    <CardRow first={first} className="flex-wrap">
      <Subject id={hold.subject_id} />
      <span className="text-[13.5px]">{label}</span>
      <span className="text-[13px] text-faint">
        on {targetLabel(hold.target)} · placed by <UserName id={hold.actor_id} />{" "}
        <Relative iso={hold.created_at} />
      </span>
      <span className="flex-1" />
      {/* Lifting is rung 1: it restores access a role already grants,
          and doing the opposite — holding it again — is one click away
          on the person's own page.

          Rung 1 is about the CEREMONY, not about whether the control is
          visible. `ghost` is the quieter half of a pair, and this row
          has no pair — drawn borderless it read as text until hovered,
          which on a phone is never. */}
      {/* Lifted and "couldn't lift it" take the seat the button held (route-holds:
          lifted, lift-failed). The row stays until the next read, so nothing moves
          under the hand; a failed lift keeps the button, as Try again. */}
      <Crossfade id={outcome?.kind === "applied" ? "outcome" : "open"}>
      {outcome?.kind === "applied" ? (
        <ActionOutcome
          placement="seat"
          word="Lifted"
          outcome={{
            kind: "applied",
            message: "",
            detail: `${label[0].toUpperCase()}${label.slice(1)} returns when the connected system is next brought in line.`,
          }}
        />
      ) : (
        <span className="flex flex-col items-start gap-2">
          {outcome && (
            <ActionOutcome placement="seat" className="tint-in" word="Couldn't lift it" outcome={outcome} />
          )}
          <Button
            variant="outline"
            size="sm"
            isPending={lift.isPending}
            onClick={async () => {
              try {
                await lift.mutateAsync(hold.id);
                // Not "they have it again": lifting is one column in Syndra
                // (`handleLiftAllowance` → `dbLiftAllowance`, HTTP 200 and
                // return). Nothing queues a convergence, so the account on the
                // connected system is still without the access until the next
                // pass. Reporting the stronger thing sent an operator away
                // believing somebody could work.
                setOutcome({ kind: "applied", message: "" });
              } catch (error) {
                const failed = outcomeFromError(error);
                setOutcome({
                  ...failed,
                  message:
                    failed.kind === "failed"
                      ? NOTHING_CHANGED
                      : `${sentence(failed.message)} ${NOTHING_CHANGED}`,
                });
              }
            }}
          >
            {outcome ? "Try again" : `Lift the hold on ${label}`}
          </Button>
        </span>
      )}
      </Crossfade>
      <p className="w-full text-[13.5px] text-muted">
        {hold.reason || <span className="text-faint">No reason recorded.</span>}
      </p>
      <p className="w-full text-[13px] text-faint">
        Review date passed <Relative iso={hold.review_date} />.
      </p>
    </CardRow>
  );
}
