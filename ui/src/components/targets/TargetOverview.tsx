"use client";

import { useState } from "react";

import { EmptyState, ListStates } from "@/components/states";
import { Button } from "@/components/ui/Button";
import { Card, CardHeader } from "@/components/ui/Card";
import { Input } from "@/components/ui/Input";
import { PageHeader } from "@/components/ui/PageHeader";
import { Relative } from "@/components/ui/Time";
import { targetLabel } from "@/lib/nav";
import {
  useAdoptAccount,
  useSetLifecycle,
  useTargetHealth,
  useTargetInventory,
  useTargets,
} from "@/lib/queries/useTargets";

/**
 * One add-on target's operator page (9.20, 1.18/1.19, 15.6).
 *
 * Three panels, and they answer three questions an operator asks in this order:
 * is it healthy, what lives on it that we did not put there, and can I stop it
 * writing.
 *
 * The health panel distinguishes states a single "status" would flatten:
 * unreachable, read-only, draining, backlogged, and serving-from-a-stale-mirror
 * are five different things to do next. Stale data is labelled with its age
 * rather than withheld — an operator during an outage is better served by a
 * dated answer than by a spinner.
 */
export function TargetOverview({ target }: { target: string }) {
  const roster = useTargets();
  const health = useTargetHealth(target);
  const registered = (roster.data ?? []).find((t) => t.target === target);

  return (
    <>
      <PageHeader
        title={targetLabel(target)}
        meta={registered ? `Authenticated by ${registered.auth_mode}` : undefined}
      />

      <div className="grid gap-4">
        <Card>
          <CardHeader title="Health" />
          {health.isLoading && <p className="text-sm text-[var(--fg-muted)]">Reading…</p>}
          {health.data && <HealthLines target={target} health={health.data} />}
        </Card>

        <Inventory target={target} />

        {registered && (
          <Card>
            <CardHeader title="Capabilities" />
            {!registered.callable ? (
              <p className="text-sm text-[var(--fg-muted)]">
                Registered, and it has not published a capability manifest yet.
                Registration is a deployment fact; what it can do is a runtime
                one, and nothing is offered until it answers.
              </p>
            ) : (
              <ul className="grid gap-1 text-sm">
                {registered.operations.map((op) => (
                  <li key={op.id} className="flex items-baseline gap-2">
                    <span className="font-mono">{op.id}</span>
                    <span className="text-[var(--fg-muted)]">{op.scope}</span>
                    {!op.available && (
                      <span className="text-[var(--warn-fg)]">
                        unavailable — {op.unavailable_reason}
                      </span>
                    )}
                  </li>
                ))}
              </ul>
            )}
          </Card>
        )}

        <LifecycleControl target={target} />
      </div>
    </>
  );
}

/**
 * The five states, each rendered as itself.
 *
 * `draining` and `read_only` are deliberate operator decisions and must not read
 * as faults; `circuit_open` is the backend backing off and must not read as the
 * target being down; a stale snapshot is data with an age, not an error.
 */
function HealthLines({
  target,
  health,
}: {
  target: string;
  health: NonNullable<ReturnType<typeof useTargetHealth>["data"]>;
}) {
  if (!health.reachable) {
    return (
      <p className="text-sm">
        {targetLabel(target)} is not answering. {health.detail}
      </p>
    );
  }
  return (
    <dl className="grid gap-2 text-sm">
      <Line label="Target" value={`${health.product ?? "—"} ${health.product_version ?? ""}`} />
      <Line
        label="Version"
        value={health.version_tested ? "tested" : `untested — ${health.version_note ?? ""}`}
      />
      <Line
        label="Accepting changes"
        value={
          health.lifecycle === "active"
            ? "yes"
            : `no — ${health.lifecycle} (${health.lifecycle_note ?? "no reason recorded"})`
        }
      />
      {health.in_flight !== undefined && health.in_flight > 0 && (
        <Line label="In flight" value={String(health.in_flight)} />
      )}
      {health.circuit_open && (
        <Line
          label="Backed off"
          value="Syndra is refusing its own calls after repeated failures. Not the same as the target being down."
        />
      )}
      {health.snapshot_taken_at && (
        <Line
          label="Mirror"
          value={<>reads during an outage are served from a copy taken <Relative iso={health.snapshot_taken_at} /></>}
        />
      )}
      {health.key_expires_at && (
        <Line label="Credential expires" value={<Relative iso={health.key_expires_at} />} />
      )}
      {health.log_head && (
        <Line label="Mutation log" value={`${health.log_records ?? 0} records`} />
      )}
    </dl>
  );
}

function Line({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <div className="flex gap-3">
      <dt className="w-40 shrink-0 text-[var(--fg-muted)]">{label}</dt>
      <dd>{value}</dd>
    </div>
  );
}

