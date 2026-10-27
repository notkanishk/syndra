"use client";

import { useEffect, useState } from "react";

import { EmptyState, ListStates } from "@/components/states";
import { ArrowLink } from "@/components/ui/ArrowLink";
import { StatusDot } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Card } from "@/components/ui/Card";
import { CopyableValue } from "@/components/ui/CopyableValue";
import { FieldLabel, Input } from "@/components/ui/Input";
import { Notice } from "@/components/ui/Notice";
import { Unfold } from "@/components/ui/Unfold";
import { NotOpenYet } from "@/components/ui/NotOpenYet";
import { PageHeader } from "@/components/ui/PageHeader";
import { Relative } from "@/components/ui/Time";
import { Withheld } from "@/components/ui/Withheld";
import { ConnectionGuide, deviceFrom, type Device, type GuideFolder } from "@/components/storage/ConnectionGuide";
import { describeFailure, sentence } from "@/lib/outcome";
import { formatBytes, formatClock, formatShortDate, humanizeKey } from "@/lib/format";
import { targetLabel } from "@/lib/nav";
import { useCatalogProjects } from "@/lib/queries/useCatalogUsers";
import {
  useMyStorage,
  useSetStorageCredential,
  type MyTargetView,
  type RoleSource,
  type StoragePlace,
} from "@/lib/queries/useMyStorage";
import { oneShot } from "@/lib/secret";

/**
 * A member's own storage page (route-storage; A-01, A-02).
 *
 * One card per system. A three-step line opens every state, so a member can
 * always see where they are: a role gives you access, your account is
 * created, you set a password. Then one display line saying where they are,
 * and the one thing to do, if there is one.
 *
 * Only what was read: places come from the target's own folder permissions
 * for the groups the account is actually in, addresses are built once by the
 * backend from the registered host, and usage that couldn't be read is never
 * drawn as zero.
 */
export function MyStorage() {
  const targets = useMyStorage();

  return (
    <>
      <PageHeader
        title="Network storage"
        lede="Your account on the makerspace's shared storage, and how to connect to it. Your access follows the roles you hold."
      />
      <ListStates
        isLoading={targets.isLoading}
        error={targets.error}
        isEmpty={(targets.data ?? []).length === 0}
        onRetry={() => targets.refetch()}
        errorTitle="Your storage access could not be read"
        empty={
          <EmptyState
            title="No shared storage yet."
            guidance="This makerspace has not connected any storage to Syndra. When it does, your account and how to reach it appear here."
          />
        }
      >
        <div className="grid gap-4">
          {(targets.data ?? []).map((view) => (
            <TargetCard key={view.target} view={view} />
          ))}
        </div>
      </ListStates>
    </>
  );
}

type State =
  | "none"
  | "held"
  | "waiting"
  | "needs-password"
  | "reenrol"
  | "restored"
  | "ready"
  | "unread"
  | "opens-nothing"
  | "nohost";

/** Where this member is, as one of the board's states. */
function stateOf(view: MyTargetView): State {
  const suspended = view.suspended ?? [];
  if (!view.entitled) {
    return suspended.some((s) => s.field === "enabled") ? "held" : "none";
  }
  if (!view.account) return "waiting";
  const storage = view.storage;
  if (storage?.needs_new_password) return "restored";
  if (view.credential.needs_re_enrolment) return "reenrol";
  if (storage?.needs_password || (!storage && !view.credential.set)) return "needs-password";
  if (storage?.places_readable && (storage.places ?? []).length === 0) return "opens-nothing";
  // The target's own answer about this account is missing: a read that failed,
  // not an account that is fine. Offline is its own sentence ("ready", below).
  if (!storage && view.health !== "offline") return "unread";
  if (!view.connection) return "nohost";
  return "ready";
}

const PASSWORD_STEP = new Set<State>(["needs-password", "reenrol", "restored"]);

function paused(view: MyTargetView) {
  return view.lifecycle === "draining" || view.lifecycle === "read_only";
}

