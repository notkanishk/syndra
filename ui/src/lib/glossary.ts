/**
 * Every term the product defines, defined once.
 *
 * Syndra is run by makerspace staff at a university who know what an identity
 * provider is and can name the products in their own rack. Copy that expands
 * "Zitadel" into "the service everyone signs in through" every time it appears
 * spends a clause telling them something they already know, and reads as
 * though it doubts them. The precise word is the shorter word, and it is the
 * one they would use themselves.
 *
 * So the vocabulary comes back, and the explanation moves off the line and
 * behind the word: `<Term name="cascade">cascade</Term>` sets the term in the
 * sentence and puts its definition one hover, tap or Tab away.
 *
 * Two kinds of term live here and the difference matters. **Standard** terms
 * (grant, provision, reconcile, claim, OIDC) are the field's own vocabulary —
 * a definition is a courtesy to a new staff member and a reminder to everyone
 * else. **Syndra's own** terms (cascade, outbox, merge finding, log anchor)
 * cannot be known from experience however senior the reader is, because this
 * product invented them. Those definitions are not a courtesy; they are the
 * only place the word is ever explained, and they carry the consequence, not
 * just the meaning.
 *
 * One definition per term, in one file, because the day a word is explained
 * two ways is the day it means two things. `plain-language.test.ts` checks
 * that a term's first appearance on a page is marked up, and that nothing
 * here has drifted out of use.
 */

export type TermKind = "product" | "standard" | "syndra";

export interface GlossaryEntry {
  /** What the reader sees defined. Sentence case, no trailing full stop in the term itself. */
  title: string;
  /** One or two sentences. Consequence before mechanism, as everywhere else. */
  definition: string;
  kind: TermKind;
  /**
   * True when the word is this product's own coinage and cannot be worked out
   * from ordinary English — `cascade`, `outbox`, `log anchor`.
   *
   * False for terms that ARE ordinary English used precisely: a person *holds*
   * a role and a *hold* blocks one, and no word-boundary match can tell those
   * apart. Marking them up is encouraged and unenforceable, so the guard
   * requires it only where the word is unmistakable.
   */
  mustDefine?: boolean;
}

