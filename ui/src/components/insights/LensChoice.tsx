"use client";

import type { ReactNode } from "react";

import { ENDING_DAYS, type InsightsUrl } from "@/components/insights/useInsightsUrl";
import { Input } from "@/components/ui/Input";
import { FilterPills, Select } from "@/components/ui/Select";
import { Unfold } from "@/lib/motion";

/** The gap MapTab's toolbar column puts between its lines (`gap-2.5`), folded with the line. */
const TOOLBAR_GAP = 10;
/** The gap between this line and the team's line beneath it (`gap-2`). */
const LINE_GAP = 8;

/**
 * One line: its words, then its control, at the right under the lens pills; on a phone they stack.
 * Every line is the same height (`--control-md`, the pills' own; the fields beside them are dense),
 * so swapping one lens for another moves nothing below: `Unfold` eases opening and closing, not a swap.
 */
function Line({ words, children }: { words: string; children: ReactNode }) {
  return (
    <div className="flex flex-col items-start gap-1.5 tablet:min-h-(--control-md) tablet:flex-row tablet:items-center tablet:justify-end tablet:gap-3">
      {/* The control carries the same words as its name; said once. */}
      <span aria-hidden className="text-small text-muted">
        {words}
      </span>
      {children}
    </div>
  );
}

/**
 * The choice a lens carries with it: how far ahead, or which group. People and
 * Recent sign-ins carry none. It is a line of its own beneath the toolbar, never
 * a sibling in it (owner, 20 Nov 2026: the lens you just clicked slid away when
 * its choice arrived beside it). It unfolds, so the only thing that moves is
 * what is below, by one line; a team's name unfolds beneath the group the same way.
 */
export function LensChoice({ url, lens }: { url: InsightsUrl; lens: string }) {
  const team = url.group.startsWith("team:") ? url.group.slice(5) : null;
  const line =
    lens === "ending" ? (
      <Line words="How far ahead">
        <FilterPills
          label="How far ahead"
          value={String(url.days)}
          options={ENDING_DAYS.map((d) => ({ value: String(d), label: `${d} days` }))}
          onChange={(next) => url.set({ days: next === String(ENDING_DAYS[0]) ? null : next })}
        />
      </Line>
    ) : lens === "group" ? (
      <>
        <Line words="Which group">
          <div className="w-full tablet:w-64">
            <Select
              dense
              aria-label="Which group"
              value={team === null ? url.group : "team"}
              onChange={(e) => url.set({ group: e.target.value === "left" ? null : e.target.value === "team" ? "team:" : e.target.value })}
            >
              <option value="left">People who left the makerspace</option>
              <option value="new_term">New this term</option>
              <option value="team">A team</option>
            </Select>
          </div>
        </Line>
        <Unfold open={team !== null} gap={LINE_GAP}>
          <Line words="Team name">
            <div className="w-full tablet:w-64">
              <Input
                dense
                key={team ?? ""}
                aria-label="Team name"
                placeholder="Team name"
                maxLength={100}
                defaultValue={team ?? ""}
                onBlur={(e) => team !== null && e.target.value.trim() !== team.trim() && url.set({ group: `team:${e.target.value.trim()}` })}
                onKeyDown={(e) => e.key === "Enter" && e.currentTarget.blur()}
              />
            </div>
          </Line>
        </Unfold>
      </>
    ) : null;

  return (
    <Unfold open={line !== null} gap={TOOLBAR_GAP}>
      <div data-lens-choice className="flex flex-col gap-2">
        {line}
      </div>
    </Unfold>
  );
}
