import { BADGE_TONE, retryBadge } from "@/components/shell/navTones";

/**
 * The retry pill beside a rail row's own count (lib/nav.ts `retries`): "↻ 2"
 * in amber while Syndra sends changes again on its own, danger once it gave up
 * on one nobody has marked seen. One component for the rail and both touch
 * renderings, so a thumb and a pointer read the same thing.
 */
export function RetryPill({
  counts,
  className = "",
}: {
  counts: { retrying?: number; gave_up?: number } | undefined;
  className?: string;
}) {
  const badge = retryBadge(counts);
  if (!badge) return null;
  return (
    <span
      title={badge.label}
      className={`rounded-pill px-1.5 py-0.5 text-[12.5px] font-semibold leading-none ${BADGE_TONE[badge.tone]} ${className}`}
    >
      <span aria-hidden>{badge.tone === "danger" ? "!" : "↻"} </span>
      {badge.count}
      <span className="sr-only">. {badge.label}.</span>
    </span>
  );
}
