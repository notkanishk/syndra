import { Card, CardRow, WithheldInline } from "ui";

const name = { fontSize: 15, fontWeight: 600 } as const;

/** One line per hold, with the reason, inside a dense holder list. */
export const InHolderList = () => (
  <Card>
    <CardRow first>
      <div>
        <div style={name}>Rohan Iyer</div>
        <WithheldInline items={[{ field: "group", value: "cnc-router", target: "Door access", reason: "Waiting on a supervisor sign-off" }]} />
      </div>
    </CardRow>
    <CardRow>
      <div style={name}>Meera Pillai</div>
    </CardRow>
  </Card>
);

export const ReviewDue = () => (
  <WithheldInline
    items={[
      { field: "share", value: "capstone-2026", target: "TrueNAS", reason: "Over the storage quota", reviewDue: true },
      { field: "group", value: "wood-shop-door", target: "Door access", reason: "Safety induction is due for renewal" },
    ]}
  />
);
