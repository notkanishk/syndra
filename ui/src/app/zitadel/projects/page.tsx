"use client";

import { useState } from "react";

import { EmptyState, ListStates, RowSkeleton } from "@/components/states";
import {
  UpstreamShell,
  WriteFooter,
  WriteResult,
} from "@/components/upstream/UpstreamShell";
import { Mono } from "@/components/ui/Badge";
import { CopyableValue } from "@/components/ui/CopyableValue";
import { TechnicalDetail } from "@/components/ui/TechnicalDetail";
import { Button } from "@/components/ui/Button";
import { Card, CardColumns, CardHeader } from "@/components/ui/Card";
import { AcknowledgeCount } from "@/components/ui/Acknowledge";
import { ApiError } from "@/lib/api-client";
import { outcomeFromError, succeeded, type ActionOutcome as Outcome } from "@/lib/outcome";
import { FieldHint, FieldLabel, Input } from "@/components/ui/Input";
import { Modal, ModalHeader } from "@/components/ui/Modal";
import {
  useUpstreamCreateRole,
  useUpstreamDeleteRole,
  useUpstreamProjectRoles,
  useUpstreamProjects,
  useUpstreamUpdateRole,
  type UpstreamProjectRole,
} from "@/lib/queries/useUpstream";
import { sortByName } from "@/lib/sort";

/** What is recorded: the same as making the role under Access › Roles. */
const RECORDED = "Syndra recorded this in Audit before it changed Zitadel, as Access › Roles does.";

/** The word for an outcome nobody knows (C6, W13): never "Nothing was changed". */
const NOT_KNOWN = "Not known";

/**
 * A role write's error as an outcome. OUTCOME_UNKNOWN is a write whose answer
 * was lost and whose read-back failed: Zitadel may hold the change, so it is a
 * neutral result in its own words, not a failure that "changed nothing".
 */
function roleWriteOutcome(error: unknown, what: string): Outcome {
  if (error instanceof ApiError && error.code === "OUTCOME_UNKNOWN") {
    return {
      kind: "no_change",
      message: `Zitadel didn't answer, so Syndra can't tell whether ${what}`,
      detail: "Syndra kept its record and wrote this to Audit.",
      // Re-audit R6: the step is said as the step, never followed by "nothing to do".
      next: "Open this project again in a minute to see what Zitadel holds.",
    };
  }
  return outcomeFromError(error);
}

/**
 * Projects and their roles, as the identity provider holds them.
 *
 * This is the list `/roles` cannot show: GET /api/v1/roles returns only roles
 * Syndra created, so a role somebody made upstream is invisible there. That is
 * exactly the gap this page exists to close.
 */
export default function UpstreamProjectsPage() {
  const projects = useUpstreamProjects();
  const [selected, setSelected] = useState<string | null>(null);
  const [editing, setEditing] = useState<UpstreamProjectRole | "new" | null>(null);

  const rows = sortByName(projects.data?.items ?? [], (p) => p.name);
  const activeId = selected ?? rows[0]?.id ?? null;
  const roles = useUpstreamProjectRoles(activeId);

  return (
    <UpstreamShell
      title="Projects and their roles"
      lede="Read live from Zitadel, including roles Syndra never created."
      syndraHref="/projects"
      syndraLabel="See the same projects as Syndra understands them"
    >
      <div className="flex flex-col items-stretch gap-4 tablet:flex-row tablet:flex-wrap tablet:items-start">
        <Card className="w-full tablet:w-[280px] tablet:min-w-[240px] tablet:flex-none">
          <CardHeader title="Projects" count={projects.isLoading ? undefined : rows.length} />
          <ListStates
            isLoading={projects.isLoading}
            error={projects.error}
            isEmpty={rows.length === 0}
            onRetry={() => projects.refetch()}
            // `fine` is said only when Syndra answered and Zitadel did not; ErrorState drops it for
            // "not set up" (a Syndra-side gap) and for a network fault.
            errorTitle="Couldn't read projects from Zitadel."
            fine="Syndra itself is fine."
            skeleton={<RowSkeleton rows={5} avatar={false} label="Reading projects" />}
            empty={
              <EmptyState
                title="Zitadel has no projects."
                guidance="Either there are none, or the account Syndra uses to read Zitadel is not allowed to see them."
              />
            }
          >
            {rows.map((project) => (
              <button
                key={project.id}
                type="button"
                onClick={() => setSelected(project.id)}
                aria-current={activeId === project.id ? "true" : undefined}
                className={`row-divider flex w-full flex-col items-start px-4 py-3 text-left motion-tint ${
                  activeId === project.id ? "bg-accent-soft/60" : "hover-tint"
                }`}
              >
                <span className="text-[14.5px] font-semibold [overflow-wrap:anywhere]">{project.name}</span>
              </button>
            ))}
          </ListStates>
        </Card>

        <Card className="w-full tablet:min-w-[420px] tablet:flex-1">
          <CardHeader
            title="Roles in this project"
            count={roles.data?.items.length}
            action={
              activeId ? (
                <Button size="sm" variant="outline" onClick={() => setEditing("new")}>
                  New role in Zitadel
                </Button>
              ) : undefined
            }
          />

          {activeId && (
            <div className="row-divider px-5 py-3">
              <TechnicalDetail placeKey={`project:${activeId}`}>
                <CopyableValue value={activeId} label="Project id" />
              </TechnicalDetail>
            </div>
          )}

          <ListStates
            head={
              <CardColumns>
                <span className="flex-1">Role</span>
                <span className="w-[160px]">Group</span>
                <span className="w-[160px]" />
              </CardColumns>
            }
            isLoading={roles.isLoading}
            error={roles.error}
            isEmpty={(roles.data?.items ?? []).length === 0}
            onRetry={() => roles.refetch()}
            errorTitle="Couldn't read this project's roles from Zitadel."
            fine="Syndra itself is fine."
            skeleton={<RowSkeleton rows={4} avatar={false} label="Reading roles" />}
            empty={
              <EmptyState
                title="This project has no roles in Zitadel."
                guidance="Create one here or under Access › Roles. Either way, Syndra keeps track of it."
              />
            }
          >
            {sortByName(roles.data?.items ?? [], (r) => r.displayName || r.key, (r) => r.key).map((role) => (
              <RoleRow
                key={role.key}
                projectId={activeId!}
                role={role}
                onEdit={() => setEditing(role)}
              />
            ))}
          </ListStates>
        </Card>
      </div>

      {editing && activeId && (
        <RoleDialog
          projectId={activeId}
          role={editing === "new" ? null : editing}
          onClose={() => setEditing(null)}
        />
      )}
    </UpstreamShell>
  );
}

