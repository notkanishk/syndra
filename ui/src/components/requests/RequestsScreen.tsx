"use client";

import { useRouter, useSearchParams } from "next/navigation";
import { useMemo, useState } from "react";

import { EmptyState, ListStates, RowSkeleton } from "@/components/states";
import { Avatar } from "@/components/ui/Avatar";
import { STATUS_TONE, StatusDot, type StatusTone } from "@/components/ui/Badge";
import { ActionOutcome } from "@/components/ui/ActionOutcome";
import { Button } from "@/components/ui/Button";
import { Card, CardColumns, CardHeader } from "@/components/ui/Card";
import { RehearsalDialog } from "@/components/ui/RehearsalDialog";
import {
  RowCheckbox,
  SelectAllRow,
  SelectModeToggle,
  SelectionAction,
  SelectionBar,
} from "@/components/ui/SelectionBar";
import { BULK_MAX_USERS } from "@/lib/queries/useBulkGrants";
import { useRowSelection } from "@/lib/useRowSelection";
import { FilterPills } from "@/components/ui/Select";
import { PageHeader } from "@/components/ui/PageHeader";
import { PaginationRow, usePaging } from "@/components/ui/PaginationRow";
import { FilterBar } from "@/components/ui/FilterBar";
import { ProjectName, RoleRef, UserName } from "@/components/names";
import { useApplyBulkDecision, useRehearseBulkDecision } from "@/lib/queries/useRequests";
import { useNameResolver } from "@/lib/queries/useNameResolver";
import { useTerms } from "@/lib/queries/useTerms";
import { useUserGrants } from "@/lib/queries/useUsers";
import { useGlobalRoleCatalog } from "@/lib/queries/useRoles";
import { WaitsMark } from "@/components/ui/WaitsMark";
import { useRequestDecisions } from "@/lib/request-decisions";
import {
  useRequestsAdmin,
  useRequestsMine,
  useWithdrawRequest,
  type AccessRequest,
} from "@/lib/queries/useRequests";
import { Relative } from "@/components/ui/Time";
import { formatShortDate, humanizeKey } from "@/lib/format";
import { AskForAccess } from "@/components/requests/AskForAccess";
import { HoldingNote, alreadyHeld, describeAsk, describeAskForMember } from "@/components/requests/Holding";
import { outcomeFromError, type ActionOutcome as ActionResult } from "@/lib/outcome";

type StatusFilter = "pending" | "approved" | "rejected" | "withdrawn" | "all";

/**
 * One reading of a request's status, in both registers.
 *
 * It exists because the two views were free to disagree, and did: a settled row that was not
 * `approved` used to render as "Denied", so the first withdrawal would have shown the member's
 * own retraction back to the operator as a refusal somebody made. An unrecognised status returns
 * itself rather than falling into either bucket — a status this file has not been taught about
 * must never read as a decision.
 */
export function requestOutcome(status: string, kind?: string): {
  /** The member's dot. */
  tone: StatusTone;
  /** The operator's dot, where it differs: a decision is lime, being overtaken is not one. */
  operatorTone: StatusTone;
  operator: string;
  member: string;
} {
  const o = requestOutcomeWords(status);
  // A request to keep a role that was approved says what it did: kept, extended.
  if (status === "approved" && kind === "extend") {
    return { ...o, operator: "Extended", member: "Kept", operatorTone: o.tone };
  }
  return { ...o, operatorTone: status === "fulfilled" ? "neutral" : o.tone };
}

function requestOutcomeWords(status: string): {
  tone: StatusTone;
  operator: string;
  member: string;
} {
  switch (status) {
    case "approved":
      return { tone: "healthy", operator: "Approved", member: "Approved" };
    case "rejected":
      return { tone: "neutral", operator: "Declined", member: "Not approved" };
    case "withdrawn":
      return { tone: "neutral", operator: "Withdrawn", member: "You withdrew this" };
    // Given another way while it waited (directly, or by a bundle), so closed
    // by Syndra: a resolution, not a decision. The note says how.
    case "fulfilled":
      return { tone: "healthy", operator: "Given another way", member: "You have this" };
    case "pending":
      return { tone: "neutral", operator: "Open", member: "Waiting" };
    default:
      return { tone: "warn", operator: status, member: status };
  }
}

/**
 * A settled request's status seat (route-requests): dot, word, and at most one
 * faint note under it. The same seat a decision just made reports in, so a row
 * never changes shape between the tap and the next read.
 */
