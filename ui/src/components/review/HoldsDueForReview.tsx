"use client";

import { EmptyState, ListStates } from "@/components/states";
import { Button } from "@/components/ui/Button";
import { Card, CardHeader, CardRow } from "@/components/ui/Card";
import { Relative } from "@/components/ui/Time";
import { UserName } from "@/components/names";
import { targetLabel } from "@/lib/nav";
import { useHoldsDueForReview, useLiftHold } from "@/lib/queries/useHolds";

/**
 * Holds whose review date has passed (9.25/9.26; design §25).
 *
 * Its own queue beside Expiring access, never a row inside it. Inaction means
 * the opposite thing in each: an expiring grant lapses if ignored, and a hold
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
  const lift = useLiftHold();
  const rows = due.data?.due_for_review ?? [];

  return (
    <Card>
      <CardHeader
        title="Holds due for review"
        count={rows.length}
        tone="warn"
        note="Still in force. Nothing here lapses by being listed."
      />
      <ListStates
        isLoading={due.isLoading}
        error={due.error}
        isEmpty={rows.length === 0}
        onRetry={() => due.refetch()}
        errorTitle="The review queue could not be read"
        empty={
          <EmptyState
            title="Nothing due"
            guidance="Every hold in force is inside the window somebody set for it."
          />
        }
      >
        <>
          {rows.map((hold, i) => (
            <CardRow key={hold.id} first={i === 0} className="flex-wrap">
              <span className="text-[14px]">
                <UserName id={hold.subject_id} />
              </span>
              <span className="font-mono text-[13.5px]">{hold.value}</span>
              <span className="text-[13px] text-faint">
                on {targetLabel(hold.target)} · held by <UserName id={hold.actor_id} />{" "}
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
                disabled={lift.isPending}
                onClick={() => lift.mutate(hold.id)}
              >
                Lift it
              </Button>
              <p className="w-full text-[13.5px] text-muted">
                {hold.reason || <span className="text-faint">No reason recorded.</span>}
              </p>
              <p className="w-full text-[13px] text-faint">
                {/* Extending demands a new date, which is why there is no
                    "remind me later" here: a review can be deferred but not
                    dropped, and deferring it is authoring the decision again. */}
                Asked about since <Relative iso={hold.review_date} />. To defer it, lift
                this hold and place a new one with a new date.
              </p>
            </CardRow>
          ))}
        </>
      </ListStates>
    </Card>
  );
}
