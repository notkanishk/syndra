import { ApiError, request } from "@/lib/api-client";
import { moment } from "@/lib/format";

/**
 * One state for every add-on read (addon-contract design §13.6). A component
 * meets an add-on read only as a `Read`: a count, a list or a date exists only
 * under `data`, which `none` does not have, so printing a 0, an empty state or
 * a date from a read that did not happen does not compile.
 */

/** Why a read is not a reading now, as the backend said it (design §13.1, §13.3). */
export interface Why {
  /** `cached | cooldown | app_unreachable` for a kept copy; `addon_unreachable | app_unreachable | read_refused | addon_outdated | cooldown` for none. */
  reason: string;
  /** What the pipe saw, for `addon_unreachable` only: `no_answer | wrong_host | no_key | timeout | fault`. */
  cause?: string;
  /** The add-on's own sentence, said first where the add-on is the one that knows. */
  detail?: string;
  /** Whole seconds until Syndra learns more (Retry-After). */
  retryAfter?: number;
  /** When the answer carrying it arrived (ms since the epoch): a count runs from here. */
  since?: number;
  /** The backend's sentence, when the reply carried one (a 503's `message`). */
  message?: string;
}

export type Read<T> =
  | { state: "pending" }
  | { state: "now"; data: T; takenAt: string }
  | { state: "kept"; data: T; takenAt: string; why: Why }
  | { state: "none"; why: Why };

/** A read that answered: what a query holds. */
export type Answered<T> = Exclude<Read<T>, { state: "pending" }>;

/** The backend's `read` member (design §13.3). */
export interface WireRead {
  state: "now" | "kept" | "none";
  taken_at?: string;
  reason?: string;
  cause?: string;
  detail?: string;
  retry_after?: number;
}

/** The backend's one code for a read with nothing to give (503 + Retry-After). */
export const TARGET_NOT_READ = "TARGET_NOT_READ";

export const PENDING: Read<never> = { state: "pending" };

/** Reasons, as the backend writes them. Named here so no reader spells one twice. */
export const REASON = {
  cached: "cached",
  cooldown: "cooldown",
  busy: "busy",
  addonUnreachable: "addon_unreachable",
  appUnreachable: "app_unreachable",
  readRefused: "read_refused",
  addonOutdated: "addon_outdated",
} as const;

function whyOf(w: { reason?: string; cause?: string; detail?: string; retry_after?: number }, since?: number, message?: string): Why {
  return {
    // A none with no reason is still a read that did not happen: the add-on gave nothing usable.
    reason: w.reason || REASON.addonUnreachable,
    ...(w.cause ? { cause: w.cause } : {}),
    ...(w.detail ? { detail: w.detail } : {}),
    ...(w.retry_after && w.retry_after > 0 ? { retryAfter: w.retry_after } : {}),
    ...(since ? { since } : {}),
    ...(message ? { message } : {}),
  };
}

/**
 * The one builder of a `Read` from a reply's `read` member. A reading with no
 * moment (absent, unparseable, Go's zero time) is no reading (design §13.3 "one
 * date"); so is a reply with no `read` at all.
 */
export function readOf<T>(wire: WireRead | undefined, data: T, since?: number): Answered<T> {
  if (!wire || wire.state === "none" || !wire.taken_at || !moment(wire.taken_at)) {
    return { state: "none", why: whyOf(wire ?? {}, since) };
  }
  if (wire.state === "now") return { state: "now", data, takenAt: wire.taken_at };
  return { state: "kept", data, takenAt: wire.taken_at, why: whyOf({ reason: REASON.cached, ...wire }, since) };
}

/** A reply whose `read` dates the reply itself (a reconcile pass): its `read` built once, by `readOf`. */
export type WithRead<B extends { read: WireRead }> = Omit<B, "read"> & { read: Answered<undefined> };
export const withRead = <B extends { read: WireRead }>(body: B): WithRead<B> => ({ ...body, read: readOf(body.read, undefined, Date.now()) });

/** A 503 `TARGET_NOT_READ` as the read it is (`none`, with its why); undefined for any other failure. */
export function notReadOf(error: unknown, since?: number): { state: "none"; why: Why } | undefined {
  if (!(error instanceof ApiError) || error.code !== TARGET_NOT_READ) return undefined;
  return {
    state: "none",
    why: whyOf({ ...error.details, retry_after: error.retryAfter }, since, error.message),
  };
}

/**
 * The one fetch of an add-on read: the reply as a `Read` of its body, a 503
 * `TARGET_NOT_READ` as `none`. Any other failure (Syndra's own) still throws,
 * so the query's `error` is Syndra's and never the add-on's.
 */
export async function readFrom<T extends { read?: WireRead }>(path: string, init?: Parameters<typeof request>[1]): Promise<Answered<T>> {
  try {
    const body = await request<T>(path, init);
    return readOf(body.read, body, Date.now());
  } catch (e) {
    const none = notReadOf(e, Date.now());
    if (none) return none;
    throw e;
  }
}

/** A query's `Read`: pending until it has answered. */
export function readOfQuery<T>(q: { data?: Answered<T> }): Read<T> {
  return q.data ?? PENDING;
}

/** A read that did not happen: nothing to show, count or date. */
export const notRead = <T,>(read: Read<T> | undefined): read is { state: "none"; why: Why } => read?.state === "none";

/** `f` of the data when there is any: a count or a list exists only for a read that has data. */
export function ofRead<T, U>(read: Read<T>, f: (data: T) => U): U | undefined {
  return read.state === "now" || read.state === "kept" ? f(read.data) : undefined;
}

/** The data of a read that has some. */
export const dataOf = <T,>(read: Read<T>): T | undefined => ofRead(read, (d) => d);
