"use client";

import { useMemo, useState } from "react";
import { toast } from "sonner";

import { EmptyState, ListStates, RowSkeleton } from "@/components/states";
import { Avatar } from "@/components/ui/Avatar";
import { Badge, Mono } from "@/components/ui/Badge";
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
import { FieldLabel, Input } from "@/components/ui/Input";
import { FilterPills, Select } from "@/components/ui/Select";
import { Modal, ModalFooter, ModalHeader } from "@/components/ui/Modal";
import { PageHeader } from "@/components/ui/PageHeader";
import { ProjectName, UserName } from "@/components/names";
import { useApplyBulkDecision, useRehearseBulkDecision } from "@/lib/queries/useRequests";
import { useProjects } from "@/lib/queries/useProjects";
import { useGlobalRoleCatalog } from "@/lib/queries/useRoles";
import {
  useCreateRequest,
  useDecideRequest,
  useRequestsAdmin,
  useRequestsMine,
  type AccessRequest,
} from "@/lib/queries/useRequests";
import { Relative } from "@/components/ui/Time";

type StatusFilter = "pending" | "approved" | "rejected" | "all";

export function RequestsScreen({ isOperator, userId }: { isOperator: boolean; userId: string }) {
  return isOperator ? <OperatorQueue /> : <MemberRequests userId={userId} />;
}

/** The operator queue — same row shape as the Today block, terminal actions. */
function OperatorQueue() {
  const [status, setStatus] = useState<StatusFilter>("pending");
  const requests = useRequestsAdmin(status);
  const decide = useDecideRequest();
  const [resolved, setResolved] = useState<Set<string>>(new Set());
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
      toast.success(next === "approved" ? "Approved." : "Denied.");
    } catch (error) {
      setResolved((prev) => {
        const copy = new Set(prev);
        copy.delete(entry.id);
        return copy;
      });
      toast.error(error instanceof Error ? error.message : "The decision didn't go through.");
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
            />
          }
        >
          {rows.map((entry) => (
            <div
              key={entry.id}
              className={`row-divider flex items-center gap-[18px] px-5 py-3.5 ${
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
              {entry.status === "pending" ? (
                <div className="flex shrink-0 gap-2">
                  <Button variant="accent" size="sm" onClick={() => act(entry, "approved")}>
                    Approve
                  </Button>
                  <Button size="sm" onClick={() => act(entry, "rejected")}>
                    Deny
                  </Button>
                </div>
              ) : (
                <Badge tone={entry.status === "approved" ? "accent" : "neutral"}>
                  {entry.status === "approved" ? "Approved" : "Denied"}
                </Badge>
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
  const [open, setOpen] = useState(false);

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
            <div key={entry.id} className="row-divider flex items-center gap-[18px] px-5 py-3.5">
              <div className="min-w-0 flex-1">
                <div className="truncate text-[15px] font-semibold">
                  <ProjectName id={entry.project_id} />
                </div>
                <div className="truncate text-[13.5px] text-faint">
                  {entry.justification || "No reason given"}
                </div>
              </div>
              <div className="w-[110px] shrink-0 text-[13px] text-faint">
                <Relative iso={entry.created_at} />
              </div>
              <Badge
                tone={
                  entry.status === "approved"
                    ? "accent"
                    : entry.status === "rejected"
                      ? "neutral"
                      : "warn"
                }
              >
                {entry.status === "approved"
                  ? "Approved"
                  : entry.status === "rejected"
                    ? "Not approved"
                    : "Waiting"}
              </Badge>
            </div>
          ))}
        </ListStates>
      </Card>

      <RequestDialog open={open} onClose={() => setOpen(false)} />
    </div>
  );
}

function RequestDialog({ open, onClose }: { open: boolean; onClose: () => void }) {
  const projects = useProjects();
  const roles = useGlobalRoleCatalog();
  const create = useCreateRequest();

  const [projectId, setProjectId] = useState("");
  const [roleKey, setRoleKey] = useState("");
  const [why, setWhy] = useState("");

  const projectRoles = (roles.data ?? []).filter((role) => role.project_id === projectId);

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
          <FieldLabel htmlFor="req-why">Why?</FieldLabel>
          <Input
            id="req-why"
            value={why}
            onChange={(event) => setWhy(event.target.value)}
            placeholder="Running the Trotec for my capstone build"
          />
        </div>
      </div>
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
                duration_days: 90,
              });
              toast.success("Asked. You'll see the answer here.");
              onClose();
            } catch (error) {
              toast.error(error instanceof Error ? error.message : "That didn't send.");
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
      onRehearse={() => rehearse.mutateAsync(body)}
      onApply={async () => {
        const plan = await apply.mutateAsync(body);
        onApplied();
        return plan;
      }}
      onClose={onClose}
    />
  );
}
