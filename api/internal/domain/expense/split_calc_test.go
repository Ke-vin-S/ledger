package expense_test

import (
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/Ke-vin-S/ledger/api/internal/domain/expense"
)

var (
	u1 = uuid.MustParse("00000000-0000-0000-0000-000000000001")
	u2 = uuid.MustParse("00000000-0000-0000-0000-000000000002")
	u3 = uuid.MustParse("00000000-0000-0000-0000-000000000003")
)

func si(id uuid.UUID, amount int64, units float64) expense.SplitInput {
	return expense.SplitInput{UserID: id, ShareAmount: amount, ShareUnits: units}
}

// ── equal ─────────────────────────────────────────────────────────────────────

func TestComputeSplits_Equal_EvenDivision(t *testing.T) {
	got, err := expense.ComputeSplits(expense.MethodEqual, 1000, []expense.SplitInput{si(u1, 0, 0), si(u2, 0, 0)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertAmounts(t, got, 500, 500)
	assertTotal(t, got, 1000)
}

func TestComputeSplits_Equal_RemainderGoesToFirst(t *testing.T) {
	got, err := expense.ComputeSplits(expense.MethodEqual, 1000, []expense.SplitInput{si(u1, 0, 0), si(u2, 0, 0), si(u3, 0, 0)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 1000 / 3 = 333 remainder 1 → first gets 334
	assertAmounts(t, got, 334, 333, 333)
	assertTotal(t, got, 1000)
}

func TestComputeSplits_Equal_SingleParticipant(t *testing.T) {
	got, err := expense.ComputeSplits(expense.MethodEqual, 999, []expense.SplitInput{si(u1, 0, 0)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertAmounts(t, got, 999)
	assertTotal(t, got, 999)
}

func TestComputeSplits_Equal_NilInputs_Error(t *testing.T) {
	_, err := expense.ComputeSplits(expense.MethodEqual, 1000, nil)
	if !errors.Is(err, expense.ErrInvalidSplitData) {
		t.Fatalf("want ErrInvalidSplitData, got %v", err)
	}
}

func TestComputeSplits_Equal_PreservesUserOrder(t *testing.T) {
	got, _ := expense.ComputeSplits(expense.MethodEqual, 100, []expense.SplitInput{si(u2, 0, 0), si(u1, 0, 0)})
	if got[0].UserID != u2 || got[1].UserID != u1 {
		t.Error("participant order must be preserved")
	}
}

// ── exact ─────────────────────────────────────────────────────────────────────

func TestComputeSplits_Exact_Valid(t *testing.T) {
	got, err := expense.ComputeSplits(expense.MethodExact, 1000, []expense.SplitInput{si(u1, 600, 0), si(u2, 400, 0)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertAmounts(t, got, 600, 400)
	assertTotal(t, got, 1000)
}

func TestComputeSplits_Exact_SumMismatch_Error(t *testing.T) {
	_, err := expense.ComputeSplits(expense.MethodExact, 1000, []expense.SplitInput{si(u1, 600, 0), si(u2, 300, 0)})
	if !errors.Is(err, expense.ErrInvalidSplitSum) {
		t.Fatalf("want ErrInvalidSplitSum, got %v", err)
	}
}

func TestComputeSplits_Exact_EmptyInputs_Error(t *testing.T) {
	_, err := expense.ComputeSplits(expense.MethodExact, 1000, nil)
	if !errors.Is(err, expense.ErrInvalidSplitData) {
		t.Fatalf("want ErrInvalidSplitData, got %v", err)
	}
}

// ── percentage ────────────────────────────────────────────────────────────────

func TestComputeSplits_Percentage_EvenSplit(t *testing.T) {
	got, err := expense.ComputeSplits(expense.MethodPercentage, 1000, []expense.SplitInput{si(u1, 0, 50), si(u2, 0, 50)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertAmounts(t, got, 500, 500)
	assertTotal(t, got, 1000)
}

func TestComputeSplits_Percentage_ThirdsSumCorrectly(t *testing.T) {
	got, err := expense.ComputeSplits(expense.MethodPercentage, 1000,
		[]expense.SplitInput{si(u1, 0, 33.34), si(u2, 0, 33.33), si(u3, 0, 33.33)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertTotal(t, got, 1000)
}

func TestComputeSplits_Percentage_LastAbsorbsRemainder(t *testing.T) {
	// Three equal thirds: floor(1000*0.3333) = 333 each, last gets remainder
	got, err := expense.ComputeSplits(expense.MethodPercentage, 1000,
		[]expense.SplitInput{si(u1, 0, 33.33), si(u2, 0, 33.33), si(u3, 0, 33.34)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertTotal(t, got, 1000)
	if got[2].ShareAmount <= 0 {
		t.Error("last participant must get positive share")
	}
}

func TestComputeSplits_Percentage_SumNot100_Error(t *testing.T) {
	_, err := expense.ComputeSplits(expense.MethodPercentage, 1000, []expense.SplitInput{si(u1, 0, 60), si(u2, 0, 30)})
	if !errors.Is(err, expense.ErrInvalidSplitData) {
		t.Fatalf("want ErrInvalidSplitData, got %v", err)
	}
}

func TestComputeSplits_Percentage_StoresShareUnits(t *testing.T) {
	got, _ := expense.ComputeSplits(expense.MethodPercentage, 1000, []expense.SplitInput{si(u1, 0, 70), si(u2, 0, 30)})
	if got[0].ShareUnits == nil || *got[0].ShareUnits != 70 {
		t.Error("share_units must be stored for percentage splits")
	}
}

// ── shares ────────────────────────────────────────────────────────────────────

func TestComputeSplits_Shares_EqualShares(t *testing.T) {
	got, err := expense.ComputeSplits(expense.MethodShares, 1000, []expense.SplitInput{si(u1, 0, 1), si(u2, 0, 1)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertAmounts(t, got, 500, 500)
	assertTotal(t, got, 1000)
}

func TestComputeSplits_Shares_UnequalShares(t *testing.T) {
	got, err := expense.ComputeSplits(expense.MethodShares, 1000, []expense.SplitInput{si(u1, 0, 3), si(u2, 0, 1)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertAmounts(t, got, 750, 250)
	assertTotal(t, got, 1000)
}

func TestComputeSplits_Shares_ThreeWayOddAmount(t *testing.T) {
	got, err := expense.ComputeSplits(expense.MethodShares, 1001,
		[]expense.SplitInput{si(u1, 0, 1), si(u2, 0, 1), si(u3, 0, 1)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertTotal(t, got, 1001)
}

func TestComputeSplits_Shares_ZeroUnits_Error(t *testing.T) {
	_, err := expense.ComputeSplits(expense.MethodShares, 1000, []expense.SplitInput{si(u1, 0, 0), si(u2, 0, 1)})
	if !errors.Is(err, expense.ErrInvalidSplitData) {
		t.Fatalf("want ErrInvalidSplitData, got %v", err)
	}
}

func TestComputeSplits_Shares_StoresShareUnits(t *testing.T) {
	got, _ := expense.ComputeSplits(expense.MethodShares, 1000, []expense.SplitInput{si(u1, 0, 3), si(u2, 0, 1)})
	if got[0].ShareUnits == nil || *got[0].ShareUnits != 3 {
		t.Error("share_units must be stored for shares splits")
	}
}

// ── unknown method ─────────────────────────────────────────────────────────────

func TestComputeSplits_UnknownMethod_Error(t *testing.T) {
	_, err := expense.ComputeSplits("weighted", 1000, []expense.SplitInput{si(u1, 0, 1)})
	if err == nil {
		t.Fatal("expected error for unknown split method")
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

func assertAmounts(t *testing.T, got []expense.SplitEntry, want ...int64) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("len(splits) = %d, want %d", len(got), len(want))
	}
	for i, e := range got {
		if e.ShareAmount != want[i] {
			t.Errorf("splits[%d].ShareAmount = %d, want %d", i, e.ShareAmount, want[i])
		}
	}
}

func assertTotal(t *testing.T, got []expense.SplitEntry, want int64) {
	t.Helper()
	var total int64
	for _, e := range got {
		total += e.ShareAmount
	}
	if total != want {
		t.Errorf("total = %d, want %d", total, want)
	}
}
