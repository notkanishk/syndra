"use client";

import { ArrowLink } from "@/components/ui/ArrowLink";
import { useMemo, useState } from "react";

import { EmptyState, ListStates, RowSkeleton } from "@/components/states";
import {
  DirectWriteWarning,
  UpstreamShell,
  WriteFooter,
  WriteResult,
} from "@/components/upstream/UpstreamShell";
import { AcknowledgeCount } from "@/components/ui/Acknowledge";
import { Mono } from "@/components/ui/Badge";
import { CopyableValue } from "@/components/ui/CopyableValue";
import { TechnicalDetail } from "@/components/ui/TechnicalDetail";
import { Button } from "@/components/ui/Button";
import { outcomeFromError, type ActionOutcome as Outcome } from "@/lib/outcome";
import { Card, CardHeader } from "@/components/ui/Card";
import { FilterBar } from "@/components/ui/FilterBar";
import { FieldHint, FieldLabel, Input } from "@/components/ui/Input";
import { Modal, ModalHeader } from "@/components/ui/Modal";
import { PaginationRow, usePaging } from "@/components/ui/PaginationRow";
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
import { sortByName, sortNames } from "@/lib/sort";

/** "USER_STATE_ACTIVE" → "Active": the API's word, in ours. */
function userStateLabel(state: string): string {
  return humanizeKey(state.replace(/^USER_STATE_/, "").toLowerCase());
}

/** What is recorded when a change goes straight to Zitadel: one line, and no explanation. */
const AUDIT_ONLY =
  "Syndra recorded one line in Audit and nothing else; it will not explain this access anywhere.";

/** A grant's roles, wrapping: a role key is never cut short. */
function RoleKeys({ keys }: { keys: string[] }) {
  return (
    <>
      {sortNames(keys).map((key) => (
        <Mono key={key} className="mr-1.5 text-muted [overflow-wrap:anywhere]">
          {key}
        </Mono>
      ))}
    </>
  );
}

/** The comma-separated role keys a person typed, as a list. */
function parseKeys(keys: string): string[] {
  return keys
    .split(",")
    .map((key) => key.trim())
    .filter(Boolean);
}