/** A role named the way the member's own page names it: its label. */
function useRoleName() {
  const catalog = useCatalogProjects();
  return (role: RoleSource) => {
    const project = catalog.data.find((p) => p.id === role.project_id);
    const found = project?.roles.find((r) => r.key === role.role_key);
    return found?.label || humanizeKey(role.role_key);
  };
}

/** "Fabrication lead", "Fabrication lead and Lab tech". */
function joinNames(names: string[]): string {
  if (names.length <= 1) return names[0] ?? "";
  return `${names.slice(0, -1).join(", ")} and ${names[names.length - 1]}`;
}

function TargetCard({ view }: { view: MyTargetView }) {
  const label = targetLabel(view.target);
  const state = stateOf(view);
  const roleName = useRoleName();
  const offline = view.health === "offline";
  const isPaused = paused(view);
  const passwordStep = PASSWORD_STEP.has(state);
  const hasAccount = state !== "none" && state !== "held" && state !== "waiting";
  // The password can't be taken while the system can't take it. The form is
  // replaced by a sentence, never disabled (owner, 9 Oct).
  const passwordBlocked = offline || isPaused;

  const reached = state === "none" || state === "held" ? -1 : state === "waiting" ? 0 : passwordStep ? 1 : 2;
  const roles = (view.roles ?? []).map(roleName);
  const [headline, body, aside] = copyFor(state, label, view);
  const notice = pausedNotice(view, label);

  return (
    <Card>
      <div className="flex flex-wrap items-center justify-between gap-x-5 gap-y-2.5 px-5 pt-5">
        <span className="flex flex-wrap items-baseline gap-x-3.5 gap-y-1">
          <span className="type-card-title">{label}</span>
          <SystemStatus view={view} state={state} />
        </span>
        <Steps reached={reached} state={state} />
      </div>

      <Unfold open={Boolean(offline)}>
        <Notice tone="warn" title={`${label} is offline`} blocking live className="mx-5 mt-[18px]">
          It stopped answering{view.reachable_since ? ` at ${formatClock(view.reachable_since)}` : ""}, so your
          folders won&apos;t open until it&apos;s back. Your account and files are safe, and your access
          hasn&apos;t changed. Makerspace staff can see this too, and this page updates on its own when{" "}
          {label} returns.
        </Notice>
      </Unfold>

      <div className="flex flex-col gap-2 px-5 pb-[22px] pt-[18px]">
        <h2 className="font-display text-[24px] font-semibold leading-[1.25] tracking-[-0.01em] [text-wrap:pretty]">
          {headline}
        </h2>
        <p className="max-w-[66ch] text-[15px] leading-[1.55] text-muted [text-wrap:pretty]">{body}</p>
        {aside && <span className="text-[13.5px] text-faint">{aside}</span>}
        {state === "none" && (
          <ArrowLink href="/requests?ask" className="mt-1 self-start">
            Ask for access in Requests
          </ArrowLink>
        )}
      </div>

      {passwordStep && (
        <div className="mx-5 mb-[22px] flex flex-col gap-3 rounded-panel border border-accent-line bg-accent-soft px-[22px] pb-[22px] pt-5">
          {passwordBlocked ? (
            <span className="max-w-[66ch] text-[14.5px] leading-[1.55] text-ink">{blockedSentence(view, label)}</span>
          ) : (
            <PasswordForm view={view} label={label} accent submitLabel={state === "restored" ? "Set new password" : "Set password"} />
          )}
        </div>
      )}

      <Unfold open={Boolean(notice) && !hasAccount}>
        {notice && (
          <Notice tone="warn" title={notice.title} className="mx-5 mb-[22px]">
            {notice.body}
            {notice.person && <p className="mt-1.5 text-[14px] text-muted">{PERSON}</p>}
          </Notice>
        )}
      </Unfold>

      {hasAccount && view.account && (
        <>
          <div className="row-divider flex flex-col gap-2 px-5 py-[18px]">
            <span className="type-label text-label">Sign in as</span>
            <div className="max-w-[460px]">
              <CopyableValue value={view.account.username} label="Your account name" />
            </div>
          </div>

          <Folders
            view={view}
            state={state}
            label={label}
            roleName={roleName}
            roles={roles}
            passwordBlocked={passwordBlocked}
          />

          <Unfold open={Boolean(notice)}>
            {notice && (
              <Notice tone="warn" title={notice.title} className="mx-5 mb-5">
                {notice.body}
                {notice.person && <p className="mt-1.5 text-[14px] text-muted">{PERSON}</p>}
              </Notice>
            )}
          </Unfold>

          {!passwordStep && (
            <div className="row-divider flex flex-col gap-2 px-5 pb-[22px] pt-[18px]">
              <span className="type-label text-label">Storage password</span>
              {passwordBlocked ? (
                <span className="max-w-[66ch] text-[14.5px] leading-[1.55] text-ink">{blockedSentence(view, label)}</span>
              ) : (
                <PasswordForm view={view} label={label} submitLabel="Change password" />
              )}
            </div>
          )}
        </>
      )}

      {!hasAccount && (view.suspended ?? []).filter((s) => s.field !== "enabled").length > 0 && (
        <div className="px-5 pb-5">
          <Withheld items={(view.suspended ?? []).filter((s) => s.field !== "enabled")} />
        </div>
      )}
    </Card>
  );
}

