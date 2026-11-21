# How a change is built

This is the loop that takes a decided design or fix from brief to merge. The owner set it on 21 Nov 2026 ("yes to all"), after one day's numbers: about 25 agent runs. In those runs, a builder fix round cost 1–2 hours, an Opus review cost 2–7 minutes, design rounds cost 200–700k tokens each, and taste was settled after the build. Design moves by `design_handoff_syndra/BRIDGE.md`; this file is the build half.

## The loop

1. **Settle taste before building.** Any visual choice, however small, is shown to the owner first as a small side-by-side artifact. The artifact uses the real tokens and has a replay, a hold-at-peak control and a theme switch. The owner picks before anything is built. ("show me the options visually", 21 Nov 2026.)
2. **Brief a fresh builder.** Use the `syndra-builder` agent (`.claude/agents/syndra-builder.md`). It carries the checklist, the gates and the safety rules, so the brief holds only the task. Use a new agent for each build. Never resume a long-lived one: a resumed agent re-reads its whole history.
3. **Pick the model.** Opus builds anything with logic: once-only gates, state, timing, keyboard handling, search, multi-step flows. Sonnet builds pure CSS, copy and docs.
4. **Build.** Effort is medium. The builder runs every gate once, tests included, and returns only the failures.
5. **Review.** Run the Opus code review and the Opus design review side by side, scaled to the change (BRIDGE 3b). Reviews run at high effort.
6. **Fix, inside the loop limits.**
   - A build loops until no P0 or P1 is left, for at most 4 rounds; then the owner decides whether to continue.
   - P2 and P3 go to the owner's list.
   - Every fix adds one test or layout-check rule that fails without it.
   - Later rounds review only that round's diff.
   - Each fix round goes to a **fresh** builder, handed a short note: the findings, the files, the head SHA. Not the old agent's history.
7. **Merge.** Merge when CI is green (18/18), the merge gate in `CLAUDE.md` passes, and, for a UI change, the owner has said yes on staging.

## Shared ground

- **One branch at a time on `ui/src/app/globals.css`.** Builds that touch it run in sequence. A fix round on any branch begins by merging the current `feat/addon-contract`, so conflicts surface before a review, not after it. (A parallel conflict cost one round on 21 Nov.)
- **Staging is for looking.** Deploy only when the owner will look, or when a design review needs the real page. Do not deploy on every fix round.
- **One layout check per round, by the builder.** Record the staging tape once per session and reuse it. A round that changes no CSS skips the replay and says so.
- **The staging address** is `SYNDRA_STAGING_URL`, set for every agent in the private `.claude/settings.local.json`. Never write it into a tracked file. The repo is public.

## Why these rules

Anthropic's guidance agrees with these rules:
- cap the loop, and restart rather than correct a third time;
- keep context small, and hand over through notes rather than resumed history;
- make verification cheap and run it once;
- set effort and model to the task;
- treat multi-agent work as expensive.

Sources: [Claude Code best practices](https://code.claude.com/docs/en/best-practices.md), [subagents](https://code.claude.com/docs/en/sub-agents.md), [effective context engineering](https://www.anthropic.com/engineering/effective-context-engineering-for-ai-agents), [building effective agents](https://www.anthropic.com/engineering/building-effective-agents), [the multi-agent research system](https://www.anthropic.com/engineering/multi-agent-research-system), [prompt caching](https://code.claude.com/docs/en/prompt-caching.md).
