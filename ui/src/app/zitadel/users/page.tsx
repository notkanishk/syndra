"use client";

import { ArrowLink } from "@/components/ui/ArrowLink";
import { useMemo, useState } from "react";

import { EmptyState, ListStates, RowSkeleton } from "@/components/states";
import {
  DirectWriteWarning,
  UpstreamShell,
  WriteFooter,
  WriteResult,
  sendsWhen,
} from "@/components/upstream/UpstreamShell";
import { AcknowledgeCount } from "@/components/ui/Acknowledge";
import { Mono } from "@/components/ui/Badge";
import { CopyableValue } from "@/components/ui/CopyableValue";
import { TechnicalDetail } from "@/components/ui/TechnicalDetail";
import { Button } from "@/components/ui/Button";
import { DRAIN_UNKNOWN_NEXT, outcomeFromError, succeeded, type ActionOutcome as Outcome, type AutoRetry } from "@/lib/outcome";
import { RETRYING, refusedNext, retryLine, retryNextStep } from "@/lib/retry";
import { refusalCause } from "@/lib/wait-reason";
import { Card, CardHeader } from "@/components/ui/Card";
import { FilterBar } from "@/components/ui/FilterBar";
import { FieldHint, FieldLabel, Input } from "@/components/ui/Input";
import { Modal, ModalHeader } from "@/components/ui/Modal";
import { PaginationRow, usePaging } from "@/components/ui/PaginationRow";
import { Select } from "@/components/ui/Select";
import { ProjectName } from "@/components/names";
import {
  useGrantRemovalPreview,
  useUpstreamAssignGrant,
  useUpstreamProjectRoles,
  useUpstreamProjects,
  useUpstreamRemoveGrant,
  useUpstreamUpdateGrant,
  useUpstreamUserGrants,
  useUpstreamUsers,
  type ZitadelGrantChange,
  type ZitadelRoleChange,
} from "@/lib/queries/useUpstream";
import { useAutoRetry } from "@/lib/queries/useIndicators";
import { useRolesThatWait } from "@/lib/queries/useRolesThatWait";
import { useRulePreview } from "@/lib/queries/useRulePreview";
import { useDebounce } from "@/lib/useDebounce";
import { formatList, humanizeKey } from "@/lib/format";
import { sortByName, sortNames } from "@/lib/sort";

/** "USER_STATE_ACTIVE" → "Active": the API's word, in ours. */
function userStateLabel(state: string): string {
  return humanizeKey(state.replace(/^USER_STATE_/, "").toLowerCase());
}

/** "project/role" → "role". */
const roleOf = (role: string) => role.slice(role.indexOf("/") + 1);

type Act = "given" | "revoked" | "replaced";
const ACT_WORD: Record<Act, string> = { given: "Given", revoked: "Revoked", replaced: "Replaced" };

const sendsAny = (roles: ZitadelRoleChange[], pick: (r: ZitadelRoleChange) => boolean) => roles.some(pick);

/**
 * What a change did, role by role, in the backend's words (W8): never "done"
 * for a role that waits, that Zitadel didn't take, or that was kept.
 */
