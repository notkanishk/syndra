"use client";

import { Term } from "@/components/ui/Term";
import { useState } from "react";
import { useQuery } from "@tanstack/react-query";

import { ErrorState, RowSkeleton } from "@/components/states";
import { ArrowLink } from "@/components/ui/ArrowLink";
import { Mono, StatusDot, type StatusTone } from "@/components/ui/Badge";
import { Button, ButtonLink } from "@/components/ui/Button";
import { Card, CardHeader } from "@/components/ui/Card";
import { CommandBlock } from "@/components/ui/CommandBlock";
import { Notice } from "@/components/ui/Notice";
import { MetaRow, PageHeader } from "@/components/ui/PageHeader";
import { Unfold } from "@/components/ui/Unfold";
import { request } from "@/lib/api-client";
import { useProjects } from "@/lib/queries/useProjects";
import { useZitadelHealth } from "@/lib/queries/useZitadel";

interface RotationStatus {
  key_installed?: boolean;
  last_rotated_at?: string | null;
  age_days?: number;
  threshold_days?: number;
  status?: "ok" | "warn" | "stale" | string;
  rotate_command?: string;
}

/**
 * S9 · System › Identity provider.
 *
 * Read-mostly. Zitadel owns authorization state; this screen answers "is it
 * reachable, does it agree with us, and when was the signing key last rotated"
 * — it is not a second console for editing it.
 *
 * Health is a SENTENCE with a cause and a timestamp, never a green or red dot
 * on its own. A dot tells an operator that something is wrong and nothing else;
 * the sentence tells them what broke, when it last worked, and what Syndra is
 * doing about it in the meantime.
 */
export default function IdentityProviderPage() {
  const health = useZitadelHealth();
  const projects = useProjects();
  const rotation = useQuery({
    queryKey: ["zitadel", "rotation"],
    queryFn: () => request<RotationStatus>("/zitadel/action-rotation-status"),
    retry: false,
  });

  const live = health.data?.status === "ok";
  const notConfigured = health.data?.status === "disabled";
  const projectCount = live
    ? (health.data?.projects_total ?? projects.data?.length ?? 0)
    : (projects.data?.length ?? 0);
  const keyOverdue = rotation.data?.status === "warn" || rotation.data?.status === "stale";

  return (
    <div className="flex flex-col gap-4">
      {keyOverdue && rotation.data?.age_days !== undefined && (
        <Notice tone="warn" title={`The action signing key is ${rotation.data.age_days} days old.`} live>
          It is past the {rotation.data.threshold_days ?? 90}-day limit and still works. Replace it
          when nobody is working: How to replace it, below, walks through it.
        </Notice>
      )}

      <PageHeader
        title="Zitadel"
        lede={
          <>
            Whether Syndra can reach <Term name="zitadel">Zitadel</Term>, and what it holds
            right now. Nothing on this page changes anyone’s access.
          </>
        }
        meta={
          <MetaRow>
            {health.data?.domain ? <Mono>{health.data.domain}</Mono> : null}
            {health.isLoading || health.error
              ? null
              : `${projectCount} project${projectCount === 1 ? "" : "s"}${live ? "" : ", last known"}`}
          </MetaRow>
        }
      />

      {health.isLoading ? (
        <Card>
          <RowSkeleton rows={2} avatar={false} label="Checking whether Syndra can reach Zitadel" />
        </Card>
      ) : health.error ? (
        <ErrorState
          title="Couldn't check whether Zitadel is reachable."
          error={health.error}
          onRetry={() => health.refetch()}
        />
      ) : (
        <HealthLine
          live={live}
          notConfigured={notConfigured}
          latency={health.data?.latency_ms}
          detail={health.data?.error}
          projectCount={projectCount}
        />
      )}

      <SigningKey status={rotation.data} loading={rotation.isLoading} error={rotation.error} />
      <UpstreamInspection reachable={live} />
      <UpstreamWrites reachable={live} />
      <TheName />
    </div>
  );
}

