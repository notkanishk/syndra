import type { Draft, Profile } from "@/lib/queries/useClaimShape";
import { pathKey } from "@/lib/claims";

/**
 * A shape not saved yet. `committed` is false while it is only previewed from
 * a chooser row; Start or Copy commits it into the card's rows. Nothing is
 * written until Save.
 */
export interface ShapeDraft {
  profile: Profile;
  origin: "edit" | "template" | "copy";
  committed: boolean;
  templateId?: string;
  templateVersion?: number;
  copiedFrom?: { application_id: string; version: number };
  copiedName?: string;
  notCopied: Array<{ role_key: string; name: string }>;
  /** A copied if-role row's lost role, by the row's path. */
  missing: Record<string, string>;
}

export function draftFrom(d: Draft, origin: "template" | "copy", copiedName?: string): ShapeDraft {
  const missing: Record<string, string> = {};
  for (const m of d.missing_roles ?? []) {
    const row = d.profile.claims[m.row];
    if (row) missing[pathKey(row.path)] = m.name || m.role_key;
  }
  return {
    profile: d.profile,
    origin,
    committed: false,
    templateId: d.template_id || undefined,
    templateVersion: d.template_version || undefined,
    copiedFrom: d.copied_from ?? undefined,
    copiedName,
    notCopied: d.not_copied ?? [],
    missing,
  };
}
