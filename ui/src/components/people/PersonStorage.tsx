"use client";

import { SYSTEM_WORDS } from "@/components/targets/TargetReading";
import { AdvancedLink } from "@/components/ui/AdvancedLink";
import type { ReactNode } from "react";

import { UserName } from "@/components/names/UserName";
import { RowSkeleton } from "@/components/states";
import { ArrowLink } from "@/components/ui/ArrowLink";
import { StatusDot, type StatusTone } from "@/components/ui/Badge";
import { NextStep } from "@/components/ui/NextStep";
import { Card, CardRow } from "@/components/ui/Card";
import { NotOpenYet } from "@/components/ui/NotOpenYet";
import { Notice } from "@/components/ui/Notice";
import { Relative } from "@/components/ui/Time";
import { Tinted } from "@/lib/motion";
import { formatClock, formatList, formatRelative, formatShortDate } from "@/lib/format";
import { targetLabel } from "@/lib/nav";
import { usePersonTargets, type PersonTargetView, type RoleSource, type StoragePlace } from "@/lib/queries/useMyStorage";

import { useAutoRetry } from "@/lib/queries/useIndicators";
import { useTargets } from "@/lib/queries/useTargets";
import { sortByName, sortNames } from "@/lib/sort";
import { useIsAdvanced } from "@/lib/ui-view";
import { useShownNames } from "@/components/names/shownNames";

/**
 * A person's network storage, for operators (A-10 Part 1): one read-only card
 * per connected system, under the projects on the Access tab.
 *
 * Every value set elsewhere links to its one write place: limits and holds to
 * the person's row on the system's page, folders to its mappings. Nobody but
 * the person sets their password, so that row links nowhere.
 */

const GB = 1024 ** 3;

/** "18.4 GB", "50 GB": the card speaks in the limits' own unit. */
function size(bytes: number): string {
  const n = bytes / GB;
  return `${Number.isInteger(n) ? n : n.toFixed(1)} GB`;
}

function gbWords(bytes: number | undefined): string {
  return bytes ? size(bytes) : "no limit";
}

function useRoleNames() {
  const shown = useShownNames();
  return (roles: RoleSource[] | undefined) => sortNames((roles ?? []).map((r) => shown.role(r.project_id, r.role_key)));
}

/** The person's storage section: a label, then one card per system. */
export function PersonStorage({ userId, name }: { userId: string; name: string }) {
  const views = usePersonTargets(userId);
  // The systems are known before their reads are: the card's name and caption
  // show while loading (A-10 §4).
  const roster = useTargets();
  const systems = views.data?.map((v) => v.target) ?? roster.data?.map((t) => t.target) ?? [];

  return (
    <section aria-label="Network storage" className="flex flex-col gap-3.5">
      <span className="type-label px-5 text-label">Network storage</span>
      {views.error ? (
        <Notice
          tone="danger"
          title={`Couldn't load ${name}'s storage${systems.length === 1 ? ` on ${targetLabel(systems[0])}` : ""}.`}
          action={{ label: "Try again", onClick: () => views.refetch() }}
          next="Press Try again. If it keeps failing, ask whoever runs the Syndra server."
        >
          Syndra didn&apos;t answer when this page asked. Nothing was changed.
        </Notice>
      ) : views.data ? (
        views.data.map((view) => <SystemCard key={view.target} view={view} userId={userId} name={name} />)
      ) : (
        systems.map((target) => (
          <Card key={target} id={`sys-${target}`}>
            <CardTop target={target} />
            <RowSkeleton rows={4} avatar={false} label={`Loading ${name}'s storage on ${targetLabel(target)}`} />
          </Card>
        ))
      )}
    </section>
  );
}

function CardTop({ target, status }: { target: string; status?: ReactNode }) {
  return (
    <div className="px-5 pb-3 pt-4">
      <span className="flex flex-wrap items-baseline gap-x-3.5 gap-y-1">
        <AdvancedLink href={`/system/targets/${target}`} className="type-card-title text-ink link-hover">
          {targetLabel(target)}
        </AdvancedLink>
        {status}
      </span>
    </div>
  );
}