function changeResult(res: ZitadelGrantChange | undefined, act: Act, auto: AutoRetry = true): { outcome: Outcome; word: string } {
  const roles = res?.roles ?? [];
  const recorded = "Recorded in Syndra as a change on their page, so it is explained afterwards.";
  if (roles.length === 0) {
    if (res?.status === "unchanged") {
      return { outcome: { kind: "no_change", message: "The new list is the same as the old one", detail: recorded }, word: "No change" };
    }
    return { outcome: { kind: "applied", message: `Roles ${act} in Zitadel`, detail: recorded }, word: ACT_WORD[act] };
  }
  const lines: string[] = [];
  const say = (lead: string, pick: (r: ZitadelRoleChange) => boolean) => {
    const names = sortNames(roles.filter(pick).map((r) => roleOf(r.role)));
    if (names.length) lines.push(`${lead}: ${formatList(names)}.`);
  };
  say("Given in Zitadel", (r) => r.change === "given" && r.status === "applied");
  say("Revoked in Zitadel", (r) => r.change === "revoked" && r.status === "applied");
  // Re-audit R3: a give Zitadel didn't take is retried on its own (the
  // server says when); only one that isn't goes when you send it.
  const retrying = res?.retry?.state === "retrying";
  // Neither "on its own" nor "when you send it" is known (N3): the indicators
  // aren't read yet, or the server couldn't read the next try.
  const unsure = auto === null || (res?.wait_reason === RETRYING && !res?.retry);
  const goesItself = (r: ZitadelRoleChange) => auto === true && !unsure && r.status === "pending" && (r.change === "revoked" || retrying);
  say("Waiting for you to send, because the role always waits for you", (r) => r.status === "waiting");
  if (unsure) {
    say("Not in Zitadel yet", (r) => r.status === "pending");
  } else {
    say("Not in Zitadel yet, because Zitadel didn't answer. Syndra sends it again on its own", (r) => r.change === "given" && r.status === "pending" && retrying);
    say("Not in Zitadel yet. It goes when you send it", (r) => r.change === "given" && r.status === "pending" && !retrying);
    say(
      auto
        ? "Not revoked in Zitadel yet, because Zitadel didn't take it. Revocations send on their own"
        : "Not revoked in Zitadel yet, because Zitadel didn't take it. It waits in Pending changes until you send it",
      (r) => r.change === "revoked" && r.status === "pending",
    );
  }
  if (res?.retry && sendsAny(roles, goesItself)) lines.push(retryLine(res.retry));
  // Final verification N1: each refusal with its plain cause.
  const refusals = roles.filter((r) => r.status === "failed");
  for (const reason of new Set(refusals.map((r) => r.reason ?? "refused:other"))) {
    const names = sortNames(refusals.filter((r) => (r.reason ?? "refused:other") === reason).map((r) => roleOf(r.role)));
    lines.push(`Zitadel refused this for ${formatList(names)}: ${refusalCause(reason)}.`);
  }
  say("Kept, because a bundle or a rule still gives it", (r) => r.change === "kept");
  const detail = [...lines, recorded].join(" ");

  const sent = roles.filter((r) => r.change !== "kept");
  if (sent.length === 0) {
    return { outcome: { kind: "no_change", message: "Nothing was revoked. A bundle or a rule still gives every role", detail }, word: "Nothing revoked" };
  }
  const refused = refusals.length > 0;
  // The step a refused role leaves: terminal, so Send won't move it.
  const refusedStep = refusedNext(refusals.find((r) => r.reason)?.reason, { again: "give or revoke" });
  const open = sent.filter((r) => r.status === "waiting" || r.status === "pending");
  if (open.length) {
    const itself = open.every(goesItself);
    const pendingUnsure = unsure && open.some((r) => r.status === "pending");
    return {
      outcome: {
        kind: "queued",
        message: "Recorded in Syndra. Not all of it is in Zitadel yet",
        detail,
        sendsItself: itself || pendingUnsure || undefined,
        // What Syndra sends itself says when; a refused part needs a person.
        next: pendingUnsure
          ? `${DRAIN_UNKNOWN_NEXT}${refused ? ` ${refusedStep}` : ""}`
          : refused
            ? `${itself ? "" : "Press Send on Pending changes for what waits. "}${refusedStep}`
            : itself && res?.retry
              ? retryNextStep(res.retry)
              : undefined,
      },
      word: ACT_WORD[act],
    };
  }
  if (refused) {
    // Re-audit R4: a refused row is terminal and Syndra's record now differs
    // from Zitadel, so there is something to do. N1: said as a refusal.
    const all = refusals.length === sent.length;
    return {
      outcome: {
        kind: "failed",
        recorded: true,
        message: `Recorded in Syndra. Zitadel refused this${all ? "" : " for some of the roles"}`,
        detail,
        next: refusedStep,
      },
      word: all ? "Refused by Zitadel" : "Not all in Zitadel",
    };
  }
  return { outcome: { kind: "applied", message: `Roles ${act} in Zitadel`, detail }, word: ACT_WORD[act] };
}

