import { useEffect, useRef } from "react";
import { Card, CardRow, WaitsMark } from "ui";

const role = { flex: 1, fontSize: 15, fontWeight: 600, display: "flex", alignItems: "center", gap: 8 } as const;
const muted = { fontSize: 14, color: "var(--muted)" } as const;

/** After a role that always waits for the operator. */
export const AfterARole = () => (
  <Card>
    <CardRow first>
      <span style={role}>Supervisor <WaitsMark /></span>
      <span style={muted}>4 people</span>
    </CardRow>
    <CardRow>
      <span style={role}>Operator</span>
      <span style={muted}>23 people</span>
    </CardRow>
  </Card>
);

/** In words, where the row has room. */
export const WithLabel = () => <WaitsMark withLabel />;

/** Focused: the same words a pointer gets on hover. */
export function Explained() {
  const ref = useRef<HTMLDivElement>(null);
  useEffect(() => {
    ref.current?.querySelector<HTMLElement>("[role=img]")?.focus();
  }, []);
  return (
    <div ref={ref} style={{ minHeight: 120, fontSize: 15, fontWeight: 600 }}>
      Supervisor <WaitsMark />
    </div>
  );
}
