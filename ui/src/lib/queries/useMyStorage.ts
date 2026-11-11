"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { ApiError, request } from "@/lib/api-client";
import type { EntitlementField } from "@/lib/fields";
import type { AccountHome, Standing } from "@/lib/queries/useTargets";
import type { OneShotSecret } from "@/lib/secret";
import { sortByName } from "@/lib/sort";
import { invalidateAfter } from "./invalidation";
import { useReadAgain, type Stale } from "./fresh";
import type { RetryState } from "@/lib/retry";

/**
 * A member's own storage view.
 *
 * Three states, and the page renders them as three. "Entitled with no account
 * yet" is the one a two-state design collapses, and it is the ordinary
 * experience of every new member until an operator resumes the drain.
 */

/** A role, as a project and key. */
export interface RoleSource {
  project_id: string;
  role_key: string;
}

/**
 * Somewhere the member can connect, read by the target from its own folder
 * permissions for the groups their account is actually in.
 */
export interface StoragePlace {
  share: string;
  /** The path inside the share; absent for the share's root. */
  folder?: string;
  path: string;
  access: "read" | "change";
  home?: boolean;
  /** Only where they can change files and it could be read. 0 is nothing saved yet. */
  usage?: { used_bytes: number; quota_bytes?: number };
  smb_address?: string;
  unc_address?: string;
  /** Their own roles whose groups open it. */
  roles?: RoleSource[];
}

/**
 * One read answered through the add-on contract's `results` feature: what the
 * add-on said about this person, in one of three shapes the core knows or
 * raw. Syndra shows it; it never decides on it, and `also_says` is whatever
 * else the add-on added, already quoted.
 */
export interface Observation {
  taken_at: string;
  current: boolean;
  /** Why it is a copy, when it is one (feature freshness). */
  stale?: Stale;
}

export interface RecordsResult extends Observation {
  records: Array<{
    id: string;
    label: string;
    category?: string;
    since?: string;
    due?: string;
    standing?: Standing;
  }>;
}

export interface StatusResult extends Observation {
  usable: boolean;
  standing: Standing;
  meters: Array<{ label: string; used: number; limit?: number; unit: string }>;
  /** False: the meters are not the whole of it, some usage could not be read. */
  quota_read?: boolean;
}

export interface ActivityResult extends Observation {
  events: Array<{ at: string; event: string; success: boolean; resource?: string; address?: string; detail?: string }>;
  uncovered: string[];
  truncated: boolean;
}

export interface ReadView {
  operation: string;
  /** "" is an id's own shape, shown raw. */
  shape: "records" | "activity" | "status" | "";
  /** False: unreachable, refused, or a reply missing a required key. */
  readable: boolean;
  /** Absent when not readable; JSON text (capped) when `shape` is "". */
  result?: RecordsResult | StatusResult | ActivityResult | string;
  also_says?: string[];
  /** Whole seconds: the add-on refused this read inside its cooldown and kept nothing to serve. */
  retry_after?: number;
}

