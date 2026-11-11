"use client";

import type { ReactNode } from "react";

import { UserName } from "@/components/names/UserName";
import { useShownNames } from "@/components/names/shownNames";
import { RowSkeleton } from "@/components/states";
import { SYSTEM_WORDS } from "@/components/targets/TargetReading";
import { AdvancedLink } from "@/components/ui/AdvancedLink";
import { Reads } from "@/components/results/ReadResult";
import { ArrowLink } from "@/components/ui/ArrowLink";
import { StatusDot, type StatusTone } from "@/components/ui/Badge";
import { Card, CardRow } from "@/components/ui/Card";
import { NextStep } from "@/components/ui/NextStep";
import { Notice } from "@/components/ui/Notice";
import { StandingLine } from "@/components/ui/StandingLine";
import { Relative } from "@/components/ui/Time";
import { roleOf } from "@/lib/fields";
import { formatClock, formatList, formatShortDate } from "@/lib/format";
import { isStorage } from "@/lib/nav";
import { useAutoRetry } from "@/lib/queries/useIndicators";
import { usePersonTargets, type PersonTargetView, type RoleSource } from "@/lib/queries/useMyStorage";
import { useTargets, type TargetSummary } from "@/lib/queries/useTargets";
import { sortNames } from "@/lib/sort";

/**
 * A person's connected systems, for operators (addon-contract 3.10 §6): under
 * the projects whose roles reach them, two sections in a fixed order. Network
 * storage is the storage kind's (`components/kinds/storage/PersonStorage`);
 * this file draws every other system with one kind-agnostic card, and holds
 * the parts both share. Not a word of storage in it: it is built from the same
 * core view the member's card is.
 */

/**
 * Which systems a section draws, known before their reads are (the card's name
 * shows while loading, A-10 §4): the person's views once they land, else the
 * roster. Registrations, never data: a section with none is not drawn.
 */
export function useSectionSystems(userId: string, pick: (s: { kind?: string }) => boolean) {
  const views = usePersonTargets(userId);
  const roster = useTargets();
  const systems: Pick<TargetSummary, "target" | "name">[] = (views.data ?? roster.data ?? []).filter(pick);
  return { views, systems, data: views.data?.filter(pick) };
}

export function useRoleNames() {
  const shown = useShownNames();
  return (roles: RoleSource[] | undefined) => sortNames((roles ?? []).map((r) => shown.role(r.project_id, r.role_key)));
}

/** A section: a label, an error in its place, a card per system or a skeleton per system. */
export function SystemsSection({
  label,
  userId,
  name,
  pick,
  noun,
  card,
}: {
  label: string;
  userId: string;
  name: string;
  pick: (s: { kind?: string }) => boolean;
  /** What failed to load, said the way the section heading says it. */
  noun: string;
  card: (view: PersonTargetView) => ReactNode;
}) {
  const { views, systems, data } = useSectionSystems(userId, pick);
  if (systems.length === 0) return null;
  return (
    <section aria-label={label} className="flex flex-col gap-3.5">
      <span className="type-label px-5 text-label">{label}</span>
      {views.error ? (
        <Notice
          tone="danger"
          title={`Couldn't load ${name}'s ${noun}${systems.length === 1 ? ` on ${systems[0].name}` : ""}.`}
          action={{ label: "Try again", onClick: () => views.refetch() }}
          next="Press Try again. If it keeps failing, ask whoever runs the Syndra server."
        >
          Syndra didn&apos;t answer when this page asked. Nothing was changed.
        </Notice>
      ) : data ? (
        data.map((view) => card(view))
      ) : (
        systems.map((system) => (
          <Card key={system.target} id={`sys-${system.target}`}>
            <CardTop target={system.target} name={system.name} />
            <RowSkeleton rows={4} avatar={false} label={`Loading ${name}'s ${noun} on ${system.name}`} />
          </Card>
        ))
      )}
    </section>
  );
}

export function CardTop({ target, name, status }: { target: string; name: string; status?: ReactNode }) {
  return (
    <div className="px-5 pb-3 pt-4">
      <span className="flex flex-wrap items-baseline gap-x-3.5 gap-y-1">
        <AdvancedLink href={`/system/targets/${target}`} className="type-card-title text-ink link-hover">
          {name}
        </AdvancedLink>
        {status}
      </span>
    </div>
  );
}

/** Label 170 · value · a 200 seat that holds a link or nothing (the setup card's row). */
export function Row({ label, seat, first = false, children }: { label: string; seat?: ReactNode; first?: boolean; children: ReactNode }) {
  return (
    <CardRow first={first}>
      <span className="w-full text-[13.5px] font-semibold text-muted tablet:w-[170px] tablet:shrink-0">{label}</span>
      <span className="flex min-w-0 flex-1 flex-col gap-0.5 text-[15px] [overflow-wrap:anywhere]">{children}</span>
      <span className="ml-auto flex tablet:w-[200px] tablet:shrink-0 tablet:justify-end">{seat}</span>
    </CardRow>
  );
}

export function Faint({ children, tone }: { children: ReactNode; tone?: "warn" }) {
  return (
    <span className={`text-[13px] leading-[1.5] [text-wrap:pretty] ${tone === "warn" ? "text-warn-text" : "text-faint"}`}>
      {children}
    </span>
  );
}

export function Dotted({ tone, children }: { tone: StatusTone; children: ReactNode }) {
  return (
    <span className="inline-flex items-baseline gap-2 font-semibold">
      <StatusDot tone={tone} />
      {children}
    </span>
  );
}

