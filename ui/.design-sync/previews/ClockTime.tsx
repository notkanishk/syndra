import { ClockTime } from "ui";

const line = { fontSize: 14, color: "var(--muted)" } as const;

export const LastChecked = () => (
  <div style={line}>Friday · last checked <ClockTime at={Date.UTC(2026, 9, 2, 3, 38)} /></div>
);

export const NotAnswering = () => (
  <div style={line}>Zitadel is not answering, as of <ClockTime at={Date.UTC(2026, 9, 2, 8, 42)} />.</div>
);
