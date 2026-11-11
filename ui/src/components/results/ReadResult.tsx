"use client";

import { ActivityList } from "@/components/results/ActivityList";
import { RecordsList } from "@/components/results/RecordsList";
import { StatusMeters } from "@/components/results/StatusMeters";
import { ALSO_SAYS, noMoreLines, notReadableBody, notReadableNext, notReadableTitle, type Reader } from "@/components/results/copy";
import { Notice } from "@/components/ui/Notice";
import { observed, ReadFreshness, useReadAgainPress, useReadAgainSince, type ReadState } from "@/components/ui/ReadFreshness";
import { TechnicalDetail, TechnicalPayload } from "@/components/ui/TechnicalDetail";
import { STALE } from "@/lib/queries/fresh";
import type { ActivityResult, ReadView, RecordsResult, StatusResult } from "@/lib/queries/useMyStorage";

/** What the add-on adds that Syndra does not know is shown, not trusted: the backend already caps it, and this is the second cap. */
const LINE_CAP = 400;
const LINES_CAP = 50;

export interface ReadProps {
  /** The system's name, as the page calls it. */
  name: string;
  reader: Reader;
}

/**
 * One member-scope read, drawn by its SHAPE and never by what the add-on is or
 * what the operation is called: records, status, activity, or raw. A shape the
 * core does not know is shown exactly as it came, behind Technical detail.
 */
export function ReadResult({ read, name, reader }: { read: ReadView } & ReadProps) {
  const { result } = read;
  return (
    <div className="flex flex-col gap-3 [overflow-wrap:anywhere]" data-read={read.operation}>
      {read.retry_after ? (
        // Read moments ago and nothing kept to show: the cooldown line, never an error.
        <ReadLine read={read} name={name} reader={reader} />
      ) : !read.readable || result === undefined ? (
        <Notice tone="warn" title={notReadableTitle(name)} next={notReadableNext(reader)}>
          {notReadableBody(name)}
        </Notice>
      ) : read.shape === "records" ? (
        <RecordsList result={result as RecordsResult} name={name} reader={reader} />
      ) : read.shape === "status" ? (
        <StatusMeters result={result as StatusResult} name={name} reader={reader} />
      ) : read.shape === "activity" ? (
        <ActivityList result={result as ActivityResult} name={name} reader={reader} />
      ) : (
        <TechnicalDetail>
          <TechnicalPayload>{typeof result === "string" ? result : JSON.stringify(result)}</TechnicalPayload>
        </TechnicalDetail>
      )}
      {read.also_says && read.also_says.length > 0 && <AlsoSays lines={read.also_says} />}
    </div>
  );
}

/** What one read says about its own age: its answer's, or the cooldown it ran into with nothing to show. */
function readStateOf(read: ReadView, since?: number): ReadState | null {
  if (read.retry_after) return { stale: { reason: STALE.cooldown, retry_after: read.retry_after, since } };
  const { result } = read;
  return read.readable && result && typeof result !== "string" ? observed(result) : null;
}

/**
 * The freshness line of one read, drawn alone: with its "Read again" when it is
 * a copy. For a read a kind draws itself, and for one refused inside the cooldown.
 */
export function ReadLine({ read, name, reader }: { read: ReadView } & ReadProps) {
  const again = useReadAgainPress();
  const state = readStateOf(read, useReadAgainSince());
  return state && <ReadFreshness state={state} subject={name} reader={reader} onRefresh={again} />;
}

/**
 * Every read the add-on answered, in the order it gave them; renders nothing
 * when there are none. `skip` is for a kind module that draws the body of one
 * of them itself: its freshness line stays here, so every read says how old it
 * is in one place.
 */
export function Reads({ reads, skip = [], ...props }: { reads?: ReadView[]; skip?: string[] } & ReadProps) {
  return (
    <>
      {(reads ?? []).map((read) =>
        skip.includes(read.operation) ? (
          readStateOf(read) && (
            <div key={read.operation} className="row-divider px-5 py-3">
              <ReadLine read={read} {...props} />
            </div>
          )
        ) : (
          <div key={read.operation} className="row-divider px-5 py-4">
            <ReadResult read={read} {...props} />
          </div>
        ),
      )}
    </>
  );
}

/** Raw, as quoted: React escapes it, and nothing here reads it. */
function AlsoSays({ lines }: { lines: string[] }) {
  const cut = (s: string) => (s.length > LINE_CAP ? `${s.slice(0, LINE_CAP)}…` : s);
  const over = lines.length - LINES_CAP;
  return (
    <div className="flex flex-col gap-1.5" data-also-says>
      <span className="text-[12.5px] font-semibold text-muted">{ALSO_SAYS}</span>
      <ul className="m-0 flex list-none flex-col gap-0.5 p-0 font-mono text-[12.5px] leading-[1.5] text-faint [overflow-wrap:anywhere]">
        {lines.slice(0, LINES_CAP).map((line, i) => (
          <li key={i}>{cut(line)}</li>
        ))}
        {over > 0 && <li>{noMoreLines(over)}</li>}
      </ul>
    </div>
  );
}
