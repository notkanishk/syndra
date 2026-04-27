"use client";

import { useEffect, useState } from "react";

import { Skeleton } from "@/components/ui/Skeleton";
import { useNameResolver } from "@/lib/queries/useNameResolver";

interface BundleNameProps {
  id: string | null | undefined;
  fallback?: React.ReactNode;
  className?: string;
}

const SHOW_DEBUG_IDS =
  typeof window !== "undefined" && new URLSearchParams(window.location.search).has("debug");

export function BundleName({ id, fallback = "—", className = "" }: BundleNameProps) {
  const resolver = useNameResolver();
  const [, force] = useState(0);
  useEffect(() => {
    const t = setTimeout(() => force((n) => n + 1), 0);
    return () => clearTimeout(t);
  }, [id]);

  if (!id) {
    return <span className={className}>{fallback}</span>;
  }

  const { value, resolved } = resolver.resolveBundle(id);

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