const PERSON = "Need it today? Ask makerspace staff: they can see this pause and how long it should last.";

function copyFor(state: State, label: string, view: MyTargetView): [string, string, React.ReactNode] {
  const hold = (view.suspended ?? []).find((s) => s.field === "enabled");
  const ready = "Copy an address below into your computer, then sign in with your account name and storage password.";
  switch (state) {
    case "none":
      return [
        `None of your roles reaches ${label}`,
        "So there's no account here for you yet. Access comes with a role: ask for one, and your account follows on its own.",
        "",
      ];
    case "held":
      return [
        "Your account is switched off for now",
        `Makerspace staff have put your ${label} account on hold, so it turns sign-ins away. Your files are kept and your access is unchanged. It switches back on when the hold is lifted; there's nothing to do on your side.`,
        hold?.since ? `On hold since ${formatShortDate(hold.since)}` : "",
      ];
    case "waiting":
      return [
        "Your account is on its way",
        "Your access is recorded. Makerspace staff create the account itself, and nothing is needed from you in the meantime.",
        view.recorded_at ? (
          <>
            Recorded <Relative iso={view.recorded_at} /> · there&apos;s no set time, it waits on staff
          </>
        ) : (
          "There's no set time, it waits on staff"
        ),
      ];
    case "needs-password":
      return [
        "Set a password to switch your account on",
        `Your account on ${label} is ready, but it turns every sign-in away until it has a password. This is the last step.`,
        "",
      ];
    case "reenrol":
      return [
        "Your old storage password no longer works",
        `${label} was rebuilt since you set it, and passwords didn't carry over. Set a new one below. Nothing else about your access has changed.`,
        "",
      ];
    case "restored":
      return [
        "Set a new password to open your folders again",
        `Your access was given back${view.restored_at ? ` on ${formatShortDate(view.restored_at)}` : ""}. ${label} opens your folders again only with a new password, so the one you had before no longer works. Your files are as you left them.`,
        "",
      ];
    case "opens-nothing":
      return [
        "Your account is ready, but no folder is open to you yet",
        "Your account exists and your password is set. There's just nowhere to connect to yet, and nothing for you to do.",
        "",
      ];
    case "nohost":
      return [
        "Your account is ready",
        "Sign in with your account name and storage password. Syndra doesn't have the address to connect to yet, so ask makerspace staff for it.",
        "",
      ];
    case "unread":
      return [
        "Couldn't check your account just now",
        `${label} didn't answer when this page asked, so it can't say whether your account works. Your access hasn't changed; this page reads it again on your next visit.`,
        "",
      ];
    case "ready":
      return [
        "Your account is ready",
        view.health === "offline"
          ? `Once ${label} is back, sign in as usual. Nothing needs doing on your side.`
          : (view.suspended ?? []).length > 0
            ? `${ready} Some of your access is held back for now, with the reason beside it.`
            : ready,
        "",
      ];
  }
}

