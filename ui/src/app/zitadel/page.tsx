"use client";

import { Term } from "@/components/ui/Term";
import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { ErrorState, RowSkeleton } from "@/components/states";
import { ArrowLink } from "@/components/ui/ArrowLink";
import { ActionForecast, ActionOutcome } from "@/components/ui/ActionOutcome";
import { Mono, type StatusTone } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Card, CardHeader } from "@/components/ui/Card";
import { FoldChevron } from "@/components/ui/FoldChevron";
import { Modal, ModalFooter, ModalHeader, mountedWhileOpen } from "@/components/ui/Modal";
import { Notice } from "@/components/ui/Notice";
import { MetaRow, PageHeader } from "@/components/ui/PageHeader";
import { StatusSeat } from "@/components/ui/StatusSeat";
import { Unfold } from "@/lib/motion";
import { ClaimsDelivery } from "@/components/system/ClaimsDelivery";
import { SigningKeyGuide, replaceResult, type RotateResult } from "@/components/system/SigningKeyGuide";
import { request } from "@/lib/api-client";
import { formatAt, formatShortDate } from "@/lib/format";
import { FORECAST, outcomeFromError, type ActionOutcome as Outcome } from "@/lib/outcome";
import { useClaimsDelivery, type ClaimsDelivery as Delivery } from "@/lib/queries/useClaimsDelivery";
import { useProjects } from "@/lib/queries/useProjects";
import { useZitadelHealth } from "@/lib/queries/useZitadel";
import { invalidateAfter } from "@/lib/queries/invalidation";
import { CheckPips, zitadelReading } from "@/components/targets/TargetReading";
import { useMissedChecks } from "@/lib/useMissedChecks";

interface RotationStatus {
  key_installed?: boolean;
  last_rotated_at?: string | null;
  /** syndra: Syndra's own record; keyring: Syndra saw Zitadel's key change; env: the .env date only. */
  last_rotated_source?: "syndra" | "keyring" | "env";
  age_days?: number;
  threshold_days?: number;
  status?: "disabled" | "unknown" | "ok" | "warn" | "stale" | string;
  rotate_command?: string;
  /** The register command, with the server checkout prefixed when configured. */
  register_command?: string;
  due_at?: string;
  auto?: {
    on: boolean;
    off_reason?: "switched_off" | "no_zitadel";
    lead_days?: number;
    window?: string;
    next_attempt_at?: string;
  };
  rotating?: boolean;
  last_outcome?: {
    trigger: "auto" | "button";
    started_at: string;
    finished_at?: string;
    outcome: "in_flight" | "verified" | "unverified" | "failed" | "abandoned";
    replaced?: string[];
    probe?: { pass: boolean; class?: string; detail?: string };
    error?: string;
  };
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
    // While Syndra is replacing the key (on its own, or from another tab), the row follows it.
    refetchInterval: (query) => (query.state.data?.rotating ? 5_000 : false),
  });

  const keyring = useClaimsDelivery().data?.keyring;

  const live = health.data?.status === "ok";
  const notConfigured = health.data?.status === "disabled";
  // A count is said only once it has been read: while projects load there is
  // none, and the meta says the word and nothing else (A-20 P5).
  const projectCount: number | undefined = live
    ? (health.data?.projects_total ?? projects.data?.length)
    : projects.data?.length;
  const misses = useMissedChecks(
    (health as { dataUpdatedAt?: number }).dataUpdatedAt ?? health.data,
    health.data ? live || notConfigured : undefined,
  );
  const reading = zitadelReading({ status: health.data?.status, misses });
  const keyOverdue = rotation.data?.status === "warn" || rotation.data?.status === "stale";

  return (
    <div className="flex flex-col gap-4">
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
              : projectCount === undefined
                ? null
                : `${projectCount} project${projectCount === 1 ? "" : "s"}${live ? "" : ", last known"}`}
          </MetaRow>
        }
      />

      <Unfold open={keyOverdue && rotation.data?.age_days !== undefined} gap={16}>
        <Notice tone="warn" title={`The action signing key is ${rotation.data?.age_days} days old.`} live>
          It is past the {rotation.data?.threshold_days ?? 90}-day limit and still works.{" "}
          {rotation.data?.auto?.on
            ? "Syndra replaces it on its own in the next quiet hours, and sign-ins carry on."
            : "Replacing it on its own is switched off: Replace now, below, does it, and sign-ins carry on."}
        </Notice>
      </Unfold>

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
          reading={reading}
          live={live}
          notConfigured={notConfigured}
          misses={misses}
          onCheckAgain={() => health.refetch()}
          checking={health.isFetching}
          latency={health.data?.latency_ms}
          detail={health.data?.error}
          projectCount={projectCount}
        />
      )}

      {/* Fixed, under the health line: what Zitadel is doing with claims (§4g). */}
      <ClaimsDelivery domain={health.data?.domain} connected={!notConfigured} />
      <SigningKey status={rotation.data} loading={rotation.isLoading} error={rotation.error} keyring={keyring} />
      <UpstreamInspection reachable={live} />
      <UpstreamWrites reachable={live} />
    </div>
  );
}

