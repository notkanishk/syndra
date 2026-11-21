"use client";

import { notRead } from "@/lib/queries/read";
import { SYSTEM_WORDS } from "@/components/targets/TargetReading";
import { useEffect, useState } from "react";

import { EmptyState } from "@/components/states";
import { ActionOutcome } from "@/components/ui/ActionOutcome";
import { ArrowLink } from "@/components/ui/ArrowLink";
import { StatusDot } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { MEMBER } from "@/components/results/copy";
import { ReadFreshness, useReadAgainPress } from "@/components/ui/ReadFreshness";
import { Reads, saysAge } from "@/components/results/ReadResult";
import { UsageUnread } from "@/components/results/StatusMeters";
import { Card } from "@/components/ui/Card";
import { CopyableValue } from "@/components/ui/CopyableValue";
import { FieldLabel, Input } from "@/components/ui/Input";
import { Notice } from "@/components/ui/Notice";
import { Unfold } from "@/lib/motion";
import { NotOpenYet } from "@/components/ui/NotOpenYet";
import { Relative } from "@/components/ui/Time";
import { Withheld } from "@/components/ui/Withheld";
import type { GuideFolder } from "@/components/kinds/storage/ConnectionGuide";
import { deviceFrom, type Device } from "@/components/kinds/storage/device";
import { OWN_READS, PASSWORD_STEP, drawnUsage, isHold, stateOf, storageOf, usageShown, type State } from "@/components/kinds/storage/state";
import { describeFailure, isNothingSent, nothingSentOutcome, NOTHING_SENT_WORD, sentence } from "@/lib/outcome";
import { sortByName } from "@/lib/sort";
import { formatAt, formatBytes, formatClock, formatList, formatShortDate } from "@/lib/format";
import { down } from "@/components/kinds/storage/state";
import { useCatalogProjects } from "@/lib/queries/useCatalogUsers";
import { useLongWait } from "@/lib/useLongWait";
import {
  useSetStorageCredential,
  type MyTargetView,
  type RoleSource,
  type StoragePlace,
} from "@/lib/queries/useMyStorage";
import { Meter } from "@/components/ui/Meter";
import { PLACE_NOT_COUNTED, roomLine } from "@/components/kinds/storage/captions";
import { HomeMonth } from "@/components/usage/UsageCard";
import { oneShot } from "@/lib/secret";
import dynamic from "next/dynamic";
import { RetryStatus } from "@/components/ui/RetryStatus";
import { retryLine, type RetryState } from "@/lib/retry";
import { roleName } from "@/lib/names";

// Opened on demand, so it stays out of the page's first load.
const ConnectionGuide = dynamic(() => import("@/components/kinds/storage/ConnectionGuide").then((m) => m.ConnectionGuide));

/**
 * The storage kind's card for a member (route-storage; A-01, A-02): the module
 * `components/kinds/MemberTargetCard.tsx` picks for a system of kind `storage`.
 * The page that lists the cards is `components/kinds/storage/StoragePage.tsx`.
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

function paused(view: MyTargetView) {
  return view.lifecycle === "draining" || view.lifecycle === "read_only";
}

/** A role named the way the member's own page names it: its label. */
function useRoleName() {
  const catalog = useCatalogProjects();
  return (role: RoleSource) => {
    const project = catalog.data.find((p) => p.id === role.project_id);
    const found = project?.roles.find((r) => r.key === role.role_key);
    return roleName(found?.label, role.role_key);
  };
}