function pausedNotice(view: MyTargetView, label: string): { title: string; body: string; person: boolean } | null {
  if (!view.account && paused(view) && view.entitled) {
    return {
      title: "Your account is waiting on staff, not on you",
      body: `Staff are working on ${label}, so Syndra has paused creating accounts, yours included. ${label} itself is online. Your account is first in line, and nothing has been lost.`,
      person: true,
    };
  }
  if (!view.account) return null;
  if (view.lifecycle === "draining") {
    return {
      title: "Account changes are paused for a few minutes",
      body: `${label} is online and your access hasn't changed, so you can connect and use your files now. Syndra is only holding off on account changes while the last few finish.`,
      person: false,
    };
  }
  if (view.lifecycle === "read_only") {
    return {
      title: `Account changes are paused while staff work on ${label}`,
      body: `${label} is online and your access hasn't changed, so you can connect and use your files now. Syndra has stopped changing accounts until staff are done.`,
      person: true,
    };
  }
  return null;
}

function blockedSentence(view: MyTargetView, label: string) {
  if (view.health === "offline") {
    return `You can't change your password while ${label} is offline. Your current one works again as soon as it's back.`;
  }
  return "You can't change your password while account changes are paused. Your current one keeps working, and you can change it once the pause lifts.";
}

/** The status beside the system's name: what the monitor can stand behind, and nothing when it can't tell. */
function SystemStatus({ view, state }: { view: MyTargetView; state: State }) {
  if (state === "none" || !view.health || view.health === "unknown") return null;
  const offline = view.health === "offline";
  const held = state === "held";
  const checked = view.checked_at ? (
    <>
      {" "}
      · checked <Relative iso={view.checked_at} />
    </>
  ) : null;
  return (
    <span role="status" className={`tint-in inline-flex items-center gap-[7px] text-[13px] leading-[1.4] motion-tint ${offline ? "text-warn-text" : "text-muted"}`}>
      <span aria-hidden className={`h-[7px] w-[7px] flex-none rounded-pill motion-tint ${offline ? "bg-warn" : held ? "bg-line-strong" : "bg-healthy"}`} />
      {offline ? (
        <>Offline{view.reachable_since ? ` since ${formatClock(view.reachable_since)}` : ""}</>
      ) : held ? (
        <>Online · your account is on hold</>
      ) : state === "waiting" || state === "unread" || PASSWORD_STEP.has(state) ? (
        <>Online{checked}</>
      ) : (
        <>Online and accepting sign-ins{checked}</>
      )}
    </span>
  );
}

const STEPS = ["A role gives you access", "Your account is created", "You set a password"];

/** The three-step line; a segmented bar with one line under it on a phone. */
function Steps({ reached, state }: { reached: number; state: State }) {
  const inert = state === "none" || state === "held";
  const tone = (i: number) => (i <= reached ? "done" : i === reached + 1 && !inert ? "now" : "later");
  const word =
    state === "waiting"
      ? "Step 2 of 3 · your account is being created"
      : state === "restored"
        ? "Step 3 of 3 · you set a new password"
        : PASSWORD_STEP.has(state)
          ? "Step 3 of 3 · you set a password"
          : "All three steps done";
  return (
    <>
      <ol aria-label="Your progress" className="m-0 hidden list-none flex-wrap items-center gap-x-2.5 gap-y-1.5 p-0 tablet:flex">
        {STEPS.map((step, i) => {
          const t = tone(i);
          return (
            <li key={step} className="flex items-center gap-2.5">
              {i > 0 && <span aria-hidden className="h-px w-[18px] bg-line-strong" />}
              <span
                className={`inline-flex items-center gap-[7px] whitespace-nowrap text-[13px] ${
                  t === "now" ? "font-semibold text-ink" : t === "done" ? "text-muted" : "text-faint"
                }`}
              >
                <span
                  aria-hidden
                  className={`h-[7px] w-[7px] rounded-pill ${t === "done" ? "bg-healthy" : t === "now" ? "bg-accent" : "bg-line-strong"}`}
                />
                {step}
              </span>
            </li>
          );
        })}
      </ol>
      {!inert && (
        <div aria-hidden className="w-full tablet:hidden">
          <span className="mb-[7px] flex gap-1">
            {STEPS.map((step, i) => {
              const t = tone(i);
              return (
                <span
                  key={step}
                  className={`h-1 flex-1 rounded-pill ${t === "done" ? "bg-healthy" : t === "now" ? "bg-accent" : "bg-line-strong"}`}
                />
              );
            })}
          </span>
          <span className="text-[13px] text-muted">{word}</span>
        </div>
      )}
    </>
  );
}

