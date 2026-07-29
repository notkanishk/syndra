"use client";

import { Skeleton } from "@/components/ui/Skeleton";
import { SHOW_DEBUG_IDS } from "@/components/names/UserName";
import { useNameResolver } from "@/lib/queries/useNameResolver";

interface RoleNameProps {
  projectId: string | null | undefined;
  roleKey: string | null | undefined;
  /** Fallback when resolution misses. Defaults to the raw role_key for power-user readability. */
  fallback?: React.ReactNode;
  className?: string;
}

export function RoleName({ projectId, roleKey, fallback, className = "" }: RoleNameProps) {
  const resolver = useNameResolver();

  if (!projectId || !roleKey) {
    return <span className={className}>{fallback ?? "—"}</span>;
  }

  const { value, resolved } = resolver.resolveRole(projectId, roleKey);

  if (!value) {
    if (!resolved) {
      return (
        <span
          className={className}
          title={SHOW_DEBUG_IDS ? `${projectId}:${roleKey}` : undefined}
          aria-busy="true"
        >
          <Skeleton className="inline-block w-20 h-3 align-middle" />
        </span>
      );
    }
    return (
      <span className={className} title={SHOW_DEBUG_IDS ? `${projectId}:${roleKey}` : undefined}>
        {fallback ?? roleKey}
      </span>
    );
  }

  return (
    <span className={className} title={SHOW_DEBUG_IDS ? `${projectId}:${roleKey}` : undefined}>
      {value.display_name || fallback || roleKey}
    </span>
  );
}
