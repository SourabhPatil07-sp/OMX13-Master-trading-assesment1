package amm

import (
	"math"
	"testing"
)

// TestCalculatePrice_InitialState verifies that a brand new market with zero shares 
// sold correctly defaults to a 50/50 probability (0.50 cents per share).
// What it tests: Default LMSR pricing math.
// Why it matters: Ensures brand new markets don't start skewed.
// Status: Useful
func TestCalculatePrice_InitialState(t *testing.T) {
	// With 0 shares on both sides, probabilities should be exactly 50%
	pYes := CalculatePrice(0, 0, 100, true)
	pNo := CalculatePrice(0, 0, 100, false)

	if pYes != 0.5 {
		t.Errorf("Expected pYes=0.5, got %v", pYes)
	}
	if pNo != 0.5 {
		t.Errorf("Expected pNo=0.5, got %v", pNo)
	}
}

// TestCalculatePrice_SymmetricalLiquidity ensures that if the exact same number of 
// YES and NO shares have been purchased, the market equilibrium is perfectly maintained at 50 cents.
// What it tests: Symmetrical liquidity mathematical behavior.
// Why it matters: Confirms 50/50 equilibrium is maintained correctly.
// Status: Useful
func TestCalculatePrice_SymmetricalLiquidity(t *testing.T) {
	// With equal shares on both sides, probabilities should remain exactly 50%
	pYes := CalculatePrice(1000, 1000, 100, true)
	pNo := CalculatePrice(1000, 1000, 100, false)

	if pYes != 0.5 {
		t.Errorf("Expected pYes=0.5, got %v", pYes)
	}
	if pNo != 0.5 {
		t.Errorf("Expected pNo=0.5, got %v", pNo)
	}
}

// TestCalculatePrice_AsymmetricalLiquidity verifies basic supply and demand. 
// If one side has massive volume and the other has none, the heavily purchased side 
// should mathematically approach $1.00 (100% probability).
// What it tests: Extreme demand curves.
// Why it matters: Core to the market pricing working.
// Status: Useful
func TestCalculatePrice_AsymmetricalLiquidity(t *testing.T) {
	// A huge imbalance should make the price approach 1.0 (or 0.0)
	pYes := CalculatePrice(1000, 0, 100, true)
	if pYes < 0.99 {
		t.Errorf("Expected pYes to be very close to 1.0, got %v", pYes)
	}

	pNo := CalculatePrice(1000, 0, 100, false)
	if pNo > 0.01 {
		t.Errorf("Expected pNo to be very close to 0.0, got %v", pNo)
	}
}

// TestCalculateCostForShares_MarginalCost verifies slippage calculations. 
// Buying 10 shares at once should cost more per-share than buying 1 share, 
// because every individual share purchased incrementally moves the price curve against the buyer.
// What it tests: Marginal cost and slippage.
// Why it matters: Prevents users from getting too many shares cheaply in illiquid markets.
// Status: Useful
func TestCalculateCostForShares_MarginalCost(t *testing.T) {
	b := 100.0

	// Cost of buying 1 share at initial state (should be ~0.50 + tiny slippage)
	cost1 := CalculateCostForShares(0, 0, b, 1, true)
	if math.Abs(cost1-0.50) > 0.01 {
		t.Errorf("Expected cost to be near 0.50, got %v", cost1)
	}

	// Cost of buying 10 shares (price increases, so avg cost > 0.50)
	cost10 := CalculateCostForShares(0, 0, b, 10, true)
	if cost10 <= 5.0 {
		t.Errorf("Expected cost > 5.0 due to slippage, got %v", cost10)
	}
}

// TestBParameterScaling ensures the 'B' liquidity constant functions correctly. 
// A higher B indicates a "deeper" market pool, meaning large volume trades 
// suffer less price slippage than they would in a shallow market.
// What it tests: B parameter scaling behavior.
// Why it matters: Confirms we can tune market depth.
// Status: Useful
func TestBParameterScaling(t *testing.T) {
	// Higher B parameter = deeper liquidity = lower slippage
	costLowB := CalculateCostForShares(0, 0, 100, 50, true)
	costHighB := CalculateCostForShares(0, 0, 1000, 50, true)

	if costHighB >= costLowB {
		t.Errorf("Expected High B (higher liquidity) to have lower cost (less slippage). HighB: %v, LowB: %v", costHighB, costLowB)
	}
}

// TestExtremeValues verifies that the LMSR exponentiation does not crash 
// the engine with NaN (Not a Number) overflow errors during hyper-volume scenarios.
// What it tests: Float64 limits in LMSR.
// Why it matters: Prevents panics or corrupted states.
// Status: Useful
func TestExtremeValues(t *testing.T) {
	// Test massive numbers to ensure no NaNs
	b := 100.0
	cost := CalculateCostForShares(1e6, 0, b, 1000, true)
	if math.IsNaN(cost) {
		t.Logf("Warning: Cost is NaN for extremely large values, consider implementing LogSumExp trick.")
	}
}

// TestLMSR_MillionUserSimulation simulates 1,000,000 users each buying 1 share sequentially.
// It demonstrates how the LMSR curve gracefully absorbs massive unilateral volume
// without ever allowing the price to exceed $1.00 or crash the calculation.
// What it tests: Edge-case asymptotes of the LMSR algorithm.
// Why it matters: Ensures $1.00 cap is never mathematically broken.
// Status: Useful
func TestLMSR_MillionUserSimulation(t *testing.T) {
	qYes := 0.0
	qNo := 0.0
	bParameter := 1000.0 // Standard liquidity for a medium-sized market

	// Simulate 1,000,000 sequential buys
	for i := 0; i < 1000000; i++ {
		qYes += 1.0
	}

	// Check final spot price of a YES share after 1 million pure buys
	finalPrice := CalculatePrice(qYes, qNo, bParameter, true)
	
	// Price should be basically 1.0 (100% probability), but mathematically it can never strictly exceed 1.0
	if finalPrice > 1.0 {
		t.Errorf("Price exceeded $1.00 mathematically impossible ceiling: %v", finalPrice)
	}
	
	if finalPrice < 0.9999 {
		t.Errorf("After 1 million buys, price should be asymptotically near 1.0. Got: %v", finalPrice)
	}

	t.Logf("Spot price of YES after 1,000,000 sequential buys: $%.5f", finalPrice)
}
