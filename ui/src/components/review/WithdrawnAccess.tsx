"use client";

import { useQuery } from "@tanstack/react-query";
import Link from "next/link";

import { EmptyState, ListStates } from "@/components/states";
import { Mono } from "@/components/ui/Badge";
import { Card, CardHeader } from "@/components/ui/Card";
import { PageHeader } from "@/components/ui/PageHeader";
import { Relative } from "@/components/ui/Time";
import { UserName } from "@/components/names";
import { request } from "@/lib/api-client";
import { targetLabel } from "@/lib/nav";

/**
 * Review › Withdrawn access (9.9, 9.10).
 *
 * Beside Unexplained access, never inside it, because they are different
 * questions with different answers. Drift is access that appeared and cannot be
 * explained; this is access somebody decided to take away that is still there.
 *
 * The two populations render differently on purpose:
 *
 *   queued — draining, and the only content of the signal is how long it has
 *            been. Nothing is wrong yet.
 *   spent  — terminal. Nothing will dispatch it again, so waiting produces
 *            nothing and somebody has to act.
 *
 * Merged into one count, a healthy queue of five-minute-old rows hides a
 * revocation that failed permanently three days ago.
 */
interface UnconfirmedRevocation {
  id: string;
  op_type: string;
  user_id: string;
  project_id: string;
  role_keys: string[];
  status: string;
  attempts: number;
  last_error?: string;
  created_at: string;
  target: string;
  age_seconds: number;
  spent: boolean;
  /** The join key back to the change that caused this. */
  cascade_id?: string;
}

interface Payload {
  revocations: UnconfirmedRevocation[];
  summary: { queued: number; spent: number; oldest_age_seconds: number };
  escalated: boolean;
}

export function WithdrawnAccess() {
  const query = useQuery({
    queryKey: ["governance", "unconfirmed-revocations"],
    queryFn: () => request<Payload>("/governance/unconfirmed-revocations"),
    refetchInterval: 60_000,
  });

  const rows = query.data?.revocations ?? [];
  const spent = rows.filter((r) => r.spent);
  const queued = rows.filter((r) => !r.spent);

  return (
    <>
      <PageHeader
        title="Withdrawn access"
        meta="Access somebody decided to take away that has not gone away yet."
      />
      <ListStates
        isLoading={query.isLoading}
        error={query.error}
        isEmpty={rows.length === 0}
        onRetry={() => query.refetch()}
        errorTitle="The withdrawal queue could not be read"
        empty={
          <EmptyState
            title="Nothing outstanding"
            guidance="Every revocation has reached its target."
          />
        }
      >
        {/* Both buckets always, in this order, whatever either one holds.
            They used to render only when non-empty, so a revocation going
            terminal INSERTED a red card above the queue somebody was reading —
            the list moved under them because the data changed, which is the one
            thing the structure is not allowed to do. A bucket at zero is also
            the answer to a real question: "is anything stuck?" reads better as
            a hollow zero than as a card that is not there. */}
        <div className="grid gap-4">
          <Bucket
            title="Not going to happen"
            tone="danger"
            note="Terminal. Nothing will dispatch these again."
            empty="Nothing has given up. Every withdrawal still has a way to reach its target."
            rows={spent}
          />
          <Bucket
            title="Still draining"
            tone="accent"
            note="Queued and being retried. How long they have waited is the signal."
            empty="Nothing is waiting. Every withdrawal has either landed or given up."
            rows={queued}
          />
        </div>
      </ListStates>
    </>
  );
}

/**
 * One of the two buckets, present at any count.
 *
 * The empty sentence is not decoration: on this page the interesting fact is
 * often which bucket is empty, and "no terminal failures" is a different
 * statement from "this section is not on the page today".
 */
function Bucket({
  title,
  tone,
  note,
  empty,
  rows,
}: {
  title: string;
  tone: "accent" | "danger";
  note: string;
  empty: string;
  rows: UnconfirmedRevocation[];
}) {
  return (
    <Card>
      <CardHeader title={title} count={rows.length} tone={tone} note={note} />
      {rows.length === 0 ? (
        <p className="px-5 pb-4 text-[13.5px] text-faint">{empty}</p>
      ) : (
        <ul className="grid gap-3 text-sm">
          {rows.map((row) => (
            <Row key={row.id} row={row} />
          ))}
        </ul>
      )}
    </Card>
  );
}

function Row({ row }: { row: UnconfirmedRevocation }) {
  return (
    <li className="grid gap-1">
      <div className="flex flex-wrap items-baseline gap-2">
        <UserName id={row.user_id} />
        <span className="text-muted">
          on {targetLabel(row.target)}
          {row.role_keys.length > 0 && <> · {row.role_keys.join(", ")}</>}
        </span>
        <span className="text-faint">
          decided <Relative iso={row.created_at} />
        </span>
        <span className="flex-1" />
        {row.cascade_id && (
          // The join key, as a link, exactly as it appears in the audit log and
          // on a person's activity. A finding an operator cannot trace back to
          // the change that caused it is a mystery.
          <Link
            href={`/operations/cascades?cascade=${encodeURIComponent(row.cascade_id)}`}
            className="text-[13px] font-semibold text-accent-text"
          >
            <Mono>{row.cascade_id.slice(0, 8)}</Mono>
          </Link>
        )}
      </div>
      {row.last_error && (
        // The reason, not a status. A terminal row an operator can see and not
        // act on is the whole difference between a finding and a mystery.
        <p className="text-muted">{row.last_error}</p>
      )}
    </li>
  );
}