export function Status({ view }: { view: PersonTargetView }) {
  if (!view.health || view.health === "unknown") return null;
  const offline = view.health === "offline";
  const held = view.holds.some((h) => roleOf(view.entitlement_schema, h.field) === "account");
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
 * Every kind has an offline, so it lives here.
 */
export function offlineNext(label: string, auto: boolean | null): string {
  // Not known yet whether revocations go on their own (N3): no promise either way.
  if (auto === null) {
    return `If you change their access while ${label} is offline, the change stays in Pending changes until it reaches ${label}. Otherwise there is nothing for you to do.`;
  }
  return auto
    ? `If you change their access while ${label} is offline, the change waits until it answers: a revocation then sends on its own, anything else when you press Send on Pending changes. Otherwise there is nothing for you to do.`
    : `If you change their access while ${label} is offline, the change waits in Pending changes; press Send there once it answers. Otherwise there is nothing for you to do.`;
}

/** The offline Notice, above any card's body: when it stopped, then what the card says about it. */
export function OfflineNotice({ view, children }: { view: PersonTargetView; children: ReactNode }) {
  const auto = useAutoRetry();
  if (view.health !== "offline") return null;
  return (
    <div className="px-5 pb-3">
      <Notice tone="warn" live title={`${view.name} is offline`} next={offlineNext(view.name, auto)}>
        {view.reachable_since ? `It stopped answering at ${formatClock(view.reachable_since)}. ` : ""}
        {children}
      </Notice>
    </div>
  );
}

export function HoldRow({ view, seat, page }: { view: PersonTargetView; seat: ReactNode; page: string }) {
  const isAccount = (h: { field: string }) => roleOf(view.entitlement_schema, h.field) === "account";
  const hold = view.holds.find(isAccount) ?? view.holds[0];
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
            {isAccount(hold) ? "Their whole account" : hold.value}, since {formatShortDate(hold.since)}
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

/** "Their Laser operator role reaches Big laser." */
export function reachWords(roles: string[], name: string): string {
  return roles.length
    ? `Their ${formatList(roles)} ${roles.length === 1 ? "role reaches" : "roles reach"} ${name}.`
    : `Their access reaches ${name}.`;
}

/**
 * One system that is not storage: its account, the add-on's own word on it,
 * the roles that reach it and any hold. A system Syndra has not read yet is
 * here too, for operators only, and says so with its next step.
 */
export function PersonSystemCard({ view, userId, name }: { view: PersonTargetView; userId: string; name: string }) {
  const roleNames = useRoleNames();
  const system = view.name;
  const page = `/system/targets/${view.target}?person=${encodeURIComponent(userId)}#accounts`;
  const onPage = <ArrowLink href={page}>Set on {system}&apos;s page</ArrowLink>;
  const roles = roleNames(view.roles);

  let body: ReactNode;
  if (!view.kind) {
    body = (
      <Row label="Account" first seat={<ArrowLink href={`/system/targets/${view.target}`}>Open {system}&apos;s page</ArrowLink>}>
        <span className="text-muted">Not read yet</span>
        <Faint>
          Syndra has not read {system} yet, so it can&apos;t say whether they have an account there. Its page says why.
        </Faint>
      </Row>
    );
  } else if (view.setup) {
    body = (
      <Row label="Account" first seat={<ArrowLink href={`/system/targets/${view.target}#project`}>Finish setting it up</ArrowLink>}>
        <span className="text-muted">Not yet</span>
        <Faint>
          {view.setup === "no_project"
            ? `${system} has no Zitadel project yet, so no role reaches it and nobody has an account there.`
            : `No role is mapped to ${system} yet, so nobody has an account there.`}
        </Faint>
      </Row>
    );
  } else if (!view.account && !view.entitled) {
    body = (
      <Row label="Account" first>
        <span className="text-muted">None</span>
        <Faint>None of their roles reaches {system}.</Faint>
      </Row>
    );
  } else if (!view.account) {
    body = (
      <>
        <Row label="Account" first>
          <NextStep person={userId} />
          <Faint>{reachWords(roles, system)} Syndra makes the account once you send it.</Faint>
        </Row>
        <HoldRow view={view} seat={onPage} page={page} />
      </>
    );
  } else {
    body = (
      <>
        <Row label="Account" first>
          <span className="font-mono text-[14.5px]">{view.account.username}</span>
          {view.standing ? (
            <StandingLine standing={view.standing} system={system} audience="operator" />
          ) : (
            <Faint>Made by Syndra on {formatShortDate(view.account.bound_at)}</Faint>
          )}
        </Row>
        <Row label="Roles">
          <span>{roles.length ? formatList(roles) : "None"}</span>
          <Faint>{roles.length ? reachWords(roles, system) : `None of their roles reaches ${system} any more.`}</Faint>
        </Row>
        <HoldRow view={view} seat={onPage} page={page} />
      </>
    );
  }

  return (
    <Card id={`sys-${view.target}`} className="scroll-mt-4">
      <CardTop target={view.target} name={system} status={<Status view={view} />} />
      <OfflineNotice view={view}>{name}&apos;s access hasn&apos;t changed.</OfflineNotice>
      {body}
      <Reads reads={view.reads} name={system} reader={{ audience: "operator", person: name }} />
    </Card>
  );
}

/** The Connected systems section: every system that is not storage. */
export function PersonSystems({ userId, name }: { userId: string; name: string }) {
  return (
    <SystemsSection
      label="Connected systems"
      noun="connected systems"
      userId={userId}
      name={name}
      pick={(s) => !isStorage(s)}
      card={(view) => <PersonSystemCard key={view.target} view={view} userId={userId} name={name} />}
    />
  );
}
