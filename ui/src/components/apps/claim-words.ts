import type { StatusTone } from "@/components/ui/Badge";
import type { TrailCheck } from "@/components/ui/CheckTrail";
import { clockOf, outcomePhrase, READS_LABEL } from "@/lib/claims";
import { formatAt } from "@/lib/format";
import type { CheckState, ClaimTemplate, Explanation, Lands, Profile, ProjectRole } from "@/lib/queries/useClaimShape";

/**
 * The six checks' words (A-11, A-14 §1, §5), read off the facts the
 * backend's explain step returns. The verdict is the backend's; these only
 * say it.
 */

export const STATE_TONE: Record<CheckState, StatusTone> = {
  pass: "healthy",
  none: "neutral",
  warn: "warn",
  fail: "danger",
  stale: "healthy",
};

export interface CheckContext {
  app: string;
  project: string;
  person: string;
  roles: ProjectRole[];
  /** The shape the checks ran (a draft or what applies now). */
  profile: Profile;
  templates: ClaimTemplate[];
  /** For a draft: where it came from, and rows it had to leave out. */
  draft?: { origin: "template" | "copy" | "edit"; name?: string; leftOut?: string[]; summary?: string };
  /** Zitadel console links, when Syndra knows Zitadel's address. */
  links?: { app?: string; project?: string };
}

const list = (xs: string[]) =>
  xs.length <= 1 ? (xs[0] ?? "") : `${xs.slice(0, -1).join(", ")} and ${xs[xs.length - 1]}`;

function fact<T>(e: Explanation, i: number, k: string): T | undefined {
  return e.checks[i]?.facts?.[k] as T | undefined;
}

function check1(e: Explanation, ctx: CheckContext): TrailCheck {
  const c = e.checks[0];
  const { app, project } = ctx;
  const lands = (fact<string>(e, 0, "lands") ?? "both") as Lands;
  const reads = fact<string>(e, 0, "reads") ?? "";
  const signInOnly = ctx.profile.claims.length === 0;
  const tone = STATE_TONE[c.state];
  if (fact<boolean>(e, 0, "unread")) {
    return { tone: "warn", sentence: `Syndra hasn't read Zitadel's settings for ${app} yet.`, faint: ["Zitadel didn't answer. Check again in a moment."] };
  }
  const asserted = fact<boolean>(e, 0, "project_role_assertion");
  const roleCheck = fact<boolean>(e, 0, "project_role_check");
  const jwt = fact<string>(e, 0, "access_token_type") === "JWT";
  const inside = fact<boolean>(e, 0, "id_token_userinfo_assertion");
  const reached = fact<Record<string, boolean>>(e, 0, "reached") ?? {};
  const stale = c.state === "stale" ? [`From Zitadel's last answer, at ${clockOf(fact<string>(e, 0, "read_at"))}.`] : [];

  if (signInOnly) {
    if (c.state === "warn") {
      return {
        tone,
        sentence: `Zitadel lets anyone in the organisation sign in to ${app}.`,
        faint: [`The role check is off for ${project}. ${app} has no groups, so nothing else stops them.`],
        link: ctx.links?.project ? { label: `Turn on the role check for ${project} in Zitadel`, href: ctx.links.project } : undefined,
      };
    }
    return { tone, ring: c.state === "stale", sentence: `Zitadel's settings for ${app} let its claims through.`, faint: [`${project}'s role check decides who may sign in. Nothing else is needed for this shape.`, ...stale] };
  }
  if (c.state === "fail" && !asserted) {
    return {
      tone,
      sentence: `Zitadel doesn't assert ${project}'s roles, so ${app} gets none from it.`,
      faint: [`Turn on "Assert roles on authentication" for ${project} in Zitadel, or have ${app} ask for role scopes. Then check again.`],
      link: ctx.links?.project ? { label: `Assert ${project}'s roles in Zitadel`, href: ctx.links.project } : undefined,
    };
  }
  if (c.state === "fail" && reads && !reached[reads]) {
    const where = READS_LABEL[reads] ?? reads;
    const out: TrailCheck = { tone, sentence: `Zitadel's settings for ${app} keep its claims out of ${where}, where ${app} reads them.` };
    if (reads === "access_token" && lands !== "userinfo") {
      out.faint = [`${app}'s token type in Zitadel is Bearer. Only a JWT access token carries claims.`];
      if (ctx.links?.app) out.link = { label: `Set ${app}'s token type to JWT in Zitadel`, href: ctx.links.app };
      if (ctx.templates.some((t) => t.id === "netbird" && t.reads === reads) && /netbird/i.test(app)) {
        out.faint.push("Or have NetBird read its ID token instead, with `NETBIRD_TOKEN_SOURCE=idToken`. Then check again.");
      }
    } else if (reads === "id_token" && lands !== "access_token") {
      out.faint = [`User Info inside ID Token is off for ${app} in Zitadel.`];
      if (ctx.links?.app) out.link = { label: `Turn on User Info inside ID Token for ${app} in Zitadel`, href: ctx.links.app };
    } else {
      out.faint = [`This shape lands in ${lands === "access_token" ? "the access token" : "userinfo"} only. Change where it lands, below.`];
    }
    return out;
  }
  const parts = [`${project}'s roles are asserted`];
  const readsAccess = !reads || reads === "access_token";
  if (readsAccess && lands !== "userinfo") parts.push(`the access token is ${jwt ? "a JWT" : "Bearer"}`);
  if (lands !== "access_token") parts.push(`userinfo ${inside ? "goes inside" : "stays out of"} the ID token`);
  const faint = [`${list(parts)}.${readsAccess ? "" : ` ${app} doesn't read the access token, so its type doesn't matter.`}`];
  if (roleCheck === false && c.state === "pass") faint.push(`The role check is off for ${project}, so anyone in the organisation can sign in.`);
  return { tone, ring: c.state === "stale", sentence: `Zitadel's settings for ${app} let its claims through.`, faint: [...faint, ...stale] };
}

