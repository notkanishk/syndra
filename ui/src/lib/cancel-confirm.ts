/**
 * Never invisibly (owner, 15 Oct): an action that would cancel a change still
 * waiting for you to send is refused by the backend (409 WOULD_CANCEL_QUEUED)
 * with the list, until the operator has seen exactly those changes.
 *
 * `request()` throws that refusal as a `WouldCancelQueued` to whoever made the
 * write, and the dialog that made it pushes the step into its own sheet
 * (`CancelQueuedStep`, A-04 §14). Going ahead sends the same write again
 * naming the ids it showed in its own `cancel_ids` (a planned act carries
 * its plan's; any other write, a `?cancel_ids=` query); no screen can cancel
 * silently, because the backend refuses until the write names them.
 */

/** A change Syndra queued and hasn't sent, which an action would cancel. */
export interface QueuedChange {
  id: string;
  op_type: string;
  user_id: string;
  project_id: string;
  role_keys: string[];
  source: string;
  initiated_by: string;
  /** Who started it, when the directory knows them. */
  initiated_by_name?: string;
  /** The reason as the queuer said it; never one of Syndra's own labels (backend `db.Said`). */
  said?: string;
  queued_at: string;
  /** On a retry: queued since the ids that were acknowledged. */
  new?: boolean;
}

/**
 * A write not on the planner (a mapping rule's create, update and retire,
 * retiring a bundle) names the queued changes it was shown in its query:
 * `?cancel_ids=a,b`. No ids, the path as it is: the backend admits no
 * cancellation and answers 409 WOULD_CANCEL_QUEUED if it would make one.
 */
export function withCancelIds(path: string, cancelIds?: string[]): string {
  return cancelIds?.length ? `${path}?${new URLSearchParams({ cancel_ids: cancelIds.join(",") })}` : path;
}
