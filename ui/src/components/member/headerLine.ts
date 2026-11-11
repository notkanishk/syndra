import type { StatusTone } from "@/components/ui/Badge";
import { daysUntil, formatShortDate } from "@/lib/format";
import { ABOUT } from "@/lib/greeting";
import { MEMBER_NAV, navLeaves } from "@/lib/nav";
import { sortByName } from "@/lib/sort";

/** The storage page's link, labelled as the nav labels it: one source for the name. */
const STORAGE_LEAF = navLeaves(MEMBER_NAV).find((l) => l.href === "/storage")!;

/**
 * My access's one line (A-18 §2 A): the single thing worth knowing, chosen
 * from a fixed ladder. The first rung that matches speaks; everything below
 * it still shows in its own place.
 *
 * Pure, and it counts nothing from records. Every number it says is a count of
 * roles Zitadel HOLDS (`ending`'s caller passes only observed roles); an end
 * date, a request or an approval only names things and supplies a date.
 */

/** Where the line's link goes: down to a project's card, or to another page. */
export interface HeaderLink {
  label: string;
  /** Jump to this project's card on the page. */
  projectId?: string;
  /** Or go to this page. */
  href?: string;
}

export interface HeaderLine {
  /** "none" draws no dot: only while the read is in flight. */
  tone: StatusTone | "none";
  text: string;
  link?: HeaderLink;
}

/** A role, said as "Operator in Laser cutting"; `project` is the card to jump to. */
export interface NamedRole {
  label: string;
  projectId: string;
  /** The project's name, for the link. Absent when it has no resolved name: no link then. */
  project?: string;
}

export interface HeaderFacts {
  /** The read of access is still in flight. */
  loading: boolean;
  /** Zitadel has been read (the observation has a time). */
  read: boolean;
  now: Date;
  /** This session is the person's first: the calm rung says what Syndra is. */
  firstVisit?: boolean;
  /** When they were last here and nothing in their feed is newer: "on Tuesday", "yesterday" (A-20 P18). */
  quietSince?: string;
  /** Roles Zitadel holds that end, with when. Empty until the grants are read. */
  ending: Array<NamedRole & { endsAt: string }>;
  /** Given, not on the account after a full read. */
  missing: NamedRole[];
  /** A storage system that is theirs and has no password yet. */
  storage?: { name: string };
  /** Their open requests, any order. */
  requests: Array<{ label?: string; at: string }>;
  /** Approved, not yet sent. */
  onItsWay: NamedRole[];
  /** Held roles whose access began today, by project. */
  startedToday: Array<{ project: string; projectId: string }>;
}

/** Roles that end this soon outrank everything else on the page. */
export const ENDING_WITHIN_DAYS = 14;

const WORDS = ["no", "one", "two", "three", "four", "five", "six", "seven", "eight", "nine", "ten"];
const countWord = (count: number) => WORDS[count] ?? String(count);
const capitalise = (value: string) => value.charAt(0).toUpperCase() + value.slice(1);

const jump = (role: NamedRole): HeaderLink | undefined =>
  role.project ? { label: role.project, projectId: role.projectId } : undefined;

function inDays(days: number): string {
  if (days <= 0) return "today";
  if (days === 1) return "tomorrow";
  return `in ${days} days`;
}

/** "Laser cutting and Wood shop", alphabetical. */
function list(names: string[]): string {
  const sorted = sortByName(names, (n) => n, (n) => n);
  return sorted.length < 3
    ? sorted.join(" and ")
    : `${sorted.slice(0, -1).join(", ")} and ${sorted[sorted.length - 1]}`;
}

export function headerLine(facts: HeaderFacts): HeaderLine {
  if (facts.loading) return { tone: "none", text: "Checking what you can use." };
  if (!facts.read) {
    return { tone: "neutral", text: "Zitadel hasn't answered yet, so your roles aren't shown." };
  }

  // 1 · A held role ends within 14 days: the only rung that costs something if ignored.
  const soon = facts.ending
    .map((role) => ({ role, days: daysUntil(role.endsAt, facts.now) ?? -1 }))
    .filter(({ days }) => days >= 0 && days <= ENDING_WITHIN_DAYS)
    .sort((a, b) => (a.role.endsAt < b.role.endsAt ? -1 : a.role.endsAt > b.role.endsAt ? 1 : 0));
  if (soon.length > 0) {
    const [{ role, days }] = soon;
    const text =
      soon.length === 1
        ? `${role.label} ends ${inDays(days)}.`
        : `${capitalise(countWord(soon.length))} of your roles end soon. The first, ${role.label}, ends ${inDays(days)}.`;
    return { tone: "warn", text, link: jump(role) };
  }

  // 2 · Given, and Zitadel doesn't have it.
  if (facts.missing.length > 0) {
    const [role] = sortByName(facts.missing, (r) => r.label, (r) => r.projectId);
    const text =
      facts.missing.length === 1
        ? `${role.label} isn't on your account yet.`
        : `${capitalise(countWord(facts.missing.length))} roles you were given aren't on your account yet, starting with ${role.label}.`;
    return { tone: "warn", text, link: jump(role) };
  }

  // 3 · A step only they can take.
  if (facts.storage) {
    return {
      tone: "neutral",
      text: `Your ${facts.storage.name} account is ready. Set its password to connect.`,
      link: { label: STORAGE_LEAF.label, href: STORAGE_LEAF.href },
    };
  }

  // 4 · With staff. Names the newest rather than counting records.
  if (facts.requests.length > 0) {
    const [newest] = [...facts.requests].sort((a, b) => (a.at < b.at ? 1 : a.at > b.at ? -1 : 0));
    const when = formatShortDate(newest.at);
    const text =
      facts.requests.length === 1
        ? newest.label
          ? `Makerspace staff have your request for ${newest.label}. You asked on ${when}.`
          : `Makerspace staff have your request. You asked on ${when}.`
        : newest.label
          ? `Makerspace staff have your requests. The newest, for ${newest.label}, is from ${when}.`
          : `Makerspace staff have your requests. The newest is from ${when}.`;
    return { tone: "neutral", text, link: { label: "Your requests", href: "/requests" } };
  }

  // 5 · Approved, on its way.
  if (facts.onItsWay.length > 0) {
    const [role] = sortByName(facts.onItsWay, (r) => r.label, (r) => r.projectId);
    const text =
      facts.onItsWay.length === 1
        ? `${role.label} is on its way to you.`
        : `${capitalise(countWord(facts.onItsWay.length))} roles are on their way to you, starting with ${role.label}.`;
    return { tone: "neutral", text, link: jump(role) };
  }

  // 6 · The day access starts. It never blooms: the member didn't do it.
  if (facts.startedToday.length > 0) {
    const projects = Array.from(new Map(facts.startedToday.map((p) => [p.projectId, p])).values());
    const names = projects.map((p) => p.project);
    const [first] = sortByName(projects, (p) => p.project, (p) => p.projectId);
    return {
      tone: "healthy",
      text: `From today, you can use ${list(names)}.`,
      link: { label: first.project, projectId: first.projectId },
    };
  }

  // 7 · The same words as the operator's Home; on a first visit, what Syndra is
  // instead (A-20 P2), because there is nothing else to say and the title has
  // already welcomed them.
  if (facts.firstVisit) return { tone: "none", text: ABOUT };
  // After a first visit there is no "last time"; otherwise, when the feed has
  // nothing since the previous sign-in, say so (A-20 P18).
  if (facts.quietSince) {
    return { tone: "healthy", text: `Nothing has changed since you were last here, ${facts.quietSince}.` };
  }
  return { tone: "healthy", text: "Nothing needs you right now." };
}