type GrantRef = { grantId: string; roleKeys: string[]; projectId: string };

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
  const paging = usePaging();

  const rows = useMemo(() => {
    const all = sortByName(users.data?.items ?? [], (user) => user.displayName || user.userName);
    if (!search) return all;
    return all.filter((user) =>
      [user.displayName, user.userName, user.email].join(" ").toLowerCase().includes(search),
    );
  }, [users.data, search]);

  const visible = rows.slice(0, paging.limit);
  const activeId = selected ?? rows[0]?.id ?? null;
  const active = rows.find((user) => user.id === activeId);

  return (
    <UpstreamShell
      title="People in Zitadel"
      lede="Read live from Zitadel, including accounts Syndra has never seen."
      syndraHref="/users"
      syndraLabel="See the same people as Syndra understands them"
    >
      <FilterBar
        search={
          <Input
            value={query}
            onChange={(event) => {
              setQuery(event.target.value);
              paging.back();
            }}
            placeholder="Name, username or email"
            aria-label="Search people by name, username or email"
          />
        }
      />

      <div className="flex flex-col items-stretch gap-4 tablet:flex-row tablet:flex-wrap tablet:items-start">
        <Card className="w-full tablet:w-[320px] tablet:min-w-[260px] tablet:flex-none">
          <CardHeader title="People" count={users.isLoading ? undefined : rows.length} />
          <ListStates
            isLoading={users.isLoading}
            error={users.error}
            isEmpty={rows.length === 0}
            onRetry={() => users.refetch()}
            // Not "Syndra itself is fine" — the commonest cause here is Zitadel
            // never having been configured at all, which IS a Syndra-side gap.
            // The detail line below names which one it is.
            errorTitle="Couldn't read people from Zitadel."
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
            {visible.map((user) => (
              <button
                key={user.id}
                type="button"
                onClick={() => setSelected(user.id)}
                aria-current={activeId === user.id ? "true" : undefined}
                className={`row-divider flex w-full flex-col items-start px-4 py-2.5 text-left motion-tint ${
                  activeId === user.id ? "bg-accent-soft/60" : "hover-tint"
                }`}
              >
                <span className="truncate text-[14.5px] font-semibold">
                  {user.displayName || user.userName}
                </span>
                <span className="truncate text-[12.5px] text-faint">{user.email}</span>
              </button>
            ))}
            <PaginationRow
              shown={visible.length}
              total={rows.length}
              onMore={paging.more}
              onAll={() => paging.all(rows.length)}
              onBack={paging.back}
            />
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
  const [assigning, setAssigning] = useState(false);
  const [editing, setEditing] = useState<GrantRef | null>(null);
  const [revoking, setRevoking] = useState<GrantRef | null>(null);

  const rows = grants.data?.items ?? [];

  return (
    <Card>
      <CardHeader
        title={name}
        count={grants.isLoading ? undefined : rows.length}
        note={userStateLabel(state)}
        action={
          <Button size="sm" variant="outline" onClick={() => setAssigning(true)}>
            Give roles in Zitadel
          </Button>
        }
      />

      <div className="row-divider px-5 py-3 text-[13.5px] leading-[1.55] text-muted">
        Everything below is what Zitadel holds for this person.{" "}
        <ArrowLink href={`/users/${userId}`}>Syndra&rsquo;s explanation of the same access</ArrowLink>
      </div>

      <ListStates
        isLoading={grants.isLoading}
        error={grants.error}
        isEmpty={rows.length === 0}
        onRetry={() => grants.refetch()}
        errorTitle="Couldn't read this person's roles from Zitadel."
        skeleton={<RowSkeleton rows={3} avatar={false} label="Reading roles" />}
        empty={<EmptyState title="No roles in Zitadel." />}
      >
        {rows.map((grant) => {
          const ref: GrantRef = { grantId: grant.id, roleKeys: grant.roleKeys, projectId: grant.projectId };
          return (
            <div
              key={grant.id}
              className="row-divider flex min-h-[60px] flex-col items-start gap-1.5 px-5 py-3 tablet:flex-row tablet:flex-wrap tablet:items-center tablet:gap-4"
            >
              <span className="min-w-[200px] flex-1 text-[14.5px] [overflow-wrap:anywhere]">
                <ProjectName id={grant.projectId} /> <RoleKeys keys={grant.roleKeys} />
              </span>
              <span className="basis-full tablet:order-last">
                <TechnicalDetail placeKey={`grant:${grant.id}`}>
                  <CopyableValue value={grant.id} label="Grant id" />
                </TechnicalDetail>
              </span>
              <span className="flex gap-2">
                <Button size="sm" onClick={() => setEditing(ref)}>
                  Change roles
                </Button>
                <Button size="sm" variant="danger" onClick={() => setRevoking(ref)}>
                  Revoke roles
                </Button>
              </span>
            </div>
          );
        })}
      </ListStates>

      {revoking && <RevokeDialog userId={userId} name={name} grant={revoking} onClose={() => setRevoking(null)} />}
      {assigning && <AssignDialog userId={userId} onClose={() => setAssigning(false)} />}
      {editing && <EditRolesDialog userId={userId} grant={editing} onClose={() => setEditing(null)} />}
    </Card>
  );
}

