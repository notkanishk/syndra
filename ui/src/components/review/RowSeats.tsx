"use client";

import Link from "next/link";
import type { ReactNode } from "react";

import { ProjectName, UserAvatar, UserName } from "@/components/names";
import { ServiceAccountTag, useAccountHref } from "@/components/system/ServiceAccountTag";
import { Mono } from "@/components/ui/Badge";
import { humanizeKey } from "@/lib/format";

/**
 * The who and what seats every review queue shares (A-17 §2.19: Drift, Unfinished revocations and
 * Expiring each drew their own, at 14.5 and 15px, as "Project · Role" and "Project / Role").
 * Who: avatar, a name that opens the person's page, one faint line under it. What: the project and
 * the role's words as one link, the role keys under it.
 *
 * Their widths are the table's columns, not theirs: a queue's `Columns` list takes WHO_COL,
 * WHAT_COL and NUMBER_COL (when), for its headings and its rows alike.
 *
 * Both are shares held to a minimum, What twice Who's: a fixed 180 for Who and 160 for What, with
 * the date and the wide seat, came to 798px, so on Drift the seat dropped under every row from
 * 1024 to 1240 although the rows fitted (staging, 19 Nov). Now 130 + 90 + 110 + 300 and the gaps
 * is 678; with Select's checkbox column (26 and a gap) 720, inside Drift's narrowest desktop row
 * (728 at 1080, less its 3px edge). At 140 + 120 the select-mode line was 760, and its seat
 * dropped under every row from 1080 to 1110 (measured).
 */
export const WHO_COL = "w-full min-w-0 tablet:min-w-[130px] tablet:flex-1";
export const WHAT_COL = "w-full min-w-0 tablet:min-w-[90px] tablet:flex-[2_1_0%]";

/**
 * A queue row's left edge, 3px, unpainted: a row paints it when it says something (a safety-gated
 * finding, the soonest to end), and the headings take it too, so every cell stays under its heading.
 */
export const EDGE = "border-l-[3px] border-transparent";

export function WhoSeat({
  id,
  href,
  sub,
}: {
  id: string;
  /** Only where the row already knows the account is a service account. */
  href?: string;
  /** The faint line under the name; a service account says so when none is given. */
  sub?: ReactNode;
}) {
  const accountHref = useAccountHref();
  return (
    <div className="flex w-full min-w-0 items-center gap-3">
      <UserAvatar id={id} size="row" />
      <span className="min-w-0">
        <UserName id={id} href={href ?? accountHref(id)} className="block truncate text-body font-semibold" />
        {sub ? <span className="block text-fine text-faint">{sub}</span> : <ServiceAccountTag id={id} />}
      </span>
    </div>
  );
}

export function WhatSeat({
  projectId,
  roleKeys,
  fallback,
  children,
}: {
  /** Absent for an add-on's revocation, which has no project: `fallback` says what it is instead. */
  projectId?: string;
  roleKeys: string[];
  fallback?: ReactNode;
  /** Anything else the row says about it: a system, a risk pill. */
  children?: ReactNode;
}) {
  return (
    <div className="flex w-full min-w-0 flex-col items-start gap-[3px]">
      {projectId ? (
        <Link href={`/projects/${projectId}`} className="text-body leading-[1.45] link-hover">
          <ProjectName id={projectId} link={false} />
          {roleKeys.length > 0 && ` · ${roleKeys.map(humanizeKey).join(", ")}`}
        </Link>
      ) : (
        <span className="text-body leading-[1.45]">{fallback}</span>
      )}
      <span className="flex flex-wrap gap-x-3 gap-y-1">
        {roleKeys.map((key) => (
          <Mono key={key} className="text-fine text-faint">
            {key}
          </Mono>
        ))}
      </span>
      {children}
    </div>
  );
}
