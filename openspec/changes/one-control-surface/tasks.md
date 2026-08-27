# Tasks

## 1. The variant rule

- [x] 1.1 Row and finding actions to `outline`: `Change` and `Roll back to this`
      (mappings), `Hold` (people on target), `Bring accounts in line`,
      `Forget this binding`, `Resolve this finding`, `Decide who owns it`,
      `Adopt` (inventory).
- [x] 1.2 `TakeAwayDialog`'s terminal `Close` to the default, matching
      `RemovalDialog` — the same situation had two treatments.
- [x] 1.3 The three maintenance-state buttons all `outline`. One of them was
      `ghost` when it was the state already in force, so a row of three showed
      one without a border, which reads as a rendering fault rather than as
      emphasis — the label and the disabled state already say which is current.

## 2. One definition per surface

- [x] 2.1 `PILL` exported from `Button`; imported by `Tabs`, `FilterPills`,
      `ViewSwitch`, `ChimeToggle` and the request duration choice.
- [x] 2.2 `Tabs` added; the person page's and the drift queue's hand-rolled tab
      rows both use it.
- [x] 2.3 Three copies of the "load more" pill (`UnexplainedAccess`,
      `PersonActivity`, `/users`) and the claim-remove button
      (`TokenFormatEditor`) to `Button`.
- [x] 2.4 `Badge` gains `dangerSoft` and `warnSoft`; eight hand-rolled status
      pills across six files converted.
- [x] 2.5 `StatusDot` / `STATUS_TONE` moved to `Badge` and shared between the
      target health readings and the add-on index, which had been using colour
      with no dot.
- [x] 2.6 In-sentence identifiers to `Mono` across the target plane and storage.

## 3. Layout

- [x] 3.1 The publish row stacks its input above its button, matching the
      Maintenance panel on the same page. Side by side they were ~48px against
      ~32px.

## 4. The guard

- [x] 4.1 `one-control-surface.test.ts` — reads the source, fails on a
      hand-rolled pill control or status pill outside `components/ui`, with the
      shell chrome and error pages exempt and the exemption argued in place.
- [x] 4.2 It found five offenders the visual pass had missed; all fixed.
- [x] 4.3 Mutation-checked: restoring one hand-rolled pill fails it with the
      file and line.

## 5. Against the board

- [x] 5.0 `BOARD-AUDIT.md` — §19–§31 read section by section against what
      shipped. Faithful: the nav delta, the member's three states, all five
      health readings, the unmanaged inventory's stale-blocks-adoption rule, the
      connection instructions, and `ReadFreshness` as one component.
- [x] 5.0a **One real gap.** §21 draws *confirmation required* beside the
      operations that stop and ask. Every operation carries `confirm`, the type
      declares it, and the capability list never rendered it — so the one place
      an operator can learn which operations will ask said nothing. Fixed, with
      a test, mutation-checked.
- [x] 5.0b The freshness strip's `Read again` is the strip's only action and was
      `ghost`; now `outline`, per §1.
- [ ] 5.0c Maintenance buttons are labelled with states (`Draining`) where the
      board labels them with verbs (`Drain`), so the buttons agree with the
      definition list above them instead. A button named for a state is weaker
      than one named for the act. Left as built; worth revisiting with the copy.

## 6. Verification

- [x] 6.1 `bun run test` (786), `bun run lint`, `bun run build`.
- [x] 6.2 Looked at in a browser, both themes: the target page, the add-on
      index, the drift queue, projects, requests.

## 7. The adoption form (reported from production, 2026-08-27)

- [x] 7.1 The panel opened after the whole inventory rather than under the row
      whose Adopt was clicked. `CardRow` already had a disclosure that puts a
      panel under its own row, in the product's one disclosure motion — and it
      silently dropped the panel whenever `onToggle` was absent, which is every
      row that already holds a button. That branch now renders, and the adoption
      form and its outcome both go through it.
- [x] 7.2 The confirm was `dangerConfirm`. Adoption gives somebody an account;
      red is this product's word for taking one away. Now `accent`, with the
      irreversibility where it belongs — rung 3, and a sentence that says there
      is no undo.
- [x] 7.3 The copy read as a contradiction: "hands its home directory ... to
      that person" directly above "nothing on the account changes now", with
      "that person" referring to a field that had not been asked for yet.
      Rewritten so that what MOVES (who the account belongs to) and what STAYS
      (everything in it, untouched by Syndra) are named apart, and the person is
      referred to only as "named below". The subject field gained a visible
      label; it had a placeholder and an `aria-label`, and a placeholder is gone
      the moment anybody types.

## Open

- [ ] 7.4 The shell's own chrome is exempt by argument, not by inspection. The
      tab bar and the account sheet were not read closely in this pass.
- [ ] 7.5 Three field hints are still written INSIDE their `<label>`
      (`DormantAccounts`, and two others), which makes each control's accessible
      name its title plus a paragraph. `FieldLabel`/`FieldHint` are the fix, and
      it is a sweep rather than a fix, so it is recorded rather than smuggled
      into this one.
