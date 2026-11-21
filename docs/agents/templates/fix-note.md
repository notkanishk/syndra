# Fix note — template

The orchestrator writes this for a **fresh** builder on each fix round. It replaces the old agent's history.

```markdown
Fix round <n> of 4 on PR #<n> (`<branch>`, head `<sha>`). Model: <opus|sonnet>.
Begin by merging current `origin/feat/addon-contract`.

Fix (each needs a test or layout-check rule that fails without it):
1. **<P0|P1> — <title>.** `<file:line>`. Scenario: <inputs → wrong result>. Change: <what>. Check: <how>.
2. …

Owner decisions since the last round: <date, words, or "none">.
Not this round: <P2/P3 items going to the owner's list>.
Gates: all, once. Layout replay only if CSS changed. Push and report the head SHA.
```
