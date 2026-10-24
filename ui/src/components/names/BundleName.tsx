"use client";

import { Skeleton, useTintAfterSkeleton } from "@/components/ui/Skeleton";
import { SHOW_DEBUG_IDS } from "@/components/names/UserName";
import { useNameResolver } from "@/lib/queries/useNameResolver";

interface BundleNameProps {
  id: string | null | undefined;
  fallback?: React.ReactNode;
  className?: string;
}

export function BundleName({ id, fallback = "—", className = "" }: BundleNameProps) {
  const resolver = useNameResolver();
  const { value, resolved } = id ? resolver.resolveBundle(id) : { value: undefined, resolved: true };
  // The name that lands after a skeleton tints in, not snaps (S7).
  const tint = useTintAfterSkeleton(Boolean(id) && !value && !resolved);

  if (!id) {
    return <span className={className}>{fallback}</span>;
  }

  if (!value) {
    if (!resolved) {
      return (
        <span className={className} title={SHOW_DEBUG_IDS ? id : undefined} aria-busy="true">
          <Skeleton className="inline-block w-16 h-3 align-middle" />
        </span>
      );
    }
    return (
      <span className={`${className} ${tint}`} title={SHOW_DEBUG_IDS ? id : undefined}>
        {fallback}
      </span>
    );
  }

  return (
    <span className={`${className} ${tint}`} title={SHOW_DEBUG_IDS ? id : undefined}>
      {value.name || (fallback as React.ReactNode)}
    </span>
  );
}
