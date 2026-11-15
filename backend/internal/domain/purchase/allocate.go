package purchase

import "math/big"

// Allocate splits total (hundredths) across items in proportion to their weights, so the
// shares add up to exactly total. Each share is rounded down and the leftover hundredths go,
// one each, to the items with the largest fractional remainders (earlier items win ties).
//
// It returns all zeros when total is zero or every weight is zero.
func Allocate(total int64, weights []int64) []int64 {
	shares := make([]int64, len(weights))
	if total == 0 || len(weights) == 0 {
		return shares
	}

	weightSum := new(big.Int)
	for _, weight := range weights {
		weightSum.Add(weightSum, big.NewInt(weight))
	}
	if weightSum.Sign() == 0 {
		return shares
	}

	remainders := make([]*big.Int, len(weights))
	var assigned int64
	for index, weight := range weights {
		exact := new(big.Int).Mul(big.NewInt(total), big.NewInt(weight))
		quotient, remainder := new(big.Int).QuoRem(exact, weightSum, new(big.Int))
		shares[index] = quotient.Int64()
		remainders[index] = remainder
		assigned += shares[index]
	}

	for leftover := total - assigned; leftover > 0; leftover-- {
		best := -1
		for index, remainder := range remainders {
			if remainder.Sign() < 0 {
				continue // already received its extra hundredth
			}
			if best == -1 || remainder.Cmp(remainders[best]) > 0 {
				best = index
			}
		}
		shares[best]++
		remainders[best] = big.NewInt(-1)
	}
	return shares
}
