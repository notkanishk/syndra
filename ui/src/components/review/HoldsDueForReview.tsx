"use client";

import { useState } from "react";

import { EmptyState, ListStates } from "@/components/states";
import { ActionOutcome } from "@/components/ui/ActionOutcome";
import { Button } from "@/components/ui/Button";
import { Card, CardHeader, CardRow } from "@/components/ui/Card";
import { Relative } from "@/components/ui/Time";
import { UserName } from "@/components/names";
import { targetLabel } from "@/lib/nav";
import { outcomeFromError, type ActionOutcome as Outcome } from "@/lib/outcome";
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

  return (
    <Card>
      <CardHeader
        title="Holds due"
        count={rows.length}
        tone="warn"
        note="These holds are still blocking access. Being listed here does not end them — only lifting does. Lifting clears the block in Syndra at once; the access itself returns when the connected system is next brought in line."
      />
      <ListStates
        isLoading={due.isLoading}
        error={due.error}
        isEmpty={rows.length === 0}
        onRetry={() => due.refetch()}
        errorTitle="Couldn't load holds due."
        empty={
          <EmptyState
            title="Nothing due"
            guidance="No hold has reached its review date. Holds appear here on the date the person who placed them chose."
          />
        }
      >
        <>
          {rows.map((hold, i) => (
            <HoldRow key={hold.id} hold={hold} first={i === 0} />
          ))}
        </>
      </ListStates>
    </Card>
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

function HoldRow({ hold, first }: { hold: Hold; first: boolean }) {
  const lift = useLiftHold();
  const [outcome, setOutcome] = useState<Outcome | null>(null);
  const label = holdLabel(hold);

  return (
    <CardRow first={first} className="flex-wrap">
      <span className="text-[14px]">
        <UserName id={hold.subject_id} />
      </span>
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
            setOutcome({
              kind: "applied",
              message: `Hold lifted. ${label} returns when the connected system is next brought in line.`,
            });
          } catch (error) {
            setOutcome(outcomeFromError(error));
          }
        }}
      >
        Lift the hold on {label}
      </Button>
      <p className="w-full text-[13.5px] text-muted">
        {hold.reason || <span className="text-faint">No reason recorded.</span>}
      </p>
      <p className="w-full text-[13px] text-faint">
        {/* Extending demands a new date, which is why there is no
            "remind me later" here: a review can be deferred but not
            dropped, and deferring it is authoring the decision again. */}
        Review date passed <Relative iso={hold.review_date} />. To push it back, lift this
        hold and place a new one with a new date from their page.
      </p>
      {outcome && <ActionOutcome outcome={outcome} placement="inline" className="w-full" />}
    </CardRow>
  );
}