/**
 * The action signing key: Syndra replaces it on its own, and on demand.
 *
 * This reverses #71 ("no Rotate now button"). Owner, 5 Nov 2026, "Automatic +
 * button": Syndra replaces the key itself a few days before the limit, in the
 * quiet hours, checks that a signed test sign-in still gets its claims, and
 * records it; "Replace now" does the same on demand. #71 was right while the
 * only way to rotate was a command whose aftermath (a restart, a .env edit)
 * lived on the server. Syndra now takes the new keys straight from Zitadel's
 * answer, for the claims action and the event listener alike, so there is no
 * aftermath left for a person to do. See openspec/changes/action-key-autorotation
 * and docs/adr/005-syndra-replaces-the-action-signing-key.md.
 *
 * "Do it by hand" (and "How to set it up") open the designed guide
 * (route-zitadel §2, components/system/SigningKeyGuide): for when the
 * automatic way is off, has failed, or Zitadel refuses Syndra the write.
 */
function SigningKey({
  status,
  loading,
  error,
  keyring,
}: {
  status?: RotationStatus;
  loading: boolean;
  error: unknown;
  keyring?: Delivery["keyring"];
}) {
  const [guiding, setGuiding] = useState(false);
  const [replacing, setReplacing] = useState(false);
  const notSetUp = !status?.key_installed && !error;
  const { tone, word } = keyState(status, error);
  const inFlight = isRotating(status);
  const noZitadel = status?.auto?.off_reason === "no_zitadel";

  return (
    <Card>
      <div className="flex flex-wrap items-center gap-x-6 gap-y-3 px-5 py-[18px]">
        <div className="flex min-w-0 flex-[1_1_360px] flex-col gap-1.5">
          <div className="flex flex-wrap items-center gap-x-3.5 gap-y-1.5">
            <span className="type-card-title">Action signing key</span>
            {!loading && <StatusSeat tone={tone} word={word} />}
          </div>
          <p className="max-w-[80ch] text-[14px] leading-[1.55] text-muted">
            {loading ? "Checking when the key was last replaced…" : rotationSentence(status, error)}
          </p>
          {!loading && keyring && (
            <p className="max-w-[80ch] text-[13.5px] leading-[1.55] text-muted">{keyringSentence(keyring)}</p>
          )}
        </div>
        {!loading && !error && !inFlight && (
          <span className="flex flex-wrap items-start gap-2.5">
            {status?.key_installed && (
              <Button
                size="sm"
                variant="outline"
                disabled={noZitadel}
                reason={noZitadel ? "Syndra isn't connected to Zitadel." : undefined}
                onClick={() => setReplacing(true)}
              >
                Replace now
              </Button>
            )}
            <Button size="sm" variant="outline" onClick={() => setGuiding(true)}>
              {notSetUp ? "How to set it up" : "Do it by hand"}
            </Button>
          </span>
        )}
      </div>

      <ReplaceKeyDialog open={replacing} onClose={() => setReplacing(false)} />
      <SigningKeyGuide open={guiding} status={status} onClose={() => setGuiding(false)} />
    </Card>
  );
}

