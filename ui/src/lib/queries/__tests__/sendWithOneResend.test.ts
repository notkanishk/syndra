import { describe, expect, it } from "vitest";

import { ApiError } from "@/lib/api-client";
import { lostReply, sendWithOneResend, type CredentialAnswer } from "@/lib/queries/useMyStorage";

// A-13 §4 step 1 (owner, 3 Nov): a lost reply is resent once, at once, with the
// same value, and only a confirmed `set` from either send is done.

function sender(...answers: Array<CredentialAnswer | Error>) {
  const calls: boolean[] = [];
  const send = async (resend: boolean) => {
    calls.push(resend);
    const next = answers[calls.length - 1];
    if (next === undefined) throw new Error("sent more than twice");
    if (next instanceof Error) throw next;
    return next;
  };
  return { send, calls };
}

const LOST = new TypeError("Failed to fetch");
const PROXY_GAVE_UP = new ApiError(504, "upstream timed out");

describe("the one automatic resend", () => {
  it("a confirmed first send is done, and nothing is resent", async () => {
    const { send, calls } = sender({ status: "set" });
    expect(await sendWithOneResend(send)).toEqual({ status: "set" });
    expect(calls).toEqual([false]);
  });

  it("a lost reply is sent again once, marked as the resend, and a confirmed resend is done", async () => {
    for (const lost of [LOST, PROXY_GAVE_UP, { status: "indeterminate" } as CredentialAnswer]) {
      const { send, calls } = sender(lost, { status: "set" });
      expect((await sendWithOneResend(send)).status).toBe("set");
      expect(calls).toEqual([false, true]);
    }
  });

  it("never sends a third time: two lost replies are unconfirmed", async () => {
    const { send, calls } = sender(LOST, LOST);
    expect(await sendWithOneResend(send)).toEqual({ status: "indeterminate" });
    expect(calls).toHaveLength(2);
  });

  it("a resend that hears anything but a confirmed set is unconfirmed, because the first may have landed", async () => {
    for (const second of [
      { status: "rejected" } as CredentialAnswer,
      { status: "indeterminate" } as CredentialAnswer,
      new ApiError(503, { error: "TARGET_UNREACHABLE", message: "not answering" }),
    ]) {
      const { send } = sender(LOST, second);
      expect((await sendWithOneResend(send)).status).toBe("indeterminate");
    }
  });

  it("a clear answer to the first send is final and is not resent", async () => {
    for (const first of [{ status: "rejected" } as CredentialAnswer, { status: "unreached" } as CredentialAnswer]) {
      const { send, calls } = sender(first);
      expect((await sendWithOneResend(send)).status).toBe(first.status);
      expect(calls).toEqual([false]);
    }
    const refusal = new ApiError(429, { error: "TOO_MANY_ATTEMPTS", message: "Wait" });
    const { send, calls } = sender(refusal);
    await expect(sendWithOneResend(send)).rejects.toBe(refusal);
    expect(calls).toEqual([false]);
  });

  it("tells a lost reply from one Syndra gave", () => {
    expect(lostReply(LOST)).toBe(true);
    expect(lostReply(PROXY_GAVE_UP)).toBe(true);
    expect(lostReply(new ApiError(502, { error: "CREDENTIAL_NOT_SET", message: "Nothing was changed." }))).toBe(false);
    expect(lostReply(new ApiError(0, { error: "OFFLINE", message: "not sent" }))).toBe(false);
  });
});
