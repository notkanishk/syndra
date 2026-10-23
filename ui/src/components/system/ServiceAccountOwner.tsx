"use client";

import Link from "next/link";

import { UserName } from "@/components/names/UserName";
import { StatusDot } from "@/components/ui/Badge";
import { Crossfade } from "@/components/ui/Crossfade";

/** The person who answers for it, linked; or a warn dot and "No owner" (A-07 §31). */
export function Owner({ id }: { id: string }) {
  // Handed to somebody else, or taken off them, while you look: the words crossfade (S4).
  return (
    <Crossfade id={id || "none"} className="justify-items-start">
      {id ? (
        <Link href={`/users/${encodeURIComponent(id)}`} className="hover:text-accent-text hover:underline">
          <UserName id={id} />
        </Link>
      ) : (
        <span className="inline-flex items-center gap-2 font-semibold text-warn-text">
          <StatusDot tone="warn" />
          No owner
        </span>
      )}
    </Crossfade>
  );
}
