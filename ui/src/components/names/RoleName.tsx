"use client";

import { useEffect, useState } from "react";

import { Skeleton } from "@/components/ui/Skeleton";
import { useNameResolver } from "@/lib/queries/useNameResolver";

interface RoleNameProps {
  projectId: string | null | undefined;
  roleKey: string | null | undefined;
  /** Fallback when resolution misses. Defaults to the raw role_key for power-user readability. */
  fallback?: React.ReactNode;
  className?: string;
}

const SHOW_DEBUG_IDS =
  typeof window !== "undefined" && new URLSearchParams(window.location.search).has("debug");

export function RoleName({ projectId, roleKey, fallback, className = "" }: RoleNameProps) {
  const resolver = useNameResolver();
  const [, force] = useState(0);
  useEffect(() => {
    const t = setTimeout(() => force((n) => n + 1), 0);
    return () => clearTimeout(t);
  }, [projectId, roleKey]);

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