/** Label 170 · value · a 200 seat that holds a link or nothing (the setup card's row). */
function Row({ label, seat, first = false, children }: { label: string; seat?: ReactNode; first?: boolean; children: ReactNode }) {
  return (
    <CardRow first={first}>
      <span className="w-full text-[13.5px] font-semibold text-muted tablet:w-[170px] tablet:shrink-0">{label}</span>
      <span className="flex min-w-0 flex-1 flex-col gap-0.5 text-[15px] [overflow-wrap:anywhere]">{children}</span>
      <span className="ml-auto flex tablet:w-[200px] tablet:shrink-0 tablet:justify-end">{seat}</span>
    </CardRow>
  );
}

function Faint({ children, tone }: { children: ReactNode; tone?: "warn" }) {
  return (
    <span className={`text-[13px] leading-[1.5] [text-wrap:pretty] ${tone === "warn" ? "text-warn-text" : "text-faint"}`}>
      {children}
    </span>
  );
}

function Dotted({ tone, children }: { tone: StatusTone; children: ReactNode }) {
  return (
    <span className="inline-flex items-baseline gap-2 font-semibold">
      <StatusDot tone={tone} />
      {children}
    </span>
  );
}

function Status({ view }: { view: PersonTargetView }) {
  if (!view.health || view.health === "unknown") return null;
  const offline = view.health === "offline";
  const held = view.holds.some((h) => h.field === "enabled");
  return (
    <span role="status" className={`inline-flex items-center gap-2 text-[13px] ${offline ? "text-warn-text" : "text-muted"}`}>
      <StatusDot tone={offline ? "warn" : held ? "neutral" : "healthy"} />
      {offline ? (
        <>
          {SYSTEM_WORDS.offline}{view.reachable_since ? ` since ${formatClock(view.reachable_since)}` : ""}
          {view.last_known ? ` · last read ${formatClock(view.last_known.read_at)}` : ""}
        </>
      ) : held ? (
        <>{SYSTEM_WORDS.answering} · their account is on hold</>
      ) : (
        <>
          {SYSTEM_WORDS.answering}{view.checked_at ? <> · read <Relative iso={view.checked_at} /></> : null}
        </>
      )}
    </span>
  );
}

/**
 * The step while a system is offline. Syndra changes nothing there while it is
 * unreachable (the drain halts, propagation/addon.go:81 and revocations.go:92)
 * and every change for it stays queued. Once it answers, revocations go on
 * their own, unless the background drain is off (`auto` false); every other
 * change goes when someone presses Send. Only a person who changed their access
 * meanwhile has anything to do, so it says so rather than promising nothing.
 */
export function offlineNext(label: string, auto: boolean): string {
  return auto
    ? `If you change their access while ${label} is offline, the change waits until it answers: a revocation then sends on its own, anything else when you press Send on Pending changes. Otherwise there is nothing for you to do.`
    : `If you change their access while ${label} is offline, the change waits in Pending changes; press Send there once it answers. Otherwise there is nothing for you to do.`;
}