/**
 * The one place the name is explained to somebody who is already inside.
 *
 * It lives here rather than on its own page because this screen is about the
 * relationship it describes: Zitadel is the door, Syndra is the list. An
 * operator reading "is the provider reachable" is the one person for whom the
 * split is not decoration — it is the reason there are two systems.
 *
 * Deliberately last, deliberately quiet: no accent fill, no badge. Nothing on
 * this page is more important than the health sentence at the top.
 */
function TheName() {
  return (
    <Card className="px-5 py-4">
      <p className="type-card-title">Syn keeps the door. Syndra keeps the list.</p>
      <p className="mt-2 max-w-[68ch] text-[14px] leading-[1.6] text-muted">
        Syn keeps the door of Frigg&rsquo;s hall and bars it against those who should not enter.
        Zitadel is that door: it decides whether you are who you say you are. Syndra decides what
        that gets you, and keeps the record of why.
      </p>
    </Card>
  );
}

/**
 * The signing key, as one compact row — read-only, and deliberately so.
 *
 * There is no "Rotate now" button and there should not be one. Rotation is two
 * writes that must both land: Zitadel mints a new key, and the backend must be
 * restarted holding it. A click that does the first and not the second leaves
 * every claim-injection call failing signature verification, with the browser
 * showing a success toast. The operator running it in a terminal sees the exit
 * code, the new key, and the restart they still owe.
 *
 * "How to replace it" opens the guide, and the guide is CommandBlocks: the
 * command and everything that has to happen after it (#71). The button only
 * shows the steps; it never runs one.
 */
function SigningKey({
  status,
  loading,
  error,
}: {
  status?: RotationStatus;
  loading: boolean;
  error: unknown;
}) {
  const [open, setOpen] = useState(false);
  const rotateCommand = status?.rotate_command ?? "make zitadel-actions-rotate-key";
  const notSetUp = !status?.key_installed && !error;
  const { tone, word } = keyState(status, error);

  return (
    <Card>
      <div className="flex flex-wrap items-center gap-x-6 gap-y-3 px-5 py-[18px]">
        <div className="flex min-w-0 flex-[1_1_360px] flex-col gap-1.5">
          <div className="flex flex-wrap items-center gap-x-3.5 gap-y-1.5">
            <span className="type-card-title">Action signing key</span>
            {!loading && (
              <span className="inline-flex items-center gap-2 text-[13.5px] font-semibold">
                <StatusDot tone={tone} />
                {word}
              </span>
            )}
          </div>
          <p className="max-w-[80ch] text-[14px] leading-[1.55] text-muted">
            {loading ? "Checking when the key was last replaced…" : rotationSentence(status, error)}{" "}
            Zitadel signs every sign-in request with it, so Syndra can tell they are genuine.
          </p>
        </div>
        <Button
          size="sm"
          variant={tone === "warn" ? "accent" : "outline"}
          aria-expanded={open}
          onClick={() => setOpen((value) => !value)}
        >
          {notSetUp ? "How to set it up" : "How to replace it"}
        </Button>
      </div>

      <Unfold open={open}>
        <div className="row-divider flex flex-col gap-4 px-5 py-4">
          <p className="max-w-[84ch] text-[13.5px] leading-[1.55] text-muted">
            This is for the person who runs the Syndra server. It never expires on its own;
            someone has to decide to replace it.
          </p>

          {notSetUp ? (
            <CommandBlock
              tone="warn"
              command="make zitadel-actions-register"
              caption="No key is set up, so Syndra cannot tell genuine Zitadel requests from forged ones. Run this to register with Zitadel; it creates a key."
              steps={[
                <>
                  Copy the printed key into <Mono>ZITADEL_ACTION_SIGNING_KEY</Mono> in{" "}
                  <Mono>.env</Mono>.
                </>,
                <>
                  Restart the backend: <Mono>docker compose restart backend</Mono>.
                </>,
                <>
                  Confirm it took: <Mono>make zitadel-actions-verify</Mono>.
                </>,
              ]}
            />
          ) : (
            <CommandBlock
              command={rotateCommand}
              caption="Run this in a terminal on the server Syndra runs on. This panel does not replace the key itself — you must restart afterwards, and a half-done replacement is worse than an old key."
              steps={[
                <>
                  The command writes the new key to a file under <Mono>zitadel/actions/</Mono>,
                  one per registered Action, and backs up the old one alongside it.
                </>,
                <>
                  Put the new value in <Mono>ZITADEL_ACTION_SIGNING_KEY</Mono>, and the printed
                  timestamp in <Mono>ZITADEL_ACTION_SIGNING_KEY_ROTATED_AT</Mono> — without the
                  second one this panel goes back to reporting an unknown age.
                </>,
                <>
                  Restart the backend: <Mono>docker compose restart backend</Mono>.
                </>,
                <>
                  Confirm it took: <Mono>make zitadel-actions-verify</Mono>.
                </>,
              ]}
            />
          )}

          <p className="max-w-[84ch] text-[13.5px] leading-[1.55] text-muted">
            Between replacing the key and restarting, people can still sign in, but the access
            details Syndra normally adds at sign-in are missing. Keep the gap short and do it when
            nobody is working.
          </p>
        </div>
      </Unfold>
    </Card>
  );
}

