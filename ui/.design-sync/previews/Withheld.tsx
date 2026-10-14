import { Withheld } from "ui";

const box = { maxWidth: 620 } as const;

const one = [
  { field: "group", value: "wood-shop-door", reason: "Safety induction is due for renewal" },
];

const several = [
  { field: "group", value: "cnc-router", reason: "Waiting on a supervisor sign-off after the spindle incident", target: "Door access", actorId: "Priya Nair" },
  { field: "share", value: "capstone-2026", reason: "Over the storage quota", target: "TrueNAS", actorId: "Vivek Nair", reviewDue: true },
];

export const MemberOne = () => <div style={box}><Withheld items={one} /></div>;

export const MemberSeveral = () => (
  <div style={box}><Withheld items={several.map(({ target, actorId, ...rest }) => rest)} /></div>
);

/** Operator view: names the system and who decided it. */
export const Operator = () => <div style={box}><Withheld audience="operator" items={several} /></div>;