function SystemCard({ view, userId, name }: { view: PersonTargetView; userId: string; name: string }) {
  const label = targetLabel(view.target);
  const roleNames = useRoleNames();
  const auto = useAutoRetry();
  const offline = view.health === "offline";
  const storage = view.storage ?? view.last_known?.storage;
  // Stale (A-13 d): a card drawn from the last read recedes to the page; its
  // text stays at full strength.
  const stale = offline && !view.storage && Boolean(view.last_known);
  const page = `/system/targets/${view.target}?person=${encodeURIComponent(userId)}#accounts`;
  const onPage = <ArrowLink href={page}>Set on {label}&apos;s page</ArrowLink>;
  const onMappings = <ArrowLink href={`/system/targets/${view.target}/mappings`}>Set in {label}&apos;s mappings</ArrowLink>;
  const roles = roleNames(view.roles);
  const rolesWord = (n: number) => (n === 1 ? "role reaches" : "roles reach");

  let body: ReactNode;
  if (view.setup) {
    body = (
      <div className="px-5 pb-5 pt-1">
        <NotOpenYet
          open={false}
          title={`Opens once ${label} is set up`}
          why={
            view.setup === "no_project"
              ? `${label} has no Zitadel project yet, so no role reaches it and nobody has an account there.`
              : `No role is mapped to ${label} yet, so nobody has an account there.`
          }
          step={{ label: "Finish setting it up →", href: `/system/targets/${view.target}#project` }}
        >
          <div className="flex flex-col">
            {["Account", "Password", "Home folder", "Folders", "Hold"].map((l, i) => (
              <Row key={l} label={l} first={i === 0}>
                <span className="text-muted">Not yet</span>
              </Row>
            ))}
          </div>
        </NotOpenYet>
      </div>
    );
  } else if (!view.account && !view.entitled) {
    body = (
      <Row label="Account" first>
        <span className="text-muted">None</span>
        <Faint>None of their roles reaches {label}.</Faint>
      </Row>
    );
  } else if (!view.account) {
    body = (
      <>
        <Row label="Account" first>
          <NextStep person={userId} />
          <Faint>
            {roles.length
              ? `Their ${formatList(roles)} ${rolesWord(roles.length)} ${label}.`
              : `Their access reaches ${label}.`}{" "}
            Syndra makes the account once you send it.
          </Faint>
        </Row>
        <div className="row-divider px-5 py-4">
          <NotOpenYet
            open={false}
            title="Opens once their account is made"
            why="Password, home folder and folders are read from the account itself. This is what they'll get."
          >
            <div className="flex flex-col">
              <Row label="Password" first>
                <span>Not set yet</span>
              </Row>
              <Row label="Home folder">
                <LimitLines view={view} />
              </Row>
              <Row label="Folders">
                <FolderList places={view.preview ?? []} roleNames={roleNames} />
              </Row>
            </div>
          </NotOpenYet>
        </div>
        <HoldRow view={view} seat={onPage} page={page} />
      </>
    );
  } else {
    const readable = Boolean(storage);
    const noAnswer = "didn't answer when this page asked. This page reads it again on your next visit.";
    // Offline with no last read: the Notice alone, and these two rows aren't drawn.
    const drawRead = !(offline && !readable);
    const folders = (storage?.places ?? []).filter((p) => !p.home);
    body = (
      <>
        <Row label="Account" first>
          <span className="font-mono text-[14.5px]">{view.account.username}</span>
          <Faint>
            {view.adopted ? (
              <>
                Made by hand on {label} before Syndra managed it.{" "}
                <UserName id={view.adopted.actor_id} className="text-ink" />{" "}
                assigned it to {name} on {formatShortDate(view.adopted.at)}, with everything it held.
              </>
            ) : (
              <>Made by Syndra on {formatShortDate(view.account.bound_at)}</>
            )}
          </Faint>
        </Row>
        <PasswordRow view={view} name={name} storage={storage} />
        {drawRead && (
          <Row label="Home folder" seat={onPage}>
            {!readable ? (
              <span className="text-[14px] text-muted">
                {label} {noAnswer}
              </span>
            ) : (
              <HomeUse view={view} />
            )}
          </Row>
        )}
        {drawRead && (
          <Row label="Folders" seat={onMappings}>
            {!readable || storage?.places_readable === false ? (
              <span className="text-[14px] text-muted">
                {label} {noAnswer}
              </span>
            ) : folders.length === 0 ? (
              <>
                <span>None yet</span>
                <Faint>
                  {roles.length
                    ? `Their ${formatList(roles)} ${rolesWord(roles.length)} ${label}, but no folder there is open to ${roles.length === 1 ? "it" : "them"}.`
                    : `Their access reaches ${label}, but no folder there is open to it.`}{" "}
                  Folders open to a group on {label} itself; this fills in when one does.
                </Faint>
              </>
            ) : (
              <>
                {view.holds.length > 0 && <Faint>None of these opens while the hold is on.</Faint>}
                <FolderList places={folders} roleNames={roleNames} />
              </>
            )}
          </Row>
        )}
        <HoldRow view={view} seat={onPage} page={page} />
      </>
    );
  }

  return (
    <Card id={`sys-${view.target}`} className={`scroll-mt-4 ${stale ? "!border-line-strong !bg-canvas !shadow-none motion-tint" : ""}`}>
      <CardTop target={view.target} status={<Status view={view} />} />
      {offline && (
        <div className="px-5 pb-3">
          <Notice tone="warn" live title={`${label} is offline`} next={offlineNext(label, auto)}>
            {view.reachable_since ? `It stopped answering at ${formatClock(view.reachable_since)}. ` : ""}
            {view.last_known
              ? `Home folder and Folders below are what it said at ${formatClock(view.last_known.read_at)}, ${formatRelative(view.last_known.read_at)}. `
              : ""}
            {name}&apos;s access hasn&apos;t changed, and their folders open again when it&apos;s back.
          </Notice>
        </div>
      )}
      {body}
    </Card>
  );
}

