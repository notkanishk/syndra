"use client";

import { useState } from "react";

import { EmptyState, ListStates } from "@/components/states";
import { Button } from "@/components/ui/Button";
import { Card, CardHeader } from "@/components/ui/Card";
import { CopyableValue } from "@/components/ui/CopyableValue";
import { Input } from "@/components/ui/Input";
import { PageHeader } from "@/components/ui/PageHeader";
import { Relative } from "@/components/ui/Time";
import { Withheld } from "@/components/ui/Withheld";
import { targetLabel } from "@/lib/nav";
import { useMyStorage, useSetStorageCredential, type MyTargetView } from "@/lib/queries/useMyStorage";

/**
 * A member's own storage view (group 10; design §20).
 *
 * Three states, rendered as three. They are what a member actually experiences,
 * and collapsing any two produces a screen that lies:
 *
 *   no entitlement       — no role of theirs reaches this system. No credential
 *                          form, no account name, no connection instructions,
 *                          and an explanation instead of an empty panel.
 *   entitled, no account — the change is recorded and has not been applied yet.
 *                          The credential affordance is WITHHELD: setting one
 *                          would be dispatched at an account that does not
 *                          exist, and they would be told their password was set.
 *   account present      — everything.
 *
 * The middle state is the one a two-state design gets wrong, and it is not an
 * edge case: these changes wait for an operator, so it is the ordinary
 * experience of every new member until that happens.
 *
 * One three-step spine opens all three, so they read as a progression rather
 * than as three unrelated panels — and in the first state the spine is inert,
 * explaining the mechanism without promising an outcome.
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

/**
 * The spine: role → account → password.
 *
 * Three nodes, always all three, and the reached ones are lit. A member in the
 * middle state can see that the thing they are waiting for is the second node
 * and that a third exists — which is the difference between waiting and
 * wondering whether they have missed a step.
 *
 * `reached` of -1 is the inert form: nothing lit, nothing promised.
 */
function Spine({ reached }: { reached: -1 | 0 | 1 | 2 }) {
  const steps = ["A role reaches it", "Your account is created", "You set a password"];
  return (
    <ol className="flex flex-wrap items-center gap-x-2 gap-y-1 text-[12.5px]">
      {steps.map((step, i) => (
        <li key={step} className="flex items-center gap-2">
          {i > 0 && (
            <span aria-hidden className="text-faint">
              ›
            </span>
          )}
          <span className="flex items-center gap-1.5">
            <span
              aria-hidden
              className={`size-1.5 rounded-pill ${i <= reached ? "bg-healthy" : "bg-tint-3"}`}
            />
            <span className={i <= reached ? "text-muted" : "text-faint"}>{step}</span>
          </span>
        </li>
      ))}
    </ol>
  );
}

function TargetPanel({ view }: { view: MyTargetView }) {
  const label = targetLabel(view.target);
  const held = view.suspended ?? [];

  if (!view.entitled) {
    return (
      <Card>
        <CardHeader title={label} />
        <div className="grid gap-3 px-5 pb-5">
          <Spine reached={-1} />
          <p className="text-[14px] text-muted">
            None of your roles reaches {label}, so there is no account to set up. Access
            here comes with a role — ask for the role and this follows on its own.
          </p>
          {held.length > 0 && <Withheld items={held} />}
        </div>
      </Card>
    );
  }

  if (!view.account) {
    return (
      <Card>
        <CardHeader title={label} />
        <div className="grid gap-3 px-5 pb-5">
          <Spine reached={0} />
          <p className="text-[14px] text-muted">
            Your access is recorded and your account here has not been created yet.
            Nothing is needed from you.
          </p>
          {/* An age, never an estimate, and nothing that spins. The wait is on a
              person resuming the queue, not on a timer — a spinner would say
              "still happening" about something that is not currently happening,
              and "usually within a day" is a guess about somebody's week. */}
          <p className="text-[13px] text-faint">
            {view.recorded_at ? (
              <>
                Recorded <Relative iso={view.recorded_at} /> · waits on a person, not a
                timer
              </>
            ) : (
              <>Waits on a person, not a timer</>
            )}
          </p>
          {held.length > 0 && <Withheld items={held} />}
        </div>
      </Card>
    );
  }

  return (
    <Card>
      <CardHeader title={label} />
      <div className="grid gap-4 px-5 pb-5">
        <Spine reached={view.credential.set ? 2 : 1} />

        <div className="grid gap-1.5">
          <p className="text-[13px] text-label">Sign in as</p>
          <CopyableValue value={view.account.username} label="Your account name" />
        </div>

        {view.resources && Object.keys(view.resources).length > 0 && (
          <div className="grid gap-1.5">
            <p className="text-[13px] text-label">You can reach</p>
            <ul className="grid gap-1 text-[14px] text-muted">
              {Object.entries(view.resources).map(([field, values]) => (
                <li key={field}>{values.join(", ")}</li>
              ))}
            </ul>
          </div>
        )}

        {held.length > 0 && <Withheld items={held} />}
        <CredentialForm view={view} />
      </div>
    </Card>
  );
}

