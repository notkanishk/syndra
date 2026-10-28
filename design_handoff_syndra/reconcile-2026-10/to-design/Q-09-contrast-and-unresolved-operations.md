# Q-09 · Quiet text, and changes nobody can vouch for

From Claude Code, 28 Oct 2026. Send this after Q-08. Nothing here reopens a verdict. Two parts, one answer.

---

## Part 1 · Quiet text is too quiet to read

An audit measured the text tokens against WCAG AA. The rule is 4.5:1 for text under 18px, which is all of these. Three of them fail in both themes. Nothing is built yet: the change is to tokens, so it's yours.

### What's measured

Ink at its alpha over each ground (`ui/src/app/globals.css:444-460, 515-528`):

| Token | Used for | Dark (ground, canvas, surface-2) | Light (canvas, white) |
|---|---|---|---|
| `--muted` 0.60 | ledes, notes, secondary lines (319 uses, 13–14px) | 6.7, 6.7, 6.3 ✓ | **4.36** ✗, 4.53 ✓ |
| `--faint` 0.42 dark / 0.45 light | timestamps, "Ends 2 Aug", captions (322 uses, 12.5–14px) | **3.8, 3.8, 3.8** ✗ | **2.8, 2.9** ✗ |
| `--label` 0.34 / 0.40 | `type-label` and `type-nav-group`: column headings, rail group names, field labels (12.5px, 600, caps) | **2.8, 2.9, 2.9** ✗ | **2.5, 2.5** ✗ |

The lowest alpha that clears 4.5:1 on every ground is **0.48 in dark** and **0.61 in light**. The accent, warn and danger text tokens already pass (5.4–11.2).

### Options

**A · Every text token clears 4.5 (recommended).** The order stays the same; the steps get closer, and size, weight and caps carry more of the hierarchy.
- Dark: `--muted` 0.64, `--faint` 0.50, `--label` 0.48
- Light: `--muted` 0.70, `--faint` 0.62, `--label` 0.61

**B · Two text tokens, one decoration token.** `--faint` clears 4.5 as in A. `--label` merges into `--faint` for text, and the 0.34/0.40 step stays only for things that aren't text (dividers, empty rings, a disabled seat's outline), where 3:1 is the rule.

**C · Keep the look, lift only the failures on their worst ground.** Light `--muted` 0.62, the rest as in A. This is the least visible change, but it leaves two nearly equal steps in light.

### What I need

1. Pick A, B or C, or redraw the values.
2. If the steps get too close, say how the hierarchy should be carried instead (weight, size or case).

---

## Part 2 · A change to a connected system that nobody can vouch for

### What happens

Syndra asks a connected system's add-on (TrueNAS today) to do things: set a member's storage password, adopt or release an account, purge one, change a setting. Each call is recorded before it goes out (`backend/internal/db/addon_operations.go`).

Sometimes a call's answer never comes back: the add-on restarted mid-call, the network dropped, Syndra itself restarted. The change **may or may not have happened**. Syndra can't tell which, and must not guess:
- Calling it a success claims what nobody knows.
- Calling it a failure tells a member to try again, against a system that may already hold their new password.

Today these rows are recorded and counted, but **no screen shows them**. The backend has the query ready (oldest first, with calls still in flight left out). Nobody sees one unless they read the database.

### How many, how often

Rare. It needs a crash or a dropped connection in the seconds a call is out. When it does happen, it's usually a password or a purge, which a member or an operator is waiting on.

### What the operator can do about one

- **Check the system and say what happened.** It applied, or it didn't. That settles the row.
- **For a password:** have the member set it again, which is safe once someone has looked.
- Nothing is retried automatically. That's deliberate, because a blind retry of a password or a purge is exactly what the add-on's guards refuse.

### What I need

3. **Where it lives:**
   - **a)** A Notice on the system's page (System › {target}), amber, "N changes Syndra can't confirm", opening a list.
   - **b)** A section of its own on that page.
   - **c)** A row in Review, beside Unfinished revocations.

   I recommend **a**. It's per-system, rare, and the system's page is where the operator would check.
4. **The row:**
   - What was asked (in words: "Set Ada's storage password").
   - When.
   - Who asked.
   - The two settling verbs. I suggest "It applied" and "It didn't apply".
5. **The member's side:** a member whose password change is unconfirmed currently sees nothing special. Should their storage page say so, e.g. "We couldn't confirm your last password change. Makerspace staff will check it."?

---

**Answer both parts in the design project at** `handoff/2026-10-reconcile/from-design/A-09-contrast-and-unresolved-operations.md`, numbered 1–5 as above.
