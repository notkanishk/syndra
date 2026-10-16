"use client";

import type { ReactNode } from "react";

import { BundleName, UserName } from "@/components/names";
import { ArrowLink } from "@/components/ui/ArrowLink";
import { StatusDot, type StatusTone } from "@/components/ui/Badge";
import { Card, CardHeader, CardHeaderLink } from "@/components/ui/Card";
import { formatShortDate } from "@/lib/format";
import type { MyChange } from "@/lib/queries/useMyChanges";

/**
 * Recent changes (route-me 52c): the last 30 days of your own access, newest
 * first, leading the page. The caller renders it only when there is something
 * in it: on a quiet month the page opens on your access.
 */
export function RecentChanges({
  items,
  roleName,
}: {
  items: MyChange[];
  roleName: (projectId: string, roleKey: string) => string;
}) {
  return (
    <Card>
      <CardHeader
        title="Recent changes"
        note="Last 30 days"
        action={<CardHeaderLink href="/requests">All your requests →</CardHeaderLink>}
      />
      {items.map((change, i) => (
        <ChangeRow key={`${change.kind}-${change.at}-${change.project_id}-${change.role_key}-${i}`} change={change} role={roleName(change.project_id, change.role_key)} />
      ))}
    </Card>
  );
}

/** An approval that isn't on your account yet, and why, in the member's words (N19). */
export function waitClause(waitReason: string | undefined): string {
  return waitReason?.startsWith("halted:")
    ? "Syndra couldn't reach the sign-in system when it was approved; staff will send it."
    : "makerspace staff check this role before it reaches you, and nothing more is needed from you.";
}

export function askHref(projectId: string, roleKey: string, extend = false): string {
  return `/requests?project=${encodeURIComponent(projectId)}&role=${encodeURIComponent(roleKey)}${extend ? "&extend=1" : ""}`;
}

/** "Today", otherwise "8 Oct". */
export function dayLabel(iso: string, now: Date = new Date()): string {
  return new Date(iso).toDateString() === now.toDateString() ? "Today" : formatShortDate(iso);
}

function ChangeRow({ change, role }: { change: MyChange; role: string }) {
  const { word, tone, pre, post } = describe(change);
  return (
    <div className="row-divider flex flex-wrap items-baseline gap-x-4 gap-y-1.5 px-5 py-3.5">
      <span
        className={`inline-flex w-[118px] flex-none items-center gap-2 text-[13.5px] font-semibold ${
          tone === "danger" ? "text-danger-text" : "text-ink"
        }`}
      >
        <StatusDot tone={tone} />
        {word}
      </span>
      <span className="flex min-w-0 flex-[1_1_260px] flex-col gap-[5px]">
        <span className="text-[14.5px] leading-[1.5] [text-wrap:pretty]">
          {pre}
          <strong className="font-semibold">{role}</strong>
          {post}
        </span>
        {change.note && (
          <span className="text-[14px] leading-[1.55] [text-wrap:pretty]">
            &ldquo;{change.note}&rdquo;
            {change.by && (
              <span className="text-muted">
                {" "}
                — <UserName id={change.by} />
              </span>
            )}
          </span>
        )}
        {change.kind === "request_declined" && (
          <ArrowLink href={askHref(change.project_id, change.role_key, change.request_kind === "extend")} className="self-start">
            Ask again
          </ArrowLink>
        )}
      </span>
      <span className="w-[110px] flex-none text-right text-[13px] text-faint">{dayLabel(change.at)}</span>
    </div>
  );
}

function describe(change: MyChange): { word: string; tone: StatusTone; pre: string; post: ReactNode } {
  const until = change.until ? `, until ${formatShortDate(change.until)}` : "";
  // A request to keep a role longer reads "keep {role} longer", not "for {role}".
  const keep = change.request_kind === "extend";
  const pre = keep ? "Your request to keep " : "Your request for ";
  const longer = keep ? " longer" : "";
  switch (change.kind) {
    case "request_approved":
      return change.waiting
        ? {
            word: "Approved",
            tone: "warn",
            pre,
            post: `${longer} was approved${until}. It isn't on your account yet: ${waitClause(change.wait_reason)}`,
          }
        : { word: "Approved", tone: "healthy", pre, post: `${longer} was approved${until}.` };
    case "request_declined":
      return { word: "Not approved", tone: "neutral", pre, post: `${longer} was not approved.` };
    case "request_withdrawn":
      return { word: "Withdrawn", tone: "neutral", pre, post: `${longer} was withdrawn.` };
    case "taken_away":
      // The board says "Taken away"; the copy guide's one word for ending
      // someone's access is "revoke" (plain-language-copy §6), and its guard
      // refuses the other.
      return { word: "Revoked", tone: "danger", pre: "", post: " was revoked." };
    default:
      if (change.via?.bundle_id) {
        return {
          word: "Given",
          tone: "healthy",
          pre: "",
          post: (
            <>
              {" "}
              was given to you, because you&rsquo;re in <BundleName id={change.via.bundle_id} />.
            </>
          ),
        };
      }
      if (change.via?.rule_id) {
        return { word: "Given", tone: "healthy", pre: "", post: " came with your other access, automatically." };
      }
      return { word: "Given", tone: "healthy", pre: "", post: ` was given to you${until}.` };
  }
}