export function StorageMemberCard({ view }: { view: MyTargetView }) {
  const label = view.name;
  const state = stateOf(view);
  const roleName = useRoleName();
  const again = useReadAgainPress();
  const isPaused = paused(view);
  const passwordStep = PASSWORD_STEP.has(state);
  const hasAccount = state !== "none" && state !== "held" && state !== "waiting";
  const storage = storageOf(view);
  // The password can't be taken while the system can't take it. The form is
  // replaced by a sentence, never disabled (owner, 9 Oct).
  const passwordBlocked = down(view) || isPaused;

  const reached = state === "none" || state === "held" ? -1 : state === "waiting" ? 0 : passwordStep ? 1 : 2;
  const roles = (view.roles ?? []).map(roleName);
  const [headline, body, aside] = copyFor(state, label, view);
  const notice = pausedNotice(view, label);
  // The usage Folders draws, place by place: what the unread-usage notice below stands beside.
  const drawn = hasAccount && storage?.places_readable ? (storage.places ?? []).map(drawnUsage) : [];
  // The add-on's kept copy (design §13.3): the card recedes, as an operator's does, and is dated.
  // Its storage.status line below dates it when the reply carries one; otherwise the strip does.
  const kept = view.read?.state === "kept";
  const datedBelow = view.read?.state === "kept" && (view.read.data.reads ?? []).some((r) => OWN_READS.includes(r.operation) && saysAge(r));

  return (
    <Card stale={kept}>
      <div className="flex flex-col gap-2 px-5 pt-5">
        <span className="flex flex-wrap items-baseline gap-x-3.5 gap-y-1">
          <span className="type-card-title">{label}</span>
          <SystemStatus view={view} state={state} />
        </span>
        <Steps reached={reached} state={state} />
      </div>

      <div className="flex flex-col gap-2 px-5 pb-[22px] pt-[18px]">
        <h2 className="font-display text-heading font-semibold tracking-[-0.01em] [text-wrap:pretty]">
          {headline}
        </h2>
        <p className="measure text-body leading-[1.55] text-muted [text-wrap:pretty]">{body}</p>
        {aside && <span className="text-small text-faint">{aside}</span>}
        {/* Why it couldn't be read, in the member's one sentence (design §13.1), with Read again; a cooldown counts down. */}
        {view.read && (notRead(view.read) || (kept && !datedBelow)) && (
          <ReadFreshness read={view.read} subject={label} system={label} reader={MEMBER} onRefresh={again} />
        )}
        {state === "none" && (
          <ArrowLink href="/requests?ask" className="mt-1 self-start">
            Ask for access in Requests
          </ArrowLink>
        )}
      </div>

      {passwordStep && (
        <div className="mx-5 mb-[22px] flex flex-col gap-3">
          {passwordBlocked ? (
            <Blocked view={view} label={label} />
          ) : (
            <PasswordForm view={view} label={label} accent submitLabel={state === "restored" ? "Set new password" : "Set password"} />
          )}
        </div>
      )}

      <Unfold open={Boolean(notice) && !hasAccount}>
        {notice && (
          <Notice
            tone="warn"
            title={notice.title}
            className="mx-5 mb-[22px]"
            {...(notice.person ? { next: PERSON } : { nothingToDo: PAUSE_ENDS })}
          >
            {notice.body}
          </Notice>
        )}
      </Unfold>

      {hasAccount && view.account && (
        <>
          <div className="row-divider flex flex-col gap-2 px-5 py-[18px]">
            <span className="type-label text-label">Sign in as</span>
            <div className="max-w-(--field-max)">
              <CopyableValue value={view.account.username} label="Your account name" />
            </div>
          </div>

          {/* Nothing read: the line above says why once; no folders, no addresses, no usage. */}
          {!notRead(view.read) && (
          <Folders
            view={view}
            state={state}
            label={label}
            roleName={roleName}
            roles={roles}
            passwordBlocked={passwordBlocked}
          />
          )}

          <Unfold open={Boolean(notice)}>
            {notice && (
              <Notice
                tone="warn"
                title={notice.title}
                className="mx-5 mb-5"
                {...(notice.person ? { next: PERSON } : { nothingToDo: PAUSE_ENDS })}
              >
                {notice.body}
              </Notice>
            )}
          </Unfold>

          {!passwordStep && (
            <div className="row-divider flex flex-col gap-2 px-5 pb-[22px] pt-[18px]">
              <span className="type-label text-label">Storage password</span>
              {passwordBlocked ? (
                <Blocked view={view} label={label} />
              ) : (
                <PasswordForm view={view} label={label} submitLabel="Change password" />
              )}
            </div>
          )}
        </>
      )}

      {!hasAccount && (view.suspended ?? []).filter((s) => !isHold(view, s)).length > 0 && (
        <div className="px-5 pb-5">
          <Withheld items={(view.suspended ?? []).filter((s) => !isHold(view, s))} schemaOf={() => view.entitlement_schema} />
        </div>
      )}
      {storage?.quota_read === false && (
        <div className="row-divider px-5 py-4">
          <UsageUnread name={label} some={usageShown(drawn)} reader={MEMBER} />
        </div>
      )}
      {view.read && !notRead(view.read) && <Reads reads={view.read.data.reads} skip={OWN_READS} name={label} reader={MEMBER} />}
    </Card>
  );
}

