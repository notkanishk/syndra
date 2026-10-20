"use client";

import { useIsAdvanced } from "@/lib/ui-view";

/**
 * The one designed fallback for a name nothing could place, so a person, a
 * project and a role all fail the same way (#35): a plain faint phrase in the
 * name's slot, never the id. The id is the forensic half of the row, so it
 * follows as faint mono text, and only in Advanced; Basic says it in words and
 * stops. Every Name component and every server-view heading that cannot name
 * its project comes here, so a degraded read has one look instead of a raw id
 * on each page that thought of its own fallback.
 */
export const UNKNOWN_ACCOUNT = "Unknown account";
export const UNKNOWN_PROJECT = "Unknown project";

/**
 * A project's name as a string, for the places that sort, filter or build a
 * sentence from it. The API names a project it could not read with its own id
 * (or with nothing), so a name that is empty or IS the id is not a name.
 */
export function projectLabel(name: string | null | undefined, id: string | null | undefined): string {
  return name && name !== id ? name : UNKNOWN_PROJECT;
}

export function Unresolved({
  label,
  id,
  className = "",
}: {
  /** Faint when it is the shared phrase; a caller's own wording is shown as given. */
  label: React.ReactNode;
  id?: string | null;
  className?: string;
}) {
  const advanced = useIsAdvanced();
  return (
    <span className={className}>
      {typeof label === "string" ? <span className="text-faint">{label}</span> : label}
      {advanced && id ? (
        <>
          {" "}
          <span className="type-mono text-faint">{id}</span>
        </>
      ) : null}
    </span>
  );
}
