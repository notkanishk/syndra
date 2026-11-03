"use client";

import Link from "next/link";
import { useState, type ReactNode } from "react";

import { ErrorState, RowSkeleton } from "@/components/states";
import { ArrowLink } from "@/components/ui/ArrowLink";
import { Mono, StatusDot, type StatusTone } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Card, CardColumns, CardHeader } from "@/components/ui/Card";
import { Notice } from "@/components/ui/Notice";
import { Unfold } from "@/components/ui/Unfold";
import { LANDS_LABEL, outcomePhrase } from "@/lib/claims";
import { formatClock, formatList, formatSignIn, formatWhen } from "@/lib/format";
import {
  useClaimsDelivery,
  type ClaimApp,
  type ClaimFinding,
  type ClaimsDelivery as Delivery,
  type ConfigRow,
} from "@/lib/queries/useClaimsDelivery";
import { useProjects } from "@/lib/queries/useProjects";
import type { Lands } from "@/lib/queries/useClaimShape";

const DAY = 24 * 60 * 60 * 1000;

/** The kinds of finding that are about delivery itself, per app. */
const LIVE_KINDS = new Set(["CLAIMS_NOT_CALLED", "CLAIMS_NOT_SERVED", "SIGNIN_EVENTS_MISSING"]);
/** Answers that carried the person's claims: a new member's welcome roles, and Zitadel's roles alone, included. */
const SERVED = ["served", "served_lkg", "served_welcome", "served_roles_only"];
/** Answers that carry no fault: the app's own settings, or not Zitadel at all. */
const QUIET_OUTCOMES = new Set(["served", "served_lkg", "served_welcome", "served_roles_only", "no_grants", "not_landing", "sig_missing"]);

/**
 * System › Zitadel › Claims delivery (claims-delivery-integrity S3–S5, §4g).
 * Always here, in one seat. Each open dysfunction is one Notice above the
 * parts, grouped by kind; the parts are Live (each app's last day) and
 * Configuration (Zitadel's setup against what delivery needs). Both are one
 * calm sentence when all is well, their detail one fold away; a problem opens
 * its fold.
 */
export function ClaimsDelivery({ domain, connected }: { domain?: string; connected: boolean }) {
  const delivery = useClaimsDelivery();
  const projects = useProjects();
  const d = delivery.data;
  const projectName = (id?: string) => projects.data?.find((p) => p.project.id === id)?.project.name;
  const names: Names = {
    app: (clientID) => d?.apps.find((a) => a.client_id === clientID),
    appByID: (id) => d?.apps.find((a) => a.app_id === id),
    project: projectName,
    domain,
  };
  const open = (d?.findings ?? []).filter((f) => !f.resolved_at && f.severity !== "info");
  const settled = (d?.findings ?? []).filter((f) => f.resolved_at);

  return (
    <section id="claims-delivery" aria-label="Claims delivery" className="flex scroll-mt-5 flex-col gap-4">
      {groupByKind(open).map((group) => (
        <FindingNotice key={group[0].kind} group={group} names={names} />
      ))}
      <Card stale={Boolean(d && delivery.error)}>
        <CardHeader
          title="Claims delivery"
          note={
            d
              ? delivery.error
                ? `From the last read, at ${formatClock(new Date(delivery.dataUpdatedAt).toISOString())}`
                : `Read ${formatClock(new Date(delivery.dataUpdatedAt).toISOString())}`
              : undefined
          }
        />
        {!d ? (
          delivery.error ? (
            <div className="px-5 pb-5">
              <ErrorState title="Couldn't read how claims are delivered." error={delivery.error} onRetry={() => delivery.refetch()} />
            </div>
          ) : (
            <div className="px-5 pb-5">
              <RowSkeleton rows={2} avatar={false} label="Reading how claims are delivered" />
            </div>
          )
        ) : (
          <>
            <Live d={d} open={open} />
            <Configuration d={d} connected={connected} names={names} />
            {settled.length > 0 && (
              <p className="row-divider px-5 py-3 text-[13px] text-faint">
                Settled in the last 24h: {settled.length === 1 ? "1 problem" : `${settled.length} problems`}, the last at{" "}
                {formatWhen(settled.map((f) => f.resolved_at!).sort().at(-1))}.
              </p>
            )}
            <AlertNote notes={d.alert.notes} />
          </>
        )}
      </Card>
    </section>
  );
}

