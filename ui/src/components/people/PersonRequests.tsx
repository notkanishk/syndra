"use client";

import Link from "next/link";
import { useMemo, useState } from "react";

import { RoleRef } from "@/components/names";
import { EmptyState, ListStates, RowSkeleton } from "@/components/states";
import { ActionOutcome } from "@/components/ui/ActionOutcome";
import { Button } from "@/components/ui/Button";
import { Card, CardColumns } from "@/components/ui/Card";
import { Relative } from "@/components/ui/Time";
import { describeDuration } from "@/lib/format";
// Aliased: this file already has an `Outcome`, which is the decision a request
// received. This one is what the write did.
import { outcomeFromError, type ActionOutcome as ActionResult } from "@/lib/outcome";
import {
  useDecideRequest,
  useRequestsAdmin,
  type AccessRequest,
} from "@/lib/queries/useRequests";

/**
 * A person's requests, on their record.
 *
 * This used to be a signpost pointing at /requests, which is the queue of
 * everybody's *pending* work — so the one question this tab is opened to answer
 * ("what has this person asked for, and what did we say?") was the one question
 * the destination couldn't answer. Decided requests aren't in the queue at all.
 *
 * So it shows the full history, decisions included, and the operator can act on
 * anything still pending without leaving the person they were looking at.
 */
export function PersonRequests({
  userId,
  name,
  isOperator,
}: {
  userId: string;
  name: string;
  isOperator: boolean;
}) {
  // The backend has no per-requester filter, but it does have "all", and the
  // request table is small enough that narrowing here is honest — unlike the
  // audit tail, nothing is truncated on the way.
  const requests = useRequestsAdmin("all");
  const decide = useDecideRequest();
  const [resolved, setResolved] = useState<Set<string>>(new Set());
  const [outcomes, setOutcomes] = useState<Record<string, ActionResult | null>>({});

  const mine = useMemo(
    () =>
      (requests.data ?? [])
        .filter((entry) => entry.requester_id === userId)
        .sort((a, b) => b.created_at.localeCompare(a.created_at)),
    [requests.data, userId],
  );

  const pending = mine.filter((entry) => entry.status === "pending" && !resolved.has(entry.id));

  async function act(id: string, status: "approved" | "rejected") {
    setResolved((prev) => new Set(prev).add(id));
    setOutcomes((prev) => ({ ...prev, [id]: null }));
    try {
      await decide.mutateAsync({ id, status });
      // Reported on the row that ran it, and the row keeps its seat: it leaves
      // on the next read rather than under the thumb that decided it.
      setOutcomes((prev) => ({
        ...prev,
        [id]: {
          kind: "applied",
          message: status === "approved" ? `Approved for ${name}` : `Denied for ${name}`,
        },
      }));
    } catch (error) {
      // Put it back: a row that vanished on a failed write would read as a
      // decision that was recorded.
      setResolved((prev) => {
        const next = new Set(prev);
        next.delete(id);
        return next;
      });
      setOutcomes((prev) => ({ ...prev, [id]: outcomeFromError(error) }));
    }
  }

  return (
    <Card>
      <CardColumns>
        <span className="w-[250px]">What they asked for</span>
        <span className="flex-1">Why</span>
        <span className="w-[110px]">When</span>
        <span className="w-[190px] text-right">Outcome</span>
      </CardColumns>

      <ListStates
        isLoading={requests.isLoading}
        error={requests.error}
        isEmpty={mine.length === 0}
        onRetry={() => requests.refetch()}
        errorTitle="Couldn't load this person's requests."
        skeleton={<RowSkeleton rows={3} avatar={false} label="Loading requests" />}
        empty={
          <EmptyState
            title={`${name} hasn’t asked for anything.`}
            guidance="Requests appear here when this person asks for access to a project role, along with what was decided."
          />
        }
      >
        {mine.map((entry) => (
          <RequestRow
            key={entry.id}
            entry={entry}
            isOperator={isOperator}
            optimisticallyResolved={resolved.has(entry.id)}
            isPending={decide.isPending}
            onDecide={act}
            outcome={outcomes[entry.id] ?? null}
          />
        ))}
      </ListStates>

      {isOperator && pending.length > 0 && (
        <div className="border-t border-line px-5 py-3 text-[13px] text-faint">
          {pending.length} still open — deciding here is the same decision as on{" "}
          <Link href="/requests" className="font-semibold text-accent-text">
            Requests
          </Link>
          .
        </div>
      )}
    </Card>
  );
}

function RequestRow({
  entry,
  isOperator,
  optimisticallyResolved,
  isPending,
  onDecide,
  outcome,
}: {
  entry: AccessRequest;
  isOperator: boolean;
  optimisticallyResolved: boolean;
  isPending: boolean;
  onDecide: (id: string, status: "approved" | "rejected") => void;
  outcome: ActionResult | null;
}) {
  const open = entry.status === "pending" && !optimisticallyResolved;

  return (
    <div className="row-divider flex min-h-[60px] flex-col items-start gap-2 px-5 py-3.5 tablet:flex-row tablet:flex-wrap tablet:items-center tablet:gap-4">
      <span className="w-full text-[14.5px] tablet:w-[250px] tablet:shrink-0 tablet:truncate">
        <RoleRef projectId={entry.project_id} roleKey={entry.role_key} />
        <span className="block text-[12.5px] text-faint">
          {describeDuration(entry.duration_days)}
        </span>
      </span>
      <span className="w-full text-[14px] text-muted tablet:min-w-[200px] tablet:flex-1 tablet:truncate">
        {entry.justification ? `“${entry.justification}”` : "No reason given"}
      </span>
      <span className="text-[13px] text-faint tablet:w-[110px] tablet:shrink-0">
        <Relative iso={entry.created_at} />
      </span>
      <span className="flex w-full items-center gap-3 tablet:w-[190px] tablet:shrink-0 tablet:justify-end tablet:gap-2">
        {open && isOperator ? (
          <>
            <Button
              variant="accent"
              size="sm"
              isPending={isPending}
              onClick={() => onDecide(entry.id, "approved")}
            >
              Approve
            </Button>
            <Button size="sm" isPending={isPending} onClick={() => onDecide(entry.id, "rejected")}>
              Deny
            </Button>
          </>
        ) : (
          <Outcome status={optimisticallyResolved ? "approved" : entry.status} note={entry.review_note} />
        )}
      </span>

      {outcome && <ActionOutcome outcome={outcome} placement="inline" className="w-full" />}
    </div>
  );
}

/**
 * The decision, in the colour it belongs to. A denied request is not an error
 * state — somebody made a correct call — so it reads muted rather than red.
 */
function Outcome({ status, note }: { status: string; note?: string }) {
  const style =
    status === "approved"
      ? "text-accent-text"
      : status === "rejected"
        ? "text-muted"
        : "text-warn-text";
  const label =
    status === "approved" ? "Approved" : status === "rejected" ? "Declined" : "Waiting on a decision";

  return (
    <span className="text-right">
      <span className={`text-[13.5px] font-semibold ${style}`}>{label}</span>
      {note ? <span className="block text-[12.5px] text-faint">{note}</span> : null}
    </span>
  );
}
