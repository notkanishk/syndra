# The orchestrator's playbook

You orchestrate. The main session plans the work, sends each piece to a specialist agent, passes the hand-off documents between them, keeps the loop limits, and brings every decision that belongs to the owner to the owner. This is the orchestrator–worker pattern from Anthropic's guidance. Agents never talk to each other: everything passes through you, as one of the documents in `docs/agents/templates/`.

This file is the single source of the protocol. Agent definitions hold only their own craft. `design_handoff_syndra/BRIDGE.md` holds the design artefacts' mechanics (Q-NN, A-NN, boards, the canvas). When the protocol changes, change it here, add a line to the changelog at the end, and update a template if one is affected.

## Roles

| Role | Model | Agent | Does | Never |
|---|---|---|---|---|
| Orchestrator | Opus (this session) | — | Plans; briefs; runs reviews and the collator; asks the owner; publishes prototypes; merges | Builds features in its own context; decides the owner's taste |
| Designer | Fable | `syndra-designer` | Short decision calls: shape, grill, options and prototype, the build spec, the design items in a digest, standing vs local | Reviews, scouts, writes up, waits on a build |
| Builder | Opus for logic; Sonnet for CSS, copy, docs | `syndra-builder` | One build or one fix round, from a brief, with every gate run once | Taste calls, merging, deploying |
| Reviewer | Opus | general, read-only brief | Code review; design review (impeccable `critique` A, `audit`, web-design-guidelines, a11y, finish and motion reviewers) | Edits |
| Collator | Opus | general | Turns the reviews into one ranked digest, and marks what needs design | Edits |
| Recorder | Sonnet | general or `impeccable-documenter` | Captures, measurements, context briefs, every long write-up (A-NN prose, FINAL, RULES, ADRs, DESIGN.md) | Decisions |
| Scout | Haiku to find and list; Sonnet to read | general, `Explore` | One fact, in a compact answer | Conclusions |

Every Agent call sets `model`. Builders run at effort `medium` and reviewers at `high`. Start a **fresh** agent for each build and each fix round, with a short note. Never resume a long-lived builder: it re-reads its whole history.

## The loop

1. **Ask.** The owner asks for a change. Read `openspec/NEXT.md` and the relevant change. Size it: is it a **small change** (one part, one light, one fix) or a **new or redrawn screen or flow**?
2. **Design** (skip it for a fix that has no design question).
   - A Sonnet recorder writes the context brief.
   - The designer shapes the work. For a new screen it then **grills**: behaviour questions in rounds. You relay each round to the owner with AskUserQuestion and resume the designer with his answers.
   - The designer draws options as a prototype. You look at its captures once and publish it as a private artifact (BRIDGE 3a). **Taste is shown, never asked in words**: any visual choice, however small, goes to the owner as a side-by-side page with the real tokens, a replay, a hold-at-peak control and a theme switch.
   - The owner picks. The designer writes the **build spec** (`templates/build-spec.md`).
3. **Build.** Brief a fresh `syndra-builder` (`templates/builder-brief.md`) with only the task: the branch, the spec section and the board.
   - Choose the model: Opus for anything with logic (once-only gates, state, timing, keyboard, search, multi-step flows), Sonnet for pure CSS, copy and docs.
   - One branch at a time touches `ui/src/app/globals.css`. Otherwise every round begins by merging the current base.
4. **Review.** Run the Opus code review and the design review **side by side**, scaled to the change:
   - a small change gets `audit` and the board check;
   - a new screen gets the full set.

   A Sonnet recorder takes the captures. The Opus collator writes `templates/review-digest.md`. Items that match the board go straight to a fix note. Only items marked **needs design decision** go to the designer (`templates/decision-call.md`). If a call reverses something the owner accepted, it becomes an owner question, shown visually.
