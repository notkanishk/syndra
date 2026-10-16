"use client";

import Link from "next/link";
import { useMemo, useState, type ReactNode } from "react";

import { FoldHeading } from "@/components/bundles/FoldHeading";
import { Mono } from "@/components/ui/Badge";
import { ChoiceRow } from "@/components/ui/ChoiceRow";
import { Input } from "@/components/ui/Input";
import { NextStep } from "@/components/ui/NextStep";
import { Unfold } from "@/components/ui/Unfold";
import { WaitsMark } from "@/components/ui/WaitsMark";
import { formatAgoShort, formatShortDate } from "@/lib/format";

/**
 * What a person already has of a role, said in the row's note seat before
 * anyone picks (A-04 §10, §12). Only `none` and, for a member, `until` can be
 * chosen: the rest are ticked grey.
 */
export type RoleNote =
  | { kind: "none" }
  /** Held, with no end. */
  | { kind: "held" }
  /** Held until a date. A member can pick it (Ask to keep it); an operator is offered Extend. */
  | { kind: "until"; until: string }
  | { kind: "asked"; at?: string }
  /** Given, and not yet sent to Zitadel. */
  | { kind: "onway"; at?: string };

export interface PickerRole {
  key: string;
  name: string;
  description?: string;
  /** An operator's: how many people hold it, from what Zitadel shows. */
  people?: number;
  /** An operator's: the role always waits for them to send. */
  waits?: boolean;
  note: RoleNote;
}

export interface PickerPlace {
  id: string;
  name: string;
  roles: PickerRole[];
}

export interface RolePickerOneProps {
  mode: "one";
  /** A member reads names and descriptions only. An operator also reads keys, counts and the Waits-for-you mark. */
  audience: "member" | "operator";
  places: PickerPlace[];
  /** The chosen role, as `placeId:roleKey`. */
  value: string | null;
  onChange: (id: string | null) => void;
  /** The place a link named: it opens first, at the top. */
  cameFrom?: string;
  /** The role a link named, drawn with an edge so the eye lands on it. */
  outlined?: string;
  disabled?: boolean;
  labelledBy?: string;
}

const plural = (n: number, one: string, many: string) => `${n} ${n === 1 ? one : many}`;

export const roleId = (placeId: string, roleKey: string) => `${placeId}:${roleKey}`;

/** Held, asked for and on its way can't be chosen; one with an end can, for a member. */
function pickable(role: PickerRole, audience: "member" | "operator"): boolean {
  if (role.note.kind === "none") return true;
  return role.note.kind === "until" && audience === "member";
}

/**
 * The role picker in single choice (A-04 §1, §12): one search over place and
 * role, the chosen role as one chip, places as folds, radio seats. The same
 * part serves a member asking for access and an operator giving it; what each
 * is shown differs only in the note seat and in what a row carries.
 *
 * Data comes from the caller, because a member reads /catalog and an operator
 * reads the role catalogue, and the picker is the same either way.
 */
