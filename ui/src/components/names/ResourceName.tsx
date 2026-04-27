"use client";

import { BundleName } from "@/components/names/BundleName";
import { ProjectName } from "@/components/names/ProjectName";
import { RoleName } from "@/components/names/RoleName";
import { UserName } from "@/components/names/UserName";

export type ResourceKind = "user" | "project" | "role" | "bundle" | string;

interface ResourceNameProps {
  /** Discriminator picked from the audit log row's target_kind / resource_kind. */
  kind: ResourceKind | null | undefined;
  /** ID for user/project/bundle. For role, may be the composite "<project_id>:<role_key>". */
  id: string | null | undefined;
  fallback?: React.ReactNode;
  className?: string;
}

/**
 * Dispatches to the right Name component based on a runtime kind. Used by
 * audit-log rows where the same column may render any of the four entity
 * types depending on the action. For role kind, the id may be either the
 * composite "<project_id>:<role_key>" or a bare role key (in which case
 * resolution will silently miss → fallback).
 */
export function ResourceName({ kind, id, fallback = "—", className = "" }: ResourceNameProps) {
  if (!kind || !id || id === "-") {
    return <span className={className}>{fallback}</span>;
  }
  switch (kind) {
    case "user":
      return <UserName id={id} fallback={fallback} className={className} />;
    case "project":
      return <ProjectName id={id} fallback={fallback} className={className} />;
    case "bundle":
      return <BundleName id={id} fallback={fallback} className={className} />;
    case "role": {
      const idx = id.indexOf(":");
      if (idx === -1) {
        return <span className={className}>{id}</span>;
      }
      return (
        <RoleName
          projectId={id.slice(0, idx)}
          roleKey={id.slice(idx + 1)}
          fallback={fallback}
          className={className}
        />
      );
    }
    default:
      return <span className={className}>{id}</span>;
  }
}
