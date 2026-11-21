# Design decision call — template

The orchestrator sends this to `syndra-designer` for a short call. Taste is decided here; it is never reviewed here.

```markdown
Short design-decision call, not a review. Read only what this names.

Context: <spec section, owner decisions, the board frame>.
Items (from the digest, marked "needs design decision"):
1. <item>: <facts with numbers>. Options: A <…>, B <…>. Reviewer recommends <X>.

Return, per item:
- your call;
- one line of why, against Syndra's sources and the owner's taste;
- the exact spec a builder applies.

If a call reverses something the owner accepted, add a numbered owner question, with your recommendation worded so "yes" accepts it. It will be shown to him visually.
Under 20 lines. Write no files.
```