function PasswordRow({ view, name, storage }: { view: PersonTargetView; name: string; storage: PersonTargetView["storage"] }) {
  const label = targetLabel(view.target);
  const own = `on their Network storage page`;
  if (storage?.needs_new_password) {
    return (
      <Row label="Password">
        <Dotted tone="neutral">Needs a new password</Dotted>
        <Faint>
          {view.restored_at ? `Their access was given back on ${formatShortDate(view.restored_at)}. ` : "Their access was given back. "}
          {label} opens their folders again only once {name} sets a new one {own}. Nobody else can.
        </Faint>
      </Row>
    );
  }
  if (view.credential.needs_re_enrolment) {
    return (
      <Row label="Password">
        <Dotted tone="neutral">Needs a new password</Dotted>
        <Faint>{label} was rebuilt since they set it, and passwords didn&apos;t carry over.</Faint>
      </Row>
    );
  }
  if (view.credential.set) {
    return (
      <Row label="Password">
        <span>
          Set
          {view.credential.last_changed_at ? `, last changed ${formatShortDate(view.credential.last_changed_at)}` : ""}
        </span>
        <Faint>Only {name} sets it, {own}.</Faint>
      </Row>
    );
  }
  // Syndra has no record of one. TrueNAS decides whether it works: never "Not
  // set" for an account that works (an adopted one).
  if (storage ? storage.usable || !storage.needs_password : view.adopted) {
    return (
      <Row label="Password">
        <span>Set before Syndra managed this account</span>
        <Faint>
          {label} accepts it, so it works. Syndra has no date for it, and only {name} can change it.
        </Faint>
      </Row>
    );
  }
  return (
    <Row label="Password">
      <span>Not set yet</span>
      <Faint>
        Their account turns every sign-in away until {name} sets one {own}.
      </Faint>
    </Row>
  );
}

/** The limit in force first, in ink; the one it replaces under it, faint and in words. */
function LimitLines({ view }: { view: PersonTargetView }) {
  const roleNames = useRoleNames();
  const home = view.home;
  if (!home || (!home.gives_home && !home.own)) {
    return (
      <>
        <span className="text-muted">None</span>
        <Faint>None of their roles gives a home folder.</Faint>
      </>
    );
  }
  const from = roleNames(home.roles_limit_from ? [home.roles_limit_from] : [])[0];
  const rolesWords = home.roles_limit === undefined ? "No limit" : `Up to ${gbWords(home.roles_limit)}`;
  if (!home.own) {
    return (
      <>
        <Faint>
          {rolesWords}
          {from ? `, from their ${from} role` : ""}
        </Faint>
        {home.own_ended && home.roles_limit !== undefined && (
          <Faint tone="warn">
            Their own {gbWords(home.own_ended.bytes)} ended on {formatShortDate(home.own_ended.ended_at)} with{" "}
            {size(home.used_bytes ?? 0)} in their folder. They can open and delete files but can&apos;t save new
            ones until they&apos;re under {gbWords(home.roles_limit)}. Nothing was deleted.
          </Faint>
        )}
      </>
    );
  }
  const own = home.own;
  return (
    <>
      {own.review_due ? (
        <span className="inline-flex items-baseline gap-2 text-[13.5px] text-warn-text [text-wrap:pretty]">
          <StatusDot tone="warn" />
          Review was due {formatShortDate(own.review_date)}. It stays at {gbWords(own.bytes)} until you keep it or end it.
        </span>
      ) : (
        <span className="text-[13.5px] text-ink [text-wrap:pretty]">
          {own.bytes === 0 ? "No limit" : `Up to ${gbWords(own.bytes)}`}, their own ·{" "}
          {own.expires_at ? `ends ${formatShortDate(own.expires_at)}` : `review on ${formatShortDate(own.review_date)}`} · “
          {own.reason}”
        </span>
      )}
      {from && (
        <Faint>
          In place of the {home.roles_limit === undefined ? "no limit" : gbWords(home.roles_limit)} their {from} role gives.
        </Faint>
      )}
    </>
  );
}

