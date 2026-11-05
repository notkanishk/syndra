"use client";


import { UserName } from "@/components/names/UserName";
import { Crossfade } from "@/lib/motion";
import { StatusSeat } from "@/components/ui/StatusSeat";

/** The person who answers for it, linked; or a warn dot and "No owner" (A-07 §31). */
export function Owner({ id }: { id: string }) {
  // Handed to somebody else, or taken off them, while you look: the words crossfade (S4).
  return (
    <Crossfade id={id || "none"} className="justify-items-start">
      {id ? (
        <UserName id={id} />
      ) : (
        <StatusSeat tone="warn" word="No owner" />
      )}
    </Crossfade>
  );
}
