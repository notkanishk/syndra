import { BundleName } from "ui";

const col = { display: "flex", flexDirection: "column", gap: 10, fontSize: 15 } as const;

/**
 * Bundle names resolve from the name catalog (`/api`), not served here, so an
 * id shows its fallback - which is what a retired bundle reads as in the app.
 */
export const RetiredBundle = () => (
  <p style={{ fontSize: 14.5, color: "var(--muted)", margin: 0 }}>
    Access was granted by <BundleName id="bdl_woodshop_starter" fallback="a bundle since retired" />.
  </p>
);

export const FallbacksAndEmpty = () => (
  <div style={col}>
    <BundleName id="bdl_laser_operator" fallback="Laser operator kit" />
    <BundleName id={null} />
  </div>
);