interface Names {
  app: (clientID: string) => ClaimApp | undefined;
  appByID: (id: string) => ClaimApp | undefined;
  project: (id?: string) => string | undefined;
  domain?: string;
}

function groupByKind(findings: ClaimFinding[]): ClaimFinding[][] {
  const by = new Map<string, ClaimFinding[]>();
  for (const f of findings) by.set(f.kind, [...(by.get(f.kind) ?? []), f]);
  return Array.from(by.values());
}

const plural = (n: number, one: string, many = `${one}s`) => `${n} ${n === 1 ? one : many}`;

/**
 * An app's name inside a Notice, or plain words for one Syndra can't name.
 * Plain prose (RULES: notices don't link); the Live table below links it.
 */
function AppName({ clientID, names }: { clientID: string; names: Names }) {
  const app = names.app(clientID);
  if (app?.name) return <span className="font-semibold">{app.name}</span>;
  if (clientID === "-") return <span>a sign-in that named no app</span>;
  return (
    <span>
      an app Syndra doesn&rsquo;t know, <Mono>{clientID}</Mono>
    </span>
  );
}

function AppList({ findings, names }: { findings: ClaimFinding[]; names: Names }) {
  return (
    <>
      {findings.map((f, i) => (
        <span key={f.subject}>
          {i > 0 && (i === findings.length - 1 ? " and " : ", ")}
          <AppName clientID={f.subject} names={names} />
        </span>
      ))}
    </>
  );
}

const statusOf = (s?: Record<string, unknown>) =>
  (s?.status ?? (s?.reason as Record<string, unknown> | undefined)?.status) as number | undefined;

const CHECK_PART: Record<string, string> = {
  R1: "the claims action",
  R2: "which steps call it",
  R3: "the event listener",
  R4: "Actions v1",
  R5: "the apps' token settings",
  R6: "the apps and their projects",
  R7: "the projects' roles",
  R8: "the token settings",
  R9: "Syndra's own setup",
};

const GRANT_VIEWER = (
  <>
    Zitadel refused Syndra&rsquo;s service account. Give it the read-only <Mono>IAM_OWNER_VIEWER</Mono> role in
    Zitadel, and the next check reads it.
  </>
);

/** One Notice for one dysfunction, however many apps or checks it touches. */
function FindingNotice({ group, names }: { group: ClaimFinding[]; names: Names }) {
  const kind = group[0].kind;
  const tone = group.some((f) => f.severity === "hard") ? "danger" : "warn";
  const apps = group.length;
  let title: string;
  let body: ReactNode;
  switch (kind) {
    case "CLAIMS_NOT_CALLED":
      title = `Zitadel didn't reach Syndra for ${plural(apps, "app")}`;
      body = (
        <>
          People signed in to <AppList findings={group} names={names} />, and Zitadel didn&rsquo;t ask Syndra for
          their claims, so their tokens went out without them. Configuration, below, compares Zitadel&rsquo;s
          Actions with what delivery needs.
        </>
      );
      break;
    case "CLAIMS_NOT_SERVED":
      title = `Syndra answered ${plural(apps, "app")} without people's claims`;
      body = (
        <>
          {group.map((f, i) => (
            <span key={f.subject}>
              {i > 0 && " "}
              <AppName clientID={f.subject} names={names} />: {outcomePhrase(f.sample?.outcome as string) || "no reason was recorded"}.
            </span>
          ))}{" "}
          Zitadel called, so the sign-in went ahead, but the token carried none of Syndra&rsquo;s claims.
        </>
      );
      break;
    case "SIGNIN_EVENTS_MISSING":
      title = "No sign-in has reached Syndra for a day";
      body =
        "Zitadel still asks Syndra for claims, but its sign-in events have stopped arriving, so Syndra can't tell whether every sign-in got its claims. Check the event listener in Zitadel's Actions.";
      break;
    case "CONFIG_DRIFT":
      title = "Zitadel's setup differs from what claim delivery needs";
      body = `${plural(apps, "check")} ${apps === 1 ? "differs" : "differ"}. Configuration, below, says what was expected, what Zitadel has, and where to change it.`;
      break;
    case "CANNOT_CHECK": {
      const refused = group.some((f) => statusOf(f.sample) === 403);
      const parts = formatList(Array.from(new Set(group.map((f) => CHECK_PART[f.subject.split(":")[0]] ?? f.subject))));
      title = "Syndra can't read part of Zitadel's setup";
      body = refused ? (
        <>
          It couldn&rsquo;t read {parts}. {GRANT_VIEWER}
        </>
      ) : (
        `It couldn't read ${parts}: Zitadel didn't answer. Syndra tries again every few minutes.`
      );
      break;
    }
    default:
      title = "Claim delivery needs a look";
      body = (
        <>
          Syndra found <Mono>{kind}</Mono> on {plural(apps, "subject")}.
        </>
      );
  }
  return (
    <Notice tone={tone} title={title} live>
      {body}
    </Notice>
  );
}

