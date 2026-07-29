"use client";

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

// With ?debug in the URL, Name components expose the raw id via `title` for
// forensic access. Shared by all four Name components.
export const SHOW_DEBUG_IDS =
  typeof window !== "undefined" && new URLSearchParams(window.location.search).has("debug");

/**
 * Renders the resolved display name for a Zitadel user id. While loading,
 * shows a Skeleton block. On miss, renders `fallback` (default em dash) with
 * the raw id available via the `title` attribute for forensic access.
 *
 * Re-rendering when the catalog lands is the NameResolverProvider's job: its
 * context value is memoized on the catalog query state, so consumers re-render
 * exactly when resolution data changes.
 */
export function UserName({ id, fallback = "—", showEmail = false, className = "" }: UserNameProps) {
  const resolver = useNameResolver();

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
