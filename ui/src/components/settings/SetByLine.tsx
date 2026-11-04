"use client";

import { UserName } from "@/components/names/UserName";
import { formatShortDate } from "@/lib/format";

/** The foot of a settings card: who last set it and when, or what it is until someone does. */
export function SetByLine({ by, at, never }: { by?: string | null; at?: string | null; never: string }) {
  return (
    <p className="row-divider px-5 py-3 text-[13px] text-faint">
      {by && at ? (
        <>
          Set by <UserName id={by} fallback="Someone" /> on {formatShortDate(at)}.
        </>
      ) : (
        never
      )}
    </p>
  );
}
