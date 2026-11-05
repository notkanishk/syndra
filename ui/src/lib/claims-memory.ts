import type { ShapeDraft } from "@/components/apps/shape-draft";

/**
 * What an app's claims page keeps for this tab, so leaving it and coming back
 * is not losing it (A-19 P10.6): the draft, and who "Preview for" was showing.
 * Session memory only, per app, never across visits. Every access is guarded:
 * storage can be absent or full, and the page works without it.
 */
const draftKey = (appId: string) => `syndra:claims-draft:${appId}`;
const personKey = (appId: string) => `syndra:claims-person:${appId}`;

export type KeptDraft = { draft: ShapeDraft; at: string };

export function readKeptDraft(appId: string): KeptDraft | null {
  try {
    const raw = window.sessionStorage.getItem(draftKey(appId));
    if (!raw) return null;
    const kept = JSON.parse(raw) as KeptDraft;
    return kept?.draft?.profile && kept.at ? kept : null;
  } catch {
    return null;
  }
}

/** A draft is kept once it is committed to the card's rows; a chooser's preview is not. `null` forgets it. */
export function keepDraft(appId: string, draft: ShapeDraft | null): void {
  try {
    if (draft?.committed) {
      window.sessionStorage.setItem(draftKey(appId), JSON.stringify({ draft, at: new Date().toISOString() }));
    } else window.sessionStorage.removeItem(draftKey(appId));
  } catch {
    /* nothing to keep it in */
  }
}

export function readKeptPerson(appId: string): string {
  try {
    return window.sessionStorage.getItem(personKey(appId)) ?? "";
  } catch {
    return "";
  }
}

export function keepPerson(appId: string, personId: string): void {
  try {
    if (personId) window.sessionStorage.setItem(personKey(appId), personId);
  } catch {
    /* nothing to keep it in */
  }
}
