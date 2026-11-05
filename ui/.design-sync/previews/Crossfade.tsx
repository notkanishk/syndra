import { Crossfade } from "ui";

const p = { fontSize: 14.5, color: "var(--ink)", margin: 0 } as const;

/** Words that change in place: the old layer fades out and the new fades in, in one cell, so nothing around it shifts. Under reduced motion the words swap at once. */
export const Seat = () => (
  <p style={p}>
    <Crossfade id="approved">Approved</Crossfade>
  </p>
);

export const InsideASentence = () => (
  <p style={p}>
    Aanya Rao, <Crossfade id="state" inline>waiting for you to send</Crossfade>.
  </p>
);
