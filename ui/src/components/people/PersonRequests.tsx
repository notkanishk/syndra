"use client";

import Link from "next/link";
import { useMemo } from "react";

import { RoleRef } from "@/components/names";
import { EmptyState, ListStates, RowSkeleton } from "@/components/states";
import { StatusSeat, keepResult, requestOutcome } from "@/components/requests/RequestsScreen";
import { ActionOutcome } from "@/components/ui/ActionOutcome";
import { Button } from "@/components/ui/Button";
import { Card, CardColumns } from "@/components/ui/Card";
import { Relative } from "@/components/ui/Time";
import { describeAsk } from "@/components/requests/Holding";
import { useRequestDecisions, type Decided } from "@/lib/request-decisions";
import { useTerms } from "@/lib/queries/useTerms";
import { useRequestsAdmin, type AccessRequest } from "@/lib/queries/useRequests";

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
  // The queue's own decide: an approval here says what it says there.
  const { act, resolved, outcomes, isPending, cancelStep } = useRequestDecisions();

  const mine = useMemo(
    () =>
      (requests.data ?? [])
        .filter((entry) => entry.requester_id === userId)
        .sort((a, b) => b.created_at.localeCompare(a.created_at)),
    [requests.data, userId],
  );

  const pending = mine.filter((entry) => entry.status === "pending" && !resolved.has(entry.id));

  return (
    <Card>
      {cancelStep}
      <CardColumns>
        <span className="w-[250px]">What they asked for</span>
        <span className="flex-1">Why</span>
        <span className="w-[110px]">When</span>
        <span className="w-[196px]">Outcome</span>
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
            isPending={isPending}
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
  onDecide: (entry: AccessRequest, status: "approved" | "rejected") => void;
  outcome: Decided | null;
}) {
  const open = entry.status === "pending" && !optimisticallyResolved;
  const terms = useTerms();

  return (
    <div className="row-divider flex min-h-[60px] flex-col items-start gap-2 px-5 py-3.5 tablet:flex-row tablet:flex-wrap tablet:items-center tablet:gap-4">
      <span className="w-full text-[14.5px] tablet:w-[250px] tablet:shrink-0 tablet:truncate">
        <RoleRef projectId={entry.project_id} roleKey={entry.role_key} />
        <span className="block text-[12.5px] text-faint">
          {describeAsk(entry, terms.data)}
        </span>
      </span>
      <span className="w-full text-[14px] text-muted [text-wrap:pretty] tablet:min-w-[200px] tablet:flex-1">
        {entry.justification ? `“${entry.justification}”` : "No reason given"}
      </span>
      <span className="text-[13px] text-faint tablet:w-[110px] tablet:shrink-0">
        <Relative iso={entry.created_at} />
      </span>
      {/* One seat: the buttons, then the decision in their place (#96). */}
      <span className="flex w-full flex-col gap-2 tablet:w-[196px] tablet:shrink-0">
        {open && isOperator ? (
          <span className="flex items-center gap-3 tablet:gap-2">
            <Button
              variant="accent"
              size="sm"
              isPending={isPending}
              onClick={() => onDecide(entry, "approved")}
            >
              Approve
            </Button>
            <Button size="sm" isPending={isPending} onClick={() => onDecide(entry, "rejected")}>
              Decline
            </Button>
          </span>
        ) : outcome && optimisticallyResolved ? (
          <ActionOutcome placement="seat" word={outcome.word} outcome={outcome.outcome} />
        ) : optimisticallyResolved ? null : (
          <Outcome status={entry.status} note={[keepResult(entry), entry.review_note].filter(Boolean).join(" · ")} />
        )}
        {/* A failed write puts the buttons back and says why under them. */}
        {outcome && !optimisticallyResolved && (
          <ActionOutcome placement="seat" outcome={outcome.outcome} />
        )}
      </span>
    </div>
  );
}

/**
 * The decision, in the colour it belongs to. A denied request is not an error
 * state — somebody made a correct call — so it reads muted rather than red.
 */
function Outcome({ status, note }: { status: string; note?: string }) {
  // The queue's own words, so a withdrawn or fulfilled request never reads as
  // "Waiting on a decision" here, as a withdrawn one did.
  const said = requestOutcome(status);
  const label = status === "pending" ? "Waiting on a decision" : said.operator;
  return <StatusSeat tone={said.tone} word={label} note={note || undefined} />;
}
