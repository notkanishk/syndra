"use client";

import { ArrowLink } from "@/components/ui/ArrowLink";
import { StatusDot, type StatusTone } from "@/components/ui/Badge";
import { Term } from "@/components/ui/Term";
import { formatClock, formatWhen } from "@/lib/format";
import { useClaimsStatus, type ClaimsStatus } from "@/lib/queries/useClaimsDelivery";

const DAY = 24 * 60 * 60 * 1000;

/**
 * Home's one claims line (claims-delivery-integrity §4g): always here, in the
 * same seat, whatever it says. "Claims: delivered to every sign-in in the last
 * 24h", or "Claims: 2 problems →" to System › Zitadel. `problems` is the count
 * the external alert acts on, so the two never disagree. Loading and a failed
 * first read say nothing about health.
 */
export function ClaimsLine() {
  const status = useClaimsStatus();
  const s = status.data;
  const tone: StatusTone | null = !s ? null : s.status === "ok" ? "healthy" : s.status === "failing" ? "danger" : "warn";

  return (
    <div className="flex min-h-6 flex-wrap items-center gap-x-2 gap-y-1 pt-1 text-[13.5px] text-muted">
      {tone && <StatusDot tone={tone} />}
      <span>
        <Term name="claim">Claims</Term>:
      </span>
      {!s ? (
        <span className="text-faint">{status.error ? "Couldn't check how delivery is going." : "checking"}</span>
      ) : s.status === "ok" ? (
        <span>{calmWords(s)}</span>
      ) : (
        <ArrowLink href="/zitadel#claims-delivery">
          {s.problems === 1 ? "1 problem" : s.problems > 1 ? `${s.problems} problems` : "needs a look"}
        </ArrowLink>
      )}
      {s && status.error && <span className="text-faint">{" "}· as of {formatClock(new Date(status.dataUpdatedAt).toISOString())}</span>}
    </div>
  );
}

/** The calm sentence's end: since a restart under a day ago, the time it began. */
function calmWords(s: ClaimsStatus, now = Date.now()): string {
  const since = new Date(s.checking_since).getTime();
  const span = now - since < DAY ? `since ${formatWhen(s.checking_since)}` : "in the last 24h";
  const n = s.sign_ins_24h;
  const checked = (n.delivered ?? 0) + (n.not_called ?? 0) + (n.not_served ?? 0) + (n.first_sign_in_before_access ?? 0) + (n.nothing_to_deliver ?? 0);
  return checked === 0 ? `no sign-ins to check ${span}` : `delivered to every sign-in ${span}`;
}
