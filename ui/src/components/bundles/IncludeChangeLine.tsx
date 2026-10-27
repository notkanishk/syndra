import type { IncludeChange } from "@/lib/queries/useBundleVersions";

/** "+ Safety basics v3", "Safety basics v3 → v4", "− Safety basics". */
export function IncludeChangeLine({ change }: { change: IncludeChange }) {
  if (change.from_version === 0) {
    return (
      <>
        <span aria-label="Adds" className="w-3 font-semibold text-accent-text">
          +
        </span>
        <span>
          {change.child_name} v{change.to_version}
        </span>
      </>
    );
  }
  if (change.to_version === 0) {
    return (
      <>
        <span aria-label="Drops" className="w-3 font-semibold text-warn-text">
          &minus;
        </span>
        <span>{change.child_name}</span>
      </>
    );
  }
  return (
    <>
      <span aria-hidden className="w-3" />
      <span>
        {change.child_name} v{change.from_version} → v{change.to_version}
      </span>
    </>
  );
}