/**
 * Replace now: one sentence, Cancel first (and focused), then the act. While
 * the request is out the seat says what Syndra will do; the sheet then becomes
 * its own result, with Close.
 */
const ReplaceKeyDialog = mountedWhileOpen(function ReplaceKeyDialog({
  onClose,
}: {
  open: boolean;
  onClose: () => void;
}) {
  const client = useQueryClient();
  // final: the server answered with what happened, so there is nothing to press twice.
  const [result, setResult] = useState<{ outcome: Outcome; word: string; final: boolean } | null>(null);
  const replace = useMutation({
    mutationFn: () =>
      request<RotateResult>("/zitadel/action-key/rotate", { method: "POST", body: { force: true } }),
    onSettled: () => invalidateAfter(client, "signingKey"),
  });
  const done = result?.final ?? false;

  return (
    <Modal open onClose={onClose} busy={replace.isPending} size="sm" labelledBy="replace-key-title">
      <ModalHeader
        title="Replace the action signing key"
        titleId="replace-key-title"
        lede="Zitadel makes a new key and Syndra starts using it at once, so sign-ins carry on."
      />
      {!result && replace.isPending && <ActionForecast className="mx-6 mb-1" word={FORECAST.replace} />}
      {result && <ActionOutcome outcome={result.outcome} word={result.word} className="mx-6 mb-1" />}
      <ModalFooter
        cancel={done ? undefined : { onClick: onClose, disabled: replace.isPending }}
        note={
          result
            ? undefined
            : "Syndra then signs a test sign-in with the new key and checks it gets its claims."
        }
      >
        {done ? (
          <Button variant="outline" onClick={onClose}>
            Close
          </Button>
        ) : (
          <Button
            variant="accent"
            isPending={replace.isPending}
            onClick={async () => {
              setResult(null);
              try {
                setResult({ ...replaceResult(await replace.mutateAsync()), final: true });
              } catch (err) {
                // Refused before anything happened: the act stays, to try again.
                setResult({ outcome: outcomeFromError(err), word: "Not replaced", final: false });
              }
            }}
          >
            Replace the key
          </Button>
        )}
      </ModalFooter>
    </Modal>
  );
});

/** What the keyring last did: Syndra follows both keys in Zitadel on its own (S5). */
function keyringSentence(k: Delivery["keyring"]): string {
  if (k.error?.status === 403) {
    return "Syndra isn't allowed to read the key from Zitadel, so it checks with the key in .env alone. Give Syndra's service account the read-only IAM_OWNER_VIEWER role in Zitadel.";
  }
  if (k.error) {
    return "Syndra couldn't read the key from Zitadel on its last try. The key in .env and the last key it read keep working.";
  }
  if (!k.read_at) return "Syndra hasn't read the key from Zitadel yet. The key in .env works meanwhile.";
  const changed = k.changed_at ? ` It last changed ${formatAt(k.changed_at)}.` : "";
  return `Syndra reads the claims key and the event listener's key from Zitadel every minute, and right after a call it can't verify, so a new key never needs a restart. Last read ${formatAt(k.read_at)}.${changed}`;
}

function isRotating(status: RotationStatus | undefined): boolean {
  return Boolean(status?.rotating || status?.last_outcome?.outcome === "in_flight");
}

/**
 * The last attempt, unless it is a failure that a later replacement (by hand,
 * say) has overtaken: then it says nothing about the key in use.
 */
function latestAttempt(status: RotationStatus): RotationStatus["last_outcome"] {
  const lo = status.last_outcome;
  if (!lo) return undefined;
  const failed = lo.outcome === "failed" || lo.outcome === "abandoned";
  const overtaken = status.last_rotated_at && Date.parse(lo.started_at) < Date.parse(status.last_rotated_at);
  if (failed && overtaken) return undefined;
  return lo;
}