/** "Your folders": one block per place, home first. */
function Folders({
  view,
  state,
  label,
  roleName,
  roles,
  passwordBlocked,
}: {
  view: MyTargetView;
  state: State;
  label: string;
  roleName: (r: RoleSource) => string;
  roles: string[];
  /** Offline or paused: there is no password field to send them to. */
  passwordBlocked: boolean;
}) {
  const [guideOpen, setGuideOpen] = useState(false);
  const [device, setDevice] = useState<Device>("mac");
  useEffect(() => setDevice(deviceFrom(navigator.userAgent)), []);

  const storage = view.storage;
  const places = storage?.places ?? [];
  const offline = view.health === "offline";
  const locked = PASSWORD_STEP.has(state);
  const withheld = (view.suspended ?? []).filter((s) => s.field !== "enabled");
  const host = view.connection?.host;
  const guideFolders: GuideFolder[] = places
    .filter((p) => p.smb_address && p.unc_address)
    .map((p) => ({
      label: p.home ? "Your home folder" : nameOf(p),
      share: p.share,
      folder: p.folder ?? "",
      smb: p.smb_address!,
      unc: p.unc_address!,
    }));
  const guide = !offline && !!host && guideFolders.length > 0;

  return (
    <div className="row-divider flex flex-col gap-3.5 px-5 pb-5 pt-[18px]">
      <div className="flex flex-wrap items-baseline gap-x-3.5 gap-y-1">
        <span className="type-label text-label">Your folders</span>
        {!locked && offline && (
          <span className="text-[13px] text-faint">As last known · they open again when {label} is back</span>
        )}
        {guide && (
          <span className="ml-auto hidden flex-none whitespace-nowrap tablet:flex">
            <Button variant="ghost" size="sm" onClick={() => setGuideOpen(true)}>
              First time? How to connect
            </Button>
          </span>
        )}
      </div>

      {state === "unread" || (storage && !storage.places_readable) ? (
        <p className="text-[14px] text-muted">
          {label} couldn&apos;t say which folders are open to you just now. Your access hasn&apos;t changed; this
          page reads it again on your next visit.
        </p>
      ) : state === "opens-nothing" ? (
        <div className="flex items-start gap-2.5 rounded-inner border border-dashed border-line-strong px-4 py-3.5 text-[14px] leading-[1.55] text-muted [text-wrap:pretty]">
          <span aria-hidden className="mt-[7px] h-[7px] w-[7px] flex-none rounded-pill border-[1.5px] border-line-strong" />
          <span>
            None yet.{" "}
            {roles.length > 0
              ? `Your ${joinNames(roles)} ${roles.length === 1 ? "role reaches" : "roles reach"} ${label}, but no folder there is open to ${roles.length === 1 ? "it" : "them"}.`
              : `Your access reaches ${label}, but no folder there is open to it.`}{" "}
            Makerspace staff open folders on {label} itself; this list fills in by itself when they do.
          </span>
        </div>
      ) : (
        <NotOpenYet
          open={!locked}
          title={state === "restored" ? "Set a new password to open these" : "Set your password to open these"}
          why={`Until then, ${label} turns every sign-in away.`}
          step={passwordBlocked ? undefined : { label: "Set it above ↑", href: `#${passwordFieldId(view)}` }}
        >
          <div className="flex flex-col gap-3.5">
            {places.map((place) => (
              <PlaceBlock
                key={place.path}
                place={place}
                offline={offline}
                showAddresses={!!host}
                homeKept={!!place.home && !!view.home_kept}
                roleName={roleName}
              />
            ))}
          </div>
        </NotOpenYet>
      )}

      {guide && (
        <div className="tablet:hidden">
          <Button variant="outline" size="sm" className="w-full" onClick={() => setGuideOpen(true)}>
            First time? How to connect
          </Button>
        </div>
      )}
      {withheld.length > 0 && <Withheld items={withheld} />}

      {guide && host && view.account && (
        <ConnectionGuide
          open={guideOpen}
          onClose={() => setGuideOpen(false)}
          host={host}
          folders={guideFolders}
          username={view.account.username}
          system={label}
          defaultDevice={device}
        />
      )}
    </div>
  );
}

