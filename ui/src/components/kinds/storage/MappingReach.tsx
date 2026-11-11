"use client";

import { Fragment } from "react";

import { StatusDot } from "@/components/ui/Badge";
import { Relative } from "@/components/ui/Time";
import { formatClock } from "@/lib/format";
import { limitWords } from "@/lib/role-mapping-plan";
import type { ReachPlace, TargetReach } from "@/lib/queries/useTargets";

/**
 * What a storage system's groups open, on the mappings screen and under the
 * picker (A-02 §3), and the home folder line under a mapping (A-03 §2). Storage
 * words, so they live in the storage kind; the screen renders these.
 */

/** Said in the remove-mapping dialog when the role gave a home folder. */
export const HOMES_STAY = " New home folders stop; the ones already made stay, with their files.";

/**
 * The freshness of what each group opens, said once in the card header
 * (A-02 §3). The mappings themselves are Syndra's own record and need none, so
 * the card never recedes; when the read is the last one, only this note turns
 * amber with a ring (A-13 §2).
 */
export function OpensNote({ name, reach }: { name: string; reach: TargetReach | undefined }) {
  if (!reach) return null;
  if (!reach.read) return <>What each group opens: not read from {name} yet</>;
  return reach.current ? (
    <>
      What each group opens, read from {name}&apos;s folder permissions <Relative iso={reach.read_at} />
    </>
  ) : (
    <span className="inline-flex items-baseline gap-1.5 text-warn-text">
      <StatusDot tone="warn" ring className="self-center" />
      <span>
        What each group opens, as last read from {name}&apos;s folder permissions at {formatClock(reach.read_at)}
      </span>
    </span>
  );
}

/**
 * What a group opens, under the picker, so the operator sees what a mapping
 * gives before the preview (A-02 §3).
 */
export function OpensLine({ name, places }: { name: string; places: ReachPlace[] | undefined }) {
  if (!places) return null;
  if (places.length === 0) {
    return <span className="text-[13.5px] text-warn-text">Opens nothing on {name} yet.</span>;
  }
  return (
    <span className="text-[13.5px] leading-[1.5] text-ink [text-wrap:pretty]">
      Opens{" "}
      {places.map((p, i) => (
        <Fragment key={p.path}>
          {i > 0 && (i === places.length - 1 ? ", and " : ", ")}
          <span className="type-mono text-[12.5px]">{placeName(p)}</span>,{" "}
          {p.access === "change" ? "can change files" : "read only"}
        </Fragment>
      ))}
      .
    </span>
  );
}

/** `main › waddle_share`. */
export function placeName(p: ReachPlace): string {
  return [p.share, ...(p.folder ? p.folder.split("/") : [])].join(" › ");
}

/** One mapping's reach, a row per place, or the warning that it opens nothing. */
export function OpensRows({
  name,
  group,
  places,
  lastKnown,
  holders,
}: {
  name: string;
  group: string;
  places: ReachPlace[];
  /** Last known, not a fresh read: the lines stay full strength, their dot rings (A-13). */
  lastKnown: boolean;
  /** How many people hold the role; undefined while it is being read. */
  holders: number | undefined;
}) {
  return (
      <span className="flex flex-col gap-0.5">
        {places.length === 0 ? (
          <span className="flex items-baseline gap-2 text-[13px] leading-[1.5] text-warn-text [text-wrap:pretty]">
            <StatusDot tone="warn" ring={lastKnown} />
            <span>
              Opens nothing yet. No folder on {name} gives {group} anything, so its{" "}
              {holders === undefined ? "" : `${holders} `}
              {holders === 1 ? "person gets" : "people get"} an account and nowhere to connect.{" "}
              <span className="text-ink">Look at the folder&apos;s permissions on {name}.</span>
            </span>
          </span>
        ) : (
          places.map((place, i) => (
            <span key={place.path} className="flex flex-wrap items-baseline gap-x-2 gap-y-0.5 text-[13px] leading-[1.5] text-muted">
              <span className="text-faint">{i === 0 ? "Opens" : "and"}</span>
              <span className="type-mono text-[12.5px] text-ink">{placeName(place)}</span>
              <span>{place.access === "change" ? "can change files" : "read only"}</span>
            </span>
          ))
        )}
      </span>
  );
}

/** "Home folder, up to 20 GB, in main › homes", read only: its one write place is the dialog (A-03 §2). */
export function MappingHomeLine({ quota, homeRoot }: { quota: string | undefined; homeRoot: string }) {
  return (
    <span className="text-[13px] leading-[1.5] text-muted">
      Home folder, {limitWords(quota)}
      {homeRoot && (
        <>
          , in <span className="type-mono text-[12.5px] text-ink">{homeRoot}</span>
        </>
      )}
    </span>
  );
}
