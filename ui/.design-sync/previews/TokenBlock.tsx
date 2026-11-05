import { TokenBlock } from "ui";

export const Claims = () => (
  <TokenBlock
    comment="What the laser cutter app receives"
    lines={[
      { key: "sub", text: '"sub": "2481",', theirs: true },
      { key: "email", text: '"email": "aanya@example.edu",', theirs: true },
      { key: "roles", text: '"roles": ["operator", "trained"]' },
    ]}
  />
);

/** A draft marks each line in a gutter, so a difference never rests on colour. */
export const Draft = () => (
  <TokenBlock
    lines={[
      { key: "roles", text: '"roles": ["operator"],', mark: " " },
      { key: "bay", text: '"bay": "laser",', mark: "+" },
      { key: "old", text: '"team": "wood",', mark: "−" },
    ]}
  />
);

export const Raw = () => <TokenBlock raw={{ sub: "2481", roles: ["operator"] }} />;

export const Empty = () => <TokenBlock empty="{} Nothing is shaped for this app yet." />;
