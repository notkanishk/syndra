# Documentation

Two bodies of documentation live in this repository, and they answer different
questions.

**[`openspec/`](../openspec/) is the authority on intent** — what each capability
is supposed to do, what has been decided, and what is still open. Start there if
you want to understand or change behaviour.

**`docs/` (here) is narrative and historical** — design thinking, self-assessment,
and the implementation plans that produced the code. Start here if you want to
understand *why* something ended up the way it did.

## In this directory

| Document | What it is |
|---|---|
| [`DESIGN-BRIEF.md`](DESIGN-BRIEF.md) | The UI/UX brief — visual language, layout principles, the Basic/Advanced split |
| [`AUDIT.md`](AUDIT.md) | Rolling self-assessment: bloat, spec drift, correctness concerns, prioritized recommendations. Written to be uncomfortable |
| [`UI-CAPABILITY-GAPS.md`](UI-CAPABILITY-GAPS.md) | Backend capabilities with no frontend surface, and the reverse |
| [`adr/`](adr/) | Architecture decision records — a decision, its context, and what was rejected. Unlike the plans below, these are meant to stay true |
| [`superpowers/plans/`](superpowers/plans/) | Dated implementation plans, one per wave of work. Historical record — a plan describes what was intended at the time, not necessarily what shipped |
| [`superpowers/specs/`](superpowers/specs/) | Design documents produced in response to audit findings |
| `assets/` | Images used by the README |

## Where to go instead

| Question | Answer lives in |
|---|---|
| What does this capability do? | [`openspec/INDEX.md`](../openspec/INDEX.md) |
| What is missing or broken? | [`openspec/NEXT.md`](../openspec/NEXT.md) |
| How is the system structured? | [`openspec/changes/syndra-core-architecture/design.md`](../openspec/changes/syndra-core-architecture/design.md) |
| How do I deploy it? | [`DEPLOY.md`](../DEPLOY.md) |
| How do I run it locally? | [`README.md`](../README.md) |
| How do I contribute? | [`CONTRIBUTING.md`](../CONTRIBUTING.md) |
| How do the Zitadel Actions work? | [`zitadel/actions/README.md`](../zitadel/actions/README.md) |
| How do add-ons work, and how do I write one? | [`addons/README.md`](../addons/README.md) |

## A caveat on the historical documents

The plans under `superpowers/` and the older sections of `AUDIT.md` are snapshots.
They were accurate when written and have not been retrofitted as the code moved.
Where a historical document and a spec disagree, **the spec wins** — and where a
spec and the code disagree, that is a bug worth reporting.