function RevokeDialog({
  userId,
  name,
  grant,
  onClose,
}: {
  userId: string;
  name: string;
  grant: GrantRef;
  onClose: () => void;
}) {
  const remove = useUpstreamRemoveGrant();
  const [outcome, setOutcome] = useState<Outcome | null>(null);
  const [acknowledged, setAcknowledged] = useState(false);
  const done = outcome?.kind === "applied";

  return (
    <Modal open onClose={onClose} busy={remove.isPending} size="sm" labelledBy="revoke-grant-title">
      <ModalHeader
        title={`Revoke ${grant.roleKeys.length === 1 ? "this role" : "these roles"} from ${name} in Zitadel?`}
        titleId="revoke-grant-title"
        lede={
          done
            ? undefined
            : "Revoking ends their access. It happens the moment you press the button, with no preview, and Syndra keeps no record of why."
        }
      />
      {!done && (
        <div className="flex flex-col gap-3.5 px-6">
          <div className="text-[14.5px]">
            <ProjectName id={grant.projectId} /> <RoleKeys keys={grant.roleKeys} />
          </div>
          <AcknowledgeCount
            checked={acknowledged}
            onChange={setAcknowledged}
            count={grant.roleKeys.length}
            noun="roles"
            verb="revokes"
            consequence={`${name} loses these roles once the change reaches Zitadel. Revocations send on their own, every few minutes. Syndra records the revocation as its own, so it is explained afterwards.`}
          />
        </div>
      )}
      {outcome && <WriteResult outcome={outcome} word="Revoked" />}

      <WriteFooter outcome={outcome} onClose={onClose}>
        <Button
          variant="dangerConfirm"
          disabled={!acknowledged}
          reason={!acknowledged ? "Check the box above to confirm you understand the consequence." : undefined}
          isPending={remove.isPending}
          onClick={async () => {
            try {
              await remove.mutateAsync({ userId, grantId: grant.grantId });
              setOutcome({ kind: "applied", message: "Roles revoked in Zitadel", detail: AUDIT_ONLY });
            } catch (error) {
              setOutcome(outcomeFromError(error));
            }
          }}
        >
          {grant.roleKeys.length === 1
            ? "Revoke this role in Zitadel"
            : `Revoke these ${grant.roleKeys.length} roles in Zitadel`}
        </Button>
      </WriteFooter>
    </Modal>
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
  const done = outcome?.kind === "applied";

  const parsed = parseKeys(keys);
  const available = sortNames(roles.data?.items.map((role) => role.key) ?? []);

  return (
    <Modal open onClose={onClose} busy={assign.isPending} size="sm">
      <ModalHeader
        title="Give roles in Zitadel"
        lede={done ? undefined : "This gives the roles in Zitadel directly, skipping Syndra."}
      />
      {!done && (
        <div className="flex flex-col gap-3.5 px-6">
          <div>
            <FieldLabel htmlFor="assign-project">Project</FieldLabel>
            <Select
              id="assign-project"
              value={projectId}
              onChange={(event) => setProjectId(event.target.value)}
            >
              <option value="">Choose…</option>
              {sortByName(projects.data?.items ?? [], (p) => p.name).map((project) => (
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
            {available.length > 0 && (
              <p className="mt-1.5 text-[12.5px] text-faint">Available: {available.join(", ")}</p>
            )}
          </div>
          <DirectWriteWarning
            traced
            what="Syndra records this grant as its own, so it is explained afterwards rather than surfacing under Drift."
            acknowledged={acknowledged}
            onAcknowledge={setAcknowledged}
          />
        </div>
      )}
      {outcome && <WriteResult outcome={outcome} word="Given" />}

      <WriteFooter outcome={outcome} onClose={onClose}>
        <Button
          // Granting, not revoking: accent, so it cannot be mistaken for "Revoke roles".
          variant="accent"
          disabled={!projectId || parsed.length === 0 || !acknowledged}
          reason={
            !projectId
              ? "Choose a project first."
              : parsed.length === 0
                ? "Enter at least one role key."
                : !acknowledged
                  ? "Check the box above to confirm you understand this skips Syndra."
                  : undefined
          }
          isPending={assign.isPending}
          onClick={async () => {
            try {
              await assign.mutateAsync({ userId, projectId, roleKeys: parsed });
              setOutcome({ kind: "applied", message: "Roles given in Zitadel", detail: AUDIT_ONLY });
            } catch (error) {
              setOutcome(outcomeFromError(error));
            }
          }}
        >
          Give roles in Zitadel
        </Button>
      </WriteFooter>
    </Modal>
  );
}

function EditRolesDialog({
  userId,
  grant,
  onClose,
}: {
  userId: string;
  grant: GrantRef;
  onClose: () => void;
}) {
  const update = useUpstreamUpdateGrant();
  const [outcome, setOutcome] = useState<Outcome | null>(null);
  const [keys, setKeys] = useState(grant.roleKeys.join(", "));
  const [acknowledged, setAcknowledged] = useState(false);
  const done = outcome?.kind === "applied";

  const parsed = parseKeys(keys);

  return (
    <Modal open onClose={onClose} busy={update.isPending} size="sm">
      <ModalHeader
        title="Replace this person's roles in Zitadel"
        lede={done ? undefined : "The new set replaces the old one entirely. Any role you leave out is revoked."}
      />
      {!done && (
        <div className="flex flex-col gap-3.5 px-6">
          <div>
            <FieldLabel htmlFor="edit-roles">Role keys</FieldLabel>
            <Input id="edit-roles" value={keys} onChange={(event) => setKeys(event.target.value)} />
            <FieldHint>The short name other systems see, e.g. trained, comma separated.</FieldHint>
          </div>
          <DirectWriteWarning
            traced
            what="Replacing the roles revokes any this person holds that you leave out."
            acknowledged={acknowledged}
            onAcknowledge={setAcknowledged}
          />
        </div>
      )}
      {outcome && <WriteResult outcome={outcome} word="Replaced" />}

      <WriteFooter outcome={outcome} onClose={onClose}>
        <Button
          variant="dangerConfirm"
          disabled={parsed.length === 0 || !acknowledged}
          reason={
            parsed.length === 0
              ? "Enter at least one role key."
              : !acknowledged
                ? "Check the box above to confirm any role left out is revoked."
                : undefined
          }
          isPending={update.isPending}
          onClick={async () => {
            try {
              await update.mutateAsync({ userId, grantId: grant.grantId, roleKeys: parsed });
              setOutcome({ kind: "applied", message: "Roles replaced in Zitadel", detail: AUDIT_ONLY });
            } catch (error) {
              setOutcome(outcomeFromError(error));
            }
          }}
        >
          Replace roles in Zitadel
        </Button>
      </WriteFooter>
    </Modal>
  );
}
