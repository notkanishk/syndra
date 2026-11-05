import { UnfoldOnArrival } from "ui";

/** A part that appears after the page did opens to its own height instead of pushing the rest down. Under reduced motion it is simply there. */
export const Arriving = () => (
  <UnfoldOnArrival>
    <p style={{ margin: 0, fontSize: 14.5, color: "var(--muted)" }}>Three people hold this role through a bundle.</p>
  </UnfoldOnArrival>
);
