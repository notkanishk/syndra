"use client";

import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { useMemo, useState } from "react";

import { RolePicker } from "@/components/bundles/RolePicker";
import { projectLabel } from "@/components/names/Unresolved";
import { UserName } from "@/components/names/UserName";
import { EmptyState, ListStates, RowSkeleton } from "@/components/states";
import { ActionOutcome } from "@/components/ui/ActionOutcome";
import { Mono } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Card, CardColumns } from "@/components/ui/Card";
import { Modal, ModalFooter, ModalHeader } from "@/components/ui/Modal";
import { NextStep } from "@/components/ui/NextStep";
import { RunLabel, SortHead, orderBy, runStarts, useSort } from "@/components/ui/SortHead";
import { PageHeader } from "@/components/ui/PageHeader";
import { formatShortDate } from "@/lib/format";
import { heldSortKey, holdersLine, peopleSuffix } from "@/lib/holders";
import { failureMessage, outcomeFromError, sentence, stateLine, succeeded, type ActionOutcome as Outcome } from "@/lib/outcome";
import { usePendingPropagations } from "@/lib/queries/usePropagation";
import { useGlobalRoleCatalog, type CatalogRole } from "@/lib/queries/useRoles";
import {
  useRolesThatWait,
  useSetRolesWait,
} from "@/lib/queries/useRolesThatWait";
import { compareNames } from "@/lib/sort";
import { personOr, roleName } from "@/lib/names";

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

  return (
    <div className="flex flex-col gap-4">
      <PageHeader
        title="Roles that wait"
        lede="When someone would gain or lose one of these roles, however it happens, the change waits in Pending changes until you send it. Every other role goes straight to Zitadel."
        actions={
          <Button variant="accent" onClick={() => setAdding(true)}>
            Add roles
          </Button>
        }
      />

      <WaitingRoles
        isLoading={waits.isLoading}
        error={waits.error}
        onRetry={() => waits.refetch()}
        marked={waits.data?.roles ?? []}
        byId={byId}
        landed={landed}
      />

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
          onMarked={(ids) => setLanded(new Set(ids))}
        />
      )}
    </div>
  );
}

function peopleOf(r: CatalogRole | undefined) {
  return r
    ? holdersLine(
        r.assigned_user_count,
        r.confirmed_user_count,
        r.observed_user_count,
      ).headline
    : "Not yet";
}

type WaitSort = "role" | "marked" | "people";

