"use client";

import Link from "next/link";
import { useEffect, useRef, useState } from "react";

import { UserAvatar, UserName } from "@/components/names";
import { ActionOutcome } from "@/components/ui/ActionOutcome";
import { Badge, Mono, StatusDot } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { CardColumns } from "@/components/ui/Card";
import { CheckTrail } from "@/components/ui/CheckTrail";
import { ChoiceRow } from "@/components/ui/ChoiceRow";
import { CopiedSeat, useConfirmFor } from "@/components/ui/CopiedSeat";
import { FieldLabel, Input } from "@/components/ui/Input";
import { Modal, ModalFooter, ModalHeader } from "@/components/ui/Modal";
import { PaginationRow, usePaging } from "@/components/ui/PaginationRow";
import { SignInRow } from "@/components/ui/SignInRow";
import { StatusSeat } from "@/components/ui/StatusSeat";
import { TokenBlock, type TokenBlockLine } from "@/components/ui/TokenBlock";
import { Unfold } from "@/components/ui/Unfold";
import { clockOf, literal, minutesLeft } from "@/lib/claims";
import { useClipboardAvailable } from "@/lib/clipboard";
import { outcomeFromError, type ActionOutcome as Outcome } from "@/lib/outcome";
import { useCatalogUsers, type CatalogUser } from "@/lib/queries/useCatalogUsers";
import {
  useDebugSignIns,
  useStartDebug,
  useStopDebug,
  type Check,
  type DebugSession,
  type SignIn,
} from "@/lib/queries/useClaimShape";
import { useUiView } from "@/lib/ui-view";

import { check5, check6, checkWords, verdict, type CheckContext } from "./claim-words";

const clockSec = (iso: string) =>
  new Date(iso).toLocaleTimeString("en-GB", { hour: "2-digit", minute: "2-digit", second: "2-digit", hour12: false });

/** Re-renders once a minute, so "14 min left" changes in place (S4), never ticking. */
function useMinute() {
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    const t = setInterval(() => setNow(Date.now()), 60_000);
    return () => clearInterval(t);
  }, []);
  return now;
}

/**
 * Debug mode (A-14 §6): one chosen account's real sign-ins to this app, live,
 * for 15 minutes the server enforces. Off by default; only here, beside the
 * preview. What it sees stays in Syndra's memory for an hour, never its
 * database; signatures are dropped before capture. A settings row of the app
 * page's tools card (A-15 §6); its live list unfolds under it, in the card.
 */
export function DebugMode({ appId, ctx }: { appId: string; ctx: Omit<CheckContext, "person"> }) {
  const { selfId } = useUiView();
  const [on, setOn] = useState(false);
  const live = useDebugSignIns(appId, on);
  const session = live.data?.session ?? null;
  const isOn = Boolean(live.data?.on);
  useEffect(() => setOn(isOn), [isOn]);
  const now = useMinute();
  const users = useCatalogUsers();
  const stop = useStopDebug(appId);
  const [sheet, setSheet] = useState<"on" | "switch" | null>(null);
  const [stopOutcome, setStopOutcome] = useState<Outcome | null>(null);
  const nameOf = (id?: string) => (id === selfId ? "You" : (users.data.find((u) => u.id === id)?.name ?? "Someone"));
  const whose = session ? (session.user_id === selfId ? "Your" : `${nameOf(session.user_id)}'s`) : "";

  // The server ends it; the page asks again the moment it should have.
  useEffect(() => {
    if (!isOn || !session) return;
    const t = setTimeout(() => live.refetch(), Math.max(0, new Date(session.ends_at).getTime() - Date.now()) + 500);
    return () => clearTimeout(t);
  }, [isOn, session, live]);

  const ended = session && !isOn;
  return (
    <>
      <div className="row-divider flex flex-wrap items-center gap-x-[18px] gap-y-2 px-[22px] py-3.5">
        <span className="w-[170px] flex-none text-[14px] font-semibold">Debug mode</span>
        <div className="flex min-w-0 flex-[1_1_260px] flex-col gap-1">
          {!session ? (
            <StatusSeat tone="neutral" word="Off" note="See one account's sign-ins as they happen, for 15 minutes." />
          ) : isOn ? (
            <StatusSeat tone="healthy" word="On" note={`${whose} sign-ins · ends ${clockOf(session.ends_at)}, ${minutesLeft(session.ends_at, now)} min left`} />
          ) : (
            <StatusSeat
              tone="neutral"
              word="Ended"
              note={
                session.ended_by && session.ended_by !== "system"
                  ? `Turned off at ${clockOf(session.ended_at)} by ${session.ended_by === selfId ? "you" : nameOf(session.ended_by)}.`
                  : `At ${clockOf(session.ends_at)}, after 15 minutes.`
              }
            />
          )}
          {stopOutcome && <ActionOutcome outcome={stopOutcome} placement="inline" />}
        </div>
        <span className="ml-auto flex flex-wrap justify-end gap-2 whitespace-nowrap">
          {isOn ? (
            <>
              <Button size="sm" onClick={() => setSheet("switch")}>
                Change account
              </Button>
              <Button
                size="sm"
                isPending={stop.isPending}
                onClick={async () => {
                  setStopOutcome(null);
                  try {
                    await stop.mutateAsync();
                  } catch (e) {
                    setStopOutcome(outcomeFromError(e));
                  }
                }}
              >
                Turn off
              </Button>
            </>
          ) : (
            <Button size="sm" onClick={() => setSheet("on")}>
              {ended ? "Turn on again" : "Turn on debug mode"}
            </Button>
          )}
        </span>
      </div>
      <Unfold open={Boolean(session)}>
        {session && (
          <LiveList
            appId={appId}
            signIns={live.data?.sign_ins ?? []}
            on={isOn}
            session={session}
            ctx={ctx}
            mine={session.user_id === selfId}
            nameOf={(id) => users.data.find((u) => u.id === id)?.name ?? "this person"}
          />
        )}
      </Unfold>
      {sheet && (
        <DebugSheet
          appId={appId}
          app={ctx.app}
          mode={sheet}
          session={isOn ? session : null}
          people={users.data}
          selfId={selfId}
          onClose={() => setSheet(null)}
        />
      )}
    </>
  );
}