function keyState(
  status: RotationStatus | undefined,
  error: unknown,
): { tone: StatusTone; word: string } {
  if (error) return { tone: "warn", word: "Unknown" };
  if (!status?.key_installed) return { tone: "warn", word: "Not set up" };
  if (status.age_days === undefined) return { tone: "warn", word: "Age unknown" };
  if (status.status === "warn" || status.status === "stale") {
    return { tone: "warn", word: "Past its limit" };
  }
  return { tone: "healthy", word: "Within its limit" };
}

/**
 * One sentence carrying state, cause and consequence. The stat card above
 * gives the headline; this says what it means. `disabled` is the case worth
 * spelling out — it reads like "not set up yet" and actually means every
 * inbound Action request is being trusted unchecked.
 */
function rotationSentence(status: RotationStatus | undefined, error: unknown): string {
  if (error) {
    return "Couldn't check when the key was last replaced. The key itself still works.";
  }
  if (!status) return "";

  if (!status.key_installed) {
    return "No key is set up, so Syndra accepts every request that claims to come from Zitadel without checking. Anyone who can reach Syndra on the network could change what a sign-in grants. Set up a key before real access depends on this.";
  }

  const threshold = status.threshold_days ?? 90;
  const age = status.age_days;

  if (age === undefined) {
    return `The key works, but Syndra does not know when it was last replaced, so it cannot warn you when the ${threshold}-day limit is reached. Set ZITADEL_ACTION_SIGNING_KEY_ROTATED_AT to the date the rotate command printed.`;
  }
  if (status.status === "stale") {
    return `The key is ${age} days old — more than twice the ${threshold}-day limit. Replace it now.`;
  }
  if (status.status === "warn") {
    return `The key is ${age} days old, over the ${threshold}-day limit. Plan to replace it.`;
  }
  return `The key is ${age} days old, within the ${threshold}-day limit. Nothing to do.`;
}

/** A round chevron that turns when its fold opens; the one fold control on this page. */
function Chevron({ open }: { open: boolean }) {
  return (
    <span
      aria-hidden
      className={`flex size-7 flex-none items-center justify-center rounded-pill border border-line-strong text-[13px] text-muted motion-turn ${
        open ? "rotate-90" : ""
      }`}
    >
      ›
    </span>
  );
}

/**
 * Health as one line, with the detail folded. Reachability is the only fact the
 * backend can answer today, so that is all the fold holds; when it can't be
 * answered there is nothing to fold and the Notice says why. The line is still
 * a sentence with a cause, never a dot on its own.
 */
