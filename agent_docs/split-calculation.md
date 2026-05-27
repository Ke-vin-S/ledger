# Split Calculation Reference

## Overview

Splits are calculated server-side only. The client sends `split_method` and `share_units` per participant. The server computes `share_amount` (integer minor units) and validates the sum.

## Method: equal

- Ignore `share_units` entirely.
- `base = expense.amount / len(participants)` (integer division)
- `remainder = expense.amount % len(participants)`
- Assign `base` to all participants.
- Add `remainder` cents to the **payer's own share** (or the first participant if payer is not splitting).
- This ensures `SUM(share_amount) == expense.amount` exactly.

```go
func equalSplit(total int64, participants []uuid.UUID) map[uuid.UUID]int64 {
    n := int64(len(participants))
    base := total / n
    remainder := total % n
    shares := make(map[uuid.UUID]int64, n)
    for _, p := range participants {
        shares[p] = base
    }
    shares[participants[0]] += remainder // first participant absorbs remainder
    return shares
}
```

## Method: exact

- `share_units` = amount in minor units per participant (int64).
- Server validates: `SUM(share_units) == expense.amount`. Reject 422 if not.
- `share_amount = share_units` (direct assignment).

## Method: percentage

- `share_units` = percentage (float64, e.g. 33.33).
- Server validates: `SUM(share_units) == 100.0` (within ±0.01 tolerance for float rounding).
- `share_amount = ROUND(expense.amount * share_units / 100)` per participant.
- After rounding, re-check sum. Adjust the first participant's share by the delta.

```go
func percentageSplit(total int64, percs map[uuid.UUID]float64) map[uuid.UUID]int64 {
    shares := make(map[uuid.UUID]int64)
    var sum int64
    var first uuid.UUID
    for p, pct := range percs {
        s := int64(math.Round(float64(total) * pct / 100.0))
        shares[p] = s
        sum += s
        first = p
    }
    shares[first] += total - sum // correction
    return shares
}
```

## Method: shares (weights)

- `share_units` = relative weight (int or float, e.g. 1, 2, 1.5).
- `total_weight = SUM(share_units)`
- `share_amount = ROUND(expense.amount * weight / total_weight)` per participant.
- Apply same delta correction as percentage method.

## Validation (all methods)

After calculating shares:
```
SUM(share_amount) MUST equal expense.amount
```
If not: this is a server-side bug — log it, return 500. Do not return a split with an invalid sum.

## Version bump on correction

When an expense is corrected with a new amount or new splits:
1. Insert new `expense_splits` rows with `version = expense.version + 1`.
2. Do NOT delete or update existing `expense_splits` rows.
3. The current split is always `WHERE version = (SELECT version FROM expenses WHERE id = expense_id)`.
