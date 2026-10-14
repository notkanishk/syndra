import { Card, CardRow, RowField } from "ui";

const name = { width: 170, fontSize: 15, fontWeight: 600 } as const;
const muted = { flex: 1, fontSize: 14.5, color: "var(--muted)" } as const;

/** The label/value pairs a disclosed row is built from — what the column headings said on a wide screen. */
export const InDisclosedRow = () => (
  <Card>
    <CardRow
      first
      expanded
      onToggle={() => {}}
      disclosure={
        <>
          <RowField label="Granted by">Priya Nair</RowField>
          <RowField label="Came from">Bundle · Wood shop basics</RowField>
          <RowField label="Expires">30 Nov 2026</RowField>
        </>
      }
    >
      <div style={name}>Ishita Sen</div>
      <div style={muted}>Wood shop · Supervisor</div>
    </CardRow>
  </Card>
);

export const Standalone = () => (
  <div style={{ display: "flex", gap: 32 }}>
    <RowField label="Requested">2 Oct 2026, 10:42</RowField>
    <RowField label="Reason">Capstone enclosure on the CNC router</RowField>
  </div>
);
