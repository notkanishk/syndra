"use client";

import { useSearchParams } from "next/navigation";
import { useMemo, useState } from "react";

import { RolePicker } from "@/components/bundles/RolePicker";
import { projectLabel } from "@/components/names/Unresolved";
import { UserName } from "@/components/names/UserName";
import { EmptyState, ListStates, RowSkeleton } from "@/components/states";
import { ActionOutcome } from "@/components/ui/ActionOutcome";
import { Mono } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Card, CardColumns, CardHeader } from "@/components/ui/Card";
import { Seat } from "@/components/ui/Choice";
import { Modal, ModalFooter, ModalHeader } from "@/components/ui/Modal";
import { NextStep } from "@/components/ui/NextStep";
import { Notice } from "@/components/ui/Notice";
import { PageHeader } from "@/components/ui/PageHeader";
import { formatShortDate, humanizeKey } from "@/lib/format";
import { holdersLine } from "@/lib/holders";
import { describeFailure, sentence } from "@/lib/outcome";
import { usePendingPropagations } from "@/lib/queries/usePropagation";
import { useGlobalRoleCatalog, type CatalogRole } from "@/lib/queries/useRoles";
import {
  useRolesThatWait,
  useSetRolesWait,
  useSwitchToRolesThatWait,
  type WaitSuggestion,
} from "@/lib/queries/useRolesThatWait";

const roleId = (projectId: string, roleKey: string) =>
  `${projectId}:${roleKey}`;

/**
 * Roles that wait for you (N14 option B, N16): the one write place for which
 * roles wait in Pending changes. When someone would gain or lose one, however
 * it happens, the change waits until the operator sends it; every other role
 * goes straight to Zitadel. Everywhere else shows the mark, read-only, with a
 * link here.
 *
 * Before the switch over, the page is the one-time move: the roles that a
 * bundle or rule set to wait used to give, each with why, to keep waiting.
 */
export default function RolesThatWaitPage() {
  const waits = useRolesThatWait();
  const catalog = useGlobalRoleCatalog();
  // ?add=project:role, from a new role's "Make it wait for you →": the
  // picker opens with it chosen.
  const params = useSearchParams();
  const asked = params.get("add");
  const [adding, setAdding] = useState(Boolean(asked));
  const [landed, setLanded] = useState<ReadonlySet<string>>(new Set());

  const byId = useMemo(() => {
    const m = new Map<string, CatalogRole>();
    for (const r of catalog.data ?? [])
      m.set(roleId(r.project_id, r.role_key), r);
    return m;
  }, [catalog.data]);

  const switched = waits.data?.switched_over ?? false;

  return (
    <div className="flex flex-col gap-7">
      <PageHeader
        eyebrow="Changes"
        title="Roles that wait for you"
        lede="When someone would gain or lose one of these roles, however it happens, the change waits in Pending changes until you send it. Every other role goes straight to Zitadel. For the whole makerspace."
        actions={
          switched ? (
            <Button variant="accent" onClick={() => setAdding(true)}>
              Add roles
            </Button>
          ) : undefined
        }
      />

      {waits.data && !switched ? (
        <OneTimeMove suggestions={waits.data.suggestions} byId={byId} />
      ) : (
        <WaitingRoles
          isLoading={waits.isLoading}
          error={waits.error}
          onRetry={() => waits.refetch()}
          marked={waits.data?.roles ?? []}
          byId={byId}
          landed={landed}
          onAdd={() => setAdding(true)}
        />
      )}

      {adding && (
        <AddRoles
          initial={asked}
          marked={
            new Set(
              (waits.data?.roles ?? []).map((r) =>
                roleId(r.project_id, r.role_key),
              ),
            )
          }
          byId={byId}
          onClose={() => setAdding(false)}
          onMarked={(ids) => {
            setAdding(false);
            setLanded(new Set(ids));
          }}
        />
      )}
    </div>
  );
}

function roleName(r: CatalogRole | undefined, roleKey: string) {
  return r?.display_name || humanizeKey(roleKey);
}

function peopleOf(r: CatalogRole | undefined) {
  return r
    ? holdersLine(
        r.assigned_user_count,
        r.confirmed_user_count,
        r.observed_user_count,
      ).headline
    : "—";
}

