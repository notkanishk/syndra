import { useState } from "react";
import { ChoiceRow, WaitsMark } from "ui";

const list = { display: "grid", gap: 4, maxWidth: 560 } as const;

/** Many of many: chosen rows take the violet tint; locked says where; blocked says why. */
export function PickRoles() {
  const [chosen, setChosen] = useState<string[]>(["operator"]);
  const toggle = (key: string) =>
    setChosen((cur) => (cur.includes(key) ? cur.filter((k) => k !== key) : [...cur, key]));
  return (
    <div style={list}>
      <ChoiceRow
        selected={chosen.includes("member")}
        onToggle={() => toggle("member")}
        locked
        lockedNote="Given by the Wood Shop bundle"
        title="Member"
        note="You have this"
      />
      <ChoiceRow
        selected={chosen.includes("operator")}
        onToggle={() => toggle("operator")}
        title="Operator"
        description="Can start a cut job on their own."
      />
      <ChoiceRow
        selected={chosen.includes("supervisor")}
        onToggle={() => toggle("supervisor")}
        title="Supervisor"
        description="Can sign off other people's inductions."
        aside={<WaitsMark withLabel />}
      />
      <ChoiceRow
        selected={false}
        onToggle={() => {}}
        blocked
        blockedReason="Needs a safety induction first."
        title="Maintenance"
        description="Can change blades and calibrate the bed."
      />
    </div>
  );
}

/** One of many: the seat holds a dot, and `name` groups the radios. */
export function PickOneBundle() {
  const [pick, setPick] = useState("wood");
  return (
    <div style={list}>
      <ChoiceRow single name="bundle" selected={pick === "wood"} onToggle={() => setPick("wood")} title="Wood Shop" meta="v3 · 4 roles" description="Everything a new wood shop member needs." />
      <ChoiceRow single name="bundle" selected={pick === "laser"} onToggle={() => setPick("laser")} title="Laser Cutter" meta="v2 · 2 roles" description="Operator and the safety log." />
      <ChoiceRow single name="bundle" selected={pick === "elec"} onToggle={() => setPick("elec")} title="Electronics Bench" meta="v1 · 3 roles" description="Soldering stations and the parts store." note="Yours until 18 Dec" />
    </div>
  );
}
