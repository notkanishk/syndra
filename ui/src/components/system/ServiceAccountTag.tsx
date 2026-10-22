"use client";

import { serviceAccountHref, useServiceAccountIds } from "@/lib/queries/useServiceAccounts";

/**
 * Where access is checked, a service account stays in the list (A-07 §29): its
 * description says "Service account" where a person's title would be, and its
 * name opens its own page rather than a person's.
 */
export function useAccountHref(): (id: string) => string {
  const ids = useServiceAccountIds();
  return (id) => (ids.has(id) ? serviceAccountHref(id) : `/users/${id}`);
}

export function ServiceAccountTag({ id, className = "" }: { id: string; className?: string }) {
  const ids = useServiceAccountIds();
  if (!ids.has(id)) return null;
  return <span className={`block text-[12.5px] text-faint ${className}`}>Service account</span>;
}
