import { NO_LIMIT, noMoreLines, ofLimit } from "@/components/results/copy";
import { NOTHING_TO_DO } from "@/lib/outcome";
import type { StatsUnit } from "@/lib/queries/useStats";

/**
 * Every sentence the statistics surfaces say (the member card, the person
 * page, the target overview, Insights), in one place so the copy guide
 * (`plain-language-copy/design.md` section 15) and a test hold them to the same
 * words. Syndra's own sentences only: what the add-on wrote (a metric's label,
 * a dimension, a gap's reason) is quoted and capped in the renderer, never here.
 *
 * Every card ends on a next step or "Nothing for you to do.". Nothing ranks
 * people or systems (a guard reads this file for the words that would).
 */

export { NO_LIMIT, noMoreLines, ofLimit, NOTHING_TO_DO };

/** The card. */
export const MEMBER_TITLE = "What I have used";
export const operatorTitle = (person: string) => `What ${person} has used`;

/** A counter, summed over a window of days. */
export const inDays = (value: string) => `${value} in the last 30 days`;
export const inMonths = (value: string) => `${value} in the last 12 months`;
export const NOTHING_COUNTED = "Nothing counted in the last 30 days";

/** A gauge: its latest value and the day it was taken. */
export const latestOn = (value: string, day: string) => `${value} on ${day}`;
export const NO_RECENT_VALUE = "No value in the last 30 days";
export const LIMIT_UNREAD = "Its limit could not be read, so there is no meter.";

/** Days nobody could observe, said, never drawn as zero. */
export const GAPS_HEADING = "Couldn't count these days";
export const gapLine = (range: string, where: string, reason: string) => `${range}${where ? `, ${where}` : ""}: ${reason}`;
export const GAPS_NOTE = "A day left blank was not counted. It is not a zero.";

/** States. */
export const NOT_COUNTED = "Not counted yet";
export const NOT_COUNTED_NEXT = `${NOTHING_TO_DO} The numbers appear here once Syndra has counted.`;
export const ABSENT = `This add-on does not report statistics. ${NOTHING_TO_DO}`;
export const UNAVAILABLE = "Syndra couldn't read these numbers just now. Reload this page to try again.";

/** The target overview (operators). */
export const OVERVIEW_TITLE = "What has been used here";
export const OVERVIEW_LEDE = "Totals for everyone who used it, over the last 30 days and the last 12 months. One person's numbers are on their page.";
export const lastCounted = (age: string) => `Last counted ${age}`;
export const unbound = (n: number) =>
  `${n} ${n === 1 ? "account" : "accounts"} Syndra does not manage also used it. ${n === 1 ? "It is" : "They are"} in the totals.`;
export const retention = (days: number, set: boolean) =>
  `Per-person numbers are kept for ${days} days${set ? ", as an operator set" : ""}. Totals are kept for 5 years.`;

/** What the last count did not keep, by the reason the backend gives. `{label}` is the metric's own (quoted by the caller). */
export const DROPPED: Record<string, (n: number, label: string) => string> = {
  not_a_day: (n) => `${n} ${n === 1 ? "value" : "values"} that were not for a whole day`,
  bad_value: (n) => `${n} ${n === 1 ? "value" : "values"} that were not numbers`,
  not_per_subject: (n) => `${n} per-person ${n === 1 ? "value" : "values"} for a number that is not per person`,
  unbound_subject: (n) => `${n} ${n === 1 ? "value" : "values"} for accounts Syndra does not manage`,
  undeclared_dimension: (n) => `${n} ${n === 1 ? "value" : "values"} with a breakdown the add-on did not declare`,
  person_in_dimension: (n) => `${n} ${n === 1 ? "value" : "values"} broken down by a person`,
  past_retention: (n) => `${n} ${n === 1 ? "value" : "values"} older than Syndra keeps`,
  series: (_, label) => `some of the series, because the add-on reports more ${label} series than Syndra keeps`,
};
export const droppedLine = (parts: string[]) => `The last count did not keep ${parts.join(", ")}.`;

/** Insights (operators, Advanced view). */
export const INSIGHTS_TAB = "Usage";
export const INSIGHTS_LEDE = "How much of each thing is used across the makerspace, as spreads over the people who used it. No names are shown.";
export const windowFrom = (day: string) => `The last 30 days, from ${day}`;
export const members = (n: number) => `${n} ${n === 1 ? "person" : "people"}`;
export const spread = (p50: string, p90: string) => `Half are at ${p50} or under; nine in ten are at ${p90} or under.`;
export const INSIGHTS_EMPTY = `No system reports per-person statistics yet. ${NOTHING_TO_DO}`;
export const INSIGHTS_NEXT = `${NOTHING_TO_DO} One person's numbers are on their page in People.`;
export const INSIGHTS_NEXT_LINK = "Open People";

/** A unit's group heading, in the registry's own words. */
export const UNIT_HEADING: Record<StatsUnit, string> = {
  count: "Counts",
  items: "Items",
  bytes: "Sizes",
  seconds: "Durations",
  grams: "Weights",
};

/** A sparkline's hover text and its description for a screen reader. */
export const dayValue = (day: string, value: string) => `${day}: ${value}`;
export const dayGap = (day: string) => `${day}: couldn't count`;
export const dayNoValue = (day: string) => `${day}: no new value`;
export const sparkLabel = (what: string, gaps: boolean) => `${what}, day by day over the last 30 days${gaps ? ". Days that could not be counted are hatched" : ""}`;
export const INSIGHTS_LOADING = "Loading the usage";
export const INSIGHTS_ERROR = "Couldn't read the usage.";
export const INSIGHTS_SUBJECT = "The usage";
