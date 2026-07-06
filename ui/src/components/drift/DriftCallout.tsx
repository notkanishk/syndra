"use client";

import Link from "next/link";
import { useEffect, useRef, useState } from "react";

import { ProjectName } from "@/components/names/ProjectName";
import { UserName } from "@/components/names/UserName";
import type { DriftItem } from "@/lib/queries/useGovernance";

interface Props {
  count: number;
  top: DriftItem[];
}

/**
 * Full-width, UNDISMISSIBLE dashboard callout above the stat grid. Red, with a
 * top-3 preview and "Triage all →". No dismiss control — drift must not be
 * silenced. Contrast with the dismissible amber PendingCallout.
 */
export function DriftCallout({ count, top }: Props) {
  const prevCount = useRef(count);
  const [countRose, setCountRose] = useState(false);

  useEffect(() => {
    if (count > prevCount.current) setCountRose(true);
    prevCount.current = count;
  }, [count]);

  if (count <= 0) return null;
  return (
    <div
      role="alert"
      className="rounded-card border border-error/50 bg-[color-mix(in_srgb,var(--error)_15%,transparent)] px-5 py-4"
    >
      <div className="flex items-center justify-between gap-4">
        <span className="text-on-surface">
          <span aria-hidden>⚠ </span>
          <strong
            className={countRose ? "inline-block motion-safe:animate-count-emphasis" : "inline-block"}
            onAnimationEnd={() => setCountRose(false)}
          >
            {count}
          </strong>{" "}
          out-of-band {count === 1 ? "change needs" : "changes need"} triage
        </span>
        <Link href="/governance/drift" className="font-medium text-error hover:underline">
          Triage all →
        </Link>
      </div>
      <ul className="mt-3 space-y-1 text-sm text-on-surface-variant">
        {top.slice(0, 3).map((d) => (
          <li key={d.id}>
            <UserName id={d.user_id} /> · <ProjectName id={d.project_id} /> · {d.role_keys.join(", ")}
            <span className="ml-2 text-xs opacity-70">({d.drift_type})</span>
          </li>
        ))}
      </ul>
    </div>
  );
}
