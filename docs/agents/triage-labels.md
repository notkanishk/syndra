# Triage Labels

The skills speak in terms of five canonical triage roles. The tracker is OpenSpec
(see `issue-tracker.md`), so a label is an inline tag at the end of a `tasks.md`
item, e.g. `- [ ] 4.3 The plan names what it would cancel [ready-for-agent]`.

| Label in mattpocock/skills | Label in our tracker | Meaning                                  |
| -------------------------- | -------------------- | ---------------------------------------- |
| `needs-triage`             | `needs-triage`       | Maintainer needs to evaluate this issue  |
| `needs-info`               | `needs-info`         | Waiting on reporter for more information |
| `ready-for-agent`          | `ready-for-agent`    | Fully specified, ready for an AFK agent  |
| `ready-for-human`          | `ready-for-human`    | Requires human implementation            |
| `wontfix`                  | `wontfix`            | Will not be actioned                     |

`ready-for-human` covers everything operator-gated: anything that writes to
production, the NAS, or Zitadel, which only the owner approves.
