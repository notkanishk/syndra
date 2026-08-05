# Login Doorway

**Status:** Complete
**Source:** `design_handoff_syndra_login/` — README, `login-reference.html`, `Syndra Brand.dc.html` panel `8a`
**Phase:** 5.5
**Fidelity:** High. Colours, typography, spacing, easing curves and durations in the handoff are final.

## Why

`/login` is the only unauthenticated route and the only screen a visitor sees before deciding
whether this software is trustworthy. It was carrying the pre-rename two-column marketing layout:
a headline, a paragraph that changed text depending on whether an identity provider was
configured, and a `Continue` link in a panel. Nothing on it said what happens when the provider
does not answer — a member who could not get in learned that from a console error, or from
nothing at all.

The brand exploration settled the screen. It is ceremonial rather than operational: an arch (the
doorway), an orb suspended in it, the wordmark, and one button on the threshold. Its three states —
resting, opening, refused — are told **through the arch itself** rather than through banners. That
is the whole reason the design exists: a red toast above a working login screen says a thing failed;
an arch that closes into a complete amber line says *the door did not open*, which is the actual
fact.

## What Changes

**One screen.** `/login` is rebuilt to the handoff. There is no email field, no password field, no
"or continue with", no sign-up and no password reset — Zitadel is the sole identity provider and
owns all of that.

**Three states, one enum.** `idle → opening` on click; `unreachable` when the visitor arrives back
from a failed round trip. `unreachable → idle` on retry replays the entrance, which is the door
reopening.

**Ten error codes, three honest messages.** `/auth/callback` and `/auth/zitadel` between them
redirect with ten distinct `?error=` values. The handoff specified one failure copy. Flattening
`access_denied` into "Zitadel didn't answer" would tell a member the provider was silent when it had
refused them by name, so the failure picture stays single and the sentence behind it has three
variants: refused, handshake, silent. The code itself is never rendered — it arrives in a URL anyone
can type.

**The dev identity picker keeps the ceremony.** The handoff has no demo mode, but `/login` still
lists five demo identities when `ZITADEL_DOMAIN` is unset. Rather than shipping two unrelated login
screens, the same door opens and the identities fade in behind it, in the slot the handoff gives to
the handoff message. No fourth state, no second layout.

**A brand violet ramp, spent once.** The brand locks `#7f5af0` as the primary accent. The app's dark
room still runs on lime and its light room already runs on violet; unifying them repaints every
screen and is its own change. So `--violet-500/400/300` and `--on-violet` are declared in
`globals.css` as a theme-independent ramp that only the doorway spends today. Unifying later means
pointing `--accent` at them in each theme block — no component changes.

**The stranger's first request is now no request.** `NameResolverProvider` mounts in the root layout,
which wraps the unauthenticated route as well as every signed-in one, so `/login` was firing
`GET /api/proxy/{catalog,bundles}` and collecting 401s. Nothing leaked — the backend refused them
correctly — but a page that exists to earn trust should not open by asking for things it is not
allowed to have. Both queries now take an `enabled` gate fed by the session the layout already
resolves.

**The screen says what Syndra does.** The Syn line closed on "the defence called upon at trial" —
mythology trivia that gives Syn a biography and breaks the board's own voice rule, *a doorkeeper, not
a character*. The gloss survives because "keeps the door" is the screen itself; what follows it now
names Syndra's job rather than more of Syn's. The branding turn's title card — *Syn keeps the door.
Syndra keeps the list.* — takes two placements and no more: the README, and System › Identity
provider, the one screen where the split is operationally literal.

## Impact

- **Affected specs**: operational-readiness — delta at
  [`specs/operational-readiness/spec.md`](specs/operational-readiness/spec.md): one action, state told
  through the composition, error semantics, motion as cover, and the gating of the dev picker
- **Affected code**: `ui/src/app/login/page.tsx`, `ui/src/components/login/{LoginDoor.tsx,ceremony.ts}`,
  `ui/src/lib/login-error.ts`, `ui/src/app/globals.css`, and — for the unauthenticated-fetch gate —
  `ui/src/app/layout.tsx`, `ui/src/components/providers.tsx`, `ui/src/lib/queries/useNameResolver.tsx`, and — for the name — `README.md` and
  `ui/src/app/zitadel/page.tsx`
- **No backend change.** `/auth/zitadel`, `/auth/callback` and `/auth/login` are untouched; the
  screen is a new face on the existing flow.
- **Behaviour change**: the sign-in control is now an `<a href="/auth/zitadel">` rather than a
  button with a click handler, so the flow works with JavaScript disabled and the animation is cover
  for the redirect rather than a gate in front of it.
