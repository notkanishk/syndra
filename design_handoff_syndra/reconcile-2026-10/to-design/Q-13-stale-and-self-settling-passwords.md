# Q-13 · Stale data that stays readable, and passwords that settle themselves

From Claude Code, 3 Nov 2026, after the owner's answers to A-09. Both parts revise A-09.

---

## Part 1 · Marking stale data without fading it unreadable

### Owner (3 Nov)

- Fix the accessibility; don't leave it as is.
- Stale data must still *visibly* show that something changed or is wrong.
- Keep it elegant. Ask whether the set below looks busy or chaotic.
- He'd still dim it, by about 80% if not the previous 0.62.

### The set the owner accepted, for you to judge

1. The amber Notice at the top of the content column, as today: "TrueNAS stopped answering at 09:41. Showing what it said then."
2. On each affected card, an amber "Last known · 2 hours ago" in the read line's seat.
3. The card's outline turns amber. A box already means something is wrong.
4. Freshness dots on values become the amber ring, which already means "not a fresh read; Refresh won't help" (86c).
5. Controls that need the system live are turned off, each with one line saying why.
6. The owner's addition: a dim of about 0.80.

### Locked (measured, A-09 §1)

With A-09's tokens (dark `--muted` 0.70, `--faint` 0.56; light 0.78, 0.66), the lowest alpha that clears 4.5:1 on every ground, tints included, is 0.53 dark and 0.63 light. A 0.80 dim over the whole card fails:
- `--faint` drops to about 0.45 (dark) and 0.53 (light), so it fails.
- `--muted` drops to 0.56 and 0.62, so it passes in dark and is borderline in light.

Any whole-card dim below about 0.95 breaks `--faint` in light.

### What I need

1. **Which of 1–6 stay**, and in what form, so the result reads calm. Is the amber outline (3) too much next to the Notice and the amber line?
2. **The dim.** Pick one, or redraw:
   - **(a)** No dim; colour and words carry it.
   - **(b)** Dim only what isn't text (bars, dots, avatars, chart fills) to 0.80; text stays full.
   - **(c)** A stale scope that lifts `--muted` and `--faint` so that after a 0.80 dim they still clear 4.5:1 (e.g. dark faint 0.70, light 0.80 before the dim).
   - **(d)** Something else.
3. **Draw it:** one card fresh, the same card stale, both themes, plus a whole page (System › TrueNAS) stale.

---

## Part 2 · A member's password change that settles itself

### Owner (3 Nov)

As self-driven as possible, with little admin intervention.

### Locked (code)

- A normal password change gets a clear answer: the add-on calls TrueNAS (`user.update`) and replies "done" or "refused".
- "Unknown" happens only when the reply is lost after the request left: the add-on restarted, the network dropped, or Syndra restarted.
- Syndra never stores the password, and TrueNAS never shows one, so nothing can be read back.
- **Sending the same password again is safe.**
  - If the first one landed, the second sets the same value again and replies "done".
  - If the first one never landed, the second sets it and replies "done".
  
  Either way, the second "done" proves TrueNAS now has that password. The page still holds what the member typed (in memory only), so it can resend without asking.
- So the flow becomes:
  1. **The reply is lost.** The page resends the same password once, at once. The member sees only "Changed".
  2. **The resend gets no answer either** (TrueNAS really down). The member is told plainly, the form stays open, and repeating is safe.
  3. **The member leaves.** Their storage page keeps one line until they next set a password, and then it clears itself.
- Password changes never reach the operator's Waiting on you. Only changes nobody else can settle do, such as an account delete.

### What I need

4. **The member's words and frames** for steps 1–3 (step 1 shows nothing extra unless the resend takes noticeably long). Replace A-09 §5's "Makerspace staff are checking".
5. **Confirm the operator side:** A-09 §3–4 keeps purges, and drops `password.set`. `password.rotate` (part of a revoke) is resent by Syndra itself, since Syndra generated that password.

---

**Answer at** `handoff/2026-10-reconcile/from-design/A-13-stale-and-self-settling-passwords.md` on the canvas, numbered 1–5, with the board beside it.
