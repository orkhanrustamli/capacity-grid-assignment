# Decisions

Yours to write, not your AI's. Short is good — bullets are fine, and half a page is
plenty. We read this first.

## What did the spec not tell you?

There are things this brief doesn't specify. Which ones did you hit, what did you decide,
and why?

- Weekends: left out of capacity and shown separately.
- Week numbering: ISO weeks, named by their Monday. 2025-12-29 is week 1 of 2026.
- Exactly 100% counts as "full", not "over".
- Zero capacity: any booked hours count as over.
- A capacity edit applies from a chosen week onward, so past weeks keep the old value (a dated history).
- After an edit, only that one person's row is re-read.
- Scale: pagination, an over-only filter and a count of over-allocated people.

## What did you notice that looked wrong?

Anything in the output that didn't match what you expected. Whether you fixed it or left
it, we want to know you saw it.

- hours_per_day rows are 15 pieces of one booking (14 pieces of x hours and 1 of 2x), so they are summed.
- The seed contains weekend hours.
- Some weeks have no bookings but still get columns.
- Eli has 0 hours of capacity but still has work.
- The schema's indexes push long-range queries into full table scans.
- Non-Latin names (Greek, Cyrillic, Chinese/Japanese/Korean) sort after "zz".

## What did the AI get wrong that you caught?

One concrete example. Every real session has one.

- It claimed a flat 80–110 ms cost after measuring only 8 weeks. 53 weeks took 2.3 s, which migration 00004 fixed.
- The plan's 53-week test values were off by one week.
- A sub-select passed as an argument silently stopped Postgres from inlining the function.
- A date test passed for the wrong reason: Date.UTC reads year 0002 as 1902.
- Clicking Load more during a save could undo the saved value on screen.

## What would you do differently with a week?

- A summary table kept up to date on every write.
- Save results shown at page level, which also closes the back-button gap.
- Holidays and time off.
- Login and an audit log.
- Sorting worst-first.
- Re-structure codebase more to split BE and FE folder structure. Current code structure + Docker config made it difficult to do proper package splitting inside especially BE code (/api)
- One shared API contract. Write an OpenAPI document and generate the TypeScript types from it, so the frontend and backend can't drift apart silently. Today they only line up because one person wrote both sides.
- One end-to-end test of the edit flow. Use Playwright against the running Compose stack: edit a capacity, see the row refresh, and see the failure message when the API is down. The unit tests mock the network, and this is the one path a reviewer will click through by hand.