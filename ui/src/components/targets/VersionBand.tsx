"use client";

import { useState } from "react";

import { Relative } from "@/components/ui/Time";
import { Badge, STATUS_TONE, StatusDot, type StatusTone } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Card } from "@/components/ui/Card";
import { Input } from "@/components/ui/Input";
import { RoleRef, UserName } from "@/components/names";
import {
  usePublishMappingVersion,
  type MappingChange,
  type MappingHistory,
} from "@/lib/queries/useMappings";

/**
 * The version band (design M1, M2, M5). One seat under the page title, three
 * readings, and it never changes shape.
 *
 * Publish is inert rather than absent when there is nothing to publish, and the
 * version count keeps its seat at zero, because a band that appeared with its
 * first unpublished edit would be structure moving in response to data — on the
 * one screen where an operator most needs to know that the thing they are
 * reading is or is not what a rollback would restore.
 *
 * The three readings, and why each is the tone it is:
 *
 *   clean       lime      the working copy is version N. Nothing pending.
 *   unpublished amber     version N plus edits nobody has published. A normal
 *                         working state, not a fault — so amber, never red.
 *   none        neutral   nothing published yet. Not a deficiency on day two,
 *                         so neither lime nor amber; see the neutral tone.
 *
 * When there are unpublished edits the band ENUMERATES them. That is the whole
 * argument for this screen existing: "rolling back undoes work listed nowhere"
 * is true only while nothing lists it, and four rows discharge it.
 */
export function VersionBand({ target, history }: { target: string; history?: MappingHistory }) {
  const [note, setNote] = useState("");
  const publish = usePublishMappingVersion(target);

  const versions = history?.versions ?? [];
  const changes = history?.unpublished_changes ?? [];
  const current = history?.current_version ?? 0;
  const nothingPublished = versions.length === 0;
  const pending = changes.length > 0;

  const reading: { tone: StatusTone; title: string } = nothingPublished
    ? { tone: "neutral", title: "Nothing has been published" }
    : pending
      ? {
          tone: "warn",
          title: `Version ${current}, plus ${count(changes.length, "edit")} nobody has published`,
        }
      : { tone: "healthy", title: `Working copy matches version ${current}` };

  const newest = versions[0];

  return (
    <Card>
      <div className="flex flex-wrap items-start gap-4 px-5 py-4">
        <div className="flex min-w-0 flex-1 flex-col gap-1.5">
          <span className="flex items-center gap-2.5">
            <StatusDot tone={reading.tone} />
            <span className={`text-[15.5px] font-semibold ${STATUS_TONE[reading.tone].label}`}>
              {reading.title}
            </span>
          </span>

          <p className="text-[13.5px] leading-[1.55] text-muted">
            {nothingPublished ? (
              <>
                No version exists yet, so there is nothing to roll back to and nothing to
                compare the working copy against.
              </>
            ) : pending ? (
              <>
                The rows below are the working copy and they are what Syndra converges
                against. Version {current} is what a rollback would return you to, and it
                does not contain {changes.length === 1 ? "this edit" : "these"}.
              </>
            ) : (
              <>
                Published <Relative iso={newest?.published_at} /> by{" "}
                <UserName id={newest?.published_by ?? ""} fallback="somebody" />
                {newest?.note ? ` — “${newest.note}”` : ""}. Nothing is waiting to be
                published, so what the rows below say is what version {current} says.
              </>
            )}
          </p>
        </div>

        <span className="flex shrink-0 items-center gap-3">
          <span className="text-[13px] text-faint">
            Versions ·{" "}
            <Badge hollow={versions.length === 0}>{versions.length}</Badge>
          </span>
          {/* Inert, never absent. The seat is what keeps the band one shape
              across all three readings. */}
          <Button
            size="sm"
            disabled={!pending || publish.isPending}
            isPending={publish.isPending}
            onClick={() => publish.mutate(note, { onSuccess: () => setNote("") })}
          >
            {pending ? `Publish as version ${current + 1}` : "Publish"}
          </Button>
        </span>
      </div>

      {pending && (
        <div className="row-divider px-5 py-4">
          <p className="mb-2.5 text-[13px] text-faint">
            {nothingPublished
              ? "Not yet in any version"
              : `The ${changes.length === 1 ? "one" : word(changes.length)} · what a rollback to version ${current} would undo`}
          </p>
          <div className="grid gap-1.5">
            {changes.map((change) => (
              <ChangeRow key={rowKey(change)} change={change} />
            ))}
          </div>

          {/* The sentence an operator needs most. Every other tool in their life
              has taught them that unpublished means not yet in effect, and here
              it is exactly backwards. */}
          <p className="mt-3 max-w-[78ch] text-[13.5px] leading-[1.55] text-muted">
            Each of these landed through its own rehearsal, so each one has already been
            approved once.{" "}
            <strong className="font-semibold text-ink">
              Publishing does not re-apply them — they are already what Syndra converges
              against.
            </strong>{" "}
            Publishing records this set as version {current + 1} so that a later rollback
            has something to return to.
          </p>

          <div className="mt-3 max-w-[46rem]">
            <Input
              aria-label="Why this set"
              placeholder="Why this set is the one to keep"
              value={note}
              onChange={(event) => setNote(event.target.value)}
            />
            <p className="mt-[7px] text-[13px] text-faint">
              A note is required to publish and never to edit: the note explains a set, and
              an edit is explained by its rehearsal.
            </p>
          </div>
        </div>
      )}

      {Boolean(publish.error) && (
        <div className="row-divider px-5 py-3 text-[13.5px] text-danger-text">
          {publish.error instanceof Error
            ? publish.error.message
            : "That set could not be published. Nothing was changed."}
        </div>
      )}
    </Card>
  );
}

