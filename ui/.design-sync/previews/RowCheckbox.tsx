import { useState } from "react";
import { RowCheckbox, Card } from "ui";

const people = [
  { id: "1", name: "Aanya Rao", role: "Laser cutter · Operator" },
  { id: "2", name: "Kabir Mehta", role: "3D printers · Member" },
  { id: "3", name: "Ishita Sen", role: "Wood shop · Supervisor" },
];
const rowStyle = { display: "flex", alignItems: "center", gap: 14, padding: "10px 20px", borderTop: "1px solid var(--line)", cursor: "pointer" } as const;

/** The whole row is the label, so the row is the hit area. */
export const InRows = () => {
  const [picked, setPicked] = useState<Record<string, boolean>>({ "1": true, "3": true });
  return (
    <Card>
      {people.map((p) => (
        <label key={p.id} style={{ ...rowStyle, background: picked[p.id] ? "var(--accent-soft)" : undefined }}>
          <RowCheckbox
            label={`Select ${p.name}`}
            checked={!!picked[p.id]}
            onChange={() => setPicked({ ...picked, [p.id]: !picked[p.id] })}
          />
          <span style={{ width: 150, fontSize: 15, fontWeight: 600 }}>{p.name}</span>
          <span style={{ fontSize: 14.5, color: "var(--muted)" }}>{p.role}</span>
        </label>
      ))}
    </Card>
  );
};

export const States = () => (
  <div style={{ display: "flex", gap: 8, alignItems: "center" }}>
    <RowCheckbox label="Select Rohan Iyer" checked={false} onChange={() => {}} />
    <RowCheckbox label="Select Meera Pillai" checked onChange={() => {}} />
    <RowCheckbox label="Select Arjun Nair" disabled checked={false} onChange={() => {}} />
  </div>
);
