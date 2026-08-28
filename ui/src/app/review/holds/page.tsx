"use client";

import { HoldsDueForReview } from "@/components/review/HoldsDueForReview";
import { PageHeader } from "@/components/ui/PageHeader";

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
  return (
    <>
      <PageHeader
        title="Holds due"
        lede="Holds (a block on someone's access, with a date to look at it again) whose review date has passed. Doing nothing keeps the access blocked."
      />
      <HoldsDueForReview />
    </>
  );
}
