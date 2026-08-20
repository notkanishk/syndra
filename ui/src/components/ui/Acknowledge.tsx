"use client";

import { useId, useState } from "react";

import { Input } from "@/components/ui/Input";

/**
 * The acknowledgement ladder (design §31 B).
 *
 * Three rungs, and the rung is set by WHAT CANNOT BE UNDONE — never by how
 * important the action feels. Rung 1 is not a component: it is the plan stating
 * its numbers and the button naming them, which every rehearsal already does.
 * The other two live here so that the gesture for "this reaches further than it
 * looks" is the same gesture everywhere, and so is the gesture for "this takes
 * something real away from a named person".
 *
 *   rung 2  AcknowledgeCount  — tick the number, inside the sentence
 *   rung 3  ConfirmByTyping   — type the name, which is what unlocks the fill
 *
 * A daily action on rung 3 is a ceremony people stop reading, and once they
 * stop reading it, it protects nothing on the day it matters. Rung 2 exists
 * precisely so routine-but-wide work is not pushed up into a ritual. When in
 * doubt, go down a rung and make the copy better.
 *
 * None of this is the protection. The backend refuses an unconfirmed
 * `account.adopt`, `account.purge` or revocation regardless — these are what a
 * person meets, not what guards the data.
 */

/**
 * Rung 2 — the quantity sits INSIDE the sentence being ticked.
 *
 * "I understand this changes 34 people", not a checkbox beside a number
 * somewhere else on the page. Ticking it means having read it, which is the
 * entire mechanism; a checkbox labelled "I understand" next to a figure in
 * another paragraph is a checkbox people tick without moving their eyes.
 */
export function AcknowledgeCount({
  checked,
  onChange,
  count,
  noun,
  /** What actually happens to them, if "changes" is not precise enough. */
  verb = "changes",
  /** The irreversible part, when it is not the count. Named after the sentence. */
  consequence,
  disabled = false,
}: {
  checked: boolean;
  onChange: (next: boolean) => void;
  count: number;
  noun: string;
  verb?: string;
  consequence?: React.ReactNode;
  disabled?: boolean;
}) {
  const id = useId();
  return (
    <div className="rounded-inner border border-warn-line bg-warn-soft px-4 py-3">
      <label htmlFor={id} className="flex cursor-pointer items-start gap-3 text-[14px]">
        <input
          id={id}
          type="checkbox"
          className="mt-1 size-4 shrink-0 accent-[var(--accent)]"
          checked={checked}
          disabled={disabled}
          onChange={(e) => onChange(e.target.checked)}
        />
        <span>
          I understand this {verb}{" "}
          <span className="font-semibold">
            {count} {count === 1 ? singular(noun) : noun}
          </span>
          .
        </span>
      </label>
      {consequence && <p className="mt-2 pl-7 text-[13px] text-muted">{consequence}</p>}
    </div>
  );
}

/**
 * Rung 3 — typing the name is what unlocks the control.
 *
 * Reserved for taking real access from a named person, or for anything with no
 * undo. The gesture is deliberately the most annoying one in the product and is
 * used in exactly four places, so that meeting it means something.
 *
 * The comparison is trimmed and case-insensitive: this is a speed bump for
 * attention, not a spelling test, and failing somebody on a capital letter
 * teaches them to copy and paste — which defeats the whole point.
 */
export function ConfirmByTyping({
  expected,
  value,
  onChange,
  /** What the operator is being asked to name. "account", "person", "bundle". */
  noun = "name",
  disabled = false,
}: {
  expected: string;
  value: string;
  onChange: (next: string) => void;
  noun?: string;
  disabled?: boolean;
}) {
  const id = useId();
  return (
    <div className="grid gap-1.5">
      <label htmlFor={id} className="text-[13.5px] text-muted">
        Type the {noun} <span className="font-mono text-ink">{expected}</span> to confirm
      </label>
      <Input
        id={id}
        value={value}
        disabled={disabled}
        autoComplete="off"
        spellCheck={false}
        // A phone keyboard capitalises the first letter and offers to correct
        // what it thinks is a misspelling — and what it is looking at is an
        // account name. `typedMatches` already forgives case, which is the
        // same failure seen from the other end; these stop the operator having
        // to fight the keyboard to reach a control they have decided to use.
        autoCapitalize="none"
        autoCorrect="off"
        inputMode="text"
        onChange={(e) => onChange(e.target.value)}
      />
    </div>
  );
}

/** Whether what was typed matches. One definition, so no two dialogs disagree. */
export function typedMatches(expected: string, typed: string): boolean {
  return typed.trim().toLowerCase() === expected.trim().toLowerCase() && expected.trim() !== "";
}

/** "people" → "person" for the count of one, without dragging in a library. */
function singular(noun: string): string {
  if (noun === "people") return "person";
  if (noun.endsWith("ies")) return `${noun.slice(0, -3)}y`;
  if (noun.endsWith("s")) return noun.slice(0, -1);
  return noun;
}

/**
 * A modal's confirming row, when the confirmation is rung 3.
 *
 * Here rather than in each dialog because the pairing is the rule: the solid
 * danger fill exists nowhere else in the product, and it must be unreachable
 * until the name is typed. Two dialogs that each build this from parts is two
 * chances for one of them to render an armed red button.
 */
export function useTypedConfirmation(expected: string) {
  const [typed, setTyped] = useState("");
  return {
    typed,
    setTyped,
    armed: typedMatches(expected, typed),
    reset: () => setTyped(""),
  };
}
