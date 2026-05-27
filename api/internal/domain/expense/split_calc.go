package expense

import (
	"fmt"
	"math"
)

// ComputeSplits returns the final split entries for the given method.
// For equal: first participant absorbs the integer remainder.
// For percentage/shares: the last participant absorbs the remainder.
func ComputeSplits(method string, amount int64, inputs []SplitInput) ([]SplitEntry, error) {
	switch method {
	case MethodEqual:
		return computeEqual(amount, inputs)
	case MethodExact:
		return computeExact(amount, inputs)
	case MethodPercentage:
		return computePercentage(amount, inputs)
	case MethodShares:
		return computeShares(amount, inputs)
	default:
		return nil, fmt.Errorf("unknown split method: %s", method)
	}
}

func computeEqual(amount int64, inputs []SplitInput) ([]SplitEntry, error) {
	n := int64(len(inputs))
	if n == 0 {
		return nil, ErrInvalidSplitData
	}
	base := amount / n
	remainder := amount - base*n

	out := make([]SplitEntry, n)
	for i, inp := range inputs {
		out[i] = SplitEntry{UserID: inp.UserID, ShareAmount: base}
	}
	out[0].ShareAmount += remainder
	return out, nil
}

func computeExact(amount int64, inputs []SplitInput) ([]SplitEntry, error) {
	if len(inputs) == 0 {
		return nil, ErrInvalidSplitData
	}
	var total int64
	out := make([]SplitEntry, len(inputs))
	for i, inp := range inputs {
		total += inp.ShareAmount
		out[i] = SplitEntry{UserID: inp.UserID, ShareAmount: inp.ShareAmount}
	}
	if total != amount {
		return nil, ErrInvalidSplitSum
	}
	return out, nil
}

func computePercentage(amount int64, inputs []SplitInput) ([]SplitEntry, error) {
	if len(inputs) == 0 {
		return nil, ErrInvalidSplitData
	}
	var sum float64
	for _, inp := range inputs {
		sum += inp.ShareUnits
	}
	if math.Abs(sum-100.0) > 0.01 {
		return nil, fmt.Errorf("%w: percentages sum to %.4f, need 100", ErrInvalidSplitData, sum)
	}

	out := make([]SplitEntry, len(inputs))
	var assigned int64
	last := len(inputs) - 1
	for i, inp := range inputs {
		u := inp.ShareUnits
		var amt int64
		if i == last {
			amt = amount - assigned
		} else {
			amt = int64(math.Round(float64(amount) * inp.ShareUnits / 100.0))
			assigned += amt
		}
		out[i] = SplitEntry{UserID: inp.UserID, ShareAmount: amt, ShareUnits: &u}
	}
	return out, nil
}

func computeShares(amount int64, inputs []SplitInput) ([]SplitEntry, error) {
	if len(inputs) == 0 {
		return nil, ErrInvalidSplitData
	}
	var totalUnits float64
	for _, inp := range inputs {
		if inp.ShareUnits <= 0 {
			return nil, fmt.Errorf("%w: share_units must be positive", ErrInvalidSplitData)
		}
		totalUnits += inp.ShareUnits
	}

	out := make([]SplitEntry, len(inputs))
	var assigned int64
	last := len(inputs) - 1
	for i, inp := range inputs {
		u := inp.ShareUnits
		var amt int64
		if i == last {
			amt = amount - assigned
		} else {
			amt = int64(math.Floor(float64(amount) * inp.ShareUnits / totalUnits))
			assigned += amt
		}
		out[i] = SplitEntry{UserID: inp.UserID, ShareAmount: amt, ShareUnits: &u}
	}
	return out, nil
}
