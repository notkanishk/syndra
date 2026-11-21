import { healthReplyOf, type TargetHealthReply, type TargetHealthWire } from "@/lib/queries/useTargets";
import { readNow } from "@/test-utils/wire";

/** The health route's body as its hook answers it: the monitor beside the add-on's `Read` (a read now unless it says). */
export function asHealth(body: TargetHealthWire): TargetHealthReply {
  return healthReplyOf({ read: readNow(), ...body });
}