/** A round chevron and its words: the one fold control in this section. */
function Fold({ open, onToggle, show, hide }: { open: boolean; onToggle: () => void; show: string; hide: string }) {
  return (
    <span className="ml-auto inline-flex flex-none">
      <Button variant="ghost" size="sm" aria-expanded={open} onClick={onToggle}>
        {open ? hide : show}
      </Button>
    </span>
  );
}

/** A fold that opens itself on a problem and otherwise waits; a choice holds until that changes. */
function useFold(bad: boolean): [boolean, () => void] {
  const [choice, setChoice] = useState<{ bad: boolean; open: boolean } | null>(null);
  const open = choice?.bad === bad ? choice.open : bad;
  return [open, () => setChoice({ bad, open: !open })];
}

function PartHead({ label, tone, sentence, fold }: { label: string; tone: StatusTone; sentence: ReactNode; fold: ReactNode }) {
  return (
    <div className="row-divider flex flex-wrap items-center gap-x-3 gap-y-1.5 px-5 py-3.5">
      <span className="type-label w-full">{label}</span>
      <StatusDot tone={tone} />
      <span className="min-w-0 flex-[1_1_320px] text-[14px] leading-[1.55] text-ink [text-wrap:pretty]">{sentence}</span>
      {fold}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Live

function Live({ d, open }: { d: Delivery; open: ClaimFinding[] }) {
  const live = open.filter((f) => LIVE_KINDS.has(f.kind));
  const [unfolded, toggle] = useFold(live.length > 0);
  const since = d.sign_ins.checking_since;
  const restarted = since && Date.now() - new Date(since).getTime() < DAY;
  const span = restarted ? `since ${formatWhen(since)}` : "in the last 24h";
  const calls = d.apps.reduce((n, a) => n + a.calls, 0);
  const signedIn = d.apps.some((a) => a.last_sign_in_at) || calls > 0;
  const troubled = new Set(live.filter((f) => f.kind !== "SIGNIN_EVENTS_MISSING").map((f) => f.subject)).size;

  const sentence = troubled
    ? `${plural(troubled, "app")} ${troubled === 1 ? "needs" : "need"} a look ${span}.`
    : live.length
      ? "Syndra can't check sign-ins right now: none has reached it for a day."
      : signedIn
      ? `Every sign-in ${span} got its claims.`
      : `Nobody has signed in ${span}, so there is nothing to check yet.`;

  return (
    <>
      <PartHead
        label="Live"
        tone={live.length ? (live.some((f) => f.severity === "hard") ? "danger" : "warn") : signedIn ? "healthy" : "neutral"}
        sentence={
          <>
            {sentence}
            {restarted && (
              <span className="block text-[13px] text-faint">
                Syndra started checking at {formatClock(since)}. Sign-ins before that aren&rsquo;t checked, and none is counted as failed.
              </span>
            )}
          </>
        }
        fold={d.apps.length > 0 && <Fold open={unfolded} onToggle={toggle} show="Show each app" hide="Hide each app" />}
      />
      {d.apps.length > 0 && (
        <Unfold open={unfolded}>
          <CardColumns className="row-divider">
            <span className="tablet:w-[220px] tablet:shrink-0">App</span>
            <span className="tablet:w-[130px] tablet:shrink-0">Last sign-in</span>
            <span className="tablet:w-[130px] tablet:shrink-0">Last delivered</span>
            <span className="min-w-0 flex-1">Answers, 24h</span>
            <span className="w-[110px] shrink-0 text-right">Answer time</span>
          </CardColumns>
          {d.apps.map((app) => (
            <AppRow key={app.client_id} app={app} />
          ))}
        </Unfold>
      )}
    </>
  );
}

function AppRow({ app }: { app: ClaimApp }) {
  const served = SERVED.reduce((n, o) => n + (app.outcomes[o] ?? 0), 0);
  const others = Object.entries(app.outcomes).filter(([o, n]) => n > 0 && !SERVED.includes(o));
  return (
    <div className="row-divider flex flex-col gap-1.5 px-5 py-3 text-[14px] tablet:flex-row tablet:items-baseline tablet:gap-[18px]">
      <span className="flex min-w-0 flex-col gap-0.5 tablet:w-[220px] tablet:shrink-0">
        {app.name && app.app_id ? (
          <Link href={`/applications/${app.app_id}`} className="font-semibold text-ink link-hover">
            {app.name}
          </Link>
        ) : app.name ? (
          <span className="font-semibold">{app.name}</span>
        ) : app.client_id === "-" ? (
          <>
            <span className="font-semibold">No app named</span>
            <span className="text-[13px] text-faint">Zitadel&rsquo;s call named no app</span>
          </>
        ) : (
          <>
            <span className="font-semibold">Not an app in Syndra</span>
            <Mono className="truncate text-[12.5px] text-faint" title={app.client_id}>
              {app.client_id}
            </Mono>
          </>
        )}
      </span>
      <span className="text-[13px] text-faint tablet:w-[130px] tablet:shrink-0">{when(app.last_sign_in_at, "No sign-in")}</span>
      <span className="text-[13px] text-faint tablet:w-[130px] tablet:shrink-0">{when(app.last_served_at, "Not delivered")}</span>
      <span className="min-w-0 flex-1 text-[13.5px] text-muted [text-wrap:pretty]">
        {app.calls === 0 ? (
          <span className="text-faint">No calls</span>
        ) : (
          <>
            {served} served
            {others.map(([o, n]) => (
              <span key={o} className={QUIET_OUTCOMES.has(o) ? "text-faint" : "text-warn-text"}>
                {" · "}
                {outcomePhrase(o)}: {n}
              </span>
            ))}
          </>
        )}
      </span>
      <span className="text-[13.5px] tabular-nums text-muted tablet:w-[110px] tablet:shrink-0 tablet:text-right">{answerTime(app)}</span>
    </div>
  );
}

const when = (iso: string | null, none: string) => (iso ? formatSignIn(iso) : none);

/** p99 is a bucket's upper bound; none past the last bucket means over 3 s. */
function answerTime(app: ClaimApp): string {
  if (app.calls === 0) return "—";
  if (app.p99_le_ms == null) return "over 3 s";
  return app.p99_le_ms >= 1000 ? `under ${app.p99_le_ms / 1000} s` : `under ${app.p99_le_ms} ms`;
}

// ---------------------------------------------------------------------------
// Configuration

const PROBLEM = new Set(["drift", "cannot"]);

function Configuration({ d, connected, names }: { d: Delivery; connected: boolean; names: Names }) {
  const cfg = d.configuration;
  const rows = cfg?.rows ?? [];
  const drift = rows.filter((r) => r.state === "drift").length;
  const cannot = rows.filter((r) => r.state === "cannot").length;
  const [unfolded, toggle] = useFold(drift + cannot > 0);
  const at = cfg ? formatWhen(cfg.checked_at) : "";

  const sentence = !cfg
    ? connected
      ? "Not checked yet. Syndra checks Zitadel's setup a few minutes after it starts."
      : "Not checked: Syndra isn't connected to Zitadel, so there is nothing to compare."
    : drift
      ? `${plural(drift, "check")} ${drift === 1 ? "differs" : "differ"} from what claim delivery needs. Checked ${at}.`
      : cannot
        ? `Syndra couldn't read ${plural(cannot, "part")} of Zitadel's setup. Checked ${at}.`
        : `Zitadel's setup matches what claim delivery needs. Checked ${at}.`;

  return (
    <>
      <PartHead
        label="Configuration"
        tone={!cfg ? "neutral" : drift ? "danger" : cannot ? "warn" : "healthy"}
        sentence={sentence}
        fold={cfg && rows.length > 0 && <Fold open={unfolded} onToggle={toggle} show="Show the checklist" hide="Hide the checklist" />}
      />
      {cfg && rows.length > 0 && (
        <Unfold open={unfolded}>
          <ol aria-label="Configuration checklist">
            {rows.map((row) => (
              <ConfigLine key={row.subject} row={row} names={names} />
            ))}
          </ol>
        </Unfold>
      )}
    </>
  );
}

const STATE_TONE: Record<ConfigRow["state"], StatusTone> = {
  pass: "healthy",
  drift: "danger",
  info: "neutral",
  cannot: "warn",
  not_needed: "neutral",
};

function ConfigLine({ row, names }: { row: ConfigRow; names: Names }) {
  const problem = PROBLEM.has(row.state);
  const fix = problem || row.state === "info" ? fixFor(row, names) : null;
  return (
    <li className="row-divider flex flex-col gap-1 px-5 py-3 tablet:flex-row tablet:items-baseline tablet:gap-[18px]">
      <span className="flex min-w-0 flex-1 items-baseline gap-2.5">
        <StatusDot tone={STATE_TONE[row.state]} />
        <span className="flex min-w-0 flex-col gap-0.5">
          <span className="text-[14px] text-ink [text-wrap:pretty]">
            <span className="text-faint">{row.check} · </span>
            {labelOf(row, names)}
          </span>
          {row.state === "cannot" ? (
            <span className="text-[13px] leading-[1.5] text-muted">
              {statusOf(row.facts) === 403 ? GRANT_VIEWER : `Zitadel didn't answer${row.facts?.error ? `: ${String(row.facts.error)}` : ""}.`}
            </span>
          ) : (
            problem && (
              <span className="text-[13px] leading-[1.5] text-muted [overflow-wrap:anywhere]">
                Expected <Value v={row.check === "R5" ? landsWords(row.expected) : row.expected} /> · found{" "}
                <Value v={row.check === "R5" ? reachWords(row.found) : row.found} />
              </span>
            )
          )}
          {row.state === "info" && note(row) && <span className="text-[13px] leading-[1.5] text-muted">{note(row)}</span>}
        </span>
      </span>
      {fix && <span className="text-[13.5px] tablet:ml-auto tablet:w-[220px] tablet:shrink-0 tablet:text-right">{fix}</span>}
    </li>
  );
}

function Value({ v }: { v: unknown }) {
  if (v === undefined || v === null || (Array.isArray(v) && v.length === 0)) return <span className="text-faint">nothing</span>;
  const text = Array.isArray(v) ? v.join(", ") : typeof v === "object" ? JSON.stringify(v) : String(v);
  return <Mono>{text}</Mono>;
}

/** R5's raw values, said: where a shape lands, and what Zitadel's app settings let through. */
const landsWords = (v: unknown) => LANDS_LABEL[v as Lands] ?? v;
function reachWords(v: unknown): unknown {
  if (!v || typeof v !== "object") return v;
  const f = v as { access_token_type?: string; id_token_userinfo_assertion?: boolean };
  return `${f.access_token_type ?? "unknown"} access token, userinfo ${f.id_token_userinfo_assertion ? "inside" : "outside"} the ID token`;
}

function appName(names: Names, id?: unknown): string {
  return (typeof id === "string" && names.appByID(id)?.name) || "An app";
}

/** Each row's plain words (claims-delivery-integrity §4b, R1–R9). */
function labelOf(row: ConfigRow, names: Names): ReactNode {
  const sub = row.subject.startsWith(`${row.check}:`) ? row.subject.slice(row.check.length + 1) : "";
  if (row.state === "cannot") return `Couldn't read ${CHECK_PART[row.check] ?? row.check} in Zitadel`;
  switch (row.check) {
    case "R1":
      return (
        {
          present: <>Zitadel has the claims action, <Mono>syndra-claim-injector</Mono></>,
          endpoint: "It calls Syndra's address",
          type: "It waits for Syndra's answer",
          interrupt_on_error: "A failed call doesn't stop the sign-in",
          timeout: "Its time limit",
          payload_type: "What it sends Syndra",
          signing_key: "Zitadel signs with a key Syndra holds",
        } as Record<string, ReactNode>
      )[sub] ?? sub;
    case "R2":
      return sub === "preuserinfo" ? "Userinfo calls only the claims action" : "Making an access token calls only the claims action";
    case "R3":
      return sub === "target" ? (
        <>Zitadel has the event listener, <Mono>syndra-event-listener</Mono></>
      ) : (
        <>
          Zitadel sends <Mono>{sub}</Mono> to the event listener
        </>
      );
    case "R4": {
      const keys = Number(row.facts?.keys ?? (Array.isArray(row.found) ? row.found.length : 0));
      return row.state === "info" ? `Actions v1 still sets ${plural(keys, "key")}` : "No Actions v1 script sets claims";
    }
    case "R5":
      return `${appName(names, row.facts?.app_id ?? sub)}'s claims reach where its shape sends them`;
    case "R6":
      return sub.startsWith("shape:")
        ? "A saved shape names an app that's gone from Zitadel"
        : `${appName(names, sub.replace(/^app:/, ""))}'s project is ready for sign-ins`;
    case "R7": {
      const project = names.project(String(row.facts?.project_id ?? sub.split(":")[0])) ?? "A project";
      const role = row.facts?.role as string | undefined;
      return role ? (
        <>
          A shape names <Mono>{role}</Mono>, which {project} no longer has
        </>
      ) : (
        `${project}'s roles match its apps' shapes`
      );
    }
    case "R8":
      return "No Zitadel setting to check: version 4 has no switch for this";
    case "R9":
      return (
        {
          key_present: "The claims signing key is set",
          url_https: "Syndra's address uses HTTPS",
          url_resolves: "Syndra's address can be found",
          env_key: "The key in .env is older than Zitadel's",
        } as Record<string, string>
      )[sub] ?? sub;
  }
  return row.subject;
}

/** An informational row's one line: what it means for you. */
function note(row: ConfigRow): string | null {
  if (row.check === "R4") {
    const collide = (row.facts?.collide as string[] | undefined) ?? [];
    return collide.length
      ? `${formatList(collide)} ${collide.length === 1 ? "is" : "are"} also in a shape. Actions v1 runs first, so Syndra's value is dropped. Retire the v1 script when you're ready.`
      : "None of them is in a shape, so nothing of Syndra's is dropped.";
  }
  if (row.check === "R9") return "Syndra follows Zitadel's key on its own. Update .env at the next deploy.";
  return null;
}

function External({ href, children }: { href: string; children: ReactNode }) {
  return (
    <a href={href} target="_blank" rel="noreferrer" className="font-semibold text-accent-text link-hover">
      {children} →
    </a>
  );
}


/** The one place to fix a row, or where it is set. */
function fixFor(row: ConfigRow, names: Names): ReactNode {
  const zc = names.domain ? `https://${names.domain}/ui/console` : null;
  const f = row.facts ?? {};
  switch (row.check) {
    case "R1":
    case "R2":
    case "R3":
      return zc && row.state !== "cannot" ? <External href={`${zc}/actions`}>Open Actions in Zitadel</External> : null;
    case "R5":
      return zc && f.project_id && f.app_id ? (
        <External href={`${zc}/projects/${f.project_id}/apps/${f.app_id}`}>Open {appName(names, f.app_id)} in Zitadel</External>
      ) : null;
    case "R6":
    case "R7": {
      const app = typeof f.app_id === "string" ? f.app_id : Array.isArray(f.app_ids) ? (f.app_ids[0] as string) : undefined;
      if (app) return <ArrowLink href={`/applications/${app}`}>Open {appName(names, app)}&rsquo;s claims</ArrowLink>;
      return f.project_id ? <ArrowLink href={`/projects/${f.project_id}`}>Open {names.project(String(f.project_id)) ?? "the project"}</ArrowLink> : null;
    }
    case "R9":
      return <span className="text-faint">Set in the server&rsquo;s .env</span>;
  }
  return null;
}

function AlertNote({ notes }: { notes: string[] }) {
  if (notes.includes("EXTERNAL_ALERT_UNREACHABLE")) {
    return (
      <p className="row-divider px-5 py-3 text-[13px] text-faint">
        Syndra couldn&rsquo;t reach the alert outside it on its last try, so a silence there may be Syndra&rsquo;s, not delivery&rsquo;s.
      </p>
    );
  }
  if (notes.includes("NO_EXTERNAL_ALERT")) {
    return (
      <p className="row-divider px-5 py-3 text-[13px] text-faint">
        No alert outside Syndra is set up, so if Syndra itself stops, nobody is told. Set <Mono>SYNDRA_DEADMAN_URL</Mono> in the
        server&rsquo;s .env to have one.
      </p>
    );
  }
  return null;
}