/**
 * Setting the storage password.
 *
 * The scope sentence is body copy, not a hint, and it is load-bearing: members
 * reasonably assume one password, and this one is neither their Syndra sign-in
 * nor their Google account.
 *
 * The value is sent and kept nowhere — not in the response, not in the query
 * cache, and not in Syndra's database.
 */
function CredentialForm({ view }: { view: MyTargetView }) {
  const [password, setPassword] = useState("");
  const set = useSetStorageCredential(view.target);

  if (!view.reachable) {
    // Replaced, never disabled. A credential set against a target that never
    // answered must not be reported as done, so the field does not exist to be
    // submitted in the first place.
    return (
      <div className="rounded-inner border border-warn-line bg-warn-soft px-4 py-3">
        <p className="text-[13.5px] font-semibold text-warn-text">
          {targetLabel(view.target)} is not answering
        </p>
        <p className="mt-1 text-[13.5px] text-muted">
          A password set now would not reach it, so the form is not here. Nothing about
          your access has changed — come back shortly.
        </p>
      </div>
    );
  }

  const unconfirmed = Boolean(set.data?.status && set.data.status !== "set");

  return (
    <form
      className="grid gap-2"
      onSubmit={(e) => {
        e.preventDefault();
        set.mutate(password, { onSuccess: () => setPassword("") });
      }}
    >
      {view.credential.needs_re_enrolment && (
        <div className="rounded-inner border border-warn-line bg-warn-soft px-4 py-3">
          <p className="text-[13.5px] font-semibold text-warn-text">
            Your old storage password no longer works
          </p>
          <p className="mt-1 text-[13.5px] text-muted">
            You set one before this system changed, and it went with the system it was
            for. Set a new one below — nothing else about your access changed.
          </p>
        </div>
      )}
      <label className="text-[14px]" htmlFor="storage-password">
        {view.credential.set ? "Change your storage password" : "Set a storage password"}
      </label>
      <Input
        id="storage-password"
        type="password"
        autoComplete="new-password"
        value={password}
        onChange={(e) => setPassword(e.target.value)}
      />
      <p className="text-[15px] text-muted">
        This password is used only for {targetLabel(view.target)}. It is not your Syndra
        sign-in, and not your Google account.
      </p>
      {view.credential.set && view.credential.last_changed_at && (
        <p className="text-[13px] text-faint">
          Last changed <Relative iso={view.credential.last_changed_at} />.
        </p>
      )}
      <div>
        <Button type="submit" variant="accent" disabled={!password || set.isPending}>
          {set.isPending ? "Setting…" : "Set password"}
        </Button>
      </div>
      {set.data && (
        // An outcome the target did not confirm is not success, and it does not
        // get the calm voice. Retrying blind is the wrong reflex on a path that
        // may have landed.
        <p role="status" className={`text-[13.5px] ${unconfirmed ? "text-warn-text" : "text-muted"}`}>
          {set.data.detail}
        </p>
      )}
      {set.error && (
        <p className="text-[13.5px] text-danger-text">
          {set.error instanceof Error ? set.error.message : "That could not be applied."}
        </p>
      )}
    </form>
  );
}
