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
 */
export function WhoSeat({
  id,
  href,
  sub,
  className = "tablet:w-[180px] tablet:shrink-0",
}: {
  id: string;
  /** Only where the row already knows the account is a service account. */
  href?: string;
  /** The faint line under the name; a service account says so when none is given. */
  sub?: ReactNode;
  className?: string;
}) {
  const accountHref = useAccountHref();
  return (
    <div className={`flex w-full min-w-0 items-center gap-3 ${className}`}>
      <UserAvatar id={id} size="row" />
      <span className="min-w-0">
        <UserName id={id} href={href ?? accountHref(id)} className="block text-[14.5px] font-semibold" />
        {sub ? <span className="block text-[12.5px] text-faint">{sub}</span> : <ServiceAccountTag id={id} />}
      </span>
    </div>
  );
}

export function WhatSeat({
  projectId,
  roleKeys,
  fallback,
  children,
  className = "tablet:w-[200px] tablet:shrink-0",
}: {
  /** Absent for an add-on's revocation, which has no project: `fallback` says what it is instead. */
  projectId?: string;
  roleKeys: string[];
  fallback?: ReactNode;
  /** Anything else the row says about it: a system, a risk pill. */
  children?: ReactNode;
  className?: string;
}) {
  return (
    <div className={`flex w-full min-w-0 flex-col items-start gap-[3px] ${className}`}>
      {projectId ? (
        <Link href={`/projects/${projectId}`} className="text-[14px] leading-[1.45] link-hover">
          <ProjectName id={projectId} link={false} />
          {roleKeys.length > 0 && ` · ${roleKeys.map(humanizeKey).join(", ")}`}
        </Link>
      ) : (
        <span className="text-[14px] leading-[1.45]">{fallback}</span>
      )}
      <span className="flex flex-wrap gap-x-3 gap-y-1">
        {roleKeys.map((key) => (
          <Mono key={key} className="text-[12.5px] text-faint">
            {key}
          </Mono>
        ))}
      </span>
      {children}
    </div>
  );
}
