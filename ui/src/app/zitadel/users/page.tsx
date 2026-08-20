"use client";

import Link from "next/link";
import { useMemo, useState } from "react";

import { EmptyState, ListStates, RowSkeleton } from "@/components/states";
import { DirectWriteWarning, UpstreamShell } from "@/components/upstream/UpstreamShell";
import { ActionOutcome } from "@/components/ui/ActionOutcome";
import { Mono } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { outcomeFromError, type ActionOutcome as Outcome } from "@/lib/outcome";
import { Card, CardHeader } from "@/components/ui/Card";
import { FieldLabel, Input } from "@/components/ui/Input";
import { Modal, ModalFooter, ModalHeader } from "@/components/ui/Modal";
import { Select } from "@/components/ui/Select";
import { ProjectName } from "@/components/names";
import {
  useUpstreamAssignGrant,
  useUpstreamProjectRoles,
  useUpstreamProjects,
  useUpstreamRemoveGrant,
  useUpstreamUpdateGrant,
  useUpstreamUserGrants,
  useUpstreamUsers,
} from "@/lib/queries/useUpstream";
import { useDebounce } from "@/lib/useDebounce";

/**
 * Users and their grants, as the identity provider holds them.
 *
 * Syndra's own person page answers "what can this person get into, and why".
 * This page answers a narrower and occasionally vital question: "what does the
 * provider actually have on file for them" — which is the only way to see a
 * grant Syndra has no record of.
 */
export default function UpstreamUsersPage() {
  const users = useUpstreamUsers();
  const [query, setQuery] = useState("");
  const [selected, setSelected] = useState<string | null>(null);
  const search = useDebounce(query, 200).trim().toLowerCase();

  const rows = useMemo(() => {
    const all = users.data?.items ?? [];
    if (!search) return all;
    return all.filter((user) =>
      [user.displayName, user.userName, user.email].join(" ").toLowerCase().includes(search),
    );
  }, [users.data, search]);

  const activeId = selected ?? rows[0]?.id ?? null;
  const active = rows.find((user) => user.id === activeId);

  return (
    <UpstreamShell
      title="Users"
      lede="Read live from the identity provider, including accounts Syndra has never seen."
      syndraHref="/users"
      syndraLabel="See the same people as Syndra understands them"
    >
      <div className="flex flex-wrap items-start gap-5">
        <Card className="w-[320px] min-w-[260px] flex-none">
          <CardHeader title="Accounts" count={rows.length} />
          <div className="row-divider px-4 py-3">
            <Input
              value={query}
              onChange={(event) => setQuery(event.target.value)}
              placeholder="Search name, username or email"
              aria-label="Search upstream users"
            />
          </div>
          <ListStates
            isLoading={users.isLoading}
            error={users.error}
            isEmpty={rows.length === 0}
            onRetry={() => users.refetch()}
            errorTitle="Couldn't read users from the identity provider."
            skeleton={<RowSkeleton rows={6} label="Reading users" />}
            empty={
              <EmptyState
                title={search ? "Nobody matches that." : "No users upstream."}
                guidance={
                  search
                    ? "Try part of an email address."
                    : "Either none exist, or the service account cannot see them."
                }
              />
            }
          >
            {rows.slice(0, 100).map((user) => (
              <button
                key={user.id}
                type="button"
                onClick={() => setSelected(user.id)}
                aria-current={activeId === user.id ? "true" : undefined}
                className={`row-divider flex w-full flex-col items-start px-4 py-2.5 text-left motion-tint ${
                  activeId === user.id ? "bg-accent-soft/60" : "hover:bg-[var(--hover)]"
                }`}
              >
                <span className="truncate text-[14.5px] font-semibold">
                  {user.displayName || user.userName}
                </span>
                <span className="truncate text-[12.5px] text-faint">{user.email}</span>
              </button>
            ))}
            {rows.length > 100 && (
              <div className="row-divider px-4 py-2.5 text-[13px] text-faint">
                and {rows.length - 100} more — narrow the search
              </div>
            )}
          </ListStates>
        </Card>

        <div className="min-w-[420px] flex-1">
          {active ? (
            <UserGrants
              userId={active.id}
              name={active.displayName || active.userName}
              state={active.state}
            />
          ) : null}
        </div>
      </div>
    </UpstreamShell>
  );
}