/** The roles that wait, gathered by project, each with its verb. */
function WaitingRoles({
  isLoading,
  error,
  onRetry,
  marked,
  byId,
  landed,
  onAdd,
}: {
  isLoading: boolean;
  error: unknown;
  onRetry: () => void;
  marked: {
    project_id: string;
    role_key: string;
    marked_by: string;
    marked_at: string;
  }[];
  byId: Map<string, CatalogRole>;
  landed: ReadonlySet<string>;
  onAdd: () => void;
}) {
  const stop = useSetRolesWait();
  const [failure, setFailure] = useState<string | null>(null);
  const [stopped, setStopped] = useState<string | null>(null);
  const pending = usePendingPropagations();
  const waitingChanges = (pending.data ?? []).filter(
    (r) => r.wait_reason === "role_waits",
  ).length;

  const runs = useMemo(() => {
    // By project id, labelled by name: two projects nobody could name are two
    // groups, and an id is never the label.
    const out = new Map<string, { project: string; rows: typeof marked }>();
    for (const m of marked) {
      const project = projectLabel(byId.get(roleId(m.project_id, m.role_key))?.project_name, m.project_id);
      const run = out.get(m.project_id) ?? { project, rows: [] };
      run.rows.push(m);
      out.set(m.project_id, run);
    }
    return Array.from(out.entries()).sort(([, a], [, b]) => a.project.localeCompare(b.project));
  }, [marked, byId]);

  return (
    <Card>
      <CardColumns>
        <span className="flex-1">Role ↑</span>
        <span className="hidden w-[200px] tablet:block">Marked</span>
        <span className="w-[110px] text-right">People</span>
        <span className="w-[140px]" aria-hidden />
      </CardColumns>

      <ListStates
        isLoading={isLoading}
        error={error}
        isEmpty={marked.length === 0}
        onRetry={onRetry}
        errorTitle="Couldn't load the roles that wait."
        skeleton={
          <RowSkeleton
            rows={3}
            avatar={false}
            label="Loading roles that wait"
          />
        }
        empty={
          <EmptyState
            title="No role waits for you."
            guidance="Every change goes straight to Zitadel. Add the roles that deserve a look every time someone gains or loses them."
            action={{ label: "Add roles", onClick: onAdd }}
          />
        }
      >
        {runs.map(([projectId, { project, rows }]) => (
          <div key={projectId}>
            <div className="row-divider px-5 pb-1.5 pt-3.5 text-[12.5px] font-semibold text-faint">
              {project} · {rows.length}
            </div>
            {rows.map((m) => {
              const id = roleId(m.project_id, m.role_key);
              const role = byId.get(id);
              return (
                <div
                  key={id}
                  className={`row-divider flex flex-wrap items-center gap-x-[18px] gap-y-2 px-5 py-3.5 ${
                    landed.has(id) ? "flash" : ""
                  }`}
                >
                  <div className="min-w-[min(220px,100%)] flex-1">
                    <span className="text-[15px] font-semibold">
                      {roleName(role, m.role_key)}
                    </span>{" "}
                    <Mono className="font-normal text-faint">{m.role_key}</Mono>
                  </div>
                  <span className="hidden w-[200px] text-[13.5px] text-muted tablet:block">
                    <UserName id={m.marked_by} fallback="Someone" />,{" "}
                    {formatShortDate(m.marked_at)}
                  </span>
                  <span className="w-[110px] text-right text-[15px] font-semibold tabular-nums">
                    {peopleOf(role)}
                  </span>
                  <span className="flex w-[140px] justify-end whitespace-nowrap">
                    <Button
                      size="sm"
                      disabled={stop.isPending}
                      onClick={async () => {
                        setFailure(null);
                        try {
                          await stop.mutateAsync({ ids: [id], waits: false });
                          setStopped(roleName(role, m.role_key));
                        } catch (e) {
                          setFailure(sentence(describeFailure(e)));
                        }
                      }}
                    >
                      Stop waiting
                    </Button>
                  </span>
                </div>
              );
            })}
          </div>
        ))}
        {waitingChanges > 0 && (
          <div className="row-divider flex flex-wrap items-center gap-3 px-5 py-3.5">
            <NextStep />
            <span className="text-[13px] text-faint">
              {waitingChanges} {waitingChanges === 1 ? "change" : "changes"} to
              these roles
            </span>
          </div>
        )}
      </ListStates>
        {stopped && (
          <div className="row-divider px-5 py-3">
            <ActionOutcome
              outcome={{
                kind: "applied",
                message: `${stopped} goes straight to Zitadel now`,
                detail: "Changes already waiting stay in Pending changes.",
              }}
            />
          </div>
        )}
        {failure && (
          <div className="row-divider px-5 py-3">
            <Notice tone="danger" title="That role still waits.">
              {failure} Nothing was changed.
            </Notice>
          </div>
        )}
    </Card>
  );
}

