"use client";

import { useState } from "react";

import { EmptyState, ListStates } from "@/components/states";
import { Card, CardHeader } from "@/components/ui/Card";
import { Button } from "@/components/ui/Button";
import { Input } from "@/components/ui/Input";
import { PageHeader } from "@/components/ui/PageHeader";
import { Relative } from "@/components/ui/Time";
import { targetLabel } from "@/lib/nav";
import { useMyStorage, useSetStorageCredential, type MyTargetView } from "@/lib/queries/useMyStorage";

/**
 * A member's own storage view (group 10).
 *
 * Three states, rendered as three. They are what a member actually experiences,
 * and collapsing any two produces a screen that lies:
 *
 *   no entitlement       — no role of theirs reaches this system. No credential
 *                          form, no connection instructions, and an explanation
 *                          instead of an empty panel.
 *   entitled, no account — the change is queued and has not been applied yet.
 *                          The credential affordance is WITHHELD: setting one
 *                          would be dispatched at an account that does not
 *                          exist, and they would be told their password was set.
 *   account present      — everything.
 *
 * The middle state is the one a two-state design gets wrong, and it is not an
 * edge case: these changes wait for an operator, so it is the ordinary
 * experience of every new member until that happens.
 */
export function MyStorage() {
  const targets = useMyStorage();

  return (
    <>
      <PageHeader
        title="Network storage"
        meta="Shared storage for makerspace work. Your access follows the roles you hold."
      />
      <ListStates
        isLoading={targets.isLoading}
        error={targets.error}
        isEmpty={(targets.data ?? []).length === 0}
        onRetry={() => targets.refetch()}
        errorTitle="Your storage access could not be read"
        empty={
          <EmptyState
            title="No storage systems"
            guidance="This deployment does not run any shared storage."
          />
        }
      >
        <div className="grid gap-4">
          {(targets.data ?? []).map((view) => (
            <TargetPanel key={view.target} view={view} />
          ))}
        </div>
      </ListStates>
    </>
  );
}

function TargetPanel({ view }: { view: MyTargetView }) {
  const label = targetLabel(view.target);

  if (!view.entitled) {
    return (
      <Card>
        <CardHeader title={label} />
        <p className="text-sm text-[var(--fg-muted)]">
          None of your roles reaches {label}, so there is no account to set up.
          Access here comes with a role — request one and it follows automatically.
        </p>
        {view.suspended && view.suspended.length > 0 && (
          <Withheld suspended={view.suspended} />
        )}
      </Card>
    );
  }

  if (!view.account) {
    return (
      <Card>
        <CardHeader title={label} />
        <p className="text-sm text-[var(--fg-muted)]">
          Your access has been recorded and your account here has not been created
          yet. Nothing is needed from you — this usually clears within a day.
        </p>
      </Card>
    );
  }

  return (
    <Card>
      <CardHeader title={label} />
      <dl className="grid gap-3 text-sm">
        <div>
          <dt className="text-[var(--fg-muted)]">Sign in as</dt>
          <dd className="font-mono">{view.account.username}</dd>
        </div>
        {view.resources && Object.keys(view.resources).length > 0 && (
          <div>
            <dt className="text-[var(--fg-muted)]">You can reach</dt>
            <dd>
              {Object.entries(view.resources).map(([field, values]) => (
                <div key={field}>{values.join(", ")}</div>
              ))}
            </dd>
          </div>
        )}
      </dl>
      {view.suspended && view.suspended.length > 0 && <Withheld suspended={view.suspended} />}
      <CredentialForm view={view} />
    </Card>
  );
}

/**
 * What an operator has withheld, with the reason.
 *
 * A member seeing access they expect to have and do not, with no explanation,
 * asks an operator. One who can read the reason does not have to — and the
 * reason is already recorded, so withholding it here would be a deliberate
 * omission rather than a missing feature.
 */
function Withheld({ suspended }: { suspended: NonNullable<MyTargetView["suspended"]> }) {
  return (
    <div className="mt-4 rounded-md border border-[var(--warn-border)] bg-[var(--warn-bg)] p-3 text-sm">
      <p className="font-medium">Some access is currently withheld</p>
      <ul className="mt-1 list-disc pl-5">
        {suspended.map((s) => (
          <li key={`${s.field}:${s.value}`}>
            {s.value} — {s.reason}
          </li>
        ))}
      </ul>
    </div>
  );
}

/**
 * Setting the storage password.
 *
 * The scope sentence is not decoration: members reasonably assume one password,
 * and this one is neither their Syndra sign-in nor their Google account. The
 * value is sent and kept nowhere — not in the response, not in the cache, and
 * not in Syndra's database.
 */
function CredentialForm({ view }: { view: MyTargetView }) {
  const [password, setPassword] = useState("");
  const set = useSetStorageCredential(view.target);

  if (!view.reachable) {
    return (
      <p className="mt-4 text-sm text-[var(--fg-muted)]">
        {targetLabel(view.target)} is not answering right now, so a password set
        here would not reach it. Nothing has changed — try again shortly.
      </p>
    );
  }

  return (
    <form
      className="mt-4 grid gap-2"
      onSubmit={(e) => {
        e.preventDefault();
        set.mutate(password, { onSuccess: () => setPassword("") });
      }}
    >
      {view.credential.needs_re_enrolment && (
        <p className="rounded-md border border-[var(--warn-border)] bg-[var(--warn-bg)] p-3 text-sm">
          You set a storage password before this system changed. That password no
          longer works — set a new one below. Nothing else about your access
          changed.
        </p>
      )}
      <label className="text-sm" htmlFor="storage-password">
        {view.credential.set ? "Change your storage password" : "Set a storage password"}
      </label>
      <Input
        id="storage-password"
        type="password"
        autoComplete="new-password"
        value={password}
        onChange={(e) => setPassword(e.target.value)}
      />
      <p className="text-xs text-[var(--fg-muted)]">
        Used only for {targetLabel(view.target)}. It is not your Syndra sign-in
        and not your Google account.
      </p>
      {view.credential.set && view.credential.last_changed_at && (
        <p className="text-xs text-[var(--fg-muted)]">
          Last changed <Relative iso={view.credential.last_changed_at} />.
        </p>
      )}
      <div>
        <Button type="submit" disabled={!password || set.isPending}>
          {set.isPending ? "Setting…" : "Set password"}
        </Button>
      </div>
      {set.data && <p className="text-sm">{set.data.detail}</p>}
      {set.error && (
        <p className="text-sm text-[var(--danger-fg)]">
          {set.error instanceof Error ? set.error.message : "That could not be applied."}
        </p>
      )}
    </form>
  );
}