function UserGrants({ userId, name, state }: { userId: string; name: string; state: string }) {
  const grants = useUpstreamUserGrants(userId);
  const remove = useUpstreamRemoveGrant();
  const [outcome, setOutcome] = useState<Outcome | null>(null);
  const [assigning, setAssigning] = useState(false);
  const [editing, setEditing] = useState<{ grantId: string; roleKeys: string[]; projectId: string } | null>(
    null,
  );

  const rows = grants.data?.items ?? [];

  return (
    <Card>
      <CardHeader
        title={name}
        count={rows.length}
        note={state}
        action={
          <Button size="sm" variant="danger" onClick={() => setAssigning(true)}>
            Assign a grant upstream
          </Button>
        }
      />

      <div className="row-divider px-5 py-3 text-[13.5px] leading-[1.55] text-muted">
        Everything below is what the identity provider holds for this person.{" "}
        <Link href={`/users/${userId}`} className="font-semibold text-accent-text">
          Syndra&rsquo;s explanation of the same access →
        </Link>
      </div>

      <ListStates
        isLoading={grants.isLoading}
        error={grants.error}
        isEmpty={rows.length === 0}
        onRetry={() => grants.refetch()}
        errorTitle="Couldn't read this person's grants."
        skeleton={<RowSkeleton rows={3} avatar={false} label="Reading grants" />}
        empty={
          <EmptyState
            title="No grants upstream."
            guidance="The identity provider holds nothing for this account."
          />
        }
      >
        {rows.map((grant) => (
          <div key={grant.id} className="row-divider flex flex-wrap items-center gap-4 px-5 py-3">
            <span className="min-w-[200px] flex-1 truncate text-[14.5px]">
              <ProjectName id={grant.projectId} fallback={grant.projectId} />{" "}
              {grant.roleKeys.map((key) => (
                <Mono key={key} className="mr-1.5 text-muted">
                  {key}
                </Mono>
              ))}
            </span>
            <Mono className="w-[170px] shrink-0 truncate text-faint">{grant.id}</Mono>
            <span className="flex gap-2">
              <Button
                size="sm"
                onClick={() =>
                  setEditing({
                    grantId: grant.id,
                    roleKeys: grant.roleKeys,
                    projectId: grant.projectId,
                  })
                }
              >
                Change roles
              </Button>
              <Button
                size="sm"
                variant="danger"
                isPending={remove.isPending}
                onClick={async () => {
                  try {
                    await remove.mutateAsync({ userId, grantId: grant.id });
                    setOutcome({
                      kind: "applied",
                      message: "Grant removed in Zitadel",
                      detail: "Syndra has no record of this change beyond the audit line.",
                    });
                  } catch (error) {
                    setOutcome(outcomeFromError(error));
                  }
                }}
              >
                Remove
              </Button>
            </span>
          </div>
        ))}
      </ListStates>

      {assigning && <AssignDialog userId={userId} onClose={() => setAssigning(false)} />}
      {editing && (
        <EditRolesDialog userId={userId} grant={editing} onClose={() => setEditing(null)} />
      )}
    </Card>
  );
}