/**
 * The one-time move, the first visit after the update: every role something
 * set to wait used to give, with why, chosen to keep waiting. Nothing is
 * sent while choosing; changes already waiting stay waiting.
 */
function OneTimeMove({
  suggestions,
  byId,
}: {
  suggestions: WaitSuggestion[];
  byId: Map<string, CatalogRole>;
}) {
  const ids = suggestions.map((s) => roleId(s.project_id, s.role_key));
  // All of them, to start: keeping what waited waiting is the choice that
  // changes nothing, and the operator unticks what doesn't deserve it.
  const [chosen, setChosen] = useState<ReadonlySet<string>>(() => new Set(ids));
  const mark = useSetRolesWait();
  const finish = useSwitchToRolesThatWait();
  const [failure, setFailure] = useState<string | null>(null);
  const busy = mark.isPending || finish.isPending;

  const bundles = new Set(
    suggestions.flatMap((s) =>
      s.via.filter((v) => v.kind === "bundle").map((v) => v.id),
    ),
  ).size;
  const rules = new Set(
    suggestions.flatMap((s) =>
      s.via.filter((v) => v.kind === "rule").map((v) => v.id),
    ),
  ).size;
  const n = chosen.size;

  async function done(marking: string[]) {
    setFailure(null);
    try {
      await mark.mutateAsync({ ids: marking, waits: true });
      await finish.mutateAsync();
    } catch (e) {
      setFailure(sentence(describeFailure(e)));
    }
  }

  return (
    <Card>
      <CardHeader
        title="Bundles and rules no longer have their own modes"
        note="Once, after the update"
      />
      <p className="row-divider max-w-[86ch] px-5 py-4 text-[14.5px] leading-[1.55] text-muted [text-wrap:pretty]">
        {bundles + rules > 0 ? (
          <>
            Until today, {plural(bundles, "bundle")}
            {rules ? ` and ${plural(rules, "rule")}` : ""} waited for you and
            the rest went straight through. From now on, the role decides. These
            roles were given by something that waited. Choose the ones that
            should keep waiting.
          </>
        ) : (
          <>
            Until today, a bundle or rule could wait for you as a whole. From
            now on, the role decides. Nothing was set to wait, so there is
            nothing to carry over; add a role here whenever one deserves a look.
          </>
        )}
      </p>

      {suggestions.map((s) => {
        const id = roleId(s.project_id, s.role_key);
        const role = byId.get(id);
        const on = chosen.has(id);
        return (
          <label
            key={id}
            className={`row-divider flex cursor-pointer items-center gap-[18px] px-5 py-3.5 motion-tint ${
              on ? "bg-accent-soft" : "hover:bg-[var(--hover)]"
            }`}
          >
            <Seat
              checked={on}
              disabled={busy}
              onChange={() =>
                setChosen((prev) => {
                  const next = new Set(prev);
                  if (!next.delete(id)) next.add(id);
                  return next;
                })
              }
              aria-label={roleName(role, s.role_key)}
            />
            <span className="min-w-0 flex-1">
              <span className="text-[15px] font-semibold">
                {roleName(role, s.role_key)}
              </span>{" "}
              <Mono className="font-normal text-faint">{s.role_key}</Mono>
              <span className="block text-[13px] text-muted">{givenBy(s)}</span>
            </span>
            <span className="w-[110px] text-right text-[15px] font-semibold tabular-nums">
              {peopleOf(role)}
            </span>
          </label>
        );
      })}

      {failure && (
        <div className="px-5 pt-4">
          <Notice tone="danger" title="That didn't finish.">
            {failure} The roles marked so far stay marked; press it again to
            finish.
          </Notice>
        </div>
      )}

      <div className="flex flex-wrap items-center gap-3 px-5 py-4">
        <Button
          variant="accent"
          isPending={busy}
          disabled={busy}
          onClick={() => done(Array.from(chosen))}
        >
          {n > 0 ? `Mark ${n} and finish` : "Finish with none marked"}
        </Button>
        {n > 0 && (
          <Button disabled={busy} onClick={() => done([])}>
            Mark none
          </Button>
        )}
        <span className="text-[12.5px] text-faint">
          Nothing is sent while you choose. Changes already waiting stay
          waiting.
        </span>
      </div>
    </Card>
  );
}