export const GLOSSARY = {
  // ---- The products in the rack -------------------------------------------
  zitadel: {
    title: "Zitadel",
    definition:
      "The identity provider. It authenticates people and holds the roles an app reads at sign-in. Syndra decides what anybody should hold and writes it there; Zitadel never decides on its own.",
    kind: "product",
  },
  truenas: {
    title: "TrueNAS",
    definition:
      "The storage server. Syndra creates and maintains a Unix account and group membership there for anybody whose roles reach it, and never touches an account it did not create without being told to.",
    kind: "product",
  },
  google: {
    title: "Google Workspace",
    definition:
      "The university's directory, and the only place a person's identity actually originates. Zitadel federates to it, so a person exists in Syndra because they exist in Google.",
    kind: "product",
  },

  // ---- The field's own vocabulary -----------------------------------------
  grant: {
    title: "Grant",
    definition:
      "One person holding one role in one project. A direct grant was given by hand and stands on its own; a grant carried by a bundle or produced by a rule disappears when its source does.",
    kind: "standard",
  },
  entitlement: {
    title: "Entitlement",
    definition:
      "What a role entitles somebody to on a connected system — the TrueNAS groups it maps to, and therefore the shares and datasets it opens. Roles live in Zitadel; entitlements are what Syndra resolves them into.",
    kind: "standard",
  },
  provision: {
    title: "Provision",
    definition:
      "Create the account and everything that goes with it — home directory, primary group, shell — rather than merely adding a membership to an account that already exists.",
    kind: "standard",
  },
  reconcile: {
    title: "Reconciliation",
    definition:
      "The pass that compares what Syndra believes a system holds against what it actually holds, and classifies each difference rather than resolving it by overwriting — one side moved, both did, or the account is gone. A change somebody made by hand survives it, and is reported instead of being silently reverted.",
    kind: "standard",
  },
  drift: {
    title: "Drift",
    definition:
      "Access that exists on a system with no record in Syndra explaining how it got there. Every Syndra-mediated change leaves a trace before it is made, so anything without one was made another way and needs a decision.",
    kind: "standard",
  },
  claim: {
    title: "Claim",
    definition:
      "One field inside the token an app receives at sign-in. Syndra shapes the roles claim; an app that reads the wrong key sees no roles at all.",
    kind: "standard",
  },
  token: {
    title: "Token",
    definition:
      "What an app is handed when somebody signs in, carrying their identity and their roles. It is issued fresh at each sign-in, so a change reaches an app the next time its user signs in — not immediately.",
    kind: "standard",
  },
  oidc: {
    title: "OIDC",
    definition:
      "OpenID Connect — the protocol most apps here use to sign people in and receive their roles in a token.",
    kind: "standard",
  },
  saml: {
    title: "SAML",
    definition:
      "An older sign-in protocol some vendor applications still require. It carries the same roles as OIDC in a different envelope.",
    kind: "standard",
  },
  roleKey: {
    title: "Role key",
    definition:
      "A role's stable machine name — what an app matches on, and what appears in a token. The display name can change; the key cannot, because apps are configured against it.",
    kind: "standard",
  },
  serviceAccount: {
    title: "Service account",
    definition:
      "A machine's account rather than a person's. It holds roles like anybody else, but nobody signs in as it, so it never appears in a review queue meant for people.",
    kind: "standard",
  },

  // ---- Syndra's own, which nobody can know without being told -------------
  bundle: {
    title: "Bundle",
    definition:
      "A named set of roles handed out as one unit, and versioned. Somebody holds a specific version, so publishing a new one does not move anybody until you say so.",
    kind: "syndra",
  },
  automaticRule: {
    title: "Automatic rule",
    definition:
      "Holding one role produces another, with nobody clicking. A rule applies to everybody who holds its input today and everybody who is given it later.",
    kind: "syndra",
  },
  cascade: {
    title: "Cascade",
    definition:
      "Every change one edit sets off. Editing a bundle or a rule touches everybody it reaches, and those changes are grouped under one handle (c_…) so they can be followed, and confirmed or refused, together.",
    kind: "syndra",
    mustDefine: true,
  },
  outbox: {
    title: "Outbox",
    definition:
      "Where a decided change waits before it reaches Zitadel or a connected system. Syndra records the intent first and dispatches second, so an interrupted run loses nothing and never leaves a change nobody can account for.",
    kind: "syndra",
    mustDefine: true,
  },
  drain: {
    title: "Drain",
    definition:
      "One pass over the outbox, dispatching what is waiting in order. Operator-triggered, except revocations, which drain on a timer because a delayed revocation is retained access.",
    kind: "syndra",
    mustDefine: true,
  },
  hold: {
    title: "Hold",
    definition:
      "A deliberate block on access a role still grants, with a date to look at it again. It stays in force until somebody lifts it — the review date is a reminder, not an expiry.",
    kind: "syndra",
  },
  mapping: {
    title: "Mapping",
    definition:
      "Which group on a connected system a role turns into. On TrueNAS the group is what decides which shares and datasets an account can open.",
    kind: "syndra",
  },
  mergeFinding: {
    title: "Merge finding",
    definition:
      "A difference reconciliation could not resolve on its own — both sides changed since Syndra last looked, or the account is gone from the system entirely. It is recorded rather than guessed at, and waits for a person.",
    kind: "syndra",
    mustDefine: true,
  },
  adopt: {
    title: "Adopt",
    definition:
      "Take an account Syndra did not create and record it as belonging to a person from now on. Everything the account already holds becomes theirs; nothing about it is changed.",
    kind: "syndra",
  },
  baseline: {
    title: "Baseline",
    definition:
      "The point in a connected system's change log that Syndra trusts as the last verified state. Accepting a new one discards the ability to detect tampering before it, which is why it is never done to clear a warning.",
    kind: "syndra",
    mustDefine: true,
  },
  logAnchor: {
    title: "Log anchor",
    definition:
      "A hash chaining a system's change log to the last state Syndra verified. If it does not match, the log was altered or truncated after Syndra read it — the one finding here that is not routine.",
    kind: "syndra",
    mustDefine: true,
  },
  unvouched: {
    title: "Unvouched",
    definition:
      "A system Syndra could not read on its last attempt, so it can say nothing about what that system currently holds. Not a fault in the access — a gap in the evidence.",
    kind: "syndra",
    mustDefine: true,
  },
  intentLedger: {
    title: "Intent ledger",
    definition:
      "The record written before every direct grant Syndra makes in Zitadel. It is what lets a later sweep tell a change Syndra made from one somebody made by hand.",
    kind: "syndra",
    mustDefine: true,
  },
} as const satisfies Record<string, GlossaryEntry>;

export type TermName = keyof typeof GLOSSARY;

export function term(name: TermName): GlossaryEntry {
  return GLOSSARY[name];
}
