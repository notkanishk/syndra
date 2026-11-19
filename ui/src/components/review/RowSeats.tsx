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
 * WHAT_COL and FIGURE_COL, for its headings and its rows alike.
 */
export const WHO_COL = "w-full tablet:w-[180px] tablet:flex-none";
export const WHAT_COL = "w-full min-w-0 tablet:w-auto tablet:min-w-[160px] tablet:flex-1";
/** When it was found, revoked, or ends: a figure, right-aligned under a right-aligned heading (G16). */
export const FIGURE_COL = "tablet:w-(--seat-number) tablet:flex-none tablet:text-right";

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
        <UserName id={id} href={href ?? accountHref(id)} className="block text-body font-semibold" />
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
