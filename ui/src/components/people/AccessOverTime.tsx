"use client";

import { useShownNames } from "@/components/names/shownNames";
import { projectLabel } from "@/components/names/Unresolved";
import {
  CAPTION,
  chartSummary,
  CUT_SHORT,
  lineName,
  lineSentence,
  NOTE,
  NOTHING,
  placeRuns,
  READ_FAILED,
  TITLE,
  whenLine,
  yearOf,
} from "@/components/people/accessRuns";
import { ErrorState } from "@/components/states";
import { Card, CardHeader } from "@/components/ui/Card";
import { useAccessHistory } from "@/lib/queries/useUsers";
import { useTerms } from "@/lib/queries/useTerms";
import { compareNames } from "@/lib/sort";
import { termName } from "@/lib/term-dates";
import { civilDay } from "@/lib/time";

/** Board part 3, frame 19: from the tablet step, a 230px name seat and the track, 16px apart. */
const GRID = "grid grid-cols-1 @tablet:grid-cols-[230px_minmax(0,1fr)] @tablet:gap-x-4";
/** The name seat: the line's name, its dates under it in the fine size, 9px above and below. */
const NAME = "flex flex-col py-[9px]";
/**
 * Each cell carries its own rule (the board's), so the divider breaks at the column gap; the first
 * line has none. On a phone the track sits under its name, inside the same rule.
 */
const rule = (first: boolean, track = false) => (first ? "" : track ? "@tablet:border-t @tablet:border-divider" : "border-t border-divider");

/**
 * Access over time (A-24 §9 vivid, Q11; owner 19 Nov 2026): one card, a line per role from the day it
 * was given to the day it ended, on this year's axis with the terms marked and today in violet. Ink is
 * held today, grey has ended, the dashed amber run is the time left before an end. The usage band's
 * grammar (A-23): the strong baseline colour, the accent for today; spans, as the term line draws, not
 * a chart. The dates are Syndra's records; held today is the standing, the page's own read of Zitadel.
 * The picture is one image named by the board's sentence; each line is also one sentence in a list a
 * screen reader reads. Operators, in Advanced (the caller).
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
        <span key={m.name + m.left} className="absolute inset-y-0 w-px bg-line-strong" style={{ left: `${m.left}%` }} />
      ))}
      <span data-today className="absolute inset-y-0 w-px bg-accent" style={{ left: `${now}%` }} />
    </>
  );

  let body;
  if (history.isPending) {
    // The lines held today are known before the read; the ones that ended this year are not, and
    // arrive with it. So the card holds at least as many lines as are held today, one at the least,
    // each in the same two lines of type as a real one.
    body = (
      <div aria-busy="true" aria-label="Loading" className="flex flex-col">
        {Array.from({ length: Math.max(1, lines) }).map((_, i) => (
          <div key={i} className={GRID}>
            <span className={`${NAME} ${rule(i === 0)}`}>
              <span className="text-body leading-[1.3]">
                <span className="skeleton-bar inline-block h-[10px] w-40 align-middle" />
              </span>
              <span className="text-fine">&nbsp;</span>
            </span>
            <span className={`flex min-h-6 items-center ${rule(i === 0, true)}`}>
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
      <>
        <ul className="sr-only">
          {roles.map((role) => (
            <li key={role.project_id + "/" + role.role_key}>{lineSentence(role, first, project(role), names.role(role.project_id, role.role_key))}</li>
          ))}
        </ul>
        <div role="img" aria-label={chartSummary(roles)} className="flex flex-col">
          {roles.map((role, i) => {
            const { runs, ahead } = placeRuns(role, today);
            return (
              <div key={role.project_id + "/" + role.role_key} data-line className={GRID}>
                <span className={`${NAME} ${rule(i === 0)}`}>
                  <span className="text-body font-semibold leading-[1.3] [overflow-wrap:anywhere]">
                    {lineName(names.role(role.project_id, role.role_key), project(role))}
                  </span>
                  <span data-figure data-when className="text-fine text-faint">
                    {whenLine(role, first)}
                  </span>
                </span>
                <span className={`relative min-h-6 ${rule(i === 0, true)}`}>
                  {hairlines}
                  {runs.map((r, j) => (
                    <span
                      key={j}
                      data-run={r.held ? "held" : "ended"}
                      data-clipped={r.clipped || undefined}
                      className={`absolute top-1/2 h-1.5 min-w-1.5 -translate-y-1/2 rounded-pill ${r.held ? "bg-ink" : "bg-line-strong"}`}
                      style={{ left: `${r.left}%`, width: `${r.width}%` }}
                    />
                  ))}
                  {ahead && (
                    <span data-ahead className="absolute top-1/2 -translate-y-1/2 border-t-2 border-dashed border-warn" style={{ left: `${ahead.left}%`, width: `${ahead.width}%` }} />
                  )}
                </span>
              </div>
            );
          })}
          {/* The axis, under the tracks: every term where it starts, and today. Late in December the
              word turns to the left of its line, so it never leaves the card. */}
          <div className={GRID}>
            <span />
            <span data-axis data-figure className="relative h-6 text-fine text-faint">
              {marks.map((m) => (
                <span key={m.name + m.left} className={`absolute top-1.5 whitespace-nowrap ${m.left > 0 ? "pl-1.5" : ""}`} style={{ left: `${m.left}%` }}>
                  {m.name}
                </span>
              ))}
              <span className={`absolute top-1.5 whitespace-nowrap text-accent-text ${now > 90 ? "-translate-x-full pr-1.5" : "pl-1.5"}`} style={{ left: `${now}%` }}>
                Today
              </span>
            </span>
          </div>
        </div>
      </>
    );
  }

  return (
    <Card role="group" aria-label={TITLE} className="@container">
      <CardHeader title={TITLE} note={NOTE} caption={history.data?.truncated ? `${CAPTION} ${CUT_SHORT}` : CAPTION} />
      {/* A failed read is the card's Notice, at the card's own inset (bare). */}
      {history.isError ? (
        <ErrorState bare title={READ_FAILED} error={history.error} onRetry={() => void history.refetch()} />
      ) : (
        <div className="px-(--card-x) pt-1 pb-4">{body}</div>
      )}
    </Card>
  );
}