/**
 * Why a request closed with nothing decided, in the member's words (A-04
 * §13.1). Without it a closed request looks like a lost one.
 */
export function overtakenNote(request: AccessRequest): string {
  const on = request.resolved_at ? ` on ${formatShortDate(request.resolved_at)}` : "";
  return `Makerspace staff gave it to you directly${on}, so there was nothing left to decide.`;
}

export function StatusSeat({ tone, word, note }: { tone: StatusTone; word: string; note?: string }) {
  return (
    <span className="flex flex-col gap-1">
      <span
        className={`inline-flex items-center gap-2 whitespace-nowrap text-[13.5px] font-semibold ${STATUS_TONE[tone].label}`}
      >
        <StatusDot tone={tone} />
        {word}
      </span>
      {note && <span className="pl-[14px] text-[12.5px] leading-[1.45] text-faint">{note}</span>}
    </span>
  );
}

export function RequestsScreen({ isOperator, userId }: { isOperator: boolean; userId: string }) {
  return isOperator ? <OperatorQueue userId={userId} /> : <MemberRequests userId={userId} />;
}

/** The operator queue — same row shape as the Home block, terminal actions. */
function OperatorQueue({ userId }: { userId?: string }) {
  const [status, setStatus] = useState<StatusFilter>("pending");
  const requests = useRequestsAdmin(status);
  const { act, resolved, outcomes, cancelStep } = useRequestDecisions();
  const catalog = useGlobalRoleCatalog();
  const [bulkStatus, setBulkStatus] = useState<"approved" | "rejected" | null>(null);
  const ask = useAskOnArrival();
  const terms = useTerms();

  // Decided rows stay in `rows` — the row reports its own outcome below and
  // only actually leaves once the refetch lands with its new status. Filtering
  // them out here the instant `act()` fires used to remove the row before its
  // own "Approved"/"Declined" line could ever be seen, which contradicted the
  // promise a few lines down that the row "keeps its seat" and "leaves on the
  // next read, not under the thumb that decided it".
  const rows = useMemo(() => requests.data ?? [], [requests.data]);

  // Only open, not-yet-decided-this-session requests can be decided, so only
  // they can be selected — offering a checkbox on a settled row would be
  // offering an action that cannot happen, and `resolved` catches the window
  // before a decided row's cached status has caught up.
  const openRows = useMemo(
    () => rows.filter((entry) => entry.status === "pending" && !resolved.has(entry.id)),
    [rows, resolved],
  );

  // The queue must have a visible end, the way the drift queue and the people
  // list already do. This screen rendered every row it was given: a long queue
  // paid its whole cost on arrival, and there was no point at which a reader
  // could tell how much more there was.
  //
  // The cap is on RENDERING and on nothing else. Selection stays scoped to
  // every open row above, so "select all" keeps meaning the whole pending
  // queue rather than quietly meaning the page — which would change what the
  // bar's ceiling message is counting without saying so.
  const paging = usePaging();
  const visible = useMemo(() => rows.slice(0, paging.limit), [rows, paging.limit]);
  const selection = useRowSelection(useMemo(() => openRows.map((entry) => entry.id), [openRows]));
  // Deciding one request at a time is what this queue is for; the bulk verbs
  // are the exception, so they arrive behind a named control rather than a
  // column of checkboxes that is always in front of every row.
  const [selecting, setSelecting] = useState(false);

  return (
    <div className="flex flex-col gap-[18px]">
      <PageHeader
        title="Requests"
        lede="What members have asked for. Approve gives them the access for the time they asked, starting now; decline closes it, and they may ask again."
      />
      {cancelStep}

      {/* One control, so it stays on the row rather than behind a panel: a
          disclosure hiding a single five-way switch costs a click and hides
          nothing. Selection mode is an action on the list, so it trails. */}
      <FilterBar
        leading={
          <FilterPills<StatusFilter>
            label="Filter by status"
            value={status}
            onChange={setStatus}
            options={[
              { value: "pending", label: "Open" },
              { value: "approved", label: "Approved" },
              { value: "rejected", label: "Declined" },
              { value: "withdrawn", label: "Withdrawn" },
              { value: "all", label: "All" },
            ]}
          />
        }
        trailing={
          openRows.length > 0 ? (
            <SelectModeToggle active={selecting} onToggle={() => setSelecting((on) => !on)} />
          ) : null
        }
      />

      <Card>
        {openRows.length > 0 && (
          <CardColumns>
            {selecting && <span className="w-11 shrink-0 desktop:w-[26px]" />}
            <span className="w-[170px]">Who</span>
            <span className="w-[250px]">What they asked for</span>
            <span className="flex-1">Why</span>
            <span className="w-[66px]">When</span>
            <span className="w-[196px]">Decision</span>
          </CardColumns>
        )}
        {/* Only open requests can be selected; a decided one is not part of the
            working set, so the first number here is the open count and the
            second is everything the current filter shows. */}
        {selecting && openRows.length > 0 && (
          <SelectAllRow
            inScope={openRows.length}
            total={rows.length}
            noun={["request", "requests"]}
            allSelected={selection.allSelected}
            {...selection.headerCheckboxProps}
          />
        )}

        <div data-selection-scope {...selection.containerProps}>
        <ListStates
          isLoading={requests.isLoading}
          error={requests.error}
          isEmpty={rows.length === 0}
          onRetry={() => requests.refetch()}
          errorTitle="Couldn't load requests."
          skeleton={<RowSkeleton rows={4} label="Loading requests" />}
          empty={
            <EmptyState
              title={status === "pending" ? "No open requests." : `No ${status} requests match those filters.`}
              guidance={
                status === "pending"
                  ? "New requests appear here the moment someone submits one."
                  : "Try another filter."
              }
              // An empty pending queue is resolved work; an empty filter result
              // is just a filter that matched nothing.
              resolved={status === "pending"}
            />
          }
        >
          {visible.map((entry) => (
            <div
              key={entry.id}
              className={`row-divider flex min-h-[60px] flex-col items-start gap-2 px-5 py-3.5 tablet:flex-row tablet:items-center tablet:gap-[18px] ${
                selection.isSelected(entry.id) ? "bg-accent-soft" : ""
              }`}
              {...selection.rowProps(entry.id)}
            >
              {selecting && openRows.length > 0 && (
                <span className="w-11 shrink-0 desktop:w-[26px]">
                  {entry.status === "pending" ? (
                    <RowCheckbox
                      label="Select this request"
                      {...selection.checkboxProps(entry.id)}
                    />
                  ) : null}
                </span>
              )}
              <Avatar name={undefined} />
              <div className="w-full truncate text-[15px] font-semibold tablet:w-[170px] tablet:shrink-0">
                <UserName id={entry.requester_id} />
              </div>
              <div className="w-full text-[14.5px] text-ink/80 tablet:w-[250px] tablet:shrink-0 tablet:truncate">
                {entry.kind === "extend" && "Keep "}
                <RoleRef projectId={entry.project_id} roleKey={entry.role_key} />
                {catalog.data?.some(
                  (r) =>
                    r.project_id === entry.project_id &&
                    r.role_key === entry.role_key &&
                    r.waits_for_send,
                ) && <WaitsMark className="ml-2" />}
                {/* Half of the ask. A decision made without it is a decision
                    about a different request. */}
                <span className="block text-[12.5px] text-faint">
                  {describeAsk(entry, terms.data)}
                </span>
              </div>
              <div className="w-full min-w-0 text-[14px] text-muted tablet:flex-1 tablet:truncate">
                {entry.justification ? `“${entry.justification}”` : "No reason given"}
              </div>
              <div className="text-[13px] text-faint tablet:w-[66px] tablet:shrink-0">
                <Relative iso={entry.created_at} />
              </div>
              {/* The Decision seat (#96): the buttons, then the decision in
                  their place — one seat, so no column moves on the tap. The
                  row keeps it until the next read; no toast. */}
              <div className="w-full tablet:w-[196px] tablet:shrink-0">
                {entry.status === "pending" && !resolved.has(entry.id) ? (
                  <div className="flex flex-col gap-2">
                    <div className="flex w-full flex-row-reverse gap-3 tablet:flex-row tablet:gap-2">
                      <Button
                        variant="accent"
                        size="sm"
                        className="flex-1 tablet:flex-none"
                        disabled={alreadyHeld(entry)}
                        reason={alreadyHeld(entry) ? "They already have this." : undefined}
                        onClick={() => act(entry, "approved")}
                      >
                        Approve
                      </Button>
                      <Button size="sm" onClick={() => act(entry, "rejected")}>
                        Decline
                      </Button>
                    </div>
                    {/* A failed write puts the buttons back and says why under them. */}
                    {outcomes[entry.id] && (
                      <ActionOutcome placement="seat" outcome={outcomes[entry.id].outcome} />
                    )}
                  </div>
                ) : outcomes[entry.id] ? (
                  <ActionOutcome
                    placement="seat"
                    word={outcomes[entry.id].word}
                    outcome={outcomes[entry.id].outcome}
                  />
                ) : resolved.has(entry.id) ? null : (
                  <StatusSeat
                    tone={requestOutcome(entry.status, entry.kind).operatorTone}
                    word={requestOutcome(entry.status, entry.kind).operator}
                    note={
                      entry.status === "approved" &&
                      entry.reviewer_id &&
                      entry.reviewer_id === entry.requester_id
                        ? "by the person who asked"
                        : undefined
                    }
                  />
                )}
              </div>
              {/* Allowed, and said before it happens (owner, 15 Oct): an
                  operator can approve their own request, and it is recorded
                  as exactly that. */}
              {entry.status === "pending" && !resolved.has(entry.id) && <HoldingNote request={entry} />}
              {entry.status === "pending" && !resolved.has(entry.id) && userId && entry.requester_id === userId && (
                <span className="w-full text-[13px] text-faint">
                  Your own request. Approving it is allowed, and Change history records that you approved your own.
                </span>
              )}
            </div>
          ))}

          <PaginationRow
            shown={visible.length}
            total={rows.length}
            onMore={paging.more}
            onAll={() => paging.all(rows.length)}
            onBack={paging.back}
          />
        </ListStates>
        </div>
      </Card>

      {/* What this operator has asked for themselves, once they have: the
          member's card, in the member's place (A-04 §2.1). */}
      {userId && <YourRequests userId={userId} hideWhenEmpty onAsk={() => ask.setOpen(true)} />}

      <SelectionBar
        count={selecting ? selection.count : 0}
        noun={["request", "requests"]}
        // One server constant — `services.BulkMaxUsers` — caps the grant, the
        // request and the drift bulk endpoints alike, which is why a
        // users-shaped name is the right import here. Without it the operator
        // select-alls, taps, and meets the 4xx afterwards: the bar exists to
        // say the number before the tap, not to let the server say it after.
        ceiling={BULK_MAX_USERS}
        onTakeCeiling={() =>
          selection.selectOnly(openRows.slice(0, BULK_MAX_USERS).map((entry) => entry.id))
        }
        onClear={selection.clear}
      >
        {/* Both open a decision dialog rather than deciding. */}
        <SelectionAction onClick={() => setBulkStatus("approved")}>
          Review approving these
        </SelectionAction>
        {/* Not `danger`: declining a pending request takes nothing away —
            nobody has the access yet. The per-row Decline above (default
            variant) already gets this right; the bulk twin did not. */}
        <SelectionAction onClick={() => setBulkStatus("rejected")}>
          Review declining these
        </SelectionAction>
      </SelectionBar>

      {bulkStatus && (
        <BulkDecisionDialog
          status={bulkStatus}
          ids={Array.from(selection.selected)}
          onClose={() => setBulkStatus(null)}
          onApplied={selection.clear}
        />
      )}

      {/* An operator's own ask, arriving from My access. It used to land on
          this queue with the dialog shut and the choice lost. */}
      <AskForAccess
        key={`${ask.linkedProject}:${ask.linkedRole}`}
        open={ask.open}
        onClose={ask.close}
        initial={{ projectId: ask.linkedProject, roleKey: ask.linkedRole }}
        userId={userId ?? ""}
        selfDecides
      />
    </div>
  );
}

