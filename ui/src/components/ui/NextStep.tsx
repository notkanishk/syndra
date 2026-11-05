import { AdvancedLink } from "@/components/ui/AdvancedLink";

import { StatusDot } from "@/components/ui/Badge";

/**
 * A step left, said where the first one was taken (proposed/NextStep.md).
 *
 * The status the thing that changed wears until the operator sends it: an
 * amber dot and "Waiting for you to send →" to Pending changes, with one faint
 * line under it saying what isn't true yet and why. Never "waiting to be
 * sent", which reads as automatic; never a box, a new colour, or a send made
 * on the operator's behalf.
 */
export function NextStep({
  why,
  href,
  person,
  role,
  dot = true,
  className = "",
}: {
  /** What isn't true yet, by name: "He can't supervise yet · Supervisor always waits for you". */
  why?: string;
  href?: string;
  /** A person's Zitadel id: Pending changes opens narrowed to them, with the way out attached (A-19 P3). */
  person?: string;
  /** A role, as project id and role key: Pending changes opens narrowed to it, with the way out attached. */
  role?: { projectId: string; roleKey: string };
  /** Off where a status word above already wears the amber dot (ActionOutcome's seat). */
  dot?: boolean;
  className?: string;
}) {
  const to =
    href ??
    (person
      ? `/governance/pending?person=${encodeURIComponent(person)}`
      : role
        ? `/governance/pending?role=${encodeURIComponent(`${role.projectId}/${role.roleKey}`)}`
        : "/governance/pending");
  return (
    <span className={`inline-flex flex-col gap-0.5 ${className}`}>
      <span className="inline-flex items-center gap-2">
        {dot && <StatusDot tone="warn" />}
        <AdvancedLink
          href={to}
          className="text-[13.5px] font-semibold text-accent-text link-hover"
        >
          Waiting for you to send <span aria-hidden>→</span>
        </AdvancedLink>
      </span>
      {why && <span className="pl-[15px] text-[12.5px] text-faint">{why}</span>}
    </span>
  );
}