function HomeUse({ view }: { view: PersonTargetView }) {
  const home = view.home;
  const label = targetLabel(view.target);
  const used = home?.used_bytes;
  const quota = home?.quota_bytes ?? 0;
  const hasHome = Boolean(home?.home);
  const kept = hasHome && home && !home.gives_home && !home.own;
  if (!hasHome && !(home?.gives_home || home?.own)) return <LimitLines view={view} />;
  const pct = used !== undefined && quota > 0 ? Math.min(100, (used / quota) * 100) : 0;
  const full = pct >= 90;
  const over = used !== undefined && quota > 0 && used >= quota;
  return (
    <>
      {used === undefined ? (
        <span className="text-[14px] text-muted">{label} couldn&apos;t say how much is used just now</span>
      ) : over ? (
        <span className="text-warn-text [text-wrap:pretty]">
          Full. They can open and delete files here, but can&apos;t save new ones until they&apos;re under {size(quota)}.
        </span>
      ) : (
        <span>
          {quota > 0 ? `${size(used)} of ${size(quota)}` : `${size(used)} used · no limit set`}
          {full && <span className="text-warn-text"> · nearly full</span>}
        </span>
      )}
      {used !== undefined && quota > 0 && (
        <span aria-hidden className="my-1 block h-[5px] w-full max-w-[360px] overflow-hidden rounded-pill bg-line">
          <span className={`block h-full rounded-pill ${full ? "bg-warn" : "bg-muted"}`} style={{ width: `${Math.max(pct, 1)}%` }} />
        </span>
      )}
      {kept ? (
        <Faint>No role of theirs gives a home folder any more. It stays, with their files, and only they can open it.</Faint>
      ) : (
        <LimitLines view={view} />
      )}
    </>
  );
}

function folderName(place: StoragePlace) {
  if (!place.folder) return place.share;
  const parts = place.folder.split("/");
  return parts[parts.length - 1];
}

function FolderList({ places, roleNames }: { places: StoragePlace[]; roleNames: (r: RoleSource[] | undefined) => string[] }) {
  const advanced = useIsAdvanced();
  if (places.length === 0) return <span className="text-muted">None yet</span>;
  return (
    <ul className="m-0 flex list-none flex-col gap-1.5 p-0">
      {sortByName(places, folderName, (p) => p.path).map((p) => {
        const roles = roleNames(p.roles);
        return (
          <li key={p.path} className="flex flex-col">
            <span className="flex flex-wrap items-baseline gap-x-2.5 gap-y-0.5">
              <span className="font-semibold [overflow-wrap:anywhere]">{folderName(p)}</span>
              <span className="text-[13px] text-muted">{p.access === "change" ? "Can change files" : "Read only"}</span>
              {roles.length > 0 && <span className="text-[13px] text-faint">from {formatList(roles)}</span>}
            </span>
            <Tinted show={advanced}>
              <span className="block font-mono text-[12.5px] text-faint [overflow-wrap:anywhere]">
                {[p.share, p.folder].filter(Boolean).join("/")}
              </span>
            </Tinted>
          </li>
        );
      })}
    </ul>
  );
}

function HoldRow({ view, seat, page }: { view: PersonTargetView; seat: ReactNode; page: string }) {
  const hold = view.holds.find((h) => h.field === "enabled") ?? view.holds[0];
  return (
    <Row label="Hold" seat={seat}>
      {!hold ? (
        <>
          <span className="text-muted">None</span>
          {view.restored_at && <Faint>The last one ended on {formatShortDate(view.restored_at)}</Faint>}
        </>
      ) : (
        <>
          <Dotted tone="neutral">
            {hold.field === "enabled" ? "Their whole account" : hold.value}, since {formatShortDate(hold.since)}
          </Dotted>
          <Faint>
            “{hold.reason}” · put on hold by{" "}
            <UserName id={hold.actor_id} className="text-ink" />
            {hold.expires_at
              ? ` · ends ${formatShortDate(hold.expires_at)}`
              : hold.review_date
                ? ` · review on ${formatShortDate(hold.review_date)}`
                : ""}
            {view.holds.length > 1 && (
              <>
                {" · "}
                <AdvancedLink href={page} className="font-semibold text-accent-text link-hover">
                  and {view.holds.length - 1} more
                </AdvancedLink>
              </>
            )}
          </Faint>
        </>
      )}
    </Row>
  );
}