/**
 * The unmanaged inventory (1.18/1.19).
 *
 * These are never drift and are never presented as such: a real NAS holds
 * `root`, service accounts and whatever an admin made by hand, and classifying
 * those as untraced access would bury the triage queue on the first sweep after
 * deployment. Adoption is the one way an account leaves this list, and it is an
 * operator decision — the account may belong to somebody else entirely.
 */
function Inventory({ target }: { target: string }) {
  const inventory = useTargetInventory(target);
  const adopt = useAdoptAccount(target);
  const [adopting, setAdopting] = useState<string | null>(null);
  const [subjectId, setSubjectId] = useState("");

  return (
    <Card>
      <CardHeader
        title="Accounts Syndra did not create"
        note="Reported, never triaged. These are not drift."
      />
      <ListStates
        isLoading={inventory.isLoading}
        error={inventory.error}
        isEmpty={(inventory.data?.unmanaged ?? []).length === 0}
        onRetry={() => inventory.refetch()}
        errorTitle="The account list could not be read"
        empty={
          <EmptyState
            title="Nothing unmanaged"
            guidance="Every account on this target belongs to somebody Syndra provisioned."
          />
        }
      >
        <>
          {inventory.data && !inventory.data.current && (
            <p className="mb-3 text-sm text-[var(--warn-fg)]">
              Served from a copy taken <Relative iso={inventory.data.read_at} />. Do not
              adopt from a list this old — the account may have moved since.
            </p>
          )}
          {inventory.data?.truncated && (
            <p className="mb-3 text-sm text-[var(--warn-fg)]">
              The account list was longer than one read returns. What is here is
              real; what is missing is unknown.
            </p>
          )}
          <ul className="grid gap-2 text-sm">
            {(inventory.data?.unmanaged ?? []).map((account) => (
              <li key={account.username} className="flex items-center gap-3">
                <span className="font-mono">{account.username}</span>
                {account.uid !== undefined && (
                  <span className="text-[var(--fg-muted)]">uid {account.uid}</span>
                )}
                <Button
                  variant="ghost"
                  onClick={() => setAdopting(account.username)}
                  disabled={!inventory.data?.current}
                >
                  Adopt
                </Button>
              </li>
            ))}
          </ul>
          {adopting && (
            <form
              className="mt-4 grid gap-2 border-t border-[var(--border)] pt-4 text-sm"
              onSubmit={(e) => {
                e.preventDefault();
                adopt.mutate(
                  { username: adopting, subjectId },
                  { onSuccess: () => { setAdopting(null); setSubjectId(""); } },
                );
              }}
            >
              <p>
                Adopting <span className="font-mono">{adopting}</span> hands its home
                directory, its shares and its group memberships to that person.
                Nothing on the account changes now; the next convergence applies
                their entitlements to it.
              </p>
              <Input
                aria-label="Person to adopt it for"
                placeholder="Subject id"
                value={subjectId}
                onChange={(e) => setSubjectId(e.target.value)}
              />
              <div className="flex gap-2">
                <Button type="submit" disabled={!subjectId || adopt.isPending}>
                  Adopt for this person
                </Button>
                <Button type="button" variant="ghost" onClick={() => setAdopting(null)}>
                  Cancel
                </Button>
              </div>
            </form>
          )}
        </>
      </ListStates>
    </Card>
  );
}

/**
 * Stopping the add-on writing, without a redeploy (15.6).
 *
 * `draining` and `read_only` differ in one way that matters during a credential
 * rotation: draining lets the calls already issued settle, and read-only does
 * not. An operator waiting to pull a key out from under a call needs the first.
 */
function LifecycleControl({ target }: { target: string }) {
  const set = useSetLifecycle(target);
  const [reason, setReason] = useState("");

  return (
    <Card>
      <CardHeader title="Maintenance" />
      <p className="text-sm text-[var(--fg-muted)]">
        Draining refuses new changes and lets the ones already sent finish — this
        is what makes a credential rotation safe. Read-only refuses every change
        immediately. Reads keep working in both.
      </p>
      <div className="mt-3 grid gap-2">
        <Input
          aria-label="Reason"
          placeholder="Why — this is what an operator reads later"
          value={reason}
          onChange={(e) => setReason(e.target.value)}
        />
        <div className="flex gap-2">
          {["active", "draining", "read_only"].map((state) => (
            <Button
              key={state}
              variant="ghost"
              disabled={!reason || set.isPending}
              onClick={() => set.mutate({ state, reason })}
            >
              {state.replace("_", " ")}
            </Button>
          ))}
        </div>
      </div>
    </Card>
  );
}
