package purchase_test

import (
	"math/rand"
	"testing"

	"gumunova/backend/internal/domain/purchase"
)

func sum(values []int64) (total int64) {
	for _, value := range values {
		total += value
	}
	return total
}

func TestAllocateKnownCases(t *testing.T) {
	tests := []struct {
		name    string
		total   int64
		weights []int64
		want    []int64
	}{
		{"proportional", 10_000, []int64{28_000, 28_000, 56_000}, []int64{2_500, 2_500, 5_000}},
		{"three equal parts leave the extra cent to the first", 10_000, []int64{1, 1, 1}, []int64{3_334, 3_333, 3_333}},
		{"largest remainder wins", 100, []int64{1, 2}, []int64{33, 67}},
		{"ties go to earlier items", 1, []int64{5, 5}, []int64{1, 0}},
		{"single item takes everything", 5_050, []int64{999}, []int64{5_050}},
		{"zero total", 0, []int64{1, 2, 3}, []int64{0, 0, 0}},
		{"zero weights", 500, []int64{0, 0}, []int64{0, 0}},
		{"no items", 500, nil, []int64{}},
		{"zero weight item gets nothing", 100, []int64{0, 3, 1}, []int64{0, 75, 25}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := purchase.Allocate(test.total, test.weights)
			if len(got) != len(test.want) {
				t.Fatalf("got %v, want %v", got, test.want)
			}
			for i := range got {
				if got[i] != test.want[i] {
					t.Fatalf("got %v, want %v", got, test.want)
				}
			}
		})
	}
}

func TestAllocateAlwaysAddsUpToTheTotal(t *testing.T) {
	random := rand.New(rand.NewSource(7))
	for round := 0; round < 2_000; round++ {
		weights := make([]int64, 1+random.Intn(12))
		for i := range weights {
			weights[i] = 1 + random.Int63n(5_000_000_000) // up to 50 million in currency units
		}
		total := random.Int63n(999_999_999_999)

		shares := purchase.Allocate(total, weights)

		if sum(shares) != total {
			t.Fatalf("round %d: shares %v add up to %d, want %d", round, shares, sum(shares), total)
		}
		for i, share := range shares {
			if share < 0 {
				t.Fatalf("round %d: negative share %d at %d", round, share, i)
			}
		}
	}
}

func TestAllocateIsMonotonicInTheWeights(t *testing.T) {
	shares := purchase.Allocate(1_000_000, []int64{100, 200, 300, 400})
	for i := 1; i < len(shares); i++ {
		if shares[i] < shares[i-1] {
			t.Errorf("a heavier item must not receive less: %v", shares)
		}
	}
}