function check2(e: Explanation, ctx: CheckContext): TrailCheck {
  const keys = fact<string[]>(e, 1, "roles") ?? [];
  const labels = keys.map((k) => ctx.roles.find((r) => r.key === k)?.label ?? k);
  if (keys.length === 0) {
    return {
      tone: "neutral",
      sentence: `${ctx.person} holds no roles in ${ctx.project}.`,
      faint: [`That isn't an error: none of the roles they hold belong to ${ctx.project}.`],
    };
  }
  if (keys.length === 1) {
    return { tone: "healthy", sentence: `${ctx.person} holds ${labels[0]} in ${ctx.project}.`, faint: [`${labels[0]}, as Zitadel shows it.`] };
  }
  return { tone: "healthy", sentence: `${ctx.person} holds ${keys.length} roles in ${ctx.project}.`, faint: [`${list(labels)}, as Zitadel shows them.`] };
}

function check3(e: Explanation, ctx: CheckContext): TrailCheck {
  const c = e.checks[2];
  const at = fact<string>(e, 2, "compiled_at");
  switch (c.state) {
    case "none":
      return {
        tone: "neutral",
        sentence: "Nothing to build for them.",
        faint: [ctx.profile.claims.length === 0 ? "This shape sends nothing of its own." : `They hold no roles in ${ctx.project}.`],
      };
    case "warn":
      return {
        tone: "warn",
        sentence: "Their claims aren't built yet.",
        faint: ["Syndra builds them at their next change. Until then Zitadel gets nothing from Syndra for them."],
      };
    case "fail":
      return { tone: "danger", sentence: "Syndra couldn't read their claims.", faint: [String(fact<string>(e, 2, "error") ?? "")] };
    default: {
      const today = at && new Date(at).toDateString() === new Date().toDateString();
      return {
        tone: "healthy",
        sentence: "Their claims are ready.",
        faint: at ? [`Built at ${clockOf(at)}${today ? " today" : ""}, after their last change.`] : undefined,
      };
    }
  }
}

function describeShape(p: Profile, ctx: CheckContext): string {
  if (p.claims.length === 0) return "It adds nothing; Zitadel's own claims do the work.";
  const tops = Array.from(new Set(p.claims.map((c) => c.path[0])));
  if (p.claims.length === 1 && p.claims[0].kind === "roles" && p.claims[0].path.length === 1) {
    const included = ctx.roles.filter((r) => {
      const m = p.roles?.find((x) => x.key === r.key);
      return m ? m.included : p.include_new;
    }).length;
    const own = (p.roles ?? []).some((r) => r.as);
    return `It makes \`${tops[0]}\` from ${included} of ${ctx.project}'s ${ctx.roles.length} roles, in ${own ? `${ctx.app}'s` : "Syndra's"} words.`;
  }
  if (tops.length === 1 && p.claims.every((c) => c.path.length > 1)) {
    return `It makes one object, \`${tops[0]}\`, holding ${list(p.claims.map((c) => `\`${c.path.slice(1).join(" › ")}\``))}.`;
  }
  const nested = tops.filter((t) => p.claims.some((c) => c.path[0] === t && c.path.length > 1));
  return `It makes ${tops.length === 1 ? "1 claim" : `${tops.length} claims`}${nested.length ? `; ${list(nested.map((n) => `\`${n}\``))} ${nested.length === 1 ? "is an object" : "are objects"}` : ""}.`;
}

