"use client";

import { CopyableValue } from "@/components/ui/CopyableValue";

/**
 * The one designed fallback for a name nothing could place, so a person, a
 * project and a role all fail the same way (#35): a plain faint phrase in the
 * name's slot, never the id. The id is the only thing anyone can look up in
 * Zitadel, in Basic as much as in Advanced (A-07 §7), so it follows in both
 * views as faint mono text that copies. Every Name component and every server-view heading that cannot name
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
  idLabel = "id",
  className = "",
}: {
  /** Faint when it is the shared phrase; a caller's own wording is shown as given. */
  label: React.ReactNode;
  id?: string | null;
  /** What the id is, for the copy control's name: "account id", "project id". */
  idLabel?: string;
  className?: string;
}) {
  return (
    <span className={className}>
      {typeof label === "string" ? <span className="text-faint">{label}</span> : label}
      {id ? (
        <>
          {" "}
          <CopyableValue inline value={id} label={idLabel} />
        </>
      ) : null}
    </span>
  );
}
