import { RoleRef } from "@/components/names";
import type { BundleDraft, IncludeChange } from "@/lib/queries/useBundleVersions";

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
        <span aria-label="Removes" className="w-3 font-semibold text-warn-text">
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

/**
 * Everything the next version changes, one line each: the bundles it takes in or lets go, then
 * the roles it adds and removes. The unpublished card and the publish dialog both read it here,
 * so the two can't say different things.
 */
export function ChangeLines({ draft, className = "" }: { draft: BundleDraft; className?: string }) {
  return (
    <ul className={`flex flex-col gap-1.5 text-[14px] ${className}`}>
      {(draft.include_changes ?? []).map((change) => (
        <li key={`i${change.child_bundle_id}`} className="flex items-baseline gap-2">
          <IncludeChangeLine change={change} />
        </li>
      ))}
      {draft.added.map((role) => (
        <li key={`+${role.zitadel_project_id}:${role.zitadel_role_key}`} className="flex items-baseline gap-2">
          <span aria-label="Adds" className="w-3 font-semibold text-accent-text">
            +
          </span>
          <RoleRef projectId={role.zitadel_project_id} roleKey={role.zitadel_role_key} />
        </li>
      ))}
      {draft.removed.map((role) => (
        <li key={`-${role.zitadel_project_id}:${role.zitadel_role_key}`} className="flex items-baseline gap-2">
          <span aria-label="Removes" className="w-3 font-semibold text-warn-text">
            &minus;
          </span>
          <RoleRef projectId={role.zitadel_project_id} roleKey={role.zitadel_role_key} />
        </li>
      ))}
    </ul>
  );
}
