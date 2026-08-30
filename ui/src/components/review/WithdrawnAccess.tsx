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
import { shortId } from "@/lib/audit-vocabulary";
import { targetLabel } from "@/lib/nav";

/**
 * Review › Unfinished revocations (9.9, 9.10).
 *
 * Beside Unexplained access, never inside it, because they are different
 * questions with different answers. Unexplained access appeared and cannot be
 * explained; this is access somebody revoked that is still there.
 *
 * The two populations render differently on purpose:
 *
 *   queued — still being sent, and the only content of the signal is how long
 *            it has been. Nothing is wrong yet.
 *   spent  — given up. Nothing will send it again, so waiting produces nothing
 *            and somebody has to act.
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

interface Listing {
  revocations: UnconfirmedRevocation[];
  summary: { queued: number; spent: number; oldest_age_seconds: number };
  escalated: boolean;
}

export function WithdrawnAccess() {
  const query = useQuery({
    queryKey: ["governance", "unconfirmed-revocations"],
    queryFn: () => request<Listing>("/governance/unconfirmed-revocations"),
    refetchInterval: 60_000,
  });

  const rows = query.data?.revocations ?? [];
  const spent = rows.filter((r) => r.spent);
  const queued = rows.filter((r) => !r.spent);

  return (
    <>
      <PageHeader
        title="Unfinished revocations"
        lede="Access somebody revoked that has not gone away yet. Revocations still on the way clear themselves; the ones Syndra has given up on do not, and those people still have the access."
      />
      <ListStates
        isLoading={query.isLoading}
        error={query.error}
        isEmpty={rows.length === 0}
        onRetry={() => query.refetch()}
        errorTitle="Couldn't load unfinished revocations."
        empty={
          <EmptyState
            title="Nothing outstanding"
            guidance="Every revocation has reached the system it was sent to."
          />
        }
      >
        {/* Both buckets always, in this order, whatever either one holds.
            They used to render only when non-empty, so a revocation being
            given up on INSERTED a red card above the queue somebody was
            reading — the list moved under them because the data changed,
            which is the one thing the structure is not allowed to do. A
            bucket at zero is also the answer to a real question: "is anything
            stuck?" reads better as a hollow zero than as a card that is not
            there. */}
        <div className="grid gap-4">
          <Bucket
            title="Given up"
            tone="danger"
            note="Syndra tried its limit and stopped. This person still has the access: revoke it again from their page, or end it in the connected system by hand."
            empty="Nothing has been given up on. Every revocation still has a way through."
            rows={spent}
          />
          <Bucket
            title="Still on the way"
            tone="accent"
            note="Waiting to be sent, and Syndra tries again by itself. The only thing to watch is how long one has waited."
            empty="Nothing is waiting. Every revocation has either gone through or been given up on."
            rows={queued}
          />
          <p className="text-[13px] text-faint">
            A c_ handle links to the edit that set the revocation off, in Change history.
          </p>
        </div>
      </ListStates>
    </>
  );
}

/**
 * One of the two buckets, present at any count.
 *
 * The empty sentence is not decoration: on this page the interesting fact is
 * often which bucket is empty, and "nothing given up on" is a different
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
          revoked <Relative iso={row.created_at} />
        </span>
        <span className="flex-1" />
        {row.cascade_id && (
          // The join key, as a link, exactly as it appears in the audit log and
          // on a person's activity. A finding an operator cannot trace back to
          // the change that caused it is a mystery.
          <Link
            href={`/operations/cascades?cascade=${encodeURIComponent(row.cascade_id)}`}
            aria-label="See the edit that set off this revocation, in Change history"
            className="text-[13px] font-semibold text-accent-text"
          >
            <Mono>{shortId(row.cascade_id, "c")}</Mono>
          </Link>
        )}
      </div>
      {row.last_error && (
        // The reason, not a status. A given-up row an operator can see and not
        // act on is the whole difference between a finding and a mystery.
        <p className="text-muted">Last error: {row.last_error}</p>
      )}
    </li>
  );
}
