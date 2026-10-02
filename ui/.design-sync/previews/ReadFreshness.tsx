import { ReadFreshness } from "ui";

const ago = (min: number) => new Date(Date.now() - min * 60_000).toISOString();
const col = { display: "grid", gap: 10 } as const;

export const Live = () => <ReadFreshness state={{ readAt: ago(0.2), current: true }} subject="What Zitadel confirmed" />;

export const Ageing = () => <ReadFreshness state={{ readAt: ago(4), current: true }} subject="The account list" />;

export const Stale = () => (
  <div style={col}>
    <ReadFreshness state={{ readAt: ago(14), current: true }} subject="The account list" onRefresh={() => {}} />
    <ReadFreshness state={{ readAt: null }} subject="The account list" />
  </div>
);

export const Provisional = () => <ReadFreshness state={{ readAt: ago(22), current: false }} />;

export const Truncated = () => (
  <ReadFreshness state={{ readAt: ago(2), current: true, truncated: true }} subject="The TrueNAS share list" />
);
