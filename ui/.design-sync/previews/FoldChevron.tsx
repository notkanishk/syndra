import { FoldChevron } from "ui";

const row = { display: "flex", alignItems: "center", gap: 24, color: "var(--muted)" } as const;

/** Turns a quarter when its fold opens. Under reduced motion it is the open or closed shape, with no turn between. */
export const ClosedAndOpen = () => (
  <div style={row}>
    <FoldChevron open={false} />
    <FoldChevron open />
  </div>
);
