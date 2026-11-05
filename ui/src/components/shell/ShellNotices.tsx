"use client";

import { createContext, useContext, useId, useLayoutEffect, useState } from "react";

import { DegradedBanner } from "@/components/states/DegradedBanner";
import { OfflineBanner } from "@/components/states/OfflineBanner";

/**
 * The shell's own Notices (offline, degraded) and the one seat they sit in
 * (A-20 P3, G7: a Notice sits under the PageHeader, never above it).
 *
 * A page's header mounts `<ShellNotices />` right after itself; the provider
 * lets exactly one outlet claim the seat, so a page with two headers (a
 * loading frame and the real one) never says it twice. A page with no header
 * at all leaves the seat unclaimed and the provider paints the Notices at the
 * top of the content instead, so an outage is never silent. The claim is made
 * in a layout effect, before the first paint.
 *
 * Offline is above degraded because it qualifies it: with no network the mode
 * read cannot refresh either.
 */
type Seat = { owner: string | null; claim: (id: string) => void; release: (id: string) => void };
const SeatContext = createContext<Seat | null>(null);

export function ShellNoticesProvider({ children }: { children: React.ReactNode }) {
  const [owner, setOwner] = useState<string | null>(null);
  const [seat] = useState(() => ({
    claim: (id: string) => setOwner((was) => was ?? id),
    release: (id: string) => setOwner((was) => (was === id ? null : was)),
  }));
  return (
    <SeatContext.Provider value={{ owner, ...seat }}>
      {owner === null && <Notices />}
      {children}
    </SeatContext.Provider>
  );
}

function Notices({ under = false }: { under?: boolean }) {
  return (
    <>
      <OfflineBanner under={under} />
      <DegradedBanner under={under} />
    </>
  );
}

/** The outlet: renders the Notices when it holds the seat, nothing otherwise. */
export function ShellNotices() {
  const seat = useContext(SeatContext);
  const id = useId();
  const owner = seat?.owner;
  const claim = seat?.claim;
  const release = seat?.release;
  useLayoutEffect(() => {
    if (owner === null) claim?.(id);
  }, [owner, claim, id]);
  useLayoutEffect(() => () => release?.(id), [release, id]);
  return owner === id ? <Notices under /> : null;
}
