"use client";

import { Skeleton } from "@/components/ui/Skeleton";
import { SHOW_DEBUG_IDS } from "@/components/names/UserName";
import { useNameResolver } from "@/lib/queries/useNameResolver";

interface ProjectNameProps {
  id: string | null | undefined;
  fallback?: React.ReactNode;
  className?: string;
}

export function ProjectName({ id, fallback = "—", className = "" }: ProjectNameProps) {
  const resolver = useNameResolver();

  if (!id) {
    return <span className={className}>{fallback}</span>;
  }
  if (id === "-") {
    return <span className={`text-muted ${className}`}>—</span>;
  }

  const { value, resolved } = resolver.resolveProject(id);

  if (!value) {
    if (!resolved) {
      return (
        <span className={className} title={SHOW_DEBUG_IDS ? id : undefined} aria-busy="true">
          <Skeleton className="inline-block w-16 h-3 align-middle" />
        </span>
      );
    }
    return (
      <span className={className} title={SHOW_DEBUG_IDS ? id : undefined}>
        {fallback}
      </span>
    );
  }

  return (
    <span className={className} title={SHOW_DEBUG_IDS ? id : undefined}>
      {value.name || (fallback as React.ReactNode)}
    </span>
  );
}
