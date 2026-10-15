# Domain Docs

How the engineering skills should consume this repo's domain documentation.
Layout: **single-context**.

## Before exploring, read these

- **`GLOSSARY.md`** at the repo root, if it exists. None yet: `/domain-modeling`
  creates it lazily when a term is actually resolved.
- **`openspec/INDEX.md`**, then the change or capability spec for the area you
  touch. Specs are the contract; where code and spec disagree, flag the drift.
- **`docs/adr/`**: ADRs that touch the area (numbered `001-…`, three digits).
- **`openspec/changes/plain-language-copy/design.md`**: the vocabulary for any
  text a person reads in the UI. Its decided words win over synonyms.

If a file doesn't exist, proceed silently.

## Use the project's vocabulary

When output names a domain concept (a ticket, a refactor, a hypothesis, a test
name), use the term as the specs and the copy guide define it. Don't drift to
synonyms they avoid.

## Flag ADR conflicts

If output contradicts an ADR or a spec's decision, say so explicitly rather than
overriding it silently:

> _Contradicts ADR-001 (event listener wire format), but worth reopening because…_