function LiveList({
  signIns,
  on,
  session,
  ctx,
  mine,
  nameOf,
}: {
  appId: string;
  signIns: SignIn[];
  on: boolean;
  session: DebugSession;
  ctx: Omit<CheckContext, "person">;
  mine: boolean;
  nameOf: (id?: string) => string;
}) {
  const [inside, setInside] = useState(false);
  const [openSeq, setOpenSeq] = useState<number | null>(null);
  // What you're reading never moves: while the pointer or focus is in the
  // list, or a sign-in is open, new rows wait and the read line counts them.
  const held = inside || openSeq !== null;
  const [cutoff, setCutoff] = useState<number | null>(null);
  const newest = signIns[0]?.seq ?? 0;
  useEffect(() => {
    setCutoff(held ? (c) => c ?? newest : null);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [held]);
  const shown = cutoff === null ? signIns : signIns.filter((s) => s.seq <= cutoff);
  const waiting = signIns.length - shown.length;
  const paging = usePaging();
  const firstSeen = useRef<number | null>(null);
  if (firstSeen.current === null) firstSeen.current = newest;

  return (
    <div onPointerEnter={() => setInside(true)} onPointerLeave={() => setInside(false)} onFocus={() => setInside(true)} onBlur={(e) => !e.currentTarget.contains(e.relatedTarget) && setInside(false)}>
      <div className="row-divider flex flex-wrap items-center gap-x-3 gap-y-1 px-[22px] py-2.5">
        <span className="inline-flex items-center gap-2 text-[13.5px] text-muted">
          <StatusDot tone={on ? "healthy" : "neutral"} />
          {on ? "Live · newest on top" : "Seen while it was on · newest on top"}
        </span>
        {on && waiting > 0 && (
          <span className="ml-auto text-[13px] text-faint">
            {waiting === 1 ? "1 new sign-in waits" : `${waiting} new sign-ins wait`} until you {openSeq !== null ? "close this one" : "move off the list"}.
          </span>
        )}
      </div>
      {shown.length === 0 ? (
        <div className="flex flex-col gap-1 px-[22px] pb-[22px] pt-4">
          <span className="inline-flex items-center gap-2.5 text-[14.5px]">
            {on && <span aria-hidden className="breathe size-2 rounded-pill bg-healthy" />}
            {on ? `Waiting for ${mine ? "your" : `${nameOf(session.user_id)}'s`} next sign-in to ${ctx.app}.` : "Nothing was seen while it was on."}
          </span>
          {on && mine && <span className="pl-[18px] text-[13px] text-faint">Sign in to {ctx.app} in another window to see it here.</span>}
        </div>
      ) : (
        <>
          <CardColumns>
            <span className="w-7 flex-none" />
            <span className="w-[96px] flex-none">Time ↓</span>
            <span className="w-[210px] flex-none">Person</span>
            <span className="w-[120px] flex-none">Asked for</span>
            <span className="min-w-[150px] flex-1">Syndra answered</span>
            <span className="w-[120px] flex-none text-right">Checks</span>
          </CardColumns>
          {shown.slice(0, paging.limit).map((s) => (
            <Arriving key={s.seq} fresh={s.seq > (firstSeen.current ?? 0)}>
              <SignInRow
                time={clockSec(s.at)}
                person={
                  <>
                    <UserAvatar id={s.user_id} size="row" />
                    <Link href={`/users/${s.user_id}`} className="text-[14.5px] font-semibold link-hover">
                      <UserName id={s.user_id} />
                    </Link>
                  </>
                }
                asked={s.function.endsWith("preaccesstoken") ? "Access token" : "Userinfo"}
                answered={answeredLine(s)}
                verdict={verdict(Array.isArray(s.checks) ? s.checks : null)}
                open={openSeq === s.seq}
                onToggle={() => setOpenSeq((o) => (o === s.seq ? null : s.seq))}
              >
                <OpenedSignIn s={s} ctx={{ ...ctx, person: nameOf(s.user_id) }} />
              </SignInRow>
            </Arriving>
          ))}
          <PaginationRow shown={Math.min(paging.limit, shown.length)} total={shown.length} onMore={paging.more} onAll={() => paging.all(shown.length)} onBack={paging.back} />
        </>
      )}
      {!on && (
        <p className="row-divider px-[22px] pb-4 pt-3 text-[13px] text-faint">
          What it saw stays here until {clockOf(new Date(new Date(session.ended_at ?? session.ends_at).getTime() + 60 * 60_000).toISOString())}, then Syndra forgets it. It&rsquo;s gone sooner if Syndra restarts.
        </p>
      )}
    </div>
  );
}

/** A new sign-in opens its gap at the top and tints in (the refresh rule); never the 8px rise. */
function Arriving({ fresh, children }: { fresh: boolean; children: React.ReactNode }) {
  const [open, setOpen] = useState(!fresh);
  useEffect(() => {
    if (!open) setOpen(true);
  }, [open]);
  return <Unfold open={open}>{children}</Unfold>;
}

interface Appended {
  key: string;
  value: unknown;
}

function appended(s: SignIn): Appended[] {
  const r = s.response as { append_claims?: Appended[] } | null;
  return Array.isArray(r?.append_claims) ? r!.append_claims : [];
}

function answeredLine(s: SignIn): React.ReactNode {
  const claims = appended(s);
  if (claims.length === 0) return <span className="text-muted">Nothing</span>;
  return claims.map((c, i) => {
    const n = Array.isArray(c.value) ? c.value.length : null;
    return (
      <span key={c.key}>
        {i > 0 && ", "}
        <Mono>{c.key}</Mono>
        {n !== null && `, ${n === 1 ? "1 value" : `${n} values`}`}
      </span>
    );
  });
}

function CopyButton({ idle, value }: { idle: string; value: unknown }) {
  const [done, setDone] = useState(false);
  const confirmFor = useConfirmFor();
  const canCopy = useClipboardAvailable();
  if (!canCopy) return null;
  return (
    <Button
      size="sm"
      onClick={async () => {
        await navigator.clipboard.writeText(JSON.stringify(value, null, 2));
        confirmFor(setDone);
      }}
    >
      <CopiedSeat done={done} idle={idle} word="Copied" />
    </Button>
  );
}

/** One sign-in, opened (frame debug-open): what Zitadel sent, what Syndra answered, the checks, and a labelled rebuild. */
function OpenedSignIn({ s, ctx }: { s: SignIn; ctx: CheckContext }) {
  const req = (s.request ?? {}) as {
    user?: { id?: string; preferred_login_name?: string; username?: string; human?: { email?: string; is_email_verified?: boolean; display_name?: string } };
    user_grants?: Array<{ projectId?: string; roles?: string[] }>;
  };
  const roleCount = (req.user_grants ?? []).reduce((n, g) => n + (g.roles?.length ?? 0), 0);
  const claims = appended(s);
  const checks = Array.isArray(s.checks) ? (s.checks as Check[]) : null;
  const built = checks?.[2]?.facts?.compiled_at as string | undefined;
  const asked = s.function.endsWith("preaccesstoken") ? "access-token" : "userinfo";
  const theirs: TokenBlockLine[] = [
    ["sub", req.user?.id],
    ["email", req.user?.human?.email],
    ["email_verified", req.user?.human?.is_email_verified],
    ["name", req.user?.human?.display_name],
    ["preferred_username", req.user?.preferred_login_name ?? req.user?.username],
  ]
    .filter(([, v]) => v !== undefined)
    .map(([k, v]) => ({ key: String(k), text: `"${k}": ${literal(v)}`, theirs: true }));
  const ours: TokenBlockLine[] = claims.map((c) => ({ key: c.key, text: `"${c.key}": ${literal(c.value)}` }));
  const answered = { user_id: s.user_id, checks: checks ?? [], token: Object.fromEntries(claims.map((c) => [c.key, c.value])), read_at: s.at };
  const trail = checks ? [...checkWords(answered, ctx), check5(answered, ctx, false), ...check6(answered, ctx)] : null;

  return (
    <>
      <div className="flex flex-wrap gap-[22px]">
        <section className="flex min-w-0 flex-[1_1_380px] flex-col gap-2">
          <span className="type-label">What Zitadel sent</span>
          <p className="text-[14px] leading-[1.5]">
            Zitadel asked for {ctx.app}&rsquo;s {asked} claims for {ctx.person}, listing {roleCount === 1 ? "1 role" : `${roleCount} roles`} in {ctx.project}.
          </p>
          <TokenBlock raw={s.request} className="text-[12.5px] leading-[1.7]" />
          <span className="text-[12.5px] text-faint">Signed by Zitadel. The signature isn&rsquo;t shown.</span>
          <span>
            <CopyButton idle="Copy request" value={s.request} />
          </span>
        </section>
        <section className="flex min-w-0 flex-[1_1_380px] flex-col gap-2">
          <span className="type-label">What Syndra answered</span>
          <p className="text-[14px] leading-[1.5]">
            Syndra added {claims.length === 1 ? "1 claim" : `${claims.length} claims`}, in {Math.round(s.duration_ms)} ms
            {built ? `, from their claims built at ${clockOf(built)}` : ""}.
          </p>
          <TokenBlock raw={s.response} className="text-[12.5px] leading-[1.7]" />
          <span>
            <CopyButton idle="Copy answer" value={s.response} />
          </span>
        </section>
      </div>
      <section className="flex flex-col gap-2">
        <span className="type-label">The checks for this sign-in</span>
        {trail ? <CheckTrail checks={trail} compact /> : <p className="text-[14px] text-muted">Syndra couldn&rsquo;t check this sign-in.</p>}
      </section>
      <section className="flex max-w-[760px] flex-col gap-2">
        <span className="flex flex-wrap items-center gap-2.5">
          <span className="type-label">Your claims, as {ctx.app} will see them</span>
          <Badge hollow>Rebuilt, not the signed token</Badge>
        </span>
        <p className="max-w-[70ch] text-[13.5px] leading-[1.55] text-muted">
          Zitadel adds its own claims, signs the token and hands it to {ctx.app} after Syndra answers, so Syndra never sees the token. This is rebuilt from what Zitadel sent and what Syndra answered.
        </p>
        <TokenBlock lines={[...theirs, ...ours]} />
        <span className="text-[12.5px] text-faint">
          The faded lines are Zitadel&rsquo;s own, rebuilt from what it sent. <Mono>iss</Mono>, <Mono>aud</Mono> and the times are left out.
        </span>
      </section>
    </>
  );
}

/** Turn on, or change whose sign-ins: one account at a time, you by default (owner, 3 Nov). */
function DebugSheet({
  appId,
  app,
  mode,
  session,
  people,
  selfId,
  onClose,
}: {
  appId: string;
  app: string;
  mode: "on" | "switch";
  session: DebugSession | null;
  people: CatalogUser[];
  selfId?: string;
  onClose: () => void;
}) {
  const start = useStartDebug(appId);
  const me = people.find((u) => u.id === selfId);
  const current = session?.user_id;
  const [chosen, setChosen] = useState<string>(mode === "on" ? (selfId ?? "") : "");
  const [query, setQuery] = useState("");
  const [done, setDone] = useState<{ ends_at: string; switched: boolean } | null>(null);
  const [outcome, setOutcome] = useState<Outcome | null>(null);
  const nameOf = (id: string) => (id === selfId ? "you" : (people.find((u) => u.id === id)?.name ?? "this person"));
  const needle = query.trim().toLowerCase();
  const matches = needle
    ? people.filter((u) => u.id !== selfId && (u.name.toLowerCase().includes(needle) || u.email.toLowerCase().includes(needle))).slice(0, 5)
    : [];
  const ends = session?.ends_at ?? new Date(Date.now() + 15 * 60_000).toISOString();
  const chosenName = chosen ? nameOf(chosen) : "";
  const verb = mode === "on" ? "Turn on debug mode" : chosen ? `Switch to ${chosenName === "you" ? "you" : chosenName}` : "Switch";

  const confirm = async () => {
    setOutcome(null);
    try {
      const res = await start.mutateAsync(chosen && chosen !== selfId ? chosen : undefined);
      setDone({ ends_at: res.ends_at, switched: res.switched });
    } catch (e) {
      setOutcome(outcomeFromError(e));
    }
  };

  return (
    <Modal open onClose={onClose} busy={start.isPending} size="md">
      <ModalHeader
        title={mode === "on" ? "Turn on debug mode" : "Change whose sign-ins"}
        lede={
          done
            ? undefined
            : mode === "on"
              ? `See one account's sign-ins to ${app} live: what Zitadel asked, what Syndra answered, and the six checks. It turns itself off after 15 minutes.`
              : `Debug mode captures one account at a time. It's showing ${current === selfId ? "your" : `${nameOf(current ?? "")}'s`} sign-ins to ${app} until ${clockOf(ends)}.`
        }
      />
      {done ? (
        <div role="status" className="lift-in flex flex-col gap-2 px-6 pb-2">
          <span className="flex items-center gap-2.5">
            <StatusDot tone="healthy" bloom />
            <span className="font-display text-[22px] font-semibold">{done.switched ? "Switched" : "On"}</span>
          </span>
          <p className="text-[15px] leading-[1.55]">
            {done.switched
              ? `Debug mode now shows ${chosenName === "you" ? "your" : `${chosenName}'s`} sign-ins, until ${clockOf(done.ends_at)}.`
              : `Debug mode is on for ${app} until ${clockOf(done.ends_at)}, for ${chosen === selfId || !chosen ? "your" : `${chosenName}'s`} sign-ins.`}
          </p>
          <p className="text-[13px] text-faint">Each sign-in appears under the preview as it happens.</p>
        </div>
      ) : (
        <div className="flex flex-col gap-3 px-6 pb-2">
          <div className="flex items-baseline gap-2.5">
            <span className="type-label">Whose sign-ins</span>
            <span className="text-[12.5px] text-faint">One account at a time</span>
          </div>
          <div className="flex flex-col gap-1.5">
            {selfId && (
              <ChoiceRow
                single
                name="scope"
                selected={chosen === selfId}
                locked={current === selfId}
                lockedNote="Captured now"
                onToggle={() => setChosen(selfId)}
                title={`You, ${me?.name ?? "yourself"}`}
                description={`Sign in to ${app} yourself to see it here.`}
              />
            )}
            {current && current !== selfId && (
              <ChoiceRow single name="scope" selected={false} locked lockedNote="Captured now" onToggle={() => {}} title={nameOf(current)} />
            )}
            {chosen && chosen !== selfId && chosen !== current && !matches.some((u) => u.id === chosen) && (
              <ChoiceRow single name="scope" selected onToggle={() => {}} title={chosenName} />
            )}
          </div>
          <div>
            <FieldLabel htmlFor="debug-someone">Someone else</FieldLabel>
            <Input id="debug-someone" placeholder="Choose a person" aria-label="Someone else: choose a person" value={query} onChange={(e) => setQuery(e.target.value)} />
            <div className="mt-1.5 flex flex-col gap-1.5">
              {matches.map((u) => (
                <ChoiceRow
                  key={u.id}
                  single
                  name="scope"
                  selected={chosen === u.id}
                  locked={current === u.id}
                  lockedNote="Captured now"
                  onToggle={() => setChosen(u.id)}
                  title={u.name}
                  meta={u.email}
                />
              ))}
            </div>
          </div>
          <p className="text-[13px] leading-[1.5] text-faint">
            Syndra keeps what it sees in memory for an hour, never in its database. Names and emails show, because this is for you to debug. Signatures and secrets never do.
          </p>
          {outcome && <ActionOutcome outcome={outcome} />}
        </div>
      )}
      <ModalFooter
        note={
          done
            ? undefined
            : mode === "on"
              ? `Ends at ${clockOf(ends)}, 15 minutes from now. While it runs, choosing another account moves the capture to them. Change history records each.`
              : chosen
                ? `Moves the capture to ${chosenName} from now on. It still ends at ${clockOf(ends)}. ${current === selfId ? "Your" : `${nameOf(current ?? "")}'s`} sign-ins seen so far stay in the list. Change history records the switch.`
                : `It still ends at ${clockOf(ends)}. A switch never makes a capture longer.`
        }
      >
        {done ? (
          <Button onClick={onClose}>Close</Button>
        ) : (
          <>
            <Button onClick={onClose}>Cancel</Button>
            <Button variant="accent" isPending={start.isPending} disabled={!chosen || chosen === current} onClick={confirm}>
              {verb}
            </Button>
          </>
        )}
      </ModalFooter>
    </Modal>
  );
}