/** The folder's own name, the word people use for it. */
function nameOf(place: StoragePlace) {
  if (!place.folder) return place.share;
  const parts = place.folder.split("/");
  return parts[parts.length - 1];
}

function PlaceBlock({
  place,
  offline,
  showAddresses,
  homeKept,
  roleName,
}: {
  place: StoragePlace;
  offline: boolean;
  showAddresses: boolean;
  homeKept: boolean;
  roleName: (r: RoleSource) => string;
}) {
  const can = place.access === "change";
  const roles = (place.roles ?? []).map(roleName);
  const where = place.home
    ? `In ${[place.share, ...(place.folder ?? "").split("/").slice(0, -1)].join(" › ")} · private to you`
    : place.folder
      ? `In ${[place.share, ...place.folder.split("/").slice(0, -1)].join(" › ")}${roles.length ? ` · from your ${joinNames(roles)} ${roles.length === 1 ? "role" : "roles"}` : ""}`
      : roles.length
        ? `From your ${joinNames(roles)} ${roles.length === 1 ? "role" : "roles"}`
        : "";

  const usage = place.usage;
  const quota = usage?.quota_bytes ?? 0;
  const pct = usage && quota > 0 ? Math.min(100, (usage.used_bytes / quota) * 100) : 0;
  const full = pct >= 90;
  // At or over its limit TrueNAS refuses new writes and keeps every file, so
  // the member can still open and delete (a limit set below what they hold).
  const over = Boolean(usage && quota > 0 && usage.used_bytes >= quota);
  let use: string | null = null;
  if (can) {
    if (offline) use = "Usage unavailable while offline";
    else if (usage) {
      use = !usage.used_bytes
        ? "Nothing saved here yet"
        : quota > 0
          ? `${formatBytes(usage.used_bytes)} of ${formatBytes(quota)}${full ? " · nearly full" : ""}`
          : `${formatBytes(usage.used_bytes)} used · no limit set`;
    }
  }

  return (
    <div className="flex flex-col gap-2.5 rounded-inner border border-line bg-surface-0 px-4 py-3.5">
      <div className="flex flex-wrap items-baseline gap-x-3.5 gap-y-1">
        <span className="flex min-w-0 flex-[1_1_260px] flex-col gap-0.5">
          <span className="flex flex-wrap items-baseline gap-x-2.5 gap-y-0.5">
            <span className="text-[15.5px] font-semibold [overflow-wrap:anywhere]">
              {place.home ? "Your home folder" : nameOf(place)}
            </span>
            <span className="whitespace-nowrap text-[13px] text-muted">{can ? "Can change files" : "Read only"}</span>
          </span>
          {where && <span className="text-[13px] text-faint [text-wrap:pretty]">{where}</span>}
        </span>
        {use && <span className={`ml-auto text-[13.5px] ${!offline && full ? "text-warn-text" : "text-muted"}`}>{use}</span>}
      </div>
      {can && over && !offline && (
        <span className="max-w-[70ch] text-[14px] leading-[1.55] text-warn-text [text-wrap:pretty]">
          Full. You can open and delete files here, but not save new ones until you&apos;re under {formatBytes(quota)}.
        </span>
      )}
      {homeKept && (
        <span className="max-w-[70ch] text-[14px] leading-[1.55] text-ink [text-wrap:pretty]">
          None of the roles you hold gives a home folder any more. It stays, with your files in it, and only you can
          open it. Syndra never moves or deletes a home folder.
        </span>
      )}
      {can && usage && quota > 0 && !offline && (
        <div aria-hidden className="h-[5px] overflow-hidden rounded-pill bg-line">
          <div className={`h-full rounded-pill ${full ? "bg-warn" : "bg-muted"}`} style={{ width: `${Math.max(pct, 1)}%` }} />
        </div>
      )}
      {showAddresses && place.smb_address && place.unc_address && (
        <div className="grid grid-cols-[repeat(auto-fit,minmax(min(100%,300px),1fr))] gap-x-3.5 gap-y-2.5">
          <div className="flex min-w-0 flex-col gap-[5px]">
            <span className="text-[12.5px] text-muted">Mac and Linux · Connect to Server</span>
            <CopyableValue value={place.smb_address} label="Address for Mac and Linux" />
          </div>
          <div className="flex min-w-0 flex-col gap-[5px]">
            <span className="text-[12.5px] text-muted">Windows · File Explorer address bar</span>
            <CopyableValue value={place.unc_address} label="Address for Windows" />
          </div>
        </div>
      )}
    </div>
  );
}