function HealthLine({
  live,
  notConfigured,
  latency,
  detail,
  projectCount,
}: {
  live: boolean;
  notConfigured: boolean;
  latency?: number;
  detail?: string;
  projectCount: number;
}) {
  const [open, setOpen] = useState(false);
  const word = live ? "Answering" : notConfigured ? "Not connected" : "Not answering";

  return (
    <div className="flex flex-col gap-3">
      <div className="flex flex-wrap items-center gap-x-2.5 gap-y-1.5 px-5 text-[13.5px]">
        <span className="inline-flex items-center gap-2 font-semibold">
          <StatusDot tone={live ? "healthy" : "warn"} />
          {word}
        </span>
        <span className="text-muted">
          {live
            ? `in ${latency ?? 0} ms, and sees ${projectCount} project${projectCount === 1 ? "" : "s"}`
            : notConfigured
              ? "Syndra is not set up to talk to Zitadel"
              : "the projects check waits for it"}
        </span>
        <span className="flex-1" />
        {live && (
          <button
            type="button"
            aria-expanded={open}
            onClick={() => setOpen((value) => !value)}
            className="inline-flex min-h-11 items-center gap-2 font-semibold text-muted motion-tint hover:text-ink desktop:min-h-6"
          >
            {open ? "Hide checks" : "Show checks"}
            <Chevron open={open} />
          </button>
        )}
      </div>

      {live && (
        <Unfold open={open}>
          <Card>
            <div className="row-divider flex flex-col gap-1 px-5 py-3.5 tablet:flex-row tablet:items-baseline tablet:gap-[18px]">
              <span className="text-[14px] font-semibold tablet:w-[180px] tablet:shrink-0">Reachable</span>
              <span className="text-[13.5px] leading-[1.55] text-muted">
                Replied in {latency ?? 0} ms. Syndra is talking to Zitadel normally, and changes you
                send from Pending changes reach it as soon as you send them.
              </span>
            </div>
            <div className="row-divider flex flex-col gap-1 px-5 py-3.5 tablet:flex-row tablet:items-baseline tablet:gap-[18px]">
              <span className="text-[14px] font-semibold tablet:w-[180px] tablet:shrink-0">Projects</span>
              <span className="text-[13.5px] leading-[1.55] text-muted">
                Syndra&apos;s own Zitadel account can read the {projectCount} project{projectCount === 1 ? "" : "s"} it
                manages.
              </span>
            </div>
          </Card>
        </Unfold>
      )}

      {!live && (
        <Notice
          tone={notConfigured ? "warn" : "danger"}
          title={notConfigured ? "Not connected" : "Unreachable"}
        >
          {notConfigured
            ? "Syndra is not set up to talk to Zitadel, so nothing decided here changes anyone's real access. "
            : "Syndra's last attempt to reach Zitadel failed. "}
          {detail ||
            "Syndra is showing what it last knew. Nothing is lost: changes waiting for you to send stay in order and go through when Zitadel is back."}
        </Notice>
      )}
    </div>
  );
}

/**
 * Reads only. All three are unavailable while the provider is unreachable, with
 * the reason in visible copy rather than a tooltip: they read live, and there
 * is nothing to read.
 */
function UpstreamInspection({ reachable }: { reachable: boolean }) {
  // `sub` is the board's own line under each name (route-zitadel); the route is the link, never a label.
  const rows: Array<{ label: string; sub: string; open: string; href: string }> = [
    {
      label: "Projects and their roles",
      sub: "Each with how it compares with Syndra's record",
      open: "Open projects",
      href: "/zitadel/projects",
    },
    {
      label: "People",
      sub: "Everyone Zitadel knows, and the roles it holds for each",
      open: "Open people",
      href: "/zitadel/users",
    },
    {
      label: "Roles held",
      sub: "Every role on every person, in one list",
      open: "Open roles held",
      href: "/zitadel/grants",
    },
  ];

  return (
    <Card>
      <CardHeader
        title="Look inside Zitadel"
        note="What Zitadel holds, read straight from it. These pages only read: nothing on them changes anything."
      />
      {rows.map((row) => (
        <div key={row.href} className="row-divider flex flex-wrap items-center gap-x-[18px] gap-y-1.5 px-5 py-3.5">
          <span className="flex min-w-0 flex-[1_1_260px] flex-col gap-0.5">
            <span className="text-[14.5px] font-semibold">{row.label}</span>
            <span className="text-[13px] leading-[1.5] text-muted">{row.sub}</span>
          </span>
          {reachable ? (
            <ArrowLink href={row.href}>{row.open}</ArrowLink>
          ) : (
            <span className="text-[13.5px] font-semibold text-faint">{row.open}</span>
          )}
        </div>
      ))}
      {!reachable && (
        <div className="row-divider px-5 py-3 text-[13.5px] text-muted">
          Unavailable while Zitadel isn&rsquo;t answering: these read it live, and there is nothing
          to read.
        </div>
      )}
    </Card>
  );
}

