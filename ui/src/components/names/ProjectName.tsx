"use client";

import { NameLink, nameHref, useNameLinks } from "@/components/names/NameLink";
import { Unresolved, UNKNOWN_PROJECT } from "@/components/names/Unresolved";
import { Skeleton, useTintAfterSkeleton } from "@/components/ui/Skeleton";
import { SHOW_DEBUG_IDS } from "@/components/names/UserName";
import { useNameResolver } from "@/lib/queries/useNameResolver";

interface ProjectNameProps {
  id: string | null | undefined;
  /**
   * The name a server view already carries for this project. The API sends no
   * name ("") for a project it could not read (one-path W2d), never its id; an
   * empty one falls through to the resolver and then the shared fallback.
   */
  name?: string | null;
  fallback?: React.ReactNode;
  className?: string;
  /** Links to the project's page (G14). Only a page's own subject passes false. */
  link?: boolean;
}

export function ProjectName({ id, name, fallback, className = "", link: linkProp }: ProjectNameProps) {
  const inDialog = useNameLinks();
  const link = linkProp ?? inDialog;
  const resolver = useNameResolver();
  const lookup = Boolean(id) && !name && id !== "-";
  const { value, resolved } = lookup ? resolver.resolveProject(id ?? "") : { value: undefined, resolved: true };
  // The name that lands after a skeleton tints in, not snaps (S7).
  const tint = useTintAfterSkeleton(lookup && !value && !resolved);

  if (!id) {
    return <span className={className}>{fallback ?? "—"}</span>;
  }
  if (name) {
    return linked(id, name, className, link);
  }
  if (id === "-") {
    return <span className={`text-muted ${className}`}>—</span>;
  }

  if (!value && !resolved) {
    return (
      <span className={className} title={SHOW_DEBUG_IDS ? id : undefined} aria-busy="true">
        <Skeleton className="inline-block w-16 h-3 align-middle" />
      </span>
    );
  }
  if (!value?.name) {
    return <Unresolved label={fallback ?? UNKNOWN_PROJECT} id={id} idLabel="project id" href={link ? nameHref.project(id) : undefined} className={`${className} ${tint}`} />;
  }

  return linked(id, value.name, `${className} ${tint}`, link, SHOW_DEBUG_IDS ? id : undefined);
}

function linked(id: string, text: string, className: string, link: boolean, title?: string) {
  return link ? (
    <NameLink href={nameHref.project(id)} className={className} title={title}>
      {text}
    </NameLink>
  ) : (
    <span className={className} title={title}>
      {text}
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
  link: linkProp,
}: {
  name: string;
  resolved: boolean;
  id?: string;
  className?: string;
  link?: boolean;
}) {
  const inDialog = useNameLinks();
  const link = linkProp ?? inDialog;
  if (resolved) {
    return id ? linked(id, name, className, link) : <span className={className}>{name}</span>;
  }
  return <Unresolved label={UNKNOWN_PROJECT} id={id ?? name} idLabel="project id" className={className} />;
}