/**
 * Setting the storage password. The value goes to the target and is kept
 * nowhere: not in the response, not in the query cache, not in Syndra's
 * database.
 */
/** One per system: two cards' fields shared an id, so "Set it above" jumped to the first. */
function passwordFieldId(view: MyTargetView): string {
  return `storage-password-${view.target}`;
}

function PasswordForm({
  view,
  label,
  submitLabel,
  accent = false,
}: {
  view: MyTargetView;
  label: string;
  submitLabel: string;
  accent?: boolean;
}) {
  const [password, setPassword] = useState("");
  const set = useSetStorageCredential(view.target);
  const unconfirmed = Boolean(set.data?.status && set.data.status !== "set");
  const scope = `Only for ${label}: not your Syndra sign-in or your Google account.`;

  return (
    <form
      className="flex flex-col gap-2"
      onSubmit={(e) => {
        e.preventDefault();
        set.mutate(oneShot(password), { onSuccess: () => setPassword("") });
      }}
    >
      <div className="flex max-w-[460px] flex-col gap-1.5">
        {accent && <FieldLabel htmlFor={passwordFieldId(view)}>Storage password</FieldLabel>}
        <div className="flex flex-wrap items-center gap-2.5">
          <div className="min-w-0 flex-[1_1_220px]">
            <Input
              id={passwordFieldId(view)}
              type="password"
              autoComplete="new-password"
              aria-label={accent ? undefined : "New storage password"}
              value={password}
              onChange={(e) => setPassword(e.target.value)}
            />
          </div>
          <span className="flex-none whitespace-nowrap">
            <Button type="submit" variant={accent ? "accent" : "outline"} disabled={!password} isPending={set.isPending}>
              {submitLabel}
            </Button>
          </span>
        </div>
      </div>
      <p className={`max-w-[62ch] leading-[1.55] [text-wrap:pretty] ${accent ? "text-[14px] text-ink" : "text-[13.5px] text-muted"}`}>
        {accent ? (
          <>{scope} Syndra passes it to {label} and doesn&apos;t keep a copy.</>
        ) : view.credential.last_changed_at ? (
          <>
            Last changed <Relative iso={view.credential.last_changed_at} />. {scope}
          </>
        ) : (
          scope
        )}
      </p>
      {set.data && (
        // An outcome the target didn't confirm is not success, and doesn't get the calm voice.
        <p
          role="status"
          // Done lifts and blooms once; not confirmed appears still, on tint (S6).
          className={`${unconfirmed ? "tint-in text-warn-text" : "lift-in flex items-center gap-2 text-muted"} text-[13.5px]`}
        >
          {!unconfirmed && <StatusDot tone="healthy" bloom />}
          {set.data.detail}
        </p>
      )}
      {set.error && <p className="text-[13.5px] text-danger-text">{sentence(describeFailure(set.error))}</p>}
    </form>
  );
}
