"use client";

import React from "react";

/**
 * Access an operator has deliberately taken away, with the reason (design §20,
 * §25, §26).
 *
 * One component because it is one object wearing three faces. A **hold** is
 * authored on the row it holds, listed under Review, and chased when its review
 * date passes — and what the member reads on their own page is the same row. If
 * the member's view and the operator's queue described it differently, the two
 * of them would be talking about different things on the phone.
 *
 * The word in the interface is "withheld", never "suppressed" and never
 * "allowance". The API calls the object an allowance, which reads as permission
 * granted when it does the opposite; that name stays in the code.
 *
 * The reason is not optional decoration. A member who sees access they expect
 * and do not have, with no explanation, asks an operator. One who can read the
 * reason does not — which is the entire return on making the field mandatory
 * when the hold was authored.
 */

export interface WithheldItem {
  /** The entitlement field it applies to — `group`, `share`. */
  field: string;
  /** The value being withheld, as the member would recognise it. */
  value: string;
  reason: string;
  /** Which system it is on. Named to an operator, implied to a member. */
  target?: string;
  /** Who decided it. An operator's first question; not a member's. */
  actorId?: string;
  /** Past the date somebody promised to look at it again. */
  reviewDue?: boolean;
}

/** The inline pill, for a row inside a longer list — a door, a share. */
export function WithheldPill({ className = "" }: { className?: string }) {
  return (
    <span
      className={`inline-flex items-center rounded-pill bg-warn-soft px-2.5 py-0.5 text-[12px] font-semibold text-warn-text ${className}`}
    >
      Withheld
    </span>
  );
}

/**
 * The panel, for the whole-target case: one or more things held, each with why.
 *
 * Amber rather than danger. Nothing has gone wrong — somebody decided this, and
 * it is reversible by the person who decided it. Red would tell a member their
 * account is broken and send them to an operator to report a fault.
 */
export function Withheld({
  items,
  /** Who it reads to. The member's page and the operator's row say it differently. */
  audience = "member",
  className = "",
}: {
  items: WithheldItem[];
  audience?: "member" | "operator";
  className?: string;
}) {
  if (items.length === 0) return null;

  return (
    <div className={`rounded-inner border border-warn-line bg-warn-soft px-4 py-3 ${className}`}>
      <div className="flex items-center gap-2">
        <WithheldPill />
        <p className="text-[13.5px] font-semibold text-warn-text">
          {audience === "member"
            ? items.length === 1
              ? "Something a role of yours grants is being held"
              : "Some of what your roles grant is being held"
            : `${items.length} ${items.length === 1 ? "hold" : "holds"} in force`}
        </p>
      </div>
      <ul className="mt-2 grid gap-1.5">
        {items.map((item) => (
          <li key={`${item.target ?? ""}:${item.field}:${item.value}`} className="text-[13.5px]">
            <span className="font-mono text-[13px] text-ink">{item.value}</span>
            {audience === "operator" && item.target && (
              <span className="text-muted"> on {item.target}</span>
            )}
            {audience === "operator" && item.actorId && (
              // Who decided it, first. An operator looking at a hold is
              // deciding whether to lift it, and that is a conversation with a
              // person rather than a judgement about a row.
              <span className="text-muted"> — held by {item.actorId}</span>
            )}
            <span className="text-muted">
              {audience === "operator" ? (item.reason ? `, because ${item.reason}` : "") : ` — ${item.reason}`}
            </span>
            {item.reviewDue && (
              <span className="text-warn-text"> · past its review date</span>
            )}
          </li>
        ))}
      </ul>
      {audience === "operator" && (
        // The half of "why does this person have access to X" that says they do
        // not. A role-holder list reads as full access, and this is precisely
        // the case where it is not.
        <p className="mt-2 text-[13px] text-faint">
          They may still hold a role that maps to it; this is why they cannot use it.
        </p>
      )}
      {audience === "member" && (
        // The mechanism, once, at the bottom. A member who reads this knows the
        // hold is a decision with an owner rather than a fault with a queue.
        <p className="mt-2 text-[13px] text-faint">
          A hold does not take the role away. An operator lifts it, or it lapses on its
          own date.
        </p>
      )}
    </div>
  );
}

/**
 * The one-line form, for a row inside a dense list.
 *
 * The banner above is right on a page about one person and wrong repeated forty
 * times down a table — a warning that appears on most rows is a background
 * colour, not a warning. This says the same thing in a line, and it says the
 * REASON: an operator scanning a holder list is deciding who to act on, and
 * "withheld" without the why moves the question rather than answering it.
 *
 * Same component file as the banner deliberately. They are two densities of one
 * object, and the day the vocabulary changes it must change in both.
 */
export function WithheldInline({ items }: { items: WithheldItem[] }) {
  if (items.length === 0) return null;
  return (
    <ul className="mt-1 grid gap-0.5">
      {items.map((item) => (
        <li
          key={`${item.target ?? ""}:${item.field}:${item.value}`}
          className="flex flex-wrap items-baseline gap-x-1.5 text-[13px]"
        >
          <WithheldPill />
          <span className="font-mono text-[12.5px] text-ink">{item.value}</span>
          {item.target && <span className="text-faint">on {item.target}</span>}
          {item.reason && <span className="text-muted">— {item.reason}</span>}
          {item.reviewDue && <span className="text-warn-text">· past its review date</span>}
        </li>
      ))}
    </ul>
  );
}
