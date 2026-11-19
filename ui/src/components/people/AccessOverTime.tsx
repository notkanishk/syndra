"use client";

import { useShownNames } from "@/components/names/shownNames";
import { projectLabel } from "@/components/names/Unresolved";
import { CAPTION, CUT_SHORT, lineSentence, NOTE, NOTHING, placeRuns, READ_FAILED, TITLE, whenLine, yearOf } from "@/components/people/accessRuns";
import { ErrorState } from "@/components/states";
import { Card, CardHeader } from "@/components/ui/Card";
import { useAccessHistory } from "@/lib/queries/useUsers";
import { useTerms } from "@/lib/queries/useTerms";
import { compareNames } from "@/lib/sort";
import { termName } from "@/lib/term-dates";
import { civilDay } from "@/lib/time";

/** The name seat, as the board's (A-24 frame 19): from the tablet step, name · track. */
const GRID = "grid grid-cols-1 gap-y-1.5 @tablet:grid-cols-[230px_minmax(0,1fr)] @tablet:gap-x-4 @tablet:gap-y-0";
/** A line's name seat: the role in its project, its dates under it. Its two lines are the line's height. */
const NAME = "flex flex-col justify-center py-2";

/**
 * Access over time (A-24 §9 vivid, Q11; owner 19 Nov 2026): one card, a line per role from the day it
 * was given to the day it ended, on this year's axis with the terms marked and today in violet. Ink is
 * held today, grey has ended, the dashed amber run is the time left before an end. The usage band's
 * grammar (A-23): the strong baseline colours, a tick where the record starts short of the window,
 * the accent for today; spans, as the term line draws, not a chart. The dates are Syndra's records;
 * held today is the standing, the page's own read of Zitadel. Operators, in Advanced (the caller).
 */
export function AccessOverTime({ userId, lines }: { userId: string; lines: number }) {
  const today = civilDay(new Date());
  const { first, last, at } = yearOf(today);
  const history = useAccessHistory(userId, first);
  const terms = useTerms();
  const names = useShownNames();

  const project = (r: { project_name: string }) => projectLabel(r.project_name);
  const roles = [...(history.data?.roles ?? [])].sort(
    (a, b) => compareNames(project(a), project(b)) || compareNames(names.role(a.project_id, a.role_key), names.role(b.project_id, b.role_key)),
  );
  // Every term the year touches, where it starts (a term from last year at the axis's start).
  const marks = (terms.data ?? []).filter((t) => t.end >= first && t.start <= last).map((t) => ({ left: at(t.start), name: termName(t) }));
  const now = at(today);

  // Hairlines behind every track: each term's start, and today in violet (the board's gradient).
  const hairlines = (
    <>
      {marks.map((m) => (
        <span key={m.name + m.left} aria-hidden className="absolute inset-y-0 w-px bg-line-strong" style={{ left: `${m.left}%` }} />
      ))}
      <span aria-hidden data-today className="absolute inset-y-0 w-px bg-accent" style={{ left: `${now}%` }} />
    </>
  );

  let body;
  if (history.isPending) {
    body = (
      <div aria-busy="true" aria-label="Loading" className="flex flex-col">
        {Array.from({ length: Math.max(1, lines) }).map((_, i) => (
          // The same two lines of type as a real line, so the card is as tall before the read as after.
          <div key={i} className={`${GRID} ${i > 0 ? "border-t border-divider" : ""}`}>
            <span className={NAME}>
              <span className="text-body leading-[1.3]">
                <span className="skeleton-bar inline-block h-[10px] w-40 align-middle" />
              </span>
              <span className="text-small">&nbsp;</span>
            </span>
            <span className="flex min-h-6 items-center">
              <span className="skeleton-bar h-1.5 w-full" />
            </span>
          </div>
        ))}
      </div>
    );
  } else if (roles.length === 0) {
    body = <p className="text-body text-muted">{NOTHING}</p>;
  } else {
    body = (
      <ul className="flex flex-col">
        {roles.map((role, i) => {
          const where = project(role);
          const name = names.role(role.project_id, role.role_key);
          const { runs, ahead } = placeRuns(role, today);
          return (
            <li key={role.project_id + "/" + role.role_key} data-line className={`${GRID} ${i > 0 ? "border-t border-divider" : ""}`}>
              <span className="sr-only">{lineSentence(role, first, where, name)}</span>
              <span aria-hidden className={NAME}>
                <span className="text-body font-semibold leading-[1.3] [overflow-wrap:anywhere]">{`${name} in ${where}`}</span>
                <span data-figure className="text-small text-faint">
                  {whenLine(role, first)}
                </span>
              </span>
              <span aria-hidden className="relative min-h-6">
                {hairlines}
                {runs.map((r, j) => (
                  <span
                    key={j}
                    data-run={r.held ? "held" : "ended"}
                    data-clipped={r.clipped || undefined}
                    className={`absolute top-1/2 h-1.5 min-w-1.5 -translate-y-1/2 rounded-pill ${r.clipped ? "rounded-l-none" : ""} ${r.held ? "bg-ink" : "bg-line-strong"}`}
                    style={{ left: `${r.left}%`, width: `${r.width}%` }}
                  >
                    {/* Began before the year, or off the record: the band's tick at the window's edge. */}
                    {r.clipped && <span className="absolute -top-1 left-0 h-3.5 w-px bg-faint" />}
                  </span>
                ))}
                {ahead && (
                  <span data-ahead className="absolute top-1/2 -translate-y-1/2 border-t-2 border-dashed border-warn" style={{ left: `${ahead.left}%`, width: `${ahead.width}%` }} />
                )}
              </span>
            </li>
          );
        })}
        {/* The axis, under the tracks: each term where it starts, and today. */}
        <li aria-hidden className={GRID}>
          <span />
          <span data-axis data-figure className="relative h-6 text-fine text-faint">
            {marks
              .filter((m) => Math.abs(m.left - now) > 8)
              .map((m) => (
                <span key={m.name + m.left} className="absolute top-1.5 whitespace-nowrap pl-1.5" style={{ left: `${m.left}%` }}>
                  {m.name}
                </span>
              ))}
            <span className={`absolute top-1.5 whitespace-nowrap text-accent-text ${now > 85 ? "-translate-x-full pr-1.5" : "pl-1.5"}`} style={{ left: `${now}%` }}>
              Today
            </span>
          </span>
        </li>
      </ul>
    );
  }

  return (
    <Card role="group" aria-label={TITLE} className="@container">
      <CardHeader title={TITLE} note={NOTE} caption={history.data?.truncated ? `${CAPTION} ${CUT_SHORT}` : CAPTION} />
      {/* A failed read is the card's Notice, at the card's own inset (bare). */}
      {history.isError ? (
        <ErrorState bare title={READ_FAILED} error={history.error} onRetry={() => void history.refetch()} />
      ) : (
        <div className="px-(--card-x) pb-4">{body}</div>
      )}
    </Card>
  );
}
