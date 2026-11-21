"use client";

import { notRead } from "@/lib/queries/read";
import { ActivityList } from "@/components/results/ActivityList";
import { RecordsList } from "@/components/results/RecordsList";
import { StatusMeters } from "@/components/results/StatusMeters";
import { ALSO_SAYS, noMoreLines, type Reader } from "@/components/results/copy";
import { ReadFreshness, useReadAgainPress } from "@/components/ui/ReadFreshness";
import { TechnicalDetail, TechnicalPayload } from "@/components/ui/TechnicalDetail";
import type { Read } from "@/lib/queries/read";
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
  const r = read.read;
  // A read with nothing to give says why and its next step, once (design §13.1): never a list, a meter or a date.
  if (r.state !== "now" && r.state !== "kept") {
    return (
      <div className="flex flex-col gap-3 [overflow-wrap:anywhere]" data-read={read.operation}>
        <ReadLine read={read} name={name} reader={reader} />
      </div>
    );
  }
  const result = r.data;
  return (
    <div className="flex flex-col gap-3 [overflow-wrap:anywhere]" data-read={read.operation}>
      {read.shape === "records" ? (
        <RecordsList result={result as RecordsResult} read={r} name={name} reader={reader} />
      ) : read.shape === "status" ? (
        <StatusMeters result={result as StatusResult} read={r} name={name} reader={reader} />
      ) : read.shape === "activity" ? (
        <ActivityList result={result as ActivityResult} read={r} name={name} reader={reader} />
      ) : (
        <TechnicalDetail>
          <TechnicalPayload>{typeof result === "string" ? result : JSON.stringify(result)}</TechnicalPayload>
        </TechnicalDetail>
      )}
      {read.also_says && read.also_says.length > 0 && <AlsoSays lines={read.also_says} />}
    </div>
  );
}

/**
 * The freshness line of one read, drawn alone: its age with "Read again" when it
 * is a copy, or why it has nothing (design §13.1). For a read a kind draws itself,
 * and for one that has no data.
 */
export function ReadLine({ read, name, reader }: { read: ReadView } & ReadProps) {
  return <FreshnessOf read={read.read} name={name} reader={reader} />;
}

/** One read's line, under the page's Read again: the one way a result renderer says its age or its why. */
export function FreshnessOf({ read, name, reader }: { read: Read<unknown> } & ReadProps) {
  const again = useReadAgainPress();
  return <ReadFreshness read={read} subject={name} system={name} reader={reader} onRefresh={again} />;
}

/**
 * Every read the add-on answered, in the order it gave them; renders nothing
 * when there are none. `skip` is for a kind module that draws the body of one
 * of them itself: its freshness line stays here, so every read says how old it
 * is in one place.
 */
export const saysAge = (read: ReadView) => notRead(read.read) || typeof read.read.data !== "string";

export function Reads({ reads, skip = [], ...props }: { reads?: ReadView[]; skip?: string[] } & ReadProps) {
  return (
    <>
      {(reads ?? []).map((read) =>
        skip.includes(read.operation) ? (
          // A raw answer (a string) has no age to say; any other read does, or its why.
          saysAge(read) && (
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
      <span className="text-fine font-semibold text-muted">{ALSO_SAYS}</span>
      <ul className="m-0 flex list-none flex-col gap-0.5 p-0 font-mono text-fine leading-[1.5] text-faint [overflow-wrap:anywhere]">
        {lines.slice(0, LINES_CAP).map((line, i) => (
          <li key={i}>{cut(line)}</li>
        ))}
        {over > 0 && <li>{noMoreLines(over)}</li>}
      </ul>
    </div>
  );
}
