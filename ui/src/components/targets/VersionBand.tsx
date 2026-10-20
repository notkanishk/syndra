"use client";

import { describeFailure, sentence } from "@/lib/outcome";
import { useState } from "react";

import { Relative } from "@/components/ui/Time";
import { STATUS_TONE, StatusDot, type StatusTone } from "@/components/ui/Badge";
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
 * Publish is inert rather than absent when there is nothing to publish (and
 * says so beneath it), because a band that appeared with its first unpublished
 * edit would be structure moving in response to data — on the one screen where
 * an operator most needs to know that the thing they are reading is or is not
 * what a rollback would restore. The version count lives on the published
 * versions card, as the board draws it.
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

  // The board's three bands (route-system-targets-id-mappings): a title, one
  // sentence, Publish at the right. Pending edits are already live, so the
  // sentence is where "publishing applies nothing again" is said.
  const reading: { tone: StatusTone; title: string } = pending
    ? {
        tone: "warn",
        title: nothingPublished
          ? "Nothing published yet"
          : `Version ${current}, plus ${count(changes.length, "change")} not yet published`,
      }
    : nothingPublished
      ? { tone: "neutral", title: "Nothing published yet" }
      : { tone: "healthy", title: `Matches version ${current}` };

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

          <p className="pl-[18px] text-[14px] leading-[1.55] text-muted [text-wrap:pretty]">
            {pending ? (
              <>
                The rows below are live: each change went through its own preview.{" "}
                <strong className="font-semibold text-ink">Publishing applies nothing again.</strong>{" "}
                It records {nothingPublished ? "them" : "the set"} as version {current + 1}, so a
                later rollback has something to return to.
                {!nothingPublished &&
                  ` Rolling back to version ${current} would undo ${changes.length === 1 ? "it" : `all ${word(changes.length)}`}.`}
              </>
            ) : nothingPublished ? (
              <>
                No version exists, so there is nothing to roll back to and nothing to compare the
                rows against.
              </>
            ) : (
              <>
                Published <Relative iso={newest?.published_at} /> by{" "}
                <UserName id={newest?.published_by ?? ""} fallback="somebody" />
                {newest?.note ? `: “${newest.note}”` : ""}. What the rows say is what version{" "}
                {current} says.
              </>
            )}
          </p>
        </div>

        <span className="flex shrink-0 flex-col items-end gap-1.5 whitespace-nowrap">
          {/* Inert, never absent. The seat is what keeps the band one shape
              across all three readings. */}
          {/* The band says a note is required and then published without one.
              The note is the whole audit record a rollback operator reads
              before deciding whether to come back — an empty one makes the
              version a date with no argument, which is the state the sentence
              beneath this field exists to prevent. The backend refuses it too;
              this is where somebody is told, not what enforces it. */}
          <Button
            size="sm"
            variant={pending ? "accent" : "outline"}
            disabled={!pending || !note.trim() || publish.isPending}
            isPending={publish.isPending}
            reason={
              !pending
                ? "Nothing to publish. This lights up once there's an unpublished edit to save."
                : !note.trim()
                  ? "Say why this set is the one to keep. It is what the next person to roll back reads."
                  : undefined
            }
            onClick={() => publish.mutate(note, { onSuccess: () => setNote("") })}
          >
            {pending ? `Publish as version ${current + 1}` : "Publish"}
          </Button>
          {!pending && <span className="text-[12.5px] text-faint">Nothing to publish</span>}
        </span>
      </div>

      {pending && (
        <div className="row-divider px-5 py-4">
          <p className="mb-2.5 text-[13px] text-faint">
            {nothingPublished
              ? "Not yet in any version"
              : `${changes.length === 1 ? "This one edit" : `These ${word(changes.length)} edits`} · what a rollback to version ${current} would undo`}
          </p>
          <div className="grid gap-1.5">
            {changes.map((change) => (
              <ChangeRow key={rowKey(change)} change={change} />
            ))}
          </div>

          <div className="mt-3 max-w-[46rem]">
            <Input
              aria-label="Why this set"
              placeholder="Why this set is the one to keep"
              value={note}
              onChange={(event) => setNote(event.target.value)}
            />
            <p className="mt-[7px] text-[13px] text-faint">
              A note is required: it is the only record of why this set was right.
            </p>
          </div>
        </div>
      )}

      {Boolean(publish.error) && (
        <div className="row-divider px-5 py-3 text-[13.5px] text-danger-text">
          {sentence(describeFailure(publish.error))}
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
        {count(change.holders, "person affected", "people affected")}
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
