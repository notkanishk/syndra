"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { ApiError, request } from "@/lib/api-client";
import type { AccountHome } from "@/lib/queries/useTargets";
import type { OneShotSecret } from "@/lib/secret";
import { targetLabel } from "@/lib/nav";
import { sortByName } from "@/lib/sort";
import { invalidateAfter } from "./invalidation";

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

export interface MyTargetView {
  target: string;
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
    username: string;
    /** Whether they can connect right now. */
    usable: boolean;
    /** The one action that fixes it, when that is what is wrong. */
    needs_password: boolean;
    smb_enabled: boolean;
    shares?: Array<{ share: string; used_bytes: number; quota_bytes?: number }>;
    /** "Nothing used" and "could not look" are the same zero without this. */
    usage_readable?: boolean;
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

/** A person's storage on every connected system (operators). */
export function usePersonTargets(userId: string, enabled = true) {
  return useQuery({
    queryKey: ["users", userId, "targets"],
    enabled,
    queryFn: () => request<{ targets: PersonTargetView[] }>(`/users/${encodeURIComponent(userId)}/targets`),
    select: (data) => sortByName(data.targets ?? [], (t) => targetLabel(t.target), (t) => t.target),
  });
}

export function useMyStorage() {
  return useQuery({
    queryKey: ["me", "targets"],
    queryFn: () => request<{ targets: MyTargetView[] }>("/me/targets"),
    select: (data) => sortByName(data.targets ?? [], (t) => targetLabel(t.target), (t) => t.target),
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
 * One send, and on a lost reply one more, at once, with the same value (A-13
 * §4 step 1, owner 3 Nov). Only a confirmed `set` from either send is done; if
 * neither is answered with one, the change is unconfirmed, whatever the second
 * send heard, because the first may have landed. A clear refusal of the first
 * send is final and is not resent.
 */
export async function sendWithOneResend(send: (resend: boolean) => Promise<CredentialAnswer>): Promise<CredentialAnswer> {
  try {
    const first = await send(false);
    if (first.status !== "indeterminate") return first;
  } catch (error) {
    if (!lostReply(error)) throw error;
  }
  try {
    const second = await send(true);
    if (second.status === "set") return second;
  } catch {
    // Unanswered again, or refused after a first send that may have landed:
    // either way nobody knows, and the page says so.
  }
  return { status: "indeterminate" };
}

export function useSetStorageCredential(target: string) {
  const client = useQueryClient();
  return useMutation({
    // The value is taken out of its box once and held only by this call, in
    // memory, until a final answer: never stored, never in the query cache.
    mutationFn: (password: OneShotSecret) => {
      const value = password.take();
      return sendWithOneResend((resend) =>
        request<CredentialAnswer>(`/me/targets/${target}/credential`, {
          method: "POST",
          body: resend ? { password: value, resend: true } : { password: value },
        }),
      );
    },
    onSettled: () => invalidateAfter(client, "storageCredential"),
  });
}