function RoleRow({
  projectId,
  role,
  onEdit,
}: {
  projectId: string;
  role: UpstreamProjectRole;
  onEdit: () => void;
}) {
  const remove = useUpstreamDeleteRole();
  const [outcome, setOutcome] = useState<Outcome | null>(null);
  const [confirming, setConfirming] = useState(false);
  const [acknowledged, setAcknowledged] = useState(false);
  // Who Zitadel shows holding the role, as the refusal named them. Set only
  // once Syndra has refused for that reason; the next press confirms the count.
  const [held, setHeld] = useState<{ count: number; holders: string } | null>(null);
  const done = succeeded(outcome);
  const name = role.displayName || role.key;

  return (
    <>
      <div className="row-divider flex min-h-[60px] flex-col items-start gap-1.5 px-5 py-3 tablet:flex-row tablet:flex-wrap tablet:items-center tablet:gap-4">
        <span className="min-w-[200px] flex-1 text-[14.5px] [overflow-wrap:anywhere]">
          {role.displayName || role.key} <Mono className="text-faint">{role.key}</Mono>
        </span>
        <span className="text-[13.5px] text-muted [overflow-wrap:anywhere] tablet:w-[160px]">{role.group || "—"}</span>
        <span className="flex justify-end gap-2 tablet:w-[160px]">
          <Button size="sm" onClick={onEdit} aria-label={`Edit ${role.displayName || role.key}`}>
            Edit
          </Button>
          <Button
            size="sm"
            variant="danger"
            onClick={() => setConfirming(true)}
            aria-label={`Delete ${role.displayName || role.key}`}
          >
            Delete
          </Button>
        </span>
      </div>

      {confirming && (
        <Modal open onClose={() => setConfirming(false)} busy={remove.isPending} size="sm">
          <ModalHeader
            title={`Delete ${name} in Zitadel?`}
            lede={
              done
                ? undefined
                : "Syndra checks first. It refuses while Syndra itself gives this role, or while it can’t tell who holds it. If people hold it in Zitadel, it names them and asks you again before anything changes."
            }
          />
          {/* The count comes from Syndra's refusal, never from this screen's own reading:
              the confirmation names exactly the people the backend will take it from. */}
          {!done && held && (
            <div className="px-6">
              <AcknowledgeCount
                checked={acknowledged}
                onChange={setAcknowledged}
                count={held.count}
                noun="people"
                verb="revokes this role for"
                consequence={`Zitadel shows ${held.holders} holding ${name}. Deleting it revokes it for them at once.`}
              />
            </div>
          )}
          {outcome && <WriteResult outcome={outcome} word="Deleted" neutral={NOT_KNOWN} />}

          <WriteFooter outcome={outcome} onClose={() => setConfirming(false)}>
            {/* Retires on success: left armed, a second click deleted the same
                role again. A refused attempt keeps the button so there is a retry. */}
            <Button
              variant="dangerConfirm"
              disabled={held !== null && !acknowledged}
              reason={held && !acknowledged ? "Check the box above to confirm who loses it." : undefined}
              isPending={remove.isPending}
              onClick={async () => {
                try {
                  await remove.mutateAsync({ projectId, key: role.key, confirmHolders: held?.count });
                  setOutcome({ kind: "applied", message: `${role.key} deleted in Zitadel`, detail: RECORDED });
                } catch (error) {
                  if (error instanceof ApiError && error.code === "ROLE_HELD") {
                    setHeld({ count: Number(error.details?.count ?? 0), holders: error.details?.holders ?? "" });
                    setAcknowledged(false);
                    setOutcome(null);
                    return;
                  }
                  setOutcome(roleWriteOutcome(error, `Zitadel deleted ${name}, and took it from anybody holding it`));
                }
              }}
            >
              {held ? `Delete and revoke it for ${held.count} ${held.count === 1 ? "person" : "people"}` : "Delete role in Zitadel"}
            </Button>
          </WriteFooter>
        </Modal>
      )}
    </>
  );
}

