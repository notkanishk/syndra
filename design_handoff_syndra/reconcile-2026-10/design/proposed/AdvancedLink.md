# AdvancedLink (proposed, A-20, 5 Nov 2026)

A link from a Basic surface to an Advanced-only page that switches the view on the way, so the reader lands with a rail that has the row. Not a new part on screen: it is what `ArrowLink` and `NextStep` do when the destination is Advanced-only and the view is Basic. Named here so the rule has a home.

## What it is
`ArrowLink` as it is (violet, ending in →, A-08 2a on hover). On click, if `useUiView().view === "basic"` and `nav.ts` marks the destination `advancedOnly`, it sets the view to Advanced first (the switch glides, S10; the Advanced rail opens by Unfold, A-08 §3) and then navigates. If the view is already Advanced, or the destination is in Basic, it is a plain link. Nothing else changes: no scroll, no reveal, no second control.

## Where it appears
Every link from a Basic-visible surface to an Advanced-only route. Today those are plain links (A-20 §1.5): Home's "Expiring access" and "and N more", "Open bundles", the Makerspace strip's three tiles, "Decide on Drift", every `NextStep`, the storage card's three links, "Set in Roles that wait", "Check a project's roles in Zitadel". Home's sentence "Pending changes and Drift are in Advanced." becomes one of these.

## Copy
The link's own words, unchanged. It never says "in Advanced": the view change is the answer, not a warning.

## Motion
The ViewSwitch glides (S10) and the Advanced rail unfolds (A-08 §3) before the route changes. No arrive on the landing page beyond S11's own.

## Not for
- A page that exists in both views: a plain link.
- Revealing a panel on the same page: that is `revealInAdvanced`, which stays for its one use (the rule behind a role on a person's page).
- A button: a button does something; this only goes somewhere.

## Props
No new props. `ArrowLink` and `NextStep` read the view and `nav.ts` themselves. Tests: a Basic click on an Advanced href sets the view before `router.push`; an Advanced click does not touch the view; a Basic href never does.
