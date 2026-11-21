# Builder brief — template

The orchestrator writes this. `syndra-builder` already carries the checklist, the gates and the safety rules, so this holds only the task. Keep it under 20 lines.

```markdown
Build <what> (<A-NN or task id>). Model: <opus|sonnet>.

- **Branch:** `<branch>` from `origin/feat/addon-contract`. Open a PR into it. Do not merge or deploy.
- **Spec:** `<path to A-NN>` §"For the build" · **comp:** `<prototype path>` option <X>
- **Context it rests on:** <one or two lines: owner decisions, anything the spec assumes>
- **Out of scope:** <what not to touch>
- **Report:** per syndra-builder's "Report" section.
```