function check4(e: Explanation, ctx: CheckContext): TrailCheck {
  const c = e.checks[3];
  const source = fact<string>(e, 3, "source");
  const version = fact<number>(e, 3, "version");
  const suggested = fact<string>(e, 3, "suggested_template");
  const tplName = (id?: string) => ctx.templates.find((t) => t.id === id)?.name ?? id ?? "";
  const tone = STATE_TONE[c.state];
  if (ctx.draft && fact<boolean>(e, 3, "draft")) {
    const leftOut = ctx.draft.leftOut?.length
      ? ` ${list(ctx.draft.leftOut.map((k) => `\`${k}\``))} ${ctx.draft.leftOut.length === 1 ? "is" : "are"} left out until it has a role from ${ctx.project}.`
      : "";
    if (ctx.draft.origin === "template") {
      return {
        tone,
        sentence: `The ${ctx.draft.name} template applies, as a draft.`,
        faint: [source === "app" ? "Not saved yet." : `Not saved yet. ${ctx.app} gets ${ctx.project}'s default until you save.`],
      };
    }
    if (ctx.draft.origin === "copy") {
      return { tone, sentence: `Your draft, copied from ${ctx.draft.name}, applies.`, faint: [`Not saved yet.${leftOut}`] };
    }
    return { tone, sentence: `Your draft of what ${ctx.app} is told applies.`, faint: [`Not saved yet.${ctx.draft.summary ? ` ${ctx.draft.summary}` : ""}${leftOut}`] };
  }
  if (suggested) {
    const keys = fact<string[]>(e, 3, "keys") ?? [];
    return {
      tone,
      sentence: `${ctx.project}'s default sends ${list(keys.map((k) => `\`${k}\``)) || "nothing"}, which ${ctx.app} doesn't read.`,
      faint: [`Start from the ${tplName(suggested)} template.`],
    };
  }
  if (source === "app") {
    if (ctx.profile.claims.length === 0) return { tone, sentence: "The Sign-in only shape applies.", faint: [describeShape(ctx.profile, ctx)] };
    return { tone, sentence: `What ${ctx.app} is told applies, version ${version}.`, faint: [describeShape(ctx.profile, ctx)] };
  }
  return {
    tone,
    sentence: `${ctx.project}'s default applies.`,
    faint: [source === "builtin" ? `Nobody has set one for ${ctx.project}, so it's Syndra's built-in \`roles\` list.` : `It is set on ${ctx.project}'s page.`],
  };
}

const ZITADEL_ADDS = "Zitadel adds `sub`, `email`, `name` and `preferred_username` itself.";

/**
 * Check 5's sentence (A-15 §5), and `foot`, the one faint line under the token
 * that sits below the trail. Whatever the foot doesn't say, the check does.
 */
export function check5(e: Explanation, ctx: CheckContext, asDraft: boolean): TrailCheck & { foot: string } {
  const c = e.checks[4];
  const tone = STATE_TONE[c.state];
  const keys = list(Object.keys(e.token ?? {}).map((k) => `\`${k}\``));
  const answers = asDraft ? "Syndra would answer with" : "Syndra answers with";
  if (c.state === "none") {
    return {
      tone,
      sentence: asDraft
        ? "Zitadel would receive no Syndra claims for them once you save, on purpose."
        : "Zitadel receives no Syndra claims for them, on purpose.",
      faint: [`${ctx.app} reads Zitadel's own \`email\`, \`email_verified\` and \`preferred_username\`. That is all it needs.`],
      foot: ZITADEL_ADDS,
    };
  }
  if (c.state === "fail" && e.checks[0]?.state === "fail") {
    const reached = (e.checks[0].facts.reached ?? {}) as Record<string, boolean>;
    const where = ["access_token", "id_token", "userinfo"].filter((k) => reached[k]).map((k) => READS_LABEL[k]);
    const reads = String(e.checks[0].facts.reads ?? "");
    const its = (READS_LABEL[reads] ?? "its token").replace(/^the /, "its ");
    return {
      tone,
      sentence: where.length ? `${answers} ${keys}, but only for ${list(where)}.` : `${answers} ${keys}, but it doesn't reach the app.`,
      faint: [ZITADEL_ADDS],
      foot: `${ctx.app} reads ${its}, so it sees none of this${/netbird/i.test(ctx.app) ? ", and empties every group it made from a token" : ""}.`,
    };
  }
  if (c.state === "fail") {
    return {
      tone,
      sentence: "Zitadel receives nothing from Syndra for them yet.",
      faint: ["Once their claims are built, it would receive what's below."],
      foot: ZITADEL_ADDS,
    };
  }
  return { tone, sentence: `${answers} ${keys || "nothing"}, below.`, foot: ZITADEL_ADDS };
}