const KIND: Record<MappingChange["kind"], { label: string; tone: StatusTone }> = {
  added: { label: "Added", tone: "healthy" },
  changed: { label: "Changed", tone: "warn" },
  removed: { label: "Removed", tone: "danger" },
};

/**
 * One unpublished edit: what it is, who made it, when, and how many people it
 * moved.
 *
 * A REMOVAL carries no actor and no time, and says so rather than leaving a
 * gap. A deleted row takes its `updated_by` with it and nothing records the
 * deletion — attributing it to whoever published the version would name
 * somebody who did not do it, and an empty space would read as a rendering
 * fault rather than as an absence of knowledge.
 */
function ChangeRow({ change }: { change: MappingChange }) {
  const kind = KIND[change.kind];
  return (
    <div className="flex flex-wrap items-baseline gap-x-2.5 gap-y-1 rounded-inner bg-tint-1 px-3.5 py-2.5 text-[13.5px]">
      <span className={`font-semibold ${STATUS_TONE[kind.tone].label}`}>{kind.label}</span>
      <span className="font-semibold">
        <RoleRef projectId={change.project_id} roleKey={change.role_key} />
      </span>
      <span className="type-mono text-muted">{change.field}</span>
      <span className="text-muted">
        {change.kind === "changed" ? (
          <>
            from <span className="type-mono">{change.was_value}</span> to{" "}
            <span className="type-mono">{change.value}</span>
          </>
        ) : change.kind === "removed" ? (
          <>
            was <span className="type-mono">{change.was_value}</span>
          </>
        ) : (
          <>
            → <span className="type-mono">{change.value}</span>
          </>
        )}
      </span>
      <span className="flex-1" />
      <span className="text-[13px] text-faint">
        {change.actor ? (
          <>
            <UserName id={change.actor} />
            {change.at ? (
              <>
                , <Relative iso={change.at} />
              </>
            ) : null}{" "}
            ·{" "}
          </>
        ) : (
          // Said, not left blank.
          <>Nothing records who removed it · </>
        )}
        {count(change.holders, "person", "people")}
      </span>
    </div>
  );
}

function rowKey(change: MappingChange): string {
  return `${change.kind}:${change.project_id}:${change.role_key}:${change.field}`;
}

function count(n: number, one: string, many = `${one}s`): string {
  return `${n} ${n === 1 ? one : many}`;
}

function word(n: number): string {
  return ["no", "one", "two", "three", "four", "five"][n] ?? String(n);
}
