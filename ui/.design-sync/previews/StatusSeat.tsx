import { StatusSeat } from "ui";

const col = { display: "grid", gap: 16, maxWidth: 320 } as const;

/** A dot, the word, and one faint note. */
export const Tones = () => (
  <div style={col}>
    <StatusSeat tone="healthy" word="In Zitadel" note="Confirmed 2 minutes ago" />
    <StatusSeat tone="warn" word="Waiting for you" note="Supervisor always waits for you" />
    <StatusSeat tone="danger" word="Didn't reach Zitadel" note="Syndra will try again in 5 minutes" />
    <StatusSeat tone="accent" word="Sending" />
    <StatusSeat tone="neutral" word="Not set up" />
  </div>
);
