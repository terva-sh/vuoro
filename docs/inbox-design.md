# Inbox design: proposals

Status: proposal, 2026-10-05. Nothing here is decided. The design of record is
[plan.md](plan.md); a proposal the owner accepts gets a decision number and
moves into the plan.

[mockups/inbox.html](mockups/inbox.html) sketches the result with synthetic
data and the real stylesheet. Open it in a browser from `docs/mockups/`.

## What the first real inbox showed

On 2026-10-05 the owner's GitLab account produced 51 open changes. With the
refresh fixed (D36) and the ghost user ignored, ten were on their turn. The
window then showed:

- **Repeated reasons.** One change listed "ehlertjd is waiting for your
  answer in an unresolved thread" five times and "naterichman …" three times.
  A reason is per thread, and the row prints every one.
- **Dead changes first.** "Oldest unanswered first" put a review requested
  1,973 days ago at the top. The rule is right for live work and wrong for
  abandoned work, which never leaves.
- **Low density.** Each row is a card about 170px tall. "GitLab ·
  gitlab.com" repeats on every row of a one-account inbox, and every row has
  two links: the title opens vuoro, and "Open on GitLab ↗" opens the forge.
- **Unused width.** The list stops at about 1,250px, and the rest of a wide
  window is empty.
- **One snooze.** "Snooze 1 day" is the only choice, so a change you know is
  parked comes back every morning.
- **Theirs is long.** Forty-one changes waiting on others share the page with
  the ten waiting on you.

## Proposals

Each one says whether it needs an owner decision.

### 1. One line per kind of reason (no decision; a display change)

Group a row's reasons by rule, and count: "8 threads await your reply: alice
(5), bo (3)". The turn engine keeps one reason per thread, because opening a
row still needs each thread. Only the row's summary changes.

### 2. A Dormant group (decision: the threshold, and whether it is configurable)

A change with no human activity for 90 days moves out of "Your turn" and
"Their turn" into a folded "Dormant" group. Its turn is unchanged, and any
new forge event returns it. `turn.Turn.LastActivity` already exists for this.
Sorting within "Your turn" stays oldest unanswered first (D30), and it works
again once the abandoned changes are out of the way.

The alternatives are to sort by most recent activity, which buries a request
nobody has answered, or to leave it to snooze, which makes the person clear the
same dead changes by hand.

### 3. Compact rows, and a detail pane (decision: the layout)

Rows become two lines: an age badge, the title, and a short project
reference, then the role and the summarised reason. The account label shows
only when there is more than one account. The forge link moves to the detail
pane and a keyboard key.

Selecting a row fills a pane on the right with what is waiting: the threads
that await a reply, with author, file:line, and their first line; who
requested what; and the change's actions: open in vuoro, triage, snooze, and
open on the forge. The inbox snapshot already holds the threads, so the pane
needs no new forge read.

The alternative is a single dense column, which is simpler and keeps the
window narrow, but leaves the person opening each change to learn what it
needs.

### 4. Age you can read at a glance (no decision; follows the palette rule)

The age is a badge whose colour follows the colourblind-safe ramp, blue under
two days, amber under two weeks, and magenta beyond, always with the number in
it. Today the age is muted text at the end of the row.

### 5. Snooze choices (decision: which lengths)

Snooze becomes a menu: until tomorrow, for a week, and until something
changes. A snooze already ends on a new forge event (its event key), so the
last choice is a snooze with no date. The proposal is to drop the fixed one
day.

### 6. Their turn folds by default (no decision)

"Their turn" shows its count and opens on demand, as "Snoozed" does. It is
for checking on, not for working through.

### 7. Keyboard (no decision)

`j`/`k` move, Enter opens in vuoro, `s` snoozes, `t` triages, `o` opens on
the forge, `r` refreshes, and `/` focuses the open-by-URL field. The keys
are listed under the detail pane.

### 8. The header (no decision)

The open-by-URL field currently shows a placeholder URL that reads like a
value. It becomes a smaller "Open a change by URL…" field, and the account's
refresh state moves into the header as one line, with the warning or error
from D36 beside it.

### 9. Review state on the row (later)

A row whose change has a vuoro run shows the run's state ("vuoro review: 5
findings"), so the inbox says what is already drafted. This needs the inbox
to read run records, which the plan keeps out of the inbox package today
(section 3.1), so it would be a join in `app`.

## Configuration that helps today

`flywheel-admin-bot` is a regular GitLab account (`type: HUMAN`), so only
the config can mark it as automated:

```json
"bots": ["flywheel-admin-bot"]
```

on the gitlab.com forge entry. In the owner's inbox this moved two more
changes off their turn.

## Suggested order

1, 4, 6, 7 and 8 are display-only and can land together. 2 and 5 each need
one owner decision. 3 is the largest change and benefits from living with the
others for a few days first.