/**
 * "Ask for this" in the catalogue links here with the ask already chosen
 * (`?project=…&role=…`), and "Ask for access" elsewhere with `?ask`, so the
 * dialog opens on arrival rather than making somebody find the button again.
 * The params are consumed as the dialog closes: a refresh after the request
 * is sent should not reopen the form.
 *
 * For a member and an operator alike. An operator's own access is asked for
 * like anyone's (A-03 §4), from My access; it then waits in the queue, where
 * approving it is said to be their own.
 */
function useAskOnArrival() {
  const router = useRouter();
  const params = useSearchParams();
  const linkedProject = params.get("project") ?? "";
  const linkedRole = params.get("role") ?? "";
  const linked = Boolean(linkedProject) || params.has("ask") || params.has("extend");
  const [open, setOpen] = useState(linked);
  function close() {
    setOpen(false);
    if (linked) router.replace("/requests", { scroll: false });
  }
  return { open, setOpen, close, linkedProject, linkedRole };
}

/** A member's own requests, plus the form to make one. No jargon. */
function MemberRequests({ userId }: { userId: string }) {
  const { open, setOpen, close: closeDialog, linkedProject, linkedRole } = useAskOnArrival();

  return (
    <div className="flex flex-col gap-[18px]">
      <PageHeader
        title="Requests"
        lede="Ask for access, and see what happened to what you asked for. Makerspace staff decide; the answer appears here."
        actions={
          <Button variant="accent" onClick={() => setOpen(true)}>
            Ask for access
          </Button>
        }
      />

      <YourRequests userId={userId} onAsk={() => setOpen(true)} />

      <AskForAccess
        // Remounted per link so the prefill applies to whichever role was
        // clicked, not only the first.
        key={`${linkedProject}:${linkedRole}`}
        open={open}
        onClose={closeDialog}
        initial={{ projectId: linkedProject, roleKey: linkedRole }}
        userId={userId}
      />
    </div>
  );
}

