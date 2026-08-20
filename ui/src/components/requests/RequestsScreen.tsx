"use client";

import { useRouter, useSearchParams } from "next/navigation";
import { useMemo, useState } from "react";

import { EmptyState, ListStates, RowSkeleton } from "@/components/states";
import { Avatar } from "@/components/ui/Avatar";
import { Badge } from "@/components/ui/Badge";
import { ActionOutcome } from "@/components/ui/ActionOutcome";
import { Button } from "@/components/ui/Button";
import { Card, CardColumns, CardHeader } from "@/components/ui/Card";
import { RehearsalDialog } from "@/components/ui/RehearsalDialog";
import {
  RowCheckbox,
  SelectAllCheckbox,
  SelectionAction,
  SelectionBar,
} from "@/components/ui/SelectionBar";
import { useRowSelection } from "@/lib/useRowSelection";
import { FieldHint, FieldLabel, Input } from "@/components/ui/Input";
import { FilterPills, Select } from "@/components/ui/Select";
import { Modal, ModalFooter, ModalHeader } from "@/components/ui/Modal";
import { PageHeader } from "@/components/ui/PageHeader";
import { ProjectName, RoleRef, UserName } from "@/components/names";
import { useApplyBulkDecision, useRehearseBulkDecision } from "@/lib/queries/useRequests";
import { useProjects } from "@/lib/queries/useProjects";
import { useGlobalRoleCatalog } from "@/lib/queries/useRoles";
import {
  useCreateRequest,
  useDecideRequest,
  useRequestsAdmin,
  useRequestsMine,
  useWithdrawRequest,
  type AccessRequest,
} from "@/lib/queries/useRequests";
import { Relative } from "@/components/ui/Time";
import { describeDuration, formatLongDate } from "@/lib/format";
import { outcomeFromError, type ActionOutcome as ActionResult } from "@/lib/outcome";
import { daysUntilTermEnd } from "@/lib/term";

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
export function requestOutcome(status: string): {
  tone: "accent" | "neutral" | "warn";
  operator: string;
  member: string;
} {
  switch (status) {
    case "approved":
      return { tone: "accent", operator: "Approved", member: "Approved" };
    case "rejected":
      return { tone: "neutral", operator: "Denied", member: "Not approved" };
    case "withdrawn":
      return { tone: "neutral", operator: "Withdrawn", member: "You withdrew this" };
    case "pending":
      return { tone: "warn", operator: "Open", member: "Waiting" };
    default:
      return { tone: "warn", operator: status, member: status };
  }
}
/** What a member asks for, in the units a member thinks in. */
type HowLong = "7" | "30" | "term";

export function RequestsScreen({ isOperator, userId }: { isOperator: boolean; userId: string }) {
  return isOperator ? <OperatorQueue /> : <MemberRequests userId={userId} />;
}

