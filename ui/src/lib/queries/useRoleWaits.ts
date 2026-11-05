"use client";

import { useRolesThatWait } from "@/lib/queries/useRolesThatWait";

/**
 * Whether a role is one of Roles that wait. False until the list has been read:
 * a forecast is Syndra's own next step, so it names a wait only when it knows.
 * Its own module so a surface that forecasts can be tested without the list.
 */
export function useRoleWaits(): (projectId: string, roleKey: string) => boolean {
  const waits = useRolesThatWait().data?.roles ?? [];
  return (projectId, roleKey) => waits.some((r) => r.project_id === projectId && r.role_key === roleKey);
}