function keyState(
  status: RotationStatus | undefined,
  error: unknown,
): { tone: StatusTone; word: string } {
  if (error) return { tone: "warn", word: "Unknown" };
  if (!status?.key_installed) return { tone: "warn", word: "Not set up" };
  if (isRotating(status)) return { tone: "neutral", word: "Replacing" };
  const lo = latestAttempt(status);
  if (lo?.outcome === "unverified") return { tone: "warn", word: "Not confirmed" };
  if (lo?.outcome === "failed" || lo?.outcome === "abandoned") return { tone: "warn", word: "Not replaced" };
  if (status.age_days === undefined) return { tone: "warn", word: "Age unknown" };
  if (status.status === "warn" || status.status === "stale") {
    return { tone: "warn", word: "Past its limit" };
  }
  return { tone: "healthy", word: "Within its limit" };
}

/**
 * One sentence carrying state, cause and consequence. `disabled` is the case
 * worth spelling out: it reads like "not set up yet" and actually means every
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
  if (isRotating(status)) {
    return "Replacing… Zitadel is making a new key and Syndra starts using it the moment it arrives. Sign-ins carry on.";
  }

  const auto = status.auto?.on ?? false;
  const quiet = status.auto?.window ? ` (${status.auto.window})` : "";
  const nextTry = status.auto?.next_attempt_at ? `, ${formatAt(status.auto.next_attempt_at)}` : "";
  const due = status.due_at ? formatShortDate(status.due_at) : "";
  const lo = latestAttempt(status);

  if (lo?.outcome === "unverified") {
    return `Replaced ${formatAt(lo.finished_at ?? lo.started_at)}, but the test sign-in after it didn't get its claims. Syndra still accepts the old key and the new one, and won't replace it again on its own. Claims delivery, above, says what went wrong.`;
  }
  if (lo?.outcome === "failed" || lo?.outcome === "abandoned") {
    const what = lo.outcome === "failed" ? "Zitadel didn't replace it" : "the attempt was cut off before it finished";
    const then = auto
      ? ` Syndra tries again in the next quiet hours${nextTry}, at most once a day.`
      : " Replace it now, or by hand below.";
    return `Syndra tried to replace the key ${formatAt(lo.started_at)} and ${what}. The old key still works.${then}`;
  }
  if (lo?.outcome === "verified" && status.last_rotated_source === "syndra") {
    const when = formatShortDate(lo.finished_at ?? status.last_rotated_at);
    const next = due ? ` Next by ${due}.` : "";
    return lo.trigger === "auto" ? `Replaced on its own on ${when}.${next}` : `Replaced on ${when}.${next}`;
  }

  if (!auto) {
    const why =
      status.auto?.off_reason === "no_zitadel"
        ? "Syndra isn't connected to Zitadel, so it can't replace the key on its own."
        : "Replacing it on its own is switched off.";
    if (status.status === "warn" || status.status === "stale") return `${why} Replace it now, or by hand below.`;
    return due ? `Due on ${due}. ${why} Replace it before then.` : `${why} Syndra doesn't know when it was last replaced.`;
  }
  if (status.age_days === undefined) {
    return `Syndra doesn't know when the key was last replaced, so it replaces it on its own in the next quiet hours${quiet}.`;
  }
  // Overdue: the Notice above the page says how old it is; the row says what happens next.
  if (status.status === "warn" || status.status === "stale") {
    return `Syndra replaces it on its own in the next quiet hours${nextTry || quiet}.`;
  }
  return due ? `Due on ${due}; Syndra replaces it on its own before then.` : `Syndra replaces it on its own before the ${status.threshold_days ?? 90}-day limit.`;
}

/**
 * Health as one line. Reachability is the only fact the backend can answer
 * today; when it can't be answered the Notice says why. The line is still a
 * sentence with a cause, never a dot on its own.
 */