export interface MyTargetView {
  target: string;
  /** What the system is called, the same name the operator's roster carries. */
  name: string;
  /** At least one mapped role reaches this target for this person. */
  entitled: boolean;
  /** The add-on-reported name they connect with, or absent. */
  account?: { username: string; bound_at?: string };
  /** What their current entitlements reach — only what resolves. */
  resources?: Record<string, string[]>;
  /** What an operator has withheld, with the reason, so "no access" is never unexplained. */
  suspended?: Array<{ field: string; value: string; reason: string; actor_id: string; since?: string }>;
  /** Their own roles that reach this system. */
  roles?: RoleSource[];
  credential: {
    set: boolean;
    /**
     * Enrolled before the LLDAP bridge was retired. Their old credential is
     * gone with the system it was for, so they have to set a new one — and
     * "you enrolled before the change" is a different sentence from "you have
     * never set one" to somebody who remembers doing it.
     */
    needs_re_enrolment?: boolean;
    last_changed_at?: string;
  };
  /** What kind of system this is, and what its fields are: the accepted
   * manifest's. Both absent until one is accepted ("not read yet"). */
  kind?: string;
  entitlement_schema?: EntitlementField[];
  /** Every member-scope read the add-on answered under `results`, in the add-on's order. */
  reads?: ReadView[];
  /** Whether the account can be used yet, and who is waited on, in the add-on's words. */
  standing?: Standing;
  /** The add-on answered. A member whose target is down is told, not shown a form that fails. */
  reachable: boolean;
  /**
   * What the health monitor can stand behind: `offline` only after three
   * missed checks in a row, `unknown` when Syndra can't tell yet.
   */
  health?: "online" | "offline" | "unknown";
  checked_at?: string;
  /** When it stopped answering, set only while offline. */
  reachable_since?: string;
  /** When access was given back, while the account waits for a new password. */
  restored_at?: string;
  /**
   * Present only while their latest password change here is unconfirmed: sent,
   * no answer to it or to its one automatic resend, and no later confirmed
   * change has settled it (A-13 §4 step 3). Cleared by their next confirmed
   * change, and by nothing else.
   */
  password_change?: { state: "unconfirmed"; at: string };
  /** A home folder no role they hold gives any more. It stays. */
  home_kept?: boolean;
  /**
   * The state an operator put the target in: `active`, `draining` or
   * `read_only`.
   *
   * Not a boolean, and the two pauses are not interchangeable. A drain is
   * minutes and ends by itself; read-only is somebody working on the server and
   * ends when they say so. One earns the word "shortly" and the other must
   * never be given it — an estimate attached to an open-ended pause is the
   * small lie that makes the rest of a page untrustworthy.
   */
  lifecycle?: "active" | "draining" | "read_only";
  /**
   * When their access was written down, for the middle state.
   *
   * The wait is on a person resuming the drain, not on a timer, so the page
   * states an age rather than an estimate. "This usually clears within a day"
   * is a guess about somebody's week; "recorded two days ago" is true.
   */
  recorded_at?: string;
  /**
   * How to reach it, from the add-on's own registration.
   *
   * Absent when the deployment has not named a share host, and the page then
   * omits the instructions rather than printing one that does not answer.
   */
  connection?: { protocol: string; host: string };
  /**
   * What the TARGET says about this account right now.
   *
   * Not the same as `credential` above, which is Syndra's record that a
   * password was set. That record cannot say whether the target still accepts
   * it — and an account Syndra created has password authentication disabled
   * until its member sets one, so it exists, is correct, and refuses them.
   */
  storage?: {
    /** Whether they can connect right now. */
    usable: boolean;
    /** The one action that fixes it, when that is what is wrong. */
    needs_password: boolean;
    /** "Nothing used" and "could not look" are the same zero without this.
     *  False when any share's usage was unread (a partial read too). */
    quota_read?: boolean;
    /** Access given back: SMB waits for a new password. */
    needs_new_password?: boolean;
    places?: StoragePlace[];
    /** False is "couldn't read", never "none". */
    places_readable?: boolean;
  };
}

/**
 * A system the page is waiting on: offline, or paused by an operator. While one
 * shows, the page reads again every 30s, so "this page updates on its own"
 * stays true.
 */
export function waitingOn(view: MyTargetView): boolean {
  return view.health === "offline" || view.lifecycle === "draining" || view.lifecycle === "read_only";
}

/** A hold in force on one person on one system, as the person page reads it. */
export interface PersonHold {
  field: string;
  value: string;
  reason: string;
  actor_id: string;
  since: string;
  review_date?: string;
  expires_at?: string;
}

/**
 * A person's storage on one system, for operators (A-10 Part 1): the member's
 * own view of it, plus what only an operator reads. Read-only.
 */
export interface PersonTargetView extends MyTargetView {
  /** Made by hand on the system and assigned by an operator, not made by Syndra. */
  adopted?: { actor_id: string; at: string };
  /** Their home folder limit and where it comes from, with use as last read. */
  home?: AccountHome;
  holds: PersonHold[];
  /** The last successful read, while the system is offline. */
  last_known?: { storage: NonNullable<MyTargetView["storage"]>; read_at: string };
  /** Why nobody can have an account there yet. */
  setup?: "no_project" | "no_mapping";
  /** What an account still to be made will open. */
  preview?: StoragePlace[];
}

const personTargetsPath = (userId: string) => `/users/${encodeURIComponent(userId)}/targets`;
const personTargetsKey = (userId: string) => ["users", userId, "targets"];

/** "Read again" on a person's systems: the same GET, asking the systems themselves. */
export const usePersonTargetsAgain = (userId: string) =>
  useReadAgain<{ targets: PersonTargetView[] }>(personTargetsKey(userId), personTargetsPath(userId));

/** A person's storage on every connected system (operators). */
export function usePersonTargets(userId: string, enabled = true) {
  return useQuery({
    queryKey: personTargetsKey(userId),
    enabled,
    queryFn: () => request<{ targets: PersonTargetView[] }>(personTargetsPath(userId)),
    select: (data) => sortByName(data.targets ?? [], (t) => t.name, (t) => t.target),
  });
}

