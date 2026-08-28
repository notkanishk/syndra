"use client";

import Link from "next/link";
import { useMemo, useState } from "react";

import { EmptyState, ListStates, RowSkeleton } from "@/components/states";
import { DirectWriteWarning, UpstreamShell } from "@/components/upstream/UpstreamShell";
import { AcknowledgeCount } from "@/components/ui/Acknowledge";
import { ActionOutcome } from "@/components/ui/ActionOutcome";
import { Mono } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { outcomeFromError, type ActionOutcome as Outcome } from "@/lib/outcome";
import { Card, CardHeader } from "@/components/ui/Card";
import { FieldHint, FieldLabel, Input } from "@/components/ui/Input";
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
import { humanizeKey } from "@/lib/format";

/** "USER_STATE_ACTIVE" → "Active": the API's word, in ours. */
function userStateLabel(state: string): string {
  return humanizeKey(state.replace(/^USER_STATE_/, "").toLowerCase());
}

/** What is recorded when a change goes straight to Zitadel: one line, and no explanation. */
const AUDIT_ONLY =
  "Syndra recorded one line in Audit and nothing else; it will not explain this access anywhere.";

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
      title="People in Zitadel"
      lede="Read live from Zitadel, including accounts Syndra has never seen."
      syndraHref="/users"
      syndraLabel="See the same people as Syndra understands them"
    >
      <div className="flex flex-col items-stretch gap-5 tablet:flex-row tablet:flex-wrap tablet:items-start">
        <Card className="w-full tablet:w-[320px] tablet:min-w-[260px] tablet:flex-none">
          <CardHeader title="People" count={rows.length} />
          <div className="row-divider px-4 py-3">
            <Input
              value={query}
              onChange={(event) => setQuery(event.target.value)}
              placeholder="Name, username or email"
              aria-label="Search people by name, username or email"
            />
          </div>
          <ListStates
            isLoading={users.isLoading}
            error={users.error}
            isEmpty={rows.length === 0}
            onRetry={() => users.refetch()}
            errorTitle="Couldn't read people from Zitadel. Syndra itself is fine."
            skeleton={<RowSkeleton rows={6} label="Reading people" />}
            empty={
              <EmptyState
                title={search ? "Nobody matches that." : "Zitadel has no people."}
                guidance={
                  search
                    ? "Try part of an email address."
                    : "Either there are none, or the account Syndra uses to read Zitadel is not allowed to see them."
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

        <div className="w-full tablet:min-w-[420px] tablet:flex-1">
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
  const [revoking, setRevoking] = useState<{ grantId: string; roleKeys: string[]; projectId: string } | null>(
    null,
  );
  const [acknowledged, setAcknowledged] = useState(false);

  const rows = grants.data?.items ?? [];

  return (
    <Card>
      <CardHeader
        title={name}
        count={rows.length}
        note={userStateLabel(state)}
        action={
          <Button size="sm" variant="danger" onClick={() => setAssigning(true)}>
            Give roles in Zitadel
          </Button>
        }
      />

      <div className="row-divider px-5 py-3 text-[13.5px] leading-[1.55] text-muted">
        Everything below is what Zitadel holds for this person.{" "}
        <Link href={`/users/${userId}`} className="font-semibold text-accent-text">
          Syndra&rsquo;s explanation of the same access →
        </Link>
      </div>

      <ListStates
        isLoading={grants.isLoading}
        error={grants.error}
        isEmpty={rows.length === 0}
        onRetry={() => grants.refetch()}
        errorTitle="Couldn't read this person's roles from Zitadel. Syndra itself is fine."
        skeleton={<RowSkeleton rows={3} avatar={false} label="Reading roles" />}
        empty={
          <EmptyState
            title="No roles in Zitadel."
            guidance="Zitadel has given this person no roles."
          />
        }
      >
        {rows.map((grant) => (
          <div key={grant.id} className="row-divider flex min-h-[60px] flex-col items-start gap-1.5 px-5 py-3 tablet:flex-row tablet:flex-wrap tablet:items-center tablet:gap-4">
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
                onClick={() => {
                  setAcknowledged(false);
                  setRevoking({
                    grantId: grant.id,
                    roleKeys: grant.roleKeys,
                    projectId: grant.projectId,
                  });
                }}
              >
                Revoke roles
              </Button>
            </span>
          </div>
        ))}
      </ListStates>

      {outcome && <ActionOutcome outcome={outcome} className="mx-5 mb-4" />}

      {revoking && (
        <Modal
          open
          onClose={() => setRevoking(null)}
          busy={remove.isPending}
          size="sm"
          labelledBy="revoke-grant-title"
        >
          <ModalHeader
            title={`Revoke ${revoking.roleKeys.length === 1 ? "this role" : "these roles"} from ${name} in Zitadel?`}
            titleId="revoke-grant-title"
            lede="Revoking ends their access. It happens the moment you press the button, with no preview, and Syndra keeps no record of why."
          />
          <div className="flex flex-col gap-3.5 px-6">
            <div className="text-[14.5px]">
              <ProjectName id={revoking.projectId} fallback={revoking.projectId} />{" "}
              {revoking.roleKeys.map((key) => (
                <Mono key={key} className="mr-1.5 text-muted">
                  {key}
                </Mono>
              ))}
            </div>
            <AcknowledgeCount
              checked={acknowledged}
              onChange={setAcknowledged}
              count={revoking.roleKeys.length}
              noun="roles"
              verb="revokes"
              consequence={`${name} loses these roles in Zitadel at once. Anything Syndra believes it gave them may be given back within about a minute, and the change will show up under Unexplained access.`}
            />
          </div>
          <ModalFooter>
            <Button
              variant="dangerConfirm"
              disabled={!acknowledged}
              isPending={remove.isPending}
              onClick={async () => {
                try {
                  await remove.mutateAsync({ userId, grantId: revoking.grantId });
                  setOutcome({
                    kind: "applied",
                    message: "Roles revoked in Zitadel",
                    detail: AUDIT_ONLY,
                  });
                  setRevoking(null);
                } catch (error) {
                  setOutcome(outcomeFromError(error));
                  setRevoking(null);
                }
              }}
            >
              {revoking.roleKeys.length === 1
                ? "Revoke this role in Zitadel"
                : `Revoke these ${revoking.roleKeys.length} roles in Zitadel`}
            </Button>
            <Button onClick={() => setRevoking(null)}>Keep the roles</Button>
          </ModalFooter>
        </Modal>
      )}

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
  const [acknowledged, setAcknowledged] = useState(false);

  const parsed = keys
    .split(",")
    .map((key) => key.trim())
    .filter(Boolean);

  return (
    <Modal open onClose={onClose} busy={assign.isPending} size="sm">
      <ModalHeader
        title="Give roles in Zitadel"
        lede="This gives the roles in Zitadel directly, skipping Syndra."
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
          <FieldLabel htmlFor="assign-roles">Role keys</FieldLabel>
          <Input
            id="assign-roles"
            value={keys}
            onChange={(event) => setKeys(event.target.value)}
            placeholder="trained, maintainer"
          />
          <FieldHint>
            The short name other systems see, e.g. trained. Pick from the list below, comma
            separated.
          </FieldHint>
          {roles.data && (
            <p className="mt-1.5 text-[12.5px] text-faint">
              Available: {roles.data.items.map((role) => role.key).join(", ") || "none"}
            </p>
          )}
        </div>
        <DirectWriteWarning
          what="Syndra will see this as access it did not give."
          acknowledged={acknowledged}
          onAcknowledge={setAcknowledged}
        />
      </div>
      {outcome && <ActionOutcome outcome={outcome} className="mx-6 mb-1" />}

      <ModalFooter>
        <Button
          variant="danger"
          disabled={!projectId || parsed.length === 0 || !acknowledged}
          isPending={assign.isPending}
          onClick={async () => {
            try {
              await assign.mutateAsync({ userId, projectId, roleKeys: parsed });
              setOutcome({
                kind: "applied",
                message: "Roles given in Zitadel",
                detail: AUDIT_ONLY,
              });
              onClose();
            } catch (error) {
              setOutcome(outcomeFromError(error));
            }
          }}
        >
          Give roles in Zitadel
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
  const [acknowledged, setAcknowledged] = useState(false);

  const parsed = keys
    .split(",")
    .map((key) => key.trim())
    .filter(Boolean);

  return (
    <Modal open onClose={onClose} busy={update.isPending} size="sm">
      <ModalHeader
        title="Replace this person's roles in Zitadel"
        lede="The new set replaces the old one entirely — any role you leave out is revoked."
      />
      <div className="flex flex-col gap-3.5 px-6">
        <div>
          <FieldLabel htmlFor="edit-roles">Role keys</FieldLabel>
          <Input
            id="edit-roles"
            value={keys}
            onChange={(event) => setKeys(event.target.value)}
          />
          <FieldHint>The short name other systems see, e.g. trained, comma separated.</FieldHint>
        </div>
        <DirectWriteWarning
          what="Replacing the roles here can quietly revoke access Syndra believes it gave."
          acknowledged={acknowledged}
          onAcknowledge={setAcknowledged}
        />
      </div>
      {outcome && <ActionOutcome outcome={outcome} className="mx-6 mb-1" />}

      <ModalFooter>
        <Button
          variant="danger"
          disabled={parsed.length === 0 || !acknowledged}
          isPending={update.isPending}
          onClick={async () => {
            try {
              await update.mutateAsync({ userId, grantId: grant.grantId, roleKeys: parsed });
              setOutcome({
                kind: "applied",
                message: "Roles replaced in Zitadel",
                detail: AUDIT_ONLY,
              });
              onClose();
            } catch (error) {
              setOutcome(outcomeFromError(error));
            }
          }}
        >
          Replace roles in Zitadel
        </Button>
        <Button onClick={onClose}>Cancel</Button>
      </ModalFooter>
    </Modal>
  );
}
