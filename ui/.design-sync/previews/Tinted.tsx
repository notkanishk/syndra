import { Tinted } from "ui";

const p = { fontSize: 14.5, color: "var(--ink)", margin: 0 } as const;

/** What the Advanced view adds tints in rather than snapping in; under reduced motion it is simply there or not. */
export const Shown = () => (
  <p style={p}>
    Project <Tinted show>p-9001</Tinted>
  </p>
);

export const Hidden = () => (
  <p style={p}>
    Project <Tinted show={false}>p-9001</Tinted>
  </p>
);