function HealthLine({
  reading,
  live,
  notConfigured,
  misses,
  onCheckAgain,
  checking,
  latency,
  detail,
  projectCount,
}: {
  reading: ReturnType<typeof zitadelReading>;
  live: boolean;
  notConfigured: boolean;
  misses: number;
  onCheckAgain: () => void;
  checking?: boolean;
  latency?: number;
  detail?: string;
  projectCount?: number;
}) {
  const word = reading.word;
  const sees = projectCount === undefined ? "" : `, and sees ${projectCount} project${projectCount === 1 ? "" : "s"}`;
  // One or two missed checks are shown as pips and raise no Notice; three in a row is Offline.
  const pips = !live && !notConfigured;

  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-wrap items-center gap-x-2.5 gap-y-1.5 px-5 text-[13.5px]">
        <StatusSeat tone={reading.tone} word={word} />
        {pips && <CheckPips recent={Array(Math.min(misses, 3)).fill("missed")} />}
        <span className="text-muted">
          {live
            ? `in ${latency ?? 0} ms${sees}`
            : notConfigured
              ? "Syndra is not set up to talk to Zitadel"
              : "the projects check waits for it"}
        </span>
        <Button size="sm" variant="outline" onClick={onCheckAgain} disabled={checking}>
          Check again
        </Button>
      </div>

      <Unfold open={notConfigured || reading.offline} gap={16}>
        <Notice tone={notConfigured ? "warn" : "neutral"} title={`${word}.`}>
          {notConfigured
            ? "Syndra is not set up to talk to Zitadel, so nothing decided here changes anyone's real access. "
            : "Three checks in a row went unanswered. Syndra keeps checking. "}
          {detail ||
            "Syndra is showing what it last knew. Nothing is lost: changes waiting for you to send stay in order and go through when Zitadel is back."}
        </Notice>
      </Unfold>
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
      <CardHeader title="Look inside Zitadel" note="What Zitadel holds, read straight from it." />
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
 * each says its own truth (#75). Since W1a both are recorded: a person's roles
 * go through the person page's pipe, a role through Access › Roles' pipe
 * (trace, then Zitadel, then a read-back). What still differs is that a role
 * delete takes it from everybody Zitadel shows holding it, once confirmed.
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
          <strong className="font-semibold text-ink">Recorded in Syndra, then changed in Zitadel at once.</strong>{" "}
          The same as making a role under Access › Roles. Deleting a role somebody holds is refused
          until you confirm who loses it.
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
    <Card>
      <button
        type="button"
        onClick={() => setOpen((value) => !value)}
        aria-expanded={open}
        className="flex min-h-[var(--tap)] w-full flex-wrap items-center gap-x-3 gap-y-2 px-5 py-4 text-left"
      >
        <FoldChevron open={open} />
        <span className="type-card-title text-danger-text">Change Zitadel directly</span>
        <span className="text-[13.5px] text-faint">last resort</span>
      </button>

      <Unfold open={open}>
        {doors.map((door) => (
          <div key={door.href} className="row-divider flex flex-wrap items-center gap-x-[18px] gap-y-2.5 px-5 py-3.5">
            <span className="flex min-w-0 flex-[1_1_320px] flex-col gap-1">
              <span className="text-[14.5px] font-semibold">{door.label}</span>
              <span className="max-w-[80ch] text-[13.5px] leading-[1.55] text-muted">{door.warning}</span>
              <span className="max-w-[80ch] text-[13.5px] leading-[1.55] text-muted">
                {door.instead} <ArrowLink href={door.syndraHref}>{door.syndra}</ArrowLink>
              </span>
            </span>
            {reachable ? (
              <ArrowLink href={door.href}>{door.door}</ArrowLink>
            ) : (
              <span className="text-[13.5px] font-semibold text-faint">{door.door}</span>
            )}
          </div>
        ))}

        {!reachable && (
          <div className="row-divider px-5 py-3 text-[13.5px] text-muted">
            Unavailable while Zitadel isn&rsquo;t answering: these change it directly.
          </div>
        )}
      </Unfold>
    </Card>
  );
}