/**
 * The card of what you asked for, and what became of it. A member's page, and
 * an operator's under the queue once they have asked for anything: one row
 * component, so a request reads the same to whoever made it.
 */
function YourRequests({
  userId,
  onAsk,
  hideWhenEmpty = false,
}: {
  userId: string;
  onAsk: () => void;
  hideWhenEmpty?: boolean;
}) {
  const requests = useRequestsMine();
  const withdraw = useWithdrawRequest();
  const resolver = useNameResolver();
  const terms = useTerms();
  const grants = useUserGrants(userId);
  // A member's own rows report their own withdrawals, in the row.
  const [outcomes, setOutcomes] = useState<Record<string, ActionResult | null>>({});
  // Same idiom as OperatorQueue.act(): marked synchronously, before the await,
  // because `withdraw.isPending` clears the moment the request settles and
  // ["requests"] has not refetched yet — a second tap in that window withdrew
  // an already-withdrawn request.
  const [resolved, setResolved] = useState<Set<string>>(new Set());

  async function withdrawOne(entry: AccessRequest) {
    setResolved((prev) => new Set(prev).add(entry.id));
    try {
      await withdraw.mutateAsync(entry.id);
      setOutcomes((prev) => ({
        ...prev,
        // Neutral, not applied: nothing was given or taken (#51).
        [entry.id]: {
          kind: "no_change",
          message: "You withdrew this. Nobody will be asked to decide it.",
          detail: "Nobody will be asked to decide it",
        },
      }));
    } catch (error) {
      setResolved((prev) => {
        const copy = new Set(prev);
        copy.delete(entry.id);
        return copy;
      });
      setOutcomes((prev) => ({ ...prev, [entry.id]: outcomeFromError(error) }));
    }
  }

  const mine = (requests.data ?? []).filter((entry) => entry.requester_id === userId);
  if (hideWhenEmpty && mine.length === 0) return null;

  // What a request to keep a role did, said from where the end is now: the
  // read of a settled request doesn't carry the old or the new end.
  const endNow = (entry: AccessRequest) =>
    (grants.data ?? []).find((g) => g.project_id === entry.project_id && g.role_key === entry.role_key)?.expires_at ?? null;

  return (
    <Card>
      <CardHeader title="Your requests" count={mine.length} />
      <ListStates
        isLoading={requests.isLoading}
        error={requests.error}
        isEmpty={mine.length === 0}
        onRetry={() => requests.refetch()}
        errorTitle="Couldn't load your requests."
        skeleton={<RowSkeleton rows={3} avatar={false} label="Loading your requests" />}
        empty={
          <EmptyState
            title="You haven't asked for anything yet."
            guidance="If there's a machine you need and can't use, ask here and makerspace staff will decide."
            action={{ label: "Ask for access", onClick: onAsk }}
          />
        }
      >
        {mine.map((entry) => (
          <div
            key={entry.id}
            className="row-divider flex min-h-[60px] flex-col items-start gap-2 px-5 py-3.5 tablet:flex-row tablet:items-center tablet:gap-[18px]"
          >
            <div className="w-full min-w-0 tablet:flex-1">
              {/* The role, not only the place: two asks on one project were
                  two identical rows. */}
              <div className="text-[15px] font-semibold [text-wrap:pretty]">
                {entry.kind === "extend" && "Keep "}
                <ProjectName id={entry.project_id} /> ·{" "}
                {resolver.resolveRole(entry.project_id, entry.role_key).value?.display_name ||
                  humanizeKey(entry.role_key)}
              </div>
              <div className="text-[13.5px] text-faint [text-wrap:pretty]">
                {describeAskForMember(entry, terms.data)}
              </div>
              <div className="text-[13.5px] text-faint [text-wrap:pretty]">
                {entry.justification || "No reason given"}
              </div>
            </div>
            <div className="w-[110px] shrink-0 text-[13px] text-faint tablet:text-right">
              <Relative iso={entry.created_at} />
            </div>
            {/* One status seat: dot, word, note. A withdrawal reports here,
                in the member's own word, and keeps it until the next read. */}
            <div className="w-full tablet:w-auto tablet:min-w-[220px] tablet:shrink-0">
              {outcomes[entry.id] && resolved.has(entry.id) ? (
                <ActionOutcome
                  placement="seat"
                  word="You withdrew this"
                  outcome={outcomes[entry.id]!}
                />
              ) : (
                <div className="flex flex-col gap-2">
                  <StatusSeat
                    tone={requestOutcome(entry.status, entry.kind).tone}
                    word={requestOutcome(entry.status, entry.kind).member}
                    note={
                      entry.status === "fulfilled" ? overtakenNote(entry) : keepNote(entry, endNow(entry))
                    }
                  />
                  {outcomes[entry.id] && <ActionOutcome placement="seat" outcome={outcomes[entry.id]!} />}
                </div>
              )}
            </div>
            {/*
              Withdraw sits on the row rather than behind a dialog, outline and last (#109).
              Nothing is destroyed by it and nobody else's access moves — it takes an ask out
              of somebody's queue, which is the one thing here a member is entitled to do
              without being asked twice.
            */}
            <div className="flex shrink-0 tablet:min-w-[96px] tablet:justify-end">
              {entry.status === "pending" && !resolved.has(entry.id) && (
                <Button
                  size="sm"
                  variant="outline"
                  disabled={withdraw.isPending}
                  onClick={() => withdrawOne(entry)}
                >
                  Withdraw
                </Button>
              )}
            </div>
          </div>
        ))}
      </ListStates>
    </Card>
  );
}