/** "Read again" on a member's own systems: the same GET, asking the systems themselves. */
export const useMyStorageAgain = () => useReadAgain<{ targets: MyTargetView[] }>(["me", "targets"], "/me/targets");

export function useMyStorage(enabled = true) {
  return useQuery({
    queryKey: ["me", "targets"],
    enabled,
    // Never `fresh`: a load and every poll below are answered from the add-on's copy.
    queryFn: () => request<{ targets: MyTargetView[] }>("/me/targets"),
    select: (data) => sortByName(data.targets ?? [], (t) => t.name, (t) => t.target),
    refetchInterval: (query) =>
      (query.state.data?.targets ?? []).some(waitingOn) ? 30_000 : false,
  });
}

/**
 * Setting the storage credential.
 *
 * The value goes to the target and is kept nowhere: not in the query cache, not
 * in the response, not in Syndra's database. What comes back is metadata.
 *
 * The one-shot box is what makes the first of those true. A mutation's
 * variables outlive the request in the MutationCache, so taking the password as
 * a plain argument retained it — under this exact sentence.
 */
/** What the backend said, or what the page concluded when nothing it said arrived. */
export interface CredentialAnswer {
  /** `set` is the only confirmed done; the other three are not success. */
  status: "set" | "indeterminate" | "rejected" | "unreached";
  detail?: string;
  /**
   * On a lost reply, where the change stands under the one retry model
   * (extcall.AddonResend): retrying, with when the page sends it again and
   * "try 2 of 2"; after that resend, waiting (nothing tries again until the
   * member does).
   */
  retry?: RetryState;
}

/**
 * A reply that never arrived: the network failed, or a proxy in front gave up
 * before the backend answered (a 502–504 with none of Syndra's own codes).
 * Anything Syndra itself answered is a clear answer, never this.
 */
export function lostReply(error: unknown): boolean {
  if (!(error instanceof ApiError)) return true;
  return error.status >= 502 && error.status <= 504 && error.code === "UNKNOWN_ERROR";
}

/**
 * When the server said nothing at all (the reply itself was lost), the page
 * still sends once more, now. The numbers are the server's rule
 * (extcall.AddonResend: two tries), which its parity test holds at two.
 */
const RESEND_NOW: RetryState = { state: "retrying", try: 2, of: 2 };

const sleep = (ms: number) => new Promise<void>((resolve) => setTimeout(resolve, ms));

/**
 * One send, and on a lost reply one more with the same value (A-13 §4 step 1,
 * owner 3 Nov), at the next try the server named (the one retry model, owner
 * 5 Nov). `onRetry` hears it first, so the page can say "Not confirmed yet.
 * Syndra is trying again at 14:05" while it waits. Only a confirmed `set` from
 * either send is done; if neither is answered with one, the change is
 * unconfirmed, whatever the second send heard, because the first may have
 * landed. A clear refusal of the first send is final and is not resent.
 */
export async function sendWithOneResend(
  send: (resend: boolean) => Promise<CredentialAnswer>,
  onRetry: (retry: RetryState) => void = () => {},
  wait: (ms: number) => Promise<void> = sleep,
): Promise<CredentialAnswer> {
  let retry = RESEND_NOW;
  try {
    const first = await send(false);
    if (first.status !== "indeterminate") return first;
    if (first.retry) retry = first.retry;
  } catch (error) {
    if (!lostReply(error)) throw error;
  }
  onRetry(retry);
  if (retry.next_try) await wait(Math.max(0, new Date(retry.next_try).getTime() - Date.now()));
  let after: RetryState = { state: "waiting", try: retry.of, of: retry.of };
  try {
    const second = await send(true);
    if (second.status === "set") return second;
    if (second.retry) after = second.retry;
  } catch {
    // Unanswered again, or refused after a first send that may have landed:
    // either way nobody knows, and the page says so.
  }
  return { status: "indeterminate", retry: after };
}

export function useSetStorageCredential(target: string, onRetry?: (retry: RetryState) => void) {
  const client = useQueryClient();
  return useMutation({
    // The value is taken out of its box once and held only by this call, in
    // memory, until a final answer: never stored, never in the query cache.
    mutationFn: (password: OneShotSecret) => {
      const value = password.take();
      return sendWithOneResend(
        (resend) =>
          request<CredentialAnswer>(`/me/targets/${target}/credential`, {
            method: "POST",
            body: resend ? { password: value, resend: true } : { password: value },
          }),
        onRetry,
      );
    },
    onSettled: () => invalidateAfter(client, "storageCredential"),
  });
}
