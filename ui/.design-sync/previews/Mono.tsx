import { Mono } from "ui";

const col = { display: "flex", flexDirection: "column", gap: 8 } as const;
const line = { fontSize: 14.5, color: "var(--muted)" } as const;

export const RoleKeys = () => (
  <div style={col}>
    <Mono>laser-cutter.operator</Mono>
    <Mono>cnc-router.supervisor</Mono>
    <Mono>wood-shop.member</Mono>
  </div>
);

export const InSentence = () => (
  <div style={line}>
    Aanya Rao holds <Mono>laser-cutter.operator</Mono> through the claim{" "}
    <Mono>urn:zitadel:iam:org:project:roles</Mono>.
  </div>
);
