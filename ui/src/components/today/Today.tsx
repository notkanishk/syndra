"use client";

import Link from "next/link";
import { useState } from "react";
import { toast } from "sonner";

import { AccessSource } from "@/components/access/AccessSource";
import { Makerspace } from "@/components/today/Makerspace";
import { ErrorState, RowSkeleton } from "@/components/states";
import { Button, ButtonLink } from "@/components/ui/Button";
import { Card, CardHeader, CardRow } from "@/components/ui/Card";
import { Mono } from "@/components/ui/Badge";
import { ProjectName, UserAvatar, UserName } from "@/components/names";
import { useGovernanceSummary } from "@/lib/queries/useGovernance";
import {
  useDecideRequest,
  useRequestsAdmin,
  type AccessRequest,
} from "@/lib/queries/useRequests";
import { useDrainPropagations } from "@/lib/queries/usePropagation";
import { useCreateGrant } from "@/lib/queries/useUsers";
import type { SessionUser } from "@/lib/session";
import { useIsAdvanced } from "@/lib/ui-view";
import { ClockTime, Relative } from "@/components/ui/Time";
import { formatShortDate, formatWeekday, daysUntil } from "@/lib/format";

/**
 * Today — the operator landing. Two zones, in a fixed order.
 *
 * **Work, on top.** Open requests, expiring access, and — in Advanced — pending
 * changes and unexplained access. Every block here is something you can finish.
 * Nothing is ever inserted above this zone, and nothing below it can push it
 * down: the queue is the first thing under the cursor or the page has failed.
 *
 * **The makerspace, below.** Always present, including on a day when the queue
 * is empty.
 *
 * That second zone is a deliberate revision of this page's original contract,
 * which read "actionable work only — no counts you cannot act on, no charts".
 * That rule was right about the top of the page and wrong about the rest of it:
 * it assumed a non-empty queue, and most days the queue is empty. An operator
 * landing on "Nothing needs you." and nothing else learns nothing about the
 * space they run, so they go hunting through the nav — which is the navigation
 * this page exists to prevent. The half of the rule worth keeping is kept, and
 * enforced below: **every number is a link into the thing it counts.** Still no
 * charts, still nothing you can only look at.
 *
 * Basic shows two work blocks. Advanced appends two more, because Pending
 * changes and Unexplained access belong to the machine, and the machine lives
 * in Advanced.
 */
export function Today({ session }: { session: SessionUser }) {
  const advanced = useIsAdvanced();
  const summary = useGovernanceSummary();
  const requests = useRequestsAdmin("pending");

  const pending = requests.data ?? [];
  const expiring = summary.data?.expiring_grants ?? [];
  const propagation = summary.data?.pending_propagation;
  const drift = summary.data?.drift;

  const blocks = advanced
    ? pending.length + expiring.length + (propagation?.count ?? 0) + (drift?.count ?? 0)
    : pending.length + expiring.length;

  const who = firstName(session);
  const loading = summary.isLoading || requests.isLoading;
  const error = summary.error ?? requests.error;
  // While the load is failing the count is unknown, and "nothing needs you" is
  // exactly the wrong thing to say to somebody whose queue might be full.
  const headline = error ? "Couldn't check." : workSentence(blocks, loading);

  return (
    <div className="flex flex-col gap-6">
      <div>
        <h1 className="type-greeting">
          {greeting()}
          {who ? `, ${who}` : ""}. <span className="text-ink/40">{headline}</span>
        </h1>
        <div className="mt-2 text-[14.5px] text-faint">
          {formatWeekday()} · last checked <ClockTime />
        </div>
      </div>

      {error ? (
        <ErrorState
          title="Couldn't load today's work."
          error={error}
          onRetry={() => {
            summary.refetch();
            requests.refetch();
          }}
        />
      ) : loading ? (
        <Card>
          <RowSkeleton rows={4} label="Loading today's work" />
        </Card>
      ) : blocks === 0 ? (
        // One calm line, not a full-page card: the page continues below, and a
        // hero-sized empty state would read as the end of it.
        <div className="panel flex items-center gap-3 px-5 py-4">
          <span aria-hidden className="h-2.5 w-2.5 rounded-pill bg-accent" />
          <span className="text-[14.5px]">
            <strong className="font-semibold">Nothing needs you.</strong>{" "}
            <span className="text-muted">
              No open requests, and no access expiring in the next 14 days. Checked <ClockTime />.
            </span>
          </span>
        </div>
      ) : (
        <>
          {pending.length > 0 && <OpenRequests requests={pending} />}
          {expiring.length > 0 && <ExpiringSoon grants={expiring} />}

          {advanced && (propagation?.count ?? 0) > 0 && (
            <PendingChanges
              count={propagation!.count}
              reachable={propagation!.zitadel_reachable}
            />
          )}
          {advanced && (drift?.count ?? 0) > 0 && <UnexplainedAccess count={drift!.count} />}

          {!advanced && (
            <p className="max-w-[820px] text-[14px] leading-[1.55] text-faint">
              In Basic these blocks are your whole queue. Pending changes and unexplained access
              aren&rsquo;t hidden from you — they belong to the machine, and the machine lives in
              Advanced.
            </p>
          )}
        </>
      )}

      {/* The work zone above never moves. This is appended, never inserted. */}
      {!error && !loading && <Makerspace />}
    </div>
  );
}