/**
 * The escape hatches, deliberately folded and deliberately loud.
 *
 * Each control carries its own warning beside it (#72): a warning three
 * scroll-lengths above the button it applies to is a warning nobody reads. And
 * each says its own truth (#75): a person's roles go through Syndra's ledger
 * and Pending changes, so they are recorded; a project or role edit goes
 * straight out, so the next drift check lists it with nobody's name on it.
 * One warning for both would be false for one of them.
 */
function UpstreamWrites({ reachable }: { reachable: boolean }) {
  const [open, setOpen] = useState(false);

  const doors = [
    {
      label: "People’s roles",
      warning: (
        <>
          <strong className="font-semibold text-ink">Recorded in Syndra, but with no preview and no reason.</strong>{" "}
          It is written to Audit and waits in Pending changes until you send it.
        </>
      ),
      instead: "Inside Syndra: a person’s page, with a reason and a plan.",
      syndra: "People",
      syndraHref: "/users",
      door: "Change people’s roles",
      href: "/zitadel/users",
    },
    {
      label: "Projects and roles",
      warning: (
        <>
          <strong className="font-semibold text-danger-text">Goes straight to Zitadel, and Syndra keeps no record of it.</strong>{" "}
          One line in Audit and nothing else: no reason, no plan, no Change history. The next drift
          check lists it under Drift, with nobody’s name on it.
        </>
      ),
      instead: "Inside Syndra: a project’s page, which keeps track of every role.",
      syndra: "Projects",
      syndraHref: "/projects",
      door: "Change projects and roles",
      href: "/zitadel/projects",
    },
  ];

  return (
    <div className="rounded-card border border-dashed border-danger-line">
      <button
        type="button"
        onClick={() => setOpen((value) => !value)}
        aria-expanded={open}
        className="flex w-full flex-wrap items-center gap-x-3 gap-y-2 px-5 py-4 text-left"
      >
        <Chevron open={open} />
        <span className="type-card-title text-danger-text">Change Zitadel directly</span>
        <span className="text-[13.5px] text-faint">last resort</span>
      </button>

      <Unfold open={open}>
        <div className="danger-note mx-5 mb-4 px-4 py-3.5">
          <div className="text-[14.5px] font-semibold text-danger-text">
            Do not use these unless nothing else will do.
          </div>
          <p className="mt-1 max-w-[86ch] text-[14px] leading-[1.55] text-muted">
            Everything here can be done inside Syndra, with a reason and a record kept. Each control
            below says what it skips.
          </p>
        </div>

        {doors.map((door) => (
          <div key={door.href} className="row-divider flex flex-wrap items-center gap-x-[18px] gap-y-2.5 px-5 py-3.5">
            <span className="flex min-w-0 flex-[1_1_320px] flex-col gap-1">
              <span className="text-[14.5px] font-semibold">{door.label}</span>
              <span className="max-w-[80ch] text-[13.5px] leading-[1.55] text-muted">{door.warning}</span>
              <span className="max-w-[80ch] text-[13.5px] leading-[1.55] text-muted">
                {door.instead}{" "}
                <a href={door.syndraHref} className="font-semibold text-accent-text motion-tint hover:text-ink">
                  {door.syndra} →
                </a>
              </span>
            </span>
            {reachable ? (
              <ButtonLink href={door.href} variant="danger" size="sm">
                {door.door} →
              </ButtonLink>
            ) : (
              <Button variant="danger" size="sm" disabled>
                {door.door} →
              </Button>
            )}
          </div>
        ))}

        {!reachable && (
          <div className="row-divider px-5 py-3 text-[13.5px] text-muted">
            Unavailable while Zitadel isn&rsquo;t answering: these change it directly.
          </div>
        )}
      </Unfold>
    </div>
  );
}
