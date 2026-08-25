# Syndra has no mobile view, and the shell is why

## Why

The application is desktop-only by construction, and not by omission — by
layout. `AppShell` is `flex h-screen overflow-hidden` with a `w-[252px]
flex-none` rail that has no breakpoint, no toggle and no drawer. At 390px the
rail takes 65% of the viewport and the content column gets about 138px. Nothing
else in the tree can rescue that: there are **nine** responsive Tailwind
variants in the entire product, across five files, and the only width media
query in `globals.css` scales the login arch.

That is the blocking issue, but it is not the only one. Two more make a phone
actively unsafe rather than merely cramped:

- **Bulk selection is mouse-shaped.** Five surfaces — People, Unexplained
  access, Requests, Expiring access, Automatic rules — drive selection from
  shift-click ranges and a 4px-threshold pointer-drag paint. On touch a drag is
  a scroll, so the gesture is not degraded, it is absent. Keyboard parity
  exists and needs a keyboard.
- **Information lives in five `title` attributes**, one of which is the only
  explanation for a failed Zitadel read. Hover does not exist on touch, and the
  product's own rule already forbids tooltips.

The member surface is the sharpest case. Members are the largest population,
their whole surface is four screens, and every one of them — their access, a
request, storage, sign-in — is something they do on a phone, standing in the
space, not at a desk.

## What changes

**One application at a different width.** Same routes, same components, same
copy; CSS does the reflow and JS decides only what CSS cannot — the shape of
navigation, and whether a dialog is a dialog or a sheet. No parallel route
group, no phone-only components, no second implementation to keep in step.

Three states, named for devices rather than for numbers: phone below 720,
tablet from 720 to 1080, desktop above it.

- **Navigation** becomes a tab bar where the destination count allows it and a
  rising sheet where it does not, built from `lib/nav.ts` unchanged.
- **Dialogs become sheets** below the tablet breakpoint, reusing the existing
  focus trap and the `busy` dismissal gate.
- **Dense rows disclose** rather than truncate: a primary line, a secondary line
  carrying the access source, and the remaining fields behind a tap.
- **Toasts are removed from the product**, desktop included, and replaced by a
  vocabulary reported by the surface that ran the action.
- **Selection becomes an explicit named mode** on all five surfaces, so nobody
  loses a capability by picking up a phone.
- **Platform behaviour that has never existed** is added: an offline state
  distinct from `degraded`, a session that survives a phone being backgrounded
  for days, an installable home-screen app, and a landscape rule.

## Impact

- Affected specs: `basic-advanced-ia` (the navigation contract gains a touch
  form and a breakpoint model)
- Affected code: `ui/` only. **No backend change is required** — the two
  endpoints the design believed were missing both exist and both already return
  what the screens need.
- Removed dependency: `sonner`, with `lib/toast.ts` and `lib/drain-toast.ts`.
  `lib/drain-outcome.ts` stays; only its presentation changes.
