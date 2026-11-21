# Product

<!-- impeccable:product-schema 1 -->

This file is a map for impeccable, not an authority. Syndra's own sources decide: `design_handoff_syndra/BRIDGE.md`, `design_handoff_syndra/reconcile-2026-10/RULES.md` and `FINAL.md`, `design_handoff_syndra/craft/CRAFT.md`, the grammar of light (`reconcile-2026-10/from-design/A-25-syndra-alive.md` §2), `ui/src/app/globals.css` with `ui/src/components`, and the copy guide (`openspec/changes/plain-language-copy/design.md`). Where this file disagrees with one of them, the source wins and this file is stale. It was written on 21 Nov 2026 from those documents, with no interview. A line the sources do not state is marked (inferred).

## Platform

web

## Users

- **Operators (makerspace staff).** They run an academic makerspace at a university and configured most of its systems. They give people roles, send the changes that wait, and review drift, expiring access and holds. Write for a competent colleague (copy guide §1). On screen they are "makerspace staff"; a member never meets the word "operator" (copy guide §7).
- **Members.** Students, faculty and visiting researchers. They come to see what they can use, to ask for more, and to reach storage, perhaps twice a term. They administer nothing (copy guide §1).
- **The person who runs the server.** One or two people. Text for them is marked as theirs (copy guide §1).

There are two views, never called modes (`openspec/changes/basic-advanced-ia/design.md` §1). **Basic** is a smaller job done completely. **Advanced** adds the machine's words, is for operators only, and is not rendered for members at all. Switching never changes the URL, and Basic never leaves a dead end. Who can reach what is `ui/src/lib/nav.ts`, the one navigation contract.

## Product Purpose

Syndra is the access console of a university makerspace: a policy and orchestration layer over Zitadel, with Google Workspace as the only identity provider (`CLAUDE.md`; `openspec/changes/syndra-core-architecture/design.md` §1). Operators give people roles by hand, by bundle or by automatic rule. Members see their access and storage, and ask for more.

Success, in the owner's words: on arrival the first screen answers "what is this, and is it working?"; the 90% jobs are the easiest to see and reach; the 10% are one deliberate step away, and fully technical when opened (`.claude/agents/syndra-designer.md`, "The owner's taste").

## Positioning

- **Holding is observed.** Who holds what is read from Zitadel; Syndra's records only explain it. Unknown is said as unknown, never as zero (CRAFT §0.4, §12).
- **A preview is the thing it previews.** One shaper computes both the token and its preview (`CLAUDE.md`, "Token shape"; CRAFT §8.1).
- **Removal is named after its source,** and the confirmation says what the person is left holding (`basic-advanced-ia/design.md` §3).
- **An access console with a keeper's voice,** not "a boring corporate admin tool" (CRAFT §0). That this is what a neighbouring console could not copy is (inferred).

## Operating Context

- An operator works in a desktop window; the density is designed at 1440 and 1800 (`globals.css`, "Density"). Boards draw 1440 and 390, in both themes where colour matters (BRIDGE.md).
- The systems are named as they are: Zitadel for sign-in, TrueNAS for network storage, and other connected systems as add-ons (copy guide §1, §7).
- A change may finish now or wait for an operator to send it. Waiting, retries and failures are always said, with the step that is left (RULES, "A step left is said where you took the first one"; G26, G27).
- Times show in the makerspace's configured zone, never the browser's. "Until <date>" means the whole day. Term dates are set in Makerspace settings only (RULES; copy guide §2, "The keeper's voice").
- Design moves as question, answer, board and prototype, then a review of the build against its board (BRIDGE.md).

## Capabilities and Constraints

- **Surfaces.** Operators: Home, People, Projects, Roles, Apps, Requests, Bundles, Automatic rules, Insights, Changes, Review, System and Makerspace settings. Members: My access, Requests, storage and their connected systems (`ui/src/lib/nav.ts`). Structure never moves in response to data.
- **One source each.** Tokens live only in `ui/src/app/globals.css`, navigation only in `ui/src/lib/nav.ts`, and a setting has one write place. Both themes are authored in full; a component never holds a hex (`CLAUDE.md`, "Key Conventions"; RULES).
- **Vocabulary.** One word per thing, for everybody (copy guide §6). Revoke, Remove, Retire and Delete each mean one act; counts say "people"; service accounts are not people (RULES).
- **Messages.** Nothing is a toast and nothing times out. Each kind of message has one part that carries it (RULES G20 to G31).
- **Not decided here.** The type scale and the page frame are marked "recommended, pending owner" in `globals.css` (the Density block). They are recorded as shipped, not as settled.