/** Checks 1 to 4, in words; check 5 is drawn with its token by the caller. */
export function checkWords(e: Explanation, ctx: CheckContext): TrailCheck[] {
  return [check1(e, ctx), check2(e, ctx), check3(e, ctx), check4(e, ctx)];
}

/**
 * Check 6 (S3): Zitadel reached Syndra for this app recently. The same for
 * everyone: it reads the app's last call against its last sign-in. An empty
 * list when the answer has no sixth check, so the trail never invents one.
 */
export function check6(e: Explanation, ctx: CheckContext): TrailCheck[] {
  const c = e.checks[5];
  if (!c) return [];
  const f = (k: string) => c.facts?.[k] as string | null | undefined;
  const { app } = ctx;
  if (c.state === "stale" || c.facts?.unread) {
    return [{ tone: "neutral", ring: true, sentence: `Syndra couldn't read its record of Zitadel's calls for ${app}.`, faint: ["Check again in a moment."] }];
  }
  switch (c.state) {
    case "pass":
      return [{ tone: "healthy", sentence: `Zitadel reached Syndra for ${app} recently.`, faint: [`Last called ${formatAt(f("last_call_at"))}, and Syndra served its claims.`] }];
    case "warn":
      return [{
        tone: "warn",
        sentence: `Zitadel reached Syndra for ${app}, but the last answer carried no claims.`,
        faint: [`Last called ${formatAt(f("last_call_at"))}: ${outcomePhrase(f("last_outcome") ?? undefined) || "no reason was recorded"}.`],
      }];
    case "fail": {
      const at = formatAt(f("last_sign_in_at"));
      const lastIn = at ? `, the last ${at},` : "";
      const out: TrailCheck = f("last_call_at")
        ? {
            tone: "danger",
            sentence: `Zitadel stopped reaching Syndra for ${app}.`,
            faint: [`Its last call was ${formatAt(f("last_call_at"))}. People signed in after it${lastIn} and their tokens went out without Syndra's claims.`],
          }
        : {
            tone: "danger",
            sentence: `Zitadel has never called Syndra for ${app}.`,
            faint: [`People signed in${lastIn} and their tokens went out without Syndra's claims.`],
          };
      out.faint!.push("Claims delivery, on System › Zitadel, says what in Zitadel's setup differs.");
      return [out];
    }
    default:
      return [{ tone: "neutral", sentence: `Nobody has signed in to ${app} lately, so there is no call to look at.` }];
  }
}

const COUNT_WORD = ["no", "one", "two", "three", "four", "five", "six", "seven"];

/**
 * The preview's verdict line (A-15 §5), read off the trail's own tones so the
 * line and the trail can't disagree: "All six checks pass", "Check 1 fails",
 * "Checks 1 and 5 fail", "Check 4 needs a look", each " on this draft" with one.
 */
export function verdictOf(checks: { tone: StatusTone }[], draft: boolean): { tone: StatusTone; text: string } {
  const at = (tone: StatusTone) => checks.flatMap((c, i) => (c.tone === tone ? [String(i + 1)] : []));
  const failed = at("danger");
  const warned = at("warn");
  const text = failed.length
    ? failed.length === 1
      ? `Check ${failed[0]} fails`
      : `Checks ${list(failed)} fail`
    : warned.length
      ? warned.length === 1
        ? `Check ${warned[0]} needs a look`
        : `Checks ${list(warned)} need a look`
      : `All ${COUNT_WORD[checks.length] ?? checks.length} checks pass`;
  return { tone: failed.length ? "danger" : warned.length ? "warn" : "healthy", text: `${text}${draft ? " on this draft" : ""}` };
}

/** "All pass" / "No roles" / "Fails check 1" for one sign-in or test person. */
export function verdict(checks: { state: CheckState }[] | null | undefined): { tone: StatusTone; word: string } {
  if (!checks?.length) return { tone: "neutral", word: "Not checked" };
  const failed = checks.findIndex((c) => c.state === "fail");
  if (failed >= 0) return { tone: "danger", word: `Fails check ${failed + 1}` };
  const warned = checks.findIndex((c) => c.state === "warn");
  if (warned >= 0) return { tone: "warn", word: `Check ${warned + 1} needs a look` };
  if (checks[1]?.state === "none") return { tone: "neutral", word: "No roles" };
  return { tone: "healthy", word: "All pass" };
}
