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

/**
 * For a project name a server view already resolved (or fell back on) —
 * a person's access view groups roles under a `project_name` it computed
 * itself, with a same-request `project_name_resolved` verdict rather than a
 * client-side lookup. Unresolved renders the same honest way `UserName`
 * does: the id stays on screen, labelled, never standing alone in the name's
 * slot — an operator's project heading is the id an unresolved project is
 * searched by, not a fabricated name.
 */
export function ResolvedProjectName({
  name,
  resolved,
  id,
  className = "",
}: {
  name: string;
  resolved: boolean;
  id?: string;
  className?: string;
}) {
  if (resolved) {
    return <span className={className}>{name}</span>;
  }
  return (
    <span className={className}>
      <span className="text-faint">Unknown project</span>{" "}
      <span className="type-mono text-faint">{id ?? name}</span>
    </span>
  );
}