## Brand Commitments

- **The name** is Syndra.
- **The voice** is plain language: a well-run university office, calm, courteous, exact, never clever, never lecturing (copy guide §2). Syndra speaks as the door-keeper who knows everyone, and the character is in when she speaks and how exactly, not in extra words (copy guide, "The keeper's voice"). Three tones: warm, plain, grave (CRAFT §9.2).
- **The violet mark and the lamp.** The sign-in is an arch of violet light, the mark is a lamp, and the two themes are a dark room and daylight. "She carries a lamp, and the light is her tone of voice" (A-25 §1). The mark is a mark, not a mascot (CRAFT §9.8).
- **"Calm by default."** Spacing, rhythm and type do the work; a box means something is wrong (RULES).
- **"Elegant, posh, elevating, enriching."** The owner's bar for every surface (`.claude/agents/syndra-designer.md`; A-26, the owner on round 2).
- **"Syndra should have a soul … feel alive and a joy to interact with"** (the owner, A-25). Delight is a goal, inside trust: never on a consent, a failure or a loss of access (CRAFT §0.4).

The visual system itself is `DESIGN.md`, which maps `globals.css`.

## Evidence on Hand

- **No real people, anywhere that can leave the machine.** Drawings use WWE ring names; the operator may be "Kanishk Singh"; emails use `example.edu`; never an IP or a hostname, because the repo is public (`.claude/agents/syndra-designer.md`, "Safety and files"; RULES).
- **Captures of staging or production show real members.** They live only under the gitignored `.impeccable/` and are never quoted in an answer, a board or a prototype (BRIDGE.md step 3b).
- **The product never shows sample data.** Degraded shows the last read, with its age, or nothing (RULES).
- **What exists:** the settled choices (`FINAL.md`), the answers (`reconcile-2026-10/from-design/`), the boards and prototypes (`reconcile-2026-10/design/`), the component bundle (`ui/ds-bundle/README.md`), the mark (`ui/src/components/shell/SyndraMark.tsx`), three fonts shipped with the app (`ui/src/app/fonts/`), and the research behind the craft (`craft/CRAFT.md`, `craft/SOURCES.md`).
- **What does not exist, and must not be made up:** testimonials, customer names, usage figures, benchmarks, press.

## Product Principles

1. **"Calm by default."** The first screen answers "what is this, and is it working?". The common jobs are in reach; the rare ones are one deliberate step away, and fully technical when opened.
2. **Elegance by subtraction.** "Show nothing at rest that is not earned." Every sentence earns its place, and a fact is said once.
3. **"One pattern per job, cohesive across the app."** Never a one-off control when a design-system part exists.
4. **"Nothing says what doesn't happen."** Consent, plan rows, outcomes and help text match what the system does, and every outcome ends with a next step, or "Nothing for you to do."
5. **A soul, inside trust.** "Delightful and a pleasure to use", spent in proportion to rarity, and steady under the hand: a clicked control never moves.

Sources: `.claude/agents/syndra-designer.md` ("The owner's taste", "Standing rules"), RULES, CRAFT §0.

## Accessibility & Inclusion

- **Contrast.** Text holds 4.5:1 in both themes, on every ground it can sit on and under the hour's light at its strongest. Tests hold it (`ui/src/__tests__/contrast.test.ts`, `craft-places.test.tsx`, `craft-ceremony.test.tsx`; RULES, "Colour is semantic"). The smallest text is 12.5px (RULES, A-09).
- **Never colour alone.** Every dot, tint and badge has a word beside it (copy guide §9).
- **Forced colours.** Every light is gone and the parts keep their meaning (A-25 §2 rule 6; the `forced-colors` blocks in `globals.css`).
- **Reduced motion.** Every movement has a still form, drawn. Reduced is not none: colour stays, travel goes (RULES, "Motion"; CRAFT §1.9).
- **Reduced transparency.** A translucent pane goes solid and nothing else changes (A-26, "Less asked for"). Nothing translucent ships today, so `globals.css` has no rule for it yet.
- **Keyboard.** Every interactive element has Syndra's focus ring, which appears at once; a skip link leads to the content; nothing a key triggers animates (`globals.css`, base layer; CRAFT §12).
- **Phone.** Nothing a thumb must hit is under 44px. Dialogs become sheets from the edge, and search keeps its seat (`--tap`; RULES, "Phone", G30).
- **Standard.** WCAG 2 at level AA is (inferred): the sources cite its criteria by number (contrast, 1.4.8, 2.5.8) and name no level.
