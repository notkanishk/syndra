# Review digest — template

The Opus collator writes this from the code review and the design review.

```markdown
Digest, round <n>, PR #<n> head `<sha>`. Scale: small | full.
Verdict: merge | fix | needs owner.

| # | P | Where | What changes | Why (rule or finding) | Check | Source |
|---|---|---|---|---|---|---|
| 1 | P0 | `file:line` | … | … | … | code / design / both |

**Needs design decision** (the designer is called only for these):
- <item>: the conflict with the board / a new behaviour / a trade-off. Options A, B, with the reviewer's recommendation.

**For the owner's list** (P2/P3, no new round): …
**Verified, not reproduced, or rejected** (one line each, with evidence): …
```
