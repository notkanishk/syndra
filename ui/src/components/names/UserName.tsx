"use client";

import { useEffect, useState } from "react";

import { Skeleton } from "@/components/ui/Skeleton";
import { useNameResolver } from "@/lib/queries/useNameResolver";

interface UserNameProps {
  /** Zitadel user ID. */
  id: string | null | undefined;
  /** Rendered when the id is empty/falsy or resolution misses. Default: em dash. */
  fallback?: React.ReactNode;
  /** When true, the email is rendered as a muted suffix after the display name. */
  showEmail?: boolean;
  /** Optional className applied to the wrapper span. */
  className?: string;
}

const SHOW_DEBUG_IDS =
  typeof window !== "undefined" && new URLSearchParams(window.location.search).has("debug");

/**
 * Renders the resolved display name for a Zitadel user id. While loading,
 * shows a Skeleton block. On miss, renders `fallback` (default em dash) with
 * the raw id available via the `title` attribute for forensic access.
 *
 * Single-tick batching is handled by the surrounding NameResolverProvider —
 * mounting many <UserName/>s in one render produces ONE /lookup request.
 */
export function UserName({ id, fallback = "—", showEmail = false, className = "" }: UserNameProps) {
  const resolver = useNameResolver();
  // useState forces a re-render when the resolver's cache fills in. The
  // resolver itself is memoized; we read from it once per render.
  const [, force] = useState(0);
  useEffect(() => {
    // Bump on mount so the next render re-reads the cache. The resolver
    // updates its internal Map asynchronously after the lookup resolves.
    const t = setTimeout(() => force((n) => n + 1), 0);
    return () => clearTimeout(t);
  }, [id]);

  if (!id) {
    return <span className={className}>{fallback}</span>;
  }

  if (id === "system" || id === "-") {
    return <span className={`text-on-surface-variant ${className}`}>System</span>;
  }

  const { value, resolved } = resolver.resolveUser(id);

  if (!value) {
    if (!resolved) {
      return (
        <span className={className} title={SHOW_DEBUG_IDS ? id : undefined} aria-busy="true">
          <Skeleton className="inline-block w-20 h-3 align-middle" />
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
      {value.display_name || (fallback as React.ReactNode)}
      {showEmail && value.email ? (
        <span className="text-on-surface-variant"> · {value.email}</span>
      ) : null}
    </span>
  );
}
