# Tasks: refinement sweep (A-17)

Source: `design_handoff_syndra/reconcile-2026-10/from-design/A-17-refinement-sweep.md`. Branch `feat/refinement-sweep-build`, which tracks `origin/feat/refinement-sweep` and has `main` (A-16) merged in. UI only, plus one backend copy line.

## 1 · Rules and shared parts (§1, §2.1)
- [x] 1.1 G1–G19 added to `design_handoff_syndra/reconcile-2026-10/RULES.md`.
- [x] 1.2 `FoldChevron` is the one fold control: FoldHeading, SignInRow, and the six hand copies on Pending, Drift, Unfinished revocations, Expiring, the person page and Zitadel.
- [x] 1.3 `EmptyState`: its action is an ArrowLink (href) or an outline sm Button (onClick); the resolved dot is a StatusDot. It gains `lift`/`bloom` for S13 (Pending's EverythingSent).
- [x] 1.4 `CardHeaderLink` renders ArrowLink, and callers no longer type "→".
- [x] 1.5 `accentSoft` retired (2 callers); `link-tint` retired from globals.css once its 10 callers moved.
- [x] 1.6 `ActionOutcome.word` is required, every caller says its act's word, the inline form is dot + word, and the "Applied" fallback is deleted (`OUTCOME_LABEL.applied` = "Done").
- [x] 1.7 `ModalFooter` has a `cancel` slot (ghost, optional `reason`) placed by `CANCEL_FIRST` (Q1). Every dialog with a Cancel beside an act uses it, and `dangerConfirm` never takes a dialog's first focus.
- [x] 1.8 `PageHeader` owns header→content (`HEADER_TO_CONTENT`, Q5; `BELOW_HEADER` for Home and the member's pages). Pages stack at `gap-4`. `FilterBar` takes `read`, placed 10 below it.
- [x] 1.9 `ChosenChip`, the one chosen-with-× shape (RolePicker, RolePickerOne, ServiceAccountDialogs).
- [x] 1.10 SelectionBar ("Select only the N shown", SelectionAction on `buttonClasses`), CancelQueuedStep (PaginationRow), RehearsalDialog (stale plan Notice, footer through the slot), PlanReview (A–Z, wrap, Badge), DegradedBanner (outline sm toggle).
- [x] 1.11 nav: Incoming events matches `/operations` exactly, so it no longer lights on Change history. Still to check in the browser after deploy: does clicking Change history land on it?
- [x] 1.12 Shell: the Advanced/Basic pill beside the logo and the sheet's VIEW_EXPLANATION are gone; inactive rows are `text-muted`; AccountSheet's install control is a Button, its link arrow is hidden from screen readers, and its caption is one sentence.
- [x] 1.13 Badge neutral, RowField and Avatar no longer use alpha text. Em dashes are out of the shared parts' copy and the glossary. Withheld names its actor and its system.

## 2 · Screens (§2.2–§2.28): one commit each
- [x] 2.2 Home (Home, Makerspace, ClaimsLine)
- [x] 2.3–2.5 People list, a person's page and dialogs. The cleanup hint is rewritten at its source, `backend/internal/services/views.go`.
- [x] 2.6, 2.25, 2.26 Requests, My access, Network storage
- [x] 2.7, 2.8, 2.15 Projects (with ProjectClaimDefault's interim fixes), Roles, Roles that wait
- [x] 2.9, 2.27 Apps (header, tools card, preview, history, debug; claims card excluded), Settings
- [x] 2.10, 2.11 Bundles, Automatic rules
- [x] 2.12–2.14, 2.16, 2.17 Insights, Pending changes, Change history, Incoming events, Audit
- [x] 2.18–2.21 Drift, Unfinished revocations, Expiring access, Holds due
- [x] 2.22, 2.23 Zitadel, Connected systems, Service accounts
- [x] 2.24 TrueNAS (TakeAwayDialog renamed RevokeDialog)
- [x] 2.28 Login: untouched (Q4)

## 3 · Guards (mechanical rules)
- [x] 3.1 `plain-language.test.ts`: no em dash in on-screen copy (G11). The Q4 files are parked.
- [x] 3.2 `design-system.test.ts`: no `<button>` styled as a link, and `link-tint` stays retired (G1).
- [x] 3.3 `one-control-surface.test.ts`:
  - [x] the lone-ghost guard (A-16) now covers the whole app;
  - [x] folds only through FoldChevron, with no `›` toggle (G3);
  - [x] no `text-ink/NN` (G13);
  - [x] Cancel goes in ModalFooter's slot beside an act (G4);
  - [x] no nav-section eyebrow (G6);
  - [x] no read line placed by a negative margin (G5);
  - [x] no raw account id rendered as a child (G10);
  - [x] "Replace roles in Zitadel" is argued as destructive.
- [x] 3.4 `EmptyState.test.tsx` checks the action's two forms (G9).

## Q · Owner questions (A-17 §3): today's behaviour, one line each
- [ ] Q1 Dialog footer order. `ui/src/components/ui/Modal.tsx` `CANCEL_FIRST = false`, which was the majority (act first) when asked. `true` gives the recommended Cancel-first. CancelQueuedStep keeps "Keep it" first per RULES. `ready-for-human`
- [ ] Q2 Pending's empty Send button. `sendLabel()` in `ui/src/app/governance/pending/page.tsx` still says "Send 0 changes". `ready-for-human`
- [ ] Q3 My access's "Nine roles across six rooms". The line is unchanged in `MemberAccess.tsx`; deleting it is the one change. `ready-for-human`
- [ ] Q4 Mythology lines (the Zitadel card and the login line). Both unchanged. The em-dash guard parks `LoginDoor.tsx` and `app/zitadel/page.tsx` in `PARKED`. `ready-for-human`
- [ ] Q5 Header→content. `HEADER_TO_CONTENT = 18` in `PageHeader.tsx`; 28 is the recommended value. `ready-for-human`
- [ ] Q6 Ids in Advanced meta (person, project, app client id, grant id in a person's project headings). Left where they were. `ready-for-human`
- [ ] Q7 "Grant direct access". `GIVE_ROLE_LABEL` in `ui/src/components/people/GrantDirectAccess.tsx` holds it. `ready-for-human`

## Left, with reasons
- [ ] L1 G14 "names link by default". UserName, ProjectName and BundleName have no `link` prop yet. The plain names A-17 listed are wrapped in `link-hover` Links on each screen. Flipping the default touches every caller and risks nested anchors, so it is its own change. `needs-triage`
- [ ] L2 A-16's ShapeCard still has a hand-copied chevron and a Cancel-first footer. It was kept as merged; the sweep guards park it. `ready-for-agent` when the claims card reopens.
- [ ] L3 `ActionOutcome` has no "partly done" kind. RevokeDialog's partial result uses `applied` with the word "Partly done", which blooms green. `needs-triage`
- [ ] L4 Polish skipped by the area passes:
  - Insights: LaneMap pills, the Timeline track, Map vs Map | List.
  - Drift and Expiring: hand-made all-clear lines.
  - Holds due: column heads.
  - Role page: no search.
  - `Input` has no sm height (ProjectClaimDefault's 46px fields).
  - StatusSeat's word is a string, so the member loses the Term popover on "Zitadel".
  - The Change history Term still mentions `c_` handles.
  - `machineName` still returns "onboarding (automatic)".
  - `ready-for-agent`
