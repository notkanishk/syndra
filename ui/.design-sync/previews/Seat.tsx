import { Seat } from "ui";

const row = { display: "flex", alignItems: "center", gap: 20 } as const;

export const Check = () => (
  <div style={row}>
    <Seat aria-label="Not chosen" defaultChecked={false} />
    <Seat aria-label="Chosen" defaultChecked />
    <Seat aria-label="Held by something else" locked />
  </div>
);

/** One of many draws a dot. Held is always a tick, never a dot. */
export const OneOfMany = () => (
  <div style={row}>
    <Seat single name="how-long" aria-label="One week" defaultChecked />
    <Seat single name="how-long" aria-label="One month" />
  </div>
);