/** Approve / Deny resolve in place with a toast and remove the row. They never navigate. */
function OpenRequests({ requests }: { requests: AccessRequest[] }) {
  const decide = useDecideRequest();
  const [resolved, setResolved] = useState<Set<string>>(new Set());

  const visible = requests.filter((entry) => !resolved.has(entry.id));
  if (visible.length === 0) return null;

  async function act(id: string, status: "approved" | "rejected", who: string) {
    setResolved((prev) => new Set(prev).add(id));
    try {
      await decide.mutateAsync({ id, status });
      toast.success(status === "approved" ? `Approved for ${who}` : `Denied for ${who}`);
    } catch (error) {
      // Put the row back: a row that vanished on a failed write would read as
      // a decision that was recorded.
      setResolved((prev) => {
        const next = new Set(prev);
        next.delete(id);
        return next;
      });
      toast.error(error instanceof Error ? error.message : "The decision didn't go through.");
    }
  }

  return (
    <Card>
      <CardHeader
        title="Open requests"
        count={visible.length}
        action={
          <Link href="/requests" className="text-[13.5px] font-semibold text-accent-text">
            See all →
          </Link>
        }
      />
      {visible.map((entry) => (
        <CardRow key={entry.id}>
          <UserAvatar id={entry.requester_id} size="list" />
          <div className="w-[170px] shrink-0 truncate text-[15px] font-semibold">
            <UserName id={entry.requester_id} />
          </div>
          <div className="w-[250px] shrink-0 truncate text-[14.5px] text-ink/80">
            <ProjectName id={entry.project_id} /> / <Mono>{entry.role_key}</Mono>
          </div>
          <div className="min-w-0 flex-1 truncate text-[14px] text-muted">
            {entry.justification ? `“${entry.justification}”` : "No reason given"}
          </div>
          <div className="w-[66px] shrink-0 text-[13px] text-faint">
            <Relative iso={entry.created_at} />
          </div>
          <div className="flex shrink-0 gap-2">
            <Button
              variant="accent"
              size="sm"
              onClick={() => act(entry.id, "approved", entry.requester_id)}
            >
              Approve
            </Button>
            <Button size="sm" onClick={() => act(entry.id, "rejected", entry.requester_id)}>
              Deny
            </Button>
          </div>
        </CardRow>
      ))}
    </Card>
  );
}

/**
 * Extending is the only action. Doing nothing is not an action and is not
 * drawn as one — the row states the outcome of inaction in words.
 */
function ExpiringSoon({
  grants,
}: {
  grants: Array<{ id: string; user_id: string; project_id: string; role_key: string; expires_at?: string | null }>;
}) {
  return (
    <Card>
      <CardHeader
        title="Direct access expiring soon"
        count={grants.length}
        tone="warn"
        note="Extending is the only action — doing nothing lets it lapse."
      />
      {grants.map((grant) => (
        <ExpiringRow key={grant.id} grant={grant} />
      ))}
    </Card>
  );
}

