import { Skeleton, Card } from "ui";

const stack = { display: "flex", flexDirection: "column", gap: 12, padding: 16 } as const;

/** Representative geometry: the bars match the row they stand in for. */
export const PersonRow = () => (
  <Card>
    <div style={{ display: "flex", alignItems: "center", gap: 12, padding: 16 }}>
      <Skeleton style={{ width: 34, height: 34, borderRadius: 999 }} />
      <div style={{ display: "flex", flexDirection: "column", gap: 6, flex: 1 }}>
        <Skeleton style={{ height: 16, width: "40%" }} />
        <Skeleton style={{ height: 14, width: "65%" }} />
      </div>
    </div>
  </Card>
);

export const TextBlock = () => (
  <div style={stack}>
    <Skeleton style={{ height: 20, width: "33%" }} />
    <Skeleton style={{ height: 16, width: "90%" }} />
    <Skeleton style={{ height: 16, width: "75%" }} />
  </div>
);