function RoleDialog({
  projectId,
  role,
  onClose,
}: {
  projectId: string;
  role: UpstreamProjectRole | null;
  onClose: () => void;
}) {
  const create = useUpstreamCreateRole();
  const [outcome, setOutcome] = useState<Outcome | null>(null);
  const update = useUpstreamUpdateRole();
  const [key, setKey] = useState(role?.key ?? "");
  const [displayName, setDisplayName] = useState(role?.displayName ?? "");
  const [group, setGroup] = useState(role?.group ?? "");
  const done = succeeded(outcome);

  const busy = create.isPending || update.isPending;

  return (
    <Modal open onClose={onClose} busy={busy} size="sm">
      <ModalHeader
        title={role ? `Edit ${role.key} in Zitadel` : "New role in Zitadel"}
        lede={
          done
            ? undefined
            : "The same as Access › Roles: Syndra records it, then changes Zitadel, and puts its record back if Zitadel did not make the change."
        }
      />
      {!done && (
        <div className="flex flex-col gap-3.5 px-6">
          <div>
            <FieldLabel htmlFor="upstream-role-key">Role key</FieldLabel>
            <Input
              id="upstream-role-key"
              value={key}
              disabled={Boolean(role)}
              onChange={(event) => setKey(event.target.value)}
              placeholder="trained"
            />
            <FieldHint>The short name other systems see, e.g. trained. Lowercase, no spaces.</FieldHint>
          </div>
          <div>
            <FieldLabel htmlFor="upstream-role-name">Display name</FieldLabel>
            <Input
              id="upstream-role-name"
              value={displayName}
              onChange={(event) => setDisplayName(event.target.value)}
              placeholder="Trained on the laser cutter"
            />
          </div>
          <div>
            <FieldLabel htmlFor="upstream-role-group">Group</FieldLabel>
            <Input
              id="upstream-role-group"
              value={group}
              onChange={(event) => setGroup(event.target.value)}
              placeholder="Safety-gated"
            />
            <FieldHint>Optional. A label Zitadel uses to group related roles; Syndra ignores it.</FieldHint>
          </div>
        </div>
      )}
      {/* The dialog reports its own result and stays open to do it, so a refusal
          and what Syndra recorded are read where the change was made. */}
      {outcome && (
        <WriteResult outcome={outcome} word={role ? "Saved" : "Created"} neutral={NOT_KNOWN} className="mx-6 mb-1 mt-3" />
      )}

      <WriteFooter outcome={outcome} onClose={onClose}>
        <Button
          // Saving a display name grants and revokes nothing: non-destructive, so accent.
          variant="accent"
          disabled={!key.trim()}
          reason={!key.trim() ? "Enter a role key first." : undefined}
          isPending={busy}
          onClick={async () => {
            try {
              if (role) {
                await update.mutateAsync({ projectId, key: role.key, displayName, group });
                setOutcome({ kind: "applied", message: `${role.key} updated in Zitadel`, detail: RECORDED });
              } else {
                await create.mutateAsync({ projectId, key: key.trim(), displayName, group });
                setOutcome({ kind: "applied", message: `${key.trim()} created in Zitadel`, detail: RECORDED });
              }
            } catch (error) {
              setOutcome(
                roleWriteOutcome(error, role ? `Zitadel saved the change to ${role.key}` : `Zitadel created ${key.trim()}`),
              );
            }
          }}
        >
          {role ? "Save in Zitadel" : "Create in Zitadel"}
        </Button>
      </WriteFooter>
    </Modal>
  );
}