export function RolePickerOne({
  audience,
  places,
  value,
  onChange,
  cameFrom,
  outlined,
  disabled = false,
  labelledBy,
}: RolePickerOneProps) {
  const [query, setQuery] = useState("");
  // A place opened or closed by hand keeps that, whatever the search does.
  const [folded, setFolded] = useState<Record<string, boolean>>({});
  const needle = query.trim().toLowerCase();
  const operator = audience === "operator";

  const shown = useMemo(() => {
    const matches = (place: PickerPlace, role: PickerRole) =>
      !needle ||
      [place.name, role.name, role.description ?? "", operator ? role.key : ""]
        .join(" ")
        .toLowerCase()
        .includes(needle);
    return places
      .map((place) => ({ place, roles: place.roles.filter((role) => matches(place, role)) }))
      .filter((entry) => entry.roles.length > 0 || (!needle && entry.place.roles.length === 0))
      .sort((a, b) => {
        // The place a link named leads, then A to Z.
        if (a.place.id === cameFrom) return -1;
        if (b.place.id === cameFrom) return 1;
        return a.place.name.localeCompare(b.place.name);
      });
  }, [places, needle, cameFrom, operator]);

  const chosen = useMemo(() => {
    if (!value) return null;
    for (const place of places) {
      const role = place.roles.find((r) => roleId(place.id, r.key) === value);
      if (role) return { place, role };
    }
    return null;
  }, [places, value]);

  return (
    <>
      <Input
        type="search"
        value={query}
        onChange={(event) => setQuery(event.target.value)}
        placeholder="Search by place or role"
        aria-label="Search roles"
      />

      {chosen && (
        <div className="mt-3 flex flex-wrap items-center gap-2.5 desktop:gap-2" aria-label="Chosen">
          <span className="type-label mr-1">Chosen</span>
          <span
            key={value}
            className="chip-in inline-flex h-9 items-center gap-1 rounded-pill border border-accent-line bg-accent-soft pl-3 pr-1 text-[13px] font-semibold"
          >
            {chosen.role.name}
            <span className="font-normal text-muted"> · {chosen.place.name}</span>
            {/* The chip stays 36px; on touch the × reaches 44px by extending past it, invisibly. */}
            <button
              type="button"
              disabled={disabled}
              onClick={() => onChange(null)}
              aria-label={`Remove ${chosen.role.name}`}
              className="group -my-1 -mr-2 flex min-h-[44px] min-w-[44px] items-center justify-center desktop:m-0 desktop:min-h-0 desktop:min-w-0"
            >
              <span
                aria-hidden
                className="flex size-7 items-center justify-center rounded-pill text-muted motion-tint group-hover:bg-[var(--hover)]"
              >
                ×
              </span>
            </button>
          </span>
        </div>
      )}

      <div
        className="mt-3 max-h-[46vh] overflow-y-auto rounded-inner border border-line-strong"
        role="radiogroup"
        aria-labelledby={labelledBy}
        aria-label={labelledBy ? undefined : "Roles"}
      >
        {shown.length === 0 ? (
          <p className="px-4 py-6 text-[14px] text-muted">
            {operator
              ? `No role matches “${query}”. A role created directly in Zitadel may not be listed here yet. Check there, or create it in Syndra.`
              : `Nothing you can ask for matches “${query}”. Ask for access, and say what you need in Why.`}
          </p>
        ) : (
          shown.map(({ place, roles }) => {
            const holdsChosen = chosen?.place.id === place.id;
            // Open while searching, when it holds the chosen role, or when it
            // is the place a link named. A held role never opens its place.
            const open = folded[place.id] ?? (Boolean(needle) || holdsChosen || place.id === cameFrom);
            const held = place.roles.filter((r) => !pickable(r, audience) && r.note.kind !== "asked" && r.note.kind !== "onway");
            return (
              <div key={place.id} className="row-divider">
                <FoldHeading
                  open={open}
                  onClick={() => setFolded((f) => ({ ...f, [place.id]: !open }))}
                  title={place.name}
                  count={foldCount(place, holdsChosen, held.length, operator)}
                  picked={holdsChosen}
                />
                <Unfold open={open}>
                  <div className="flex flex-col gap-1.5 px-2 pb-2">
                    {roles.length === 0 ? (
                      <p className="px-3.5 py-2 text-[14px] text-muted">
                        {operator
                          ? "Nothing is set up here yet, so there's nothing to give."
                          : "Nothing is set up here yet, so there's nothing to ask for. Makerspace staff can set it up."}
                      </p>
                    ) : (
                      roles.map((role) => {
                        const id = roleId(place.id, role.key);
                        const canPick = pickable(role, audience);
                        return (
                          <ChoiceRow
                            key={id}
                            single
                            name={canPick ? "role-one" : undefined}
                            selected={value === id}
                            locked={!canPick}
                            disabled={disabled}
                            outlined={outlined === id}
                            onToggle={() => onChange(id)}
                            title={
                              <>
                                {role.name}
                                {operator && role.waits && <WaitsMark className="ml-2" />}
                              </>
                            }
                            meta={operator ? <Mono className="font-normal">{role.key}</Mono> : undefined}
                            description={role.description}
                            note={noteSeat(role, audience)}
                          />
                        );
                      })
                    )}
                  </div>
                </Unfold>
              </div>
            );
          })
        )}
      </div>
    </>
  );
}

/** "3 roles", "1 chosen", "Nothing yet", or "1 you have" when everything in it is already held. */
function foldCount(place: PickerPlace, holdsChosen: boolean, held: number, operator: boolean): string {
  if (holdsChosen) return "1 chosen";
  if (place.roles.length === 0) return "Nothing yet";
  if (held === place.roles.length) {
    return operator ? (held === 1 ? "1 has this" : `${held} have this`) : `${held} you have`;
  }
  return plural(place.roles.length, "role", "roles");
}

/** What the row says about itself on the right: the person's standing first, then (operator) how many hold it. */
function noteSeat(role: PickerRole, audience: "member" | "operator"): ReactNode {
  const operator = audience === "operator";
  const n = role.note;
  switch (n.kind) {
    case "held":
      return operator ? "Has this" : "You have this";
    case "until":
      return operator ? (
        <>
          <span>Has this until {formatShortDate(n.until)}</span>
          <Link
            href="/review/expiring-access?within=30"
            className="inline-flex min-h-[44px] items-center font-semibold text-accent-text motion-tint hover:brightness-110 desktop:min-h-0"
          >
            Extend <span aria-hidden>→</span>
          </Link>
        </>
      ) : (
        `Yours until ${formatShortDate(n.until)}`
      );
    case "asked":
      return n.at ? `Asked ${formatAgoShort(n.at)}` : "Asked";
    case "onway":
      return operator ? (
        <>
          <span>{n.at ? `Given ${formatAgoShort(n.at)}` : "Given"}</span>
          <NextStep dot={false} />
        </>
      ) : (
        "On its way"
      );
    default:
      if (!operator || role.people === undefined) return undefined;
      return plural(role.people, "person", "people");
  }
}
