import { Avatar } from "ui";

const row = { display: "flex", alignItems: "center", gap: 16 } as const;
const item = { display: "flex", alignItems: "center", gap: 10, fontSize: 15, fontWeight: 600 } as const;

export const Sizes = () => (
  <div style={row}>
    <Avatar name="Aanya Rao" size="row" />
    <Avatar name="Kabir Mehta" size="list" />
    <Avatar name="Ishita Sen" size="header" />
  </div>
);

export const WithName = () => (
  <div style={{ display: "flex", flexDirection: "column", gap: 12 }}>
    <span style={item}><Avatar name="Rohan Iyer" />Rohan Iyer</span>
    <span style={item}><Avatar name="Meera Pillai" />Meera Pillai</span>
  </div>
);

/** A single name gives two letters; a missing one leaves the shape blank. */
export const SingleAndMissing = () => (
  <div style={row}>
    <Avatar name="Priyanka" />
    <Avatar name={null} />
  </div>
);