/** The operator queue — same row shape as the Home block, terminal actions. */
function OperatorQueue() {
  const [status, setStatus] = useState<StatusFilter>("pending");
  const requests = useRequestsAdmin(status);
  const decide = useDecideRequest();
  const [resolved, setResolved] = useState<Set<string>>(new Set());
  const [outcomes, setOutcomes] = useState<Record<string, ActionResult | null>>({});
  const [bulkStatus, setBulkStatus] = useState<"approved" | "rejected" | null>(null);

  const rows = useMemo(
    () => (requests.data ?? []).filter((entry) => !resolved.has(entry.id)),
    [requests.data, resolved],
  );

  // Only open requests can be decided, so only they can be selected — offering
  // a checkbox on a settled row would be offering an action that cannot happen.
  const openRows = useMemo(() => rows.filter((entry) => entry.status === "pending"), [rows]);
  const selection = useRowSelection(useMemo(() => openRows.map((entry) => entry.id), [openRows]));

  async function act(entry: AccessRequest, next: "approved" | "rejected") {
    setResolved((prev) => new Set(prev).add(entry.id));
    try {
      await decide.mutateAsync({ id: entry.id, status: next });
      setOutcomes((prev) => ({
        ...prev,
        [entry.id]: {
          kind: "applied",
          message: next === "approved" ? "Approved" : "Denied",
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

  return (
    <div className="flex flex-col gap-[18px]">
      <PageHeader
        title="Requests"
        actions={
          <FilterPills<StatusFilter>
            label="Filter by status"
            value={status}
            onChange={setStatus}
            options={[
              { value: "pending", label: "Open" },
              { value: "approved", label: "Approved" },
              { value: "rejected", label: "Denied" },
              { value: "withdrawn", label: "Withdrawn" },
              { value: "all", label: "All" },
            ]}
          />
        }
      />

      <Card>
        {openRows.length > 0 && (
          <CardColumns>
            <span className="w-[26px]">
              <SelectAllCheckbox
                label={
                  selection.allSelected
                    ? "Clear the selection"
                    : `Select all ${openRows.length} open requests`
                }
                {...selection.headerCheckboxProps}
              />
            </span>
            <span className="w-[170px]">Who</span>
            <span className="w-[250px]">What they asked for</span>
            <span className="flex-1">Why</span>
            <span className="w-[66px]">When</span>
            <span className="w-[150px] text-right">Decision</span>
          </CardColumns>
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
              title={status === "pending" ? "No open requests." : "Nothing here."}
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
          {rows.map((entry) => (
            <div
              key={entry.id}
              className={`row-divider flex min-h-[60px] flex-col items-start gap-2 px-5 py-3.5 tablet:flex-row tablet:items-center tablet:gap-[18px] ${
                selection.isSelected(entry.id) ? "bg-accent-soft/30" : ""
              }`}
              {...selection.rowProps(entry.id)}
            >
              {openRows.length > 0 && (
                <span className="w-[26px]">
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
                <RoleRef projectId={entry.project_id} roleKey={entry.role_key} />
                {/* Half of the ask. A decision made without it is a decision
                    about a different request. */}
                <span className="block text-[12.5px] text-faint">
                  {describeDuration(entry.duration_days)}
                </span>
              </div>
              <div className="w-full min-w-0 text-[14px] text-muted tablet:flex-1 tablet:truncate">
                {entry.justification ? `“${entry.justification}”` : "No reason given"}
              </div>
              <div className="text-[13px] text-faint tablet:w-[66px] tablet:shrink-0">
                <Relative iso={entry.created_at} />
              </div>
              {entry.status === "pending" ? (
                <div className="flex w-full flex-row-reverse gap-3 tablet:w-auto tablet:shrink-0 tablet:flex-row tablet:gap-2">
                  <Button
                    variant="accent"
                    size="sm"
                    className="flex-1 tablet:flex-none"
                    onClick={() => act(entry, "approved")}
                  >
                    Approve
                  </Button>
                  <Button size="sm" onClick={() => act(entry, "rejected")}>
                    Deny
                  </Button>
                </div>
              ) : (
                <Badge tone={requestOutcome(entry.status).tone}>
                  {requestOutcome(entry.status).operator}
                </Badge>
              )}

              {/* The row reports its own decision and keeps its seat. It
                  leaves on the next read, not under the thumb that decided
                  it — a row that vanishes on the tap takes the evidence of
                  what just happened with it. */}
              {outcomes[entry.id] && (
                <ActionOutcome outcome={outcomes[entry.id]!} placement="inline" className="w-full" />
              )}
            </div>
          ))}
        </ListStates>
        </div>
      </Card>

      <SelectionBar
        count={selection.count}
        noun={["request", "requests"]}
        onClear={selection.clear}
      >
        <SelectionAction onClick={() => setBulkStatus("approved")}>Approve</SelectionAction>
        <SelectionAction tone="danger" onClick={() => setBulkStatus("rejected")}>
          Deny
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
    </div>
  );
}

/** A member's own requests, plus the form to make one. No jargon. */
function MemberRequests({ userId }: { userId: string }) {
  const requests = useRequestsMine();
  const withdraw = useWithdrawRequest();
  const router = useRouter();
  const params = useSearchParams();

  /**
   * "Ask for this" in the catalogue links here with the ask already chosen, so
   * the dialog opens on arrival rather than making somebody re-find the thing
   * they just clicked. The params are consumed as the dialog closes — a
   * refresh after the request is sent should not reopen the form.
   */
  const linkedProject = params.get("project") ?? "";
  const linkedRole = params.get("role") ?? "";
  const [open, setOpen] = useState(Boolean(linkedProject));
  // A member's own rows report their own withdrawals, in the row.
  const [outcomes, setOutcomes] = useState<Record<string, ActionResult | null>>({});

  function closeDialog() {
    setOpen(false);
    if (linkedProject) router.replace("/requests", { scroll: false });
  }

  const mine = (requests.data ?? []).filter((entry) => entry.requester_id === userId);

  return (
    <div className="flex flex-col gap-[18px]">
      <PageHeader
        title="Requests"
        meta="Ask for access, and see what happened to what you asked for."
        actions={
          <Button variant="accent" onClick={() => setOpen(true)}>
            Ask for access
          </Button>
        }
      />

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
              guidance="If there's a machine you need and can't use, ask here and a lab manager will decide."
              action={{ label: "Ask for access", onClick: () => setOpen(true) }}
            />
          }
        >
          {mine.map((entry) => (
            <div
              key={entry.id}
              className="row-divider flex min-h-[60px] flex-col items-start gap-2 px-5 py-3.5 tablet:flex-row tablet:items-center tablet:gap-[18px]"
            >
              <div className="min-w-0 flex-1">
                <div className="truncate text-[15px] font-semibold">
                  <ProjectName id={entry.project_id} />
                </div>
                <div className="truncate text-[13.5px] text-faint">
                  {describeDuration(entry.duration_days)} ·{" "}
                  {entry.justification || "No reason given"}
                </div>
              </div>
              <div className="w-[110px] shrink-0 text-[13px] text-faint">
                <Relative iso={entry.created_at} />
              </div>
              {/*
                Withdraw sits on the row rather than behind a dialog. Nothing is destroyed by it
                and nobody else's access moves — it takes an ask out of somebody's queue, which
                is the one thing here a member is entitled to do without being asked twice.
              */}
              {entry.status === "pending" && (
                <Button
                  size="sm"
                  variant="ghost"
                  disabled={withdraw.isPending}
                  onClick={async () => {
                    try {
                      await withdraw.mutateAsync(entry.id);
                      setOutcomes((prev) => ({
                        ...prev,
                        [entry.id]: {
                          kind: "applied",
                          message: "You withdrew this",
                          detail: "Nobody will be asked to decide it.",
                        },
                      }));
                    } catch (error) {
                      setOutcomes((prev) => ({ ...prev, [entry.id]: outcomeFromError(error) }));
                    }
                  }}
                >
                  Withdraw
                </Button>
              )}
              <Badge tone={requestOutcome(entry.status).tone}>
                {requestOutcome(entry.status).member}
              </Badge>

              {outcomes[entry.id] && (
                <ActionOutcome outcome={outcomes[entry.id]!} placement="inline" className="w-full" />
              )}
            </div>
          ))}
        </ListStates>
      </Card>

      <RequestDialog
        // Remounted per link so the prefill applies to whichever role was
        // clicked, not only the first.
        key={`${linkedProject}:${linkedRole}`}
        open={open}
        onClose={closeDialog}
        initial={{ projectId: linkedProject, roleKey: linkedRole }}
      />
    </div>
  );
}

function RequestDialog({
  open,
  onClose,
  initial,
}: {
  open: boolean;
  onClose: () => void;
  /** Prefill from a "Ask for this" link in the catalogue. */
  initial?: { projectId?: string; roleKey?: string };
}) {
  const projects = useProjects();
  const roles = useGlobalRoleCatalog();
  const create = useCreateRequest();
  const [askOutcome, setAskOutcome] = useState<ActionResult | null>(null);

  const [projectId, setProjectId] = useState(initial?.projectId ?? "");
  const [roleKey, setRoleKey] = useState(initial?.roleKey ?? "");
  const [why, setWhy] = useState("");
  const [howLong, setHowLong] = useState<HowLong>("term");

  const projectRoles = (roles.data ?? []).filter((role) => role.project_id === projectId);
  const days = howLong === "term" ? daysUntilTermEnd() : Number(howLong);
  const until = new Date(Date.now() + days * 86_400_000).toISOString();

  if (!open) return null;

  return (
    <Modal open onClose={onClose} busy={create.isPending} size="sm" labelledBy="request-title">
      <ModalHeader
        title="Ask for access"
        titleId="request-title"
        lede="Say what you need and why. A lab manager decides — you'll see the answer here."
      />
      <div className="flex flex-col gap-3.5 px-6">
        <div>
          <FieldLabel htmlFor="req-project">What do you need to use?</FieldLabel>
          <Select
            id="req-project"
            value={projectId}
            onChange={(event) => {
              setProjectId(event.target.value);
              setRoleKey("");
            }}
          >
            <option value="">Choose…</option>
            {(projects.data ?? []).map((entry) => (
              <option key={entry.project.id} value={entry.project.id}>
                {entry.project.name}
              </option>
            ))}
          </Select>
        </div>
        <div>
          <FieldLabel htmlFor="req-role">What do you need to do there?</FieldLabel>
          <Select
            id="req-role"
            value={roleKey}
            disabled={!projectId}
            onChange={(event) => setRoleKey(event.target.value)}
          >
            <option value="">{projectId ? "Choose…" : "Pick a place first"}</option>
            {projectRoles.map((role) => (
              <option key={role.role_key} value={role.role_key}>
                {role.display_name || role.role_key}
              </option>
            ))}
          </Select>
        </div>
        <div>
          {/*
            Every request used to be filed as ninety days regardless of what
            was asked for, so a student who needed the laser for one week was
            recorded as wanting it for a quarter — and the operator deciding it
            had no way to know the difference.

            Three options, no free-form number: a member thinks in weeks and
            terms, not in days, and the resolved date underneath turns the
            choice back into something concrete.
          */}
          <FieldLabel>How long do you need it?</FieldLabel>
          <div className="mb-2 flex flex-wrap gap-2">
            {(
              [
                ["7", "A week"],
                ["30", "A month"],
                ["term", "Until the end of term"],
              ] as Array<[HowLong, string]>
            ).map(([value, label]) => (
              <button
                key={value}
                type="button"
                aria-pressed={howLong === value}
                onClick={() => setHowLong(value)}
                className={`rounded-pill px-3.5 py-[7px] text-[13px] font-semibold motion-tint ${
                  howLong === value ? "bg-accent-dense text-accent-ink" : "bg-tint-2 text-ink"
                }`}
              >
                {label}
              </button>
            ))}
          </div>
          <FieldHint>
            If it&rsquo;s approved, it runs to {formatLongDate(until)} and then stops. You can ask
            again.
          </FieldHint>
        </div>
        <div>
          <FieldLabel htmlFor="req-why">Why?</FieldLabel>
          <Input
            id="req-why"
            value={why}
            onChange={(event) => setWhy(event.target.value)}
            placeholder="Running the Trotec for my capstone build"
          />
        </div>
      </div>
      {/* The sheet becomes its own result. A member is being told who decides
          and that nothing else is needed from them, and a dialog that closes
          itself takes both sentences with it. */}
      {askOutcome && <ActionOutcome outcome={askOutcome} className="mx-6 mb-1" />}

      <ModalFooter>
        <Button
          variant="accent"
          disabled={!projectId || !roleKey}
          isPending={create.isPending}
          onClick={async () => {
            try {
              await create.mutateAsync({
                project_id: projectId,
                role_key: roleKey,
                justification: why,
                duration_days: days,
              });
              setAskOutcome({
                kind: "applied",
                message: "Asked",
                detail: "You'll see the answer on this page — nothing else is needed from you.",
              });
            } catch (error) {
              setAskOutcome(outcomeFromError(error));
            }
          }}
        >
          Send request
        </Button>
        <Button onClick={onClose}>Cancel</Button>
      </ModalFooter>
    </Modal>
  );
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
      title={status === "approved" ? "Approve requests" : "Deny requests"}
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