function plural(n: number, noun: string) {
  return `${n} ${noun}${n === 1 ? "" : "s"}`;
}

/** "Given by Staff, which waited" · "Given by the rule from Laser cutter · Operator, which waited". */
function givenBy(s: WaitSuggestion) {
  const names = s.via.map((v) =>
    v.kind === "bundle" ? v.name || "a bundle" : "a rule",
  );
  const list =
    names.length <= 2
      ? names.join(" and ")
      : `${names.slice(0, -1).join(", ")} and ${names.at(-1)}`;
  return `Given by ${list}, which waited`;
}

/** The role picker, marking roles to wait. The footer note says what marking means first. */
function AddRoles({
  initial,
  marked,
  byId,
  onClose,
  onMarked,
}: {
  initial?: string | null;
  marked: ReadonlySet<string>;
  byId: Map<string, CatalogRole>;
  onClose: () => void;
  onMarked: (ids: string[]) => void;
}) {
  const [selected, setSelected] = useState<ReadonlySet<string>>(
    () => new Set(initial && !marked.has(initial) ? [initial] : []),
  );
  const mark = useSetRolesWait();
  const [failure, setFailure] = useState<string | null>(null);
  const chosen = Array.from(selected);
  const names = chosen.map((id) =>
    roleName(byId.get(id), id.slice(id.indexOf(":") + 1)),
  );
  const people = chosen.reduce((sum, id) => {
    const r = byId.get(id);
    return sum + (r ? (r.observed_user_count ?? r.assigned_user_count) : 0);
  }, 0);

  return (
    <Modal
      open
      onClose={onClose}
      busy={mark.isPending}
      size="md"
      labelledBy="add-waits-title"
    >
      <ModalHeader
        title="Add roles that wait"
        titleId="add-waits-title"
        lede="Choose the roles that should always wait for you."
      />
      <div className="px-6">
        <RolePicker
          selected={selected}
          onToggle={(id) =>
            setSelected((prev) => {
              const next = new Set(prev);
              if (!next.delete(id)) next.add(id);
              return next;
            })
          }
          lockedIds={marked}
          lockedNote="On your list"
          disabled={mark.isPending}
          legend={false}
        />
      </div>
      {failure && (
        <div className="mx-6 mt-3">
          <Notice tone="danger" title="Not all of them were marked.">
            {failure} The roles still ticked weren&apos;t marked; press it again
            for the rest.
          </Notice>
        </div>
      )}
      <ModalFooter
        note={
          chosen.length
            ? `From now on, ${listOf(names)} ${chosen.length > 1 ? "wait" : "waits"} for you. ${people} ${
                people === 1 ? "person has" : "people have"
              } ${chosen.length > 1 ? "them" : "it"} today, and nothing changes for them now.`
            : "Choose the roles that should always wait for you."
        }
      >
        <Button
          variant="accent"
          disabled={!chosen.length || mark.isPending}
          isPending={mark.isPending}
          onClick={async () => {
            setFailure(null);
            try {
              await mark.mutateAsync({ ids: chosen, waits: true });
              onMarked(chosen);
            } catch (e) {
              setFailure(sentence(describeFailure(e)));
            }
          }}
        >
          {chosen.length
            ? `Mark ${chosen.length} ${chosen.length > 1 ? "roles" : "role"}`
            : "Mark roles"}
        </Button>
        <Button variant="ghost" onClick={onClose}>
          Cancel
        </Button>
      </ModalFooter>
    </Modal>
  );
}

function listOf(names: string[]) {
  return names.length <= 2
    ? names.join(" and ")
    : `${names.slice(0, -1).join(", ")} and ${names.at(-1)}`;
}