// Final verification: the pause ends when an operator sets the add-on back to
// active (or a restart does), never by itself, and nothing resumes on its end
// alone: a revocation goes on the background drain, anything else on an
// operator's Send (handlers/my_storage.go Lifecycle). So no "by itself".
const PAUSE_ENDS = "Makerspace staff end the pause; account changes held back wait until then.";
const PERSON = "Need it today? Ask makerspace staff: they can see this pause and how long it should last.";

function copyFor(state: State, label: string, view: MyTargetView): [string, string, React.ReactNode] {
  const hold = (view.suspended ?? []).find((s) => isHold(view, s));
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
            Recorded <Relative iso={view.recorded_at} /> · there&apos;s no set time, it waits on makerspace staff
          </>
        ) : (
          "There's no set time, it waits on makerspace staff"
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
      // One sentence for both outages (design §13.1): which box is down is not the member's to fix.
      return [
        "Couldn't check your account just now",
        notRead(view.read)
          ? "Your access hasn't changed."
          : `${label} didn't say whether your account works. Your access hasn't changed; this page reads it again on your next visit.`,
        "",
      ];
    case "ready":
      return [
        "Your account is ready",
        (view.suspended ?? []).length > 0
            ? `${ready} Some of your access is held back for now, with the reason beside it.`
            : ready,
        "",
      ];
  }
}

function pausedNotice(view: MyTargetView, label: string): { title: string; body: string; person: boolean } | null {
  if (!view.account && paused(view) && view.entitled) {
    return {
      title: "Your account is waiting on makerspace staff, not on you",
      body: `Makerspace staff are working on ${label}, so Syndra has paused creating accounts, yours included. ${label} itself is online. Your account is first in line, and nothing has been lost.`,
      person: true,
    };
  }
  if (!view.account) return null;
  if (view.lifecycle === "draining") {
    return {
      title: "Account changes are paused while the last few finish",
      body: `${label} is online and your access hasn't changed, so you can connect and use your files now. Syndra is holding off on new account changes while the last few finish, so makerspace staff can work on ${label}.`,
      person: false,
    };
  }
  if (view.lifecycle === "read_only") {
    return {
      title: `Account changes are paused while makerspace staff work on ${label}`,
      body: `${label} is online and your access hasn't changed, so you can connect and use your files now. Syndra has stopped changing accounts until makerspace staff are done.`,
      person: true,
    };
  }
  return null;
}

function blockedSentence(view: MyTargetView, label: string) {
  if (down(view)) {
    return `You can't change your password while ${label} isn't answering. Your current one keeps working, and you can change it once ${label} answers.`;
  }
  return "You can't change your password while account changes are paused. Your current one keeps working, and you can change it once the pause lifts.";
}

