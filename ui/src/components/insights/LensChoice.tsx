"use client";

import { ENDING_DAYS, type InsightsUrl } from "@/components/insights/useInsightsUrl";
import { Input } from "@/components/ui/Input";
import { FilterPills, Select } from "@/components/ui/Select";

/** The choice a lens carries with it: how far ahead, or which group. People and Recent use carry none. */
export function LensChoice({ url, lens }: { url: InsightsUrl; lens: string }) {
  if (lens === "ending")
    return (
      <FilterPills
        label="How far ahead"
        value={String(url.days)}
        options={ENDING_DAYS.map((d) => ({ value: String(d), label: `${d} days` }))}
        onChange={(next) => url.set({ days: next === String(ENDING_DAYS[0]) ? null : next })}
      />
    );
  if (lens !== "group") return null;

  const team = url.group.startsWith("team:") ? url.group.slice(5) : null;
  return (
    <>
      <div className="w-full tablet:w-64">
        <Select
          aria-label="Which group"
          value={team === null ? url.group : "team"}
          onChange={(e) => url.set({ group: e.target.value === "left" ? null : e.target.value === "team" ? "team:" : e.target.value })}
        >
          <option value="left">People who left the makerspace</option>
          <option value="new_term">New this term</option>
          <option value="team">A team</option>
        </Select>
      </div>
      {team !== null && (
        <div className="w-full tablet:w-48">
          <Input
            key={team}
            aria-label="Team name"
            placeholder="Team name"
            maxLength={100}
            defaultValue={team}
            onBlur={(e) => e.target.value.trim() !== team.trim() && url.set({ group: `team:${e.target.value.trim()}` })}
            onKeyDown={(e) => e.key === "Enter" && e.currentTarget.blur()}
          />
        </div>
      )}
    </>
  );
}
