"use client";

import { ActivityList } from "@/components/results/ActivityList";
import { RecordsList } from "@/components/results/RecordsList";
import { StatusMeters } from "@/components/results/StatusMeters";
import { ALSO_SAYS, noMoreLines, notReadableBody, notReadableNext, notReadableTitle, type Reader } from "@/components/results/copy";
import { Notice } from "@/components/ui/Notice";
import { TechnicalDetail, TechnicalPayload } from "@/components/ui/TechnicalDetail";
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
      {!read.readable || result === undefined ? (
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

/**
 * Every read the add-on answered, in the order it gave them; renders nothing
 * when there are none. `skip` is for a kind module that draws one of them
 * itself.
 */
export function Reads({ reads, skip = [], ...props }: { reads?: ReadView[]; skip?: string[] } & ReadProps) {
  const shown = (reads ?? []).filter((r) => !skip.includes(r.operation));
  if (shown.length === 0) return null;
  return (
    <>
      {shown.map((read) => (
        <div key={read.operation} className="row-divider px-5 py-4">
          <ReadResult read={read} {...props} />
        </div>
      ))}
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