/** The roles that wait, gathered by project, each with its verb. */
function WaitingRoles({
  isLoading,
  error,
  onRetry,
  marked,
  byId,
  landed,
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
}) {
  const stop = useSetRolesWait();
  // One slot per card (G23): a press clears it, a later result replaces it.
  const [result, setResult] = useState<{ word: string; outcome: Outcome } | null>(null);
  const pending = usePendingPropagations();
  const waitingChanges = (pending.data ?? []).filter(
    (r) => r.wait_reason === "role_waits",
  ).length;
  // Changes waiting on each role, so a role's own row can open Pending narrowed to it.
  const waitingOn = (projectId: string, roleKey: string) =>
    (pending.data ?? []).some(
      (r) => r.wait_reason === "role_waits" && r.project_id === projectId && (r.role_keys ?? []).includes(roleKey),
    );

  // Rows gathered by project (a quiet label and its count), A to Z; the heading in force orders
  // them inside each project. Two projects nobody could name sort by id, so they never interleave.
  const [sort, setSort, view] = useSort<WaitSort>("role", { list: "roles-that-wait", keys: ["role", "marked", "people"] });
  const { sorted: rows, runAt } = useMemo(() => {
    const by: Record<WaitSort, (m: (typeof marked)[number]) => string | number> = {
      role: (m) => roleName(byId.get(roleId(m.project_id, m.role_key))?.display_name, m.role_key),
      marked: (m) => Date.parse(m.marked_at) || 0,
      people: (m) => {
        const r = byId.get(roleId(m.project_id, m.role_key));
        return heldSortKey(r?.observed_user_count);
      },
    };
    const label = (m: (typeof marked)[number]) =>
      projectLabel(byId.get(roleId(m.project_id, m.role_key))?.project_name, m.project_id);
    const sorted = orderBy(marked, view.sort, by).sort(
      (a, b) => compareNames(label(a), label(b)) || compareNames(a.project_id, b.project_id),
    );
    return { sorted, runAt: runStarts(sorted, label) };
  }, [marked, byId, view.sort]);

  return (
    <Card>
      <ListStates
        head={
          <CardColumns>
            <span className="flex-1">
              <SortHead label="Role" name="role" sort={sort} onSort={setSort} />
            </span>
            <span className="hidden w-[200px] tablet:block">
              <SortHead label="Marked" name="marked" sort={sort} onSort={setSort} />
            </span>
            <span className="w-[110px] text-right">
              <SortHead label="People" name="people" sort={sort} onSort={setSort} />
            </span>
            <span className="w-[140px]" aria-hidden />
          </CardColumns>
        }
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
          />
        }
      >
        <div className={view.className}>
          {rows.map((m, index) => {
            const id = roleId(m.project_id, m.role_key);
            const role = byId.get(id);
            const run = runAt(index);
            return (
              <div key={id}>
                {run && <RunLabel {...run} />}
                <div
                  className={`row-divider flex flex-wrap items-center gap-x-[18px] gap-y-2 px-5 py-3.5 ${
                    landed.has(id) ? "flash" : ""
                  }`}
                >
                  <div className="min-w-[min(220px,100%)] flex-1">
                    <Link
                      href={`/projects/${m.project_id}/roles/${encodeURIComponent(m.role_key)}`}
                      className="text-[15px] font-semibold link-hover"
                    >
                      {roleName(role?.display_name, m.role_key)}
                    </Link>{" "}
                    <Mono className="font-normal text-faint">{m.role_key}</Mono>
                    {waitingOn(m.project_id, m.role_key) && (
                      <NextStep className="mt-1 block" role={{ projectId: m.project_id, roleKey: m.role_key }} />
                    )}
                  </div>
                  <span className="hidden w-[200px] text-[13.5px] text-muted tablet:block">
                    <UserName id={m.marked_by} fallback={personOr(null, true)} />,{" "}
                    {formatShortDate(m.marked_at)}
                  </span>
                  <span className="w-full text-[15px] font-semibold tabular-nums tablet:w-[110px] tablet:shrink-0 tablet:text-right">
                    {peopleOf(role)}
                    <span className="text-[13px] font-normal text-faint tablet:hidden">
                      {peopleSuffix(peopleOf(role))}
                    </span>
                  </span>
                  <span className="flex w-[140px] justify-end whitespace-nowrap">
                    <Button
                      size="sm"
                      disabled={stop.isPending}
                      onClick={async () => {
                        setResult(null);
                        try {
                          await stop.mutateAsync({ ids: [id], waits: false });
                          setResult({
                            word: "Stopped",
                            outcome: {
                              kind: "applied",
                              message: `${roleName(role?.display_name, m.role_key)} goes straight to Zitadel now`,
                              detail: "Changes already waiting stay in Pending changes.",
                            },
                          });
                        } catch (e) {
                          setResult({
                            word: "Not stopped",
                            outcome: { kind: "failed", message: `${sentence(failureMessage(e))} ${stateLine(e)}` },
                          });
                        }
                      }}
                    >
                      Stop waiting
                    </Button>
                  </span>
                </div>
              </div>
            );
          })}
        </div>
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
        {result && (
          <div className="row-divider px-5 py-3">
            <ActionOutcome placement="inline" word={result.word} outcome={result.outcome} />
          </div>
        )}
    </Card>
  );
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
  // The sheet becomes its result and ends on Close; it never closes onto the list.
  // A failed press is a result too, but the picker stays so it can be pressed again.
  const [outcome, setOutcome] = useState<Outcome | null>(null);
  const done = succeeded(outcome);
  const chosen = Array.from(selected);
  const names = chosen.map((id) =>
    roleName(byId.get(id)?.display_name, id.slice(id.indexOf(":") + 1)),
  );
  // What Zitadel shows, summed; undefined when any of them was not checked,
  // so the sentence never states a recorded number as who has them.
  const people = chosen.reduce<number | undefined>((sum, id) => {
    const seen = byId.get(id)?.observed_user_count;
    return sum === undefined || seen === undefined || seen === null ? undefined : sum + seen;
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
          disabled={mark.isPending || done}
          legend={false}
        />
      </div>
      {outcome && <ActionOutcome outcome={outcome} word={done ? "Marked" : "Not marked"} className="mx-6 mt-3" />}
      <ModalFooter
        cancel={done ? undefined : { onClick: onClose }}
        note={
          done
            ? undefined
            : chosen.length
            ? `From now on, ${listOf(names)} ${chosen.length > 1 ? "wait" : "waits"} for you. ${
                people === undefined
                  ? `Nothing changes now for the people who have ${chosen.length > 1 ? "them" : "it"} today.`
                  : `${people} ${people === 1 ? "person has" : "people have"} ${
                      chosen.length > 1 ? "them" : "it"
                    } today, and nothing changes for them now.`
              }`
            : "Choose the roles that should always wait for you."
        }
      >
        {done ? (
          <Button variant="outline" onClick={onClose}>Close</Button>
        ) : (
          <Button
            variant="accent"
            disabled={!chosen.length || mark.isPending}
            isPending={mark.isPending}
            onClick={async () => {
              setOutcome(null);
              try {
                await mark.mutateAsync({ ids: chosen, waits: true });
                setOutcome({
                  kind: "applied",
                  message: `${listOf(names)} now ${chosen.length > 1 ? "wait" : "waits"} for you`,
                  detail: "Nothing changes for the people who have it today.",
                });
                onMarked(chosen);
              } catch (e) {
                setOutcome(outcomeFromError(e));
              }
            }}
          >
            {chosen.length
              ? `Mark ${chosen.length} ${chosen.length > 1 ? "roles" : "role"}`
              : "Mark roles"}
          </Button>
        )}
      </ModalFooter>
    </Modal>
  );
}

function listOf(names: string[]) {
  return names.length <= 2
    ? names.join(" and ")
    : `${names.slice(0, -1).join(", ")} and ${names.at(-1)}`;
}