function AssignDialog({ userId, onClose }: { userId: string; onClose: () => void }) {
  const projects = useUpstreamProjects();
  const [projectId, setProjectId] = useState("");
  const roles = useUpstreamProjectRoles(projectId || null);
  const [keys, setKeys] = useState("");
  const assign = useUpstreamAssignGrant();
  const [outcome, setOutcome] = useState<Outcome | null>(null);

  const parsed = keys
    .split(",")
    .map((key) => key.trim())
    .filter(Boolean);

  return (
    <Modal open onClose={onClose} busy={assign.isPending} size="sm">
      <ModalHeader
        title="Assign a grant upstream"
        lede="This creates the grant in the identity provider directly."
      />
      <div className="flex flex-col gap-3.5 px-6">
        <div>
          <FieldLabel htmlFor="assign-project">Project</FieldLabel>
          <Select
            id="assign-project"
            value={projectId}
            onChange={(event) => setProjectId(event.target.value)}
          >
            <option value="">Choose…</option>
            {(projects.data?.items ?? []).map((project) => (
              <option key={project.id} value={project.id}>
                {project.name}
              </option>
            ))}
          </Select>
        </div>
        <div>
          <FieldLabel htmlFor="assign-roles">Role keys, comma separated</FieldLabel>
          <Input
            id="assign-roles"
            value={keys}
            onChange={(event) => setKeys(event.target.value)}
            placeholder="trained, maintainer"
          />
          {roles.data && (
            <p className="mt-1.5 text-[12.5px] text-faint">
              Available: {roles.data.items.map((role) => role.key).join(", ") || "none"}
            </p>
          )}
        </div>
        <DirectWriteWarning what="Syndra will see this as access it did not cause." />
      </div>
      {outcome && <ActionOutcome outcome={outcome} className="mx-6 mb-1" />}

      <ModalFooter>
        <Button
          variant="danger"
          disabled={!projectId || parsed.length === 0}
          isPending={assign.isPending}
          onClick={async () => {
            try {
              await assign.mutateAsync({ userId, projectId, roleKeys: parsed });
              setOutcome({
                kind: "applied",
                message: "Grant assigned in Zitadel",
                detail: "Syndra has no record of this change beyond the audit line.",
              });
              onClose();
            } catch (error) {
              setOutcome(outcomeFromError(error));
            }
          }}
        >
          Assign upstream
        </Button>
        <Button onClick={onClose}>Cancel</Button>
      </ModalFooter>
    </Modal>
  );
}

function EditRolesDialog({
  userId,
  grant,
  onClose,
}: {
  userId: string;
  grant: { grantId: string; roleKeys: string[]; projectId: string };
  onClose: () => void;
}) {
  const update = useUpstreamUpdateGrant();
  const [outcome, setOutcome] = useState<Outcome | null>(null);
  const [keys, setKeys] = useState(grant.roleKeys.join(", "));

  const parsed = keys
    .split(",")
    .map((key) => key.trim())
    .filter(Boolean);

  return (
    <Modal open onClose={onClose} busy={update.isPending} size="sm">
      <ModalHeader
        title="Change the roles on this grant"
        lede="The new set replaces the old one entirely — anything you leave out is removed."
      />
      <div className="flex flex-col gap-3.5 px-6">
        <div>
          <FieldLabel htmlFor="edit-roles">Role keys, comma separated</FieldLabel>
          <Input
            id="edit-roles"
            value={keys}
            onChange={(event) => setKeys(event.target.value)}
          />
        </div>
        <DirectWriteWarning what="Replacing a role set upstream can silently remove access Syndra thinks it granted." />
      </div>
      {outcome && <ActionOutcome outcome={outcome} className="mx-6 mb-1" />}

      <ModalFooter>
        <Button
          variant="danger"
          disabled={parsed.length === 0}
          isPending={update.isPending}
          onClick={async () => {
            try {
              await update.mutateAsync({ userId, grantId: grant.grantId, roleKeys: parsed });
              setOutcome({
                kind: "applied",
                message: "Grant updated in Zitadel",
                detail: "Syndra has no record of this change beyond the audit line.",
              });
              onClose();
            } catch (error) {
              setOutcome(outcomeFromError(error));
            }
          }}
        >
          Replace roles upstream
        </Button>
        <Button onClick={onClose}>Cancel</Button>
      </ModalFooter>
    </Modal>
  );
}
