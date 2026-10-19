/**
 * Never invisibly (owner, 15 Oct): an action that would cancel a change still
 * waiting for you to send is refused by the backend (409 WOULD_CANCEL_QUEUED)
 * with the list, until the operator has seen exactly those changes.
 *
 * `request()` throws that refusal as a `WouldCancelQueued` to whoever made the
 * write, and the dialog that made it pushes the step into its own sheet
 * (`CancelQueuedStep`, A-04 §14). Going ahead sends the same request again
 * with `armAck` naming the ids it showed; no screen can cancel silently,
 * because the backend refuses until it has been told.
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
  reason?: string;
  queued_at: string;
  /** On a retry: queued since the ids that were acknowledged. */
  new?: boolean;
}

export const CANCEL_ACK_HEADER = "X-Syndra-Cancel-Ack";

/**
 * The ids to send with the next request to exactly this method and path.
 * Keyed by the request, not ambient: a different write made meanwhile never
 * carries an acknowledgement it wasn't given. Consumed by one request.
 */
const armed = new Map<string, string[]>();

const keyOf = (method: string, path: string) => `${method.toUpperCase()} ${path}`;

/** The next `method path` request names these changes as seen and agreed to. */
export function armAck(method: string, path: string, ids: string[]): void {
  armed.set(keyOf(method, path), ids);
}

/** What `request()` takes for the request it is about to send. */
export function takeAck(method: string, path: string): string[] | undefined {
  const key = keyOf(method, path);
  const ids = armed.get(key);
  armed.delete(key);
  return ids;
}
