"use client";

import { MetaRow } from "@/components/ui/PageHeader";
import { Term } from "@/components/ui/Term";
import { HoldsDueForReview } from "@/components/review/HoldsDueForReview";
import { PageHeader } from "@/components/ui/PageHeader";
import { formatShortDate } from "@/lib/format";
import { useHoldsDueForReview } from "@/lib/queries/useHolds";

/**
 * Review › Holds due.
 *
 * Its own destination beside Expiring access rather than a section inside it.
 * The two queues read the same way and mean opposite things: expiring access
 * lapses if nobody acts, and a hold stays in force. Sitting them in one list
 * would put "do nothing and access ends" next to "do nothing and access stays
 * blocked", under one heading, for an operator working down it with one mental
 * model.
 */
export default function HoldsDuePage() {
  // The list's own read, shared with the list below it (one query, one answer).
  const due = useHoldsDueForReview();
  const rows = due.data?.due_for_review ?? [];
  const oldest = rows
    .map((hold) => hold.review_date)
    .filter((date): date is string => Boolean(date))
    .sort()[0];

  return (
    <div className="flex flex-col gap-[18px]">
      <PageHeader
        title="Holds due"
        meta={
          rows.length > 0 ? (
            <MetaRow>
              {[
                `${rows.length} ${rows.length === 1 ? "hold" : "holds"}`,
                oldest && `the oldest due ${formatShortDate(oldest)}`,
              ]}
            </MetaRow>
          ) : undefined
        }
        lede={
          <>
            <Term name="hold">Holds</Term> whose review date has passed. Doing nothing keeps the
            access blocked: being listed here ends nothing. Lifting clears the block in Syndra at
            once, and the access returns when the connected system is next brought in line. To keep
            a hold with a later date, lift it and place a new one from the person&apos;s page.
          </>
        }
      />
      <HoldsDueForReview />
    </div>
  );
}