function ExpiringRow({
  grant,
}: {
  grant: { id: string; user_id: string; project_id: string; role_key: string; expires_at?: string | null };
}) {
  // Extending re-submits the grant with a later date: POST upserts on
  // (user, project, role), so this renews in place rather than duplicating.
  const extend = useCreateGrant(grant.user_id);
  const remaining = daysUntil(grant.expires_at);

  return (
    <CardRow>
      <UserAvatar id={grant.user_id} size="list" />
      <div className="w-[170px] shrink-0 truncate text-[15px] font-semibold">
        <UserName id={grant.user_id} />
      </div>
      <div className="w-[250px] shrink-0 truncate text-[14.5px] text-ink/80">
        <ProjectName id={grant.project_id} /> / <Mono>{grant.role_key}</Mono>
      </div>
      <div className="flex min-w-0 flex-1 items-center gap-3">
        <AccessSource kind="direct" />
        <span className="truncate text-[14px] text-muted">
          No action — expires {formatShortDate(grant.expires_at)}
        </span>
      </div>
      <div className="w-[66px] shrink-0 text-[13.5px] font-semibold text-warn-text">
        {remaining === null ? "—" : `${remaining} days`}
      </div>
      <Button
        size="sm"
        isPending={extend.isPending}
        onClick={async () => {
          try {
            await extend.mutateAsync({
              project_id: grant.project_id,
              role_key: grant.role_key,
              reason: "Extended from Today",
              duration_days: 90,
            });
            toast.success("Extended by 90 days.");
          } catch (error) {
            toast.error(error instanceof Error ? error.message : "The extension didn't go through.");
          }
        }}
      >
        Extend
      </Button>
    </CardRow>
  );
}

function PendingChanges({ count, reachable }: { count: number; reachable: boolean }) {
  const drain = useDrainPropagations();

  return (
    <Card>
      <CardHeader title="Pending changes" count={count} />
      <CardRow>
        <div className="flex-1 text-[14.5px]">
          {count} Zitadel {count === 1 ? "write" : "writes"} queued
          <span className="text-faint"> — waiting for confirmation</span>
        </div>
        <Button
          variant="outline"
          size="sm"
          disabled={!reachable}
          isPending={drain.isPending}
          onClick={async () => {
            try {
              await drain.mutateAsync();
              toast.success("Queued writes resumed.");
            } catch (error) {
              toast.error(error instanceof Error ? error.message : "The drain didn't start.");
            }
          }}
        >
          Resume now
        </Button>
      </CardRow>
      {!reachable && (
        // A disabled action states its reason in the row, not on hover: hover
        // doesn't exist on touch and doesn't survive a screenshot.
        <div className="border-t border-dashed border-warn-line bg-warn-soft px-[18px] py-3 text-[13.5px] text-warn-text">
          Disabled — identity provider unreachable. Writes stay queued; nothing is lost.
        </div>
      )}
    </Card>
  );
}

function UnexplainedAccess({ count }: { count: number }) {
  return (
    <Card>
      <CardHeader title="Unexplained access" count={count} tone="danger" />
      <CardRow>
        <div className="flex-1 text-[14.5px]">
          {count} {count === 1 ? "grant" : "grants"} MkAuth can&rsquo;t explain
        </div>
        <ButtonLink href="/governance/drift" size="sm">
          Open triage
        </ButtonLink>
      </CardRow>
    </Card>
  );
}

function greeting(): string {
  const hour = new Date().getHours();
  if (hour < 12) return "Morning";
  if (hour < 17) return "Afternoon";
  return "Evening";
}

/**
 * The greeting addresses a person, so it must never address an id. When the
 * session has no name at all the greeting simply drops the address rather than
 * greeting a Zitadel subject — "Morning." is fine; "Morning, 318...447." is not.
 */
function firstName(session: SessionUser): string {
  const source = session.name.trim() || (session.email.split("@")[0] ?? "");
  return source.trim().split(/\s+/)[0] ?? "";
}

function workSentence(count: number, loading: boolean): string {
  if (loading) return "Checking.";
  if (count === 0) return "Nothing needs you.";
  if (count === 1) return "One thing needs you.";
  return `${spell(count)} things need you.`;
}

const WORDS = ["Zero", "One", "Two", "Three", "Four", "Five", "Six", "Seven", "Eight", "Nine", "Ten"];

function spell(count: number): string {
  return WORDS[count] ?? String(count);
}