/** The form's place while it can't be used: one sentence, or, with a change in doubt, the two as one line (A-13 §4). */
function Blocked({ view, label }: { view: MyTargetView; label: string }) {
  if (view.password_change) return <UnconfirmedChange view={view} label={label} blocked />;
  return <span className="measure text-body leading-[1.55] text-ink">{blockedSentence(view, label)}</span>;
}

/** The status beside the system's name: what the monitor can stand behind, and nothing when it can't tell. */
function SystemStatus({ view, state }: { view: MyTargetView; state: State }) {
  if (state === "none" || view.health === "checking") return null;
  const offline = down(view);
  const held = state === "held";
  const checked = view.checked_at ? (
    <>
      {" "}
      · checked <Relative iso={view.checked_at} />
    </>
  ) : null;
  return (
    <span role="status" className={`tint-in inline-flex items-center gap-2 text-small leading-[1.4] motion-tint ${offline ? "text-warn-text" : "text-muted"}`}>
      <StatusDot tone={offline ? "warn" : held ? "neutral" : "healthy"} />
      {offline ? (
        // One word for both outages to a member (design §13.1).
        <>{SYSTEM_WORDS.notAnswering}{view.since ? ` since ${formatClock(view.since)}` : ""}</>
      ) : held ? (
        <>{SYSTEM_WORDS.answering} · your account is on hold</>
      ) : state === "waiting" || state === "unread" || PASSWORD_STEP.has(state) ? (
        <>{SYSTEM_WORDS.answering}{checked}</>
      ) : (
        <>{SYSTEM_WORDS.answering} and accepting sign-ins{checked}</>
      )}
    </span>
  );
}

const STEPS = ["A role gives you access", "Your account is created", "You set a password"];

