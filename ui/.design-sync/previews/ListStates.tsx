import { Card, CardHeader, CardRow, EmptyState, ListStates, RowSkeleton } from "ui";

const name = { width: 170, fontSize: 15, fontWeight: 600 } as const;
const muted = { flex: 1, fontSize: 14.5, color: "var(--muted)" } as const;

const empty = (
  <EmptyState
    title="No one holds this role yet"
    guidance="Grant it from a person's page, or add it to a bundle."
  />
);

const rows = (
  <>
    <CardRow first>
      <div style={name}>Aanya Rao</div>
      <div style={muted}>Laser Lab / laser_operator</div>
    </CardRow>
    <CardRow>
      <div style={name}>Ishita Sen</div>
      <div style={muted}>Laser Lab / laser_operator</div>
    </CardRow>
  </>
);

const List = (p: { isLoading?: boolean; error?: unknown; isEmpty?: boolean }) => (
  <Card>
    <CardHeader title="Laser operators" />
    <ListStates
      isLoading={!!p.isLoading}
      error={p.error ?? null}
      isEmpty={!!p.isEmpty}
      onRetry={() => {}}
      errorTitle="Could not load role members"
      empty={empty}
      skeleton={<RowSkeleton rows={3} label="Loading role members" />}
    >
      {rows}
    </ListStates>
  </Card>
);

export const Loaded = () => <List />;
export const Loading = () => <List isLoading />;
export const Empty = () => <List isEmpty />;
/** Error wins over empty: an empty list that is really a failed request is the costliest lie. */
export const Failed = () => <List isEmpty error={new Error("Syndra's server did not answer in time.")} />;
