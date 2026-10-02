import { FilterChip } from "ui";

const row = { display: "flex", flexWrap: "wrap", alignItems: "center", gap: 8 } as const;

export const Single = () => (
  <div style={row}>
    <FilterChip onClear={() => {}} clearLabel="Clear filter: holding laser_cutter.operator">
      Holding laser_cutter.operator in Laser lab
    </FilterChip>
  </div>
);

/** Bundle and version clear independently, so they are two chips. */
export const BundleAndVersion = () => (
  <div style={row}>
    <FilterChip onClear={() => {}} clearLabel="Clear filter: in the Wood shop basics bundle">
      In the Wood shop basics bundle
    </FilterChip>
    <FilterChip onClear={() => {}} clearLabel="Clear filter: on v3">
      on v3
    </FilterChip>
  </div>
);
