"use client";

import type { ReactNode } from "react";

import { BundleName, UserName } from "@/components/names";
import { ArrowLink } from "@/components/ui/ArrowLink";
import { StatusDot, type StatusTone } from "@/components/ui/Badge";
import { Card, CardHeader, CardHeaderLink } from "@/components/ui/Card";
import { formatShortDate } from "@/lib/format";
import { keptEnds, stillEnds } from "@/lib/keep-ends";
import type { MyChange } from "@/lib/queries/useMyChanges";

/**
 * Recent changes (route-me 52c): the last 30 days of your own access, newest
 * first, leading the page. The caller renders it only when there is something
 * in it: on a quiet month the page opens on your access.
 */
export function RecentChanges({
  items,
  roleName,
  triggerRole,
  endOf,
}: {
  items: MyChange[];
  roleName: (projectId: string, roleKey: string) => string;
  /**
   * The role a rule gives this one for, when the page can tell. /me/changes
   * says only that a rule gave it (`via.rule_id`), so the page reads the
   * trigger off the access it already holds; undefined falls back to "a
   * makerspace rule" (A-06 §2).
   */
  triggerRole?: (projectId: string, roleKey: string) => string | undefined;
  /** When the role the page shows ends now, for "It still ends" on a request to keep it that wasn't approved. */
  endOf?: (projectId: string, roleKey: string) => string | null;
}) {
  return (
    <Card>
      <CardHeader
        title="Recent changes"
        note="Last 30 days"
        action={<CardHeaderLink href="/requests">All your requests →</CardHeaderLink>}
      />
      {items.map((change, i) => (
        <ChangeRow
          key={`${change.kind}-${change.at}-${change.project_id}-${change.role_key}-${i}`}
          change={change}
          role={roleName(change.project_id, change.role_key)}
          trigger={triggerRole?.(change.project_id, change.role_key)}
          endsNow={endOf?.(change.project_id, change.role_key) ?? null}
        />
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

function ChangeRow({
  change,
  role,
  trigger,
  endsNow,
}: {
  change: MyChange;
  role: string;
  trigger?: string;
  endsNow: string | null;
}) {
  const { word, tone, pre, post } = describe(change, trigger, endsNow);
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

function describe(change: MyChange, trigger?: string, endsNow: string | null = null): { word: string; tone: StatusTone; pre: string; post: ReactNode } {
  const until = change.until ? `, until ${formatShortDate(change.until)}` : "";
  // A request to keep a role reads "keep {role}", not "for {role}" (A-06 §3).
  const keep = change.request_kind === "extend";
  const pre = keep ? "Your request to keep " : "Your request for ";
  switch (change.kind) {
    case "request_approved":
      if (keep && !change.waiting) {
        // Said from the ends the request recorded; an older one that has none says its
        // end as before.
        const ends = keptEnds(change);
        return {
          word: "Kept",
          tone: "healthy",
          pre: "You keep ",
          post: ends
            ? `. Your request to keep it was approved. ${ends}`
            : `${change.until ? ` until ${formatShortDate(change.until)}` : ""}. Your request to keep it was approved.`,
        };
      }
      return change.waiting
        ? {
            word: "Approved",
            tone: "warn",
            pre,
            post: ` was approved${until}. It isn't on your account yet: ${waitClause(change.wait_reason)}`,
          }
        : { word: "Approved", tone: "healthy", pre, post: ` was approved${until}.` };
    case "request_declined": {
      // The end they had when it was declined; for an older request, the end the page
      // knows now. Only an end still ahead "still ends" anywhere.
      const said = keep ? stillEnds(change.ended_before ?? endsNow) : undefined;
      return { word: "Not approved", tone: "neutral", pre, post: ` was not approved.${said ? ` ${said}` : ""}` };
    }
    case "request_withdrawn":
      // Said in the first person, like the request row's "You withdrew this".
      return {
        word: "Withdrawn",
        tone: "neutral",
        pre: keep ? "You withdrew your request to keep " : "You withdrew your request for ",
        post: ".",
      };
    case "cancelled_removal":
      // A removal queued for you was cancelled before it was sent (A-04 §14.8). It carries
      // no decider or reason, so the sentence names neither.
      return {
        word: "Kept",
        tone: "healthy",
        pre: "You keep ",
        post: ". A removal was on its way to you. Makerspace staff cancelled it, so nothing changes.",
      };
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
        return {
          word: "Given",
          tone: "healthy",
          pre: "",
          post: trigger
            ? ` came automatically, because you have ${trigger}.`
            : " was given automatically by a makerspace rule.",
        };
      }
      return { word: "Given", tone: "healthy", pre: "", post: ` was given to you${until}.` };
  }
}
