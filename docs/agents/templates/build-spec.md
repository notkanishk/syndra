# Build spec — template

The designer writes this as the last section of the A-NN answer, once the owner has picked. A builder must be able to build from it without asking anything. A missing line is a question for the designer, never a guess. Delete the lines that do not apply.

```markdown
## For the build

- **Builder:** Opus | Sonnet — <one line why: logic, or pure CSS, copy or docs>
- **Branch:** `feat/addon-contract-<slug>` · **replaces:** <what this removes or supersedes>
- **The comp:** `design/<name>-prototype.html` (option <X>) · board frame(s) <ids>
- **Files to touch:** `ui/src/...` (one line each: what changes)
- **Tokens:** <existing ones used> · new: `--<name>` = <value or derivation>, both themes, with fallback
- **Layout:** at 1440 … · at 390 …
- **States:** rest · loading · empty · partial · error · stale · offline · each in one line, with its words
- **Keyboard:** <key → effect>. Focus after each action. What Tab does. Esc.
- **Motion:** <what moves, from → to, duration token, easing token> · reduced motion: <its form> · forced colours: <its form>
- **Words:** <exact copy, per the copy guide; every outcome ends with a next step>
- **Never:** <the cases where it must not show, play or change>
- **Tests to add:** <each guard that must fail without the change>
- **Layout check:** <a rule to add or extend, or "none">
- **Owner decisions this rests on:** <date, his words>
```