/** The roles among `keys` that wait for you; undefined until the list is read. */
function useWaitingAmong(projectId: string, keys: string[]): string[] | undefined {
  const waits = useRolesThatWait().data?.roles;
  if (!waits || !projectId) return undefined;
  return keys.filter((key) => waits.some((w) => w.project_id === projectId && w.role_key === key));
}

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
  const auto = useAutoRetry();
  const remove = useUpstreamRemoveGrant();
  const [result, setResult] = useState<{ outcome: Outcome; word: string } | null>(null);
  const outcome = result?.outcome ?? null;
  const [acknowledged, setAcknowledged] = useState(false);
  const done = succeeded(outcome);
  // C4, re-audit R18: the count is the roles that go, by the server's own
  // removal rule (a preview of the very walk the revoke runs), including a
  // role a rule gave from one that goes. Never worked out here; unread, the
  // revoke waits for it.
  const preview = useGrantRemovalPreview(userId, grant.grantId);
  const reading = preview.isLoading;
  const unread = !reading && !preview.data;
  const goingPairs = preview.data?.revoked_roles ?? [];
  const going = goingPairs.map(roleOf);
  const kept = preview.data ? (preview.data.retained_roles ?? []).map(roleOf) : undefined;
  const waiting = useWaitingAmong(
    grant.projectId,
    goingPairs.filter((pair) => pair.startsWith(`${grant.projectId}/`)).map(roleOf),
  );
  const nothingGoes = Boolean(preview.data) && going.length === 0;

  return (
    <Modal open onClose={onClose} busy={remove.isPending} size="sm" labelledBy="revoke-grant-title">
      <ModalHeader
        title={`Revoke ${grant.roleKeys.length === 1 ? "this role" : "these roles"} from ${name} in Zitadel?`}
        titleId="revoke-grant-title"
        lede={
          done
            ? undefined
            : "Revoked as a person’s page revokes. A role a bundle or a rule still gives them stays. Every other role on this grant is revoked, including any Syndra has no record of."
        }
      />
      {!done && (
        <div className="flex flex-col gap-3.5 px-6">
          <div className="text-[14.5px]">
            <ProjectName id={grant.projectId} /> <RoleKeys keys={grant.roleKeys} />
          </div>
          {kept && kept.length > 0 && (
            <p className="text-[13.5px] leading-[1.55] text-muted">
              Stays, because a bundle or a rule still gives it: {formatList(sortNames(kept))}.
            </p>
          )}
          {nothingGoes ? (
            <p className="text-[13.5px] leading-[1.55] text-muted">
              Nothing on this grant would be revoked: a bundle or a rule gives {name} every role on it.
            </p>
          ) : reading || unread ? (
            <p className="text-[13.5px] leading-[1.55] text-muted">
              {reading
                ? "Reading what this would revoke…"
                : "Syndra couldn't read what this would revoke, so it won't ask you to confirm a count. Try again in a minute."}
            </p>
          ) : (
            <AcknowledgeCount
              checked={acknowledged}
              onChange={setAcknowledged}
              count={going.length}
              noun="roles"
              verb="revokes"
              consequence={`${name} loses ${going.length === 1 ? "it" : "them"} once the change reaches Zitadel. ${sendsWhen(waiting)} Syndra records it as its own, so it is explained afterwards.`}
            />
          )}
        </div>
      )}
      {result && <WriteResult outcome={result.outcome} word={result.word} neutral={result.word} />}

      <WriteFooter outcome={outcome} onClose={onClose}>
        <Button
          variant="dangerConfirm"
          disabled={reading || unread || nothingGoes || !acknowledged}
          reason={
            reading
              ? "Reading what this would revoke first."
              : unread
                ? "Syndra couldn't read what this would revoke."
                : nothingGoes
                ? "Nothing here would be revoked."
                : !acknowledged
                  ? "Check the box above to confirm you understand the consequence."
                  : undefined
          }
          isPending={remove.isPending}
          onClick={async () => {
            try {
              const res = await remove.mutateAsync({ userId, grantId: grant.grantId });
              setResult(changeResult(res, "revoked", auto));
            } catch (error) {
              setResult({ outcome: outcomeFromError(error), word: "" });
            }
          }}
        >
          {going.length === 1 ? "Revoke this role in Zitadel" : `Revoke these ${going.length} roles in Zitadel`}
        </Button>
      </WriteFooter>
    </Modal>
  );
}