/**
 * A request to keep a role, in its status seat (A-06 §3): what is true of the
 * end until it's decided, and after. Only what is known is said: the read of a
 * settled request carries neither the old end nor the new one, so a result
 * names the end the role has now.
 */
export function keepNote(request: AccessRequest, endNow: string | null): string | undefined {
  if (request.kind !== "extend") return undefined;
  if (request.status === "pending") {
    return request.current_until
      ? `Until it's decided, it still ends ${formatShortDate(request.current_until)}.`
      : undefined;
  }
  if (!endNow) return undefined;
  if (request.status === "approved") return `It now ends ${formatShortDate(endNow)}.`;
  // A role that has ended since doesn't "still end" anywhere.
  if (request.status === "rejected" && new Date(endNow).getTime() > Date.now()) {
    return `It still ends ${formatShortDate(endNow)}.`;
  }
  return undefined;
}

/**
 * Deciding a batch of requests, rehearsed.
 *
 * The same dialog People opens for a bulk grant, because approving nine
 * requests IS nine grants — the inbox framing is what makes that easy to
 * forget, so the plan says it per row before anything is written.
 */
function BulkDecisionDialog({
  status,
  ids,
  onClose,
  onApplied,
}: {
  status: "approved" | "rejected";
  ids: string[];
  onClose: () => void;
  onApplied: () => void;
}) {
  const rehearse = useRehearseBulkDecision();
  const apply = useApplyBulkDecision();
  const body = { ids, status };

  return (
    <RehearsalDialog
      title={status === "approved" ? "Approve requests" : "Decline requests"}
      lede=""
      noun={["request", "requests"]}
      destructive={false}
      onRehearse={(acknowledgeScope) =>
        rehearse.mutateAsync({ ...body, acknowledge_scope: acknowledgeScope })
      }
      onApply={async (planId) => {
        const plan = await apply.mutateAsync({ ...body, plan_id: planId });
        onApplied();
        return plan;
      }}
      onClose={onClose}
    />
  );
}

