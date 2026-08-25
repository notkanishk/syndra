"use client";

import { formatBytes } from "@/lib/format";
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
import { oneShot } from "@/lib/secret";
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
        <NotYetUsable view={view} />
        <CredentialForm view={view} />
        <StorageUsage view={view} />
        <ConnectionInstructions view={view} />
      </div>
    </Card>
  );
}

/**
 * The account exists and will not let them in yet.
 *
 * Syndra creates the account before any password exists — it has none to set —
 * and the target disables password authentication until the member sets one. So
 * this page could show an account name, working mount instructions and a green
 * spine to somebody whose every connection attempt is refused, and nothing on
 * it said why.
 *
 * Read from the TARGET, not from Syndra's record that a password was set: the
 * record cannot say whether the target still accepts it.
 *
 * Framed as an unfinished step rather than a fault, because that is what it is,
 * and the action is directly below.
 */
function NotYetUsable({ view }: { view: MyTargetView }) {
  const storage = view.storage;
  if (!storage || storage.usable) return null;

  return (
    <div className="rounded-card border border-warn-line bg-warn-soft px-4 py-3">
      <p className="text-[14px] text-warn-text">
        {storage.needs_password ? (
          <>
            <strong className="font-semibold">Your account is ready, but not switched on yet.</strong>{" "}
            It will refuse you until you set a password below — that is what activates it.
          </>
        ) : (
          <>
            <strong className="font-semibold">Your account is on hold.</strong> It exists, and
            the system is not accepting it right now. Setting a password will not change that;
            an operator has to.
          </>
        )}
      </p>
    </div>
  );
}

/**
 * How much room they are using.
 *
 * The most-asked question a member has about storage, and until now Syndra
 * could not answer it at all — the number lives on the target and nothing read
 * it.
 *
 * A quota of zero is NOT a full bar: TrueNAS reports no quota field at all when
 * none is set, which is the common case, and drawing 100% for "no limit" would
 * be the most alarming possible way to say "you are fine".
 */
function StorageUsage({ view }: { view: MyTargetView }) {
  const storage = view.storage;
  if (!storage?.usage_readable || !storage.shares?.length) return null;

  return (
    <div className="grid gap-2">
      <p className="text-[13px] text-label">You are using</p>
      {storage.shares.map((share) => {
        const limited = (share.quota_bytes ?? 0) > 0;
        const pct = limited ? Math.min(100, (share.used_bytes / share.quota_bytes!) * 100) : 0;
        return (
          <div key={share.share} className="grid gap-1">
            <div className="flex items-baseline gap-2 text-[14px]">
              <span className="text-muted">{share.share}</span>
              <span className="flex-1" />
              <span className="font-mono text-[13.5px]">
                {formatBytes(share.used_bytes)}
                {limited ? ` of ${formatBytes(share.quota_bytes!)}` : ""}
              </span>
            </div>
            {limited ? (
              <div className="h-1.5 w-full overflow-hidden rounded-pill bg-line">
                <div
                  className={pct >= 90 ? "h-full bg-warn" : "h-full bg-accent"}
                  style={{ width: `${Math.max(pct, 1)}%` }}
                />
              </div>
            ) : (
              <p className="text-[12.5px] text-faint">No limit set on this share.</p>
            )}
          </div>
        );
      })}
    </div>
  );
}

/** Bytes as a person reads them. Binary units, because that is what a file
 *  manager will show them beside it. */
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
        set.mutate(oneShot(password), { onSuccess: () => setPassword("") });
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

/**
 * How to actually connect (10.8; design §30).
 *
 * The only screen in the product where somebody retypes a string into another
 * application, so every one of them is copyable rather than selectable prose.
 *
 * Three rules it must not break, all of them the same rule: only describe what
 * is real. The host comes from the add-on's registration, so moving the NAS is
 * a deployment change; the resources come from what their entitlements actually
 * resolve to, never from a template; and a withheld resource is named as
 * excluded rather than silently dropped, because a member who knows their
 * folder is missing on purpose does not spend twenty minutes hunting a typo.
 */
function ConnectionInstructions({ view }: { view: MyTargetView }) {
  if (!view.connection || !view.account) return null;

  const shares = Object.values(view.resources ?? {}).flat();
  if (shares.length === 0) return null;

  const host = view.connection.host;
  const held = view.suspended ?? [];

  return (
    <div className="grid gap-3 border-t border-line pt-4">
      <p className="text-[13px] text-label">Connecting</p>

      {shares.map((share) => (
        <div key={share} className="grid gap-2">
          <p className="text-[13.5px] text-muted">{share}</p>
          {/* Two rows, not three. macOS and Linux take the same string, and
              rendering it twice under two headings asks a member to compare
              two identical lines to find out they are identical. */}
          <div className="grid gap-1.5">
            <Platform
              label="macOS (Finder › Go › Connect to Server) and Linux (Files › Other Locations)"
              value={`smb://${host}/${share}`}
            />
            <Platform label="Windows — File Explorer address bar" value={`\\\\${host}\\${share}`} />
          </div>
        </div>
      ))}

      <p className="text-[13px] text-faint">
        Sign in as <span className="font-mono text-muted">{view.account.username}</span> with
        the password you set above.
      </p>

      {held.length > 0 && (
        // Named as excluded, never silently dropped.
        <p className="text-[13px] text-warn-text">
          {held.map((item) => item.value).join(", ")}{" "}
          {held.length === 1 ? "is" : "are"} not in this list on purpose — see the hold above.
        </p>
      )}
    </div>
  );
}

function Platform({ label, value }: { label: string; value: string }) {
  return (
    <div className="grid gap-1">
      <p className="text-[12.5px] text-faint">{label}</p>
      <CopyableValue value={value} label={label} />
    </div>
  );
}
