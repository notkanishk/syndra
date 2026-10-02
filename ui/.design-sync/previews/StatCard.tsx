import { StatCard } from "ui";

const row = { display: "flex", flexWrap: "wrap", gap: 18 } as const;

/** Tone is the axis: neutral, accent (a state chosen on purpose), warn (deadline), danger (decide now). */
export const Neutral = () => (
  <div style={row}>
    <StatCard label="Projects in Zitadel" value="9" detail="As Zitadel reported them just now." />
  </div>
);

export const Accent = () => (
  <div style={row}>
    <StatCard
      label="Sync"
      value="Paused"
      tone="accent"
      detail="Paused by Priya Nair for TrueNAS maintenance. Nothing is lost; changes wait until it resumes."
    />
  </div>
);

export const Warn = () => (
  <div style={row}>
    <StatCard
      label="Action signing key"
      value="214 days old"
      tone="warn"
      detail="Older than the 180 days we aim for. Replace it from the steps below."
    />
  </div>
);

export const Danger = () => (
  <div style={row}>
    <StatCard
      label="Connection"
      value="Unreachable"
      tone="danger"
      detail="The last attempt to reach Zitadel failed. Decisions made here wait until it answers again."
    />
  </div>
);

export const ThreeTileRow = () => (
  <div style={row}>
    <StatCard label="Action signing key" value="42 days old" detail="Replaced on 21 Aug 2026." />
    <StatCard label="Projects in Zitadel" value="9" detail="As Zitadel reported them just now." />
    <StatCard label="Connection" value="Connected" detail="Syndra can read and change access in Zitadel." />
  </div>
);