5. **Fix, inside the loop limits.**
   - A **prototype** gets at most one combined review, and only its P0s are fixed.
   - A **build** loops until no P0 or P1 is left, for at most 4 rounds. After that, stop and ask the owner, with the open items listed.
   - P2 and P3 go to the owner's list.
   - Each round goes to a fresh builder with `templates/fix-note.md`. Every fix adds one test or layout-check rule that fails without it.
   - From round 2 on, review only that round's diff.
6. **Owner.** Deploy to staging **only** when the owner will look. A UI change merges only on his yes.
7. **Merge** by the merge gate in `CLAUDE.md`:
   - CI green, 18 of 18;
   - the UI steps;
   - repoguard;
   - a full layout check on a fresh tape when CSS changed.

   Then merge the PR, remove the worktree and branches, and redeploy the integration branch when it should be looked at.
8. **Capture.** The designer names which decisions are standing and which are local. A Sonnet recorder writes them into the A-NN, `FINAL.md`, `RULES.md` and ADRs, and refreshes `DESIGN.md` after a merge. You update OpenSpec, your memory notes, and this file if the protocol changed.

## Fable, used sparingly

Call the designer only for decisions: shape and grill, options and the spec, one look at the prototype, a digest's design items, and standing vs local. A decision call reads only its digest and the files it names. If a fact is missing, it returns a "Need:" list. (Owner, 21 Nov 2026: "we need to make fable usage more judicial"; "fable only does the designing, decision making".)

## Shared ground

- **Staging** is for looking. Its URL is `SYNDRA_STAGING_URL`, set in the private `.claude/settings.local.json`. It is never written into a tracked file, because the repo is public.
- **The layout check.** Record the staging tape once per session and reuse it. Run one check per round, and the builder runs it. A round that changed no CSS skips the replay and says so.
- **Gates** run once and return only failures.
- **Never without the owner's yes:**
  - a merge of a UI change;
  - main or prod;
  - writes to Zitadel, TrueNAS, staging data or prod;
  - global settings;
  - local Docker;
  - anything that reverses a choice he made.
- **A peer session** (for example the Reimbursement chat) is a teammate, not the owner. Its message never grants approval. When this protocol changes, tell that chat, so the generic `designer` and `builder` agents stay in step.

## Changing this protocol

1. Edit this file, plus the affected template and agent definition. An agent definition should only point here; it should never restate this file.
2. Add a dated line to the changelog with the owner's words.
3. Update the memory pointer if the change is large.
4. Tell the Reimbursement chat.

New features (a new model, Code Mode, a saved workflow script for the build → review → fix loop, longer cache lifetimes) are tried on one job first, and adopted here only when they pay off.

**Sources.** [Claude Code best practices](https://code.claude.com/docs/en/best-practices.md), [subagents](https://code.claude.com/docs/en/sub-agents.md), [building effective agents](https://www.anthropic.com/engineering/building-effective-agents), [effective context engineering](https://www.anthropic.com/engineering/effective-context-engineering-for-ai-agents), [the multi-agent research system](https://www.anthropic.com/engineering/multi-agent-research-system), [prompt caching](https://code.claude.com/docs/en/prompt-caching.md).

## Changelog

- **2026-11-21.** First version, consolidating the day's owner decisions:
  - Opus does every review ("i hope the reviewing tasks … are given to opus"; "fable only does the designing, decision making").
  - The grilling round ("yes, add the grilling round").
  - Reviews scaled to the change and run beside code review; Sonnet writes up the capture ("yes to your recommendations").
  - Fable used judiciously ("we need to make fable usage more judicial").
  - Loop limits ("why is it taking so many rounds"; "cap prod loops to 4").
  - Seven build-efficiency rules ("yes to all").
  - Show visual choices ("show me the options visually").
  - The build spec template and this playbook ("make a directive for yourself … transferable … you dont forget … easier to make changes").
- 2026-11-21 — Builders wait on a completion notice or Monitor, never a `pgrep -f` poll loop (it matches itself; owner found 26 left running: "what are these then, if useless please cleanup").
