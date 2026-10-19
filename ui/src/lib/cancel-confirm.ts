/**
 * Never invisibly (owner, 15 Oct): an action that would cancel a change still
 * waiting for you to send is refused by the backend (409 WOULD_CANCEL_QUEUED) with
 * the list, until the operator has seen exactly those changes. `request()`
 * asks the one confirmation mounted at the root, then sends again naming the
 * ids it was shown. No screen has to be built to say so, and none can cancel
 * silently.
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
  reason?: string;
  queued_at: string;
}

type Confirmer = (changes: QueuedChange[]) => Promise<boolean>;

let confirmer: Confirmer | null = null;

/** Mounted once at the root. Returns the unregister. */
export function setCancelConfirmer(fn: Confirmer): () => void {
  confirmer = fn;
  return () => {
    if (confirmer === fn) confirmer = null;
  };
}

/** Asks the operator; with nobody to ask, the answer is no. */
export function confirmCancellations(changes: QueuedChange[]): Promise<boolean> {
  return confirmer ? confirmer(changes) : Promise.resolve(false);
}

export const CANCEL_ACK_HEADER = "X-Syndra-Cancel-Ack";