function AssignDialog({ userId, onClose }: { userId: string; onClose: () => void }) {
  const auto = useAutoRetry();
  const projects = useUpstreamProjects();
  const [projectId, setProjectId] = useState("");
  const roles = useUpstreamProjectRoles(projectId || null);
  const [keys, setKeys] = useState("");
  const assign = useUpstreamAssignGrant();
  const [result, setResult] = useState<{ outcome: Outcome; word: string } | null>(null);
  const outcome = result?.outcome ?? null;
  const [acknowledged, setAcknowledged] = useState(false);
  const done = succeeded(outcome);

  const parsed = parseKeys(keys);
  const available = sortNames(roles.data?.items.map((role) => role.key) ?? []);
  const waiting = useWaitingAmong(projectId, parsed);
  // What the automatic rules also give with these roles, said before the
  // click, from the endpoint the person page's give reads (audit C2): this
  // give goes through the same write (giveRoles → grantDirect), so the same
  // rules follow it. Only for keys that exist, so typing doesn't ask.
  const rules = useRulePreview(userId, projectId, parsed.filter((key) => available.includes(key)));
  const ruleLines = (rules.data ?? []).map((row) => row.sentence);

  return (
    <Modal open onClose={onClose} busy={assign.isPending} size="sm">
      <ModalHeader
        title="Give roles in Zitadel"
        lede={
          done
            ? undefined
            : "Given as a person’s page gives them: each role must still exist in Zitadel, and they must not already have it."
        }
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
          {(ruleLines.length > 0 || rules.isError) && (
            <div className="flex flex-col gap-1 text-[13px] leading-[1.5] text-muted" data-testid="rule-preview">
              {ruleLines.map((line) => (
                <span key={line}>{line}</span>
              ))}
              {rules.isError && (
                <span>Syndra couldn&rsquo;t read the automatic rules, so a role one of them also gives isn&rsquo;t named here.</span>
              )}
            </div>
          )}
          <DirectWriteWarning
            what="Syndra records this grant as its own, so it is explained afterwards rather than surfacing under Drift."
            sends={sendsWhen(waiting)}
            acknowledged={acknowledged}
            onAcknowledge={setAcknowledged}
          />
        </div>
      )}
      {result && <WriteResult outcome={result.outcome} word={result.word} neutral={result.word} />}

      <WriteFooter outcome={outcome} onClose={onClose}>
        <Button
          // Granting, not revoking: accent, so it cannot be mistaken for "Revoke roles".
          variant="accent"
          disabled={!projectId || parsed.length === 0 || !acknowledged || rules.isLoading}
          reason={
            !projectId
              ? "Choose a project first."
              : parsed.length === 0
                ? "Enter at least one role key."
                : rules.isLoading
                  ? "Checking what automatic rules also give."
                  : !acknowledged
                    ? "Check the box above to confirm."
                    : undefined
          }
          isPending={assign.isPending}
          onClick={async () => {
            try {
              const res = await assign.mutateAsync({ userId, projectId, roleKeys: parsed });
              setResult(changeResult(res, "given", auto));
            } catch (error) {
              setResult({ outcome: outcomeFromError(error), word: "" });
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
  const auto = useAutoRetry();
  const update = useUpstreamUpdateGrant();
  const [result, setResult] = useState<{ outcome: Outcome; word: string } | null>(null);
  const outcome = result?.outcome ?? null;
  const [keys, setKeys] = useState(grant.roleKeys.join(", "));
  const [acknowledged, setAcknowledged] = useState(false);
  const done = succeeded(outcome);

  const parsed = parseKeys(keys);
  // Both halves of a replace are delivered by the same rule: what is added and what is left out.
  const changed = [
    ...parsed.filter((key) => !grant.roleKeys.includes(key)),
    ...grant.roleKeys.filter((key) => !parsed.includes(key)),
  ];
  const waiting = useWaitingAmong(grant.projectId, changed);

  return (
    <Modal open onClose={onClose} busy={update.isPending} size="sm">
      <ModalHeader
        title="Replace this person's roles in Zitadel"
        lede={
          done
            ? undefined
            : "The new set replaces the old one. Any role you leave out is revoked, unless a bundle or a rule still gives it."
        }
      />
      {!done && (
        <div className="flex flex-col gap-3.5 px-6">
          <div>
            <FieldLabel htmlFor="edit-roles">Role keys</FieldLabel>
            <Input id="edit-roles" value={keys} onChange={(event) => setKeys(event.target.value)} />
            <FieldHint>The short name other systems see, e.g. trained, comma separated.</FieldHint>
          </div>
          <DirectWriteWarning
            what="Replacing the roles revokes any you leave out, unless a bundle or a rule still gives it."
            sends={sendsWhen(waiting)}
            acknowledged={acknowledged}
            onAcknowledge={setAcknowledged}
          />
        </div>
      )}
      {result && <WriteResult outcome={result.outcome} word={result.word} neutral={result.word} />}

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
              const res = await update.mutateAsync({ userId, grantId: grant.grantId, roleKeys: parsed });
              setResult(changeResult(res, "replaced", auto));
            } catch (error) {
              setResult({ outcome: outcomeFromError(error), word: "" });
            }
          }}
        >
          Replace roles in Zitadel
        </Button>
      </WriteFooter>
    </Modal>
  );
}
