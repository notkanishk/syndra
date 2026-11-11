import { ApiError } from "@/lib/api-client";
import { outcomeFromError, type ActionOutcome } from "@/lib/outcome";
import type { OperationState } from "@/lib/queries/useMemberAction";
import * as copy from "./copy";

/** A word and the outcome it heads: what `ActionOutcome` draws. */
export interface Said {
  word: string;
  outcome: ActionOutcome;
}

/** A refusal or failure of a preview or confirm, in the member's words. A code with none falls to the shared wording. */
export function saidFromError(error: unknown): Said {
  const said = error instanceof ApiError ? copy.refusal(error.code, error.details) : undefined;
  if (!(error instanceof ApiError) || !said) return { word: copy.WORD.notDone, outcome: outcomeFromError(error) };
  return {
    word: copy.WORD.notDone,
    outcome: {
      kind: error.status >= 500 ? "failed" : "refused",
      message: said.message,
      next: said.next,
      requestId: error.status >= 500 ? error.details?.request_id : undefined,
    },
  };
}

/** Where a confirm stands, as an outcome. `late`: the last ask for it did not get through. */
export function saidFromState(state: OperationState["state"], late = false): Said {
  switch (state) {
    case "done":
      return { word: copy.WORD.done, outcome: { kind: "applied", message: copy.DONE } };
    case "no_change":
      return { word: copy.WORD.noChange, outcome: { kind: "no_change", message: copy.NO_CHANGE } };
    case "not_done":
      return { word: copy.WORD.notDone, outcome: { kind: "refused", message: copy.NOT_DONE, next: copy.NOT_DONE_NEXT } };
    case "ask_desk":
      // Amber, not red: nobody failed, and a step is left. `next` keeps it from reading as "Press Send".
      return { word: copy.WORD.askDesk, outcome: { kind: "queued", message: copy.ASK_DESK, detail: copy.ASK_DESK_DETAIL, next: copy.ASK_DESK_NEXT } };
    case "checking":
      return {
        word: copy.WORD.checking,
        outcome: { kind: "queued", message: copy.CHECKING, detail: late ? copy.CHECKING_LATE : copy.CHECKING_DETAIL, next: copy.CHECKING_NEXT },
      };
  }
}
