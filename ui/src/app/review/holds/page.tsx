"use client";

import { HoldsDueForReview } from "@/components/review/HoldsDueForReview";
import { PageHeader } from "@/components/ui/PageHeader";

/**
 * Review › Holds due.
 *
 * Its own destination beside Expiring access rather than a section inside it.
 * The two queues read the same way and mean opposite things: an expiring grant
 * lapses if nobody acts, and a hold stays in force. Sitting them in one list
 * would put "do nothing and access ends" next to "do nothing and access stays
 * blocked", under one heading, for an operator working down it with one mental
 * model.
 */
export default function HoldsDuePage() {
  return (
    <>
      <PageHeader
        title="Holds due"
        meta="Access somebody withheld on purpose, past the date they said they would look at it again."
      />
      <HoldsDueForReview />
    </>
  );
}
