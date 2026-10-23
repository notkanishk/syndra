"use client";

import { Unresolved, UNKNOWN_PROJECT } from "@/components/names/Unresolved";
import { Skeleton } from "@/components/ui/Skeleton";
import { SHOW_DEBUG_IDS } from "@/components/names/UserName";
import { useNameResolver } from "@/lib/queries/useNameResolver";

interface ProjectNameProps {
  id: string | null | undefined;
  /**
   * The name a server view already carries for this project. Used when it is a
   * real name; the API sends the id (or nothing) for a project it could not
   * read, and that falls through to the resolver and then the shared fallback.
   */
  name?: string | null;
  fallback?: React.ReactNode;
  className?: string;
}

export function ProjectName({ id, name, fallback, className = "" }: ProjectNameProps) {
  const resolver = useNameResolver();

  if (!id) {
    return <span className={className}>{fallback ?? "—"}</span>;
  }
  if (name && name !== id) {
    return <span className={className}>{name}</span>;
  }
  if (id === "-") {
    return <span className={`text-muted ${className}`}>—</span>;
  }

  const { value, resolved } = resolver.resolveProject(id);

  if (!value && !resolved) {
    return (
      <span className={className} title={SHOW_DEBUG_IDS ? id : undefined} aria-busy="true">
        <Skeleton className="inline-block w-16 h-3 align-middle" />
      </span>
    );
  }
  if (!value?.name) {
    return <Unresolved label={fallback ?? UNKNOWN_PROJECT} id={id} idLabel="project id" className={className} />;
  }

  return (
    <span className={className} title={SHOW_DEBUG_IDS ? id : undefined}>
      {value.name}
    </span>
  );
}

/**
 * For a project name a server view already resolved (or fell back on) —
 * a person's access view groups roles under a `project_name` it computed
 * itself, with a same-request `project_name_resolved` verdict rather than a
 * client-side lookup. Unresolved renders the same honest way `UserName`
 * does: the shared "Unknown project" phrase, with the id beside it in
 * Advanced, never standing alone in the name's slot.
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
  return <Unresolved label={UNKNOWN_PROJECT} id={id ?? name} idLabel="project id" className={className} />;
}