/** The three steps, one muted line under the title: a filled dot is done, a ring is not yet. */
function Steps({ reached, state }: { reached: number; state: State }) {
  const inert = state === "none" || state === "held";
  const tone = (i: number) => (i <= reached ? "done" : i === reached + 1 && !inert ? "now" : "later");
  return (
    <ol aria-label="Your progress" className="m-0 flex list-none flex-wrap items-center gap-x-5 gap-y-1 p-0 text-small text-muted">
      {STEPS.map((step, i) => {
        const t = tone(i);
        return (
          <li key={step} className="inline-flex items-center gap-2">
            <StatusDot tone={t === "done" ? "healthy" : t === "now" ? "accent" : "neutral"} ring={t === "later"} />
            <span className={t === "now" ? "font-semibold text-ink" : ""}>{step}</span>
          </li>
        );
      })}
    </ol>
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

  const storage = storageOf(view);
  const locked = PASSWORD_STEP.has(state);
  const withheld = (view.suspended ?? []).filter((s) => !isHold(view, s));
  const host = view.connection?.host;
  // Nothing is pinned: the home folder sorts by its name like the rest.
  const places = sortByName(storage?.places ?? [], placeName, (p) => p.path);
  const guideFolders: GuideFolder[] = places
    .filter((p) => p.smb_address && p.unc_address)
    .map((p) => ({
      label: placeName(p),
      share: p.share,
      folder: p.folder ?? "",
      smb: p.smb_address!,
      unc: p.unc_address!,
    }));
  const guide = !!host && guideFolders.length > 0;

  return (
    <div className="row-divider flex flex-col gap-3.5 px-5 pb-5 pt-[18px]">
      <div className="flex flex-wrap items-baseline gap-x-3.5 gap-y-1">
        <span className="type-label text-label">Your folders</span>
        {guide && (
          <Button variant="outline" size="sm" className="ml-auto" onClick={() => setGuideOpen(true)}>
            First time? How to connect
          </Button>
        )}
      </div>

      {state === "unread" || (storage && !storage.places_readable) ? (
        <p className="text-body text-muted">
          {label} couldn&apos;t say which folders are open to you just now. Your access hasn&apos;t changed; this
          page reads it again on your next visit.
        </p>
      ) : state === "opens-nothing" ? (
        <EmptyState
          title="No folder is open to you yet."
          guidance={`${
            roles.length > 0
              ? `Your ${formatList(roles)} ${roles.length === 1 ? "role reaches" : "roles reach"} ${label}, but no folder there is open to ${roles.length === 1 ? "it" : "them"}.`
              : `Your access reaches ${label}, but no folder there is open to it.`
          } Makerspace staff open folders on ${label} itself; this list fills in by itself when they do.`}
        />
      ) : (
        <NotOpenYet
          open={!locked}
          title={state === "restored" ? "Set a new password to open these" : "Set your password to open these"}
          why={`Until then, ${label} turns every sign-in away.`}
          step={passwordBlocked ? undefined : { label: "Set it above ↑", href: `#${passwordFieldId(view)}` }}
        >
          <div className="flex flex-col">
            {places.map((place) => (
              <PlaceBlock
                key={place.path}
                place={place}
                showAddresses={!!host}
                homeKept={!!place.home && !!view.home_kept}
                roleName={roleName}
                month={place.home ? <HomeMonth view={view} /> : null}
              />
            ))}
          </div>
        </NotOpenYet>
      )}

      {withheld.length > 0 && <Withheld items={withheld} schemaOf={() => view.entitlement_schema} />}

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

/** What a place is called on screen, and what it sorts by. */
const placeName = (place: StoragePlace) => (place.home ? "Your home folder" : nameOf(place));

function PlaceBlock({
  place,
  showAddresses,
  homeKept,
  roleName,
  month,
}: {
  place: StoragePlace;
  showAddresses: boolean;
  homeKept: boolean;
  roleName: (r: RoleSource) => string;
  /** The home folder's month (A-23 R1): under its meter, before the addresses. */
  month?: React.ReactNode;
}) {
  const can = place.access === "change";
  const roles = (place.roles ?? []).map(roleName);
  const where = place.home
    ? `In ${[place.share, ...(place.folder ?? "").split("/").slice(0, -1)].join(" › ")} · private to you`
    : place.folder
      ? `In ${[place.share, ...place.folder.split("/").slice(0, -1)].join(" › ")}${roles.length ? ` · from your ${formatList(roles)} ${roles.length === 1 ? "role" : "roles"}` : ""}`
      : roles.length
        ? `From your ${formatList(roles)} ${roles.length === 1 ? "role" : "roles"}`
        : "";

  const usage = drawnUsage(place);
  const quota = usage?.quota_bytes ?? 0;
  // At or over its limit TrueNAS refuses new writes and keeps every file, so
  // the member can still open and delete (a limit set below what they hold).
  const over = Boolean(usage && quota > 0 && usage.used_bytes >= quota);
  let use: string | null = null;
  if (can) {
    if (usage) {
      use = !usage.used_bytes
        ? "Nothing saved here yet"
        : quota > 0
          ? `${formatBytes(usage.used_bytes)} of ${formatBytes(quota)}`
          : `${formatBytes(usage.used_bytes)} used · no limit set`;
    } else if (place.not_counted) use = PLACE_NOT_COUNTED[place.not_counted] ?? null;
  }
  // The meter draws when there is a limit to read against; how much is left is said under it (A-24 Q18).
  const metered = can && usage && quota > 0 ? usage : null;
  const room = metered ? roomLine(metered.used_bytes, quota) : null;

  return (
    <div className="row-divider flex flex-col gap-2.5 py-3.5 first:border-t-0 first:pt-0 last:pb-0">
      <div className="flex flex-wrap items-baseline gap-x-3.5 gap-y-1">
        <span className="flex min-w-0 flex-[1_1_260px] flex-col gap-0.5">
          <span className="flex flex-wrap items-baseline gap-x-2.5 gap-y-0.5">
            <span className="text-body font-semibold [overflow-wrap:anywhere]">
              {placeName(place)}
            </span>
            <span className="whitespace-nowrap text-small text-muted">{can ? "Can change files" : "Read only"}</span>
          </span>
          {where && <span className="text-small text-faint [text-wrap:pretty]">{where}</span>}
        </span>
        {use && <span className="ml-auto text-small text-muted">{use}</span>}
      </div>
      {can && over && (
        <span className="measure text-body leading-[1.55] text-warn-text [text-wrap:pretty]">
          Full. You can open and delete files here, but not save new ones until you&apos;re under {formatBytes(quota)}.
        </span>
      )}
      {homeKept && (
        <span className="measure text-body leading-[1.55] text-ink [text-wrap:pretty]">
          None of the roles you hold gives a home folder any more. It stays, with your files in it, and only you can
          open it. Syndra never moves or deletes a home folder.
        </span>
      )}
      {metered && (
        <Meter value={metered.used_bytes} limit={quota} label={`${placeName(place)} usage`} valueText={use ?? undefined} />
      )}
      {room && <span className="text-small text-muted">{room}</span>}
      {month}
      {showAddresses && place.smb_address && place.unc_address && (
        <div className="grid grid-cols-[repeat(auto-fit,minmax(min(100%,300px),1fr))] gap-x-3.5 gap-y-2.5">
          <div className="flex min-w-0 flex-col gap-[5px]">
            <span className="text-fine text-muted">Mac and Linux · Connect to Server</span>
            <CopyableValue value={place.smb_address} label="Address for Mac and Linux" />
          </div>
          <div className="flex min-w-0 flex-col gap-[5px]">
            <span className="text-fine text-muted">Windows · File Explorer address bar</span>
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
 * database. The field keeps what was typed, masked and in this page's memory
 * only, until the add-on confirms it, so a lost reply can be sent again at
 * once and, if that is lost too, by one more press (A-13 §4).
 */
/** One per system: two cards' fields shared an id, so "Set it above" jumped to the first. */
function passwordFieldId(view: MyTargetView): string {
  return `storage-password-${view.target}`;
}

/** When no confirmed answer has come this long after the press, one line says to keep the page open. */
export const SLOW_ANSWER_MS = 4000;

/**
 * Step 3: back later, the change still unconfirmed. One line above the form,
 * amber dot, no box, true either way; it clears itself on the next confirmed
 * change. While the system isn't answering it is also the blocked sentence.
 */
function UnconfirmedChange({ view, label, blocked }: { view: MyTargetView; label: string; blocked?: boolean }) {
  return (
    <Result
      tone="warn"
      title={`Your password change ${formatAt(view.password_change?.at)} may not have reached ${label}.`}
      body={
        blocked
          ? `You can set it again here once ${label} answers. If your new password doesn't work then, your old one still does.`
          : "If your new password doesn't work, your old one still does. Set it again below to be sure."
      }
    />
  );
}

/** What happened to a password change: one shape for done, unanswered and refused. A dot, the fact, then what it means. */
function Result({
  tone,
  title,
  body,
  glow = false,
  alert = false,
}: {
  tone: "healthy" | "warn" | "danger";
  title: string;
  body?: string;
  glow?: boolean;
  alert?: boolean;
}) {
  return (
    <div role={alert ? "alert" : "status"} className={`${glow ? "lift-in " : ""}isolate flex measure items-start gap-2.5`}>
      <StatusDot tone={tone} glow={glow} className="mt-2" />
      <span className="flex min-w-0 flex-col gap-1">
        <span className="text-body font-semibold leading-[1.55] text-ink [text-wrap:pretty]">{title}</span>
        {body && <span className="text-body leading-[1.55] text-muted [text-wrap:pretty]">{body}</span>}
      </span>
    </div>
  );
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
  // Said while the page waits to send it again: never "done" before Syndra knows.
  const [retrying, setRetrying] = useState<RetryState | null>(null);
  const set = useSetStorageCredential(view.target, setRetrying);
  const status = set.data?.status;
  const changed = status === "set";
  const noAnswer = status === "unconfirmed";
  const refused = status === "rejected";
  // Step 3's line, until this visit's own answer speaks for the change.
  const returned = Boolean(view.password_change) && !set.data && !set.isPending;
  const slow = useLongWait(set.isPending, SLOW_ANSWER_MS);
  const scope = `Only for ${label}: not your Syndra sign-in or your Google account.`;

  return (
    <form
      className="flex flex-col gap-2"
      onSubmit={(e) => {
        e.preventDefault();
        setRetrying(null);
        // The field clears only on a confirmed done: otherwise it still holds
        // what was typed, so one press sends it again.
        set.mutate(oneShot(password), {
          onSuccess: (answer) => {
            if (answer.status === "set") setPassword("");
          },
        });
      }}
    >
      <Unfold open={returned}>
        <div className="pb-1">
          <UnconfirmedChange view={view} label={label} />
        </div>
      </Unfold>
      <div className="flex max-w-(--field-max) flex-col gap-1.5">
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
            {/* In progress across both sends: the dot breathes in the padding, the label and width hold (A-08). */}
            <Button type="submit" variant={accent ? "accent" : "outline"} disabled={!password} isPending={set.isPending}>
              {submitLabel}
            </Button>
          </span>
        </div>
      </div>
      <Unfold open={slow && !retrying}>
        <p role="status" className="measure text-small leading-[1.55] text-muted [text-wrap:pretty]">
          Still waiting for {label}. Keep this page open until it answers.
        </p>
      </Unfold>
      {retrying && set.isPending && (
        // The one retry model's seat (RetryStatus), in the member's voice.
        <RetryStatus retry={retrying} system={label} own />
      )}
      {/* One result slot, in the member's words (A-22 §3.26). While Syndra is
          trying again the seat above is RetryStatus own; once it stops, the
          same sentences come back here. */}
      {noAnswer && !set.isPending && (
        // Step 2: a failure appears still; it never lifts.
        <ActionOutcome
          placement="inline"
          word="Not confirmed yet"
          outcome={{
            kind: "queued",
            sendsItself: true,
            message: set.data?.retry
              ? `${retryLine(set.data.retry, label)} Your password may not have changed.`
              : `${label} didn't answer, so your password may not have changed.`,
            next: `Try again in a minute: press ${submitLabel}. Sending the same password twice is safe, and ${label} then has the one you set.`,
          }}
        />
      )}
      {changed && !set.isPending && (
        // Only a confirmed done says this, on either send (owner, 3 Nov).
        <ActionOutcome
          placement="inline"
          word="Changed"
          outcome={{ kind: "applied", message: `${label} has your new password.` }}
        />
      )}
      {refused && !set.isPending && (
        <ActionOutcome
          placement="inline"
          word="Not changed"
          outcome={{
            kind: "failed",
            message: `${label} refused the change. Nothing was changed.`,
          }}
        />
      )}
      <p className="measure text-small leading-[1.55] text-muted [text-wrap:pretty]">
        {accent ? (
          <>{scope} Syndra passes it to {label} and doesn&apos;t keep a copy.</>
        ) : changed ? (
          <>Last changed just now. {scope}</>
        ) : !noAnswer && !returned && view.credential.last_changed_at ? (
          // Gone while the change is in doubt: it may no longer be true.
          <>
            Last changed <Relative iso={view.credential.last_changed_at} />. {scope}
          </>
        ) : (
          scope
        )}
      </p>
      {set.error && (
        // Nothing went out (503 NOTHING_SENT, design §13.4): the backend's sentence ends on its step.
        isNothingSent(set.error) ? (
          <ActionOutcome placement="inline" word={NOTHING_SENT_WORD} outcome={nothingSentOutcome(set.error)} />
        ) : (
          <ActionOutcome
            placement="inline"
            word="Not changed"
            outcome={{ kind: "failed", message: sentence(describeFailure(set.error)) }}
          />
        )
      )}
    </form>
  );
}
